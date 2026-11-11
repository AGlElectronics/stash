package audit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func TestRecordWritesSnapshots(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Admin")
	u, _ := auth.FromContext(ctx)
	q := sqlcgen.New(pool)
	id := uuid.New()

	if err := Record(ctx, q, "thing.update", "thing", id, map[string]any{"a": 1}, struct {
		B string `json:"b"`
	}{"two"}, "because"); err != nil {
		t.Fatal(err)
	}
	var before, after []byte
	var actor, name, note string
	if err := pool.QueryRow(context.Background(), `SELECT before_json, after_json, actor_user_id::text, actor_display_name, note
		FROM audit_log WHERE entity_id = $1`, id).Scan(&before, &after, &actor, &name, &note); err != nil {
		t.Fatal(err)
	}
	var b, a map[string]any
	_ = json.Unmarshal(before, &b)
	_ = json.Unmarshal(after, &a)
	if b["a"] != float64(1) || a["b"] != "two" || actor != u.ID.String() || name != "Admin" || note != "because" {
		t.Fatalf("row: %v %v %s %s %s", b, a, actor, name, note)
	}

	if err := Record(context.Background(), q, "job.run", "thing", id, nil, nil, ""); err != nil {
		t.Fatal(err)
	}
	var nulls bool
	_ = pool.QueryRow(context.Background(), `SELECT before_json IS NULL AND after_json IS NULL AND actor_user_id IS NULL
		FROM audit_log WHERE action = 'job.run'`).Scan(&nulls)
	if !nulls {
		t.Fatal("nil snapshots and no user must be stored as NULL")
	}
	if err := Record(ctx, q, "bad", "thing", id, make(chan int), nil, ""); err == nil {
		t.Fatal("unmarshalable snapshots must fail")
	}
}
