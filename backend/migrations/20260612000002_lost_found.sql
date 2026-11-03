-- Lost & Found reports: replaces the ConOps AppSheet/Google Sheets app.
-- Front desk staff log items guests lost or items that were found; reports
-- are resolved by handing the item back ("returned") or by end-of-con
-- accounting ("closed" = unclaimed / disposed / donated). location is free
-- text on purpose — "left near gym" is not a Stash locations record.

-- +goose Up
CREATE TABLE lost_found_reports (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_number       TEXT NOT NULL UNIQUE,
    type                TEXT NOT NULL CHECK (type IN ('lost','found')),
    status              TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','returned','closed')),
    item_name           TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    photo_file_id       UUID REFERENCES files(id) ON DELETE SET NULL,
    location            TEXT NOT NULL DEFAULT '',
    date_reported       TIMESTAMPTZ NOT NULL DEFAULT now(),
    reported_by_user_id UUID REFERENCES users(id),
    contact_name        TEXT NOT NULL DEFAULT '',
    contact_info        TEXT NOT NULL DEFAULT '',
    returned_to         TEXT NOT NULL DEFAULT '',
    resolved_by_user_id UUID REFERENCES users(id),
    resolved_at         TIMESTAMPTZ,
    resolution_note     TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX lost_found_reports_status_idx   ON lost_found_reports(status);
CREATE INDEX lost_found_reports_type_idx     ON lost_found_reports(type);
CREATE INDEX lost_found_reports_reported_idx ON lost_found_reports(date_reported DESC);

-- +goose Down
DROP TABLE IF EXISTS lost_found_reports;
