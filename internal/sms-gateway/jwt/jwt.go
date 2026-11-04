package jwt

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Service interface {
	GenerateTokenPair(
		ctx context.Context,
		userID string,
		scopes []string,
		accessTTL time.Duration,
	) (*TokenPairInfo, error)
	RefreshTokenPair(ctx context.Context, refreshToken string) (*TokenPairInfo, error)
	ListTokens(ctx context.Context, userID string) ([]TokenSummary, error)
	ParseToken(ctx context.Context, token string) (*Claims, error)
	RevokeToken(ctx context.Context, userID, jti string) error
	RevokeByUser(ctx context.Context, userID string) error
}

type Claims struct {
	jwt.RegisteredClaims

	UserID string   `json:"user_id"`
	Scopes []string `json:"scopes"`
}

type RefreshClaims struct {
	Claims

	OriginalScopes []string `json:"original_scopes"`
}

type TokenInfo struct {
	ID        string
	Token     string
	ExpiresAt time.Time
}

type TokenPairInfo struct {
	Access  TokenInfo
	Refresh TokenInfo
}

// TokenSummary is the display-only metadata returned by ListTokens - it never carries
// the raw signed token string, which is only ever returned once at issuance.
type TokenSummary struct {
	ID        string
	Scopes    []string
	CreatedAt time.Time
	ExpiresAt time.Time
}
