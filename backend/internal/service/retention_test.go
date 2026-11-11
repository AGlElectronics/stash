package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

type removedObject struct {
	key        string
	entityType string
	entityID   uuid.UUID
}

type fakeObjects struct{ removed []removedObject }

func (f *fakeObjects) RemoveEntityObject(_ context.Context, key, entityType string, entityID uuid.UUID) {
	f.removed = append(f.removed, removedObject{key, entityType, entityID})
}

// mustReport inserts a report through the service (so its audit trail is
// real) and returns its id.
func mustReport(t *testing.T, pool *pgxpool.Pool, ctx context.Context, item, contact string) uuid.UUID {
	t.Helper()
	r, err := NewLostFoundService(pool).Create(ctx, CreateLostFoundInput{
		Type: "found", ItemName: item, ContactName: contact, ContactInfo: contact + "@example.org",
	})
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	return r.ID
}

func TestRetentionAfterClose(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Desk")
	old := mustReport(t, pool, ctx, "Old wallet", "Alex")
	recent := mustReport(t, pool, ctx, "Recent hat", "Sam")
	open := mustReport(t, pool, ctx, "Open umbrella", "Kim")
	now := time.Now()
	if _, err := pool.Exec(ctx, `UPDATE lost_found_reports SET status = 'returned', resolved_at = $2, telegram_chat_id = 42 WHERE id = $1`, old, now.Add(-721*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE lost_found_reports SET status = 'closed', resolved_at = $2 WHERE id = $1`, recent, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	key := "stash/lostfound/" + old.String() + "/photo.jpg"
	if _, err := pool.Exec(ctx, `INSERT INTO files (entity_type, entity_id, file_type, bucket, storage_key, original_filename, content_type, size_bytes)
		VALUES ('lostfound', $1, 'image', 'b', $2, 'alex-id-card.jpg', 'image/jpeg', 1)`, old, key); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO audit_log (action, entity_type, entity_id, after_json) VALUES ('file.register', 'file', $1, $2)`,
		uuid.New(), `{"entity_type":"lostfound","entity_id":"`+old.String()+`","original_filename":"alex-id-card.jpg"}`); err != nil {
		t.Fatal(err)
	}

	objects := &fakeObjects{}
	res, err := NewRetentionService(pool, NewSettingsService(pool), objects).PurgeLostFound(context.Background(), now)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if len(res.Deleted) != 1 {
		t.Fatalf("deleted %v, want only the old returned report", res.Deleted)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM lost_found_reports WHERE id = ANY($1)`, []uuid.UUID{recent, open}); n != 2 {
		t.Fatalf("recent and open reports must stay, %d left", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM lost_found_reports WHERE id = $1`, old); n != 0 {
		t.Fatal("old report still there")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM files WHERE entity_id = $1`, old); n != 0 {
		t.Fatal("old report's files still there")
	}
	if len(objects.removed) != 1 || objects.removed[0].key != key {
		t.Fatalf("objects removed: %+v", objects.removed)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE action = 'lostfound.purge' AND entity_id = $1 AND actor_user_id IS NULL`, old); n != 1 {
		t.Fatalf("purge audit rows = %d, want 1", n)
	}
	var leaks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log
		WHERE coalesce(before_json::text,'') || coalesce(after_json::text,'') ~ '(Alex|alex-id-card|"telegram_chat_id": 42)'`).Scan(&leaks); err != nil {
		t.Fatal(err)
	}
	if leaks != 0 {
		t.Fatalf("%d audit rows still hold the purged guest's data", leaks)
	}
	var stillThere string
	_ = pool.QueryRow(ctx, `SELECT after_json::text FROM audit_log WHERE action = 'lostfound.create' AND entity_id = $1`, recent).Scan(&stillThere)
	if !strings.Contains(stillThere, "Sam") {
		t.Fatalf("audit of kept reports must stay intact: %s", stillThere)
	}
}

func TestRetentionAfterConAndSacred(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Admin")
	first := mustReport(t, pool, ctx, "Test item", "Nobody")
	mustReport(t, pool, ctx, "Scarf", "Lee")
	settings := NewSettingsService(pool)
	if _, err := settings.PatchJSON(ctx, []byte(`{"lostfound_retention_mode":"after_con","con_end_date":"2026-08-16","lostfound_retention_days":14,"preserve_sacred_lostfound":true}`)); err != nil {
		t.Fatal(err)
	}
	svc := NewRetentionService(pool, settings, nil)

	before, _ := time.Parse(time.RFC3339, "2026-08-29T12:00:00Z")
	if res, err := svc.PurgeLostFound(ctx, before); err != nil || len(res.Deleted) != 0 {
		t.Fatalf("before the period ends: %v %v", res.Deleted, err)
	}
	after, _ := time.Parse(time.RFC3339, "2026-08-31T00:00:01Z")
	res, err := svc.PurgeLostFound(ctx, after)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) != 1 || res.Deleted[0] != "LF-0002" {
		t.Fatalf("deleted %v, want LF-0002 (LF-0001 is protected)", res.Deleted)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM lost_found_reports WHERE id = $1`, first); n != 1 {
		t.Fatal("LF-0001 must survive while protected")
	}
}

func TestRetentionOff(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Admin")
	id := mustReport(t, pool, ctx, "Bag", "Max")
	if _, err := pool.Exec(ctx, `UPDATE lost_found_reports SET status='closed', resolved_at = now() - interval '10 years' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	settings := NewSettingsService(pool)
	if _, err := settings.PatchJSON(ctx, []byte(`{"lostfound_retention_mode":"off"}`)); err != nil {
		t.Fatal(err)
	}
	res, err := NewRetentionService(pool, settings, nil).PurgeLostFound(ctx, time.Now())
	if err != nil || len(res.Deleted) != 0 {
		t.Fatalf("off must delete nothing: %v %v", res.Deleted, err)
	}
}
