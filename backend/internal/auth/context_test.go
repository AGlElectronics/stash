package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestUserHasGlobal(t *testing.T) {
	var nilUser *User
	if nilUser.HasGlobal("item.create") {
		t.Errorf("nil user must not have global perms")
	}
	u := &User{
		Permissions: map[string]struct{}{
			"item.create": {},
		},
	}
	if !u.HasGlobal("item.create") {
		t.Errorf("expected global perm")
	}
	if u.HasGlobal("item.delete") {
		t.Errorf("did not expect missing perm")
	}
}

func TestUserHasIn_GlobalGrant(t *testing.T) {
	var nilUser *User
	dept := uuid.New()
	if nilUser.HasIn("item.create", &dept) {
		t.Errorf("nil user must not pass HasIn")
	}
	u := &User{Permissions: map[string]struct{}{"item.create": {}}}
	if !u.HasIn("item.create", &dept) {
		t.Errorf("global perm should satisfy dept-scoped check")
	}
}

func TestUserHasIn_DeptGrant(t *testing.T) {
	dept := uuid.New()
	other := uuid.New()
	u := &User{
		Permissions: map[string]struct{}{},
		DeptPermissions: map[uuid.UUID]map[string]struct{}{
			dept: {"signout.create": {}},
		},
	}
	if !u.HasIn("signout.create", &dept) {
		t.Errorf("dept-scoped perm should be granted in own dept")
	}
	if u.HasIn("signout.create", &other) {
		t.Errorf("dept-scoped perm should not be granted in other dept")
	}
	if u.HasIn("signout.create", nil) {
		t.Errorf("dept-scoped perm should not be granted globally when nil dept passed")
	}
}

func TestUserHasAny(t *testing.T) {
	var nilUser *User
	if nilUser.HasAny("signout.create") {
		t.Errorf("nil user must not pass HasAny")
	}
	dept := uuid.New()
	u := &User{
		Permissions: map[string]struct{}{},
		DeptPermissions: map[uuid.UUID]map[string]struct{}{
			dept: {"signout.create": {}},
		},
	}
	if !u.HasAny("signout.create") {
		t.Errorf("HasAny should find dept-scoped perm")
	}
	if u.HasAny("audit.view") {
		t.Errorf("HasAny should not find missing perm")
	}
}

func TestActorIDOnEmptyContext(t *testing.T) {
	if id := ActorID(context.Background()); id != nil {
		t.Errorf("expected nil ActorID on empty context, got %v", id)
	}
}

func TestPrimaryDepartment(t *testing.T) {
	var nilUser *User
	if got := nilUser.PrimaryDepartment(); got != nil {
		t.Errorf("nil user primary dept: got %v", got)
	}
	dept := uuid.New()
	u := &User{Memberships: []Membership{
		{DepartmentID: nil, Role: "admin"},
		{DepartmentID: &dept, Role: "dept_lead"},
	}}
	if got := u.PrimaryDepartment(); got == nil || *got != dept {
		t.Errorf("expected primary dept %v, got %v", dept, got)
	}
	empty := &User{}
	if got := empty.PrimaryDepartment(); got != nil {
		t.Errorf("expected nil primary dept on empty user, got %v", got)
	}
}
