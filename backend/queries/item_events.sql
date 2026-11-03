-- name: InsertItemEvent :one
INSERT INTO item_events (
    item_id, event_type, quantity,
    from_location_id, to_location_id,
    from_department_id, to_department_id,
    related_signout_id, related_event_id, related_file_id,
    condition_before, condition_after,
    actor_user_id, actor_oidc_subject, actor_display_name, note
) VALUES (
    $1, $2, $3,
    $4, $5,
    $6, $7,
    $8, $9, $10,
    $11, $12,
    $13, $14, $15, $16
)
RETURNING *;

-- name: ListItemEvents :many
SELECT * FROM item_events
WHERE item_id = sqlc.arg('item_id')
  AND (sqlc.narg('event_type')::text IS NULL OR sqlc.narg('event_type') = '' OR event_type = sqlc.narg('event_type')::text)
  AND (sqlc.narg('signout_id')::uuid IS NULL OR related_signout_id = sqlc.narg('signout_id')::uuid)
  AND (sqlc.narg('since')::timestamptz IS NULL OR occurred_at >= sqlc.narg('since')::timestamptz)
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountItemEvents :one
SELECT count(*) FROM item_events
WHERE item_id = sqlc.arg('item_id')
  AND (sqlc.narg('event_type')::text IS NULL OR sqlc.narg('event_type') = '' OR event_type = sqlc.narg('event_type')::text)
  AND (sqlc.narg('signout_id')::uuid IS NULL OR related_signout_id = sqlc.narg('signout_id')::uuid)
  AND (sqlc.narg('since')::timestamptz IS NULL OR occurred_at >= sqlc.narg('since')::timestamptz);
