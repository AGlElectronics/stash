package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// FormerUser is the name an anonymised user is shown with everywhere.
const FormerUser = "Former user"

// UserService lists the people Stash has seen and anonymises those who
// left. A user row exists because someone signed in once; it holds their
// name and email so history can say who did what.
type UserService struct {
	pool *pgxpool.Pool
}

func NewUserService(pool *pgxpool.Pool) *UserService { return &UserService{pool: pool} }

// UserSummary is one row of GET /api/users.
type UserSummary struct {
	ID           uuid.UUID `json:"id"`
	DisplayName  string    `json:"display_name"`
	Email        string    `json:"email"`
	LastLoginAt  string    `json:"last_login_at,omitempty"`
	AnonymisedAt string    `json:"anonymised_at,omitempty"`
	CreatedAt    string    `json:"created_at"`
}

// List returns users ordered by name.
func (s *UserService) List(ctx context.Context, limit, offset int32) ([]UserSummary, int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, display_name, coalesce(email, ''), last_login_at, anonymised_at, created_at
		  FROM users ORDER BY anonymised_at IS NOT NULL, lower(display_name), id
		 LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, httperror.Internal(err)
	}
	defer rows.Close()
	out := []UserSummary{}
	for rows.Next() {
		var u UserSummary
		var last, anon, created pgtype.Timestamptz
		if err := rows.Scan(&u.ID, &u.DisplayName, &u.Email, &last, &anon, &created); err != nil {
			return nil, 0, httperror.Internal(err)
		}
		u.LastLoginAt, u.AnonymisedAt, u.CreatedAt = rfc3339(last), rfc3339(anon), rfc3339(created)
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, httperror.Internal(err)
	}
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, httperror.Internal(err)
	}
	return out, total, nil
}

func rfc3339(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// Anonymise removes a person's name, email and IdP subject from Stash: the
// user row and every audit and item-history entry they made now say
// "Former user". Their actions stay in the history. If they sign in again
// they get a new, separate user.
func (s *UserService) Anonymise(ctx context.Context, id uuid.UUID, note string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var anonymised pgtype.Timestamptz
		err := tx.QueryRow(ctx, `SELECT anonymised_at FROM users WHERE id = $1 FOR UPDATE`, id).Scan(&anonymised)
		if errors.Is(err, pgx.ErrNoRows) {
			return httperror.NotFound("user")
		}
		if err != nil {
			return httperror.Internal(err)
		}
		if anonymised.Valid {
			return httperror.Conflict("user is already anonymised")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE users SET display_name = $2, email = NULL, oidc_subject = NULL,
			       is_disabled = TRUE, anonymised_at = now(), updated_at = now()
			 WHERE id = $1`, id, FormerUser); err != nil {
			return httperror.Internal(err)
		}
		for _, table := range []string{"audit_log", "item_events"} {
			if _, err := tx.Exec(ctx, `UPDATE `+table+` SET actor_display_name = $2, actor_oidc_subject = NULL
			  WHERE actor_user_id = $1`, id, FormerUser); err != nil {
				return httperror.Internal(err)
			}
		}
		return audit.Record(ctx, sqlcgen.New(tx), "user.anonymise", "user", id, nil, nil, note)
	})
}

// AnonymiseInactive anonymises users who have not signed in for the given
// number of days (0 does nothing). It returns how many were anonymised.
func (s *UserService) AnonymiseInactive(ctx context.Context, days int, now time.Time) (int, error) {
	if days <= 0 {
		return 0, nil
	}
	cutoff := now.AddDate(0, 0, -days)
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM users
		 WHERE anonymised_at IS NULL AND coalesce(last_login_at, created_at) < $1`, cutoff)
	if err != nil {
		return 0, httperror.Internal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return 0, httperror.Internal(err)
	}
	for _, id := range ids {
		if err := s.Anonymise(ctx, id, "not signed in for "+strconv.Itoa(days)+" days"); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}
