-- name: GetLostFoundReport :one
SELECT * FROM lost_found_reports WHERE id = $1 AND removed_at IS NULL;

-- name: GetLostFoundByReportNumber :one
SELECT * FROM lost_found_reports WHERE report_number = $1 AND removed_at IS NULL;

-- name: ListLostFoundReports :many
SELECT * FROM lost_found_reports
WHERE removed_at IS NULL
  AND (sqlc.narg('type')::text IS NULL OR sqlc.narg('type') = '' OR type = sqlc.narg('type')::text)
  AND (sqlc.narg('status')::text IS NULL OR sqlc.narg('status') = '' OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('search')::text IS NULL OR sqlc.narg('search') = '' OR
       lower(report_number) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(item_name)     LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(description)   LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(location)      LIKE '%' || lower(sqlc.narg('search')::text) || '%')
ORDER BY date_reported DESC, id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountLostFoundReports :one
SELECT count(*) FROM lost_found_reports
WHERE removed_at IS NULL
  AND (sqlc.narg('type')::text IS NULL OR sqlc.narg('type') = '' OR type = sqlc.narg('type')::text)
  AND (sqlc.narg('status')::text IS NULL OR sqlc.narg('status') = '' OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('search')::text IS NULL OR sqlc.narg('search') = '' OR
       lower(report_number) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(item_name)     LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(description)   LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(location)      LIKE '%' || lower(sqlc.narg('search')::text) || '%');

-- name: NextLostFoundNumber :one
-- Returns the next report number (LF-NNNN). The transaction holding the row
-- insert protects against duplicates via the unique index on report_number.
SELECT
    ('LF-' || lpad(((COALESCE(
        (SELECT max(substring(report_number from 4)::int)
            FROM lost_found_reports
            WHERE report_number LIKE 'LF-%'),
        0)) + 1)::text, 4, '0'))::text AS report_number;

-- name: CreateLostFoundReport :one
INSERT INTO lost_found_reports (
    report_number, type, item_name, description,
    photo_file_id, location, date_reported,
    reported_by_user_id, contact_name, contact_info
) VALUES (
    $1, $2, $3, $4,
    $5, $6, COALESCE(sqlc.narg('date_reported'), now()),
    $7, $8, $9
)
RETURNING *;

-- name: UpdateLostFoundReport :one
-- The service computes the final value of every column in Go (read-modify-
-- write inside a transaction) so PATCH can distinguish "not provided" from
-- "set to empty/null" without COALESCE tricks.
UPDATE lost_found_reports
SET type                = $2,
    status              = $3,
    item_name           = $4,
    description         = $5,
    photo_file_id       = $6,
    location            = $7,
    contact_name        = $8,
    contact_info        = $9,
    returned_to         = $10,
    resolved_by_user_id = $11,
    resolved_at         = $12,
    resolution_note     = $13,
    updated_at          = now()
WHERE id = $1 AND removed_at IS NULL
RETURNING *;

-- name: RemoveLostFoundReport :one
UPDATE lost_found_reports
SET removed_at = now(), updated_at = now()
WHERE id = $1 AND removed_at IS NULL
RETURNING *;

-- name: CountOpenLostFoundReports :one
SELECT count(*) FROM lost_found_reports WHERE status = 'open' AND removed_at IS NULL;

-- name: GetPublicLostFoundByNumber :one
-- Public endpoint: only returns found+open reports for guest claim lookup.
SELECT id, report_number, item_name, description, photo_file_id, location, date_reported
FROM lost_found_reports
WHERE report_number = $1 AND type = 'found' AND status = 'open' AND removed_at IS NULL;

-- name: SearchPublicLostFound :many
-- Public search: returns open "found" reports matching a text query.
SELECT id, report_number, item_name, description, photo_file_id, location, date_reported
FROM lost_found_reports
WHERE type = 'found' AND status = 'open' AND removed_at IS NULL
  AND (sqlc.narg('search')::text IS NULL OR sqlc.narg('search') = '' OR
       lower(item_name)   LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(description) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(location)    LIKE '%' || lower(sqlc.narg('search')::text) || '%')
ORDER BY date_reported DESC, id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountPublicLostFound :one
SELECT count(*) FROM lost_found_reports
WHERE type = 'found' AND status = 'open' AND removed_at IS NULL
  AND (sqlc.narg('search')::text IS NULL OR sqlc.narg('search') = '' OR
       lower(item_name)   LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(description) LIKE '%' || lower(sqlc.narg('search')::text) || '%' OR
       lower(location)    LIKE '%' || lower(sqlc.narg('search')::text) || '%');
