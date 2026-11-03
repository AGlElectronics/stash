// Package auth manages the authenticated user attached to each request and
// exposes RBAC helpers. v0.2 ships a dev bypass mode; OIDC + JIT
// provisioning land in v0.4.
package auth

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
)

// Membership records that a user has a role within a department (or globally
// if DepartmentID is nil).
type Membership struct {
	DepartmentID *uuid.UUID
	Role         string
}

// User is the authenticated principal for a request.
type User struct {
	ID          uuid.UUID
	OIDCSubject string
	Email       string
	DisplayName string
	Memberships []Membership

	// Permissions holds the permissions a user has globally (i.e. via
	// memberships with department_id IS NULL). These apply across the whole
	// system.
	Permissions map[string]struct{}

	// DeptPermissions holds the permissions a user has scoped to a specific
	// department. A user can perform an action on a resource owned by dept D
	// if they have the permission either globally OR in dept D.
	DeptPermissions map[uuid.UUID]map[string]struct{}
}

// HasGlobal returns true if the user has the given permission globally.
func (u *User) HasGlobal(perm string) bool {
	if u == nil {
		return false
	}
	_, ok := u.Permissions[perm]
	return ok
}

// HasIn returns true if the user has the given permission either globally
// or scoped to the given department. A nil deptID asks for global only.
func (u *User) HasIn(perm string, deptID *uuid.UUID) bool {
	if u == nil {
		return false
	}
	if u.HasGlobal(perm) {
		return true
	}
	if deptID == nil {
		return false
	}
	if perms, ok := u.DeptPermissions[*deptID]; ok {
		_, ok := perms[perm]
		return ok
	}
	return false
}

// HasAny returns true if the user has the given permission anywhere
// (globally or in any single department). Used for "can the user even see
// this kind of resource?" checks where dept scope is unknown up-front.
func (u *User) HasAny(perm string) bool {
	if u == nil {
		return false
	}
	if u.HasGlobal(perm) {
		return true
	}
	for _, perms := range u.DeptPermissions {
		if _, ok := perms[perm]; ok {
			return true
		}
	}
	return false
}

// EffectivePermissions returns the union of the user's global and
// department-scoped permissions, sorted. /api/me exposes this so the SPA can
// gate navigation/sections; precise (dept-aware) enforcement stays
// server-side via HasGlobal/HasIn/HasAny.
func (u *User) EffectivePermissions() []string {
	if u == nil {
		return nil
	}
	set := map[string]struct{}{}
	for p := range u.Permissions {
		set[p] = struct{}{}
	}
	for _, perms := range u.DeptPermissions {
		for p := range perms {
			set[p] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// PrimaryDepartment returns the user's first dept-scoped membership, useful
// for defaulting the destination of a "sign out to me" flow.
func (u *User) PrimaryDepartment() *uuid.UUID {
	if u == nil {
		return nil
	}
	for _, m := range u.Memberships {
		if m.DepartmentID != nil {
			return m.DepartmentID
		}
	}
	return nil
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
