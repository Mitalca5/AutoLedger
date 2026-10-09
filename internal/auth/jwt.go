package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired JWT token")
)

// Claims represents JWT custom claims including UserID and Email.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	// SessionID is the refresh token family the access token was issued for. A session that was revoked
	// no longer vouches for it: the routes that mint credentials check that the session is still active.
	SessionID string `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT token for a user with expiration specified in hours.
func GenerateToken(userID, email, secret string, expirationHours int) (string, error) {
	if expirationHours <= 0 {
		expirationHours = 72
	}
	return GenerateAccessToken(userID, email, secret, expirationHours*60)
}

// GenerateAccessToken creates a signed short-lived JWT token with expiration in minutes.
func GenerateAccessToken(userID, email, secret string, expirationMinutes int) (string, error) {
	return GenerateSessionAccessToken(userID, email, "", secret, expirationMinutes)
}

// GenerateSessionAccessToken is GenerateAccessToken for a token bound to the session (refresh token family) it was
// issued for.
func GenerateSessionAccessToken(userID, email, sessionID, secret string, expirationMinutes int) (string, error) {
	if expirationMinutes <= 0 {
		expirationMinutes = 15
	}

	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		Email:     email,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expirationMinutes) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "teslacost",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken parses and validates a signed JWT token.
func ValidateToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GenerateRefreshToken generates a cryptographically secure random 32-byte token (hex string)
// and returns both the plain token and its SHA-256 hash.
func GenerateRefreshToken() (plainToken string, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate random refresh token: %w", err)
	}
	plainToken = hex.EncodeToString(b)
	tokenHash = HashRefreshToken(plainToken)
	return plainToken, tokenHash, nil
}

// HashRefreshToken computes the SHA-256 hex digest of a refresh token string.
func HashRefreshToken(plainToken string) string {
	hash := sha256.Sum256([]byte(plainToken))
	return hex.EncodeToString(hash[:])
}

// NewUUID generates a cryptographically secure random UUID (RFC 4122 v4).
func NewUUID() (string, error) {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for UUID: %w", err)
	}
	u[6] = (u[6] & 0x0f) | 0x40 // Version 4
	u[8] = (u[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		u[0:4], u[4:6], u[6:8], u[8:10], u[10:16]), nil
}


