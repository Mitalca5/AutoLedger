package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const secretsFileName = ".autoledger-secrets.json"

type generatedSecrets struct {
	JWTSecret     string `json:"jwt_secret"`
	EncryptionKey string `json:"encryption_key"`
}

func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// loadOrCreateSecrets returns the secrets persisted in dir, creating them on first use. The file lives on the
// storage volume so the values survive restarts and upgrades; losing it would invalidate sessions and make the
// stored credentials unreadable.
func loadOrCreateSecrets(dir string) (generatedSecrets, error) {
	path := filepath.Join(dir, secretsFileName)
	if raw, err := os.ReadFile(path); err == nil {
		var s generatedSecrets
		if err := json.Unmarshal(raw, &s); err != nil || s.JWTSecret == "" || s.EncryptionKey == "" {
			return generatedSecrets{}, fmt.Errorf("%s is unreadable or incomplete", path)
		}
		return s, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return generatedSecrets{}, err
	}

	var s generatedSecrets
	var err error
	if s.JWTSecret, err = randomSecret(); err != nil {
		return s, err
	}
	if s.EncryptionKey, err = randomSecret(); err != nil {
		return s, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return s, err
	}
	raw, _ := json.Marshal(s)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadOrCreateSecrets(dir)
	}
	if err != nil {
		return s, err
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return s, err
	}
	if err := f.Close(); err != nil {
		return s, err
	}
	slog.Info("generated application secrets", "component", "security", "path", path)
	return s, nil
}

// resolveSecrets fills in the JWT secret and encryption key that the operator left unset. Only a production
// deployment generates them; a failure to persist them keeps the built-in defaults so the startup check
// refuses to run rather than silently using values that would change on the next restart.
func resolveSecrets(env, storageDir, jwtSecret, encKey string, jwtSet, encSet bool) (string, string) {
	if jwtSet && encSet || env != "production" {
		return jwtSecret, encKey
	}
	s, err := loadOrCreateSecrets(storageDir)
	if err != nil {
		slog.Error("cannot persist generated secrets; set AUTOLEDGER_JWT_SECRET and AUTOLEDGER_ENCRYPTION_KEY or make the storage directory writable",
			"component", "security", "dir", storageDir, "error", err)
		return jwtSecret, encKey
	}
	if !jwtSet {
		jwtSecret = s.JWTSecret
	}
	if !encSet {
		encKey = s.EncryptionKey
	}
	return jwtSecret, encKey
}
