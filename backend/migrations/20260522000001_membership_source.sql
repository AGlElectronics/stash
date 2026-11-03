-- +goose Up
-- Track where each user_department_roles row came from so the OIDC
-- middleware can refresh group-derived memberships on every login WITHOUT
-- destroying memberships an admin granted manually via Settings → Users.
--
-- Background: LoadOrProvisionOIDCUser used to call ClearUserDepartmentRoles
-- (DELETE every row for the user) and then re-insert from the IdP groups.
-- That meant any admin grant added through the UI survived only until the
-- target user's next request, at which point their token came in, the
-- middleware wiped their memberships, and they lost the role they had
-- "just been given". The bug presented as the role disappearing on the
-- admin's side seconds after granting it.
--
-- Now: source='oidc' is the only thing the middleware clears. source='manual'
-- (the default for everything inserted before this migration, and for every
-- write through the access API) is left alone.

ALTER TABLE user_department_roles
    ADD COLUMN source TEXT NOT NULL DEFAULT 'manual'
        CHECK (source IN ('manual', 'oidc'));

CREATE INDEX user_department_roles_user_source_idx
    ON user_department_roles (user_id, source);

-- +goose Down
DROP INDEX IF EXISTS user_department_roles_user_source_idx;
ALTER TABLE user_department_roles DROP COLUMN IF EXISTS source;
