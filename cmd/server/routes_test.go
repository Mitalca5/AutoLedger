package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/auth"
)

const testJWTSecret = "route-table-test-secret-0123456789abcdef"

type fakeTokenValidator struct{}

func (fakeTokenValidator) ValidateAPIToken(_ context.Context, _ string) (string, string, error) {
	return "user-1", "user@example.com", nil
}

var routeParam = regexp.MustCompile(`\{[^}]+\}`)

// The messages the authentication middleware answers with; any other response came from the route's handler.
const (
	noCredentialsMessage = "missing Authorization header"
	sessionOnlyMessage   = "only accepted on integration endpoints"
	badSessionMessage    = "Invalid or expired token"
)

func newRouteTable(t *testing.T) chi.Router {
	t.Helper()
	r := chi.NewRouter()
	passThrough := func(next http.Handler) http.Handler { return next }
	registerSystemRoutes(r, nil)
	registerAPIRoutes(r, testJWTSecret, fakeTokenValidator{}, fakeSessionChecker{}, passThrough, &apiHandlers{})
	return r
}

func probe(r chi.Router, method, pattern, bearer string) string {
	path := routeParam.ReplaceAllString(pattern, "1")
	req := httptest.NewRequest(method, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	func() {
		defer func() { _ = recover() }() // the handlers are nil: reaching one panics, which is what "let through" means here
		r.ServeHTTP(rec, req)
	}()
	return rec.Body.String()
}

// TestRouteTableAuthTiers pins which routes are public, which an API token opens and which need a session, so
// that a route moved into the wrong group fails here instead of widening access.
func TestRouteTableAuthTiers(t *testing.T) {
	r := newRouteTable(t)

	session, err := auth.GenerateAccessToken("user-1", "user@example.com", testJWTSecret, 5)
	if err != nil {
		t.Fatal(err)
	}
	apiToken := auth.TokenPrefix + "route-table-test"

	var public, integration, sessionOnly []string
	err = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		label := method + " " + route
		if strings.HasSuffix(route, "/*") {
			return nil
		}
		switch {
		case !strings.Contains(probe(r, method, route, ""), noCredentialsMessage):
			public = append(public, label)
		case !strings.Contains(probe(r, method, route, apiToken), sessionOnlyMessage):
			integration = append(integration, label)
			assertSessionOpens(t, r, method, route, session)
		default:
			sessionOnly = append(sessionOnly, label)
			assertSessionOpens(t, r, method, route, session)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	wantPublic := []string{
		"GET /api/auth/config",
		"GET /api/auth/oidc/callback",
		"GET /api/auth/oidc/login",
		"GET /api/health",
		"GET /api/version",
		"GET /healthz",
		"POST /api/auth/login",
		"POST /api/auth/logout",
		"POST /api/auth/refresh",
		"POST /api/auth/register",
	}
	wantIntegration := []string{
		"GET /api/integrations/homeassistant/vehicles",
		"GET /api/integrations/homeassistant/vehicles/{vehicleId}",
		"GET /api/integrations/homeassistant/vehicles/{vehicleId}/metrics",
		"POST /api/integrations/homeassistant/event",
	}
	assertSameRoutes(t, "public", public, wantPublic)
	assertSameRoutes(t, "API token", integration, wantIntegration)

	if len(sessionOnly) < 150 {
		t.Errorf("only %d session routes found, the route table lost routes", len(sessionOnly))
	}
	for _, label := range sessionOnly {
		if strings.Contains(label, "/api/integrations/") {
			t.Errorf("%s needs a session although it is an integration route", label)
		}
	}
}

func assertSessionOpens(t *testing.T, r chi.Router, method, route, session string) {
	t.Helper()
	body := probe(r, method, route, session)
	if strings.Contains(body, noCredentialsMessage) || strings.Contains(body, badSessionMessage) {
		t.Errorf("%s %s: a session must open the route, got %s", method, route, body)
	}
}

func assertSameRoutes(t *testing.T, tier string, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s routes differ\n got: %v\nwant: %v", tier, got, want)
	}
}

// fakeSessionChecker lets every session through: these tests are about which routes need which credential.
type fakeSessionChecker struct{}

func (fakeSessionChecker) IsSessionActive(context.Context, string, string) (bool, error) {
	return true, nil
}
