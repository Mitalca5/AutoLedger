package middleware

import (
	"encoding/json"
	"net/http"
)

// demoWritablePaths are the only state-changing routes a read-only demo keeps open: they open and close the
// visitor's session without touching any data.
var demoWritablePaths = map[string]bool{
	"/api/auth/login":   true,
	"/api/auth/refresh": true,
	"/api/auth/logout":  true,
}

// DemoReadOnly turns the instance into a read-only demo: every request that could change state is refused,
// whatever the credential, so uploads, synchronisation, webhooks, token creation and password changes are all out.
func DemoReadOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if demoWritablePaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "This demo is read-only", "code": "demo.read_only"})
	})
}
