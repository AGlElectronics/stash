-- name: GetSignout :one
SELECT * FROM department_signouts WHERE id = $1;

-- name: ListSignoutItems :many
SELECT * FROM department_signout_items WHERE signout_id = $1 ORDER BY created_at;

-- name: ListSignouts :many
SELECT * FROM department_signouts
WHERE (sqlc.narg('status')::text IS NULL OR sqlc.narg('status') = '' OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('to_dept_id')::uuid IS NULL OR to_department_id = sqlc.narg('to_dept_id')::uuid)
  AND (sqlc.narg('from_dept_id')::uuid IS NULL OR from_department_id = sqlc.narg('from_dept_id')::uuid)
  AND (sqlc.narg('signed_out_by')::uuid IS NULL OR signed_out_by_user_id = sqlc.narg('signed_out_by')::uuid)
  AND (sqlc.narg('search')::text IS NULL OR sqlc.narg('search') = '' OR
       lower(signout_number) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(coalesce(received_by_name, '')) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(coalesce(purpose, '')) LIKE '%' || lower(sqlc.narg('search')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountSignouts :one
SELECT count(*) FROM department_signouts
WHERE (sqlc.narg('status')::text IS NULL OR sqlc.narg('status') = '' OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('to_dept_id')::uuid IS NULL OR to_department_id = sqlc.narg('to_dept_id')::uuid)
  AND (sqlc.narg('from_dept_id')::uuid IS NULL OR from_department_id = sqlc.narg('from_dept_id')::uuid)
  AND (sqlc.narg('signed_out_by')::uuid IS NULL OR signed_out_by_user_id = sqlc.narg('signed_out_by')::uuid)
  AND (sqlc.narg('search')::text IS NULL OR sqlc.narg('search') = '' OR
       lower(signout_number) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(coalesce(received_by_name, '')) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(coalesce(purpose, '')) LIKE '%' || lower(sqlc.narg('search')::text) || '%');

-- name: NextSignoutNumber :one
-- Returns the next sequence number per year (SO-YYYY-NNNNN). The transaction
-- holding the row insert protects against duplicates via the unique index on
-- signout_number.
SELECT
    ('SO-' || to_char(now(), 'YYYY') || '-' || lpad(((COALESCE(
        (SELECT max(substring(signout_number from 9)::int)
            FROM department_signouts
            WHERE signout_number LIKE ('SO-' || to_char(now(), 'YYYY') || '-%')),
        0)) + 1)::text, 5, '0'))::text AS signout_number;

-- name: CreateSignout :one
INSERT INTO department_signouts (
    signout_number, from_department_id, to_department_id, from_location_id,
    status, purpose, expected_return_at,
    received_by_user_id, received_by_name,
    notes, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: AddSignoutItem :one
INSERT INTO department_signout_items (
    signout_id, item_id, quantity_signed_out, condition_on_signout, notes
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSignoutItem :one
SELECT * FROM department_signout_items WHERE id = $1;

-- name: UpdateSignoutItemReturn :one
UPDATE department_signout_items
SET quantity_returned = quantity_returned + sqlc.arg('returned_delta')::int,
    quantity_consumed = quantity_consumed + sqlc.arg('consumed_delta')::int,
    quantity_lost     = quantity_lost     + sqlc.arg('lost_delta')::int,
    quantity_damaged  = quantity_damaged  + sqlc.arg('damaged_delta')::int,
    condition_on_return = COALESCE(sqlc.narg('condition_on_return'), condition_on_return),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SetSignoutStatus :one
UPDATE department_signouts
SET status = sqlc.arg('status'),
    signed_out_at = CASE WHEN sqlc.arg('status') = 'signed_out' AND signed_out_at IS NULL THEN now() ELSE signed_out_at END,
    returned_at   = CASE WHEN sqlc.arg('status') = 'returned'   AND returned_at IS NULL   THEN now() ELSE returned_at   END,
    closed_at     = CASE WHEN sqlc.arg('status') = 'closed'     AND closed_at IS NULL     THEN now() ELSE closed_at     END,
    cancelled_at  = CASE WHEN sqlc.arg('status') = 'cancelled'  AND cancelled_at IS NULL  THEN now() ELSE cancelled_at  END,
    signed_out_by_user_id = COALESCE(sqlc.narg('signed_out_by_user_id'), signed_out_by_user_id),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SignoutItemsAggregate :one
SELECT
    COALESCE(SUM(quantity_signed_out), 0)::int AS signed_out,
    COALESCE(SUM(quantity_returned + quantity_consumed + quantity_lost + quantity_damaged), 0)::int AS settled
FROM department_signout_items
WHERE signout_id = $1;
