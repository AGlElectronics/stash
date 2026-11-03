-- name: InsertAuditLog :one
INSERT INTO audit_log (
    actor_user_id, actor_oidc_subject, actor_display_name,
    action, entity_type, entity_id,
    before_json, after_json, note
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListAuditLog :many
SELECT * FROM audit_log
WHERE (sqlc.narg('actor')::uuid IS NULL OR actor_user_id = sqlc.narg('actor')::uuid)
  AND (sqlc.narg('action')::text IS NULL OR sqlc.narg('action') = '' OR action = sqlc.narg('action')::text)
  AND (sqlc.narg('entity_type')::text IS NULL OR sqlc.narg('entity_type') = '' OR entity_type = sqlc.narg('entity_type')::text)
  AND (sqlc.narg('entity_id')::uuid IS NULL OR entity_id = sqlc.narg('entity_id')::uuid)
  AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since')::timestamptz)
  AND (sqlc.narg('until')::timestamptz IS NULL OR created_at <= sqlc.narg('until')::timestamptz)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountAuditLog :one
SELECT count(*) FROM audit_log
WHERE (sqlc.narg('actor')::uuid IS NULL OR actor_user_id = sqlc.narg('actor')::uuid)
  AND (sqlc.narg('action')::text IS NULL OR sqlc.narg('action') = '' OR action = sqlc.narg('action')::text)
  AND (sqlc.narg('entity_type')::text IS NULL OR sqlc.narg('entity_type') = '' OR entity_type = sqlc.narg('entity_type')::text)
  AND (sqlc.narg('entity_id')::uuid IS NULL OR entity_id = sqlc.narg('entity_id')::uuid)
  AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since')::timestamptz)
  AND (sqlc.narg('until')::timestamptz IS NULL OR created_at <= sqlc.narg('until')::timestamptz);
