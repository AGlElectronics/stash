package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// requireGlobal returns chi middleware that enforces a global RBAC permission.
func requireGlobal(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := auth.RequireGlobal(r.Context(), perm); err != nil {
				httperror.Render(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireAnyGlobal returns chi middleware that accepts any one global permission.
func requireAnyGlobal(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := auth.RequireAnyGlobal(r.Context(), perms...); err != nil {
				httperror.Render(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireAny returns chi middleware that enforces having a permission either
// globally or in any one department. Useful for read endpoints that the
// service layer further filters by department.
func requireAny(perm string) func(http.Handler) http.Handler {
	return requireAnyOf(perm)
}

// requireAnyOf accepts any one of the permissions, globally or in any department.
func requireAnyOf(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := auth.RequireAnyOf(r.Context(), perms...); err != nil {
				httperror.Render(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireItemPerm enforces perm globally or in the department that manages
// the item named by the {id} URL param.
func requireItemPerm(pool *pgxpool.Pool, perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			u, ok := auth.FromContext(ctx)
			if !ok {
				httperror.Render(w, r, httperror.New(httperror.CodeUnauthorized, "not authenticated"))
				return
			}
			if !u.HasGlobal(perm) {
				id, err := uuid.Parse(chi.URLParam(r, "id"))
				if err != nil {
					httperror.Render(w, r, httperror.BadRequest("invalid id"))
					return
				}
				item, err := sqlcgen.New(pool).GetItem(ctx, id)
				if errors.Is(err, pgx.ErrNoRows) {
					httperror.Render(w, r, httperror.NotFound("item"))
					return
				}
				if err != nil {
					httperror.Render(w, r, httperror.Internal(err))
					return
				}
				if err := auth.RequireIn(ctx, perm, item.ManagedByDepartmentID); err != nil {
					httperror.Render(w, r, err)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// fileEntityPerms maps a file's entity type to the permissions needed to see
// it and to attach or remove files on it.
var fileEntityPerms = map[string]struct{ view, write []string }{
	"item":      {view: []string{domain.PermItemView}, write: []string{domain.PermItemCreate, domain.PermItemEdit}},
	"lostfound": {view: []string{domain.PermLostfoundView}, write: []string{domain.PermLostfoundCreate, domain.PermLostfoundEdit}},
	"signout":   {view: []string{domain.PermSignoutView}, write: []string{domain.PermSignoutCreate, domain.PermSignoutReturn}},
}

// requireFileAccess checks access to files on an entity type. Writes also
// need file.upload.
func requireFileAccess(ctx context.Context, entityType string, write bool) error {
	perms, ok := fileEntityPerms[entityType]
	if !ok {
		return httperror.BadRequest("unsupported entity_type")
	}
	if !write {
		return auth.RequireAnyOf(ctx, perms.view...)
	}
	if err := auth.RequireAny(ctx, domain.PermFileUpload); err != nil {
		return err
	}
	return auth.RequireAnyOf(ctx, perms.write...)
}
