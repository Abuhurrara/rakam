package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type ImportRepo struct{ pool *pgxpool.Pool }

func NewImportRepo(pool *pgxpool.Pool) *ImportRepo { return &ImportRepo{pool: pool} }

func (r *ImportRepo) CreatePairingCode(ctx context.Context, userID, codeHash string, expiresAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning import pairing: %w", err)
	}
	defer tx.Rollback(ctx)
	var lockedUserID string
	if err := tx.QueryRow(ctx, `select id from users where id=$1 for update`, userID).Scan(&lockedUserID); err != nil {
		return fmt.Errorf("locking import pairing account: %w", err)
	}
	if _, err := tx.Exec(ctx, `delete from import_pairing_codes where user_id=$1`, userID); err != nil {
		return fmt.Errorf("replacing import pairing code: %w", err)
	}
	_, err = tx.Exec(ctx, `insert into import_pairing_codes (user_id, code_hash, expires_at) values ($1,$2,$3)`, userID, codeHash, expiresAt)
	if err != nil {
		return fmt.Errorf("creating import pairing code: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing import pairing code: %w", err)
	}
	return nil
}

func (r *ImportRepo) RedeemPairingCode(ctx context.Context, codeHash, tokenHash, name string) (domain.ImportDevice, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ImportDevice{}, fmt.Errorf("beginning device pairing: %w", err)
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `delete from import_pairing_codes where code_hash=$1 and expires_at>now() returning user_id`, codeHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ImportDevice{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return domain.ImportDevice{}, fmt.Errorf("redeeming pairing code: %w", err)
	}
	d := domain.ImportDevice{UserID: userID, Name: name}
	err = tx.QueryRow(ctx, `insert into import_devices (user_id,token_hash,name) values ($1,$2,$3) returning id,created_at`, userID, tokenHash, name).Scan(&d.ID, &d.CreatedAt)
	if err != nil {
		return domain.ImportDevice{}, fmt.Errorf("inserting import device: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.ImportDevice{}, fmt.Errorf("committing device pairing: %w", err)
	}
	return d, nil
}

func (r *ImportRepo) AuthenticateDevice(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.pool.QueryRow(ctx, `select user_id from import_devices where token_hash=$1 and revoked_at is null`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrUnauthorized
	}
	if err != nil {
		return "", fmt.Errorf("authenticating import device: %w", err)
	}
	return userID, nil
}

func (r *ImportRepo) ListDevices(ctx context.Context, userID string) ([]domain.ImportDevice, error) {
	rows, err := r.pool.Query(ctx, `select id,user_id,name,created_at,last_seen_at from import_devices where user_id=$1 and revoked_at is null order by created_at desc`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying import devices: %w", err)
	}
	defer rows.Close()
	items := []domain.ImportDevice{}
	for rows.Next() {
		var d domain.ImportDevice
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.CreatedAt, &d.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scanning import device: %w", err)
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating import devices: %w", err)
	}
	return items, nil
}

func (r *ImportRepo) RevokeDevice(ctx context.Context, userID, deviceID string) error {
	tag, err := r.pool.Exec(ctx, `update import_devices set revoked_at=now() where id=$1 and user_id=$2 and revoked_at is null`, deviceID, userID)
	if err != nil {
		return fmt.Errorf("revoking import device: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const importColumns = `id,user_id,source,source_id,kind,amount_paisa,occurred_at,description,status,transaction_id,created_at,updated_at`

type importScanner interface{ Scan(dest ...any) error }

func scanImport(row importScanner) (domain.ImportSuggestion, error) {
	var s domain.ImportSuggestion
	err := row.Scan(&s.ID, &s.UserID, &s.Source, &s.SourceID, &s.Kind, &s.AmountPaisa, &s.OccurredAt, &s.Description, &s.Status, &s.TransactionID, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func (r *ImportRepo) InsertSuggestion(ctx context.Context, s domain.ImportSuggestion) (domain.ImportSuggestion, error) {
	row := r.pool.QueryRow(ctx, `insert into import_suggestions (user_id,source,source_id,kind,amount_paisa,occurred_at,description) values ($1,$2,$3,$4,$5,$6,$7) on conflict (user_id,source,source_id) do nothing returning `+importColumns, s.UserID, s.Source, s.SourceID, s.Kind, s.AmountPaisa, s.OccurredAt, s.Description)
	created, err := scanImport(row)
	if errors.Is(err, pgx.ErrNoRows) {
		created, err = scanImport(r.pool.QueryRow(ctx, `select `+importColumns+` from import_suggestions where user_id=$1 and source=$2 and source_id=$3`, s.UserID, s.Source, s.SourceID))
	}
	if err != nil {
		return domain.ImportSuggestion{}, fmt.Errorf("inserting import suggestion: %w", err)
	}
	return created, nil
}

func (r *ImportRepo) ListSuggestions(ctx context.Context, userID string) ([]domain.ImportSuggestion, error) {
	rows, err := r.pool.Query(ctx, `select `+importColumns+` from import_suggestions where user_id=$1 and status='pending' order by occurred_at desc limit 100`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying import suggestions: %w", err)
	}
	defer rows.Close()
	items := []domain.ImportSuggestion{}
	for rows.Next() {
		s, err := scanImport(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning import suggestion: %w", err)
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating import suggestions: %w", err)
	}
	return items, nil
}

func (r *ImportRepo) IgnoreSuggestion(ctx context.Context, userID, id string) (domain.ImportSuggestion, error) {
	s, err := scanImport(r.pool.QueryRow(ctx, `update import_suggestions set status='ignored',updated_at=now() where id=$1 and user_id=$2 and status='pending' returning `+importColumns, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ImportSuggestion{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ImportSuggestion{}, fmt.Errorf("ignoring import suggestion: %w", err)
	}
	return s, nil
}

func (r *ImportRepo) ConfirmSuggestion(ctx context.Context, t domain.Transaction, suggestionID string) (domain.ImportSuggestion, domain.Transaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ImportSuggestion{}, domain.Transaction{}, fmt.Errorf("beginning import confirmation: %w", err)
	}
	defer tx.Rollback(ctx)
	var status string
	err = tx.QueryRow(ctx, `select status from import_suggestions where id=$1 and user_id=$2 for update`, suggestionID, t.UserID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ImportSuggestion{}, domain.Transaction{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ImportSuggestion{}, domain.Transaction{}, fmt.Errorf("locking import suggestion: %w", err)
	}
	if status != "pending" {
		return domain.ImportSuggestion{}, domain.Transaction{}, domain.ErrImportAlreadyReviewed
	}
	err = tx.QueryRow(ctx, `insert into transactions (user_id,kind,amount_paisa,category_id,description,occurred_at) values ($1,$2,$3,$4,$5,$6) returning id,created_at,updated_at`, t.UserID, t.Kind, t.AmountPaisa, t.CategoryID, t.Description, t.OccurredAt).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return domain.ImportSuggestion{}, domain.Transaction{}, fmt.Errorf("creating imported transaction: %w", err)
	}
	s, err := scanImport(tx.QueryRow(ctx, `update import_suggestions set status='confirmed',transaction_id=$1,updated_at=now() where id=$2 and user_id=$3 returning `+importColumns, t.ID, suggestionID, t.UserID))
	if err != nil {
		return domain.ImportSuggestion{}, domain.Transaction{}, fmt.Errorf("confirming import suggestion: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.ImportSuggestion{}, domain.Transaction{}, fmt.Errorf("committing import confirmation: %w", err)
	}
	return s, t, nil
}
