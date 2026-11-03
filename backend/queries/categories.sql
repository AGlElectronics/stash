-- name: ListCategories :many
SELECT * FROM item_categories ORDER BY name;

-- name: GetCategory :one
SELECT * FROM item_categories WHERE id = $1;

-- name: GetCategoryByName :one
SELECT * FROM item_categories WHERE lower(name) = lower($1);

-- name: CreateCategory :one
INSERT INTO item_categories (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateCategory :one
UPDATE item_categories
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM item_categories WHERE id = $1;
