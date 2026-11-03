-- +goose Up
-- Snapshot who performed each action at write time. users.display_name and
-- email are refreshed on every login (nickname can change); audit rows keep
-- the label and Keycloak sub that were true when the event happened.
ALTER TABLE audit_log
    ADD COLUMN actor_oidc_subject TEXT,
    ADD COLUMN actor_display_name TEXT;

ALTER TABLE item_events
    ADD COLUMN actor_oidc_subject TEXT,
    ADD COLUMN actor_display_name TEXT;

CREATE INDEX audit_log_actor_oidc_idx ON audit_log(actor_oidc_subject)
    WHERE actor_oidc_subject IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS audit_log_actor_oidc_idx;

ALTER TABLE item_events
    DROP COLUMN IF EXISTS actor_display_name,
    DROP COLUMN IF EXISTS actor_oidc_subject;

ALTER TABLE audit_log
    DROP COLUMN IF EXISTS actor_display_name,
    DROP COLUMN IF EXISTS actor_oidc_subject;
