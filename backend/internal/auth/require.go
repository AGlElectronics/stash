// require.go provides small helpers used by HTTP handlers to enforce RBAC
// permissions. They return *httperror.Error so handlers can pass them to
// httperror.Render directly.
package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// RequireAny verifies the user has the permission either globally or in at
// least one department. Use for read endpoints that filter results later.
func RequireAny(ctx context.Context, perm string) error {
	u, ok := FromContext(ctx)
	if !ok {
		return httperror.New(httperror.CodeUnauthorized, "not authenticated")
	}
	if u.HasAny(perm) {
		return nil
	}
	return httperror.New(httperror.CodeForbidden, "missing permission: "+perm)
}

// RequireGlobal verifies the user has the permission globally. Use for
// admin-only endpoints (e.g. managing roles, departments).
func RequireGlobal(ctx context.Context, perm string) error {
	u, ok := FromContext(ctx)
	if !ok {
		return httperror.New(httperror.CodeUnauthorized, "not authenticated")
	}
	if u.HasGlobal(perm) {
		return nil
	}
	return httperror.New(httperror.CodeForbidden, "missing permission: "+perm)
}

// RequireAnyGlobal verifies the user has at least one of the given permissions globally.
func RequireAnyGlobal(ctx context.Context, perms ...string) error {
	u, ok := FromContext(ctx)
	if !ok {
		return httperror.New(httperror.CodeUnauthorized, "not authenticated")
	}
	for _, p := range perms {
		if u.HasGlobal(p) {
			return nil
		}
	}
	return httperror.New(httperror.CodeForbidden, "missing permission")
}

// RequireIn verifies the user has the permission either globally or in the
// given department. Use for write endpoints on dept-scoped resources.
func RequireIn(ctx context.Context, perm string, deptID *uuid.UUID) error {
	u, ok := FromContext(ctx)
	if !ok {
		return httperror.New(httperror.CodeUnauthorized, "not authenticated")
	}
	if u.HasIn(perm, deptID) {
		return nil
	}
	return httperror.New(httperror.CodeForbidden, "missing permission: "+perm)
}
