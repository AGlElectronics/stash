-- Initial schema for Stash MVP.
--
-- Notable deviations from the design doc section 8:
--   - Quantities are INTEGER (not NUMERIC). Sub-unit tracking can be added
--     later if a real need appears.
--   - Containers stay in the items table via item_type='container'; the
--     "this item lives in that container" link is a parent_container_id
--     self-FK.
--   - Tags are normalized into tags + item_tags.
--   - inventory_movements is replaced by a broader item_events lifecycle log.
--   - Adds users + role-based access tables for OIDC + dev bypass auth.

-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- +goose StatementEnd

-- ----------------------------------------------------------------------------
-- USERS & RBAC
-- ----------------------------------------------------------------------------

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    oidc_subject    TEXT UNIQUE,
    email           TEXT,
    display_name    TEXT NOT NULL,
    is_disabled     BOOLEAN NOT NULL DEFAULT FALSE,
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX users_email_idx ON users (lower(email));

CREATE TABLE roles (
    name TEXT PRIMARY KEY
);

CREATE TABLE permissions (
    name        TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE role_permissions (
    role_name       TEXT NOT NULL REFERENCES roles(name)       ON DELETE CASCADE,
    permission_name TEXT NOT NULL REFERENCES permissions(name) ON DELETE CASCADE,
    PRIMARY KEY (role_name, permission_name)
);

-- ----------------------------------------------------------------------------
-- DEPARTMENTS / CATEGORIES / LOCATIONS
-- ----------------------------------------------------------------------------

CREATE TABLE departments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE item_categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id   UUID REFERENCES locations(id) ON DELETE RESTRICT,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (parent_id, name)
);

CREATE INDEX locations_parent_idx ON locations(parent_id);

-- ----------------------------------------------------------------------------
-- USER MEMBERSHIPS (RBAC)
-- ----------------------------------------------------------------------------

CREATE TABLE oidc_group_role_bindings (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    oidc_group    TEXT NOT NULL,
    role_name     TEXT NOT NULL REFERENCES roles(name) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX oidc_group_role_bindings_unique_global
    ON oidc_group_role_bindings (oidc_group, role_name)
    WHERE department_id IS NULL;

CREATE UNIQUE INDEX oidc_group_role_bindings_unique_dept
    ON oidc_group_role_bindings (oidc_group, role_name, department_id)
    WHERE department_id IS NOT NULL;

CREATE TABLE user_department_roles (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE CASCADE,
    role_name     TEXT NOT NULL REFERENCES roles(name) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX user_department_roles_user_idx ON user_department_roles(user_id);

CREATE UNIQUE INDEX user_department_roles_unique_global
    ON user_department_roles (user_id, role_name)
    WHERE department_id IS NULL;

CREATE UNIQUE INDEX user_department_roles_unique_dept
    ON user_department_roles (user_id, department_id, role_name)
    WHERE department_id IS NOT NULL;

-- ----------------------------------------------------------------------------
-- FILES (metadata only; bytes live in S3-compatible storage)
-- ----------------------------------------------------------------------------

CREATE TABLE files (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type       TEXT NOT NULL,
    entity_id         UUID NOT NULL,
    file_type         TEXT NOT NULL CHECK (file_type IN ('image','receipt','invoice','manual','warranty','attachment')),
    bucket            TEXT NOT NULL,
    storage_key       TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    content_type      TEXT NOT NULL,
    size_bytes        BIGINT NOT NULL CHECK (size_bytes >= 0),
    uploaded_by       UUID REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (bucket, storage_key)
);

CREATE INDEX files_entity_idx ON files(entity_type, entity_id);

-- ----------------------------------------------------------------------------
-- ITEMS
-- ----------------------------------------------------------------------------

CREATE TABLE items (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                     TEXT NOT NULL,
    description              TEXT NOT NULL DEFAULT '',
    category_id              UUID REFERENCES item_categories(id) ON DELETE SET NULL,
    item_type                TEXT NOT NULL CHECK (item_type IN ('asset','consumable','container')),
    quantity_owned           INTEGER NOT NULL DEFAULT 0 CHECK (quantity_owned >= 0),
    quantity_damaged         INTEGER NOT NULL DEFAULT 0 CHECK (quantity_damaged >= 0),
    quantity_lost            INTEGER NOT NULL DEFAULT 0 CHECK (quantity_lost >= 0),
    unit                     TEXT NOT NULL DEFAULT 'pcs',
    condition                TEXT NOT NULL DEFAULT 'good',
    status                   TEXT NOT NULL DEFAULT 'available' CHECK (status IN ('available','in_use','retired')),
    location_id              UUID REFERENCES locations(id) ON DELETE SET NULL,
    parent_container_id      UUID REFERENCES items(id) ON DELETE SET NULL,
    managed_by_department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    needed_by_department_id  UUID REFERENCES departments(id) ON DELETE SET NULL,
    serial_number            TEXT,
    asset_tag                TEXT,
    qr_code                  TEXT,
    owner                    TEXT NOT NULL DEFAULT 'organization',
    purchase_price           NUMERIC(12,2),
    purchase_currency        TEXT NOT NULL DEFAULT 'EUR',
    purchase_date            DATE,
    supplier                 TEXT,
    purchase_url             TEXT,
    replacement_cost         NUMERIC(12,2),
    minimum_quantity         INTEGER NOT NULL DEFAULT 0 CHECK (minimum_quantity >= 0),
    primary_image_file_id    UUID REFERENCES files(id) ON DELETE SET NULL,
    notes                    TEXT NOT NULL DEFAULT '',
    created_by               UUID REFERENCES users(id),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    retired_at               TIMESTAMPTZ
);

CREATE INDEX items_name_lower_idx       ON items (lower(name));
CREATE INDEX items_category_idx         ON items (category_id);
CREATE INDEX items_managed_dept_idx     ON items (managed_by_department_id);
CREATE INDEX items_needed_dept_idx      ON items (needed_by_department_id);
CREATE INDEX items_location_idx         ON items (location_id);
CREATE INDEX items_status_idx           ON items (status);
CREATE INDEX items_container_idx        ON items (parent_container_id);
CREATE UNIQUE INDEX items_serial_unique ON items (serial_number) WHERE serial_number IS NOT NULL AND serial_number <> '';
CREATE UNIQUE INDEX items_asset_tag_unique ON items (asset_tag) WHERE asset_tag IS NOT NULL AND asset_tag <> '';
CREATE UNIQUE INDEX items_qr_code_unique  ON items (qr_code)    WHERE qr_code    IS NOT NULL AND qr_code    <> '';

-- ----------------------------------------------------------------------------
-- TAGS
-- ----------------------------------------------------------------------------

CREATE TABLE tags (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE item_tags (
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    tag_id  UUID NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
    PRIMARY KEY (item_id, tag_id)
);

CREATE INDEX item_tags_tag_idx ON item_tags(tag_id);

-- ----------------------------------------------------------------------------
-- DEPARTMENT SIGN-OUTS
-- ----------------------------------------------------------------------------

CREATE TABLE department_signouts (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    signout_number        TEXT NOT NULL UNIQUE,
    from_department_id    UUID REFERENCES departments(id) ON DELETE SET NULL,
    to_department_id      UUID NOT NULL REFERENCES departments(id) ON DELETE RESTRICT,
    from_location_id      UUID REFERENCES locations(id) ON DELETE SET NULL,
    status                TEXT NOT NULL DEFAULT 'draft' CHECK (status IN
        ('draft','prepared','signed_out','partially_returned','returned','closed','cancelled')),
    purpose               TEXT NOT NULL DEFAULT '',
    signed_out_by_user_id UUID REFERENCES users(id),
    received_by_user_id   UUID REFERENCES users(id),
    received_by_name      TEXT,
    expected_return_at    TIMESTAMPTZ,
    signed_out_at         TIMESTAMPTZ,
    returned_at           TIMESTAMPTZ,
    closed_at             TIMESTAMPTZ,
    cancelled_at          TIMESTAMPTZ,
    notes                 TEXT NOT NULL DEFAULT '',
    created_by            UUID REFERENCES users(id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX dept_signouts_status_idx  ON department_signouts(status);
CREATE INDEX dept_signouts_to_dept_idx ON department_signouts(to_department_id);
CREATE INDEX dept_signouts_active_idx  ON department_signouts(status)
    WHERE status IN ('signed_out','partially_returned','prepared');

CREATE TABLE department_signout_items (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    signout_id           UUID NOT NULL REFERENCES department_signouts(id) ON DELETE CASCADE,
    item_id              UUID NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    quantity_signed_out  INTEGER NOT NULL CHECK (quantity_signed_out >= 0),
    quantity_returned    INTEGER NOT NULL DEFAULT 0 CHECK (quantity_returned  >= 0),
    quantity_consumed    INTEGER NOT NULL DEFAULT 0 CHECK (quantity_consumed  >= 0),
    quantity_lost        INTEGER NOT NULL DEFAULT 0 CHECK (quantity_lost      >= 0),
    quantity_damaged     INTEGER NOT NULL DEFAULT 0 CHECK (quantity_damaged   >= 0),
    condition_on_signout TEXT,
    condition_on_return  TEXT,
    notes                TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (quantity_returned + quantity_consumed + quantity_lost + quantity_damaged <= quantity_signed_out)
);

CREATE INDEX signout_items_signout_idx ON department_signout_items(signout_id);
CREATE INDEX signout_items_item_idx    ON department_signout_items(item_id);

-- ----------------------------------------------------------------------------
-- ITEM EVENTS (per-item lifecycle log; replaces inventory_movements)
-- ----------------------------------------------------------------------------

CREATE TABLE item_events (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id             UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    event_type          TEXT NOT NULL CHECK (event_type IN (
        'created','moved','quantity_adjusted','signed_out','returned','consumed',
        'marked_damaged','repaired','marked_lost','marked_found',
        'condition_changed','retired','restored'
    )),
    quantity            INTEGER NOT NULL DEFAULT 0,
    from_location_id    UUID REFERENCES locations(id) ON DELETE SET NULL,
    to_location_id      UUID REFERENCES locations(id) ON DELETE SET NULL,
    from_department_id  UUID REFERENCES departments(id) ON DELETE SET NULL,
    to_department_id    UUID REFERENCES departments(id) ON DELETE SET NULL,
    related_signout_id  UUID REFERENCES department_signouts(id) ON DELETE SET NULL,
    related_event_id    UUID REFERENCES item_events(id) ON DELETE SET NULL,
    related_file_id     UUID REFERENCES files(id) ON DELETE SET NULL,
    condition_before    TEXT,
    condition_after     TEXT,
    actor_user_id       UUID REFERENCES users(id),
    note                TEXT NOT NULL DEFAULT '',
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX item_events_item_idx     ON item_events(item_id, occurred_at DESC);
CREATE INDEX item_events_type_idx     ON item_events(event_type);
CREATE INDEX item_events_signout_idx  ON item_events(related_signout_id);
CREATE INDEX item_events_actor_idx    ON item_events(actor_user_id);

-- ----------------------------------------------------------------------------
-- AUDIT LOG (admin/forensics; raw before/after JSON for every state change)
-- ----------------------------------------------------------------------------

CREATE TABLE audit_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users(id),
    action        TEXT NOT NULL,
    entity_type   TEXT NOT NULL,
    entity_id     UUID NOT NULL,
    before_json   JSONB,
    after_json    JSONB,
    note          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_entity_idx  ON audit_log(entity_type, entity_id);
CREATE INDEX audit_log_actor_idx   ON audit_log(actor_user_id);
CREATE INDEX audit_log_created_idx ON audit_log(created_at DESC);

-- ----------------------------------------------------------------------------
-- SEED: roles, permissions and role->permission mappings
-- ----------------------------------------------------------------------------

INSERT INTO roles (name) VALUES
    ('admin'),
    ('inventory_manager'),
    ('dept_lead'),
    ('volunteer'),
    ('readonly');

INSERT INTO permissions (name, description) VALUES
    ('item.view',          'View items'),
    ('item.create',        'Create items'),
    ('item.edit',          'Edit items'),
    ('item.retire',        'Retire / restore items'),
    ('item.move',          'Move items between locations'),
    ('item.adjust',        'Adjust item quantities'),
    ('item.damage',        'Mark items damaged / repaired'),
    ('item.lose',          'Mark items lost / found'),
    ('signout.view',       'View sign-outs'),
    ('signout.create',     'Create sign-outs'),
    ('signout.confirm',    'Confirm sign-outs'),
    ('signout.return',     'Return signed-out items'),
    ('signout.cancel',     'Cancel sign-outs'),
    ('signout.close',      'Close sign-outs'),
    ('department.manage',  'Manage departments'),
    ('location.manage',    'Manage locations'),
    ('category.manage',    'Manage categories'),
    ('user.manage',        'Manage users'),
    ('access.manage',      'Manage RBAC bindings'),
    ('audit.view',         'View audit log'),
    ('export.create',      'Export data as CSV'),
    ('file.upload',        'Upload files / images');

-- admin: every permission
INSERT INTO role_permissions (role_name, permission_name)
SELECT 'admin', name FROM permissions;

-- inventory_manager: everything except access.manage and user.manage
INSERT INTO role_permissions (role_name, permission_name)
SELECT 'inventory_manager', name FROM permissions
 WHERE name NOT IN ('access.manage','user.manage');

-- dept_lead: view + sign-out actions + file uploads
INSERT INTO role_permissions (role_name, permission_name) VALUES
    ('dept_lead', 'item.view'),
    ('dept_lead', 'signout.view'),
    ('dept_lead', 'signout.create'),
    ('dept_lead', 'signout.confirm'),
    ('dept_lead', 'signout.return'),
    ('dept_lead', 'signout.cancel'),
    ('dept_lead', 'signout.close'),
    ('dept_lead', 'item.damage'),
    ('dept_lead', 'item.lose'),
    ('dept_lead', 'file.upload'),
    ('dept_lead', 'audit.view'),
    ('dept_lead', 'export.create');

-- volunteer: view + create-and-return their own sign-outs
INSERT INTO role_permissions (role_name, permission_name) VALUES
    ('volunteer', 'item.view'),
    ('volunteer', 'signout.view'),
    ('volunteer', 'signout.create'),
    ('volunteer', 'signout.return');

-- readonly: every .view permission
INSERT INTO role_permissions (role_name, permission_name)
SELECT 'readonly', name FROM permissions
 WHERE name LIKE '%.view';

-- ----------------------------------------------------------------------------
-- SEED: default OIDC group -> role bindings (admins can edit later)
-- ----------------------------------------------------------------------------

INSERT INTO oidc_group_role_bindings (oidc_group, role_name, department_id) VALUES
    ('stash-admins',             'admin',             NULL),
    ('stash-inventory-managers', 'inventory_manager', NULL),
    ('stash-readonly',           'readonly',          NULL);

-- ----------------------------------------------------------------------------
-- SEED: dev user (used in AUTH_MODE=dev)
-- ----------------------------------------------------------------------------

INSERT INTO users (id, oidc_subject, email, display_name)
VALUES ('00000000-0000-0000-0000-000000000001', 'dev:dev-admin', 'dev-admin@stash.local', 'Dev Admin');

INSERT INTO user_department_roles (user_id, department_id, role_name)
VALUES ('00000000-0000-0000-0000-000000000001', NULL, 'admin');


-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS item_events;
DROP TABLE IF EXISTS department_signout_items;
DROP TABLE IF EXISTS department_signouts;
DROP TABLE IF EXISTS item_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS user_department_roles;
DROP TABLE IF EXISTS oidc_group_role_bindings;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS item_categories;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;
