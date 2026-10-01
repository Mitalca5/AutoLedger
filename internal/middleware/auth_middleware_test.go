package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teslacost/teslacost/internal/auth"
)

func TestAuthenticateJWTAcceptsBearerHeader(t *testing.T) {
	secret := "test-secret"
	token, err := auth.GenerateAccessToken("user-1", "user@example.com", secret, 15)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var gotUserID string
	handler := AuthenticateJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotUserID != "user-1" {
		t.Errorf("expected user-1, got %q", gotUserID)
	}
}

func TestAuthenticateJWTFallsBackToAccessTokenCookie(t *testing.T) {
	secret := "test-secret"
	token, err := auth.GenerateAccessToken("user-2", "cookie@example.com", secret, 15)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var gotUserID string
	handler := AuthenticateJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: accessTokenCookieName, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotUserID != "user-2" {
		t.Errorf("expected user-2, got %q", gotUserID)
	}
}

func TestAuthenticateJWTPrefersHeaderOverCookie(t *testing.T) {
	secret := "test-secret"
	headerToken, _ := auth.GenerateAccessToken("header-user", "h@example.com", secret, 15)
	cookieToken, _ := auth.GenerateAccessToken("cookie-user", "c@example.com", secret, 15)

	var gotUserID string
	handler := AuthenticateJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+headerToken)
	req.AddCookie(&http.Cookie{Name: accessTokenCookieName, Value: cookieToken})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotUserID != "header-user" {
		t.Errorf("expected header token to take precedence, got %q", gotUserID)
	}
}

func TestAuthenticateJWTRejectsMissingCredentials(t *testing.T) {
	handler := AuthenticateJWT("test-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without credentials")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

type fakeTokenValidator map[string]string // token hash -> user ID

func (f fakeTokenValidator) ValidateAPIToken(_ context.Context, tokenHash string) (string, string, error) {
	if userID, ok := f[tokenHash]; ok {
		return userID, userID + "@example.com", nil
	}
	return "", "", errors.New("unknown token")
}

func TestAPITokensOnlyOpenIntegrationRoutes(t *testing.T) {
	secret := "test-secret"
	apiToken := auth.TokenPrefix + "0123456789abcdef0123456789abcdef0123456789abcdef"
	validator := fakeTokenValidator{auth.HashAPIToken(apiToken): "user-1"}
	session, err := auth.GenerateAccessToken("user-2", "user2@example.com", secret, 15)
	if err != nil {
		t.Fatal(err)
	}

	call := func(mw func(http.Handler) http.Handler, bearer string) (int, string) {
		var gotUserID string
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUserID = GetUserID(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Authorization", "Bearer "+bearer)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code, gotUserID
	}

	if code, _ := call(AuthenticateJWT(secret), apiToken); code != http.StatusUnauthorized {
		t.Errorf("session routes: a valid API token got %d, want 401", code)
	}
	if code, user := call(AuthenticateJWT(secret), session); code != http.StatusOK || user != "user-2" {
		t.Errorf("session routes: a session got %d for %q, want 200 for user-2", code, user)
	}
	if code, user := call(AuthenticateIntegration(secret, validator), apiToken); code != http.StatusOK || user != "user-1" {
		t.Errorf("integration routes: a valid API token got %d for %q, want 200 for user-1", code, user)
	}
	if code, _ := call(AuthenticateIntegration(secret, validator), auth.TokenPrefix+"revoked"); code != http.StatusUnauthorized {
		t.Errorf("integration routes: an unknown API token got %d, want 401", code)
	}
	if code, user := call(AuthenticateIntegration(secret, validator), session); code != http.StatusOK || user != "user-2" {
		t.Errorf("integration routes: a session got %d for %q, want 200 for user-2", code, user)
	}
}
