-- Optional expiry date for consumables and other dated stock.

-- +goose Up
ALTER TABLE items ADD COLUMN expiry_date DATE;

-- +goose Down
ALTER TABLE items DROP COLUMN IF EXISTS expiry_date;
