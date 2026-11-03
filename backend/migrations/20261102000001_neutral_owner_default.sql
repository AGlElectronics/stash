-- +goose Up
-- Neutral default owner for reusable Stash deployments (was event-specific).
ALTER TABLE items ALTER COLUMN owner SET DEFAULT 'organization';
UPDATE items SET owner = 'organization' WHERE owner = 'awoostria';

-- +goose Down
ALTER TABLE items ALTER COLUMN owner SET DEFAULT 'organization';
UPDATE items SET owner = 'awoostria' WHERE owner = 'organization';
