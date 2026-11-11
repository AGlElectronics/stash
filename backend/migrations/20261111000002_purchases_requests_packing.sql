-- Purchase records, item requests and event packing lists.
--   * item_purchases: what was bought for an item, when, for how much; a
--     purchase can add its quantity to stock.
--   * item_requests: someone asks logistics for gear (an existing item or
--     something to buy); logistics approves, orders, fulfils or rejects.
--   * packing_lists: what goes to an event or department, ticked off while
--     packing, and turned into a draft sign-out in one step.

-- +goose Up
CREATE TABLE item_purchases (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id      UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    purchased_on DATE NOT NULL DEFAULT current_date,
    quantity     INTEGER NOT NULL CHECK (quantity > 0),
    unit_price   NUMERIC(12,2) CHECK (unit_price >= 0),
    currency     TEXT NOT NULL DEFAULT 'EUR',
    supplier     TEXT NOT NULL DEFAULT '',
    reference    TEXT NOT NULL DEFAULT '',
    added_to_stock BOOLEAN NOT NULL DEFAULT FALSE,
    note         TEXT NOT NULL DEFAULT '',
    created_by   UUID REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX item_purchases_item_idx ON item_purchases (item_id, purchased_on DESC);

CREATE TABLE item_requests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title         TEXT NOT NULL,
    item_id       UUID REFERENCES items(id) ON DELETE SET NULL,
    quantity      INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    needed_by     DATE,
    reason        TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open','approved','ordered','fulfilled','rejected','cancelled')),
    requested_by  UUID REFERENCES users(id),
    decided_by    UUID REFERENCES users(id),
    decided_at    TIMESTAMPTZ,
    decision_note TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX item_requests_status_idx ON item_requests (status, created_at DESC);

CREATE TABLE packing_lists (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    event_date    DATE,
    notes         TEXT NOT NULL DEFAULT '',
    signout_id    UUID REFERENCES department_signouts(id) ON DELETE SET NULL,
    created_by    UUID REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE packing_list_items (
    list_id  UUID NOT NULL REFERENCES packing_lists(id) ON DELETE CASCADE,
    item_id  UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    packed   INTEGER NOT NULL DEFAULT 0 CHECK (packed >= 0),
    note     TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (list_id, item_id)
);

-- +goose Down
DROP TABLE IF EXISTS packing_list_items;
DROP TABLE IF EXISTS packing_lists;
DROP TABLE IF EXISTS item_requests;
DROP TABLE IF EXISTS item_purchases;
