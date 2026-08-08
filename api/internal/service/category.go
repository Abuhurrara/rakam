package service

import (
	"context"
	"fmt"
	"strings"

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
	if err := validateCategory(c); err != nil {
		return domain.Category{}, err
	}

	created, err := s.repo.Create(ctx, c)
	if err != nil {
		return domain.Category{}, fmt.Errorf("creating category: %w", err)
	}
	return created, nil
}

func (s *CategoryService) Update(ctx context.Context, c domain.Category) (domain.Category, error) {
	if err := validateCategory(c); err != nil {
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

func validateCategory(c domain.Category) error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("%w: name is required", domain.ErrInvalidCategory)
	}
	if c.Kind != domain.KindExpense && c.Kind != domain.KindIncome {
		return fmt.Errorf("%w: kind must be expense or income", domain.ErrInvalidCategory)
	}
	return nil
}
