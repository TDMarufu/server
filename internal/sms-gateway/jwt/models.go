package jwt

import (
	"fmt"
	"strings"
	"time"

	"github.com/android-sms-gateway/server/internal/sms-gateway/models"
	"gorm.io/gorm"
)

type tokenUse string

const (
	accessToken  tokenUse = "access"
	refreshToken tokenUse = "refresh"
)

type tokenModel struct {
	models.TimedModel

	ID        string    `gorm:"primaryKey;type:char(21)"`
	UserID    string    `gorm:"not null;type:char(21);index:idx_tokens_user_id"`
	TokenUse  tokenUse  `gorm:"not null;type:enum('access','refresh');default:access"`
	ParentJTI *string   `gorm:"type:char(21);index:idx_tokens_parent_jti"`
	ExpiresAt time.Time `gorm:"not null;index:idx_tokens_expires_at"`
	RevokedAt *time.Time
	// Scopes is only populated on access-token rows, for the token-listing endpoint;
	// the token's real scopes still live in its signed JWT claims and are re-validated
	// from there on every request - this column is display metadata only.
	Scopes *string `gorm:"type:varchar(512)"`
}

func newAccessTokenModel(userID string, token TokenInfo, scopes []string) *tokenModel {
	joinedScopes := strings.Join(scopes, ",")

	//nolint:exhaustruct // partial constructor
	return &tokenModel{
		ID:        token.ID,
		UserID:    userID,
		TokenUse:  accessToken,
		ExpiresAt: token.ExpiresAt,
		Scopes:    &joinedScopes,
	}
}

func newRefreshTokenModel(userID string, parentJTI string, token TokenInfo) *tokenModel {
	//nolint:exhaustruct // partial constructor
	return &tokenModel{
		ID:        token.ID,
		UserID:    userID,
		TokenUse:  refreshToken,
		ParentJTI: &parentJTI,
		ExpiresAt: token.ExpiresAt,
	}
}

func (tokenModel) TableName() string {
	return "tokens"
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(new(tokenModel)); err != nil {
		return fmt.Errorf("tokens migration failed: %w", err)
	}
	return nil
}
