package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"time"

	// The Dockerfile's runtime image is distroless and carries no system
	// tzdata, so time.LoadLocation("Asia/Karachi") would fail at startup on
	// Render even though it works fine on a dev machine. This embeds the
	// tzdata database in the binary so it works identically everywhere.
	_ "time/tzdata"

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

	secret := []byte(cfg.JWTSecret)

	loc, err := time.LoadLocation("Asia/Karachi")
	if err != nil {
		log.Fatalf("loading Asia/Karachi timezone: %v", err)
	}

	categoryRepo := postgres.NewCategoryRepo(pool)
	categorySvc := service.NewCategoryService(categoryRepo)

	transactionRepo := postgres.NewTransactionRepo(pool)
	transactionSvc := service.NewTransactionService(transactionRepo, categoryRepo, loc)

	userRepo := postgres.NewUserRepo(pool)
	authSvc := service.NewAuthService(userRepo, secret)

	personRepo := postgres.NewPersonRepo(pool)
	debtRepo := postgres.NewDebtRepo(pool)
	personSvc := service.NewPersonService(personRepo, debtRepo)
	debtSvc := service.NewDebtService(debtRepo, personRepo, categoryRepo)

	router := httpapi.NewRouter(categorySvc, transactionSvc, authSvc, personSvc, debtSvc, pool, secret)

	addr := "0.0.0.0:" + cfg.Port
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
