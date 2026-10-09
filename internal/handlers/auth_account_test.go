package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/middleware"
)

type device struct {
	refresh *http.Cookie
}

// signIn logs the user in as a new device and returns the refresh cookie it received.
func signIn(t *testing.T, h *AuthHandler, email, password, userAgent, ip string) device {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"`+email+`","password":"`+password+`"}`))
	req.Header.Set("User-Agent", userAgent)
	req.RemoteAddr = ip + ":5000"
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login as %s: %d %s", email, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == refreshTokenCookie {
			return device{refresh: c}
		}
	}
	t.Fatal("no refresh cookie")
	return device{}
}

// as runs a handler for an authenticated user, with the device's refresh cookie when there is one.
func as(userID string, d *device, method, target, body string, handler http.HandlerFunc, params map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	if len(params) > 0 {
		rc := chi.NewRouteContext()
		for k, v := range params {
			rc.URLParams.Add(k, v)
		}
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rc)
	}
	req = req.WithContext(ctx)
	if d != nil {
		req.AddCookie(d.refresh)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func refreshWith(h *AuthHandler, d device) int {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(d.refresh)
	rec := httptest.NewRecorder()
	h.RefreshToken(rec, req)
	return rec.Code
}

func listSessions(t *testing.T, h *AuthHandler, userID string, d *device) []map[string]any {
	t.Helper()
	rec := as(userID, d, http.MethodGet, "/api/auth/sessions", "", h.ListSessions, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSessionsListRevokeAndIsolation(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	hash, _ := auth.HashPassword("Correct-Horse-9")
	alice, _ := repo.CreateUser(ctx, "alice@example.org", hash)
	bob, _ := repo.CreateUser(ctx, "bob@example.org", hash)
	h := NewAuthHandler(repo, authTestConfig(), nil)

	laptop := signIn(t, h, "alice@example.org", "Correct-Horse-9", "Mozilla/5.0 (X11; Linux x86_64) Chrome/126.0 Safari/537.36", "198.51.100.1")
	phone := signIn(t, h, "alice@example.org", "Correct-Horse-9", "Mozilla/5.0 (Linux; Android 14) Chrome/126.0 Mobile Safari/537.36", "198.51.100.2")
	bobPhone := signIn(t, h, "bob@example.org", "Correct-Horse-9", "Bob-Browser", "203.0.113.9")

	sessions := listSessions(t, h, alice.ID, &laptop)
	if len(sessions) != 2 {
		t.Fatalf("expected the two devices, got %v", sessions)
	}
	currents, ips := 0, map[string]bool{}
	for _, s := range sessions {
		if s["current"] == true {
			currents++
		}
		if ip, _ := s["ip"].(string); ip != "" {
			ips[ip] = true
		}
		if strings.Contains(toJSON(s), "token") {
			t.Errorf("a session must not expose any token: %v", s)
		}
	}
	if currents != 1 || !ips["198.51.100.1"] || !ips["198.51.100.2"] {
		t.Errorf("one current session and the client addresses expected: %v", sessions)
	}
	if bobSessions := listSessions(t, h, bob.ID, &bobPhone); len(bobSessions) != 1 {
		t.Errorf("each user only sees their own sessions: %v", bobSessions)
	}

	// Bob cannot close Alice's session, and cannot tell it exists
	aliceID := sessions[0]["id"].(string)
	if rec := as(bob.ID, &bobPhone, http.MethodDelete, "/api/auth/sessions/"+aliceID, "", h.RevokeSession, map[string]string{"sessionId": aliceID}); rec.Code != http.StatusNotFound {
		t.Errorf("revoking another user's session: got %d, want 404", rec.Code)
	}
	if refreshWith(h, laptop) != http.StatusOK || refreshWith(h, phone) != http.StatusOK {
		t.Fatal("Alice's sessions must be untouched")
	}
	// (each refresh rotated the tokens: use the new cookies from now on)
	laptop = signIn(t, h, "alice@example.org", "Correct-Horse-9", "Laptop-again", "198.51.100.1")
	phone = signIn(t, h, "alice@example.org", "Correct-Horse-9", "Phone-again", "198.51.100.2")

	// Alice closes one of her sessions: it can no longer refresh, the other still can.
	sessions = listSessions(t, h, alice.ID, &phone)
	var phoneID string
	for _, s := range sessions {
		if s["current"] == true {
			phoneID = s["id"].(string)
		}
	}
	rec := as(alice.ID, &laptop, http.MethodDelete, "/api/auth/sessions/"+phoneID, "", h.RevokeSession, map[string]string{"sessionId": phoneID})
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	if refreshWith(h, phone) != http.StatusUnauthorized {
		t.Error("a revoked session must not be able to refresh")
	}
	if refreshWith(h, laptop) != http.StatusOK {
		t.Error("the other session keeps working")
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestClosingTheCurrentSessionClearsItsCookies(t *testing.T) {
	repo := authTestRepo(t)
	hash, _ := auth.HashPassword("Correct-Horse-9")
	user, _ := repo.CreateUser(context.Background(), "solo@example.org", hash)
	h := NewAuthHandler(repo, authTestConfig(), nil)
	d := signIn(t, h, "solo@example.org", "Correct-Horse-9", "UA", "198.51.100.1")

	id := listSessions(t, h, user.ID, &d)[0]["id"].(string)
	rec := as(user.ID, &d, http.MethodDelete, "/api/auth/sessions/"+id, "", h.RevokeSession, map[string]string{"sessionId": id})
	cleared := 0
	for _, c := range rec.Result().Cookies() {
		if (c.Name == refreshTokenCookie || c.Name == accessTokenCookie) && c.MaxAge < 0 {
			cleared++
		}
	}
	if rec.Code != http.StatusOK || cleared != 2 {
		t.Errorf("closing the current session must clear both cookies: %d, %d cleared", rec.Code, cleared)
	}
}

func TestLogoutAllClosesEverySession(t *testing.T) {
	repo := authTestRepo(t)
	hash, _ := auth.HashPassword("Correct-Horse-9")
	user, _ := repo.CreateUser(context.Background(), "many@example.org", hash)
	h := NewAuthHandler(repo, authTestConfig(), nil)
	a := signIn(t, h, "many@example.org", "Correct-Horse-9", "A", "198.51.100.1")
	b := signIn(t, h, "many@example.org", "Correct-Horse-9", "B", "198.51.100.2")

	rec := as(user.ID, &a, http.MethodPost, "/api/auth/logout-all", "", h.LogoutAll, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"sessions_revoked":2`) {
		t.Fatalf("logout-all: %d %s", rec.Code, rec.Body.String())
	}
	if refreshWith(h, a) != http.StatusUnauthorized || refreshWith(h, b) != http.StatusUnauthorized {
		t.Error("no session may survive")
	}
	if got := listSessions(t, h, user.ID, nil); len(got) != 0 {
		t.Errorf("no session left to list: %v", got)
	}
}

func TestChangePasswordChecksTheCurrentOneAndClosesOtherDevices(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	hash, _ := auth.HashPassword("Correct-Horse-9")
	user, _ := repo.CreateUser(ctx, "pw@example.org", hash)
	h := NewAuthHandler(repo, authTestConfig(), nil)
	mine := signIn(t, h, "pw@example.org", "Correct-Horse-9", "Mine", "198.51.100.1")
	stolen := signIn(t, h, "pw@example.org", "Correct-Horse-9", "Thief", "203.0.113.66")

	change := func(current, next string) *httptest.ResponseRecorder {
		return as(user.ID, &mine, http.MethodPost, "/api/auth/password", `{"current_password":"`+current+`","new_password":"`+next+`"}`, h.ChangePassword, nil)
	}

	if rec := change("wrong-wrong-1", "New-Password-42"); rec.Code != http.StatusForbidden {
		t.Errorf("wrong current password: got %d, want 403 (401 would look like an expired session)", rec.Code)
	}
	for name, next := range map[string]string{"too short": "short", "too long": strings.Repeat("a", 73), "unchanged": "Correct-Horse-9"} {
		if rec := change("Correct-Horse-9", next); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", name, rec.Code)
		}
	}
	if got := listSessions(t, h, user.ID, &mine); len(got) != 2 {
		t.Fatalf("both devices are signed in before the change: %v", got)
	}

	rec := change("Correct-Horse-9", "New-Password-42")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"sessions_revoked":1`) {
		t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
	}
	if refreshWith(h, stolen) != http.StatusUnauthorized {
		t.Error("the other device must be signed out")
	}
	if refreshWith(h, mine) != http.StatusOK {
		t.Error("the device that changed the password stays signed in")
	}
	login := func(pw string) int {
		return postJSON(h.Login, `{"email":"pw@example.org","password":"`+pw+`"}`).Code
	}
	if login("Correct-Horse-9") != http.StatusUnauthorized || login("New-Password-42") != http.StatusOK {
		t.Error("only the new password signs in")
	}
}

func TestChangePasswordSharesTheSignInThrottleAndRefusesSSOOnlyAccounts(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	hash, _ := auth.HashPassword("Correct-Horse-9")
	user, _ := repo.CreateUser(ctx, "guess@example.org", hash)
	h := NewAuthHandler(repo, authTestConfig(), nil)

	var last *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		last = as(user.ID, nil, http.MethodPost, "/api/auth/password", `{"current_password":"nope-nope-1","new_password":"New-Password-42"}`, h.ChangePassword, nil)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Errorf("a stolen session must not be able to guess the password without limit: got %d", last.Code)
	}

	sso, err := repo.UpsertOIDCUser(ctx, "sso@example.org", "sub-1", "https://idp.example", "SSO User", false)
	if err != nil {
		t.Fatal(err)
	}
	rec := as(sso.ID, nil, http.MethodPost, "/api/auth/password", `{"current_password":"x","new_password":"New-Password-42"}`, h.ChangePassword, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an account without a local password: got %d, want 400", rec.Code)
	}
}

func TestUpdateLanguageStoresAValidChoiceAndRejectsAnythingElse(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	hash, _ := auth.HashPassword("Correct-Horse-9")
	user, _ := repo.CreateUser(ctx, "lang@example.org", hash)
	if user.Language != "en" {
		t.Fatalf("expected the default language to be en, got %q", user.Language)
	}
	h := NewAuthHandler(repo, authTestConfig(), nil)

	rec := as(user.ID, nil, http.MethodPut, "/api/auth/language", `{"language":"fr"}`, h.UpdateLanguage, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("update to fr: %d %s", rec.Code, rec.Body.String())
	}
	got, err := repo.GetUserByID(ctx, user.ID)
	if err != nil || got.Language != "fr" {
		t.Fatalf("expected the stored language to be fr, got %q (err=%v)", got.Language, err)
	}

	rec = as(user.ID, nil, http.MethodPut, "/api/auth/language", `{"language":"de"}`, h.UpdateLanguage, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an unsupported language: got %d, want 400", rec.Code)
	}
	if got, _ := repo.GetUserByID(ctx, user.ID); got.Language != "fr" {
		t.Errorf("a rejected update must not change the stored language, got %q", got.Language)
	}
}

func TestUpdateDistanceUnitStoresAValidChoiceAndRejectsAnythingElse(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	hash, _ := auth.HashPassword("Correct-Horse-9")
	user, _ := repo.CreateUser(ctx, "units@example.org", hash)
	if user.DistanceUnit != "km" {
		t.Fatalf("expected the default distance unit to be km, got %q", user.DistanceUnit)
	}
	h := NewAuthHandler(repo, authTestConfig(), nil)

	rec := as(user.ID, nil, http.MethodPut, "/api/auth/distance-unit", `{"distance_unit":"mi"}`, h.UpdateDistanceUnit, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("update to mi: %d %s", rec.Code, rec.Body.String())
	}
	got, err := repo.GetUserByID(ctx, user.ID)
	if err != nil || got.DistanceUnit != "mi" {
		t.Fatalf("expected the stored distance unit to be mi, got %q (err=%v)", got.DistanceUnit, err)
	}

	rec = as(user.ID, nil, http.MethodPut, "/api/auth/distance-unit", `{"distance_unit":"furlong"}`, h.UpdateDistanceUnit, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an unsupported unit: got %d, want 400", rec.Code)
	}
	if got, _ := repo.GetUserByID(ctx, user.ID); got.DistanceUnit != "mi" {
		t.Errorf("a rejected update must not change the stored unit, got %q", got.DistanceUnit)
	}
}

func TestListSessionsHidesExpiredAndRevokedTokens(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	user, _ := repo.CreateUser(ctx, "old@example.org", "hash")
	ip, ua := "198.51.100.1", "UA"
	// An expired session and a fully revoked one
	if _, err := repo.CreateRefreshToken(ctx, user.ID, "hash-expired", "11111111-1111-4111-8111-111111111111", timeAgo(1), &ip, &ua); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRefreshToken(ctx, user.ID, "hash-revoked", "22222222-2222-4222-8222-222222222222", timeAhead(24), &ip, &ua); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeRefreshTokenFamily(ctx, "22222222-2222-4222-8222-222222222222"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRefreshToken(ctx, user.ID, "hash-live", "33333333-3333-4333-8333-333333333333", timeAhead(24), &ip, &ua); err != nil {
		t.Fatal(err)
	}
	sessions, err := repo.ListSessions(ctx, user.ID)
	if err != nil || len(sessions) != 1 || sessions[0].ID != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("only the live session is listed, got %+v (%v)", sessions, err)
	}
}

func timeAgo(hours int) time.Time   { return time.Now().Add(-time.Duration(hours) * time.Hour) }
func timeAhead(hours int) time.Time { return time.Now().Add(time.Duration(hours) * time.Hour) }
