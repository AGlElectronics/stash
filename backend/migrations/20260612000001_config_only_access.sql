-- Access control moves entirely into the access config file mounted into the
-- backend pod (OIDC_BINDINGS_PATH). Roles, permissions and OIDC group → role
-- bindings are no longer stored in the database; /api/access/* and the
-- per-user role override endpoints are removed from the backend in the same
-- release. The users table stays: it persists identity (stable id for FKs,
-- display name, last login), not authorization.
--
-- Before deploying, confirm every production mapping that previously lived in
-- these tables is present in the access config file (the in-cluster
-- deployment has been manifest-driven via stash-oidc-bindings.yaml already,
-- so for the cluster this only drops manual per-user overrides).

-- +goose Up
DROP TABLE IF EXISTS user_department_roles;
DROP TABLE IF EXISTS oidc_group_role_bindings;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;

-- +goose Down
-- Restores the table structure only. Role/permission seed data and any
-- bindings/overrides that existed before the Up migration are NOT restored;
-- access control is config-file driven from this point on.
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
    source        TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'oidc')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX user_department_roles_user_idx ON user_department_roles(user_id);

CREATE UNIQUE INDEX user_department_roles_unique_global
    ON user_department_roles (user_id, role_name)
    WHERE department_id IS NULL;

CREATE UNIQUE INDEX user_department_roles_unique_dept
    ON user_department_roles (user_id, department_id, role_name)
    WHERE department_id IS NOT NULL;
