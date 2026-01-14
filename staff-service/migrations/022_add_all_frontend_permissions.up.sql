-- Migration: 022_add_all_frontend_permissions
-- Description: Add all permissions required by the admin frontend
-- This ensures all permission names match what the frontend expects

-- ============================================================================
-- STEP 1: Ensure all permission categories exist
-- ============================================================================

INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111114', 'giftcards', 'Gift Cards', 'Gift card management', 'Gift', 14, true),
    ('11111111-1111-1111-1111-111111111115', 'tax', 'Tax', 'Tax configuration', 'Receipt', 15, true),
    ('11111111-1111-1111-1111-111111111116', 'locations', 'Locations', 'Store locations', 'MapPin', 16, true),
    ('11111111-1111-1111-1111-111111111120', 'delegations', 'Delegations', 'Delegation management', 'Users', 20, true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 2: Add Gift Card permissions (required by admin/app/(tenant)/gift-cards/page.tsx)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222220001', '11111111-1111-1111-1111-111111111114', 'giftcards:view', 'View Gift Cards', 'View gift card listings and details', 'giftcards', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222220002', '11111111-1111-1111-1111-111111111114', 'giftcards:create', 'Create Gift Cards', 'Create new gift cards', 'giftcards', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222220003', '11111111-1111-1111-1111-111111111114', 'giftcards:edit', 'Edit Gift Cards', 'Modify gift card details', 'giftcards', 'edit', false, false, 3),
    ('22222222-2222-2222-2222-222222220004', '11111111-1111-1111-1111-111111111114', 'giftcards:delete', 'Delete Gift Cards', 'Delete gift cards', 'giftcards', 'delete', true, false, 4),
    ('22222222-2222-2222-2222-222222220005', '11111111-1111-1111-1111-111111111114', 'giftcards:redeem', 'Redeem Gift Cards', 'Redeem gift cards for orders', 'giftcards', 'redeem', false, false, 5),
    ('22222222-2222-2222-2222-222222220006', '11111111-1111-1111-1111-111111111114', 'giftcards:balance:adjust', 'Adjust Balance', 'Manually adjust gift card balance', 'giftcards', 'adjust', true, true, 6),
    ('22222222-2222-2222-2222-222222220007', '11111111-1111-1111-1111-111111111114', 'giftcards:bulk:create', 'Bulk Create', 'Create gift cards in bulk', 'giftcards', 'bulk', false, false, 7),
    ('22222222-2222-2222-2222-222222220008', '11111111-1111-1111-1111-111111111114', 'giftcards:export', 'Export Gift Cards', 'Export gift card data', 'giftcards', 'export', false, false, 8),
    ('22222222-2222-2222-2222-222222220009', '11111111-1111-1111-1111-111111111114', 'giftcards:transactions:view', 'View Transactions', 'View gift card transaction history', 'transactions', 'view', false, false, 9)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 3: Add Location permissions (required by admin/app/(tenant)/locations/page.tsx)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222220010', '11111111-1111-1111-1111-111111111116', 'locations:view', 'View Locations', 'View store locations', 'locations', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222220011', '11111111-1111-1111-1111-111111111116', 'locations:create', 'Create Locations', 'Create new store locations', 'locations', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222220012', '11111111-1111-1111-1111-111111111116', 'locations:update', 'Edit Locations', 'Modify location details', 'locations', 'edit', true, false, 3),
    ('22222222-2222-2222-2222-222222220013', '11111111-1111-1111-1111-111111111116', 'locations:delete', 'Delete Locations', 'Remove store locations', 'locations', 'delete', true, false, 4)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 4: Add Marketing Loyalty/Segments permissions (required by loyalty/segments pages)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222220020', '11111111-1111-1111-1111-111111111104', 'marketing:loyalty:view', 'View Loyalty Program', 'View loyalty program settings', 'loyalty', 'view', false, false, 20),
    ('22222222-2222-2222-2222-222222220021', '11111111-1111-1111-1111-111111111104', 'marketing:loyalty:manage', 'Manage Loyalty Program', 'Configure loyalty tiers and rewards', 'loyalty', 'manage', true, false, 21),
    ('22222222-2222-2222-2222-222222220022', '11111111-1111-1111-1111-111111111104', 'marketing:loyalty:points:adjust', 'Adjust Points', 'Manually adjust customer loyalty points', 'points', 'adjust', true, false, 22),
    ('22222222-2222-2222-2222-222222220023', '11111111-1111-1111-1111-111111111104', 'marketing:segments:view', 'View Segments', 'View customer segments', 'segments', 'view', false, false, 23),
    ('22222222-2222-2222-2222-222222220024', '11111111-1111-1111-1111-111111111104', 'marketing:segments:manage', 'Manage Segments', 'Create and manage customer segments', 'segments', 'manage', false, false, 24),
    ('22222222-2222-2222-2222-222222220025', '11111111-1111-1111-1111-111111111104', 'marketing:carts:view', 'View Abandoned Carts', 'View abandoned cart data', 'carts', 'view', false, false, 25),
    ('22222222-2222-2222-2222-222222220026', '11111111-1111-1111-1111-111111111104', 'marketing:carts:recover', 'Recover Carts', 'Send recovery emails and offers', 'carts', 'recover', false, false, 26)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 5: Add Ticket permissions with :read naming (frontend expects tickets:read)
-- Uses existing tickets category (11111111-1111-1111-1111-111111111110)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222220030', '11111111-1111-1111-1111-111111111110', 'tickets:read', 'View Tickets', 'View support tickets', 'tickets', 'read', false, false, 1),
    ('22222222-2222-2222-2222-222222220031', '11111111-1111-1111-1111-111111111110', 'tickets:create', 'Create Tickets', 'Create new support tickets', 'tickets', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222220032', '11111111-1111-1111-1111-111111111110', 'tickets:update', 'Update Tickets', 'Modify ticket details', 'tickets', 'update', false, false, 3),
    ('22222222-2222-2222-2222-222222220033', '11111111-1111-1111-1111-111111111110', 'tickets:assign', 'Assign Tickets', 'Assign tickets to staff', 'tickets', 'assign', false, false, 4),
    ('22222222-2222-2222-2222-222222220034', '11111111-1111-1111-1111-111111111110', 'tickets:escalate', 'Escalate Tickets', 'Escalate tickets to higher level', 'tickets', 'escalate', false, false, 5),
    ('22222222-2222-2222-2222-222222220035', '11111111-1111-1111-1111-111111111110', 'tickets:resolve', 'Resolve Tickets', 'Mark tickets as resolved', 'tickets', 'resolve', false, false, 6)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 6: Add Approval permissions with :read naming (frontend expects approvals:read)
-- Uses existing approvals category (11111111-1111-1111-1111-111111111112)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222220040', '11111111-1111-1111-1111-111111111112', 'approvals:read', 'View Approvals', 'View pending approvals', 'approvals', 'read', false, false, 1),
    ('22222222-2222-2222-2222-222222220041', '11111111-1111-1111-1111-111111111112', 'approvals:create', 'Create Approvals', 'Create approval requests', 'approvals', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222220042', '11111111-1111-1111-1111-111111111112', 'approvals:approve', 'Approve Requests', 'Approve pending requests', 'approvals', 'approve', true, false, 3),
    ('22222222-2222-2222-2222-222222220043', '11111111-1111-1111-1111-111111111112', 'approvals:reject', 'Reject Requests', 'Reject pending requests', 'approvals', 'reject', true, false, 4),
    ('22222222-2222-2222-2222-222222220044', '11111111-1111-1111-1111-111111111112', 'approvals:manage', 'Manage Approvals', 'Configure approval workflows', 'approvals', 'manage', true, false, 5)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 7: Add Vendor permissions with :read naming (frontend expects vendors:read)
-- Uses existing vendors category (11111111-1111-1111-1111-111111111111)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222220050', '11111111-1111-1111-1111-111111111111', 'vendors:read', 'View Vendors', 'View vendor listings', 'vendors', 'read', false, false, 1),
    ('22222222-2222-2222-2222-222222220051', '11111111-1111-1111-1111-111111111111', 'vendors:create', 'Create Vendors', 'Onboard new vendors', 'vendors', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222220052', '11111111-1111-1111-1111-111111111111', 'vendors:update', 'Edit Vendors', 'Modify vendor details', 'vendors', 'update', true, false, 3),
    ('22222222-2222-2222-2222-222222220053', '11111111-1111-1111-1111-111111111111', 'vendors:approve', 'Approve Vendors', 'Approve vendor applications', 'vendors', 'approve', true, false, 4),
    ('22222222-2222-2222-2222-222222220054', '11111111-1111-1111-1111-111111111111', 'vendors:manage', 'Manage Vendors', 'Full vendor management access', 'vendors', 'manage', true, false, 5),
    ('22222222-2222-2222-2222-222222220055', '11111111-1111-1111-1111-111111111111', 'vendors:payout', 'Process Payouts', 'Process vendor payouts', 'vendors', 'payout', true, true, 6)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 8: Add Delegation permissions with :read naming (frontend expects delegations:read)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222220060', '11111111-1111-1111-1111-111111111120', 'delegations:read', 'View Delegations', 'View approval delegations', 'delegations', 'read', false, false, 1),
    ('22222222-2222-2222-2222-222222220061', '11111111-1111-1111-1111-111111111120', 'delegations:manage', 'Manage Delegations', 'Create and manage delegations', 'delegations', 'manage', true, false, 2)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 9: Grant ALL permissions to store_owner role
-- This ensures the tenant owner can access all features
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 10: Grant most permissions to store_admin (except sensitive finance)
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_admin', 'admin')
AND sp.name NOT IN ('settings:payments:manage', 'finance:payouts:manage', 'finance:reconciliation:manage', 'giftcards:balance:adjust')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- This migration adds 44 new permissions:
-- - 9 gift card permissions
-- - 4 location permissions
-- - 7 marketing loyalty/segments permissions
-- - 6 ticket permissions (with :read naming)
-- - 5 approval permissions (with :read naming)
-- - 6 vendor permissions (with :read naming)
-- - 2 delegation permissions (with :read naming)
--
-- All permissions are granted to store_owner and most to store_admin
