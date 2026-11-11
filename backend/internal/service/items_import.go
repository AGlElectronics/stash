package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// MaxImportRows caps one CSV import.
const MaxImportRows = 5000

// ImportRowResult is the outcome of one CSV row (row 2 is the first data
// row, as a spreadsheet numbers it).
type ImportRowResult struct {
	Row    int      `json:"row"`
	Action string   `json:"action"` // create | update | error
	ItemID string   `json:"item_id,omitempty"`
	Name   string   `json:"name,omitempty"`
	Errors []string `json:"errors,omitempty"`
}

// ImportResult is the report of a CSV import or dry run.
type ImportResult struct {
	DryRun  bool              `json:"dry_run"`
	Applied bool              `json:"applied"`
	Created int               `json:"created"`
	Updated int               `json:"updated"`
	Failed  int               `json:"failed"`
	Rows    []ImportRowResult `json:"rows"`
}

// importColumns maps accepted header names (also those of
// GET /api/export/items.csv) to the field they fill.
var importColumns = map[string]string{
	"id": "id", "name": "name", "description": "description",
	"item_type": "item_type", "type": "item_type", "size": "size",
	"quantity_owned": "quantity_owned", "unit": "unit", "condition": "condition",
	"category": "category", "category_name": "category", "category_id": "category_id",
	"location": "location", "location_path": "location", "location_id": "location_id",
	"managed_by_department": "managed_by", "managed_by_department_name": "managed_by", "managed_by_department_id": "managed_by_id",
	"needed_by_department": "needed_by", "needed_by_department_name": "needed_by", "needed_by_department_id": "needed_by_id",
	"serial_number": "serial_number", "asset_tag": "asset_tag", "qr_code": "qr_code",
	"owner": "owner", "purchase_price": "purchase_price", "purchase_currency": "purchase_currency",
	"purchase_date": "purchase_date", "expiry_date": "expiry_date", "supplier": "supplier",
	"purchase_url": "purchase_url", "replacement_cost": "replacement_cost",
	"minimum_quantity": "minimum_quantity", "notes": "notes",
}

// exportOnlyColumns appear in the items export but are not imported:
// they are computed or changed through their own actions.
var exportOnlyColumns = map[string]bool{
	"status": true, "quantity_damaged": true, "quantity_lost": true, "created_at": true,
	"parent_container_id": true, "parent_container_name": true,
}

// unformula undoes the apostrophe the CSV export puts in front of text that
// starts like a formula, so an export can be imported again unchanged.
func unformula(s string) string {
	if len(s) > 1 && s[0] == '\'' && strings.ContainsRune("=+-@\t\r", rune(s[1])) {
		return s[1:]
	}
	return s
}

// ImportCSV creates items from rows without an id and updates the items
// named by id. Empty cells leave a field unchanged. Everything runs in one
// transaction: with dryRun, or when any row fails, nothing is saved and the
// report says what would have happened. Rows need item.create (new items),
// item.edit (changes) and item.adjust (a different quantity_owned).
func (s *ItemService) ImportCSV(ctx context.Context, r io.Reader, dryRun bool) (ImportResult, error) {
	res := ImportResult{DryRun: dryRun, Rows: []ImportRowResult{}}
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		return res, httperror.Validation("the CSV file is empty or unreadable", nil)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	cols := map[string]int{}
	var unknown []string
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if field, ok := importColumns[key]; ok {
			cols[field] = i
		} else if !exportOnlyColumns[key] && key != "" {
			unknown = append(unknown, h)
		}
	}
	if len(unknown) > 0 {
		return res, httperror.Validation("unknown columns: "+strings.Join(unknown, ", "), map[string]any{"unknown_columns": unknown})
	}
	_, hasID := cols["id"]
	if _, ok := cols["name"]; !ok && !hasID {
		return res, httperror.Validation("the CSV needs a name column (new items) or an id column (updates)", nil)
	}

	var records [][]string
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return res, httperror.Validation("CSV error: "+err.Error(), nil)
		}
		records = append(records, rec)
		if len(records) > MaxImportRows {
			return res, httperror.Validation(fmt.Sprintf("at most %d rows per import", MaxImportRows), nil)
		}
	}

	sys, err := NewSettingsService(s.pool).Get(ctx)
	if err != nil {
		return res, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return res, httperror.Internal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i, rec := range records {
		get := func(field string) string {
			if idx, ok := cols[field]; ok && idx < len(rec) {
				return strings.TrimSpace(unformula(rec[idx]))
			}
			return ""
		}
		row := ImportRowResult{Row: i + 2, Name: get("name")}
		if strings.Join(rec, "") == "" {
			continue
		}
		sp, err := tx.Begin(ctx)
		if err != nil {
			return res, httperror.Internal(err)
		}
		action, itemID, rowErrs := s.importRow(ctx, sp, get, sys)
		if len(rowErrs) > 0 {
			_ = sp.Rollback(ctx)
			row.Action, row.Errors = "error", rowErrs
			res.Failed++
		} else {
			if err := sp.Commit(ctx); err != nil {
				return res, httperror.Internal(err)
			}
			row.Action, row.ItemID = action, itemID
			switch action {
			case "create":
				res.Created++
			case "update":
				res.Updated++
			}
		}
		res.Rows = append(res.Rows, row)
	}

	if dryRun || res.Failed > 0 {
		return res, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return res, httperror.Internal(err)
	}
	res.Applied = true
	return res, nil
}

// importRow applies one row; it returns the action and item id, or the
// problems found.
func (s *ItemService) importRow(ctx context.Context, tx pgx.Tx, get func(string) string, sys SystemSettings) (string, string, []string) {
	var errs []string
	addErr := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }

	optUUID := func(field string) *uuid.UUID {
		v := get(field)
		if v == "" {
			return nil
		}
		id, err := uuid.Parse(v)
		if err != nil {
			addErr("%s is not a valid id", field)
			return nil
		}
		return &id
	}
	optInt := func(field string) *int32 {
		v := get(field)
		if v == "" {
			return nil
		}
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 0 {
			addErr("%s must be a whole number of 0 or more", field)
			return nil
		}
		n32 := int32(n)
		return &n32
	}
	optDate := func(field string) *time.Time {
		v := get(field)
		if v == "" {
			return nil
		}
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			addErr("%s must be a date like 2026-08-14", field)
			return nil
		}
		return &t
	}
	optMoney := func(field string) *string {
		v := strings.ReplaceAll(get(field), ",", ".")
		if v == "" {
			return nil
		}
		if f, err := strconv.ParseFloat(v, 64); err != nil || f < 0 {
			addErr("%s must be an amount like 12.50", field)
			return nil
		}
		return &v
	}
	optStr := func(field string) *string {
		v := get(field)
		if v == "" {
			return nil
		}
		return &v
	}
	lastSegment := func(v string) string {
		if i := strings.LastIndex(v, " / "); i >= 0 {
			return strings.TrimSpace(v[i+3:])
		}
		return v
	}

	id := optUUID("id")
	qty := optInt("quantity_owned")
	minQty := optInt("minimum_quantity")
	purchaseDate, expiryDate := optDate("purchase_date"), optDate("expiry_date")
	price, replacement := optMoney("purchase_price"), optMoney("replacement_cost")
	categoryID, locationID := optUUID("category_id"), optUUID("location_id")
	managedID, neededID := optUUID("managed_by_id"), optUUID("needed_by_id")
	itemType := get("item_type")
	if itemType != "" && !domain.ItemType(itemType).Valid() {
		addErr("item_type must be asset, consumable, container or grouped")
	}
	if len(errs) > 0 {
		return "", "", errs
	}

	if id == nil {
		if !auth.HasPermission(ctx, domain.PermItemCreate) {
			return "", "", []string{"creating items needs item.create"}
		}
		if itemType == "" {
			itemType = string(domain.ItemTypeAsset)
		}
		in := CreateItemInput{
			Name: get("name"), Description: get("description"), ItemType: domain.ItemType(itemType),
			Size: get("size"), Unit: get("unit"), Condition: get("condition"),
			CategoryID: categoryID, CategoryName: get("category"),
			LocationID: locationID, LocationName: lastSegment(get("location")),
			ManagedByDepartmentID: managedID, ManagedByDeptName: get("managed_by"),
			NeededByDepartmentID: neededID, NeededByDeptName: get("needed_by"),
			SerialNumber: optStr("serial_number"), AssetTag: optStr("asset_tag"), QRCode: optStr("qr_code"),
			Owner: get("owner"), PurchasePrice: price, PurchaseCurrency: get("purchase_currency"),
			PurchaseDate: purchaseDate, ExpiryDate: expiryDate, Supplier: optStr("supplier"),
			PurchaseURL: optStr("purchase_url"), ReplacementCost: replacement, Notes: get("notes"),
		}
		if qty != nil {
			in.QuantityOwned = *qty
		} else {
			in.QuantityOwned = 1
		}
		if minQty != nil {
			in.MinimumQuantity = *minQty
		}
		item, err := s.createInTx(ctx, tx, in, sys)
		if err != nil {
			return "", "", []string{errMessage(err)}
		}
		return "create", item.ID.String(), nil
	}

	if !auth.HasPermission(ctx, domain.PermItemEdit) {
		return "", "", []string{"changing items needs item.edit"}
	}
	upd := UpdateItemInput{
		Name: optStr("name"), Description: optStr("description"), Size: optStr("size"),
		Unit: optStr("unit"), Condition: optStr("condition"),
		CategoryID: categoryID, CategoryName: get("category"),
		LocationID: locationID, LocationName: lastSegment(get("location")),
		ManagedByDepartmentID: managedID, ManagedByDeptName: get("managed_by"),
		NeededByDepartmentID: neededID, NeededByDeptName: get("needed_by"),
		SerialNumber: optStr("serial_number"), AssetTag: optStr("asset_tag"), QRCode: optStr("qr_code"),
		Owner: optStr("owner"), PurchasePrice: price, PurchaseCurrency: optStr("purchase_currency"),
		PurchaseDate: purchaseDate, ExpiryDate: expiryDate, Supplier: optStr("supplier"),
		PurchaseURL: optStr("purchase_url"), ReplacementCost: replacement, MinimumQuantity: minQty,
		Notes: optStr("notes"),
	}
	if itemType != "" {
		upd.ItemType = &itemType
	}
	before, err := s.updateInTx(ctx, tx, *id, upd)
	if err != nil {
		return "", "", []string{errMessage(err)}
	}
	if qty != nil && *qty != before.QuantityOwned {
		if !auth.HasPermission(ctx, domain.PermItemAdjust) {
			return "", "", []string{"changing quantity_owned needs item.adjust"}
		}
		if _, err := adjustInTx(ctx, tx, *id, AdjustInput{Delta: *qty - before.QuantityOwned, Note: "CSV import"}); err != nil {
			return "", "", []string{errMessage(err)}
		}
	}
	return "update", id.String(), nil
}

func errMessage(err error) string {
	var he *httperror.Error
	if errors.As(err, &he) && he.Code != httperror.CodeInternal {
		return he.Message
	}
	return "could not save this row"
}
