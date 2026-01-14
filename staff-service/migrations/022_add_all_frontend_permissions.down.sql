-- Rollback: 022_add_all_frontend_permissions
-- This removes the permissions added in the up migration

-- Remove role permission assignments for the new permissions
DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions WHERE name IN (
        'giftcards:view', 'giftcards:create', 'giftcards:edit', 'giftcards:delete',
        'giftcards:redeem', 'giftcards:balance:adjust', 'giftcards:bulk:create',
        'giftcards:export', 'giftcards:transactions:view',
        'locations:view', 'locations:create', 'locations:update', 'locations:delete',
        'marketing:loyalty:view', 'marketing:loyalty:manage', 'marketing:loyalty:points:adjust',
        'marketing:segments:view', 'marketing:segments:manage',
        'marketing:carts:view', 'marketing:carts:recover',
        'tickets:read', 'tickets:create', 'tickets:update', 'tickets:assign',
        'tickets:escalate', 'tickets:resolve',
        'approvals:read', 'approvals:create', 'approvals:approve', 'approvals:reject', 'approvals:manage',
        'vendors:read', 'vendors:create', 'vendors:update', 'vendors:approve', 'vendors:manage', 'vendors:payout',
        'delegations:read', 'delegations:manage'
    )
);

-- Remove the permissions
DELETE FROM staff_permissions WHERE name IN (
    'giftcards:view', 'giftcards:create', 'giftcards:edit', 'giftcards:delete',
    'giftcards:redeem', 'giftcards:balance:adjust', 'giftcards:bulk:create',
    'giftcards:export', 'giftcards:transactions:view',
    'locations:view', 'locations:create', 'locations:update', 'locations:delete',
    'marketing:loyalty:view', 'marketing:loyalty:manage', 'marketing:loyalty:points:adjust',
    'marketing:segments:view', 'marketing:segments:manage',
    'marketing:carts:view', 'marketing:carts:recover',
    'tickets:read', 'tickets:create', 'tickets:update', 'tickets:assign',
    'tickets:escalate', 'tickets:resolve',
    'approvals:read', 'approvals:create', 'approvals:approve', 'approvals:reject', 'approvals:manage',
    'vendors:read', 'vendors:create', 'vendors:update', 'vendors:approve', 'vendors:manage', 'vendors:payout',
    'delegations:read', 'delegations:manage'
);

-- Remove the new categories (only if no other permissions reference them)
DELETE FROM permission_categories WHERE name IN ('giftcards', 'tax', 'locations', 'delegations')
AND id NOT IN (SELECT DISTINCT category_id FROM staff_permissions WHERE category_id IS NOT NULL);
