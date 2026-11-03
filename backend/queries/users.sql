-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByOIDCSubject :one
SELECT * FROM users WHERE oidc_subject = $1;

-- name: UpsertUserBySubject :one
INSERT INTO users (oidc_subject, email, display_name, last_login_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (oidc_subject) DO UPDATE SET
    email = EXCLUDED.email,
    display_name = EXCLUDED.display_name,
    last_login_at = now(),
    updated_at = now()
RETURNING *;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY display_name
LIMIT $1 OFFSET $2;

-- name: SetUserDisabled :exec
UPDATE users SET is_disabled = $2, updated_at = now() WHERE id = $1;

-- name: ResolveUsersByIDs :many
SELECT id, display_name, email FROM users WHERE id = ANY($1::uuid[]);
