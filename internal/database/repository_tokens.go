package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// CreateAPIToken stores a hashed API token.
func (r *Repository) CreateAPIToken(ctx context.Context, userID, name, tokenHash, prefix string, expiresAt *time.Time) (*models.APIToken, error) {
	query := `
		INSERT INTO api_tokens (user_id, name, token_hash, token_prefix, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at;
	`
	tok := &models.APIToken{
		UserID:      userID,
		Name:        name,
		TokenPrefix: prefix,
		ExpiresAt:   expiresAt,
	}
	err := r.pool.QueryRow(ctx, query, userID, name, tokenHash, prefix, expiresAt).Scan(&tok.ID, &tok.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create api token: %w", err)
	}
	return tok, nil
}

// ListAPITokens returns all tokens for a user.
func (r *Repository) ListAPITokens(ctx context.Context, userID string) ([]models.APIToken, error) {
	query := `
		SELECT id, user_id, name, token_prefix, last_used_at, expires_at, created_at
		FROM api_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC;
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list api tokens: %w", err)
	}
	defer rows.Close()

	var list []models.APIToken
	for rows.Next() {
		var tok models.APIToken
		if err := rows.Scan(
			&tok.ID, &tok.UserID, &tok.Name, &tok.TokenPrefix,
			&tok.LastUsedAt, &tok.ExpiresAt, &tok.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, tok)
	}
	if list == nil {
		list = []models.APIToken{}
	}
	return list, rows.Err()
}

// RevokeAPIToken deletes a token.
func (r *Repository) RevokeAPIToken(ctx context.Context, userID, tokenID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM api_tokens WHERE id = $1 AND user_id = $2;`, tokenID, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke api token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ValidateAPIToken verifies a token hash and returns the associated user's ID and email.
func (r *Repository) ValidateAPIToken(ctx context.Context, tokenHash string) (string, string, error) {
	query := `
		SELECT t.id, t.user_id, u.email
		FROM api_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1 AND (t.expires_at IS NULL OR t.expires_at > NOW());
	`
	var tokenID, userID, email string
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(&tokenID, &userID, &email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", err
	}

	// Update last_used_at asynchronously
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = r.pool.Exec(updateCtx, `UPDATE api_tokens SET last_used_at = NOW() WHERE id = $1;`, tokenID)
	}()

	return userID, email, nil
}
