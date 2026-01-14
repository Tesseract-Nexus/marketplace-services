-- Rollback: Remove Gift Cards, Tax, Locations, and Additional Marketing Permissions

-- Remove role-permission mappings
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'giftcards:%';
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'tax:%';
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'locations:%';
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'marketing:loyalty:%';
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'marketing:carts:%';
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'marketing:segments:%';

-- Remove permissions
DELETE FROM staff_permissions WHERE name LIKE 'giftcards:%';
DELETE FROM staff_permissions WHERE name LIKE 'tax:%';
DELETE FROM staff_permissions WHERE name LIKE 'locations:%';
DELETE FROM staff_permissions WHERE name LIKE 'marketing:loyalty:%';
DELETE FROM staff_permissions WHERE name LIKE 'marketing:carts:%';
DELETE FROM staff_permissions WHERE name LIKE 'marketing:segments:%';

-- Remove permission categories
DELETE FROM permission_categories WHERE name IN ('giftcards', 'tax', 'locations');
