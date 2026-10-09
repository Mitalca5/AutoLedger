package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
)

// accountRouter mounts the account routes behind the same chain as cmd/server: a session access token, and for the
// routes that create a credential an active session.
func accountRouter(repo *database.Repository, h *AuthHandler) chi.Router {
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthenticateJWT(authTestConfig().JWTSecret))
		r.Get("/api/auth/me", h.Me)
		r.Post("/api/auth/logout-all", h.LogoutAll)
		r.Delete("/api/auth/sessions/{sessionId}", h.RevokeSession)
		r.Get("/api/auth/sessions", h.ListSessions)
		r.With(middleware.RequireActiveSession(repo)).Post("/api/auth/tokens", NewTokenHandler(repo).CreateToken)
		r.With(middleware.RequireActiveSession(repo)).Post("/api/auth/password", h.ChangePassword)
	})
	return r
}

func callWithBearer(r chi.Router, method, path, bearer, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.RemoteAddr = "198.51.100.9:4000"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// signInToken logs in and returns the access token the response carries, with the refresh cookie.
func signInToken(t *testing.T, h *AuthHandler, email, password string) (string, device) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"`+email+`","password":"`+password+`"}`))
	req.RemoteAddr = "198.51.100.8:5000"
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	var body struct {
		Token string `json:"token"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Token == "" {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == refreshTokenCookie {
			return body.Token, device{refresh: c}
		}
	}
	t.Fatal("no refresh cookie")
	return "", device{}
}

func TestRevokedSessionCannotMintCredentials(t *testing.T) {
	repo := authTestRepo(t)
	hash, _ := auth.HashPassword("Correct-Horse-9")
	user, _ := repo.CreateUser(context.Background(), "mint@example.org", hash)
	h := NewAuthHandler(repo, authTestConfig(), nil)
	r := accountRouter(repo, h)
	mint := func(jwt string) int {
		return callWithBearer(r, http.MethodPost, "/api/auth/tokens", jwt, `{"name":"Audit credential"}`).Code
	}

	stolen, device1 := signInToken(t, h, "mint@example.org", "Correct-Horse-9")
	if got := mint(stolen); got != http.StatusCreated {
		t.Fatalf("an active session creates a token: %d", got)
	}

	// "Log out all devices": the refresh sessions are revoked, the access token it already issued is not expired yet.
	if rec := callWithBearer(r, http.MethodPost, "/api/auth/logout-all", stolen, ""); rec.Code != http.StatusOK {
		t.Fatalf("logout-all: %d %s", rec.Code, rec.Body.String())
	}
	if refreshWith(h, device1) != http.StatusUnauthorized {
		t.Fatal("the refresh session must be gone")
	}
	if got := mint(stolen); got != http.StatusUnauthorized {
		t.Fatalf("a revoked session must not mint an API token any more: %d", got)
	}
	if rec := callWithBearer(r, http.MethodPost, "/api/auth/password", stolen, `{"current_password":"Correct-Horse-9","new_password":"Another-Horse-99"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("nor change the password: %d", rec.Code)
	}
	tokens, _ := repo.ListAPITokens(context.Background(), user.ID)
	if len(tokens) != 1 {
		t.Fatalf("only the token created before the revocation exists, got %d", len(tokens))
	}

	// A new sign-in works again.
	fresh, _ := signInToken(t, h, "mint@example.org", "Correct-Horse-9")
	if got := mint(fresh); got != http.StatusCreated {
		t.Fatalf("a new session creates a token: %d", got)
	}
}

func TestRevokingOneSessionOnlyEndsThatSession(t *testing.T) {
	repo := authTestRepo(t)
	hash, _ := auth.HashPassword("Correct-Horse-9")
	repo.CreateUser(context.Background(), "two@example.org", hash)
	h := NewAuthHandler(repo, authTestConfig(), nil)
	r := accountRouter(repo, h)

	first, _ := signInToken(t, h, "two@example.org", "Correct-Horse-9")
	second, _ := signInToken(t, h, "two@example.org", "Correct-Horse-9")
	claims, err := auth.ValidateToken(first, authTestConfig().JWTSecret)
	if err != nil || claims.SessionID == "" {
		t.Fatalf("an access token is bound to its session: %+v, %v", claims, err)
	}

	if rec := callWithBearer(r, http.MethodDelete, "/api/auth/sessions/"+claims.SessionID, second, ""); rec.Code != http.StatusOK {
		t.Fatalf("revoke the first session from the second: %d %s", rec.Code, rec.Body.String())
	}
	body := `{"name":"x"}`
	if rec := callWithBearer(r, http.MethodPost, "/api/auth/tokens", first, body); rec.Code != http.StatusUnauthorized {
		t.Errorf("the revoked session must not mint: %d", rec.Code)
	}
	if rec := callWithBearer(r, http.MethodPost, "/api/auth/tokens", second, body); rec.Code != http.StatusCreated {
		t.Errorf("the other session is untouched: %d", rec.Code)
	}
	if rec := callWithBearer(r, http.MethodGet, "/api/auth/me", first, ""); rec.Code != http.StatusOK {
		t.Errorf("reading with a still valid access token keeps working until it expires: %d", rec.Code)
	}
}

func TestATokenThatNamesNoSessionReadsButIsAskedToRenewBeforeMinting(t *testing.T) {
	repo := authTestRepo(t)
	user, _ := repo.CreateUser(context.Background(), "legacy@example.org", "hash")
	h := NewAuthHandler(repo, authTestConfig(), nil)
	r := accountRouter(repo, h)
	legacy, _ := auth.GenerateAccessToken(user.ID, user.Email, authTestConfig().JWTSecret, 15)

	if rec := callWithBearer(r, http.MethodGet, "/api/auth/me", legacy, ""); rec.Code != http.StatusOK {
		t.Fatalf("a token issued before sessions were named keeps reading: %d", rec.Code)
	}
	if rec := callWithBearer(r, http.MethodPost, "/api/auth/tokens", legacy, `{"name":"x"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("it must be renewed before it creates a credential: %d", rec.Code)
	}
}

func TestARefreshedAccessTokenIsBoundToTheSameSession(t *testing.T) {
	repo := authTestRepo(t)
	hash, _ := auth.HashPassword("Correct-Horse-9")
	repo.CreateUser(context.Background(), "renew@example.org", hash)
	h := NewAuthHandler(repo, authTestConfig(), nil)
	r := accountRouter(repo, h)
	first, d := signInToken(t, h, "renew@example.org", "Correct-Horse-9")
	firstClaims, _ := auth.ValidateToken(first, authTestConfig().JWTSecret)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(d.refresh)
	rec := httptest.NewRecorder()
	h.RefreshToken(rec, req)
	var body struct {
		Token string `json:"token"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
	}
	renewed, err := auth.ValidateToken(body.Token, authTestConfig().JWTSecret)
	if err != nil || renewed.SessionID != firstClaims.SessionID {
		t.Fatalf("a refresh keeps the session: %+v vs %+v (%v)", renewed, firstClaims, err)
	}
	if rec := callWithBearer(r, http.MethodPost, "/api/auth/tokens", body.Token, `{"name":"x"}`); rec.Code != http.StatusCreated {
		t.Fatalf("the renewed token creates a token: %d", rec.Code)
	}
}
