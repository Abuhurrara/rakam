package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"

	"github.com/Abuhurrara/rakam/api/internal/config"
	"github.com/Abuhurrara/rakam/api/internal/httpapi"
	"github.com/Abuhurrara/rakam/api/internal/postgres"
	"github.com/Abuhurrara/rakam/api/internal/service"
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

	seedUserID, err := postgres.SoleUserID(ctx, pool)
	if err != nil {
		log.Fatalf("resolving seeded user (did you run `make seed`?): %v", err)
	}

	categoryRepo := postgres.NewCategoryRepo(pool)
	categorySvc := service.NewCategoryService(categoryRepo)

	router := httpapi.NewRouter(categorySvc, pool, seedUserID)

	addr := "0.0.0.0:" + cfg.Port
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
