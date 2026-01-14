-- Migration: 020_add_returns_permissions
-- Description: Add missing returns permissions that match go-shared/rbac/permissions.go

-- ============================================================================
-- STEP 1: Add permission category for returns
-- ============================================================================

INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111121', 'returns', 'Returns', 'Return and refund management', 'RotateCcw', 21, true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 2: Add Returns permissions (matching go-shared naming convention)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-22222222a101', '11111111-1111-1111-1111-111111111121', 'returns:read', 'View Returns', 'View return requests', 'returns', 'read', false, false, 1),
    ('22222222-2222-2222-2222-22222222a102', '11111111-1111-1111-1111-111111111121', 'returns:create', 'Create Returns', 'Create return requests', 'returns', 'create', false, false, 2),
    ('22222222-2222-2222-2222-22222222a103', '11111111-1111-1111-1111-111111111121', 'returns:approve', 'Approve Returns', 'Approve return requests', 'returns', 'approve', true, false, 3),
    ('22222222-2222-2222-2222-22222222a104', '11111111-1111-1111-1111-111111111121', 'returns:reject', 'Reject Returns', 'Reject return requests', 'returns', 'reject', true, false, 4),
    ('22222222-2222-2222-2222-22222222a105', '11111111-1111-1111-1111-111111111121', 'returns:refund', 'Process Refunds', 'Process return refunds', 'returns', 'refund', true, true, 5),
    ('22222222-2222-2222-2222-22222222a106', '11111111-1111-1111-1111-111111111121', 'returns:inspect', 'Inspect Returns', 'Inspect returned items', 'returns', 'inspect', false, false, 6)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- ============================================================================
-- STEP 3: Grant returns permissions to store_owner
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
AND sp.name IN ('returns:read', 'returns:create', 'returns:approve', 'returns:reject', 'returns:refund', 'returns:inspect')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 4: Grant returns permissions to store_admin
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_admin', 'admin')
AND sp.name IN ('returns:read', 'returns:create', 'returns:approve', 'returns:reject', 'returns:refund', 'returns:inspect')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 5: Grant returns permissions to order_manager and customer_support
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'order_manager'
AND sp.name IN ('returns:read', 'returns:create', 'returns:approve', 'returns:reject', 'returns:inspect')
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'customer_support'
AND sp.name IN ('returns:read', 'returns:create')
ON CONFLICT (role_id, permission_id) DO NOTHING;
