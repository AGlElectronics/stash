-- name: ListDepartments :many
SELECT * FROM departments ORDER BY name;

-- name: GetDepartment :one
SELECT * FROM departments WHERE id = $1;

-- name: GetDepartmentByName :one
SELECT * FROM departments WHERE lower(name) = lower($1);

-- name: CreateDepartment :one
INSERT INTO departments (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateDepartment :one
UPDATE departments
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteDepartment :exec
DELETE FROM departments WHERE id = $1;
