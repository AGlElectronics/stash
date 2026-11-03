package service

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// mapPgError translates a Postgres error into a typed httperror.
//   - 23505 (unique_violation)         -> 409 Conflict
//   - 23503 (foreign_key_violation)    -> 409 Conflict (referenced or referencing)
//   - 23514 (check_violation)          -> 422 Validation
//   - 23502 (not_null_violation)       -> 422 Validation
//
// Anything else falls through as Internal(err).
func mapPgError(err error, _ string, conflictMsg string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return httperror.Internal(err)
	}
	switch pgErr.Code {
	case "23505":
		msg := conflictMsg
		if msg == "" {
			msg = "unique constraint violated"
		}
		return httperror.Conflict(msg)
	case "23503":
		return httperror.Conflict("referenced or referencing record exists")
	case "23514":
		return httperror.Validation("check constraint failed", map[string]any{"constraint": pgErr.ConstraintName})
	case "23502":
		return httperror.Validation("required field is null", map[string]any{"column": pgErr.ColumnName})
	default:
		return httperror.Internal(err)
	}
}
