-- name: GetItem :one
SELECT * FROM items WHERE id = $1;

-- name: ListItems :many
WITH RECURSIVE loc_sub AS (
    SELECT id FROM locations
    WHERE sqlc.narg('location_id')::uuid IS NOT NULL AND id = sqlc.narg('location_id')::uuid
    UNION ALL
    SELECT l.id FROM locations l INNER JOIN loc_sub ON l.parent_id = loc_sub.id
)
SELECT items.*
FROM items
WHERE
    (sqlc.narg('search')::text IS NULL OR sqlc.narg('search') = '' OR
     lower(items.name) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
     lower(coalesce(items.serial_number, '')) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
     lower(coalesce(items.asset_tag, ''))     LIKE '%' || lower(sqlc.narg('search')::text) || '%')
    AND (sqlc.narg('category_id')::uuid IS NULL OR items.category_id = sqlc.narg('category_id')::uuid)
    AND (sqlc.narg('managed_dept_id')::uuid IS NULL OR items.managed_by_department_id = sqlc.narg('managed_dept_id')::uuid)
    AND (sqlc.narg('needed_dept_id')::uuid IS NULL OR items.needed_by_department_id = sqlc.narg('needed_dept_id')::uuid)
    AND (sqlc.narg('parent_container_id')::uuid IS NULL OR items.parent_container_id = sqlc.narg('parent_container_id')::uuid)
    AND (sqlc.narg('parent_group_id')::uuid IS NULL OR items.parent_group_id = sqlc.narg('parent_group_id')::uuid)
    AND (
        sqlc.narg('location_id')::uuid IS NULL
        OR (
            COALESCE(sqlc.narg('include_location_descendants')::bool, FALSE) = FALSE
            AND items.location_id = sqlc.narg('location_id')::uuid
        )
        OR (
            COALESCE(sqlc.narg('include_location_descendants')::bool, FALSE) = TRUE
            AND items.location_id IN (SELECT id FROM loc_sub)
        )
    )
    AND (sqlc.narg('item_type')::text IS NULL OR sqlc.narg('item_type') = '' OR items.item_type = sqlc.narg('item_type')::text)
    AND (sqlc.narg('status')::text IS NULL OR sqlc.narg('status') = '' OR items.status = sqlc.narg('status')::text)
    AND (sqlc.narg('include_retired')::bool = TRUE OR items.retired_at IS NULL)
ORDER BY items.name
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountItems :one
WITH RECURSIVE loc_sub AS (
    SELECT id FROM locations
    WHERE sqlc.narg('location_id')::uuid IS NOT NULL AND id = sqlc.narg('location_id')::uuid
    UNION ALL
    SELECT l.id FROM locations l INNER JOIN loc_sub ON l.parent_id = loc_sub.id
)
SELECT count(*)
FROM items
WHERE
    (sqlc.narg('search')::text IS NULL OR sqlc.narg('search') = '' OR
     lower(items.name) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
     lower(coalesce(items.serial_number, '')) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
     lower(coalesce(items.asset_tag, ''))     LIKE '%' || lower(sqlc.narg('search')::text) || '%')
    AND (sqlc.narg('category_id')::uuid IS NULL OR items.category_id = sqlc.narg('category_id')::uuid)
    AND (sqlc.narg('managed_dept_id')::uuid IS NULL OR items.managed_by_department_id = sqlc.narg('managed_dept_id')::uuid)
    AND (sqlc.narg('needed_dept_id')::uuid IS NULL OR items.needed_by_department_id = sqlc.narg('needed_dept_id')::uuid)
    AND (sqlc.narg('parent_container_id')::uuid IS NULL OR items.parent_container_id = sqlc.narg('parent_container_id')::uuid)
    AND (sqlc.narg('parent_group_id')::uuid IS NULL OR items.parent_group_id = sqlc.narg('parent_group_id')::uuid)
    AND (
        sqlc.narg('location_id')::uuid IS NULL
        OR (
            COALESCE(sqlc.narg('include_location_descendants')::bool, FALSE) = FALSE
            AND items.location_id = sqlc.narg('location_id')::uuid
        )
        OR (
            COALESCE(sqlc.narg('include_location_descendants')::bool, FALSE) = TRUE
            AND items.location_id IN (SELECT id FROM loc_sub)
        )
    )
    AND (sqlc.narg('item_type')::text IS NULL OR sqlc.narg('item_type') = '' OR items.item_type = sqlc.narg('item_type')::text)
    AND (sqlc.narg('status')::text IS NULL OR sqlc.narg('status') = '' OR items.status = sqlc.narg('status')::text)
    AND (sqlc.narg('include_retired')::bool = TRUE OR items.retired_at IS NULL);

-- name: CreateItem :one
INSERT INTO items (
    name, description, category_id, item_type, size,
    quantity_owned, unit, condition, status,
    location_id, parent_container_id, parent_group_id,
    managed_by_department_id, needed_by_department_id,
    serial_number, asset_tag, qr_code, owner,
    purchase_price, purchase_currency, purchase_date, expiry_date,
    supplier, purchase_url, replacement_cost,
    minimum_quantity, notes, created_by
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $12,
    $13, $14,
    $15, $16, $17, $18,
    $19, $20, $21, $22,
    $23, $24, $25,
    $26, $27, $28
)
RETURNING *;

-- name: UpdateItem :one
UPDATE items
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    category_id = sqlc.narg('category_id'),
    item_type = COALESCE(sqlc.narg('item_type'), item_type),
    size = COALESCE(sqlc.narg('size'), size),
    unit = COALESCE(sqlc.narg('unit'), unit),
    condition = COALESCE(sqlc.narg('condition'), condition),
    location_id = sqlc.narg('location_id'),
    parent_container_id = sqlc.narg('parent_container_id'),
    parent_group_id = sqlc.narg('parent_group_id'),
    managed_by_department_id = sqlc.narg('managed_dept_id'),
    needed_by_department_id = sqlc.narg('needed_dept_id'),
    serial_number = sqlc.narg('serial_number'),
    asset_tag = sqlc.narg('asset_tag'),
    qr_code = sqlc.narg('qr_code'),
    owner = COALESCE(sqlc.narg('owner'), owner),
    purchase_price = sqlc.narg('purchase_price'),
    purchase_currency = COALESCE(sqlc.narg('purchase_currency'), purchase_currency),
    purchase_date = sqlc.narg('purchase_date'),
    expiry_date = sqlc.narg('expiry_date'),
    supplier = sqlc.narg('supplier'),
    purchase_url = sqlc.narg('purchase_url'),
    replacement_cost = sqlc.narg('replacement_cost'),
    minimum_quantity = COALESCE(sqlc.narg('minimum_quantity'), minimum_quantity),
    notes = COALESCE(sqlc.narg('notes'), notes),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SetItemAssetTag :one
UPDATE items
SET asset_tag = sqlc.narg('asset_tag'),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SetItemPrimaryImage :exec
UPDATE items SET primary_image_file_id = $2, updated_at = now() WHERE id = $1;

-- name: AdjustItemQuantity :one
UPDATE items
SET quantity_owned = quantity_owned + sqlc.arg('delta')::int,
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SetItemDamagedDelta :one
UPDATE items
SET quantity_damaged = quantity_damaged + sqlc.arg('delta')::int,
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SetItemLostDelta :one
UPDATE items
SET quantity_lost = quantity_lost + sqlc.arg('delta')::int,
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: MoveItem :one
UPDATE items
SET location_id = sqlc.narg('location_id'),
    parent_container_id = sqlc.narg('parent_container_id'),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SetItemStatus :one
UPDATE items
SET status = sqlc.arg('status'),
    retired_at = CASE WHEN sqlc.arg('status') = 'retired' THEN now() ELSE NULL END,
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ItemSignedOutAggregate :one
SELECT
    COALESCE(SUM(quantity_signed_out - quantity_returned - quantity_consumed - quantity_lost - quantity_damaged), 0)::int AS active_signed_out
FROM department_signout_items dsi
JOIN department_signouts ds ON ds.id = dsi.signout_id
WHERE dsi.item_id = $1
  AND ds.status IN ('signed_out','partially_returned','prepared');
