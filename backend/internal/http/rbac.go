package http

import (
	"net/http"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// require returns chi middleware that enforces one permission.
func require(perm string) func(http.Handler) http.Handler {
	return requireAnyOf(perm)
}

// requireAnyOf returns chi middleware that accepts any one of the permissions.
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
