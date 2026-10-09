package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
)

// registerPublicAuthRoutes registers the credential endpoints that need no session.
func (h *apiHandlers) registerPublicAuthRoutes(r chi.Router) {
	// Public Auth
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/config", h.auth.GetConfig)
		// Rate limited by IP: these are the credential-guessing surface (password brute
		// force, account enumeration via registration). 10 attempts/minute is generous for
		// a legitimate user retrying a typo but blocks automated guessing.
		r.With(httprate.LimitByIP(10, time.Minute)).Post("/register", h.auth.Register)
		r.With(httprate.LimitByIP(10, time.Minute)).Post("/login", h.auth.Login)
		// A refresh token is a credential too: guessing is pointless (256 random bits) but the route does a
		// database write, so it gets a generous limit rather than none.
		r.With(httprate.LimitByIP(30, time.Minute)).Post("/refresh", h.auth.RefreshToken)
		r.Post("/logout", h.auth.Logout)
		// OIDC Authorization Code Flow endpoints (public — no JWT required)
		r.With(httprate.LimitByIP(20, time.Minute)).Get("/oidc/login", h.auth.OIDCLogin)
		r.With(httprate.LimitByIP(20, time.Minute)).Get("/oidc/callback", h.auth.OIDCCallback)
	})
}

// registerSessionAuthRoutes registers the account endpoints of a signed-in session, including API token management.
// Creating a credential (a password, an API token) also needs the session to be active, see activeSession.
func (h *apiHandlers) registerSessionAuthRoutes(r chi.Router, activeSession func(http.Handler) http.Handler) {
	r.Get("/api/auth/me", h.auth.Me)
	r.Get("/api/auth/sessions", h.auth.ListSessions)
	r.Delete("/api/auth/sessions/{sessionId}", h.auth.RevokeSession)
	r.Post("/api/auth/logout-all", h.auth.LogoutAll)
	r.With(httprate.LimitByIP(10, time.Minute), activeSession).Post("/api/auth/password", h.auth.ChangePassword)
	// Linking an SSO identity to this account is explicit: it starts from an active signed-in session (see AuthHandler.OIDCLink).
	r.With(activeSession).Get("/api/auth/oidc/link", h.auth.OIDCLink)
	r.Put("/api/auth/language", h.auth.UpdateLanguage)
	r.Put("/api/auth/distance-unit", h.auth.UpdateDistanceUnit)

	// API Tokens (External Integrations / Home Assistant)
	r.Route("/api/auth/tokens", func(r chi.Router) {
		r.Get("/", h.token.ListTokens)
		r.With(activeSession).Post("/", h.token.CreateToken)
		r.Delete("/{tokenId}", h.token.RevokeToken)
	})
}
