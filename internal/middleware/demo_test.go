package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDemoReadOnly(t *testing.T) {
	h := DemoReadOnly(ok)
	cases := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/vehicles", http.StatusNoContent},
		{http.MethodOptions, "/api/vehicles", http.StatusNoContent},
		{http.MethodPost, "/api/auth/login", http.StatusNoContent},
		{http.MethodPost, "/api/auth/refresh", http.StatusNoContent},
		{http.MethodPost, "/api/auth/logout", http.StatusNoContent},
		{http.MethodPost, "/api/auth/register", http.StatusForbidden},
		{http.MethodPost, "/api/integrations/homeassistant/event", http.StatusForbidden},
		{http.MethodPut, "/api/auth/password", http.StatusForbidden},
		{http.MethodPatch, "/api/vehicles/1", http.StatusForbidden},
		{http.MethodDelete, "/api/vehicles/1", http.StatusForbidden},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.want {
			t.Errorf("%s %s: got %d, want %d", c.method, c.path, rec.Code, c.want)
		}
		if c.want == http.StatusForbidden && !strings.Contains(rec.Body.String(), "demo.read_only") {
			t.Errorf("%s %s: missing error code in %q", c.method, c.path, rec.Body.String())
		}
	}
}
