package models

import "time"

// APIToken represents a Personal Access Token used for API / Home Assistant access.
type APIToken struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateAPITokenRequest is sent when generating a new token.
type CreateAPITokenRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// APITokenCreatedResponse returns the generated clear-text token once.
type APITokenCreatedResponse struct {
	Token string   `json:"token"`
	Info  APIToken `json:"info"`
}
