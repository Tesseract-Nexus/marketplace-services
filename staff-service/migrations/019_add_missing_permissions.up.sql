-- Migration: 019_add_missing_permissions
-- Description: Add missing permissions for delegations, tickets, approvals, and vendors
-- These permissions are referenced in go-shared/rbac/permissions.go but were never added to the database

-- ============================================================================
-- STEP 1: Add permission categories for new permission types
-- ============================================================================

INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111117', 'tickets', 'Tickets', 'Customer support ticket management', 'Ticket', 17, true),
    ('11111111-1111-1111-1111-111111111118', 'approvals', 'Approvals', 'Approval workflow management', 'CheckCircle', 18, true),
    ('11111111-1111-1111-1111-111111111119', 'vendors', 'Vendors', 'Vendor/marketplace management', 'Store', 19, true),
    ('11111111-1111-1111-1111-111111111120', 'delegations', 'Delegations', 'Approval delegation management', 'Users', 20, true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 2: Add Ticket permissions (matching go-shared naming convention)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222c01', '11111111-1111-1111-1111-111111111117', 'tickets:read', 'View Tickets', 'View support tickets', 'tickets', 'read', false, false, 1),
    ('22222222-2222-2222-2222-222222222c02', '11111111-1111-1111-1111-111111111117', 'tickets:create', 'Create Tickets', 'Create new support tickets', 'tickets', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222222c03', '11111111-1111-1111-1111-111111111117', 'tickets:update', 'Update Tickets', 'Modify ticket details', 'tickets', 'update', false, false, 3),
    ('22222222-2222-2222-2222-222222222c04', '11111111-1111-1111-1111-111111111117', 'tickets:assign', 'Assign Tickets', 'Assign tickets to staff', 'tickets', 'assign', false, false, 4),
    ('22222222-2222-2222-2222-222222222c05', '11111111-1111-1111-1111-111111111117', 'tickets:escalate', 'Escalate Tickets', 'Escalate tickets to higher level', 'tickets', 'escalate', false, false, 5),
    ('22222222-2222-2222-2222-222222222c06', '11111111-1111-1111-1111-111111111117', 'tickets:resolve', 'Resolve Tickets', 'Mark tickets as resolved', 'tickets', 'resolve', false, false, 6)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- ============================================================================
-- STEP 3: Add Approval permissions (matching go-shared naming convention)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222d01', '11111111-1111-1111-1111-111111111118', 'approvals:read', 'View Approvals', 'View pending approvals', 'approvals', 'read', false, false, 1),
    ('22222222-2222-2222-2222-222222222d02', '11111111-1111-1111-1111-111111111118', 'approvals:create', 'Create Approvals', 'Create approval requests', 'approvals', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222222d03', '11111111-1111-1111-1111-111111111118', 'approvals:approve', 'Approve Requests', 'Approve pending requests', 'approvals', 'approve', true, false, 3),
    ('22222222-2222-2222-2222-222222222d04', '11111111-1111-1111-1111-111111111118', 'approvals:reject', 'Reject Requests', 'Reject pending requests', 'approvals', 'reject', true, false, 4),
    ('22222222-2222-2222-2222-222222222d05', '11111111-1111-1111-1111-111111111118', 'approvals:manage', 'Manage Approvals', 'Configure approval workflows', 'approvals', 'manage', true, false, 5)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- ============================================================================
-- STEP 4: Add Vendor permissions (matching go-shared naming convention)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222e01', '11111111-1111-1111-1111-111111111119', 'vendors:read', 'View Vendors', 'View vendor listings', 'vendors', 'read', false, false, 1),
    ('22222222-2222-2222-2222-222222222e02', '11111111-1111-1111-1111-111111111119', 'vendors:create', 'Create Vendors', 'Onboard new vendors', 'vendors', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222222e03', '11111111-1111-1111-1111-111111111119', 'vendors:update', 'Edit Vendors', 'Modify vendor details', 'vendors', 'update', true, false, 3),
    ('22222222-2222-2222-2222-222222222e04', '11111111-1111-1111-1111-111111111119', 'vendors:approve', 'Approve Vendors', 'Approve vendor applications', 'vendors', 'approve', true, false, 4),
    ('22222222-2222-2222-2222-222222222e05', '11111111-1111-1111-1111-111111111119', 'vendors:manage', 'Manage Vendors', 'Full vendor management access', 'vendors', 'manage', true, false, 5),
    ('22222222-2222-2222-2222-222222222e06', '11111111-1111-1111-1111-111111111119', 'vendors:payout', 'Process Payouts', 'Process vendor payouts', 'vendors', 'payout', true, true, 6)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- ============================================================================
-- STEP 5: Add Delegation permissions (matching go-shared naming convention)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-22222222f001', '11111111-1111-1111-1111-111111111120', 'delegations:read', 'View Delegations', 'View approval delegations', 'delegations', 'read', false, false, 1),
    ('22222222-2222-2222-2222-22222222f002', '11111111-1111-1111-1111-111111111120', 'delegations:manage', 'Manage Delegations', 'Create and manage delegations', 'delegations', 'manage', true, false, 2)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- ============================================================================
-- STEP 6: Grant new permissions to store_owner (all permissions)
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
AND sp.name IN (
    'tickets:read', 'tickets:create', 'tickets:update', 'tickets:assign', 'tickets:escalate', 'tickets:resolve',
    'approvals:read', 'approvals:create', 'approvals:approve', 'approvals:reject', 'approvals:manage',
    'vendors:read', 'vendors:create', 'vendors:update', 'vendors:approve', 'vendors:manage', 'vendors:payout',
    'delegations:read', 'delegations:manage'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 7: Grant new permissions to store_admin (most except sensitive)
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_admin', 'admin')
AND sp.name IN (
    'tickets:read', 'tickets:create', 'tickets:update', 'tickets:assign', 'tickets:escalate', 'tickets:resolve',
    'approvals:read', 'approvals:create', 'approvals:approve', 'approvals:reject', 'approvals:manage',
    'vendors:read', 'vendors:create', 'vendors:update', 'vendors:approve', 'vendors:manage',
    'delegations:read', 'delegations:manage'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 8: Grant customer support ticket permissions
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'customer_support'
AND sp.name IN (
    'tickets:read', 'tickets:create', 'tickets:update', 'tickets:assign', 'tickets:resolve'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 9: Grant manager approval permissions
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'store_manager'
AND sp.name IN (
    'tickets:read', 'tickets:assign', 'tickets:escalate',
    'approvals:read', 'approvals:approve', 'approvals:reject'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- Added permissions:
-- - 6 ticket permissions (tickets:read, create, update, assign, escalate, resolve)
-- - 5 approval permissions (approvals:read, create, approve, reject, manage)
-- - 6 vendor permissions (vendors:read, create, update, approve, manage, payout)
-- - 2 delegation permissions (delegations:read, manage)
-- Total: 19 new permissions
-- Assigned to: store_owner (all), store_admin (most), customer_support (tickets), store_manager (limited)
