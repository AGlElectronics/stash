package http

import (
	"net/http"
	"sort"
	"strings"

	"github.com/AGlElectronics/stash/backend/internal/auth"
)

// roleCatalogEntry is one assignable (or informational) role in the shared
// Con Services role-catalog contract consumed by HR.
type roleCatalogEntry struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Group       string   `json:"group"`
	Assignable  bool     `json:"assignable"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
	Note        string   `json:"note,omitempty"`
}

// roleCatalogHandler serves GET /api/role-catalog. Public and read-only: it
// exposes role names, descriptions and the IdP group bound to each role so
// HR can offer a picker. Groups come from the access config bindings.
func roleCatalogHandler(access *auth.AccessConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roles := make([]roleCatalogEntry, 0)
		var bindings []auth.GroupBinding
		if access != nil && access.Bindings != nil {
			bindings = access.Bindings.All()
		}
		for _, role := range access.Roles() {
			perms := sortedPerms(access.PermissionsForRole(role))
			bound := false
			for _, b := range bindings {
				if b.RoleName != role {
					continue
				}
				bound = true
				scope := "global"
				if b.DepartmentID != nil {
					scope = "department:" + b.DepartmentID.String()
				}
				roles = append(roles, roleCatalogEntry{
					Key:         role,
					Label:       humanize(role),
					Description: access.RoleDescription(role),
					Group:       b.OIDCGroup,
					Assignable:  true,
					Scope:       scope,
					Permissions: perms,
				})
			}
			if !bound {
				roles = append(roles, roleCatalogEntry{
					Key:         role,
					Label:       humanize(role),
					Description: access.RoleDescription(role),
					Assignable:  false,
					Scope:       "global",
					Permissions: perms,
					Note:        "No IdP group is bound to this role in access-config.toml.",
				})
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"module":       "stash",
			"display_name": "Stash",
			"description":  "Inventory, sign-outs and Lost & Found.",
			"roles":        roles,
		})
	}
}

func sortedPerms(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func humanize(key string) string {
	parts := strings.FieldsFunc(key, func(r rune) bool { return r == '_' || r == '-' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
