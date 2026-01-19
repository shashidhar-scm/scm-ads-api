-- migrations/0009_rbac_schema.down.sql
-- +goose Down

DROP TRIGGER IF EXISTS trg_prevent_system_role_delete ON roles;
DROP TRIGGER IF EXISTS trg_prevent_system_role_update ON roles;

DROP FUNCTION IF EXISTS prevent_system_role_modification();

DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
