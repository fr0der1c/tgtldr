package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
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

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func loadMasterKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("TGTLDR_MASTER_KEY"))
	if raw == "" {
		raw = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
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
