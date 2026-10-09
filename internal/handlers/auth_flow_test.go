package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/config"
	"github.com/teslacost/teslacost/internal/database"
)

// authTestRepo returns a repository on a freshly migrated dedicated database. Requires TEST_DATABASE_URL.
func authTestRepo(t *testing.T) *database.Repository {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dedicatedTestDatabase(t, url, "handlers"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	if err := (&database.DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return database.NewRepository(pool)
}

func authTestConfig() *config.Config {
	return &config.Config{
		JWTSecret:                  "test-secret-test-secret-test-secret-00",
		JWTAccessExpirationMinutes: 15,
		JWTRefreshExpirationDays:   30,
	}
}

func postJSON(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.5:4000"
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestLoginAnswersTheSameForUnknownAndWrongPasswordAndThrottlesAccounts(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	hash, _ := auth.HashPassword("Correct-Horse-9")
	if _, err := repo.CreateUser(ctx, "known@example.org", hash); err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(repo, authTestConfig(), nil)

	wrong := postJSON(h.Login, `{"email":"known@example.org","password":"nope-nope-nope"}`)
	unknown := postJSON(h.Login, `{"email":"nobody@example.org","password":"nope-nope-nope"}`)
	if wrong.Code != http.StatusUnauthorized || unknown.Code != http.StatusUnauthorized || wrong.Body.String() != unknown.Body.String() {
		t.Fatalf("a wrong password and an unknown address must be indistinguishable: %d %q / %d %q", wrong.Code, wrong.Body.String(), unknown.Code, unknown.Body.String())
	}

	// The wrong password above was the first failure of this account; nine more close it to attempts, even
	// with the right password. (The unknown address counted for nobody.)
	for i := 0; i < 9; i++ {
		postJSON(h.Login, `{"email":"known@example.org","password":"nope-nope-nope"}`)
	}
	blocked := postJSON(h.Login, `{"email":"KNOWN@example.org","password":"Correct-Horse-9"}`)
	if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") == "" {
		t.Fatalf("expected 429 with Retry-After, got %d %q", blocked.Code, blocked.Header().Get("Retry-After"))
	}
	// Someone else is not affected, and a success clears the counter.
	other := postJSON(h.Login, `{"email":"nobody@example.org","password":"x-x-x-x-x-x"}`)
	if other.Code != http.StatusUnauthorized {
		t.Errorf("another account must not be throttled, got %d", other.Code)
	}
}

func TestSuccessfulLoginResetsFailuresAndKeepsTheRefreshTokenOutOfTheBody(t *testing.T) {
	repo := authTestRepo(t)
	hash, _ := auth.HashPassword("Correct-Horse-9")
	if _, err := repo.CreateUser(context.Background(), "me@example.org", hash); err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(repo, authTestConfig(), nil)
	for i := 0; i < 5; i++ {
		postJSON(h.Login, `{"email":"me@example.org","password":"nope-nope-nope"}`)
	}
	ok := postJSON(h.Login, `{"email":"me@example.org","password":"Correct-Horse-9"}`)
	if ok.Code != http.StatusOK {
		t.Fatalf("login: %d %s", ok.Code, ok.Body.String())
	}
	if strings.Contains(ok.Body.String(), "refresh_token") {
		t.Error("the long-lived refresh token must not be readable by scripts: cookie only")
	}
	var refreshCookie *http.Cookie
	for _, c := range ok.Result().Cookies() {
		if c.Name == refreshTokenCookie {
			refreshCookie = c
		}
	}
	if refreshCookie == nil || !refreshCookie.HttpOnly || refreshCookie.Value == "" {
		t.Fatalf("the refresh token must be set as an HttpOnly cookie: %+v", refreshCookie)
	}
	for i := 0; i < 8; i++ { // would exceed the limit if the five earlier failures were still counted
		postJSON(h.Login, `{"email":"me@example.org","password":"nope-nope-nope"}`)
	}
	if again := postJSON(h.Login, `{"email":"me@example.org","password":"Correct-Horse-9"}`); again.Code != http.StatusOK {
		t.Errorf("failures before a success must not count afterwards, got %d", again.Code)
	}

	// The refresh route also keeps it out of the body.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(refreshCookie)
	rec := httptest.NewRecorder()
	h.RefreshToken(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "refresh_token") {
		t.Errorf("refresh: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterRejectsPasswordsBcryptWouldTruncate(t *testing.T) {
	repo := authTestRepo(t)
	h := NewAuthHandler(repo, authTestConfig(), nil)
	body := `{"email":"long@example.org","password":"` + strings.Repeat("a", 73) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

// ---------------------------------------------------------------------------------------------------------------
// OIDC against a fake identity provider that enforces PKCE and signs ID tokens.
// ---------------------------------------------------------------------------------------------------------------

type fakeIdP struct {
	srv *httptest.Server
	key *rsa.PrivateKey
	mu  sync.Mutex
	// codes maps an authorization code to what the authorization request registered.
	codes map[string]idpGrant
}

type idpGrant struct {
	challenge, nonce, email, sub string
	verified                     *bool
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIdP{key: key, codes: map[string]idpGrant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer": f.srv.URL, "authorization_endpoint": f.srv.URL + "/authorize", "token_endpoint": f.srv.URL + "/token",
			"jwks_uri": f.srv.URL + "/keys", "response_types_supported": []string{"code"},
			"subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k1",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = r.ParseForm()
		f.mu.Lock()
		grant, ok := f.codes[r.Form.Get("code")]
		f.mu.Unlock()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || r.Form.Get("code_verifier") == "" || base64.RawURLEncoding.EncodeToString(sum[:]) != grant.challenge {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "PKCE verification failed"})
			return
		}
		claims := jwt.MapClaims{"iss": f.srv.URL, "aud": "teslacost", "sub": grant.sub, "email": grant.email, "nonce": grant.nonce,
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "name": "Test User"}
		if grant.verified != nil {
			claims["email_verified"] = *grant.verified
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "k1"
		signed, err := tok.SignedString(key)
		if err != nil {
			t.Error(err)
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "id_token": signed})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// authorize plays the browser's visit to the provider: it records what the authorization request asked for and
// returns the code the provider would send back.
func (f *fakeIdP) authorize(t *testing.T, authURL, email string, verified *bool) (code, state string) {
	t.Helper()
	u, err := neturl.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatalf("the authorization request must carry an S256 PKCE challenge: %s", u.RawQuery)
	}
	code = "code-" + email
	f.mu.Lock()
	f.codes[code] = idpGrant{challenge: q.Get("code_challenge"), nonce: q.Get("nonce"), email: email, sub: "sub-" + email, verified: verified}
	f.mu.Unlock()
	return code, q.Get("state")
}

func oidcTestHandler(t *testing.T, repo *database.Repository, idp *fakeIdP, allowed []string) *AuthHandler {
	t.Helper()
	cfg := authTestConfig()
	cfg.OIDCEnabled = true
	cfg.OIDCIssuerURL = idp.srv.URL
	cfg.OIDCClientID = "teslacost"
	cfg.OIDCClientSecret = "secret"
	cfg.OIDCRedirectURL = "http://app.example/api/auth/oidc/callback"
	cfg.OIDCScopes = []string{"openid", "email", "profile"}
	cfg.OIDCAllowedEmails = allowed
	svc, err := auth.NewOIDCService(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return NewAuthHandler(repo, cfg, svc)
}

// oidcRound runs login then callback and returns the callback's response.
func oidcRound(t *testing.T, h *AuthHandler, idp *fakeIdP, email string, verified *bool, tamper func([]*http.Cookie) []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	login := httptest.NewRecorder()
	h.OIDCLogin(login, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil))
	if login.Code != http.StatusFound {
		t.Fatalf("login redirect: %d", login.Code)
	}
	code, state := idp.authorize(t, login.Header().Get("Location"), email, verified)
	cookies := login.Result().Cookies()
	if tamper != nil {
		cookies = tamper(cookies)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code="+code+"&state="+state, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.OIDCCallback(rec, req)
	return rec
}

func TestOIDCFlowUsesPKCEAndRejectsWhatIsNotBoundToTheBrowser(t *testing.T) {
	repo := authTestRepo(t)
	idp := newFakeIdP(t)
	h := oidcTestHandler(t, repo, idp, nil)
	yes := true

	// Happy path: PKCE verified by the provider, session cookies issued.
	rec := oidcRound(t, h, idp, "pkce@example.org", &yes, nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected the redirect to the app, got %d %s", rec.Code, rec.Body.String())
	}
	var hasRefresh bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == refreshTokenCookie && c.Value != "" {
			hasRefresh = true
		}
	}
	if !hasRefresh {
		t.Error("a session must be issued")
	}

	// Without the verifier cookie an intercepted code is useless.
	dropVerifier := func(cs []*http.Cookie) []*http.Cookie {
		var kept []*http.Cookie
		for _, c := range cs {
			if c.Name != oidcVerifierCookie {
				kept = append(kept, c)
			}
		}
		return kept
	}
	if rec := oidcRound(t, h, idp, "nover@example.org", &yes, dropVerifier); rec.Code != http.StatusBadRequest {
		t.Errorf("missing verifier cookie: got %d, want 400", rec.Code)
	}

	// A different verifier fails at the provider, and the error detail stays out of the response.
	wrongVerifier := func(cs []*http.Cookie) []*http.Cookie {
		for _, c := range cs {
			if c.Name == oidcVerifierCookie {
				c.Value = auth.GenerateCodeVerifier()
			}
		}
		return cs
	}
	rec = oidcRound(t, h, idp, "wrong@example.org", &yes, wrongVerifier)
	if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "PKCE") || strings.Contains(rec.Body.String(), "invalid_grant") {
		t.Errorf("wrong verifier: got %d %q", rec.Code, rec.Body.String())
	}
}

func TestOIDCRefusesAnUnverifiedEmailAndNeverLinksALocalAccountByAddress(t *testing.T) {
	repo := authTestRepo(t)
	idp := newFakeIdP(t)
	h := oidcTestHandler(t, repo, idp, nil)
	ctx := context.Background()
	yes, no := true, false

	local, err := repo.CreateUser(ctx, "victim@example.org", "hash")
	if err != nil {
		t.Fatal(err)
	}

	// Someone registers "victim@example.org" at the provider without proving they own it.
	if rec := oidcRound(t, h, idp, "victim@example.org", &no, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("unverified email: got %d, want 403", rec.Code)
	}
	if u, _ := repo.GetUserByEmail(ctx, "victim@example.org"); u.OIDCSubject != nil {
		t.Fatal("the local account must not have been linked to an unverified identity")
	}

	// A verified identity does not take over a local account either: whoever registered it first may have chosen its
	// password, and the address alone does not tell who owns it. The user is sent back to sign in, and nothing is issued.
	rec := oidcRound(t, h, idp, "victim@example.org", &yes, nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login?error=oidc_local_account_exists" {
		t.Fatalf("verified email on a local account: got %d %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, c := range rec.Result().Cookies() {
		if (c.Name == refreshTokenCookie || c.Name == accessTokenCookie) && c.Value != "" {
			t.Fatalf("no session may be issued when the account is not linked: %s", c.Name)
		}
	}
	if u, _ := repo.GetUserByEmail(ctx, "victim@example.org"); u.ID != local.ID || u.OIDCSubject != nil || u.PasswordHash == nil {
		t.Fatalf("the local account must be left as it was: %+v", u)
	}

	// A provider that omits the claim is still accepted (not every provider sends it), for an address nobody holds.
	if rec := oidcRound(t, h, idp, "noclaim@example.org", nil, nil); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/oidc-callback" {
		t.Errorf("email_verified absent: got %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := oidcRound(t, h, idp, "fresh@example.org", &yes, nil); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/oidc-callback" {
		t.Errorf("a new identity creates its account: got %d", rec.Code)
	}
}

func TestOIDCOnlyInstanceLinksALocalAccountByAddressOnce(t *testing.T) {
	repo := authTestRepo(t)
	idp := newFakeIdP(t)
	h := oidcTestHandler(t, repo, idp, nil)
	h.cfg.OIDCDisableLocalAuth = true // local sign-in is off: the accounts hold no usable password
	ctx := context.Background()
	yes := true

	local, _ := repo.CreateUser(ctx, "admin@example.org", "hash")
	if rec := oidcRound(t, h, idp, "admin@example.org", &yes, nil); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/oidc-callback" {
		t.Fatalf("verified email: got %d", rec.Code)
	}
	linked, _ := repo.GetUserByEmail(ctx, "admin@example.org")
	if linked.ID != local.ID || linked.OIDCSubject == nil || *linked.OIDCSubject != "sub-admin@example.org" {
		t.Fatalf("expected the local account to be linked, got %+v", linked)
	}

	// Another identity claiming the same address cannot take the account over: it is already linked.
	other := newFakeIdP(t)
	h2 := oidcTestHandler(t, repo, other, nil)
	h2.cfg.OIDCDisableLocalAuth = true
	if rec := oidcRound(t, h2, other, "admin@example.org", &yes, nil); rec.Header().Get("Location") == "/oidc-callback" {
		t.Error("an account already tied to an identity must not be re-pointed to another one")
	}
	if again, _ := repo.GetUserByEmail(ctx, "admin@example.org"); again.OIDCSubject == nil || *again.OIDCSubject != "sub-admin@example.org" {
		t.Errorf("the link changed: %+v", again.OIDCSubject)
	}
}

func TestOIDCAllowedEmailsIgnoreCase(t *testing.T) {
	repo := authTestRepo(t)
	idp := newFakeIdP(t)
	h := oidcTestHandler(t, repo, idp, []string{"Me@Example.org"})
	yes := true
	if rec := oidcRound(t, h, idp, "me@example.org", &yes, nil); rec.Code != http.StatusFound {
		t.Errorf("allowed address with another case: got %d", rec.Code)
	}
	if rec := oidcRound(t, h, idp, "stranger@example.org", &yes, nil); rec.Code != http.StatusForbidden {
		t.Errorf("address outside the list: got %d, want 403", rec.Code)
	}
}
