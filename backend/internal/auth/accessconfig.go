package auth

import (
	"fmt"
	"os"
	"sort"

	"github.com/pelletier/go-toml/v2"

	"github.com/AGlElectronics/stash/backend/internal/domain"
)

// AccessConfig is the in-memory snapshot of the access config file — the
// single source of truth for roles, their permissions, and OIDC group →
// role bindings. There is no DB-backed access management anymore; the file
// is loaded once at startup and the pod must restart to pick up changes.
//
// It is safe for concurrent reads: nothing is mutated after construction.
type AccessConfig struct {
	// Bindings maps OIDC group claims to roles (optionally dept-scoped).
	Bindings *GroupBindings

	rolePerms map[string]map[string]struct{}
	roleDesc  map[string]string
}

// RoleDescription returns the human description for a role ("" if none).
func (c *AccessConfig) RoleDescription(role string) string {
	if c == nil {
		return ""
	}
	if d, ok := c.roleDesc[role]; ok && d != "" {
		return d
	}
	return defaultRoleDescriptions[role]
}

var defaultRoleDescriptions = map[string]string{
	"admin":             "Everything in Stash, including access and settings.",
	"inventory_manager": "Logistics team: items, sign-outs, locations and categories.",
	"dept_lead":         "Department lead: sign-outs and returns for their department.",
	"volunteer":         "Look up items and run sign-outs.",
	"readonly":          "Read-only view of inventory and sign-outs.",
	"frontdesk":         "ConOps front desk: log and resolve Lost & Found.",
	"conops_lead":       "Full Lost & Found plus reopen, delete, audit and exports.",
	"conops_readonly":   "View Lost & Found reports only.",
}

// DefaultAccessConfig returns the built-in role catalog with no group
// bindings. Used in dev mode when no access config file is configured.
func DefaultAccessConfig() *AccessConfig {
	return &AccessConfig{
		Bindings:  &GroupBindings{byGroup: map[string][]GroupBinding{}},
		rolePerms: defaultRolePermSets(),
	}
}

// PermissionsForRole returns the permission set for a role, or nil when the
// role is unknown. The returned map must not be mutated.
func (c *AccessConfig) PermissionsForRole(role string) map[string]struct{} {
	if c == nil {
		return nil
	}
	return c.rolePerms[role]
}

// HasRole reports whether the role exists in the merged catalog.
func (c *AccessConfig) HasRole(role string) bool {
	if c == nil {
		return false
	}
	_, ok := c.rolePerms[role]
	return ok
}

// Roles returns the sorted role names in the merged catalog.
func (c *AccessConfig) Roles() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.rolePerms))
	for name := range c.rolePerms {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

type accessFile struct {
	// Roles optionally defines extra roles or overrides the permission set
	// of a built-in role. A role listed here replaces the default set
	// entirely (no merging), so the file reads as the literal truth.
	Roles map[string]accessFileRole `toml:"roles"`
	// Bindings maps OIDC group claims to roles.
	Bindings []bindingFileEntry `toml:"bindings"`
}

type accessFileRole struct {
	Description string   `toml:"description"`
	Permissions []string `toml:"permissions"`
}

// LoadAccessConfig reads and parses the access config TOML file at path.
// See infra/access-config.toml for the canonical example.
func LoadAccessConfig(path string) (*AccessConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read access config %s: %w", path, err)
	}
	cfg, err := parseAccessConfig(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func parseAccessConfig(data []byte) (*AccessConfig, error) {
	var f accessFile
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse toml: %w", err)
	}

	known := map[string]struct{}{}
	for _, p := range domain.AllPermissions() {
		known[p] = struct{}{}
	}

	rolePerms := defaultRolePermSets()
	roleDesc := map[string]string{}
	for name, entry := range f.Roles {
		if name == "" {
			return nil, fmt.Errorf("roles: role name must not be empty")
		}
		roleDesc[name] = entry.Description
		set := map[string]struct{}{}
		for _, p := range entry.Permissions {
			if _, ok := known[p]; !ok {
				return nil, fmt.Errorf("roles.%s: unknown permission %q", name, p)
			}
			set[p] = struct{}{}
		}
		rolePerms[name] = set
	}

	bindings, err := buildGroupBindings(f.Bindings)
	if err != nil {
		return nil, err
	}
	for _, b := range bindings.All() {
		if _, ok := rolePerms[b.RoleName]; !ok {
			return nil, fmt.Errorf("bindings: group %q references unknown role %q", b.OIDCGroup, b.RoleName)
		}
	}

	return &AccessConfig{Bindings: bindings, rolePerms: rolePerms, roleDesc: roleDesc}, nil
}

func defaultRolePermSets() map[string]map[string]struct{} {
	out := map[string]map[string]struct{}{}
	for role, perms := range domain.DefaultRolePermissions() {
		set := make(map[string]struct{}, len(perms))
		for _, p := range perms {
			set[p] = struct{}{}
		}
		out[role] = set
	}
	return out
}
