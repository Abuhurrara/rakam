package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL  string
	JWTSecret    string
	Port         string
	SeedEmail    string
	SeedPassword string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		Port:         os.Getenv("PORT"),
		SeedEmail:    os.Getenv("SEED_EMAIL"),
		SeedPassword: os.Getenv("SEED_PASSWORD"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if cfg.Port == "" {
		missing = append(missing, "PORT")
	}
	if cfg.SeedEmail == "" {
		missing = append(missing, "SEED_EMAIL")
	}
	if cfg.SeedPassword == "" {
		missing = append(missing, "SEED_PASSWORD")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %v", missing)
	}

	return cfg, nil
}
