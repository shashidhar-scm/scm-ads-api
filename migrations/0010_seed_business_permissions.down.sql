-- migrations/0010_seed_business_permissions.down.sql
-- +goose Down

-- Remove grants for business permissions
DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN (
        'campaigns.read','campaigns.write',
        'advertisers.read','advertisers.write',
        'creatives.read','creatives.write',
        'devices.read','devices.write',
        'venues.read','venues.write',
        'projects.read','projects.write'
    )
);

-- Remove permission rows
DELETE FROM permissions
WHERE name IN (
    'campaigns.read','campaigns.write',
    'advertisers.read','advertisers.write',
    'creatives.read','creatives.write',
    'devices.read','devices.write',
    'venues.read','venues.write',
    'projects.read','projects.write'
);
