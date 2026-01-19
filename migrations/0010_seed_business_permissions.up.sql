-- migrations/0010_seed_business_permissions.up.sql
-- +goose Up

-- Seed business permissions
INSERT INTO permissions (name, description)
VALUES
    ('campaigns.read', 'Read campaigns'),
    ('campaigns.write', 'Create/update/delete campaigns'),
    ('advertisers.read', 'Read advertisers'),
    ('advertisers.write', 'Create/update/delete advertisers'),
    ('creatives.read', 'Read creatives'),
    ('creatives.write', 'Create/update/delete creatives'),
    ('devices.read', 'Read devices'),
    ('devices.write', 'Create/update/delete devices'),
    ('venues.read', 'Read venues'),
    ('venues.write', 'Create/update/delete venues'),
    ('projects.read', 'Read projects'),
    ('projects.write', 'Create/update/delete projects')
ON CONFLICT (name) DO NOTHING;

-- Grant business permissions to super_admin and admin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name IN (
    'campaigns.read','campaigns.write',
    'advertisers.read','advertisers.write',
    'creatives.read','creatives.write',
    'devices.read','devices.write',
    'venues.read','venues.write',
    'projects.read','projects.write'
)
WHERE r.name IN ('super_admin','admin')
ON CONFLICT DO NOTHING;

-- Grant limited business permissions to advertiser (scope enforced via user_roles.advertiser_id)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name IN (
    'campaigns.read','campaigns.write',
    'creatives.read','creatives.write'
)
WHERE r.name = 'advertiser'
ON CONFLICT DO NOTHING;
