package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/teslacost/teslacost/internal/models"
)

// Users, password auth and refresh-token session management.

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error) {
	query := `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, oidc_subject, oidc_provider, display_name, language, distance_unit, created_at, updated_at;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, email, passwordHash).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.Language, &u.DistanceUnit, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, oidc_subject, oidc_provider, display_name, language, distance_unit, created_at, updated_at
		FROM users
		WHERE email = $1;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.Language, &u.DistanceUnit, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, oidc_subject, oidc_provider, display_name, language, distance_unit, created_at, updated_at
		FROM users
		WHERE id = $1;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.Language, &u.DistanceUnit, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users;`
	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// GetUserByOIDCSubject looks up a user by their IdP-issued subject claim.
// This is the primary OIDC identity lookup — email alone is not sufficient
// as the same email can appear across different providers.
func (r *Repository) GetUserByOIDCSubject(ctx context.Context, provider, subject string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, oidc_subject, oidc_provider, display_name, language, distance_unit, created_at, updated_at
		FROM users
		WHERE oidc_provider = $1 AND oidc_subject = $2;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, provider, subject).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.Language, &u.DistanceUnit, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by OIDC subject: %w", err)
	}
	return &u, nil
}

// Errors of the SSO account linking.
var (
	// ErrLocalAccountExists: a local account already has the address of the SSO identity. It is not linked on the
	// strength of the address alone: whoever registered it may not own the mailbox.
	ErrLocalAccountExists = errors.New("a local account already uses this email address")
	// ErrOIDCAlreadyLinked: the account is already tied to an SSO identity.
	ErrOIDCAlreadyLinked = errors.New("the account is already linked to an SSO identity")
	// ErrOIDCIdentityInUse: the SSO identity already belongs to another account.
	ErrOIDCIdentityInUse = errors.New("the SSO identity is already linked to another account")
	// ErrOIDCEmailMismatch: the address of the SSO identity is not the one of the account.
	ErrOIDCEmailMismatch = errors.New("the SSO identity has another email address than the account")
)

// UpsertOIDCUser performs JIT (Just-In-Time) provisioning:
// it creates the user of an SSO identity, or returns the one already linked to it.
//
// A local account that has the same address is linked only when linkLocal is set, which is for an instance that
// has switched local authentication off: its accounts hold no usable password. Otherwise the address does not prove
// that the SSO user owns the account (anyone could have registered it first, with a password of their choosing), and
// ErrLocalAccountExists is returned; the owner links the identity from a signed-in session with LinkOIDCIdentity.
func (r *Repository) UpsertOIDCUser(ctx context.Context, email, subject, provider, displayName string, linkLocal bool) (*models.User, error) {
	query := `
		INSERT INTO users (email, oidc_subject, oidc_provider, display_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (oidc_provider, oidc_subject) WHERE oidc_subject IS NOT NULL
		DO UPDATE SET
			email        = EXCLUDED.email,
			display_name = EXCLUDED.display_name,
			updated_at   = NOW()
		RETURNING id, email, password_hash, oidc_subject, oidc_provider, display_name, language, distance_unit, created_at, updated_at;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, email, subject, provider, displayName).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.Language, &u.DistanceUnit, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		// Conflict on email: an account has this address. Only a local one, with no identity yet, can be linked,
		// and only when the instance allows it; one tied to another identity is never re-pointed.
		var hasIdentity bool
		lookup := r.pool.QueryRow(ctx, `SELECT oidc_subject IS NOT NULL FROM users WHERE email = $1;`, email).Scan(&hasIdentity)
		if lookup == nil && !hasIdentity && !linkLocal {
			return nil, ErrLocalAccountExists
		}
		linkQuery := `
			UPDATE users
			SET oidc_subject  = $1,
			    oidc_provider = $2,
			    display_name  = COALESCE($3, display_name),
			    updated_at    = NOW()
			WHERE email = $4 AND oidc_subject IS NULL
			RETURNING id, email, password_hash, oidc_subject, oidc_provider, display_name, language, distance_unit, created_at, updated_at;
		`
		var linked models.User
		linkErr := r.pool.QueryRow(ctx, linkQuery, subject, provider, displayName, email).Scan(
			&linked.ID, &linked.Email, &linked.PasswordHash, &linked.OIDCSubject, &linked.OIDCProvider, &linked.DisplayName, &linked.Language, &linked.DistanceUnit, &linked.CreatedAt, &linked.UpdatedAt,
		)
		if linkErr != nil {
			return nil, fmt.Errorf("failed to upsert OIDC user: insert=%w, link=%v", err, linkErr)
		}
		return &linked, nil
	}
	return &u, nil
}

// LinkOIDCIdentity ties an SSO identity to the account of a user who is signed in and asked for it. The address of the
// identity must be the account's (compared without regard to case), the account must have no identity yet, and the
// identity no other account.
func (r *Repository) LinkOIDCIdentity(ctx context.Context, userID, email, subject, provider string) (*models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		UPDATE users
		SET oidc_subject = $3, oidc_provider = $4, updated_at = NOW()
		WHERE id::text = $1 AND oidc_subject IS NULL AND LOWER(email) = LOWER($2)
		RETURNING id, email, password_hash, oidc_subject, oidc_provider, display_name, language, distance_unit, created_at, updated_at;
	`, userID, email, subject, provider).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.Language, &u.DistanceUnit, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == nil {
		return &u, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil, ErrOIDCIdentityInUse
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to link the SSO identity: %w", err)
	}
	var identity *string
	var accountEmail string
	if lookup := r.pool.QueryRow(ctx, `SELECT oidc_subject, email FROM users WHERE id::text = $1;`, userID).Scan(&identity, &accountEmail); lookup != nil {
		return nil, ErrNotFound
	}
	if identity != nil {
		return nil, ErrOIDCAlreadyLinked
	}
	return nil, ErrOIDCEmailMismatch
}

// ============================================================================
// Refresh Tokens
// ============================================================================

// CreateRefreshToken inserts a new active refresh token record.
func (r *Repository) CreateRefreshToken(ctx context.Context, userID, tokenHash, familyID string, expiresAt time.Time, ip, userAgent *string) (*models.RefreshToken, error) {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at, created_ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, created_ip, user_agent;
	`
	var rt models.RefreshToken
	err := r.pool.QueryRow(ctx, query, userID, tokenHash, familyID, expiresAt, ip, userAgent).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.FamilyID, &rt.IsRevoked, &rt.ExpiresAt, &rt.CreatedAt, &rt.CreatedIP, &rt.UserAgent,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token: %w", err)
	}
	return &rt, nil
}

// GetRefreshTokenByHash retrieves a refresh token record by its SHA-256 hash.
func (r *Repository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, created_ip, user_agent
		FROM refresh_tokens
		WHERE token_hash = $1;
	`
	var rt models.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.FamilyID, &rt.IsRevoked, &rt.ExpiresAt, &rt.CreatedAt, &rt.CreatedIP, &rt.UserAgent,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &rt, nil
}

// ErrRefreshTokenReused is returned when a previously revoked refresh token is presented again (theft/replay).
var ErrRefreshTokenReused = errors.New("refresh token reuse detected: token was already revoked")

// RotateRefreshToken atomically revokes the old token and issues a new one under the same family.
// If the old token was already revoked, it revokes the entire family and returns ErrRefreshTokenReused.
func (r *Repository) RotateRefreshToken(ctx context.Context, oldTokenHash, newTokenHash string, expiresAt time.Time, ip, userAgent *string) (*models.RefreshToken, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var oldToken models.RefreshToken
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, created_ip, user_agent
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE;
	`, oldTokenHash).Scan(
		&oldToken.ID, &oldToken.UserID, &oldToken.TokenHash, &oldToken.FamilyID, &oldToken.IsRevoked, &oldToken.ExpiresAt, &oldToken.CreatedAt, &oldToken.CreatedIP, &oldToken.UserAgent,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find old refresh token: %w", err)
	}

	// Replay / Reuse Detection: If the old token was already revoked, someone is reusing an old token!
	if oldToken.IsRevoked {
		_, _ = tx.Exec(ctx, `UPDATE refresh_tokens SET is_revoked = TRUE WHERE family_id = $1`, oldToken.FamilyID)
		_ = tx.Commit(ctx)
		return nil, ErrRefreshTokenReused
	}

	// Check if expired
	if time.Now().After(oldToken.ExpiresAt) {
		return nil, errors.New("refresh token expired")
	}

	// Mark old token as revoked
	_, err = tx.Exec(ctx, `UPDATE refresh_tokens SET is_revoked = TRUE WHERE id = $1`, oldToken.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke old refresh token: %w", err)
	}

	// Insert new token with the exact same family_id
	var newToken models.RefreshToken
	err = tx.QueryRow(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at, created_ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, created_ip, user_agent;
	`, oldToken.UserID, newTokenHash, oldToken.FamilyID, expiresAt, ip, userAgent).Scan(
		&newToken.ID, &newToken.UserID, &newToken.TokenHash, &newToken.FamilyID, &newToken.IsRevoked, &newToken.ExpiresAt, &newToken.CreatedAt, &newToken.CreatedIP, &newToken.UserAgent,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rotated refresh token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit token rotation: %w", err)
	}

	return &newToken, nil
}

// RevokeRefreshTokenFamily marks all tokens in a family as revoked.
func (r *Repository) RevokeRefreshTokenFamily(ctx context.Context, familyID string) error {
	query := `UPDATE refresh_tokens SET is_revoked = TRUE WHERE family_id = $1;`
	_, err := r.pool.Exec(ctx, query, familyID)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token family: %w", err)
	}
	return nil
}

// RevokeRefreshToken marks a single refresh token as revoked.
func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `UPDATE refresh_tokens SET is_revoked = TRUE WHERE token_hash = $1;`
	_, err := r.pool.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllUserRefreshTokens marks all refresh tokens for a user as revoked.
func (r *Repository) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	query := `UPDATE refresh_tokens SET is_revoked = TRUE WHERE user_id = $1;`
	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all user refresh tokens: %w", err)
	}
	return nil
}

// IsSessionActive reports whether the session (refresh token family) of a user still holds a usable refresh token,
// that is whether it can still renew its access. A revoked or expired session, and one of another user, is not.
func (r *Repository) IsSessionActive(ctx context.Context, userID, familyID string) (bool, error) {
	var active bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM refresh_tokens
			WHERE user_id::text = $1 AND family_id::text = $2 AND NOT is_revoked AND expires_at > NOW()
		);
	`, userID, familyID).Scan(&active)
	if err != nil {
		return false, fmt.Errorf("failed to check the session: %w", err)
	}
	return active, nil
}

// ListSessions returns the signed-in devices of a user: the refresh token families that still hold a usable token,
// most recently used first. Each rotation adds a token to its family, so the family's own bounds are the session's.
func (r *Repository) ListSessions(ctx context.Context, userID string) ([]models.Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT family_id::text,
		       MIN(created_at), MAX(created_at),
		       (ARRAY_AGG(created_ip ORDER BY created_at DESC))[1],
		       (ARRAY_AGG(user_agent ORDER BY created_at DESC))[1]
		FROM refresh_tokens
		WHERE user_id = $1
		GROUP BY family_id
		HAVING BOOL_OR(NOT is_revoked AND expires_at > NOW())
		ORDER BY MAX(created_at) DESC;
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()
	sessions := []models.Session{}
	for rows.Next() {
		var s models.Session
		if err := rows.Scan(&s.ID, &s.StartedAt, &s.LastUsedAt, &s.IP, &s.UserAgent); err != nil {
			return nil, fmt.Errorf("failed to read a session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// RevokeUserSession revokes one of the user's sessions. It reports false when the family is not theirs (or not
// there): a session id never reveals whether it exists for someone else.
func (r *Repository) RevokeUserSession(ctx context.Context, userID, familyID string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET is_revoked = TRUE WHERE user_id = $1 AND family_id::text = $2;`, userID, familyID)
	if err != nil {
		return false, fmt.Errorf("failed to revoke session: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// RevokeUserSessionsExcept revokes every session of the user but keepFamily (all of them when it is empty) and
// returns how many sessions were still active.
func (r *Repository) RevokeUserSessionsExcept(ctx context.Context, userID, keepFamily string) (int, error) {
	var revoked int
	err := r.pool.QueryRow(ctx, `
		WITH active AS (
			SELECT DISTINCT family_id FROM refresh_tokens
			WHERE user_id = $1 AND NOT is_revoked AND expires_at > NOW() AND ($2 = '' OR family_id::text <> $2)
		), done AS (
			UPDATE refresh_tokens SET is_revoked = TRUE
			WHERE user_id = $1 AND family_id IN (SELECT family_id FROM active)
		)
		SELECT COUNT(*) FROM active;
	`, userID, keepFamily).Scan(&revoked)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke sessions: %w", err)
	}
	return revoked, nil
}

// UpdatePasswordHash replaces the local password of a user.
func (r *Repository) UpdatePasswordHash(ctx context.Context, userID, passwordHash string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1;`, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUserLanguage sets the language used for messages built outside a request
// (reminder webhooks, sync failure alerts). language must already be validated ("en" or "fr").
func (r *Repository) UpdateUserLanguage(ctx context.Context, userID, language string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE users SET language = $2, updated_at = NOW() WHERE id = $1;`, userID, language)
	if err != nil {
		return fmt.Errorf("failed to update language: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUserDistanceUnit sets the unit distances are converted to for display and form input.
// unit must already be validated ("km" or "mi"); stored distances stay in km either way.
func (r *Repository) UpdateUserDistanceUnit(ctx context.Context, userID, unit string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE users SET distance_unit = $2, updated_at = NOW() WHERE id = $1;`, userID, unit)
	if err != nil {
		return fmt.Errorf("failed to update distance unit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CleanupExpiredRefreshTokens deletes old expired/revoked refresh tokens older than 7 days.
func (r *Repository) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error) {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW() - INTERVAL '7 days' OR (is_revoked = TRUE AND created_at < NOW() - INTERVAL '7 days');`
	tag, err := r.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to clean up expired refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ============================================================================
// Vehicles
// ============================================================================

const vehicleColumns = `
	id, user_id, name, vin, current_odometer,
	estimated_kwh_100km, estimated_price_per_kwh, currency, powertrain,
	telemetry_mode, make, model,
	default_driver_id, tariff_plan_id, is_home_charger_default,
	created_at, updated_at
`

func scanVehicle(row pgx.Row, v *models.Vehicle) error {
	return row.Scan(
		&v.ID, &v.UserID, &v.Name, &v.Vin, &v.CurrentOdometer,
		&v.EstimatedKwh100km, &v.EstimatedPricePerKwh, &v.Currency, &v.Powertrain,
		&v.TelemetryMode, &v.Make, &v.Model,
		&v.DefaultDriverID, &v.TariffPlanID, &v.IsHomeChargerDefault,
		&v.CreatedAt, &v.UpdatedAt,
	)
}

func sanitizeVehicleForRole(v *models.Vehicle) {
	if v.Role != models.RoleOwner {
		v.TeslaMateAPIURL = nil
		v.TeslaMateAPIKeyEncrypted = nil
		v.TeslaMateBasicUser = nil
		v.TeslaMateBasicPassEnc = nil
	}
}
