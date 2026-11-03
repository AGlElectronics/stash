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

type LocationService struct {
	pool *pgxpool.Pool
}

func NewLocationService(pool *pgxpool.Pool) *LocationService {
	return &LocationService{pool: pool}
}

type CreateLocationInput struct {
	ParentID    *uuid.UUID
	Name        string
	Description string
}

type UpdateLocationInput struct {
	ParentID    *uuid.UUID
	ClearParent bool
	Name        *string
	Description *string
}

func (s *LocationService) List(ctx context.Context) ([]sqlcgen.Location, error) {
	q := sqlcgen.New(s.pool)
	return q.ListLocations(ctx)
}

func (s *LocationService) Get(ctx context.Context, id uuid.UUID) (sqlcgen.Location, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetLocation(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.Location{}, httperror.NotFound("location")
		}
		return sqlcgen.Location{}, httperror.Internal(err)
	}
	return row, nil
}

func (s *LocationService) Create(ctx context.Context, in CreateLocationInput) (sqlcgen.Location, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return sqlcgen.Location{}, httperror.Validation("name is required", nil)
	}
	var out sqlcgen.Location
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		if in.ParentID != nil {
			if _, err := qtx.GetLocation(ctx, *in.ParentID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return httperror.Validation("parent location does not exist", nil)
				}
				return httperror.Internal(err)
			}
		}
		row, err := qtx.CreateLocation(ctx, sqlcgen.CreateLocationParams{
			ParentID:    in.ParentID,
			Name:        in.Name,
			Description: in.Description,
		})
		if err != nil {
			return mapPgError(err, "location", "name already exists under that parent")
		}
		out = row
		return audit.Record(ctx, qtx, "location.create", "location", row.ID, nil, row, "")
	})
	return out, err
}

func (s *LocationService) Update(ctx context.Context, id uuid.UUID, in UpdateLocationInput) (sqlcgen.Location, error) {
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if v == "" {
			return sqlcgen.Location{}, httperror.Validation("name cannot be empty", nil)
		}
		in.Name = &v
	}
	var out sqlcgen.Location
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetLocation(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("location")
			}
			return httperror.Internal(err)
		}
		var parentArg *uuid.UUID
		if in.ClearParent {
			parentArg = nil
		} else if in.ParentID != nil {
			if *in.ParentID == id {
				return httperror.Validation("location cannot be its own parent", nil)
			}
			if err := assertNoLocationCycle(ctx, qtx, id, *in.ParentID); err != nil {
				return err
			}
			parentArg = in.ParentID
		} else {
			parentArg = before.ParentID
		}
		row, err := qtx.UpdateLocation(ctx, sqlcgen.UpdateLocationParams{
			ID:          id,
			ParentID:    parentArg,
			Name:        in.Name,
			Description: in.Description,
		})
		if err != nil {
			return mapPgError(err, "location", "name already exists under that parent")
		}
		out = row
		return audit.Record(ctx, qtx, "location.update", "location", id, before, row, "")
	})
	return out, err
}

// assertNoLocationCycle ensures newParent is not `locationID` and does not have
// `locationID` in its ancestor chain (prevents hierarchical cycles).
func assertNoLocationCycle(ctx context.Context, qtx *sqlcgen.Queries, locationID, newParent uuid.UUID) error {
	cur := newParent
	const maxDepth = 10000
	for i := 0; i < maxDepth; i++ {
		if cur == locationID {
			return httperror.Validation("location parent would create a cycle", nil)
		}
		row, err := qtx.GetLocation(ctx, cur)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return httperror.Internal(err)
		}
		if row.ParentID == nil {
			return nil
		}
		cur = *row.ParentID
	}
	return httperror.Validation("location hierarchy is too deep", nil)
}

func (s *LocationService) Delete(ctx context.Context, id uuid.UUID) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetLocation(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("location")
			}
			return httperror.Internal(err)
		}
		if err := qtx.DeleteLocation(ctx, id); err != nil {
			return mapPgError(err, "location", "")
		}
		return audit.Record(ctx, qtx, "location.delete", "location", id, before, nil, "")
	})
}
