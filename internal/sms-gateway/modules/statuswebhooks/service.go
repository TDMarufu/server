package statuswebhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/android-sms-gateway/server/internal/sms-gateway/modules/db"
	"github.com/android-sms-gateway/server/internal/sms-gateway/pubsub"
	"github.com/capcom6/go-helpers/slices"
	"go.uber.org/zap"
)

const pubsubTopic = "status-webhooks"

type StatusWebhookDTO struct {
	ID     string   `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
	// Secret is only ever returned once, from Replace - the stored value is never
	// echoed back by Select, so it can't leak via GET /status-webhooks.
	Secret   string `json:"secret,omitempty"`
	Disabled bool   `json:"disabled"`
}

type Service struct {
	idgen db.IDGen

	webhooks *Repository
	pubsub   pubsub.PubSub
	client   *http.Client

	logger *zap.Logger
}

func NewService(idgen db.IDGen, webhooks *Repository, pubsub pubsub.PubSub, logger *zap.Logger) *Service {
	return &Service{
		idgen: idgen,

		webhooks: webhooks,
		pubsub:   pubsub,
		client:   newHTTPClient(),

		logger: logger,
	}
}

func (s *Service) Select(userID string) ([]StatusWebhookDTO, error) {
	items, err := s.webhooks.Select(WithUserID(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to select status webhooks: %w", err)
	}

	return slices.Map(items, webhookToDTO), nil
}

// Replace creates or updates a status webhook for a user, regenerating its signing
// secret only when it's newly created (an existing secret is preserved on update,
// matching how the device-dispatched webhooks module preserves other fields).
func (s *Service) Replace(userID, extID, rawURL string, events []string) (*StatusWebhookDTO, error) {
	if err := ValidateURL(rawURL); err != nil {
		return nil, newValidationError("url", rawURL, err)
	}

	if len(events) == 0 {
		return nil, newValidationError("events", events, fmt.Errorf("at least one event is required"))
	}

	if extID == "" {
		extID = s.idgen()
	}

	secret := generateSecret()
	if existing, err := s.webhooks.Select(WithUserID(userID), WithExtID(extID)); err == nil && len(existing) > 0 {
		secret = existing[0].Secret
	}

	model := newStatusWebhook(extID, rawURL, secret, strings.Join(events, ","), userID)
	// Re-saving resets disabled/failure-count, letting an operator "re-enable" a
	// webhook that got disabled after too many consecutive failures just by
	// registering it again.
	model.Disabled = false
	model.FailureCount = 0

	if err := s.webhooks.Replace(model); err != nil {
		return nil, fmt.Errorf("failed to replace status webhook: %w", err)
	}

	dto := webhookToDTO(model)
	dto.Secret = secret

	return &dto, nil
}

func (s *Service) Delete(userID, extID string) error {
	if err := s.webhooks.Delete(WithUserID(userID), WithExtID(extID)); err != nil {
		return fmt.Errorf("failed to delete status webhook: %w", err)
	}
	return nil
}

// Notify publishes a message-status-change event for later async delivery to any
// registered status webhooks for that user/event - called from
// messages.Service.UpdateState, not delivered inline on the request path.
func (s *Service) Notify(userID string, event StatusChangedEvent) error {
	wrapper := statusEventWrapper{UserID: userID, Event: event}

	data, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("failed to serialize status event: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.pubsub.Publish(ctx, pubsubTopic, data); err != nil {
		return fmt.Errorf("failed to publish status event: %w", err)
	}

	return nil
}

// Run subscribes to status-change events and dispatches them to every matching,
// non-disabled registered webhook. Runs until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	sub, err := s.pubsub.Subscribe(ctx, pubsubTopic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to status webhooks topic: %w", err)
	}
	defer sub.Close()

	ch := sub.Receive()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			var wrapper statusEventWrapper
			if err := json.Unmarshal(msg.Data, &wrapper); err != nil {
				s.logger.Error("failed to deserialize status event", zap.Error(err))
				continue
			}

			s.dispatch(ctx, wrapper)
		}
	}
}

func (s *Service) dispatch(ctx context.Context, wrapper statusEventWrapper) {
	hooks, err := s.webhooks.SelectActiveForEvent(wrapper.UserID, wrapper.Event.Event)
	if err != nil {
		s.logger.Error("failed to select status webhooks", zap.Error(err))
		return
	}

	for _, hook := range hooks {
		s.deliver(ctx, hook, wrapper.Event)
	}
}

func (s *Service) deliver(ctx context.Context, hook *StatusWebhook, event StatusChangedEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		s.logger.Error("failed to serialize webhook payload", zap.Error(err))
		return
	}

	signature := sign(hook.Secret, payload)

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		if lastErr = s.post(ctx, hook.URL, signature, payload); lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		s.logger.Warn("status webhook delivery failed",
			zap.String("url", hook.URL), zap.Uint64("id", hook.ID), zap.Error(lastErr))
		if recErr := s.webhooks.RecordFailure(ctx, hook.ID); recErr != nil {
			s.logger.Error("failed to record webhook failure", zap.Error(recErr))
		}
		return
	}

	if recErr := s.webhooks.RecordSuccess(ctx, hook.ID); recErr != nil {
		s.logger.Error("failed to record webhook success", zap.Error(recErr))
	}
}

func (s *Service) post(ctx context.Context, url, signature string, payload []byte) error {
	reqCtx, cancel := context.WithTimeout(ctx, deliveryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature-256", "sha256="+signature)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func generateSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read on a supported OS practically never fails; if it ever
		// does, fail loudly rather than silently issue a weak/predictable secret.
		panic(fmt.Errorf("failed to generate webhook secret: %w", err))
	}
	return hex.EncodeToString(buf)
}

func webhookToDTO(w *StatusWebhook) StatusWebhookDTO {
	events := []string{}
	if w.Events != "" {
		events = strings.Split(w.Events, ",")
	}

	return StatusWebhookDTO{
		ID:       w.ExtID,
		URL:      w.URL,
		Events:   events,
		Disabled: w.Disabled,
	}
}
