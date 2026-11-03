-- name: ListTags :many
SELECT * FROM tags ORDER BY name;

-- name: UpsertTag :one
INSERT INTO tags (name) VALUES ($1)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING *;

-- name: AttachItemTag :exec
INSERT INTO item_tags (item_id, tag_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DetachItemTag :exec
DELETE FROM item_tags WHERE item_id = $1 AND tag_id = $2;

-- name: ClearItemTags :exec
DELETE FROM item_tags WHERE item_id = $1;

-- name: ListItemTags :many
SELECT t.* FROM tags t
JOIN item_tags it ON it.tag_id = t.id
WHERE it.item_id = $1
ORDER BY t.name;
