package billing

import (
	"time"

	"github.com/android-sms-gateway/server/internal/sms-gateway/models"
)

type paymentStatus string

const (
	paymentInitiated paymentStatus = "initiated"
	paymentPaid      paymentStatus = "paid"
	paymentFailed    paymentStatus = "failed"
)

// Subscription is one row per user - billing is per-account, not per-token, so
// every scoped token issued under an account shares the same subscription.
type Subscription struct {
	models.TimedModel

	UserID           string `gorm:"primaryKey;type:varchar(32)"`
	CurrentPeriodEnd time.Time
}

func (Subscription) TableName() string {
	return "subscriptions"
}

type Payment struct {
	models.TimedModel

	ID                     string `gorm:"primaryKey;type:char(21)"`
	UserID                 string `gorm:"not null;type:varchar(32);index:idx_payments_user_id"`
	PesepayReferenceNumber string `gorm:"not null;type:varchar(64);uniqueIndex:unq_payments_reference"`
	Amount                 float64
	Currency               string        `gorm:"type:varchar(8)"`
	Status                 paymentStatus `gorm:"not null;type:enum('initiated','paid','failed');default:initiated"`
}

func (Payment) TableName() string {
	return "payments"
}

func newPayment(id, userID, referenceNumber string, amount float64, currency string) *Payment {
	//nolint:exhaustruct // partial constructor
	return &Payment{
		ID:                     id,
		UserID:                 userID,
		PesepayReferenceNumber: referenceNumber,
		Amount:                 amount,
		Currency:               currency,
		Status:                 paymentInitiated,
	}
}

// NOTE: no Migrate()/AutoMigrate for this module, deliberately - schema ships via
// a goose SQL migration (internal/sms-gateway/models/migrations/mysql/). AutoMigrate
// was found to be dead/unsafe against the real production schema (see the
// status_webhooks migration incident earlier this session).
