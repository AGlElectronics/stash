-- Single-row system settings table. JSONB makes it cheap to add new flags
-- without future migrations; the application is the source of truth for
-- the schema of `data`. The CHECK ensures the table never grows past one
-- row, simplifying read/write code.

-- +goose Up
CREATE TABLE system_settings (
    id          INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    data        JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO system_settings (id, data)
VALUES (1, '{"force_barcode_on_signout": false}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS system_settings;
