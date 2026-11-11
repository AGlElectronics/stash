package auth

import (
	"strings"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/groupmatch"
)

func TestParseGroupBindings_Valid(t *testing.T) {
	tomlSrc := []byte(`
[[bindings]]
group = "stash-admin"
role = "admin"

[[bindings]]
group = "stash-inventory"
role = "inventory_manager"

[[bindings]]
group = "stash-readonly"
role = "readonly"
`)

	gb, err := parseGroupBindings(tomlSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	all := gb.All()
	if len(all) != 3 {
		t.Fatalf("got %d bindings, want 3", len(all))
	}
	if all[0].Group.Display != "stash-admin" || all[0].RoleName != "admin" {
		t.Errorf("unexpected first binding: %+v", all[0])
	}
}

func TestParseGroupBindings_RejectsDepartmentScope(t *testing.T) {
	tomlSrc := []byte(`
[[bindings]]
group = "stash-dept-foo"
role = "dept_lead"
department_id = "00000000-0000-0000-0000-000000000000"
`)
	_, err := parseGroupBindings(tomlSrc)
	if err == nil || !strings.Contains(err.Error(), "roles are global") {
		t.Fatalf("expected a 'roles are global' error, got %v", err)
	}
}

func TestParseGroupBindings_RequiresGroupAndRole(t *testing.T) {
	t.Run("missing group", func(t *testing.T) {
		_, err := parseGroupBindings([]byte(`
[[bindings]]
role = "admin"
`))
		if err == nil {
			t.Fatalf("expected error when group missing")
		}
	})
	t.Run("missing role", func(t *testing.T) {
		_, err := parseGroupBindings([]byte(`
[[bindings]]
group = "stash-admin"
`))
		if err == nil {
			t.Fatalf("expected error when role missing")
		}
	})
}

func TestParseGroupBindings_RejectsDuplicate(t *testing.T) {
	tomlSrc := []byte(`
[[bindings]]
group = "stash-admin"
role = "admin"

[[bindings]]
group = "stash-admin"
role = "admin"
`)
	_, err := parseGroupBindings(tomlSrc)
	if err == nil {
		t.Fatalf("expected error on duplicate binding")
	}
}

func TestParseGroupBindings_StoresNormalisedAndDisplayForm(t *testing.T) {
	gb, err := parseGroupBindings([]byte(`
[[bindings]]
group = " /Stash-Admin "
role = "admin"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	b := gb.All()[0]
	if b.Group.Stored != "stash-admin" || b.Group.Display != "/Stash-Admin" {
		t.Fatalf("grant = %+v", b.Group)
	}
	if _, err := parseGroupBindings([]byte(`
[[bindings]]
group = "/Stash-Admin"
role = "admin"

[[bindings]]
group = "stash-admin"
role = "admin"
`)); err == nil {
		t.Fatal("bindings that normalise to the same group must be duplicates")
	}
}

func TestGroupBindings_LookupUsesGroupmatch(t *testing.T) {
	gb, err := parseGroupBindings([]byte(`
[[bindings]]
group = "departments/security"
role = "frontdesk"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cases := []struct {
		gm     groupmatch.Config
		groups []string
		want   int
	}{
		{groupmatch.Config{}, []string{"/Departments/Security"}, 1},
		{groupmatch.Config{}, []string{"/teams/security"}, 0},
		{groupmatch.Config{Mode: groupmatch.ModeLast}, []string{"/teams/security"}, 1},
		{groupmatch.Config{Prefix: "/con2026"}, []string{"/con2026/departments/security"}, 1},
		{groupmatch.Config{Prefix: "/con2026"}, []string{"/con2025/departments/security"}, 0},
		{groupmatch.Config{}, []string{"/departments/security-lead"}, 0},
	}
	for _, tc := range cases {
		if got := gb.Lookup(tc.gm, tc.groups); len(got) != tc.want {
			t.Errorf("Lookup(%+v, %v) = %d bindings, want %d", tc.gm, tc.groups, len(got), tc.want)
		}
	}
}

func TestGroupBindings_LookupDedupesAcrossGroups(t *testing.T) {
	tomlSrc := []byte(`
[[bindings]]
group = "a-team"
role = "admin"

[[bindings]]
group = "b-team"
role = "admin"
`)
	gb, err := parseGroupBindings(tomlSrc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := gb.Lookup(groupmatch.Config{}, []string{"a-team", "b-team", "c-team"})
	if len(got) != 2 {
		t.Fatalf("got %d bindings, want 2", len(got))
	}
}

func TestGroupBindings_LookupEmptyGroups(t *testing.T) {
	gb, err := parseGroupBindings([]byte(`
[[bindings]]
group = "g"
role = "admin"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := gb.Lookup(groupmatch.Config{}, nil); len(got) != 0 {
		t.Errorf("expected no bindings for nil groups, got %d", len(got))
	}
	if got := gb.Lookup(groupmatch.Config{}, []string{}); len(got) != 0 {
		t.Errorf("expected no bindings for empty groups, got %d", len(got))
	}
}

func TestGroupBindings_NilSafe(t *testing.T) {
	var gb *GroupBindings
	if !gb.Empty() {
		t.Error("nil should be Empty")
	}
	if gb.All() != nil {
		t.Error("nil.All() should be nil")
	}
	if gb.Lookup(groupmatch.Config{}, []string{"a"}) != nil {
		t.Error("nil.Lookup() should be nil")
	}
}

func TestGroupBindings_EmptyFile(t *testing.T) {
	gb, err := parseGroupBindings([]byte(""))
	if err != nil {
		t.Fatalf("parse empty: %v", err)
	}
	if !gb.Empty() {
		t.Errorf("expected Empty=true for empty file, got %d bindings", len(gb.All()))
	}
}
