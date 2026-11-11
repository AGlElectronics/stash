package auth

import (
	"strings"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/domain"
)

func TestParseAccessConfig_DefaultsOnly(t *testing.T) {
	cfg, err := parseAccessConfig([]byte(`
[[bindings]]
group = "stash-admin"
role = "admin"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.HasRole(domain.RoleAdmin) || !cfg.HasRole(domain.RoleFrontdesk) {
		t.Fatalf("built-in roles missing: %v", cfg.Roles())
	}
	admin := cfg.PermissionsForRole(domain.RoleAdmin)
	if _, ok := admin[domain.PermLostfoundResolve]; !ok {
		t.Error("admin should hold every permission, lostfound.resolve missing")
	}
	if len(admin) != len(domain.AllPermissions()) {
		t.Errorf("admin has %d perms, want %d", len(admin), len(domain.AllPermissions()))
	}
	frontdesk := cfg.PermissionsForRole(domain.RoleFrontdesk)
	if _, ok := frontdesk[domain.PermLostfoundView]; !ok {
		t.Error("frontdesk should have lostfound.view")
	}
	if _, ok := frontdesk[domain.PermItemView]; ok {
		t.Error("frontdesk should not have item.view by default")
	}
}

func TestParseAccessConfig_LostfoundOnlyForAdminAndFrontdesk(t *testing.T) {
	cfg, err := parseAccessConfig(nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, role := range []string{domain.RoleInventoryManager, domain.RoleDeptLead, domain.RoleVolunteer, domain.RoleReadonly} {
		perms := cfg.PermissionsForRole(role)
		for _, p := range []string{domain.PermLostfoundView, domain.PermLostfoundCreate, domain.PermLostfoundResolve} {
			if _, ok := perms[p]; ok {
				t.Errorf("role %s should not have %s", role, p)
			}
		}
	}
}

func TestParseAccessConfig_CustomRole(t *testing.T) {
	src := []byte(`
[roles.conops]
permissions = ["lostfound.view", "lostfound.create"]

[[bindings]]
group = "conops-team"
role = "conops"
`)
	cfg, err := parseAccessConfig(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	perms := cfg.PermissionsForRole("conops")
	if len(perms) != 2 {
		t.Fatalf("conops has %d perms, want 2", len(perms))
	}
	got := cfg.Bindings.Lookup([]string{"conops-team"})
	if len(got) != 1 || got[0].RoleName != "conops" {
		t.Fatalf("binding lookup failed: %+v", got)
	}
}

func TestParseAccessConfig_OverrideBuiltinRole(t *testing.T) {
	src := []byte(`
[roles.readonly]
permissions = ["item.view"]
`)
	cfg, err := parseAccessConfig(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	perms := cfg.PermissionsForRole(domain.RoleReadonly)
	if len(perms) != 1 {
		t.Fatalf("override should replace the set entirely, got %d perms", len(perms))
	}
}

func TestParseAccessConfig_RejectsUnknownPermission(t *testing.T) {
	src := []byte(`
[roles.oops]
permissions = ["item.fly"]
`)
	if _, err := parseAccessConfig(src); err == nil {
		t.Fatal("expected error for unknown permission")
	}
}

func TestParseAccessConfig_RejectsBindingToUnknownRole(t *testing.T) {
	src := []byte(`
[[bindings]]
group = "g"
role = "does-not-exist"
`)
	if _, err := parseAccessConfig(src); err == nil {
		t.Fatal("expected error for binding to unknown role")
	}
}

func TestRolesForGroups_BindingsAndKeyNames(t *testing.T) {
	cfg, err := parseAccessConfig([]byte(`
[[bindings]]
group = "stash-admin"
role = "admin"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := cfg.RolesForGroups([]string{"stash-admin", "stash_volunteer", "stash_nope", "other"})
	if strings.Join(got, ",") != "admin,volunteer" {
		t.Fatalf("roles = %v, want [admin volunteer]", got)
	}
	if g := cfg.GroupForRole(domain.RoleAdmin); g != "stash-admin" {
		t.Errorf("admin group = %q, want the bound group", g)
	}
	if g := cfg.GroupForRole(domain.RoleVolunteer); g != "stash_volunteer" {
		t.Errorf("volunteer group = %q, want the role key", g)
	}
}

func TestShippedAccessConfigLoads(t *testing.T) {
	cfg, err := LoadAccessConfig("../../../infra/access-config.toml")
	if err != nil {
		t.Fatalf("infra/access-config.toml: %v", err)
	}
	if got := cfg.RolesForGroups([]string{"stash-dept-lead"}); len(got) != 1 || got[0] != domain.RoleDeptLead {
		t.Fatalf("stash-dept-lead → %v", got)
	}
}

func TestDefaultAccessConfig(t *testing.T) {
	cfg := DefaultAccessConfig()
	if !cfg.Bindings.Empty() {
		t.Error("default config should have no bindings")
	}
	if !cfg.HasRole(domain.RoleAdmin) {
		t.Error("default config should include built-in roles")
	}
}
