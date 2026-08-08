package main

import (
	"context"
	"log"

	"github.com/Abuhurrara/rakam/api/internal/config"
	"github.com/Abuhurrara/rakam/api/internal/postgres"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	if err := postgres.Seed(ctx, pool, cfg); err != nil {
		log.Fatalf("seeding: %v", err)
	}

	log.Println("seed complete")
}
