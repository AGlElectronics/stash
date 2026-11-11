package service

import (
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func TestPurchasesAddToStock(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Logistics")
	item := mustItem(t, pool, "Gaffer tape", 4)
	svc := NewPurchaseService(pool)

	p, err := svc.Record(ctx, item.ID, PurchaseInput{Quantity: 10, UnitPrice: "7,50", Supplier: "Tape Co", Reference: "INV-1", AddToStock: true})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if p.UnitPrice == nil || *p.UnitPrice != "7.50" || p.Currency != "EUR" || !p.AddedToStock {
		t.Fatalf("purchase: %+v", p)
	}
	if it := mustGetItem(t, pool, item.ID); it.QuantityOwned != 14 {
		t.Fatalf("stock = %d, want 14", it.QuantityOwned)
	}
	if _, err := svc.Record(ctx, item.ID, PurchaseInput{Quantity: 2, PurchasedOn: "2026-01-05"}); err != nil {
		t.Fatal(err)
	}
	if it := mustGetItem(t, pool, item.ID); it.QuantityOwned != 14 {
		t.Fatal("a purchase without add_to_stock must not change stock")
	}
	list, err := svc.List(ctx, item.ID)
	if err != nil || len(list) != 2 || list[1].PurchasedOn != "2026-01-05" {
		t.Fatalf("list: %+v %v", list, err)
	}
	_, err = svc.Record(ctx, item.ID, PurchaseInput{Quantity: 0})
	wantCode(t, err, httperror.CodeValidation)
	_, err = svc.Record(ctx, item.ID, PurchaseInput{Quantity: 1, UnitPrice: "cheap"})
	wantCode(t, err, httperror.CodeValidation)
}

func TestRequestsFlow(t *testing.T) {
	pool := testdb.New(t)
	volunteer := testdb.Actor(t, pool, "Volunteer", domain.PermRequestCreate)
	other := testdb.Actor(t, pool, "Other volunteer", domain.PermRequestCreate)
	logistics := testdb.Actor(t, pool, "Logistics", domain.PermRequestManage)
	svc := NewRequestService(pool)
	dept := mustDept(t, pool, "Stage")

	r, err := svc.Create(volunteer, RequestInput{Title: "Extra radios", Quantity: 4, DepartmentID: &dept.ID, NeededBy: "2026-08-14"})
	if err != nil || r.Status != "open" || r.DepartmentName == nil || *r.DepartmentName != "Stage" {
		t.Fatalf("create: %+v %v", r, err)
	}
	_, err = svc.Decide(other, r.ID, "cancelled", "")
	wantCode(t, err, httperror.CodeForbidden)
	_, err = svc.Decide(volunteer, r.ID, "approved", "")
	wantCode(t, err, httperror.CodeForbidden)

	if r, err = svc.Decide(logistics, r.ID, "approved", "we have spares"); err != nil || r.Status != "approved" {
		t.Fatalf("approve: %+v %v", r, err)
	}
	if r, err = svc.Decide(logistics, r.ID, "fulfilled", ""); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Decide(logistics, r.ID, "rejected", "")
	wantCode(t, err, httperror.CodeConflict)

	own, _ := svc.Create(volunteer, RequestInput{Title: "Ladder"})
	if own, err = svc.Decide(volunteer, own.ID, "cancelled", "not needed"); err != nil || own.Status != "cancelled" {
		t.Fatalf("own cancel: %+v %v", own, err)
	}
	_, err = svc.Create(volunteer, RequestInput{})
	wantCode(t, err, httperror.CodeValidation)

	open, err := svc.List(logistics, "fulfilled")
	if err != nil || len(open) != 1 {
		t.Fatalf("list fulfilled: %d %v", len(open), err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE entity_type = 'request'`); n != 5 {
		t.Fatalf("request audit rows = %d, want 5", n)
	}
}

func TestPackingListToSignout(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Stage lead")
	dept := mustDept(t, pool, "Stage")
	mic := mustItem(t, pool, "Microphone", 6)
	cable := mustItem(t, pool, "XLR cable", 20)
	svc := NewPackingService(pool)

	pl, err := svc.Create(ctx, PackingListInput{Name: "Main stage Friday", DepartmentID: &dept.ID, EventDate: "2026-08-14"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = svc.CreateSignout(ctx, pl.ID, nil, "")
	wantCode(t, err, httperror.CodeValidation)

	pl, err = svc.SetItems(ctx, pl.ID, []PackingLineInput{
		{ItemID: mic.ID, Quantity: 4, Packed: 4},
		{ItemID: cable.ID, Quantity: 10, Packed: 3, Note: "the long ones"},
	})
	if err != nil {
		t.Fatalf("set items: %v", err)
	}
	if pl.ItemCount != 2 || pl.PackedCount != 1 || pl.Items[0].Available != 6 {
		t.Fatalf("list: %+v", pl)
	}
	_, err = svc.SetItems(ctx, pl.ID, []PackingLineInput{{ItemID: mic.ID, Quantity: 1, Packed: 2}})
	wantCode(t, err, httperror.CodeValidation)

	so, err := svc.CreateSignout(ctx, pl.ID, nil, "")
	if err != nil {
		t.Fatalf("sign-out: %v", err)
	}
	if so.Signout.Status != string(domain.SignoutStatusDraft) || len(so.Items) != 2 || so.Signout.Purpose != "Main stage Friday" {
		t.Fatalf("draft: %+v", so)
	}
	got, _ := svc.Get(ctx, pl.ID)
	if got.SignoutID == nil || *got.SignoutID != so.Signout.ID {
		t.Fatal("the list should remember its sign-out")
	}
	if err := svc.Delete(ctx, pl.ID); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM department_signouts`); n != 1 {
		t.Fatal("deleting a list must keep its sign-out")
	}
}
