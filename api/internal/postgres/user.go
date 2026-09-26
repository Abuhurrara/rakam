package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx, `
		select id, email, password_hash, name, session_version, created_at, updated_at
		from users
		where lower(email) = $1
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.SessionVersion, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("querying user by email: %w", err)
	}
	return u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx, `
		select id, email, password_hash, name, session_version, created_at, updated_at
		from users
		where id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.SessionVersion, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("querying user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepo) GetSessionVersion(ctx context.Context, id string) (int, error) {
	var version int
	err := r.pool.QueryRow(ctx, `select session_version from users where id = $1`, id).Scan(&version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, fmt.Errorf("querying user session version: %w", err)
	}
	return version, nil
}

func (r *UserRepo) Create(ctx context.Context, email, name, passwordHash string) (domain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("beginning account creation: %w", err)
	}
	defer tx.Rollback(ctx)

	var u domain.User
	err = tx.QueryRow(ctx, `
		insert into users (email, password_hash, name)
		values ($1, $2, $3)
		returning id, email, password_hash, name, session_version, created_at, updated_at
	`, email, passwordHash, name).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.SessionVersion, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return domain.User{}, fmt.Errorf("creating user: %w", err)
	}
	if err := seedDefaultCategories(ctx, tx, u.ID); err != nil {
		return domain.User{}, fmt.Errorf("adding default categories to new account: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("committing account creation: %w", err)
	}
	return u, nil
}

func (r *UserRepo) UpdatePassword(ctx context.Context, id, passwordHash string) (int, error) {
	var version int
	err := r.pool.QueryRow(ctx, `
		update users
		set password_hash = $2, session_version = session_version + 1, updated_at = now()
		where id = $1
		returning session_version
	`, id, passwordHash).Scan(&version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, fmt.Errorf("updating user password: %w", err)
	}
	return version, nil
}
