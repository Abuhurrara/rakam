package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type fakeCategoryRepo struct {
	categories map[string]domain.Category
	nextID     int
}

func newFakeCategoryRepo() *fakeCategoryRepo {
	return &fakeCategoryRepo{categories: make(map[string]domain.Category)}
}

func (f *fakeCategoryRepo) List(ctx context.Context, userID string) ([]domain.Category, error) {
	var result []domain.Category
	for _, c := range f.categories {
		if c.UserID == userID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (f *fakeCategoryRepo) Get(ctx context.Context, userID, id string) (domain.Category, error) {
	existing, ok := f.categories[id]
	if !ok || existing.UserID != userID {
		return domain.Category{}, domain.ErrNotFound
	}
	return existing, nil
}

func (f *fakeCategoryRepo) Create(ctx context.Context, c domain.Category) (domain.Category, error) {
	f.nextID++
	c.ID = fmt.Sprintf("cat-%d", f.nextID)
	f.categories[c.ID] = c
	return c, nil
}

func (f *fakeCategoryRepo) Update(ctx context.Context, c domain.Category) (domain.Category, error) {
	existing, ok := f.categories[c.ID]
	if !ok || existing.UserID != c.UserID {
		return domain.Category{}, domain.ErrNotFound
	}
	c.IsArchived = existing.IsArchived
	f.categories[c.ID] = c
	return c, nil
}

func (f *fakeCategoryRepo) Archive(ctx context.Context, userID, id string) error {
	existing, ok := f.categories[id]
	if !ok || existing.UserID != userID {
		return domain.ErrNotFound
	}
	existing.IsArchived = true
	f.categories[id] = existing
	return nil
}

func (f *fakeCategoryRepo) Restore(ctx context.Context, userID, id string) (domain.Category, error) {
	existing, ok := f.categories[id]
	if !ok || existing.UserID != userID {
		return domain.Category{}, domain.ErrNotFound
	}
	existing.IsArchived = false
	f.categories[id] = existing
	return existing, nil
}

func validCategory(userID, name string, kind domain.Kind) domain.Category {
	return domain.Category{UserID: userID, Name: name, Kind: kind, Icon: "🏷️", Color: "#2E7D32"}
}

func TestCategoryService_Create(t *testing.T) {
	tests := []struct {
		name    string
		input   domain.Category
		wantErr error
	}{
		{
			name:  "valid expense category",
			input: validCategory("user-1", "Food", domain.KindExpense),
		},
		{
			name:  "valid income category",
			input: validCategory("user-1", "Salary", domain.KindIncome),
		},
		{
			name:    "empty name",
			input:   validCategory("user-1", "  ", domain.KindExpense),
			wantErr: domain.ErrInvalidCategory,
		},
		{
			name:    "invalid kind",
			input:   validCategory("user-1", "Food", "bogus"),
			wantErr: domain.ErrInvalidCategory,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeCategoryRepo()
			svc := NewCategoryService(repo)

			created, err := svc.Create(context.Background(), tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if created.ID == "" {
				t.Fatal("expected created category to have an ID")
			}
			if created.Name != tt.input.Name {
				t.Fatalf("got name %q, want %q", created.Name, tt.input.Name)
			}
		})
	}
}

func TestCategoryService_List_ScopesToUser(t *testing.T) {
	repo := newFakeCategoryRepo()
	svc := NewCategoryService(repo)
	ctx := context.Background()

	if _, err := svc.Create(ctx, validCategory("user-1", "Food", domain.KindExpense)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.Create(ctx, validCategory("user-2", "Rent", domain.KindExpense)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := svc.List(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Food" {
		t.Fatalf("got %+v, want only user-1's Food category", got)
	}
}

func TestCategoryService_Update_NotFoundForOtherUser(t *testing.T) {
	repo := newFakeCategoryRepo()
	svc := NewCategoryService(repo)
	ctx := context.Background()

	created, err := svc.Create(ctx, validCategory("user-1", "Food", domain.KindExpense))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := validCategory("user-2", "Hijacked", domain.KindExpense)
	updated.ID = created.ID
	_, err = svc.Update(ctx, updated)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got error %v, want %v", err, domain.ErrNotFound)
	}
}

func TestCategoryService_Archive(t *testing.T) {
	repo := newFakeCategoryRepo()
	svc := NewCategoryService(repo)
	ctx := context.Background()

	created, err := svc.Create(ctx, validCategory("user-1", "Food", domain.KindExpense))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.Archive(ctx, "user-1", created.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.categories[created.ID].IsArchived {
		t.Fatal("expected category to be archived")
	}
}

func TestCategoryService_CreateRejectsDuplicateNamesIgnoringCase(t *testing.T) {
	repo := newFakeCategoryRepo()
	svc := NewCategoryService(repo)
	ctx := context.Background()
	if _, err := svc.Create(ctx, validCategory("user-1", "Food", domain.KindExpense)); err != nil {
		t.Fatalf("creating first category: %v", err)
	}
	if _, err := svc.Create(ctx, validCategory("user-1", " food ", domain.KindExpense)); !errors.Is(err, domain.ErrDuplicateCategory) {
		t.Fatalf("creating duplicate category returned %v, want ErrDuplicateCategory", err)
	}
	if _, err := svc.Create(ctx, validCategory("user-1", "Food", domain.KindIncome)); err != nil {
		t.Fatalf("same name in another kind should be allowed: %v", err)
	}
}

func TestCategoryService_UpdateCannotChangeKind(t *testing.T) {
	repo := newFakeCategoryRepo()
	svc := NewCategoryService(repo)
	ctx := context.Background()
	created, err := svc.Create(ctx, validCategory("user-1", "Food", domain.KindExpense))
	if err != nil {
		t.Fatalf("creating category: %v", err)
	}
	updated := validCategory("user-1", "Salary", domain.KindIncome)
	updated.ID = created.ID
	if _, err := svc.Update(ctx, updated); !errors.Is(err, domain.ErrInvalidCategory) {
		t.Fatalf("changing category kind returned %v, want ErrInvalidCategory", err)
	}
}

func TestCategoryService_Restore(t *testing.T) {
	repo := newFakeCategoryRepo()
	svc := NewCategoryService(repo)
	ctx := context.Background()
	created, err := svc.Create(ctx, validCategory("user-1", "Food", domain.KindExpense))
	if err != nil {
		t.Fatalf("creating category: %v", err)
	}
	if err := svc.Archive(ctx, "user-1", created.ID); err != nil {
		t.Fatalf("archiving category: %v", err)
	}
	restored, err := svc.Restore(ctx, "user-1", created.ID)
	if err != nil || restored.IsArchived {
		t.Fatalf("Restore() = (%+v, %v), want unarchived category", restored, err)
	}
}
