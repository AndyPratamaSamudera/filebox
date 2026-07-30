package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType distinguishes access and refresh tokens.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// ErrInvalidToken is returned when a token is malformed or fails verification.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims is the JWT payload shared by access and refresh tokens.
type Claims struct {
	UserID   uint64    `json:"user_id"`
	Email    string    `json:"email"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	Type     TokenType `json:"type"`
	jwt.RegisteredClaims
}

// JWTManager issues and verifies JWT tokens.
type JWTManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

// NewJWTManager creates a JWTManager from config values.
func NewJWTManager(accessSecret, refreshSecret string, accessHours, refreshDays int) *JWTManager {
	return &JWTManager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     time.Duration(accessHours) * time.Hour,
		refreshTTL:    time.Duration(refreshDays) * 24 * time.Hour,
	}
}

// RefreshTTL returns the refresh token lifetime.
func (m *JWTManager) RefreshTTL() time.Duration { return m.refreshTTL }

// AccessTTL returns the access token lifetime.
func (m *JWTManager) AccessTTL() time.Duration { return m.accessTTL }

// GenerateAccessToken issues a short-lived access token.
func (m *JWTManager) GenerateAccessToken(userID uint64, email, username, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Email:    email,
		Username: username,
		Role:     role,
		Type:     TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.accessSecret)
}

// GenerateRefreshToken issues a long-lived refresh token bound to a session ID.
func (m *JWTManager) GenerateRefreshToken(userID uint64, email, username, role, sessionID string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Email:    email,
		Username: username,
		Role:     role,
		Type:     TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.refreshSecret)
}

// VerifyAccessToken validates an access token and returns its claims.
func (m *JWTManager) VerifyAccessToken(tokenStr string) (*Claims, error) {
	return m.parse(tokenStr, m.accessSecret, TokenTypeAccess)
}

// VerifyRefreshToken validates a refresh token and returns its claims.
func (m *JWTManager) VerifyRefreshToken(tokenStr string) (*Claims, error) {
	return m.parse(tokenStr, m.refreshSecret, TokenTypeRefresh)
}

func (m *JWTManager) parse(tokenStr string, secret []byte, expected TokenType) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Type != expected {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
