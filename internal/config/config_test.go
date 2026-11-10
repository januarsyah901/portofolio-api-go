package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(env(map[string]string{
		"DATABASE_URL": "postgres://x",
		"JWT_SECRET":   strings.Repeat("a", 32),
		"ADMIN_EMAILS": " A@B.com , c@d.com ,",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 3001 || cfg.PublicDir != "./public" || cfg.LogFormat != "text" || cfg.AppEnv != "development" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if got := strings.Join(cfg.AdminEmails, ","); got != "a@b.com,c@d.com" {
		t.Errorf("AdminEmails = %q", got)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "http://localhost:5173" {
		t.Errorf("CORSOrigins = %v", cfg.CORSOrigins)
	}
}

func TestLoadRejectsWeakSecret(t *testing.T) {
	_, err := load(env(map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": "short"}))
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected JWT_SECRET error, got %v", err)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	_, err := load(env(map[string]string{"JWT_SECRET": strings.Repeat("a", 32)}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestProductionHasNoDefaultCORS(t *testing.T) {
	cfg, err := load(env(map[string]string{
		"DATABASE_URL": "postgres://x",
		"JWT_SECRET":   strings.Repeat("a", 32),
		"APP_ENV":      "production",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CORSOrigins) != 0 {
		t.Errorf("production should not default CORS origins, got %v", cfg.CORSOrigins)
	}
}
