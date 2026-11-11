package http

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
)

// roleCatalogEntry is one role in GET /api/role-catalog
// (docs/concepts/role-catalog.md).
type roleCatalogEntry struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Group        string   `json:"group"`
	GroupDisplay string   `json:"group_display"`
	Assignable   bool     `json:"assignable"`
	Default      bool     `json:"default"`
	Scope        string   `json:"scope"`
	Privileged   bool     `json:"privileged"`
	Dynamic      bool     `json:"dynamic"`
	Permissions  []string `json:"permissions"`
}

// privilegedPermissions make a role admin-type: they change Stash's
// settings and access (access.manage), manage other people's accounts
// (user.manage) or download every record with guests' and volunteers'
// personal data (export.create).
var privilegedPermissions = map[string]bool{
	domain.PermAccessManage: true,
	domain.PermUserManage:   true,
	domain.PermExportCreate: true,
}

// roleCatalogAuth admits HR's backend with HR_SERVICE_TOKEN (constant-time
// compare) and otherwise requires a normal staff sign-in through authMW.
// Without a configured token only signed-in staff can read the catalog.
func roleCatalogAuth(hrToken string, authMW func(http.Handler) http.Handler, next http.Handler) http.HandlerFunc {
	signedIn := authMW(next)
	return func(w http.ResponseWriter, r *http.Request) {
		if hrToken != "" {
			got := bearerToken(r)
			if got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(hrToken)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}
		signedIn.ServeHTTP(w, r)
	}
}

func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// roleCatalogHandler serves GET /api/role-catalog. Every role is global and
// listed once as stash_<role>. Its group is the first group bound to it in
// access-config.toml, or the role's global group stash_<role>: group is the
// stored (normalised) form, group_display the one in the file. revision
// (also the ETag) changes whenever the list does; If-None-Match with it
// answers 304.
func roleCatalogHandler(access *auth.AccessConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roles := make([]roleCatalogEntry, 0)
		for _, role := range access.Roles() {
			perms := sortedPerms(access.PermissionsForRole(role))
			privileged := false
			for _, p := range perms {
				privileged = privileged || privilegedPermissions[p]
			}
			g := access.GroupForRole(role)
			roles = append(roles, roleCatalogEntry{
				Key:          auth.RoleKey(role),
				Label:        humanize(role),
				Description:  access.RoleDescription(role),
				Group:        g.Stored,
				GroupDisplay: g.Display,
				Assignable:   true,
				Scope:        "organization",
				Privileged:   privileged,
				Permissions:  perms,
			})
		}
		raw, _ := json.Marshal(roles)
		sum := sha256.Sum256(raw)
		revision := hex.EncodeToString(sum[:8])
		etag := `"` + revision + `"`
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "private, no-cache")
		if etagMatches(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"module":       "stash",
			"display_name": "Stash",
			"description":  "Inventory, sign-outs and Lost & Found.",
			"revision":     revision,
			"roles":        roles,
		})
	}
}

// etagMatches reads an If-None-Match header: "*", or a list of (possibly
// weak) entity tags.
func etagMatches(header, etag string) bool {
	for _, t := range strings.Split(header, ",") {
		t = strings.TrimPrefix(strings.TrimSpace(t), "W/")
		if t == "*" || t == etag {
			return true
		}
	}
	return false
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
