package statuswebhooks

import (
	"fmt"

	"github.com/android-sms-gateway/server/internal/sms-gateway/models"
	"gorm.io/gorm"
)

// StatusWebhook is owned by the account (UserID), not a device - unlike the existing
// device-dispatched `webhooks` module, delivery happens from the server itself, not
// from the phone, so there is no DeviceID here.
type StatusWebhook struct {
	models.SoftDeletableModel

	ID     uint64 `json:"-"  gorm:"->;primaryKey;type:BIGINT UNSIGNED;autoIncrement"`
	ExtID  string `json:"id" gorm:"not null;type:varchar(36);uniqueIndex:unq_status_webhooks_user_extid,priority:2"`
	UserID string `json:"-"  gorm:"<-:create;not null;type:varchar(32);uniqueIndex:unq_status_webhooks_user_extid,priority:1"`

	URL    string `json:"url"    validate:"required,http_url" gorm:"not null;type:varchar(256)"`
	Secret string `json:"secret"                               gorm:"not null;type:varchar(64)"`
	// Events is a comma-joined list of message-status events this webhook fires on
	// (e.g. "message.sent,message.delivered,message.failed").
	Events string `json:"events" validate:"required"          gorm:"not null;type:varchar(256)"`

	// Disabled after FailureCount reaches maxConsecutiveFailures, so a dead URL
	// doesn't get hammered forever. Re-saving the webhook (Replace) resets both.
	Disabled     bool `json:"disabled"      gorm:"not null;default:false"`
	FailureCount int  `json:"-"             gorm:"not null;default:0"`
}

func newStatusWebhook(extID, url, secret, events, userID string) *StatusWebhook {
	//nolint:exhaustruct // partial constructor
	return &StatusWebhook{
		ExtID:  extID,
		URL:    url,
		Secret: secret,
		Events: events,
		UserID: userID,
	}
}

func (StatusWebhook) TableName() string {
	return "status_webhooks"
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(new(StatusWebhook)); err != nil {
		return fmt.Errorf("status_webhooks migration failed: %w", err)
	}
	return nil
}
