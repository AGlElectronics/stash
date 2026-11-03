package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseGroupBindings_Valid(t *testing.T) {
	deptID := uuid.New()
	tomlSrc := []byte(`
[[bindings]]
group = "stash-admin"
role = "admin"

[[bindings]]
group = "stash-inventory"
role = "inventory_manager"
department_id = "` + deptID.String() + `"

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
	if all[0].OIDCGroup != "stash-admin" || all[0].RoleName != "admin" || all[0].DepartmentID != nil {
		t.Errorf("unexpected first binding: %+v", all[0])
	}
	if all[1].OIDCGroup != "stash-inventory" || all[1].DepartmentID == nil || *all[1].DepartmentID != deptID {
		t.Errorf("unexpected dept-scoped binding: %+v", all[1])
	}
}

func TestParseGroupBindings_RejectsBadDeptID(t *testing.T) {
	tomlSrc := []byte(`
[[bindings]]
group = "stash-admin"
role = "admin"
department_id = "not-a-uuid"
`)
	_, err := parseGroupBindings(tomlSrc)
	if err == nil {
		t.Fatalf("expected error on bad department_id, got nil")
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
	got := gb.Lookup([]string{"a-team", "b-team", "c-team"})
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
	if got := gb.Lookup(nil); len(got) != 0 {
		t.Errorf("expected no bindings for nil groups, got %d", len(got))
	}
	if got := gb.Lookup([]string{}); len(got) != 0 {
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
	if gb.Lookup([]string{"a"}) != nil {
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
