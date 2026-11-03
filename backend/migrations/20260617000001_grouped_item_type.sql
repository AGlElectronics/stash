-- Grouped items: a parent item (e.g. "Security T-Shirt") holds size variants as
-- child rows linked via parent_group_id (see 20260617000002). Each child has a
-- size label and its own quantity_owned; parent_container_id is for physical storage.

-- +goose Up
ALTER TABLE items DROP CONSTRAINT items_item_type_check;
ALTER TABLE items ADD CONSTRAINT items_item_type_check
    CHECK (item_type IN ('asset','consumable','container','grouped'));

ALTER TABLE items ADD COLUMN size TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE items DROP COLUMN IF EXISTS size;

ALTER TABLE items DROP CONSTRAINT items_item_type_check;
ALTER TABLE items ADD CONSTRAINT items_item_type_check
    CHECK (item_type IN ('asset','consumable','container'));
