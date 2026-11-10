// Package config loads and validates runtime configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MinJWTSecretLen is the shortest JWT secret the server accepts.
const MinJWTSecretLen = 32

// Config holds every setting the API needs at runtime.
type Config struct {
	Port        int
	DatabaseURL string
	JWTSecret   string
	CORSOrigins []string
	AdminEmails []string
	PublicDir   string
	LogFormat   string // "text" or "json"
	AppEnv      string // "development" or "production"
}

// Load reads configuration from environment variables and validates it.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL: getenv("DATABASE_URL"),
		JWTSecret:   getenv("JWT_SECRET"),
		CORSOrigins: splitList(getenv("CORS_ORIGINS")),
		AdminEmails: lowerAll(splitList(getenv("ADMIN_EMAILS"))),
		PublicDir:   orDefault(getenv("PUBLIC_DIR"), "./public"),
		LogFormat:   orDefault(getenv("LOG_FORMAT"), "text"),
		AppEnv:      orDefault(getenv("APP_ENV"), "development"),
	}

	var errs []error

	port, err := strconv.Atoi(orDefault(getenv("PORT"), "3001"))
	if err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("PORT must be a number between 1 and 65535"))
	}
	cfg.Port = port

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if len(cfg.JWTSecret) < MinJWTSecretLen {
		errs = append(errs, fmt.Errorf("JWT_SECRET is required and must be at least %d characters", MinJWTSecretLen))
	}
	if cfg.LogFormat != "text" && cfg.LogFormat != "json" {
		errs = append(errs, errors.New(`LOG_FORMAT must be "text" or "json"`))
	}
	if cfg.AppEnv != "development" && cfg.AppEnv != "production" {
		errs = append(errs, errors.New(`APP_ENV must be "development" or "production"`))
	}
	if len(cfg.CORSOrigins) == 0 && cfg.AppEnv == "development" {
		cfg.CORSOrigins = []string{"http://localhost:5173"}
	}

	return cfg, errors.Join(errs...)
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func lowerAll(in []string) []string {
	for i := range in {
		in[i] = strings.ToLower(in[i])
	}
	return in
}
