package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

type signoutFixture struct {
	pool *pgxpool.Pool
	svc  *SignoutService
	ctx  context.Context
	dept uuid.UUID
}

func newSignoutFixture(t *testing.T) signoutFixture {
	t.Helper()
	pool := testdb.New(t)
	return signoutFixture{
		pool: pool,
		svc:  NewSignoutService(pool),
		ctx:  testdb.Actor(t, pool, "Logistics Lead"),
		dept: mustDept(t, pool, "Stage").ID,
	}
}

func (f signoutFixture) create(t *testing.T, confirm bool, lines ...SignoutItemInput) SignoutWithItems {
	t.Helper()
	out, err := f.svc.Create(f.ctx, CreateSignoutInput{
		ToDepartmentID: f.dept,
		Purpose:        "main stage",
		ReceivedByName: "Sam",
		Items:          lines,
		Confirm:        confirm,
	})
	if err != nil {
		t.Fatalf("create sign-out: %v", err)
	}
	return out
}

func line(id uuid.UUID, qty int32) SignoutItemInput {
	return SignoutItemInput{ItemID: id, Quantity: qty}
}

func TestSignoutDraftConfirmReturnClose(t *testing.T) {
	f := newSignoutFixture(t)
	cable := mustItem(t, f.pool, "XLR cable", 10)

	draft := f.create(t, false, line(cable.ID, 4))
	if draft.Signout.Status != string(domain.SignoutStatusDraft) {
		t.Fatalf("status = %s, want draft", draft.Signout.Status)
	}
	if draft.Signout.SignedOutAt.Valid {
		t.Fatal("draft must not have signed_out_at")
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM item_events WHERE item_id = $1 AND event_type = 'signed_out'`, cable.ID); n != 0 {
		t.Fatalf("draft wrote %d signed_out events, want 0", n)
	}

	confirmed, err := f.svc.Confirm(f.ctx, draft.Signout.ID)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.Signout.Status != string(domain.SignoutStatusSignedOut) || !confirmed.Signout.SignedOutAt.Valid {
		t.Fatalf("after confirm: status=%s signed_out_at=%v", confirmed.Signout.Status, confirmed.Signout.SignedOutAt.Valid)
	}
	if confirmed.Signout.SignedOutByUserID == nil {
		t.Fatal("confirm must stamp signed_out_by_user_id")
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM item_events WHERE item_id = $1 AND event_type = 'signed_out' AND quantity = 4`, cable.ID); n != 1 {
		t.Fatalf("signed_out events = %d, want 1", n)
	}

	itemLine := confirmed.Items[0].ID
	partial, err := f.svc.Return(f.ctx, draft.Signout.ID, []ReturnLine{{SignoutItemID: itemLine, ReturnedDelta: 1}})
	if err != nil {
		t.Fatalf("partial return: %v", err)
	}
	if partial.Signout.Status != string(domain.SignoutStatusPartiallyReturned) {
		t.Fatalf("status = %s, want partially_returned", partial.Signout.Status)
	}
	if _, err := f.svc.Close(f.ctx, draft.Signout.ID); err == nil {
		t.Fatal("closing a partially returned sign-out must fail")
	} else {
		wantCode(t, err, httperror.CodeConflict)
	}

	done, err := f.svc.Return(f.ctx, draft.Signout.ID, []ReturnLine{{SignoutItemID: itemLine, ReturnedDelta: 3}})
	if err != nil {
		t.Fatalf("final return: %v", err)
	}
	if done.Signout.Status != string(domain.SignoutStatusReturned) || !done.Signout.ReturnedAt.Valid {
		t.Fatalf("status = %s returned_at=%v, want returned", done.Signout.Status, done.Signout.ReturnedAt.Valid)
	}
	if done.Items[0].QuantityReturned != 4 {
		t.Fatalf("quantity_returned = %d, want 4", done.Items[0].QuantityReturned)
	}

	closed, err := f.svc.Close(f.ctx, draft.Signout.ID)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if closed.Signout.Status != string(domain.SignoutStatusClosed) || !closed.Signout.ClosedAt.Valid {
		t.Fatalf("status = %s, want closed", closed.Signout.Status)
	}

	actions := countRows(t, f.pool, `SELECT count(DISTINCT action) FROM audit_log WHERE entity_id = $1 AND action IN
		('signout.create','signout.confirm','signout.return','signout.close')`, draft.Signout.ID)
	if actions != 4 {
		t.Fatalf("audit actions = %d, want create, confirm, return and close", actions)
	}
}

func TestSignoutFastFlowSignsOutImmediately(t *testing.T) {
	f := newSignoutFixture(t)
	strip := mustItem(t, f.pool, "Power strip", 3)

	out := f.create(t, true, line(strip.ID, 1))
	if out.Signout.Status != string(domain.SignoutStatusSignedOut) || out.Signout.SignedOutByUserID == nil {
		t.Fatalf("fast flow: status=%s signed_out_by=%v", out.Signout.Status, out.Signout.SignedOutByUserID)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM audit_log WHERE entity_id = $1 AND action = 'signout.create_and_confirm'`, out.Signout.ID); n != 1 {
		t.Fatalf("audit create_and_confirm = %d, want 1", n)
	}
	if _, err := f.svc.Confirm(f.ctx, out.Signout.ID); err == nil {
		t.Fatal("confirming an already signed-out sign-out must fail")
	} else {
		wantCode(t, err, httperror.CodeConflict)
	}
}

func TestSignoutAvailabilityIsEnforced(t *testing.T) {
	f := newSignoutFixture(t)
	radio := mustItem(t, f.pool, "Radio", 5)

	f.create(t, true, line(radio.ID, 3))

	_, err := f.svc.Create(f.ctx, CreateSignoutInput{ToDepartmentID: f.dept, Items: []SignoutItemInput{line(radio.ID, 3)}})
	wantCode(t, err, httperror.CodeConflict)

	// Damaged and lost units are not available either.
	if _, err := f.pool.Exec(context.Background(), `UPDATE items SET quantity_damaged = 1, quantity_lost = 1 WHERE id = $1`, radio.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Create(f.ctx, CreateSignoutInput{ToDepartmentID: f.dept, Items: []SignoutItemInput{line(radio.ID, 1)}})
	wantCode(t, err, httperror.CodeConflict)

	if n := countRows(t, f.pool, `SELECT count(*) FROM department_signouts`); n != 1 {
		t.Fatalf("failed creates must roll back: %d sign-outs, want 1", n)
	}
}

func TestSignoutCreateValidation(t *testing.T) {
	f := newSignoutFixture(t)
	item := mustItem(t, f.pool, "Gaffer tape", 2)
	retired := mustItem(t, f.pool, "Old projector", 1)
	if _, err := f.pool.Exec(context.Background(), `UPDATE items SET retired_at = now(), status = 'retired' WHERE id = $1`, retired.ID); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		in   CreateSignoutInput
	}{
		{"no items", CreateSignoutInput{ToDepartmentID: f.dept}},
		{"no department", CreateSignoutInput{Items: []SignoutItemInput{line(item.ID, 1)}}},
		{"zero quantity", CreateSignoutInput{ToDepartmentID: f.dept, Items: []SignoutItemInput{line(item.ID, 0)}}},
		{"unknown item", CreateSignoutInput{ToDepartmentID: f.dept, Items: []SignoutItemInput{line(uuid.New(), 1)}}},
		{"retired item", CreateSignoutInput{ToDepartmentID: f.dept, Items: []SignoutItemInput{line(retired.ID, 1)}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.Create(f.ctx, tc.in)
			wantCode(t, err, httperror.CodeValidation)
		})
	}
}

func TestSignoutReturnOutcomesUpdateInventory(t *testing.T) {
	f := newSignoutFixture(t)
	batteries := mustItem(t, f.pool, "AA batteries", 20)
	so := f.create(t, true, line(batteries.ID, 10))
	lineID := so.Items[0].ID

	out, err := f.svc.Return(f.ctx, so.Signout.ID, []ReturnLine{{
		SignoutItemID: lineID, ReturnedDelta: 4, ConsumedDelta: 3, LostDelta: 2, DamagedDelta: 1,
	}})
	if err != nil {
		t.Fatalf("return: %v", err)
	}
	if out.Signout.Status != string(domain.SignoutStatusReturned) {
		t.Fatalf("status = %s, want returned", out.Signout.Status)
	}
	it := mustGetItem(t, f.pool, batteries.ID)
	if it.QuantityOwned != 17 || it.QuantityLost != 2 || it.QuantityDamaged != 1 {
		t.Fatalf("item after return: owned=%d lost=%d damaged=%d, want 17/2/1", it.QuantityOwned, it.QuantityLost, it.QuantityDamaged)
	}
	for _, ev := range []string{"returned", "consumed", "marked_lost", "marked_damaged"} {
		if n := countRows(t, f.pool, `SELECT count(*) FROM item_events WHERE item_id = $1 AND event_type = $2 AND related_signout_id = $3`, batteries.ID, ev, so.Signout.ID); n != 1 {
			t.Errorf("%s events = %d, want 1", ev, n)
		}
	}
}

func TestSignoutReturnValidation(t *testing.T) {
	f := newSignoutFixture(t)
	a := mustItem(t, f.pool, "Ladder", 2)
	b := mustItem(t, f.pool, "Toolbox", 2)
	so := f.create(t, true, line(a.ID, 2))
	other := f.create(t, true, line(b.ID, 1))
	lineID := so.Items[0].ID

	_, err := f.svc.Return(f.ctx, so.Signout.ID, nil)
	wantCode(t, err, httperror.CodeValidation)

	_, err = f.svc.Return(f.ctx, so.Signout.ID, []ReturnLine{{SignoutItemID: lineID, ReturnedDelta: 3}})
	wantCode(t, err, httperror.CodeValidation)

	_, err = f.svc.Return(f.ctx, so.Signout.ID, []ReturnLine{{SignoutItemID: lineID, ReturnedDelta: -1}})
	wantCode(t, err, httperror.CodeValidation)

	_, err = f.svc.Return(f.ctx, so.Signout.ID, []ReturnLine{{SignoutItemID: other.Items[0].ID, ReturnedDelta: 1}})
	wantCode(t, err, httperror.CodeValidation)

	_, err = f.svc.Return(f.ctx, uuid.New(), []ReturnLine{{SignoutItemID: lineID, ReturnedDelta: 1}})
	wantCode(t, err, httperror.CodeNotFound)

	draft := f.create(t, false, line(b.ID, 1))
	_, err = f.svc.Return(f.ctx, draft.Signout.ID, []ReturnLine{{SignoutItemID: draft.Items[0].ID, ReturnedDelta: 1}})
	wantCode(t, err, httperror.CodeConflict)
}

func TestSignoutCancel(t *testing.T) {
	f := newSignoutFixture(t)
	item := mustItem(t, f.pool, "Extension cord", 2)

	draft := f.create(t, false, line(item.ID, 2))
	out, err := f.svc.Cancel(f.ctx, draft.Signout.ID, "not needed")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if out.Signout.Status != string(domain.SignoutStatusCancelled) || !out.Signout.CancelledAt.Valid {
		t.Fatalf("status = %s, want cancelled", out.Signout.Status)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM audit_log WHERE entity_id = $1 AND action = 'signout.cancel' AND note = 'not needed'`, draft.Signout.ID); n != 1 {
		t.Fatalf("cancel audit rows = %d, want 1", n)
	}
	// A cancelled draft holds no stock, so the full quantity can go out again.
	f.create(t, true, line(item.ID, 2))

	active := f.create(t, false, line(mustItem(t, f.pool, "Fan", 1).ID, 1))
	if _, err := f.svc.Confirm(f.ctx, active.Signout.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Cancel(f.ctx, active.Signout.ID, "")
	wantCode(t, err, httperror.CodeConflict)

	_, err = f.svc.Confirm(f.ctx, draft.Signout.ID)
	wantCode(t, err, httperror.CodeConflict)
}

func TestSignoutForcedBarcode(t *testing.T) {
	f := newSignoutFixture(t)
	if _, err := NewSettingsService(f.pool).PatchJSON(context.Background(), []byte(`{"force_barcode_on_signout": true}`)); err != nil {
		t.Fatal(err)
	}
	mic := mustItem(t, f.pool, "Microphone", 2, withAssetTag("ST000042"))
	other := mustItem(t, f.pool, "Mic stand", 2, withAssetTag("ST000043"))

	_, err := f.svc.Create(f.ctx, CreateSignoutInput{ToDepartmentID: f.dept, Items: []SignoutItemInput{line(mic.ID, 1)}})
	wantCode(t, err, httperror.CodeValidation)

	wrong := line(mic.ID, 1)
	wrong.ScannedCodes = []string{*other.AssetTag}
	_, err = f.svc.Create(f.ctx, CreateSignoutInput{ToDepartmentID: f.dept, Items: []SignoutItemInput{wrong}})
	wantCode(t, err, httperror.CodeValidation)

	right := line(mic.ID, 1)
	right.ScannedCodes = []string{"ST000042"}
	out := f.create(t, true, right)
	if out.Items[0].Notes != "scanned: ST000042" {
		t.Fatalf("notes = %q, want scanned code recorded", out.Items[0].Notes)
	}
}

func TestSignoutListFilters(t *testing.T) {
	f := newSignoutFixture(t)
	item := mustItem(t, f.pool, "Walkie", 10)
	f.create(t, false, line(item.ID, 1))
	f.create(t, true, line(item.ID, 1))

	all, err := f.svc.List(f.ctx, ListSignoutsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 2 {
		t.Fatalf("total = %d, want 2", all.Total)
	}
	drafts, err := f.svc.List(f.ctx, ListSignoutsInput{Status: "draft"})
	if err != nil {
		t.Fatal(err)
	}
	if drafts.Total != 1 || drafts.Items[0].Status != "draft" {
		t.Fatalf("draft filter: total=%d", drafts.Total)
	}
	bySearch, err := f.svc.List(f.ctx, ListSignoutsInput{Search: "sam"})
	if err != nil {
		t.Fatal(err)
	}
	if bySearch.Total != 2 {
		t.Fatalf("search by receiver: total=%d, want 2", bySearch.Total)
	}
	got, err := f.svc.Get(f.ctx, drafts.Items[0].ID)
	if err != nil || len(got.Items) != 1 {
		t.Fatalf("get: %v items=%d", err, len(got.Items))
	}
	_, err = f.svc.Get(f.ctx, uuid.New())
	wantCode(t, err, httperror.CodeNotFound)
}
