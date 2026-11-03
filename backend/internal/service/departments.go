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

type DepartmentService struct {
	pool *pgxpool.Pool
}

func NewDepartmentService(pool *pgxpool.Pool) *DepartmentService {
	return &DepartmentService{pool: pool}
}

type CreateDepartmentInput struct {
	Name        string
	Description string
}

type UpdateDepartmentInput struct {
	Name        *string
	Description *string
}

func (s *DepartmentService) List(ctx context.Context) ([]sqlcgen.Department, error) {
	q := sqlcgen.New(s.pool)
	return q.ListDepartments(ctx)
}

func (s *DepartmentService) Get(ctx context.Context, id uuid.UUID) (sqlcgen.Department, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetDepartment(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.Department{}, httperror.NotFound("department")
		}
		return sqlcgen.Department{}, httperror.Internal(err)
	}
	return row, nil
}

func (s *DepartmentService) Create(ctx context.Context, in CreateDepartmentInput) (sqlcgen.Department, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return sqlcgen.Department{}, httperror.Validation("name is required", nil)
	}

	var out sqlcgen.Department
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		row, err := qtx.CreateDepartment(ctx, sqlcgen.CreateDepartmentParams{
			Name:        in.Name,
			Description: in.Description,
		})
		if err != nil {
			return mapPgError(err, "department", "name already exists")
		}
		out = row
		return audit.Record(ctx, qtx, "department.create", "department", row.ID, nil, row, "")
	})
	return out, err
}

func (s *DepartmentService) Update(ctx context.Context, id uuid.UUID, in UpdateDepartmentInput) (sqlcgen.Department, error) {
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if v == "" {
			return sqlcgen.Department{}, httperror.Validation("name cannot be empty", nil)
		}
		in.Name = &v
	}
	var out sqlcgen.Department
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetDepartment(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("department")
			}
			return httperror.Internal(err)
		}
		row, err := qtx.UpdateDepartment(ctx, sqlcgen.UpdateDepartmentParams{
			ID:          id,
			Name:        in.Name,
			Description: in.Description,
		})
		if err != nil {
			return mapPgError(err, "department", "name already exists")
		}
		out = row
		return audit.Record(ctx, qtx, "department.update", "department", id, before, row, "")
	})
	return out, err
}

func (s *DepartmentService) Delete(ctx context.Context, id uuid.UUID) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetDepartment(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("department")
			}
			return httperror.Internal(err)
		}
		if err := qtx.DeleteDepartment(ctx, id); err != nil {
			return mapPgError(err, "department", "")
		}
		return audit.Record(ctx, qtx, "department.delete", "department", id, before, nil, "")
	})
}
