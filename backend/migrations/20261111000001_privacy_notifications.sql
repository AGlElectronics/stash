-- Privacy and notifications before 1.0.0:
--   * a guest can link a Lost & Found report to their Telegram chat;
--   * the bot's long-polling offset survives restarts and leader changes;
--   * users who left can be anonymised;
--   * overdue sign-outs remember when the last reminder went out.

-- +goose Up
ALTER TABLE lost_found_reports
    ADD COLUMN telegram_chat_id   BIGINT,
    ADD COLUMN telegram_linked_at TIMESTAMPTZ;

CREATE INDEX lost_found_reports_telegram_idx
    ON lost_found_reports (telegram_chat_id) WHERE telegram_chat_id IS NOT NULL;

CREATE TABLE telegram_state (
    id            SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    update_offset BIGINT NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users ADD COLUMN anonymised_at TIMESTAMPTZ;

ALTER TABLE department_signouts ADD COLUMN overdue_notified_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE department_signouts DROP COLUMN IF EXISTS overdue_notified_at;
ALTER TABLE users DROP COLUMN IF EXISTS anonymised_at;
DROP TABLE IF EXISTS telegram_state;
DROP INDEX IF EXISTS lost_found_reports_telegram_idx;
ALTER TABLE lost_found_reports
    DROP COLUMN IF EXISTS telegram_linked_at,
    DROP COLUMN IF EXISTS telegram_chat_id;
