// require.go provides small helpers used by HTTP handlers and services to
// enforce permissions. They return *httperror.Error so handlers can pass
// them to httperror.Render directly.
package auth

import (
	"context"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// Require verifies the user holds the permission.
func Require(ctx context.Context, perm string) error {
	u, ok := FromContext(ctx)
	if !ok {
		return httperror.New(httperror.CodeUnauthorized, "not authenticated")
	}
	if u.Has(perm) {
		return nil
	}
	return httperror.New(httperror.CodeForbidden, "missing permission: "+perm)
}

// RequireAnyOf verifies the user holds at least one of the permissions.
func RequireAnyOf(ctx context.Context, perms ...string) error {
	u, ok := FromContext(ctx)
	if !ok {
		return httperror.New(httperror.CodeUnauthorized, "not authenticated")
	}
	for _, p := range perms {
		if u.Has(p) {
			return nil
		}
	}
	return httperror.New(httperror.CodeForbidden, "missing permission")
}
