package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

// foreignKeyViolation is Postgres's SQLSTATE for a foreign key constraint
// violation (23503). debt_entries.person_id references people(id) with no
// ON DELETE cascade, so this is what a delete hits if a debt entry still
// points at the person — the service is expected to check first via
// DebtRepo.HasAny, but this is the authoritative backstop against a race.
const foreignKeyViolation = "23503"

type PersonRepo struct {
	pool *pgxpool.Pool
}

func NewPersonRepo(pool *pgxpool.Pool) *PersonRepo {
	return &PersonRepo{pool: pool}
}

// List returns every person for userID with their net balance computed in
// this one query — a left join against debt_entries with two independently
// coalesced sums, one per direction, subtracted. Coalescing each sum on its
// own (rather than the difference) is what makes a person with zero debt
// entries come back with balance 0 instead of NULL, and there is no
// per-person query in a loop.
func (r *PersonRepo) List(ctx context.Context, userID string) ([]domain.PersonBalance, error) {
	rows, err := r.pool.Query(ctx, `
		select p.id, p.user_id, p.name, p.phone, p.notes, p.created_at, p.updated_at,
		       coalesce(sum(case when d.direction = 'they_owe' and d.settled_at is null then d.amount_paisa end), 0)
		     - coalesce(sum(case when d.direction = 'i_owe'   and d.settled_at is null then d.amount_paisa end), 0)
		       as balance_paisa
		from people p
		left join debt_entries d on d.person_id = p.id and d.user_id = p.user_id
		where p.user_id = $1
		group by p.id
		order by p.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying people: %w", err)
	}
	defer rows.Close()

	var people []domain.PersonBalance
	for rows.Next() {
		var pb domain.PersonBalance
		if err := rows.Scan(&pb.Person.ID, &pb.Person.UserID, &pb.Person.Name, &pb.Person.Phone, &pb.Person.Notes, &pb.Person.CreatedAt, &pb.Person.UpdatedAt, &pb.BalancePaisa); err != nil {
			return nil, fmt.Errorf("scanning person: %w", err)
		}
		people = append(people, pb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating people: %w", err)
	}
	return people, nil
}

func (r *PersonRepo) Get(ctx context.Context, userID, id string) (domain.Person, error) {
	var p domain.Person
	err := r.pool.QueryRow(ctx, `
		select id, user_id, name, phone, notes, created_at, updated_at
		from people
		where id = $1 and user_id = $2
	`, id, userID).Scan(&p.ID, &p.UserID, &p.Name, &p.Phone, &p.Notes, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, fmt.Errorf("querying person: %w", err)
	}
	return p, nil
}

func (r *PersonRepo) Create(ctx context.Context, p domain.Person) (domain.Person, error) {
	err := r.pool.QueryRow(ctx, `
		insert into people (user_id, name, phone, notes)
		values ($1, $2, $3, $4)
		returning id, created_at, updated_at
	`, p.UserID, p.Name, p.Phone, p.Notes).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return domain.Person{}, fmt.Errorf("inserting person: %w", err)
	}
	return p, nil
}

func (r *PersonRepo) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `delete from people where id = $1 and user_id = $2`, id, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation {
			return domain.ErrPersonHasDebtEntries
		}
		return fmt.Errorf("deleting person: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
