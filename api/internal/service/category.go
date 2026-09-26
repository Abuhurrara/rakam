package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

type CategoryService struct {
	repo port.CategoryRepo
}

func NewCategoryService(repo port.CategoryRepo) *CategoryService {
	return &CategoryService{repo: repo}
}

func (s *CategoryService) List(ctx context.Context, userID string) ([]domain.Category, error) {
	categories, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	return categories, nil
}

func (s *CategoryService) Create(ctx context.Context, c domain.Category) (domain.Category, error) {
	c.Name = strings.TrimSpace(c.Name)
	if err := validateCategory(c); err != nil {
		return domain.Category{}, err
	}
	if err := s.ensureUniqueName(ctx, c, ""); err != nil {
		return domain.Category{}, err
	}

	created, err := s.repo.Create(ctx, c)
	if err != nil {
		return domain.Category{}, fmt.Errorf("creating category: %w", err)
	}
	return created, nil
}

func (s *CategoryService) Update(ctx context.Context, c domain.Category) (domain.Category, error) {
	c.Name = strings.TrimSpace(c.Name)
	if err := validateCategory(c); err != nil {
		return domain.Category{}, err
	}
	current, err := s.repo.Get(ctx, c.UserID, c.ID)
	if err != nil {
		return domain.Category{}, fmt.Errorf("loading category for update: %w", err)
	}
	if current.Kind != c.Kind {
		return domain.Category{}, fmt.Errorf("%w: category type cannot be changed", domain.ErrInvalidCategory)
	}
	if err := s.ensureUniqueName(ctx, c, c.ID); err != nil {
		return domain.Category{}, err
	}

	updated, err := s.repo.Update(ctx, c)
	if err != nil {
		return domain.Category{}, fmt.Errorf("updating category: %w", err)
	}
	return updated, nil
}

func (s *CategoryService) Archive(ctx context.Context, userID, id string) error {
	if err := s.repo.Archive(ctx, userID, id); err != nil {
		return fmt.Errorf("archiving category: %w", err)
	}
	return nil
}

func (s *CategoryService) Restore(ctx context.Context, userID, id string) (domain.Category, error) {
	category, err := s.repo.Restore(ctx, userID, id)
	if err != nil {
		return domain.Category{}, fmt.Errorf("restoring category: %w", err)
	}
	return category, nil
}

func (s *CategoryService) ensureUniqueName(ctx context.Context, candidate domain.Category, exceptID string) error {
	categories, err := s.repo.List(ctx, candidate.UserID)
	if err != nil {
		return fmt.Errorf("checking category name: %w", err)
	}
	for _, existing := range categories {
		if existing.ID != exceptID && existing.Kind == candidate.Kind && strings.EqualFold(existing.Name, candidate.Name) {
			return domain.ErrDuplicateCategory
		}
	}
	return nil
}

func validateCategory(c domain.Category) error {
	if c.Name == "" {
		return fmt.Errorf("%w: name is required", domain.ErrInvalidCategory)
	}
	if utf8.RuneCountInString(c.Name) > 40 {
		return fmt.Errorf("%w: name must be 40 characters or fewer", domain.ErrInvalidCategory)
	}
	if c.Kind != domain.KindExpense && c.Kind != domain.KindIncome {
		return fmt.Errorf("%w: kind must be expense or income", domain.ErrInvalidCategory)
	}
	if strings.TrimSpace(c.Icon) == "" || utf8.RuneCountInString(c.Icon) > 16 {
		return fmt.Errorf("%w: choose a valid icon", domain.ErrInvalidCategory)
	}
	if !categoryColorPattern.MatchString(c.Color) {
		return fmt.Errorf("%w: color must be a six-digit hex value", domain.ErrInvalidCategory)
	}
	if c.SortOrder < 0 {
		return fmt.Errorf("%w: sort_order cannot be negative", domain.ErrInvalidCategory)
	}
	return nil
}

var categoryColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
