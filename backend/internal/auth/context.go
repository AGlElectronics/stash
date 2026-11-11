// Package auth manages the authenticated user attached to each request and
// exposes the permission helpers. Roles are global: a role grants its
// permissions for every department, and no binding is scoped further.
package auth

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
)

// User is the authenticated principal for a request.
type User struct {
	ID          uuid.UUID
	OIDCSubject string
	Email       string
	DisplayName string
	// Roles are the Stash role names (without the stash_ prefix) the user
	// holds, sorted.
	Roles []string
	// Permissions is the union of the permissions of Roles.
	Permissions map[string]struct{}
}

// Has reports whether the user holds the permission.
func (u *User) Has(perm string) bool {
	if u == nil {
		return false
	}
	_, ok := u.Permissions[perm]
	return ok
}

// EffectivePermissions returns the user's permissions, sorted. /api/me
// exposes them so the SPA can show or hide navigation; the backend still
// checks every request.
func (u *User) EffectivePermissions() []string {
	if u == nil {
		return nil
	}
	out := make([]string, 0, len(u.Permissions))
	for p := range u.Permissions {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

type ctxKey int

const userKey ctxKey = 0

// WithUser stores the authenticated user on the context.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// FromContext returns the authenticated user, if any.
func FromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKey).(*User)
	if !ok || u == nil {
		return nil, false
	}
	return u, true
}

// MustFromContext returns the user or an error if not authenticated.
func MustFromContext(ctx context.Context) (*User, error) {
	u, ok := FromContext(ctx)
	if !ok {
		return nil, errors.New("no authenticated user on context")
	}
	return u, nil
}

// HasPermission reports whether the request's user holds perm. It is false
// for background jobs, which run without a user.
func HasPermission(ctx context.Context, perm string) bool {
	u, _ := FromContext(ctx)
	return u.Has(perm)
}

// ActorID returns a *uuid.UUID for the user, or nil when no user is present.
func ActorID(ctx context.Context) *uuid.UUID {
	u, ok := FromContext(ctx)
	if !ok {
		return nil
	}
	id := u.ID
	return &id
}

// ActorSnapshot captures identity at request time for audit/history rows.
// Display names (nickname) and email can change in Keycloak; oidc_subject
// (Keycloak sub) is the stable external id.
type ActorSnapshot struct {
	UserID      *uuid.UUID
	OIDCSubject string
	DisplayName string
}

func ActorSnapshotFromContext(ctx context.Context) ActorSnapshot {
	u, ok := FromContext(ctx)
	if !ok {
		return ActorSnapshot{}
	}
	id := u.ID
	return ActorSnapshot{
		UserID:      &id,
		OIDCSubject: u.OIDCSubject,
		DisplayName: u.DisplayName,
	}
}
