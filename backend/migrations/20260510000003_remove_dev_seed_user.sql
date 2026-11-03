-- +goose Up
-- Remove the placeholder dev user shipped in the initial migration. Production
-- and OIDC-only clusters must not retain a known local account. In AUTH_MODE=dev,
-- the next request re-provisions a dev:* user via UpsertUser (new UUID).
DELETE FROM user_department_roles WHERE user_id = '00000000-0000-0000-0000-000000000001';
DELETE FROM users WHERE id = '00000000-0000-0000-0000-000000000001';

-- +goose Down
INSERT INTO users (id, oidc_subject, email, display_name)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'dev:dev-admin',
    'dev-admin@stash.local',
    'Dev Admin'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO user_department_roles (user_id, department_id, role_name)
SELECT '00000000-0000-0000-0000-000000000001', NULL, 'admin'
WHERE NOT EXISTS (
    SELECT 1 FROM user_department_roles
    WHERE user_id = '00000000-0000-0000-0000-000000000001'
      AND department_id IS NULL
      AND role_name = 'admin'
);
