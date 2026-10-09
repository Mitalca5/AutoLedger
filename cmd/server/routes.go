package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/teslacost/teslacost/internal/config"
	"github.com/teslacost/teslacost/internal/handlers"
	appMiddleware "github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/web"
)

// maxJSONBodyBytes caps the body of API requests; uploads have their own limit in their handler.
const maxJSONBodyBytes = 1 << 20

// contentSecurityPolicy is the policy sent with every response: the built-in one, a custom one from the
// environment, or none ("off").
func contentSecurityPolicy(cfg *config.Config) string {
	switch {
	case strings.EqualFold(cfg.ContentSecurityPolicy, "off"):
		return ""
	case cfg.ContentSecurityPolicy != "":
		return cfg.ContentSecurityPolicy
	default:
		return appMiddleware.DefaultCSP
	}
}

// newRouter builds the router with the middleware every request goes through.
func newRouter(cfg *config.Config, trustedProxies []netip.Prefix) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	// The client address only comes from forwarding headers when the peer is a trusted proxy: rate limits and
	// session records depend on it.
	r.Use(appMiddleware.ClientIP(trustedProxies))
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))
	if cfg.SecurityHeaders {
		r.Use(appMiddleware.SecurityHeaders(contentSecurityPolicy(cfg)))
	}
	r.Use(appMiddleware.BodyLimit(maxJSONBodyBytes))
	if cfg.Demo {
		r.Use(appMiddleware.DemoReadOnly)
	}
	r.Use(appMiddleware.OriginCheck(append([]string{cfg.AppBaseURL}, cfg.AllowedOrigins...)))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	return r
}

// registerSystemRoutes registers the public health and version endpoints.
func registerSystemRoutes(r chi.Router, ping func(context.Context) error) {
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "disconnected"
		if ping != nil {
			pingCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if err := ping(pingCtx); err == nil {
				dbStatus = "connected"
			}
		}

		status := "healthy"
		httpStatus := http.StatusOK
		if dbStatus != "connected" {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]any{
			"status":    status,
			"database":  dbStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   AppVersion,
		})
	}
	r.Get("/api/health", healthHandler)
	r.Get("/healthz", healthHandler)
	r.Get("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"version": AppVersion,
		})
	})
}

// registerAPIRoutes mounts the API in three tiers: public authentication, the integration routes an API token
// opens, and the routes that need a signed-in session.
func registerAPIRoutes(r chi.Router, jwtSecret string, tokens appMiddleware.APITokenValidator, sessions appMiddleware.SessionChecker, idempotency func(http.Handler) http.Handler, h *apiHandlers) {
	h.registerPublicAuthRoutes(r)

	// Integration routes: the only ones an API token opens (Home Assistant, scripts); a session works too.
	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthenticateIntegration(jwtSecret, tokens))
		r.Use(idempotency)

		h.registerIntegrationRoutes(r)
	})

	// Protected routes (session only)
	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthenticateJWT(jwtSecret))
		r.Use(idempotency)

		h.registerSessionAuthRoutes(r, appMiddleware.RequireActiveSession(sessions))
		h.registerAccountRoutes(r)

		r.Route("/api/vehicles", func(r chi.Router) {
			h.registerVehicleRoutes(r)
			h.registerDriveRoutes(r)
			h.registerCostRoutes(r)
		})
	})
}

// registerSPA serves the embedded Vue frontend for every path no other route claims.
func registerSPA(r chi.Router) {
	r.Handle("/*", handlers.NewSPAServer(web.GetDistFS()))
}
