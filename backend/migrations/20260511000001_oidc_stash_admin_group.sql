-- +goose Up
-- Map Keycloak group "stash-admin" to the global admin role (all permissions).
-- The admin role and role_permissions are defined in 20260101000001_init.sql.
-- IdP must send this exact string in the OIDC "groups" claim (Keycloak group path
-- is usually the bare name, e.g. /stash-admin → "stash-admin" depending on mapper).
INSERT INTO oidc_group_role_bindings (oidc_group, role_name, department_id)
SELECT 'stash-admin', 'admin', NULL
WHERE NOT EXISTS (
    SELECT 1 FROM oidc_group_role_bindings
    WHERE oidc_group = 'stash-admin'
      AND role_name = 'admin'
      AND department_id IS NULL
);

-- +goose Down
DELETE FROM oidc_group_role_bindings
WHERE oidc_group = 'stash-admin'
  AND role_name = 'admin'
  AND department_id IS NULL;
