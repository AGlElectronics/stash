-- Aggregate counts for GET /api/dashboard so the SPA doesn't have to fetch
-- whole tables just to render stat cards.

-- name: CountOverdueSignouts :one
SELECT count(*) FROM department_signouts
WHERE status IN ('signed_out','partially_returned')
  AND expected_return_at IS NOT NULL
  AND expected_return_at < now();

-- name: CountActiveSignouts :one
SELECT count(*) FROM department_signouts
WHERE status IN ('signed_out','partially_returned');

-- name: CountPartiallyReturnedSignouts :one
SELECT count(*) FROM department_signouts
WHERE status = 'partially_returned';

-- name: ListRecentActiveSignouts :many
SELECT * FROM department_signouts
WHERE status IN ('signed_out','partially_returned','prepared')
ORDER BY created_at DESC, id DESC
LIMIT $1;

-- name: CountLowStockItems :one
-- "Available" mirrors the per-item availability math used at sign-out time:
-- owned minus damaged/lost minus actively signed-out units.
SELECT count(*) FROM items i
WHERE i.retired_at IS NULL
  AND i.minimum_quantity > 0
  AND (i.quantity_owned - i.quantity_damaged - i.quantity_lost - COALESCE((
        SELECT SUM(dsi.quantity_signed_out - dsi.quantity_returned - dsi.quantity_consumed - dsi.quantity_lost - dsi.quantity_damaged)
        FROM department_signout_items dsi
        JOIN department_signouts ds ON ds.id = dsi.signout_id
        WHERE dsi.item_id = i.id
          AND ds.status IN ('signed_out','partially_returned','prepared')
      ), 0)) < i.minimum_quantity;

-- name: CountDamagedItems :one
SELECT count(*) FROM items
WHERE retired_at IS NULL AND quantity_damaged > 0;
