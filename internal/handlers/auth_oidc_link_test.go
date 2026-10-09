package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/middleware"
)

// linkRound signs userID in, asks to link the identity of the provider, plays the provider's side as providerEmail and
// returns the callback's response. tamper changes the link cookie before it comes back.
func linkRound(t *testing.T, h *AuthHandler, idp *fakeIdP, userID, providerEmail string, tamper func(*http.Cookie)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/link", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	start := httptest.NewRecorder()
	h.OIDCLink(start, req)
	if start.Code != http.StatusFound || !strings.HasPrefix(start.Header().Get("Location"), idp.srv.URL) {
		t.Fatalf("the link must send the user to the provider: %d %q", start.Code, start.Header().Get("Location"))
	}
	yes := true
	code, state := idp.authorize(t, start.Header().Get("Location"), providerEmail, &yes)
	callback := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code="+code+"&state="+state, nil)
	var sawLink bool
	for _, c := range start.Result().Cookies() {
		if c.Name == oidcLinkCookie {
			sawLink = true
			if tamper != nil {
				tamper(c)
			}
		}
		callback.AddCookie(c)
	}
	if !sawLink {
		t.Fatal("the link request must set its proof cookie")
	}
	rec := httptest.NewRecorder()
	h.OIDCCallback(rec, callback)
	return rec
}

func issuedASession(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if (c.Name == refreshTokenCookie || c.Name == accessTokenCookie) && c.Value != "" {
			return true
		}
	}
	return false
}

// The report: a local account registered with someone else's address, then the owner signs in through SSO.
func TestPreRegisteredAddressIsNotTakenOverByTheOwnersSSO(t *testing.T) {
	repo := authTestRepo(t)
	idp := newFakeIdP(t)
	h := oidcTestHandler(t, repo, idp, nil)
	ctx := context.Background()
	yes := true

	attackerHash, _ := auth.HashPassword("Attacker-Chosen-9")
	squatted, err := repo.CreateUser(ctx, "owner@example.org", attackerHash)
	if err != nil {
		t.Fatal(err)
	}

	rec := oidcRound(t, h, idp, "owner@example.org", &yes, nil)
	if rec.Header().Get("Location") != "/login?error=oidc_local_account_exists" || issuedASession(rec) {
		t.Fatalf("the owner's SSO must not be signed in to the account the attacker created: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if u, _ := repo.GetUserByEmail(ctx, "owner@example.org"); u.ID != squatted.ID || u.OIDCSubject != nil {
		t.Fatalf("the account must stay unlinked, so the owner's data never lands in it: %+v", u)
	}
}

func TestSignedInUserLinksTheirSSOIdentityExplicitly(t *testing.T) {
	repo := authTestRepo(t)
	idp := newFakeIdP(t)
	h := oidcTestHandler(t, repo, idp, nil)
	ctx := context.Background()
	hash, _ := auth.HashPassword("Correct-Horse-9")
	user, _ := repo.CreateUser(ctx, "me@example.org", hash)

	rec := linkRound(t, h, idp, user.ID, "me@example.org", nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/account?sso=linked" {
		t.Fatalf("link: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if issuedASession(rec) {
		t.Error("linking keeps the current session: nobody is signed in by it")
	}
	linked, _ := repo.GetUserByEmail(ctx, "me@example.org")
	if linked.ID != user.ID || linked.OIDCSubject == nil || *linked.OIDCSubject != "sub-me@example.org" || linked.PasswordHash == nil {
		t.Fatalf("the account is linked and keeps its password: %+v", linked)
	}

	// From now on the SSO identity signs in to that account.
	yes := true
	signIn := oidcRound(t, h, idp, "me@example.org", &yes, nil)
	if signIn.Header().Get("Location") != "/oidc-callback" || !issuedASession(signIn) {
		t.Fatalf("SSO sign-in after linking: %d %q", signIn.Code, signIn.Header().Get("Location"))
	}

	// Asking again says it is done and does not go to the provider.
	again := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/link", nil)
	again = again.WithContext(context.WithValue(again.Context(), middleware.UserIDKey, user.ID))
	done := httptest.NewRecorder()
	h.OIDCLink(done, again)
	if done.Header().Get("Location") != "/account?sso=already_linked" {
		t.Errorf("already linked: %q", done.Header().Get("Location"))
	}
}

func TestLinkRefusesAnIdentityThatIsNotTheAccountsAndAProofThatIsNotValid(t *testing.T) {
	repo := authTestRepo(t)
	idp := newFakeIdP(t)
	h := oidcTestHandler(t, repo, idp, nil)
	ctx := context.Background()
	user, _ := repo.CreateUser(ctx, "mine@example.org", "hash")

	for name, c := range map[string]struct {
		providerEmail string
		tamper        func(*http.Cookie)
		want          string
	}{
		"an identity with another address":     {"someone.else@example.org", nil, "/account?sso=email_mismatch"},
		"an address that differs by case only": {"MINE@example.org", nil, "/account?sso=linked"},
		"a proof that expired": {"mine@example.org", func(c *http.Cookie) {
			c.Value = auth.SignOIDCLinkIntent(user.ID, authTestConfig().JWTSecret, time.Now().Add(-time.Minute))
		}, "/account?sso=failed"},
		"a proof signed with another secret": {"mine@example.org", func(c *http.Cookie) {
			c.Value = auth.SignOIDCLinkIntent(user.ID, "another-secret-another-secret-0000000", time.Now().Add(time.Hour))
		}, "/account?sso=failed"},
		"a proof that is not one": {"mine@example.org", func(c *http.Cookie) { c.Value = "garbage" }, "/account?sso=failed"},
	} {
		// every case starts from a free account
		if _, err := repo.Pool().Exec(ctx, `UPDATE users SET oidc_subject = NULL, oidc_provider = NULL WHERE id = $1`, user.ID); err != nil {
			t.Fatal(err)
		}
		rec := linkRound(t, h, idp, user.ID, c.providerEmail, c.tamper)
		if rec.Header().Get("Location") != c.want {
			t.Errorf("%s: got %q, want %q", name, rec.Header().Get("Location"), c.want)
		}
		if issuedASession(rec) {
			t.Errorf("%s: a link must never sign anyone in", name)
		}
		got, _ := repo.GetUserByEmail(ctx, "mine@example.org")
		if (c.want == "/account?sso=linked") != (got.OIDCSubject != nil) {
			t.Errorf("%s: linked = %v", name, got.OIDCSubject != nil)
		}
	}
}

func TestLinkNeedsAnSSOConfiguredInstance(t *testing.T) {
	repo := authTestRepo(t)
	h := NewAuthHandler(repo, authTestConfig(), nil)
	rec := httptest.NewRecorder()
	h.OIDCLink(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/link", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
}
