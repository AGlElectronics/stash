package service

import (
	"context"
	"testing"
	"time"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func TestAnonymiseScrubsIdentityButKeepsHistory(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Robin Volunteer")
	u, _ := auth.FromContext(ctx)
	mustItem(t, pool, "Ladder", 1)
	f := signoutFixture{pool: pool, svc: NewSignoutService(pool), ctx: ctx, dept: mustDept(t, pool, "Stage").ID}
	item := mustItem(t, pool, "Cable", 3)
	f.create(t, true, line(item.ID, 1))

	svc := NewUserService(pool)
	if err := svc.Anonymise(context.Background(), u.ID, "left the team"); err != nil {
		t.Fatalf("anonymise: %v", err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM users WHERE id = $1 AND display_name = $2 AND email IS NULL AND oidc_subject IS NULL AND anonymised_at IS NOT NULL`, u.ID, FormerUser); n != 1 {
		t.Fatal("user row not anonymised")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE actor_user_id = $1 AND (actor_display_name <> $2 OR actor_oidc_subject IS NOT NULL)`, u.ID, FormerUser); n != 0 {
		t.Fatalf("%d audit rows still name the user", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM item_events WHERE actor_user_id = $1 AND actor_display_name = $2`, u.ID, FormerUser); n == 0 {
		t.Fatal("item history should keep the events, attributed to a former user")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE action = 'user.anonymise' AND entity_id = $1`, u.ID); n != 1 {
		t.Fatal("anonymisation must be audited")
	}
	wantCode(t, svc.Anonymise(context.Background(), u.ID, ""), httperror.CodeConflict)

	list, total, err := svc.List(context.Background(), 10, 0)
	if err != nil || total != 1 || list[0].AnonymisedAt == "" {
		t.Fatalf("list: %v %d %+v", err, total, list)
	}
}

func TestAnonymiseInactive(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Old timer")
	old, _ := auth.FromContext(ctx)
	testdb.Actor(t, pool, "Regular")
	if _, err := pool.Exec(ctx, `UPDATE users SET last_login_at = now() - interval '400 days' WHERE id = $1`, old.ID); err != nil {
		t.Fatal(err)
	}
	svc := NewUserService(pool)
	if n, err := svc.AnonymiseInactive(ctx, 0, time.Now()); err != nil || n != 0 {
		t.Fatalf("0 days must do nothing: %d %v", n, err)
	}
	n, err := svc.AnonymiseInactive(ctx, 365, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("anonymised %d (%v), want 1", n, err)
	}
}
