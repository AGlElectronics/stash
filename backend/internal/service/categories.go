package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

type CategoryService struct {
	pool *pgxpool.Pool
}

func NewCategoryService(pool *pgxpool.Pool) *CategoryService {
	return &CategoryService{pool: pool}
}

type CreateCategoryInput struct {
	Name        string
	Description string
}

type UpdateCategoryInput struct {
	Name        *string
	Description *string
}

func (s *CategoryService) List(ctx context.Context) ([]sqlcgen.ItemCategory, error) {
	q := sqlcgen.New(s.pool)
	return q.ListCategories(ctx)
}

func (s *CategoryService) Get(ctx context.Context, id uuid.UUID) (sqlcgen.ItemCategory, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetCategory(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.ItemCategory{}, httperror.NotFound("category")
		}
		return sqlcgen.ItemCategory{}, httperror.Internal(err)
	}
	return row, nil
}

func (s *CategoryService) Create(ctx context.Context, in CreateCategoryInput) (sqlcgen.ItemCategory, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return sqlcgen.ItemCategory{}, httperror.Validation("name is required", nil)
	}
	var out sqlcgen.ItemCategory
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		row, err := qtx.CreateCategory(ctx, sqlcgen.CreateCategoryParams{
			Name:        in.Name,
			Description: in.Description,
		})
		if err != nil {
			return mapPgError(err, "category", "name already exists")
		}
		out = row
		return audit.Record(ctx, qtx, "category.create", "category", row.ID, nil, row, "")
	})
	return out, err
}

func (s *CategoryService) Update(ctx context.Context, id uuid.UUID, in UpdateCategoryInput) (sqlcgen.ItemCategory, error) {
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if v == "" {
			return sqlcgen.ItemCategory{}, httperror.Validation("name cannot be empty", nil)
		}
		in.Name = &v
	}
	var out sqlcgen.ItemCategory
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetCategory(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("category")
			}
			return httperror.Internal(err)
		}
		row, err := qtx.UpdateCategory(ctx, sqlcgen.UpdateCategoryParams{
			ID:          id,
			Name:        in.Name,
			Description: in.Description,
		})
		if err != nil {
			return mapPgError(err, "category", "name already exists")
		}
		out = row
		return audit.Record(ctx, qtx, "category.update", "category", id, before, row, "")
	})
	return out, err
}

func (s *CategoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetCategory(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("category")
			}
			return httperror.Internal(err)
		}
		if err := qtx.DeleteCategory(ctx, id); err != nil {
			return mapPgError(err, "category", "")
		}
		return audit.Record(ctx, qtx, "category.delete", "category", id, before, nil, "")
	})
}
