package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// deptResolver is the subset of sqlcgen.Querier used by resolveDepartmentRefTx.
type deptResolver interface {
	GetDepartmentByName(ctx context.Context, lower string) (sqlcgen.Department, error)
	CreateDepartment(ctx context.Context, arg sqlcgen.CreateDepartmentParams) (sqlcgen.Department, error)
}

// categoryResolver is the subset of sqlcgen.Querier used by resolveCategoryRefTx.
type categoryResolver interface {
	GetCategoryByName(ctx context.Context, lower string) (sqlcgen.ItemCategory, error)
	CreateCategory(ctx context.Context, arg sqlcgen.CreateCategoryParams) (sqlcgen.ItemCategory, error)
}

// locationResolver is the subset of sqlcgen.Querier used by resolveLocationRefTx.
type locationResolver interface {
	GetLocationByName(ctx context.Context, lower string) (sqlcgen.Location, error)
	CreateLocation(ctx context.Context, arg sqlcgen.CreateLocationParams) (sqlcgen.Location, error)
}

// resolveDepartmentRefTx returns id when set, otherwise finds or creates by name.
func resolveDepartmentRefTx(ctx context.Context, q deptResolver, id *uuid.UUID, name string) (*uuid.UUID, error) {
	if id != nil {
		return id, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	row, err := q.GetDepartmentByName(ctx, name)
	if err == nil {
		return &row.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, httperror.Internal(err)
	}
	created, err := q.CreateDepartment(ctx, sqlcgen.CreateDepartmentParams{
		Name:        name,
		Description: "",
	})
	if err != nil {
		if row, retryErr := q.GetDepartmentByName(ctx, name); retryErr == nil {
			return &row.ID, nil
		}
		return nil, mapPgError(err, "department", "name already exists")
	}
	return &created.ID, nil
}

func resolveCategoryRefTx(ctx context.Context, q categoryResolver, id *uuid.UUID, name string) (*uuid.UUID, error) {
	if id != nil {
		return id, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	row, err := q.GetCategoryByName(ctx, name)
	if err == nil {
		return &row.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, httperror.Internal(err)
	}
	created, err := q.CreateCategory(ctx, sqlcgen.CreateCategoryParams{
		Name:        name,
		Description: "",
	})
	if err != nil {
		if row, retryErr := q.GetCategoryByName(ctx, name); retryErr == nil {
			return &row.ID, nil
		}
		return nil, mapPgError(err, "category", "name already exists")
	}
	return &created.ID, nil
}

func resolveLocationRefTx(ctx context.Context, q locationResolver, id *uuid.UUID, name string) (*uuid.UUID, error) {
	if id != nil {
		return id, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	row, err := q.GetLocationByName(ctx, name)
	if err == nil {
		return &row.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, httperror.Internal(err)
	}
	created, err := q.CreateLocation(ctx, sqlcgen.CreateLocationParams{
		ParentID:    nil,
		Name:        name,
		Description: "",
	})
	if err != nil {
		if row, retryErr := q.GetLocationByName(ctx, name); retryErr == nil {
			return &row.ID, nil
		}
		return nil, mapPgError(err, "location", "name already exists")
	}
	return &created.ID, nil
}
