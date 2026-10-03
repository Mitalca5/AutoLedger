package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teslacost/teslacost/internal/config"
)

func clearSecretEnv(t *testing.T) {
	for _, k := range []string{"AUTOLEDGER_JWT_SECRET", "JWT_SECRET", "AUTOLEDGER_ENCRYPTION_KEY", "APP_ENCRYPTION_KEY"} {
		t.Setenv(k, "")
	}
}

func TestProductionGeneratesAndReusesSecrets(t *testing.T) {
	clearSecretEnv(t)
	dir := t.TempDir()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("AUTOLEDGER_STORAGE_DIR", dir)

	first := config.Load()
	if len(first.JWTSecret) != 64 || len(first.AppEncryptionKey) != 64 || first.JWTSecret == first.AppEncryptionKey {
		t.Fatalf("expected two distinct 64-char secrets, got %q / %q", first.JWTSecret, first.AppEncryptionKey)
	}
	if w := first.InsecureDefaults(); len(w) != 0 && len(w) != 1 { // only the DB password may remain
		t.Fatalf("unexpected warnings: %v", w)
	}
	info, err := os.Stat(filepath.Join(dir, ".autoledger-secrets.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("secrets file missing or too permissive: %v %v", err, info)
	}

	second := config.Load()
	if second.JWTSecret != first.JWTSecret || second.AppEncryptionKey != first.AppEncryptionKey {
		t.Fatal("secrets must be stable across restarts")
	}
}

func TestExplicitSecretsAreNeverReplaced(t *testing.T) {
	clearSecretEnv(t)
	dir := t.TempDir()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("AUTOLEDGER_STORAGE_DIR", dir)
	t.Setenv("AUTOLEDGER_JWT_SECRET", "my-own-jwt-secret")
	t.Setenv("AUTOLEDGER_ENCRYPTION_KEY", "my-own-encryption-key")

	cfg := config.Load()
	if cfg.JWTSecret != "my-own-jwt-secret" || cfg.AppEncryptionKey != "my-own-encryption-key" {
		t.Fatalf("explicit values were replaced: %q / %q", cfg.JWTSecret, cfg.AppEncryptionKey)
	}
	if _, err := os.Stat(filepath.Join(dir, ".autoledger-secrets.json")); !os.IsNotExist(err) {
		t.Fatal("no file should be written when both secrets are set")
	}
}

func TestOnlyTheMissingSecretIsGenerated(t *testing.T) {
	clearSecretEnv(t)
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("AUTOLEDGER_STORAGE_DIR", t.TempDir())
	t.Setenv("AUTOLEDGER_ENCRYPTION_KEY", "kept-encryption-key")

	cfg := config.Load()
	if cfg.AppEncryptionKey != "kept-encryption-key" || len(cfg.JWTSecret) != 64 {
		t.Fatalf("got %q / %q", cfg.AppEncryptionKey, cfg.JWTSecret)
	}
}

func TestDevelopmentKeepsBuiltInDefaults(t *testing.T) {
	clearSecretEnv(t)
	dir := t.TempDir()
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("AUTOLEDGER_STORAGE_DIR", dir)

	cfg := config.Load()
	if len(cfg.InsecureDefaults()) < 2 {
		t.Fatal("development should keep the well-known defaults")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatal("development must not write a secrets file")
	}
}

func TestUnwritableStorageFallsBackToRefusingDefaults(t *testing.T) {
	clearSecretEnv(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("AUTOLEDGER_STORAGE_DIR", filepath.Join(blocker, "sub"))

	if len(config.Load().InsecureDefaults()) < 2 {
		t.Fatal("unpersistable secrets must leave the defaults so startup is refused")
	}
}

func TestCorruptSecretsFileIsNotOverwritten(t *testing.T) {
	clearSecretEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, ".autoledger-secrets.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("AUTOLEDGER_STORAGE_DIR", dir)

	if len(config.Load().InsecureDefaults()) < 2 {
		t.Fatal("a corrupt file must not be silently replaced")
	}
	if b, _ := os.ReadFile(path); string(b) != "{broken" {
		t.Fatal("corrupt file was overwritten")
	}
}

func TestDatabasePasswordCanComeFromAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "db_password")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_HOST", "postgres")
	t.Setenv("AUTOLEDGER_DB_PASSWORD", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_PASSWORD_FILE", file)
	if got := config.Load().DatabaseURL; !strings.Contains(got, ":from-file@") {
		t.Fatalf("password not read from file: %s", got)
	}

	t.Setenv("DB_PASSWORD", "explicit")
	if got := config.Load().DatabaseURL; !strings.Contains(got, ":explicit@") {
		t.Fatalf("explicit password must win: %s", got)
	}

	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_PASSWORD_FILE", filepath.Join(t.TempDir(), "missing"))
	if got := config.Load().DatabaseURL; strings.Contains(got, "from-file") {
		t.Fatalf("unexpected password: %s", got)
	}
}

func TestSessionAndEncryptionSecretsCanComeFromFiles(t *testing.T) {
	clearSecretEnv(t)
	dir := t.TempDir()
	jwt, enc := filepath.Join(dir, "jwt"), filepath.Join(dir, "enc")
	for f, v := range map[string]string{jwt: "jwt-from-file\n", enc: "enc-from-file"} {
		if err := os.WriteFile(f, []byte(v), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("AUTOLEDGER_STORAGE_DIR", t.TempDir())
	t.Setenv("AUTOLEDGER_JWT_SECRET_FILE", jwt)
	t.Setenv("AUTOLEDGER_ENCRYPTION_KEY_FILE", enc)

	cfg := config.Load()
	if cfg.JWTSecret != "jwt-from-file" || cfg.AppEncryptionKey != "enc-from-file" {
		t.Fatalf("got %q / %q", cfg.JWTSecret, cfg.AppEncryptionKey)
	}
	if entries, _ := os.ReadDir(os.Getenv("AUTOLEDGER_STORAGE_DIR")); len(entries) != 0 {
		t.Fatal("no generated file expected when secrets come from files")
	}
}
