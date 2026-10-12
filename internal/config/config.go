package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env         string // "development" | "production"
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration
	// How long an Idempotency-Key is remembered. Spec says 24h.
	IdempotencyTTL time.Duration
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func Load() (Config, error) {
	c := Config{
		Env:            getenv("APP_ENV", "development"),
		Port:           getenv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		JWTTTL:         getenvDuration("JWT_TTL", 24*time.Hour),
		IdempotencyTTL: getenvDuration("IDEMPOTENCY_TTL", 24*time.Hour),
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	// allow plain seconds too, e.g. JWT_TTL=3600
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return def
}
