package service

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// lookupByCodeSQL matches the column list (and order) of GetItem so the row
// scans into sqlcgen.Item. It looks up an item by any of the human-typed/
// scanner-typed identifiers we store on items: asset_tag (the Dymo/AW-style
// barcode), qr_code, or serial_number. Trim is applied client-side; we still
// trim again here because scanners can append \r and we want a forgiving API.
const lookupByCodeSQL = `
SELECT id, name, description, category_id, item_type,
       quantity_owned, quantity_damaged, quantity_lost,
       unit, condition, status,
       location_id, parent_container_id,
       managed_by_department_id, needed_by_department_id,
       serial_number, asset_tag, qr_code, owner,
       purchase_price, purchase_currency, purchase_date,
       supplier, purchase_url, replacement_cost,
       minimum_quantity, primary_image_file_id, notes,
       created_by, created_at, updated_at, retired_at,
       size, parent_group_id, expiry_date
FROM items
WHERE asset_tag = $1
   OR qr_code   = $1
   OR serial_number = $1
ORDER BY
    CASE
        WHEN asset_tag     = $1 THEN 0
        WHEN qr_code       = $1 THEN 1
        WHEN serial_number = $1 THEN 2
        ELSE 3
    END
LIMIT 1
`

// LookupByCode resolves an item by its asset_tag (preferred), qr_code, or
// serial_number. It returns httperror.NotFound if nothing matches and
// httperror.Validation if the code is empty after trimming.
func (s *ItemService) LookupByCode(ctx context.Context, code string) (sqlcgen.Item, error) {
	code = strings.TrimSpace(code)
	// Many scanners append a CR/LF; strip those defensively.
	code = strings.TrimRight(code, "\r\n")
	if code == "" {
		return sqlcgen.Item{}, httperror.Validation("code is required", nil)
	}

	row := s.pool.QueryRow(ctx, lookupByCodeSQL, code)
	var i sqlcgen.Item
	err := row.Scan(
		&i.ID, &i.Name, &i.Description, &i.CategoryID, &i.ItemType,
		&i.QuantityOwned, &i.QuantityDamaged, &i.QuantityLost,
		&i.Unit, &i.Condition, &i.Status,
		&i.LocationID, &i.ParentContainerID,
		&i.ManagedByDepartmentID, &i.NeededByDepartmentID,
		&i.SerialNumber, &i.AssetTag, &i.QrCode, &i.Owner,
		&i.PurchasePrice, &i.PurchaseCurrency, &i.PurchaseDate,
		&i.Supplier, &i.PurchaseUrl, &i.ReplacementCost,
		&i.MinimumQuantity, &i.PrimaryImageFileID, &i.Notes,
		&i.CreatedBy, &i.CreatedAt, &i.UpdatedAt, &i.RetiredAt,
		&i.Size, &i.ParentGroupID, &i.ExpiryDate,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.Item{}, httperror.NotFound("item")
		}
		return sqlcgen.Item{}, httperror.Internal(err)
	}
	return i, nil
}
