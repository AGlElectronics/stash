-- name: ListLocations :many
SELECT * FROM locations ORDER BY name;

-- name: GetLocation :one
SELECT * FROM locations WHERE id = $1;

-- name: GetLocationByName :one
SELECT * FROM locations WHERE lower(name) = lower($1);

-- name: ListLocationChildren :many
SELECT * FROM locations WHERE parent_id = $1 ORDER BY name;

-- name: CreateLocation :one
INSERT INTO locations (parent_id, name, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateLocation :one
UPDATE locations
SET parent_id = sqlc.narg('parent_id'),
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteLocation :exec
DELETE FROM locations WHERE id = $1;
