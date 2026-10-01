package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const TokenPrefix = "al_live_"

// GenerateAPIToken creates a cryptographically secure random token, its display prefix, and its SHA-256 hash.
func GenerateAPIToken() (rawToken, prefix, tokenHash string, err error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	randomPart := hex.EncodeToString(bytes)
	rawToken = TokenPrefix + randomPart
	prefix = rawToken[:len(TokenPrefix)+4] + "..."
	tokenHash = HashAPIToken(rawToken)
	return rawToken, prefix, tokenHash, nil
}

// HashAPIToken computes the SHA-256 hexadecimal hash of an API token.
func HashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// IsAPIToken checks if a token string matches the API token prefix convention.
func IsAPIToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), TokenPrefix)
}
