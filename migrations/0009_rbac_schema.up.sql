-- migrations/0009_rbac_schema.up.sql
-- +goose Up

-- Roles
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Permissions
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Role <-> Permission mapping
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    PRIMARY KEY (role_id, permission_id)
);

-- User <-> Role mapping (optionally scoped to advertiser)
CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    advertiser_id UUID REFERENCES advertisers(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Ensure uniqueness for global assignments (advertiser_id IS NULL)
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_unique_global
    ON user_roles (user_id, role_id)
    WHERE advertiser_id IS NULL;

-- Ensure uniqueness for scoped assignments (advertiser_id IS NOT NULL)
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_unique_scoped
    ON user_roles (user_id, role_id, advertiser_id)
    WHERE advertiser_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles (user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles (role_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_advertiser_id ON user_roles (advertiser_id);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions (role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions (permission_id);

-- Prevent UPDATE/DELETE of system roles
CREATE OR REPLACE FUNCTION prevent_system_role_modification()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'UPDATE') THEN
        IF OLD.is_system THEN
            RAISE EXCEPTION 'system roles cannot be modified';
        END IF;
        RETURN NEW;
    ELSIF (TG_OP = 'DELETE') THEN
        IF OLD.is_system THEN
            RAISE EXCEPTION 'system roles cannot be deleted';
        END IF;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prevent_system_role_update ON roles;
CREATE TRIGGER trg_prevent_system_role_update
BEFORE UPDATE ON roles
FOR EACH ROW EXECUTE FUNCTION prevent_system_role_modification();

DROP TRIGGER IF EXISTS trg_prevent_system_role_delete ON roles;
CREATE TRIGGER trg_prevent_system_role_delete
BEFORE DELETE ON roles
FOR EACH ROW EXECUTE FUNCTION prevent_system_role_modification();

-- Seed fixed roles
INSERT INTO roles (name, description, is_system)
VALUES
    ('super_admin', 'Full system access', TRUE),
    ('admin', 'Global admin access', TRUE),
    ('advertiser', 'Advertiser-scoped access', TRUE)
ON CONFLICT (name) DO NOTHING;

-- Seed core permissions
INSERT INTO permissions (name, description)
VALUES
    ('roles.read', 'Read roles'),
    ('roles.write', 'Create/update/delete roles and manage role permissions'),
    ('permissions.read', 'Read permissions'),
    ('permissions.write', 'Create/update/delete permissions'),
    ('user_roles.read', 'Read user role assignments'),
    ('user_roles.write', 'Assign roles to users')
ON CONFLICT (name) DO NOTHING;

-- Grant core permissions to super_admin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name IN (
    'roles.read', 'roles.write',
    'permissions.read', 'permissions.write',
    'user_roles.read', 'user_roles.write'
)
WHERE r.name = 'super_admin'
ON CONFLICT DO NOTHING;
