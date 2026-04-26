package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr      string
	DatabaseURL   string
	MasterKey     []byte
	WebOrigin     string
	RequestTimout time.Duration
	OpenAITimeout time.Duration
}

const defaultMasterKeyFile = "/var/lib/tgtldr/master.key"

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:      env("TGTLDR_HTTP_ADDR", ":8080"),
		DatabaseURL:   env("TGTLDR_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/tgtldr?sslmode=disable"),
		WebOrigin:     env("TGTLDR_WEB_ORIGIN", "http://localhost:3000"),
		RequestTimout: envDuration("TGTLDR_REQUEST_TIMEOUT", 30*time.Second),
		OpenAITimeout: envDuration("TGTLDR_OPENAI_TIMEOUT", 3*time.Minute),
	}

	key, err := loadMasterKey()
	if err != nil {
		return Config{}, err
	}
	cfg.MasterKey = key

	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

func loadMasterKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("TGTLDR_MASTER_KEY"))
	if raw == "" {
		var err error
		raw, err = loadOrCreateMasterKeyFile(masterKeyFilePath())
		if err != nil {
			return nil, err
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode master key: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes after base64 decode, got %d", len(decoded))
	}
	return decoded, nil
}

func masterKeyFilePath() string {
	value := strings.TrimSpace(os.Getenv("TGTLDR_MASTER_KEY_FILE"))
	if value == "" {
		return defaultMasterKeyFile
	}
	return value
}

func loadOrCreateMasterKeyFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err == nil {
		raw := strings.TrimSpace(string(content))
		if raw == "" {
			return "", fmt.Errorf("master key file is empty: %s", path)
		}
		return raw, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read master key file: %w", err)
	}

	generated, err := generateEncodedMasterKey()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create master key directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(generated+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write master key file: %w", err)
	}
	return generated, nil
}

func generateEncodedMasterKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate master key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}
