package events

import (
	"context"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func TestRecordStoresActorSnapshot(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Robin")
	u, _ := auth.FromContext(ctx)
	q := sqlcgen.New(pool)
	item, err := q.CreateItem(ctx, sqlcgen.CreateItemParams{Name: "Ladder", ItemType: "asset", Unit: "pcs", Condition: "good", Status: "available", Owner: "org", PurchaseCurrency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	after := "damaged"
	id, err := Record(ctx, q, item.ID, domain.EventMarkedDamaged, Input{Quantity: 2, ConditionAfter: &after, Note: "dropped"})
	if err != nil {
		t.Fatal(err)
	}
	var actor, name, sub, note, cond string
	var qty int32
	if err := pool.QueryRow(context.Background(), `SELECT actor_user_id::text, actor_display_name, actor_oidc_subject, note, condition_after, quantity
		FROM item_events WHERE id = $1`, id).Scan(&actor, &name, &sub, &note, &cond, &qty); err != nil {
		t.Fatal(err)
	}
	if actor != u.ID.String() || name != "Robin" || sub != u.OIDCSubject || note != "dropped" || cond != "damaged" || qty != 2 {
		t.Fatalf("event row: %s %s %s %s %s %d", actor, name, sub, note, cond, qty)
	}

	// A job without a user records no actor.
	id, err = Record(context.Background(), q, item.ID, domain.EventRepaired, Input{Quantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	var anon bool
	_ = pool.QueryRow(context.Background(), `SELECT actor_user_id IS NULL AND actor_display_name IS NULL FROM item_events WHERE id = $1`, id).Scan(&anon)
	if !anon {
		t.Fatal("events without a user must have no actor")
	}

	if _, err := Record(ctx, q, item.ID, "teleported", Input{}); err == nil {
		t.Fatal("unknown event types must be refused by the database")
	}
}
