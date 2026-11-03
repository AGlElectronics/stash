-- Separate logical group membership from physical container placement.
-- Size variants link to a grouped item via parent_group_id; they can also sit in
-- one or more physical containers via parent_container_id (one row per placement).

-- +goose Up
ALTER TABLE items ADD COLUMN parent_group_id UUID REFERENCES items(id) ON DELETE SET NULL;

CREATE INDEX items_group_idx ON items (parent_group_id);

-- Existing grouped children used parent_container_id for the group link.
UPDATE items child
SET parent_group_id = child.parent_container_id,
    parent_container_id = NULL
FROM items parent
WHERE child.parent_container_id = parent.id
  AND parent.item_type = 'grouped';

-- +goose Down
UPDATE items child
SET parent_container_id = child.parent_group_id
FROM items parent
WHERE child.parent_group_id = parent.id
  AND parent.item_type = 'grouped';

DROP INDEX IF EXISTS items_group_idx;
ALTER TABLE items DROP COLUMN IF EXISTS parent_group_id;
