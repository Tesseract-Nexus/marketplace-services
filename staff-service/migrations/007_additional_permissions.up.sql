-- Migration: Add Additional Permissions for RBAC Integration
-- Adds permissions for: Tickets, Vendors, Approvals, Storefronts

-- ============================================================================
-- CREATE ROLE DEFAULT PERMISSIONS TABLE (if not exists)
-- This table maps role names to default permission names for easy seeding
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_role_default_permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    role_name VARCHAR(100) NOT NULL,
    permission_name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(role_name, permission_name)
);

CREATE INDEX IF NOT EXISTS idx_staff_role_default_permissions_role ON staff_role_default_permissions(role_name);
CREATE INDEX IF NOT EXISTS idx_staff_role_default_permissions_perm ON staff_role_default_permissions(permission_name);

-- ============================================================================
-- NEW PERMISSION CATEGORIES
-- ============================================================================

INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111110', 'tickets', 'Tickets', 'Support ticket management', 'Ticket', 10, true),
    ('11111111-1111-1111-1111-111111111111', 'vendors', 'Vendors', 'Marketplace vendor management', 'Store', 11, true),
    ('11111111-1111-1111-1111-111111111112', 'approvals', 'Approvals', 'Approval workflow management', 'CheckCircle', 12, true),
    ('11111111-1111-1111-1111-111111111113', 'storefronts', 'Storefronts', 'Storefront management', 'Globe', 13, true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - TICKETS CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222B01', '11111111-1111-1111-1111-111111111110', 'tickets:view', 'View Tickets', 'View support tickets', 'tickets', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222B02', '11111111-1111-1111-1111-111111111110', 'tickets:create', 'Create Tickets', 'Create support tickets', 'tickets', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222222B03', '11111111-1111-1111-1111-111111111110', 'tickets:update', 'Update Tickets', 'Update ticket details and status', 'tickets', 'update', false, false, 3),
    ('22222222-2222-2222-2222-222222222B04', '11111111-1111-1111-1111-111111111110', 'tickets:assign', 'Assign Tickets', 'Assign tickets to staff members', 'tickets', 'assign', false, false, 4),
    ('22222222-2222-2222-2222-222222222B05', '11111111-1111-1111-1111-111111111110', 'tickets:escalate', 'Escalate Tickets', 'Escalate tickets to higher priority', 'tickets', 'escalate', false, false, 5),
    ('22222222-2222-2222-2222-222222222B06', '11111111-1111-1111-1111-111111111110', 'tickets:resolve', 'Resolve Tickets', 'Mark tickets as resolved', 'tickets', 'resolve', false, false, 6),
    ('22222222-2222-2222-2222-222222222B07', '11111111-1111-1111-1111-111111111110', 'tickets:delete', 'Delete Tickets', 'Delete tickets permanently', 'tickets', 'delete', true, false, 7),
    ('22222222-2222-2222-2222-222222222B08', '11111111-1111-1111-1111-111111111110', 'tickets:export', 'Export Tickets', 'Export ticket data', 'tickets', 'export', false, false, 8)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - VENDORS CATEGORY (Marketplace)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222C01', '11111111-1111-1111-1111-111111111111', 'vendors:view', 'View Vendors', 'View vendor profiles and listings', 'vendors', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222C02', '11111111-1111-1111-1111-111111111111', 'vendors:create', 'Create Vendors', 'Create new vendor accounts', 'vendors', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222222C03', '11111111-1111-1111-1111-111111111111', 'vendors:edit', 'Edit Vendors', 'Modify vendor information', 'vendors', 'edit', true, false, 3),
    ('22222222-2222-2222-2222-222222222C04', '11111111-1111-1111-1111-111111111111', 'vendors:approve', 'Approve Vendors', 'Approve or reject vendor applications', 'vendors', 'approve', true, false, 4),
    ('22222222-2222-2222-2222-222222222C05', '11111111-1111-1111-1111-111111111111', 'vendors:suspend', 'Suspend Vendors', 'Suspend vendor accounts', 'vendors', 'suspend', true, true, 5),
    ('22222222-2222-2222-2222-222222222C06', '11111111-1111-1111-1111-111111111111', 'vendors:delete', 'Delete Vendors', 'Delete vendor accounts', 'vendors', 'delete', true, true, 6),
    ('22222222-2222-2222-2222-222222222C07', '11111111-1111-1111-1111-111111111111', 'vendors:payouts:view', 'View Payouts', 'View vendor payout history', 'payouts', 'view', true, false, 7),
    ('22222222-2222-2222-2222-222222222C08', '11111111-1111-1111-1111-111111111111', 'vendors:payouts:manage', 'Manage Payouts', 'Process vendor payouts', 'payouts', 'manage', true, true, 8),
    ('22222222-2222-2222-2222-222222222C09', '11111111-1111-1111-1111-111111111111', 'vendors:products:approve', 'Approve Vendor Products', 'Approve vendor product listings', 'products', 'approve', true, false, 9),
    ('22222222-2222-2222-2222-222222222C10', '11111111-1111-1111-1111-111111111111', 'vendors:commission:manage', 'Manage Commission', 'Configure vendor commission rates', 'commission', 'manage', true, false, 10)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - APPROVALS CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222D01', '11111111-1111-1111-1111-111111111112', 'approvals:view', 'View Approvals', 'View pending and completed approvals', 'approvals', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222D02', '11111111-1111-1111-1111-111111111112', 'approvals:approve', 'Approve Requests', 'Approve pending requests', 'approvals', 'approve', true, false, 2),
    ('22222222-2222-2222-2222-222222222D03', '11111111-1111-1111-1111-111111111112', 'approvals:reject', 'Reject Requests', 'Reject pending requests', 'approvals', 'reject', true, false, 3),
    ('22222222-2222-2222-2222-222222222D04', '11111111-1111-1111-1111-111111111112', 'approvals:escalate', 'Escalate Approvals', 'Escalate approvals to higher authority', 'approvals', 'escalate', true, false, 4),
    ('22222222-2222-2222-2222-222222222D05', '11111111-1111-1111-1111-111111111112', 'approvals:delegate', 'Delegate Approvals', 'Delegate approval authority to others', 'approvals', 'delegate', true, false, 5),
    ('22222222-2222-2222-2222-222222222D06', '11111111-1111-1111-1111-111111111112', 'approvals:workflows:view', 'View Workflows', 'View approval workflow configurations', 'workflows', 'view', false, false, 6),
    ('22222222-2222-2222-2222-222222222D07', '11111111-1111-1111-1111-111111111112', 'approvals:workflows:manage', 'Manage Workflows', 'Create and configure approval workflows', 'workflows', 'manage', true, false, 7),
    ('22222222-2222-2222-2222-222222222D08', '11111111-1111-1111-1111-111111111112', 'approvals:emergency', 'Emergency Bypass', 'Bypass approval for emergencies', 'approvals', 'emergency', true, true, 8),
    ('22222222-2222-2222-2222-222222222D09', '11111111-1111-1111-1111-111111111112', 'approvals:history:view', 'View Approval History', 'View approval audit trail', 'history', 'view', false, false, 9)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - STOREFRONTS CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222E01', '11111111-1111-1111-1111-111111111113', 'storefronts:view', 'View Storefronts', 'View storefront configurations', 'storefronts', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222E02', '11111111-1111-1111-1111-111111111113', 'storefronts:create', 'Create Storefronts', 'Create new storefronts', 'storefronts', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222222E03', '11111111-1111-1111-1111-111111111113', 'storefronts:edit', 'Edit Storefronts', 'Modify storefront settings', 'storefronts', 'edit', true, false, 3),
    ('22222222-2222-2222-2222-222222222E04', '11111111-1111-1111-1111-111111111113', 'storefronts:delete', 'Delete Storefronts', 'Remove storefronts', 'storefronts', 'delete', true, true, 4),
    ('22222222-2222-2222-2222-222222222E05', '11111111-1111-1111-1111-111111111113', 'storefronts:domains:manage', 'Manage Domains', 'Configure custom domains', 'domains', 'manage', true, false, 5),
    ('22222222-2222-2222-2222-222222222E06', '11111111-1111-1111-1111-111111111113', 'storefronts:theme:manage', 'Manage Theme', 'Customize storefront appearance', 'theme', 'manage', false, false, 6),
    ('22222222-2222-2222-2222-222222222E07', '11111111-1111-1111-1111-111111111113', 'storefronts:pages:manage', 'Manage Pages', 'Create and edit storefront pages', 'pages', 'manage', false, false, 7),
    ('22222222-2222-2222-2222-222222222E08', '11111111-1111-1111-1111-111111111113', 'storefronts:menus:manage', 'Manage Menus', 'Configure navigation menus', 'menus', 'manage', false, false, 8)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- ADDITIONAL RETURNS PERMISSIONS (More granular)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222310', '11111111-1111-1111-1111-111111111102', 'orders:returns:view', 'View Returns', 'View return requests', 'returns', 'view', false, false, 10),
    ('22222222-2222-2222-2222-222222222311', '11111111-1111-1111-1111-111111111102', 'orders:returns:approve', 'Approve Returns', 'Approve return requests', 'returns', 'approve', true, false, 11),
    ('22222222-2222-2222-2222-222222222312', '11111111-1111-1111-1111-111111111102', 'orders:returns:reject', 'Reject Returns', 'Reject return requests', 'returns', 'reject', true, false, 12),
    ('22222222-2222-2222-2222-222222222313', '11111111-1111-1111-1111-111111111102', 'orders:returns:inspect', 'Inspect Returns', 'Inspect returned items', 'returns', 'inspect', false, false, 13),
    ('22222222-2222-2222-2222-222222222314', '11111111-1111-1111-1111-111111111102', 'orders:returns:refund', 'Process Return Refunds', 'Issue refunds for returns', 'returns', 'refund', true, true, 14)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- UPDATE DEFAULT ROLE PERMISSIONS
-- ============================================================================

-- Add ticket permissions to existing roles
-- Owner gets all ticket permissions
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    ('owner', 'tickets:view'),
    ('owner', 'tickets:create'),
    ('owner', 'tickets:update'),
    ('owner', 'tickets:assign'),
    ('owner', 'tickets:escalate'),
    ('owner', 'tickets:resolve'),
    ('owner', 'tickets:delete'),
    ('owner', 'tickets:export')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Admin gets most ticket permissions
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    ('admin', 'tickets:view'),
    ('admin', 'tickets:create'),
    ('admin', 'tickets:update'),
    ('admin', 'tickets:assign'),
    ('admin', 'tickets:escalate'),
    ('admin', 'tickets:resolve'),
    ('admin', 'tickets:export')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Manager gets ticket management
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    ('manager', 'tickets:view'),
    ('manager', 'tickets:create'),
    ('manager', 'tickets:update'),
    ('manager', 'tickets:assign'),
    ('manager', 'tickets:resolve')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Member gets basic ticket access
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    ('member', 'tickets:view'),
    ('member', 'tickets:create'),
    ('member', 'tickets:update')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Viewer gets read-only ticket access
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    ('viewer', 'tickets:view')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Add approval permissions to roles
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    ('owner', 'approvals:view'),
    ('owner', 'approvals:approve'),
    ('owner', 'approvals:reject'),
    ('owner', 'approvals:escalate'),
    ('owner', 'approvals:delegate'),
    ('owner', 'approvals:workflows:view'),
    ('owner', 'approvals:workflows:manage'),
    ('owner', 'approvals:emergency'),
    ('owner', 'approvals:history:view'),
    ('admin', 'approvals:view'),
    ('admin', 'approvals:approve'),
    ('admin', 'approvals:reject'),
    ('admin', 'approvals:escalate'),
    ('admin', 'approvals:delegate'),
    ('admin', 'approvals:workflows:view'),
    ('admin', 'approvals:history:view'),
    ('manager', 'approvals:view'),
    ('manager', 'approvals:approve'),
    ('manager', 'approvals:reject'),
    ('manager', 'approvals:history:view'),
    ('member', 'approvals:view'),
    ('viewer', 'approvals:view')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Add vendor permissions to roles (for marketplace tenants)
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    ('owner', 'vendors:view'),
    ('owner', 'vendors:create'),
    ('owner', 'vendors:edit'),
    ('owner', 'vendors:approve'),
    ('owner', 'vendors:suspend'),
    ('owner', 'vendors:delete'),
    ('owner', 'vendors:payouts:view'),
    ('owner', 'vendors:payouts:manage'),
    ('owner', 'vendors:products:approve'),
    ('owner', 'vendors:commission:manage'),
    ('admin', 'vendors:view'),
    ('admin', 'vendors:create'),
    ('admin', 'vendors:edit'),
    ('admin', 'vendors:approve'),
    ('admin', 'vendors:suspend'),
    ('admin', 'vendors:payouts:view'),
    ('admin', 'vendors:products:approve'),
    ('manager', 'vendors:view'),
    ('manager', 'vendors:products:approve'),
    ('member', 'vendors:view'),
    ('viewer', 'vendors:view')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Add storefront permissions to roles
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    ('owner', 'storefronts:view'),
    ('owner', 'storefronts:create'),
    ('owner', 'storefronts:edit'),
    ('owner', 'storefronts:delete'),
    ('owner', 'storefronts:domains:manage'),
    ('owner', 'storefronts:theme:manage'),
    ('owner', 'storefronts:pages:manage'),
    ('owner', 'storefronts:menus:manage'),
    ('admin', 'storefronts:view'),
    ('admin', 'storefronts:create'),
    ('admin', 'storefronts:edit'),
    ('admin', 'storefronts:domains:manage'),
    ('admin', 'storefronts:theme:manage'),
    ('admin', 'storefronts:pages:manage'),
    ('admin', 'storefronts:menus:manage'),
    ('manager', 'storefronts:view'),
    ('manager', 'storefronts:theme:manage'),
    ('manager', 'storefronts:pages:manage'),
    ('manager', 'storefronts:menus:manage'),
    ('member', 'storefronts:view'),
    ('viewer', 'storefronts:view')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Add returns permissions to roles
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    ('owner', 'orders:returns:view'),
    ('owner', 'orders:returns:approve'),
    ('owner', 'orders:returns:reject'),
    ('owner', 'orders:returns:inspect'),
    ('owner', 'orders:returns:refund'),
    ('admin', 'orders:returns:view'),
    ('admin', 'orders:returns:approve'),
    ('admin', 'orders:returns:reject'),
    ('admin', 'orders:returns:inspect'),
    ('admin', 'orders:returns:refund'),
    ('manager', 'orders:returns:view'),
    ('manager', 'orders:returns:approve'),
    ('manager', 'orders:returns:reject'),
    ('manager', 'orders:returns:inspect'),
    ('member', 'orders:returns:view'),
    ('member', 'orders:returns:inspect'),
    ('viewer', 'orders:returns:view')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- Added 41 new permissions:
-- - 8 Tickets permissions
-- - 10 Vendors permissions
-- - 9 Approvals permissions
-- - 8 Storefronts permissions
-- - 5 Additional Returns permissions
-- - 1 new category for each
