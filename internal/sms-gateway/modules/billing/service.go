package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/android-sms-gateway/server/internal/sms-gateway/modules/billing/pesepay"
	"github.com/android-sms-gateway/server/internal/sms-gateway/modules/db"
	"go.uber.org/zap"
)

type Service struct {
	config Config

	payments *Repository
	client   *pesepay.Client
	idgen    db.IDGen

	logger *zap.Logger
}

func NewService(config Config, payments *Repository, idgen db.IDGen, logger *zap.Logger) *Service {
	return &Service{
		config: config,

		payments: payments,
		client:   pesepay.NewClient(config.IntegrationKey, config.EncryptionKey, config.Sandbox),
		idgen:    idgen,

		logger: logger,
	}
}

// Subscribe starts a new payment for the account's monthly subscription and
// returns the Pesepay-hosted checkout URL to redirect the client to.
func (s *Service) Subscribe(ctx context.Context, userID string) (string, error) {
	result, err := s.client.InitiateTransaction(
		ctx,
		s.config.MonthlyPrice,
		s.config.Currency,
		"Monthly API subscription",
		s.config.ResultURL,
		s.config.ReturnURL,
	)
	if err != nil {
		return "", fmt.Errorf("failed to initiate payment: %w", err)
	}

	payment := newPayment(s.idgen(), userID, result.ReferenceNumber, s.config.MonthlyPrice, s.config.Currency)
	if err := s.payments.InsertPayment(ctx, payment); err != nil {
		return "", fmt.Errorf("failed to record payment: %w", err)
	}

	return result.RedirectURL, nil
}

// HandleCallback processes Pesepay's result-callback notification. It never
// trusts the callback body's own status field - Pesepay's documentation states
// the callback is unsigned and unencrypted, so every callback triggers an
// independent CheckPayment call before anything is credited.
func (s *Service) HandleCallback(ctx context.Context, referenceNumber string) error {
	payment, err := s.payments.GetPaymentByReference(ctx, referenceNumber)
	if err != nil {
		return fmt.Errorf("failed to look up payment: %w", err)
	}

	paid, status, err := s.client.CheckPayment(ctx, referenceNumber)
	if err != nil {
		return fmt.Errorf("failed to verify payment: %w", err)
	}

	if !paid {
		s.logger.Info("payment not yet paid",
			zap.String("reference", referenceNumber), zap.String("status", status))
		return nil
	}

	if payment.Status == paymentPaid {
		// Already processed (e.g. a duplicate callback) - extending again would
		// double-credit the subscription period.
		return nil
	}

	if err := s.payments.MarkPaymentStatus(ctx, referenceNumber, paymentPaid); err != nil {
		return fmt.Errorf("failed to mark payment paid: %w", err)
	}

	if err := s.payments.ExtendSubscription(ctx, payment.UserID); err != nil {
		return fmt.Errorf("failed to extend subscription: %w", err)
	}

	s.logger.Info("subscription extended", zap.String("user_id", payment.UserID), zap.String("reference", referenceNumber))

	return nil
}

// IsActive reports whether the account's subscription currently covers access.
// Purely date-based - no separate status flag that could drift out of sync.
func (s *Service) IsActive(ctx context.Context, userID string) (bool, error) {
	sub, err := s.payments.GetSubscription(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check subscription: %w", err)
	}
	if sub == nil {
		return false, nil
	}
	return sub.CurrentPeriodEnd.After(time.Now()), nil
}

// Status returns the raw subscription row (nil if none exists) for the status
// handler to report back to the account.
func (s *Service) Status(ctx context.Context, userID string) (*Subscription, error) {
	return s.payments.GetSubscription(ctx, userID)
}
