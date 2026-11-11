package http

import (
	"net/http"
	"sort"
	"strings"

	"github.com/AGlElectronics/stash/backend/internal/auth"
)

// roleCatalogEntry is one assignable role in the shared Con Services
// role-catalog contract consumed by HR.
type roleCatalogEntry struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Group       string   `json:"group"`
	Assignable  bool     `json:"assignable"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

// roleCatalogHandler serves GET /api/role-catalog. Public and read-only.
// Every role is global and listed once as stash_<role>. Its group is the
// first group bound to it in access-config.toml, or the key itself: a
// group named stash_<role> always grants the role.
func roleCatalogHandler(access *auth.AccessConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roles := make([]roleCatalogEntry, 0)
		for _, role := range access.Roles() {
			roles = append(roles, roleCatalogEntry{
				Key:         auth.RoleKey(role),
				Label:       humanize(role),
				Description: access.RoleDescription(role),
				Group:       access.GroupForRole(role),
				Assignable:  true,
				Scope:       "global",
				Permissions: sortedPerms(access.PermissionsForRole(role)),
			})
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
