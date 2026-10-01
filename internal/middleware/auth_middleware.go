package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/teslacost/teslacost/internal/auth"
)

type contextKey string

const (
	UserIDKey    contextKey = "userID"
	UserEmailKey contextKey = "userEmail"
)

// accessTokenCookieName mirrors the constant of the same name in the handlers package
// (an internal/middleware -> internal/handlers import would be circular).
const accessTokenCookieName = "teslacost_access_token"

// extractBearerToken reads the JWT from the Authorization header (API/programmatic clients)
// or, failing that, from the HttpOnly access-token cookie set by the browser-facing SPA.
func extractBearerToken(r *http.Request) (string, error) {
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return "", fmt.Errorf("invalid Authorization format, expected 'Bearer <token>'")
		}
		return parts[1], nil
	}

	if cookie, err := r.Cookie(accessTokenCookieName); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	return "", fmt.Errorf("missing Authorization header or %s cookie", accessTokenCookieName)
}

type APITokenValidator interface {
	ValidateAPIToken(ctx context.Context, tokenHash string) (string, string, error)
}

// AuthenticateJWT returns a middleware that accepts a session access token only. An API token
// (auth.TokenPrefix) is refused here: it only opens the integration routes (AuthenticateIntegration),
// so a token copied into a home-automation setup cannot read the account, manage sessions or mint tokens.
func AuthenticateJWT(jwtSecret string) func(http.Handler) http.Handler {
	return authenticate(jwtSecret, nil)
}

// AuthenticateIntegration returns a middleware for the integration routes (/api/integrations/**):
// it accepts an API token, checked by validator, as well as a session access token.
func AuthenticateIntegration(jwtSecret string, validator APITokenValidator) func(http.Handler) http.Handler {
	return authenticate(jwtSecret, validator)
}

// authenticate validates the bearer token; API tokens are accepted only when validator is set.
func authenticate(jwtSecret string, tokenValidator APITokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, err := extractBearerToken(r)
			if err != nil {
				sendJSONError(w, http.StatusUnauthorized, err.Error())
				return
			}

			if auth.IsAPIToken(tokenString) {
				if tokenValidator == nil {
					sendJSONError(w, http.StatusUnauthorized, "API tokens are only accepted on integration endpoints")
					return
				}
				tokenHash := auth.HashAPIToken(tokenString)
				userID, email, err := tokenValidator.ValidateAPIToken(r.Context(), tokenHash)
				if err != nil {
					sendJSONError(w, http.StatusUnauthorized, "Invalid or expired API token")
					return
				}
				ctx := context.WithValue(r.Context(), UserIDKey, userID)
				ctx = context.WithValue(ctx, UserEmailKey, email)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			claims, err := auth.ValidateToken(tokenString, jwtSecret)
			if err != nil {
				sendJSONError(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the authenticated user ID from context.
func GetUserID(ctx context.Context) string {
	if val, ok := ctx.Value(UserIDKey).(string); ok {
		return val
	}
	return ""
}

// GetUserEmail retrieves the authenticated user email from context.
func GetUserEmail(ctx context.Context) string {
	if val, ok := ctx.Value(UserEmailKey).(string); ok {
		return val
	}
	return ""
}

func sendJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
