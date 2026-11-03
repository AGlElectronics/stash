-- name: InsertFile :one
INSERT INTO files (
    entity_type, entity_id, file_type,
    bucket, storage_key, original_filename,
    content_type, size_bytes, uploaded_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetFile :one
SELECT * FROM files WHERE id = $1;

-- name: ListFilesForEntity :many
SELECT * FROM files
WHERE entity_type = $1 AND entity_id = $2
  AND ($3::text IS NULL OR $3 = '' OR file_type = $3)
ORDER BY created_at DESC;

-- name: DeleteFile :exec
DELETE FROM files WHERE id = $1;
