package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/domain"
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

// LoadOrProvisionDevUser is used by the dev bypass middleware. The header
// value becomes part of an oidc_subject of the form "dev:<value>". Dev users
// always get the global admin role so dev work is unblocked.
func (r *Resolver) LoadOrProvisionDevUser(ctx context.Context, headerValue string) (*User, error) {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		headerValue = "dev-admin"
	}
	subject := "dev:" + headerValue

	display := strings.ToTitle(strings.ReplaceAll(headerValue, "-", " "))
	if display == "" {
		display = headerValue
	}
	email := headerValue + "@stash.local"

	q := sqlcgen.New(r.pool)
	row, err := q.UpsertUserBySubject(ctx, sqlcgen.UpsertUserBySubjectParams{
		OidcSubject: &subject,
		Email:       &email,
		DisplayName: display,
	})
	if err != nil {
		return nil, fmt.Errorf("provision dev user: %w", err)
	}

	return r.buildUser(row, []GroupBinding{{RoleName: domain.RoleAdmin}}), nil
}

// LoadOrProvisionOIDCUser is called by the OIDC middleware after a successful
// JWT verification. It upserts the identity row, then computes the user's
// memberships and permissions from the access config and the token's groups.
// Nothing role-related is persisted: revoking a group in the IdP or editing
// the config file takes effect on the next request / pod restart.
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
	combined := append(claims.Groups, claims.Roles...)
	return r.buildUser(row, r.access.Bindings.Lookup(combined)), nil
}

// buildUser assembles the request principal from the identity row and the
// matched bindings. Memberships are deduplicated by (department, role).
func (r *Resolver) buildUser(row sqlcgen.User, bindings []GroupBinding) *User {
	memberships := make([]Membership, 0, len(bindings))
	seen := map[string]struct{}{}
	for _, b := range bindings {
		key := b.RoleName
		if b.DepartmentID != nil {
			key = b.DepartmentID.String() + "|" + b.RoleName
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		memberships = append(memberships, Membership{DepartmentID: b.DepartmentID, Role: b.RoleName})
	}

	globalPerms := map[string]struct{}{}
	deptPerms := map[uuid.UUID]map[string]struct{}{}
	for _, m := range memberships {
		set := r.access.PermissionsForRole(m.Role)
		if set == nil {
			continue
		}
		if m.DepartmentID == nil {
			for p := range set {
				globalPerms[p] = struct{}{}
			}
			continue
		}
		d := *m.DepartmentID
		scoped, ok := deptPerms[d]
		if !ok {
			scoped = map[string]struct{}{}
			deptPerms[d] = scoped
		}
		for p := range set {
			scoped[p] = struct{}{}
		}
	}

	email := ""
	if row.Email != nil {
		email = *row.Email
	}
	subject := ""
	if row.OidcSubject != nil {
		subject = *row.OidcSubject
	}

	return &User{
		ID:              row.ID,
		OIDCSubject:     subject,
		Email:           email,
		DisplayName:     row.DisplayName,
		Memberships:     memberships,
		Permissions:     globalPerms,
		DeptPermissions: deptPerms,
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
