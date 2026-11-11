package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func strp(s string) *string { return &s }

func TestLostFoundCreateAndNumbering(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Desk", domain.PermLostfoundCreate, domain.PermLostfoundView)
	svc := NewLostFoundService(pool)

	a, err := svc.Create(ctx, CreateLostFoundInput{Type: "found", ItemName: "Keys", ContactName: "Finder"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Create(ctx, CreateLostFoundInput{Type: "lost", ItemName: "Phone"})
	if err != nil {
		t.Fatal(err)
	}
	if a.ReportNumber != "LF-0001" || b.ReportNumber != "LF-0002" || a.Status != "open" || a.ReportedByUserID == nil {
		t.Fatalf("numbers %s %s, report %+v", a.ReportNumber, b.ReportNumber, a)
	}
	for _, in := range []CreateLostFoundInput{
		{Type: "stolen", ItemName: "x"},
		{Type: "lost"},
	} {
		_, err := svc.Create(ctx, in)
		wantCode(t, err, httperror.CodeValidation)
	}
	photo := uuid.New()
	_, err = svc.Create(ctx, CreateLostFoundInput{Type: "lost", ItemName: "Bag", PhotoFileID: &photo})
	wantCode(t, err, httperror.CodeValidation)

	got, err := svc.GetByReportNumber(ctx, "LF-0002")
	if err != nil || got.ID != b.ID {
		t.Fatalf("by number: %v", err)
	}
	list, err := svc.List(ctx, ListLostFoundInput{Type: "found"})
	if err != nil || list.Total != 1 {
		t.Fatalf("list found: %+v %v", list, err)
	}
	list, _ = svc.List(ctx, ListLostFoundInput{Search: "pho"})
	if list.Total != 1 {
		t.Fatalf("search: %d", list.Total)
	}
	_, err = svc.List(ctx, ListLostFoundInput{Status: "lost-forever"})
	wantCode(t, err, httperror.CodeValidation)
}

func TestLostFoundUpdatePermissions(t *testing.T) {
	pool := testdb.New(t)
	desk := testdb.Actor(t, pool, "Front desk", domain.PermLostfoundView, domain.PermLostfoundCreate, domain.PermLostfoundResolve)
	lead := testdb.Actor(t, pool, "ConOps lead")
	svc := NewLostFoundService(pool)
	r, err := svc.Create(desk, CreateLostFoundInput{Type: "found", ItemName: "Wallet"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Update(desk, r.ID, UpdateLostFoundInput{ItemName: strp("Black wallet")})
	wantCode(t, err, httperror.CodeForbidden)

	ret, err := svc.Update(desk, r.ID, UpdateLostFoundInput{Status: strp("returned"), ReturnedTo: strp("Alex")})
	if err != nil || ret.Status != "returned" || ret.ReturnedTo != "Alex" || !ret.ResolvedAt.Valid {
		t.Fatalf("return: %+v %v", ret, err)
	}
	_, err = svc.Update(desk, r.ID, UpdateLostFoundInput{Status: strp("open")})
	wantCode(t, err, httperror.CodeForbidden)

	re, err := svc.Update(lead, r.ID, UpdateLostFoundInput{Status: strp("open")})
	if err != nil || re.Status != "open" || re.ReturnedTo != "" || re.ResolvedAt.Valid {
		t.Fatalf("reopen should clear the resolution: %+v %v", re, err)
	}
	if _, err := svc.Update(lead, r.ID, UpdateLostFoundInput{ItemName: strp("Black wallet")}); err != nil {
		t.Fatalf("lead edit: %v", err)
	}
	for _, action := range []string{"lostfound.create", "lostfound.return", "lostfound.reopen", "lostfound.update"} {
		if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE entity_id = $1 AND action = $2`, r.ID, action); n != 1 {
			t.Errorf("%s audit rows = %d", action, n)
		}
	}

	if err := svc.Delete(lead, r.ID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Get(lead, r.ID)
	wantCode(t, err, httperror.CodeNotFound)
	_, err = svc.Update(lead, uuid.New(), UpdateLostFoundInput{ItemName: strp("x")})
	wantCode(t, err, httperror.CodeNotFound)
}

func TestLostFoundSacredReportIsOptIn(t *testing.T) {
	pool := testdb.New(t)
	lead := testdb.Actor(t, pool, "Lead")
	svc := NewLostFoundService(pool)
	first, _ := svc.Create(lead, CreateLostFoundInput{Type: "found", ItemName: "Rubber duck"})

	if _, err := svc.Update(lead, first.ID, UpdateLostFoundInput{Description: strp("yellow")}); err != nil {
		t.Fatalf("off by default, LF-0001 is an ordinary report: %v", err)
	}
	if _, err := NewSettingsService(pool).PatchJSON(context.Background(), []byte(`{"preserve_sacred_lostfound": true}`)); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Update(lead, first.ID, UpdateLostFoundInput{Description: strp("blue")})
	wantCode(t, err, httperror.CodeForbidden)
	wantCode(t, svc.Delete(lead, first.ID), httperror.CodeForbidden)
	if _, err := svc.Update(lead, first.ID, UpdateLostFoundInput{Status: strp("closed")}); err != nil {
		t.Fatalf("resolving stays possible: %v", err)
	}
}

func TestLostFoundPhotoMustBelongToReport(t *testing.T) {
	pool := testdb.New(t)
	lead := testdb.Actor(t, pool, "Lead")
	svc := NewLostFoundService(pool)
	a, _ := svc.Create(lead, CreateLostFoundInput{Type: "found", ItemName: "Hat"})
	b, _ := svc.Create(lead, CreateLostFoundInput{Type: "found", ItemName: "Scarf"})
	var photo uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO files (entity_type, entity_id, file_type, bucket, storage_key, original_filename, content_type, size_bytes)
		VALUES ('lostfound', $1, 'image', 'b', $2, 'hat.jpg', 'image/jpeg', 1) RETURNING id`, a.ID, "stash/lostfound/"+a.ID.String()+"/x.jpg").Scan(&photo); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Update(lead, b.ID, UpdateLostFoundInput{PhotoFileID: &photo})
	wantCode(t, err, httperror.CodeValidation)
	if got, err := svc.Update(lead, a.ID, UpdateLostFoundInput{PhotoFileID: &photo}); err != nil || got.PhotoFileID == nil {
		t.Fatalf("own photo: %v", err)
	}
	if got, err := svc.Update(lead, a.ID, UpdateLostFoundInput{ClearPhoto: true}); err != nil || got.PhotoFileID != nil {
		t.Fatalf("clear photo: %v", err)
	}
}
