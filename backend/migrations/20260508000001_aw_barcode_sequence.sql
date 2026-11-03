-- Asset-tag sequence used by /api/items/{id}/assign-barcode and the
-- "Generate" button in the UI. The sequence starts at 1 and produces values
-- like ST000001, AW000002, ... when zero-padded to 6 digits in application
-- code. We keep formatting in Go (not in SQL) so the prefix can change later
-- without a migration.

-- +goose Up
CREATE SEQUENCE IF NOT EXISTS aw_barcode_seq START 1 INCREMENT 1 MINVALUE 1 NO MAXVALUE CACHE 1;

-- +goose Down
DROP SEQUENCE IF EXISTS aw_barcode_seq;
