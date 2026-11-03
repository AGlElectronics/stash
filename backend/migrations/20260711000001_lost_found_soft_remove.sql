-- Soft-remove Lost & Found reports: hide from the UI but keep the row (like
-- retiring inventory items). removed_at IS NULL means still visible.

-- +goose Up
ALTER TABLE lost_found_reports
    ADD COLUMN removed_at TIMESTAMPTZ;

CREATE INDEX lost_found_reports_active_idx
    ON lost_found_reports (date_reported DESC)
    WHERE removed_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS lost_found_reports_active_idx;
ALTER TABLE lost_found_reports DROP COLUMN IF EXISTS removed_at;
