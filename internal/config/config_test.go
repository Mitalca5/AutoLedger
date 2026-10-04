package config_test

import (
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teslacost/teslacost/internal/config"
)

func TestSpecialCharactersInPassword(t *testing.T) {
	os.Setenv("DB_HOST", "postgres")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "teslacost")
	os.Setenv("DB_PASSWORD", "ComplexP@ss!&M#123")
	os.Setenv("DB_NAME", "teslacost")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
	}()

	cfg := config.Load()

	// Parse with pgxpool to verify that pgx parses the host and password correctly
	pgxCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("Failed to parse DatabaseURL: %v", err)
	}

	if pgxCfg.ConnConfig.Host != "postgres" {
		t.Errorf("Expected host to be 'postgres', got '%s'", pgxCfg.ConnConfig.Host)
	}
	if pgxCfg.ConnConfig.Password != "ComplexP@ss!&M#123" {
		t.Errorf("Expected password to be 'ComplexP@ss!&M#123', got '%s'", pgxCfg.ConnConfig.Password)
	}
	if pgxCfg.ConnConfig.User != "teslacost" {
		t.Errorf("Expected user to be 'teslacost', got '%s'", pgxCfg.ConnConfig.User)
	}
}

func TestInsecureDefaultsDetectsKnownPlaceholders(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:        "super_secret_jwt_signing_key_for_teslacost_app",
		AppEncryptionKey: "generate_a_random_32_characters_key_here!",
		DatabaseURL:      "postgres://teslacost:teslacost_dev_secret@postgres:5432/teslacost?sslmode=disable",
	}

	warnings := cfg.InsecureDefaults()
	if len(warnings) != 3 {
		t.Fatalf("expected 3 warnings for JWT secret, encryption key and DB password, got %d: %v", len(warnings), warnings)
	}
}

func TestInsecureDefaultsClearOnCustomSecrets(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:        "a-real-random-secret",
		AppEncryptionKey: "another-real-random-key-value-xx",
		DatabaseURL:      "postgres://teslacost:S0m3R34lPassw0rd@postgres:5432/teslacost?sslmode=disable",
	}

	if warnings := cfg.InsecureDefaults(); len(warnings) != 0 {
		t.Fatalf("expected no warnings for custom secrets, got: %v", warnings)
	}
}

func TestNormalizeDatabaseURLWithUnescapedPassword(t *testing.T) {
	raw := "postgres://teslacost:MyPasswordM&!@123@postgres:5432/teslacost?sslmode=disable"
	normalized := config.NormalizeDatabaseURL(raw)

	pgxCfg, err := pgxpool.ParseConfig(normalized)
	if err != nil {
		t.Fatalf("Failed to parse normalized DatabaseURL: %v", err)
	}

	if pgxCfg.ConnConfig.Host != "postgres" {
		t.Errorf("Expected host to be 'postgres', got '%s'", pgxCfg.ConnConfig.Host)
	}
	if pgxCfg.ConnConfig.Password != "MyPasswordM&!@123" {
		t.Errorf("Expected password to be 'MyPasswordM&!@123', got '%s'", pgxCfg.ConnConfig.Password)
	}
	if pgxCfg.ConnConfig.User != "teslacost" {
		t.Errorf("Expected user to be 'teslacost', got '%s'", pgxCfg.ConnConfig.User)
	}
}

func TestDefaultAllowedOrigins(t *testing.T) {
	cases := []struct {
		name, baseURL, env string
		want               []string
	}{
		{"production uses only the public origin", "https://cost.example.com/app", "production", []string{"https://cost.example.com"}},
		{"development adds local dev servers", "http://localhost:8080", "development", []string{"http://localhost:8080", "http://localhost:3000", "http://localhost:5173"}},
		{"dev server already the base URL is not duplicated", "http://localhost:5173", "development", []string{"http://localhost:5173", "http://localhost:3000"}},
		{"invalid base URL in production yields nothing", "not a url", "production", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := config.DefaultAllowedOrigins(c.baseURL, c.env)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestExplicitCORSOriginsOverrideDefaults(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://a.example.com , https://b.example.com ")
	t.Setenv("APP_BASE_URL", "https://cost.example.com")
	got := config.Load().AllowedOrigins
	if len(got) != 2 || got[0] != "https://a.example.com" || got[1] != "https://b.example.com" {
		t.Fatalf("explicit origins must replace the derived ones, got %v", got)
	}
}

func TestTrustedProxiesDefaultAndOverride(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	if got := config.Load().TrustedProxies; len(got) != len(config.DefaultTrustedProxies) {
		t.Errorf("default ranges expected, got %v", got)
	}
	t.Setenv("TRUSTED_PROXIES", "203.0.113.4, 198.51.100.0/24")
	got := config.Load().TrustedProxies
	if len(got) != 2 || got[0] != "203.0.113.4" || got[1] != "198.51.100.0/24" {
		t.Errorf("override expected, got %v", got)
	}
}

func TestSecurityHeadersDefaultOn(t *testing.T) {
	t.Setenv("SECURITY_HEADERS", "")
	if !config.Load().SecurityHeaders {
		t.Error("security headers must default to on")
	}
	t.Setenv("SECURITY_HEADERS", "false")
	if config.Load().SecurityHeaders {
		t.Error("SECURITY_HEADERS=false must turn them off")
	}
}

func TestGeocodingIsOffByDefault(t *testing.T) {
	if cfg := config.Load(); cfg.GeocodingEnabled || cfg.GeocodingURL != "" {
		t.Fatalf("geocoding must be opt-in: %+v", cfg)
	}
	t.Setenv("GEOCODING_ENABLED", "true")
	t.Setenv("GEOCODING_URL", " http://nominatim.local:8080 ")
	cfg := config.Load()
	if !cfg.GeocodingEnabled || cfg.GeocodingURL != "http://nominatim.local:8080" {
		t.Fatalf("got %+v", cfg)
	}
}
