package config

import "testing"

func TestLoadDoesNotRequireLegacySeedCredentials(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "this-is-a-test-secret-at-least-32-bytes")
	t.Setenv("PORT", "8091")
	t.Setenv("SEED_EMAIL", "")
	t.Setenv("SEED_PASSWORD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("loading runtime config without seed credentials: %v", err)
	}
	if cfg.SeedEmail != "" || cfg.SeedPassword != "" {
		t.Fatal("unexpected seed credentials in config")
	}
}
