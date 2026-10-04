package config

import (
	"log/slog"
	"net"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
)

// DefaultTrustedProxies are the address ranges assumed for the reverse proxy in front of the application:
// loopback and the private ranges of Docker networks and home LANs. A proxy on a public address has to be
// listed in TRUSTED_PROXIES.
var DefaultTrustedProxies = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7",
}

// Config stores application configuration loaded from environment variables.
type Config struct {
	Port                       string
	AppBaseURL                 string
	Environment                string
	DatabaseURL                string
	AppEncryptionKey           string
	JWTSecret                  string
	JWTAccessExpirationMinutes int
	JWTRefreshExpirationDays   int
	CookieSecure               bool
	DisableRegistration        bool
	Demo                       bool   // Read-only public demo: every write is refused
	DemoEmail                  string // Optional demo account shown on the sign-in page
	DemoPassword               string
	InitialAdminEmail          string
	InitialAdminPassword       string
	AllowedOrigins             []string
	SyncIntervalMinutes        int
	ReportingTimezone          string
	StorageDir                 string // Directory for document file storage (Docker volume mount point)

	// Reverse geocoding of the positions sent by integrations (optional, off by default: coordinates leave the server)
	GeocodingEnabled   bool
	GeocodingURL       string // Nominatim base URL; empty = the public OpenStreetMap instance
	GeocodingUserAgent string // Empty = a default that identifies AutoLedger

	// Reverse proxy and browser hardening
	TrustedProxies        []string // Addresses or CIDR ranges of the reverse proxy allowed to set X-Forwarded-*; default: private ranges
	SecurityHeaders       bool     // Send the security headers (disable when the proxy already sets them)
	ContentSecurityPolicy string   // Empty = built-in policy, "off" = no CSP header, anything else replaces the policy

	// OIDC / OAuth2 SSO (optional — enabled when OIDCIssuerURL is non-empty)
	OIDCEnabled          bool
	OIDCIssuerURL        string // e.g. https://auth.homelab.local/application/o/teslacost/
	OIDCClientID         string
	OIDCClientSecret     string
	OIDCRedirectURL      string   // e.g. https://teslacost.homelab.local/api/auth/oidc/callback
	OIDCScopes           []string // default: ["openid", "email", "profile"]
	OIDCProviderName     string   // label shown in the UI, e.g. "Authentik"
	OIDCAllowedEmails    []string // optional whitelist; empty = allow all
	OIDCDisableLocalAuth bool     // when true, /login and /register endpoints are disabled
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	port := getEnvWithFallback("AUTOLEDGER_PORT", "PORT", "8080")
	appBaseURL := getEnvWithFallback("AUTOLEDGER_BASE_URL", "APP_BASE_URL", "http://localhost:8080")
	env := getEnv("ENVIRONMENT", "development")

	// Determine database URL:
	// If DB_HOST is set, build a safely URL-encoded connection string (preventing issues with special characters in passwords).
	var dbURL string
	if dbHost := getEnvWithFallback("AUTOLEDGER_DB_HOST", "DB_HOST", ""); dbHost != "" {
		dbPort := getEnvWithFallback("AUTOLEDGER_DB_PORT", "DB_PORT", "5432")
		dbUser := getEnvWithFallback("AUTOLEDGER_DB_USER", "DB_USER", "teslacost")
		dbPass := getEnvWithFallback("AUTOLEDGER_DB_PASSWORD", "DB_PASSWORD", "")
		if dbPass == "" {
			dbPass = readSecretFile(getEnvWithFallback("AUTOLEDGER_DB_PASSWORD_FILE", "DB_PASSWORD_FILE", ""))
		}
		dbName := getEnvWithFallback("AUTOLEDGER_DB_NAME", "DB_NAME", "teslacost")
		dbSSL := getEnv("DB_SSLMODE", "disable")

		u := &url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(dbUser, dbPass),
			Host:     net.JoinHostPort(dbHost, dbPort),
			Path:     "/" + dbName,
			RawQuery: "sslmode=" + dbSSL,
		}
		dbURL = u.String()
	} else {
		// No credentials in code: the password comes from DATABASE_URL, DB_PASSWORD or PGPASSWORD.
		rawURL := getEnvWithFallback("AUTOLEDGER_DATABASE_URL", "DATABASE_URL", "postgres://teslacost@localhost:5432/teslacost?sslmode=disable")
		dbURL = NormalizeDatabaseURL(rawURL)
	}

	encKey := getEnvWithFallback("AUTOLEDGER_ENCRYPTION_KEY", "APP_ENCRYPTION_KEY", "")
	if encKey == "" {
		encKey = readSecretFile(getEnv("AUTOLEDGER_ENCRYPTION_KEY_FILE", ""))
	}
	jwtSecret := getEnvWithFallback("AUTOLEDGER_JWT_SECRET", "JWT_SECRET", "")
	if jwtSecret == "" {
		jwtSecret = readSecretFile(getEnv("AUTOLEDGER_JWT_SECRET_FILE", ""))
	}
	jwtAccessExpMinutes, _ := strconv.Atoi(getEnv("JWT_ACCESS_EXPIRATION_MINUTES", "15"))
	if jwtAccessExpMinutes <= 0 {
		jwtAccessExpMinutes = 15
	}

	jwtRefreshExpDays, _ := strconv.Atoi(getEnv("JWT_REFRESH_EXPIRATION_DAYS", "30"))
	if jwtRefreshExpDays <= 0 {
		jwtRefreshExpDays = 30
	}

	// Default CookieSecure to true in production or if appBaseURL uses https
	defaultCookieSecure := strings.EqualFold(env, "production") || strings.HasPrefix(strings.ToLower(appBaseURL), "https://")
	cookieSecure := getEnvBool("COOKIE_SECURE", defaultCookieSecure)

	disableRegistration := getEnvBool("DISABLE_REGISTRATION", false)
	demo := getEnvBool("AUTOLEDGER_DEMO", false)
	if demo {
		disableRegistration = true
	}
	initialAdminEmail := getEnv("INITIAL_ADMIN_EMAIL", "")
	initialAdminPassword := getEnv("INITIAL_ADMIN_PASSWORD", "")

	allowedOrigins := parseOrigins(getEnv("CORS_ALLOWED_ORIGINS", ""))
	if len(allowedOrigins) == 0 {
		allowedOrigins = DefaultAllowedOrigins(appBaseURL, env)
	}

	syncIntervalMinutes, _ := strconv.Atoi(getEnv("SYNC_INTERVAL_MINUTES", "30"))
	if syncIntervalMinutes < 0 {
		syncIntervalMinutes = 0
	}

	trustedProxies := DefaultTrustedProxies
	if raw := getEnv("TRUSTED_PROXIES", ""); raw != "" {
		trustedProxies = parseOrigins(raw)
	}

	reportingTimezone := getEnv("APP_TIMEZONE", "Europe/Paris")
	storageDir := getEnvWithFallback("AUTOLEDGER_STORAGE_DIR", "STORAGE_DIR", "./data/documents")

	encSet, jwtSet := encKey != "", jwtSecret != ""
	if !encSet {
		encKey = "dev-default-32-byte-secret-key!!"
	}
	if !jwtSet {
		jwtSecret = "super_secret_jwt_signing_key_for_teslacost_app"
	}
	jwtSecret, encKey = resolveSecrets(strings.ToLower(env), storageDir, jwtSecret, encKey, jwtSet, encSet)

	// OIDC configuration
	oidcIssuerURL := getEnv("OIDC_ISSUER_URL", "")
	oidcClientID := getEnv("OIDC_CLIENT_ID", "")
	oidcClientSecret := getEnv("OIDC_CLIENT_SECRET", "")
	oidcRedirectURL := getEnv("OIDC_REDIRECT_URL", "")
	oidcProviderName := getEnv("OIDC_PROVIDER_NAME", "SSO")
	oidcDisableLocalAuth := getEnvBool("OIDC_DISABLE_LOCAL_AUTH", false)

	var oidcScopes []string
	if scopesRaw := getEnv("OIDC_SCOPES", "openid email profile"); scopesRaw != "" {
		for _, s := range strings.Split(scopesRaw, " ") {
			if s = strings.TrimSpace(s); s != "" {
				oidcScopes = append(oidcScopes, s)
			}
		}
	}

	var oidcAllowedEmails []string
	if emailsRaw := getEnv("OIDC_ALLOWED_EMAILS", ""); emailsRaw != "" {
		for _, e := range strings.Split(emailsRaw, ",") {
			if e = strings.TrimSpace(e); e != "" {
				oidcAllowedEmails = append(oidcAllowedEmails, e)
			}
		}
	}

	return &Config{
		Port:                       port,
		AppBaseURL:                 appBaseURL,
		Environment:                env,
		DatabaseURL:                dbURL,
		AppEncryptionKey:           encKey,
		JWTSecret:                  jwtSecret,
		JWTAccessExpirationMinutes: jwtAccessExpMinutes,
		JWTRefreshExpirationDays:   jwtRefreshExpDays,
		CookieSecure:               cookieSecure,
		DisableRegistration:        disableRegistration,
		Demo:                       demo,
		DemoEmail:                  getEnv("AUTOLEDGER_DEMO_EMAIL", ""),
		DemoPassword:               getEnv("AUTOLEDGER_DEMO_PASSWORD", ""),
		InitialAdminEmail:          initialAdminEmail,
		InitialAdminPassword:       initialAdminPassword,
		AllowedOrigins:             allowedOrigins,
		SyncIntervalMinutes:        syncIntervalMinutes,
		ReportingTimezone:          reportingTimezone,
		StorageDir:                 storageDir,
		GeocodingEnabled:           getEnvBool("GEOCODING_ENABLED", false),
		GeocodingURL:               strings.TrimSpace(getEnv("GEOCODING_URL", "")),
		GeocodingUserAgent:         strings.TrimSpace(getEnv("GEOCODING_USER_AGENT", "")),
		TrustedProxies:             trustedProxies,
		SecurityHeaders:            getEnvBool("SECURITY_HEADERS", true),
		ContentSecurityPolicy:      strings.TrimSpace(getEnv("CONTENT_SECURITY_POLICY", "")),
		OIDCEnabled:                oidcIssuerURL != "",
		OIDCIssuerURL:              oidcIssuerURL,
		OIDCClientID:               oidcClientID,
		OIDCClientSecret:           oidcClientSecret,
		OIDCRedirectURL:            oidcRedirectURL,
		OIDCScopes:                 oidcScopes,
		OIDCProviderName:           oidcProviderName,
		OIDCAllowedEmails:          oidcAllowedEmails,
		OIDCDisableLocalAuth:       oidcDisableLocalAuth,
	}
}

var knownDefaultJWTSecrets = []string{
	"super_secret_jwt_signing_key_for_teslacost_app",
	"super_secret_jwt_signing_key_for_autoledger_app",
}

var knownDefaultDBPasswords = []string{
	"teslacost_dev_secret",
	"autoledger_dev_secret",
}

var knownDefaultEncryptionKeys = []string{
	"dev-default-32-byte-secret-key!!",             // internal/config/config.go Load() fallback
	"change-this-to-a-secure-32-byte-key-in-prod!", // docker-compose.yml fallback
	"generate_a_random_32_characters_key_here!",    // .env.example placeholder
}

// InsecureDefaults returns a human-readable warning for each secret that still holds a
// known placeholder/default value. It is intended to be logged loudly (not to block startup)
// so a misconfigured production deployment is easy to spot in the logs.
func (c *Config) InsecureDefaults() []string {
	var warnings []string

	if slices.Contains(knownDefaultJWTSecrets, c.JWTSecret) {
		warnings = append(warnings, "JWT_SECRET is set to the well-known default value from the repository — anyone can forge valid session tokens")
	}

	for _, known := range knownDefaultEncryptionKeys {
		if c.AppEncryptionKey == known {
			warnings = append(warnings, "APP_ENCRYPTION_KEY is set to a well-known placeholder value — stored credentials can be decrypted by anyone with the source code")
			break
		}
	}

	if u, err := url.Parse(c.DatabaseURL); err == nil && u.User != nil {
		if pass, _ := u.User.Password(); slices.Contains(knownDefaultDBPasswords, pass) {
			warnings = append(warnings, "DB_PASSWORD is set to the well-known default value from the repository")
		}
	}

	return warnings
}

// DefaultAllowedOrigins derives the CORS origins from the public base URL, since the embedded
// frontend is same-origin as the API. Outside production, the local dev servers are added.
func DefaultAllowedOrigins(appBaseURL, env string) []string {
	var origins []string
	if u, err := url.Parse(appBaseURL); err == nil && u.Scheme != "" && u.Host != "" {
		origins = append(origins, u.Scheme+"://"+u.Host)
	}
	if !strings.EqualFold(env, "production") {
		for _, dev := range []string{"http://localhost:3000", "http://localhost:5173"} {
			if !slices.Contains(origins, dev) {
				origins = append(origins, dev)
			}
		}
	}
	return origins
}

func parseOrigins(raw string) []string {
	var origins []string
	for _, origin := range strings.Split(raw, ",") {
		if o := strings.TrimSpace(origin); o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}

func getEnvWithFallback(primaryKey, fallbackKey, defaultVal string) string {
	if val, exists := os.LookupEnv(primaryKey); exists && val != "" {
		return val
	}
	if val, exists := os.LookupEnv(fallbackKey); exists && val != "" {
		slog.Warn("using deprecated configuration environment variable",
			"deprecated", fallbackKey,
			"recommended", primaryKey,
		)
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	val, exists := os.LookupEnv(key)
	if !exists || val == "" {
		return defaultVal
	}
	val = strings.ToLower(strings.TrimSpace(val))
	return val == "true" || val == "1" || val == "yes"
}

func getEnvBoolWithFallback(primaryKey, fallbackKey string, defaultVal bool) bool {
	if val, exists := os.LookupEnv(primaryKey); exists && val != "" {
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "true" || val == "1" || val == "yes"
	}
	if val, exists := os.LookupEnv(fallbackKey); exists && val != "" {
		slog.Warn("using deprecated configuration environment variable",
			"deprecated", fallbackKey,
			"recommended", primaryKey,
		)
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultVal
}

// NormalizeDatabaseURL ensures that any special characters in the password component
// of a database URL (like '@', '&', '!', '#', '%') are properly percent-encoded,
// preventing host parsing errors in libpq/pgx when passwords contain '@'.
func NormalizeDatabaseURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return rawURL
	}

	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd == -1 {
		return rawURL
	}
	scheme := rawURL[:schemeEnd]
	rest := rawURL[schemeEnd+3:]

	authEnd := len(rest)
	if idx := strings.IndexAny(rest, "/?"); idx != -1 {
		authEnd = idx
	}
	authority := rest[:authEnd]
	pathAndQuery := rest[authEnd:]

	lastAt := strings.LastIndex(authority, "@")
	if lastAt == -1 {
		return rawURL
	}

	userInfo := authority[:lastAt]
	hostPort := authority[lastAt+1:]

	colonIdx := strings.Index(userInfo, ":")
	if colonIdx == -1 {
		return rawURL
	}

	user := userInfo[:colonIdx]
	pass := userInfo[colonIdx+1:]

	u := &url.URL{
		Scheme: scheme,
		User:   url.UserPassword(user, pass),
		Host:   hostPort,
	}

	return u.String() + pathAndQuery
}

// readSecretFile returns the trimmed content of a secret mounted as a file (Docker secrets, shared volume).
func readSecretFile(path string) string {
	if path == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Error("cannot read secret file", "component", "config", "path", path, "error", err)
		return ""
	}
	return strings.TrimSpace(string(raw))
}
