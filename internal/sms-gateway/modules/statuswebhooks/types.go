package statuswebhooks

import (
	"errors"
	"time"
)

const (
	EventMessageSent      = "message.sent"
	EventMessageDelivered = "message.delivered"
	EventMessageFailed    = "message.failed"
	EventMessageCancelled = "message.cancelled"

	maxConsecutiveFailures = 20
	deliveryTimeout        = 10 * time.Second
	// A single short retry on top of the initial attempt - this is a one-shot
	// delivery per event, not a persistent retry queue like the FCM push module;
	// sustained failure is handled by disabling the webhook, not infinite retry.
	retryDelay = 2 * time.Second
)

var (
	ErrValidationFailed = errors.New("validation failed")
	ErrNotFound         = errors.New("status webhook not found")
)

func newValidationError(field string, value any, err error) error {
	return &validationError{field: field, value: value, err: err}
}

type validationError struct {
	field string
	value any
	err   error
}

func (e *validationError) Error() string {
	return e.field + ": " + e.err.Error()
}

func (e *validationError) Unwrap() error {
	return errors.Join(ErrValidationFailed, e.err)
}

func IsValidationError(err error) bool {
	var ve *validationError
	return errors.As(err, &ve)
}

// StatusChangedEvent is the payload published internally (via pubsub) and signed/
// delivered to registered status webhooks.
type StatusChangedEvent struct {
	Event     string    `json:"event"`
	MessageID string    `json:"messageId"`
	DeviceID  string    `json:"deviceId"`
	State     string    `json:"state"`
	Timestamp time.Time `json:"timestamp"`
}

type statusEventWrapper struct {
	UserID string             `json:"userId"`
	Event  StatusChangedEvent `json:"event"`
}
