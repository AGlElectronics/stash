package http

import (
	"net/http"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
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
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := auth.RequireAny(r.Context(), perm); err != nil {
				httperror.Render(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
