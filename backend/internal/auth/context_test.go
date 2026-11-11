package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestUserHas(t *testing.T) {
	var nilUser *User
	if nilUser.Has("item.view") {
		t.Error("nil user must not have permissions")
	}
	u := &User{ID: uuid.New(), Permissions: map[string]struct{}{"item.view": {}}}
	if !u.Has("item.view") || u.Has("item.create") {
		t.Error("Has should report exactly the granted permissions")
	}
}

func TestEffectivePermissionsSorted(t *testing.T) {
	u := &User{Permissions: map[string]struct{}{"signout.view": {}, "item.view": {}}}
	got := u.EffectivePermissions()
	if len(got) != 2 || got[0] != "item.view" || got[1] != "signout.view" {
		t.Fatalf("got %v", got)
	}
}

func TestRequireHelpers(t *testing.T) {
	if err := Require(context.Background(), "item.view"); err == nil {
		t.Error("Require without a user must fail")
	}
	ctx := WithUser(context.Background(), &User{Permissions: map[string]struct{}{"item.view": {}}})
	if err := Require(ctx, "item.view"); err != nil {
		t.Errorf("Require: %v", err)
	}
	if err := Require(ctx, "item.edit"); err == nil {
		t.Error("Require must fail for a missing permission")
	}
	if err := RequireAnyOf(ctx, "item.edit", "item.view"); err != nil {
		t.Errorf("RequireAnyOf: %v", err)
	}
	if HasPermission(context.Background(), "item.view") {
		t.Error("HasPermission without a user must be false")
	}
}
