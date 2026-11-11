package service

import (
	"context"
	"strings"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func TestItemLifecycle(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Inventory Manager")
	svc := NewItemService(pool)
	dept := mustDept(t, pool, "Tech")

	item, err := svc.Create(ctx, CreateItemInput{
		Name: "  Projector ", ItemType: domain.ItemTypeAsset, QuantityOwned: 4,
		ManagedByDepartmentID: &dept.ID, CategoryName: "AV", LocationName: "Store room",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if item.Name != "Projector" || item.AssetTag == nil || !strings.HasPrefix(*item.AssetTag, "ST") {
		t.Fatalf("create should trim the name and assign a barcode: %+v", item)
	}
	if item.CategoryID == nil || item.LocationID == nil || item.Owner == "" && item.PurchaseCurrency != "EUR" {
		t.Fatalf("references/defaults not set: %+v", item)
	}

	_, err = svc.Create(ctx, CreateItemInput{Name: "", ItemType: domain.ItemTypeAsset})
	wantCode(t, err, httperror.CodeValidation)
	_, err = svc.Create(ctx, CreateItemInput{Name: "X", ItemType: "spaceship"})
	wantCode(t, err, httperror.CodeValidation)
	_, err = svc.Create(ctx, CreateItemInput{Name: "Dup", ItemType: domain.ItemTypeAsset, AssetTag: item.AssetTag})
	wantCode(t, err, httperror.CodeConflict)

	cond := "worn"
	if _, err := svc.Update(ctx, item.ID, UpdateItemInput{Condition: &cond}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := svc.AdjustQuantity(ctx, item.ID, AdjustInput{Delta: -10}); err == nil {
		t.Fatal("adjusting below zero must fail")
	}
	if it, err := svc.AdjustQuantity(ctx, item.ID, AdjustInput{Delta: 2, Note: "bought two"}); err != nil || it.QuantityOwned != 6 {
		t.Fatalf("adjust: %v %d", err, it.QuantityOwned)
	}

	if _, err := svc.MarkDamaged(ctx, item.ID, DamageInput{Quantity: 7}); err == nil {
		t.Fatal("damaging more than owned must fail")
	}
	if it, err := svc.MarkDamaged(ctx, item.ID, DamageInput{Quantity: 2}); err != nil || it.QuantityDamaged != 2 {
		t.Fatalf("damage: %v", err)
	}
	if _, err := svc.Repair(ctx, item.ID, RepairInput{Quantity: 3}); err == nil {
		t.Fatal("repairing more than damaged must fail")
	}
	if it, err := svc.Repair(ctx, item.ID, RepairInput{Quantity: 1}); err != nil || it.QuantityDamaged != 1 {
		t.Fatalf("repair: %v", err)
	}
	if it, err := svc.MarkLost(ctx, item.ID, LostInput{Quantity: 1}); err != nil || it.QuantityLost != 1 {
		t.Fatalf("lost: %v", err)
	}
	if _, err := svc.MarkFound(ctx, item.ID, LostInput{Quantity: 2}); err == nil {
		t.Fatal("finding more than lost must fail")
	}
	if it, err := svc.MarkFound(ctx, item.ID, LostInput{Quantity: 1}); err != nil || it.QuantityLost != 0 {
		t.Fatalf("found: %v", err)
	}

	av, err := svc.Availability(ctx, item.ID)
	if err != nil || av.QuantityAvailable != 5 {
		t.Fatalf("availability = %+v %v, want 5 available (6 owned, 1 damaged)", av, err)
	}

	if _, err := svc.Retire(ctx, item.ID, "end of life"); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Retire(ctx, item.ID, "")
	wantCode(t, err, httperror.CodeConflict)
	if it, err := svc.Restore(ctx, item.ID, ""); err != nil || it.Status != string(domain.ItemStatusAvailable) {
		t.Fatalf("restore: %v", err)
	}

	hist, err := svc.ListHistory(ctx, item.ID, ListHistoryInput{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"created": false, "condition_changed": false, "quantity_adjusted": false, "marked_damaged": false,
		"repaired": false, "marked_lost": false, "marked_found": false, "retired": false, "restored": false}
	for _, e := range hist.Events {
		want[e.EventType] = true
	}
	for ev, seen := range want {
		if !seen {
			t.Errorf("history lacks %s", ev)
		}
	}
	if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE entity_id = $1`, item.ID); n < 10 {
		t.Errorf("audit rows = %d, want one per change", n)
	}
}

func TestItemRolesCannotCreateReferenceData(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Volunteer Lead", domain.PermItemCreate, domain.PermItemView)
	_, err := NewItemService(pool).Create(ctx, CreateItemInput{Name: "Tent", ItemType: domain.ItemTypeAsset, ManagedByDeptName: "Brand new dept"})
	wantCode(t, err, httperror.CodeValidation)
	if n := countRows(t, pool, `SELECT count(*) FROM departments`); n != 0 {
		t.Fatalf("a department was created (%d)", n)
	}
	mustDept(t, pool, "Camping")
	if _, err := NewItemService(pool).Create(ctx, CreateItemInput{Name: "Tent", ItemType: domain.ItemTypeAsset, ManagedByDeptName: "camping"}); err != nil {
		t.Fatalf("existing department by name: %v", err)
	}
}

func TestItemImportDryRunAndApply(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Inventory Manager")
	svc := NewItemService(pool)
	mustDept(t, pool, "Stage")
	existing := mustItem(t, pool, "Old speaker", 2, withAssetTag("ST900001"))

	csvText := "\ufeffid,name,type,quantity_owned,managed_by_department_name,expiry_date,notes,status\n" +
		",Cable drum,consumable,5,Stage,2027-01-31,'=not a formula,available\n" +
		existing.ID.String() + ",Speaker (left),,4,,,,\n"

	dry, err := svc.ImportCSV(ctx, strings.NewReader(csvText), true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.Applied || dry.Created != 1 || dry.Updated != 1 || dry.Failed != 0 {
		t.Fatalf("dry run report: %+v", dry)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM items`); n != 1 {
		t.Fatalf("a dry run must not save (%d items)", n)
	}

	res, err := svc.ImportCSV(ctx, strings.NewReader(csvText), false)
	if err != nil || !res.Applied {
		t.Fatalf("import: %+v %v", res, err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM items WHERE name = 'Cable drum' AND notes = '=not a formula' AND expiry_date = '2027-01-31'`); n != 1 {
		t.Fatal("new item not created as described")
	}
	it := mustGetItem(t, pool, existing.ID)
	if it.Name != "Speaker (left)" || it.QuantityOwned != 4 {
		t.Fatalf("update not applied: %+v", it)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM item_events WHERE item_id = $1 AND event_type = 'quantity_adjusted' AND note = 'CSV import'`, existing.ID); n != 1 {
		t.Fatal("a quantity change must leave a history event")
	}
}

func TestItemImportIsAllOrNothing(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Lead", domain.PermItemCreate, domain.PermItemEdit)
	csvText := "name,item_type,quantity_owned,purchase_date,managed_by_department\n" +
		"Good row,asset,1,,\n" +
		"Bad date,asset,1,14.08.2026,\n" +
		"Bad dept,asset,1,,Nowhere\n" +
		"Bad qty,asset,-3,,\n"
	res, err := NewItemService(pool).ImportCSV(context.Background(), strings.NewReader(csvText), false)
	if err != nil {
		t.Fatal(err)
	}
	_ = ctx
	if res.Applied {
		t.Fatal("must not apply")
	}
	res, err = NewItemService(pool).ImportCSV(ctx, strings.NewReader(csvText), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied || res.Failed != 3 || res.Created != 1 {
		t.Fatalf("report: %+v", res)
	}
	if res.Rows[1].Row != 3 || !strings.Contains(res.Rows[1].Errors[0], "purchase_date") {
		t.Fatalf("row 3 error: %+v", res.Rows[1])
	}
	if !strings.Contains(res.Rows[2].Errors[0], "does not exist") {
		t.Fatalf("row 4 error: %+v", res.Rows[2])
	}
	if n := countRows(t, pool, `SELECT count(*) FROM items`); n != 0 {
		t.Fatalf("nothing may be saved when a row fails (%d items)", n)
	}

	_, err = NewItemService(pool).ImportCSV(ctx, strings.NewReader("name,colour\nX,red\n"), true)
	wantCode(t, err, httperror.CodeValidation)

	volunteer := testdb.Actor(t, pool, "Volunteer", domain.PermItemEdit)
	res, err = NewItemService(pool).ImportCSV(volunteer, strings.NewReader("name\nTent\n"), true)
	if err != nil || res.Failed != 1 || !strings.Contains(res.Rows[0].Errors[0], "item.create") {
		t.Fatalf("permission check: %+v %v", res, err)
	}
}
