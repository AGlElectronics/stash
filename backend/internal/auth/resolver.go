package auth

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// Resolver turns an authenticated identity into a *User. The users table is
// only used to persist identity (stable user id for FKs, display name,
// last login); roles and permissions are computed per-request from the
// access config file + the token's group claims and never touch the DB.
type Resolver struct {
	pool   *pgxpool.Pool
	access *AccessConfig
}

func NewResolver(pool *pgxpool.Pool, access *AccessConfig) *Resolver {
	return &Resolver{pool: pool, access: access}
}

// ParseDevUser splits a dev-mode user value of the form "name" or
// "name:role1,role2". Without roles the user is an admin. Roles may be
// written with or without the stash_ prefix.
func ParseDevUser(value string) (name string, roles []string) {
	value = strings.TrimSpace(value)
	name, list, _ := strings.Cut(value, ":")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "dev-admin"
	}
	for _, r := range strings.Split(list, ",") {
		r = strings.TrimPrefix(strings.TrimSpace(r), RoleKeyPrefix)
		if r != "" {
			roles = append(roles, r)
		}
	}
	if len(roles) == 0 {
		roles = []string{domain.RoleAdmin}
	}
	return name, roles
}

// LoadOrProvisionDevUser is used by the dev bypass middleware. The user is
// stored with the oidc_subject "dev:<name>"; the roles come from the header
// value (see ParseDevUser) and must exist in the role catalog.
func (r *Resolver) LoadOrProvisionDevUser(ctx context.Context, headerValue string) (*User, error) {
	name, roles := ParseDevUser(headerValue)
	for _, role := range roles {
		if !r.access.HasRole(role) {
			return nil, httperror.New(httperror.CodeForbidden, fmt.Sprintf("unknown dev role %q", role))
		}
	}
	subject := "dev:" + name
	display := strings.ToTitle(strings.ReplaceAll(name, "-", " "))
	email := name + "@stash.local"

	q := sqlcgen.New(r.pool)
	row, err := q.UpsertUserBySubject(ctx, sqlcgen.UpsertUserBySubjectParams{
		OidcSubject: &subject,
		Email:       &email,
		DisplayName: display,
	})
	if err != nil {
		return nil, fmt.Errorf("provision dev user: %w", err)
	}
	return r.buildUser(row, roles), nil
}

// LoadOrProvisionOIDCUser is called by the OIDC middleware after a successful
// JWT verification. It upserts the identity row, then computes the user's
// roles and permissions from the access config and the token's groups.
// Nothing role-related is persisted: removing a group in the IdP takes
// effect on the next token.
func (r *Resolver) LoadOrProvisionOIDCUser(ctx context.Context, claims *OIDCClaims) (*User, error) {
	subject := claims.Subject
	display := displayNameFromClaims(claims)
	email := strings.TrimSpace(claims.Email)

	q := sqlcgen.New(r.pool)
	oidcSub := subject
	row, err := q.UpsertUserBySubject(ctx, sqlcgen.UpsertUserBySubjectParams{
		OidcSubject: &oidcSub,
		Email:       optStrPtr(email),
		DisplayName: display,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	// Merge both groups and roles claims so the config file can bind on either.
	combined := append(append([]string{}, claims.Groups...), claims.Roles...)
	return r.buildUser(row, r.access.RolesForGroups(combined)), nil
}

// buildUser assembles the request principal from the identity row and the
// user's roles.
func (r *Resolver) buildUser(row sqlcgen.User, roles []string) *User {
	perms := map[string]struct{}{}
	uniq := map[string]struct{}{}
	for _, role := range roles {
		uniq[role] = struct{}{}
		for p := range r.access.PermissionsForRole(role) {
			perms[p] = struct{}{}
		}
	}
	sorted := make([]string, 0, len(uniq))
	for role := range uniq {
		sorted = append(sorted, role)
	}
	sort.Strings(sorted)

	email := ""
	if row.Email != nil {
		email = *row.Email
	}
	subject := ""
	if row.OidcSubject != nil {
		subject = *row.OidcSubject
	}
	return &User{
		ID:          row.ID,
		OIDCSubject: subject,
		Email:       email,
		DisplayName: row.DisplayName,
		Roles:       sorted,
		Permissions: perms,
	}
}

func optStrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// displayNameFromClaims picks a human-facing label from OIDC claims. Identity
// is always claims.Subject (Keycloak user UUID); email and display name may
// change and are refreshed on each login.
func displayNameFromClaims(c *OIDCClaims) string {
	if c == nil {
		return ""
	}
	for _, s := range []string{c.Nickname, c.Name, c.PreferredUsername} {
		if t := strings.TrimSpace(s); t != "" {
			return t
		}
	}
	return c.Subject
}
