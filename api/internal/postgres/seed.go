package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/Abuhurrara/rakam/api/internal/config"
	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type seedCategory struct {
	name  string
	icon  string
	color string
}

var seedExpenseCategories = []seedCategory{
	{"Food", "🍔", "#8B4513"},
	{"Groceries", "🛒", "#2E7D32"},
	{"Transport", "🚗", "#455A64"},
	{"Rent", "🏠", "#5D4037"},
	{"Bills & Utilities", "💡", "#F9A825"},
	{"Mobile & Internet", "📱", "#0277BD"},
	{"Shopping", "🛍️", "#AD1457"},
	{"Health", "🩺", "#C62828"},
	{"Travel", "✈️", "#00838F"},
	{"Entertainment", "🎬", "#6A1B9A"},
	{"Family", "👨‍👩‍👧", "#EF6C00"},
	{"Other", "🔖", "#616161"},
}

var seedIncomeCategories = []seedCategory{
	{"Salary", "💰", "#2E7D32"},
	{"Freelance", "💻", "#00695C"},
	{"Other", "🔖", "#616161"},
}

var seedPeople = []string{"Usman", "Moiz", "Talha", "Ali", "Baba", "SCB Rent"}

func Seed(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing seed password: %w", err)
	}

	name, _, _ := strings.Cut(cfg.SeedEmail, "@")

	var userID string
	err = pool.QueryRow(ctx, `
		insert into users (email, password_hash, name)
		values ($1, $2, $3)
		on conflict (email) do update set email = excluded.email
		returning id
	`, cfg.SeedEmail, string(hash), name).Scan(&userID)
	if err != nil {
		return fmt.Errorf("seeding user: %w", err)
	}

	if err := seedCategories(ctx, pool, userID, domain.KindExpense, seedExpenseCategories); err != nil {
		return err
	}
	if err := seedCategories(ctx, pool, userID, domain.KindIncome, seedIncomeCategories); err != nil {
		return err
	}

	for _, personName := range seedPeople {
		if _, err := pool.Exec(ctx, `
			insert into people (user_id, name)
			values ($1, $2)
			on conflict (user_id, name) do nothing
		`, userID, personName); err != nil {
			return fmt.Errorf("seeding person %q: %w", personName, err)
		}
	}

	return nil
}

func seedCategories(ctx context.Context, pool *pgxpool.Pool, userID string, kind domain.Kind, categories []seedCategory) error {
	for i, c := range categories {
		if _, err := pool.Exec(ctx, `
			insert into categories (user_id, name, kind, icon, color, sort_order)
			values ($1, $2, $3, $4, $5, $6)
			on conflict (user_id, name, kind) do nothing
		`, userID, c.name, kind, c.icon, c.color, i); err != nil {
			return fmt.Errorf("seeding category %q: %w", c.name, err)
		}
	}
	return nil
}
