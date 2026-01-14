-- Migration: Add RBAC System (Roles, Permissions, Departments, Teams, Documents)
-- This migration extends the staff system with full Role-Based Access Control

-- ============================================================================
-- ENUM TYPES
-- ============================================================================

-- Document types for staff verification
DO $$ BEGIN
    CREATE TYPE staff_document_type AS ENUM (
        'id_proof_government_id',
        'id_proof_passport',
        'id_proof_drivers_license',
        'address_proof',
        'employment_contract',
        'offer_letter',
        'tax_w9',
        'tax_i9',
        'tax_w4',
        'tax_other',
        'background_check',
        'professional_certification',
        'education_certificate',
        'emergency_contact_form',
        'nda_agreement',
        'non_compete_agreement',
        'bank_details',
        'health_insurance',
        'other'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Document verification status
DO $$ BEGIN
    CREATE TYPE document_verification_status AS ENUM (
        'pending',
        'under_review',
        'verified',
        'rejected',
        'expired',
        'requires_update'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Document access level
DO $$ BEGIN
    CREATE TYPE document_access_level AS ENUM (
        'self_only',
        'manager',
        'hr_only',
        'admin_only'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- ============================================================================
-- DEPARTMENTS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS departments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255), -- NULL means tenant-level, otherwise vendor-specific
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50),
    description TEXT,
    parent_department_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    department_head_id UUID, -- Will be FK to staff, added after staff update
    budget DECIMAL(15, 2),
    cost_center VARCHAR(100),
    location VARCHAR(255),
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

-- Indexes for departments
CREATE INDEX IF NOT EXISTS idx_departments_tenant_id ON departments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_departments_vendor_id ON departments(vendor_id);
CREATE INDEX IF NOT EXISTS idx_departments_tenant_vendor ON departments(tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_departments_parent_id ON departments(parent_department_id);
CREATE INDEX IF NOT EXISTS idx_departments_is_active ON departments(is_active);
CREATE INDEX IF NOT EXISTS idx_departments_deleted_at ON departments(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_departments_tenant_vendor_code ON departments(tenant_id, COALESCE(vendor_id, ''), code) WHERE deleted_at IS NULL AND code IS NOT NULL;

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_departments_updated_at ON departments;
CREATE TRIGGER update_departments_updated_at
    BEFORE UPDATE ON departments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- TEAMS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255), -- Inherits from department, NULL means tenant-level
    department_id UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50),
    description TEXT,
    team_lead_id UUID, -- Will be FK to staff
    max_capacity INTEGER,
    slack_channel VARCHAR(255),
    email_alias VARCHAR(255),
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

-- Indexes for teams
CREATE INDEX IF NOT EXISTS idx_teams_tenant_id ON teams(tenant_id);
CREATE INDEX IF NOT EXISTS idx_teams_vendor_id ON teams(vendor_id);
CREATE INDEX IF NOT EXISTS idx_teams_tenant_vendor ON teams(tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_teams_department_id ON teams(department_id);
CREATE INDEX IF NOT EXISTS idx_teams_team_lead_id ON teams(team_lead_id);
CREATE INDEX IF NOT EXISTS idx_teams_is_active ON teams(is_active);
CREATE INDEX IF NOT EXISTS idx_teams_deleted_at ON teams(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_teams_tenant_vendor_code ON teams(tenant_id, COALESCE(vendor_id, ''), code) WHERE deleted_at IS NULL AND code IS NOT NULL;

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_teams_updated_at ON teams;
CREATE TRIGGER update_teams_updated_at
    BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- PERMISSION CATEGORIES TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS permission_categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    icon VARCHAR(50),
    sort_order INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- STAFF PERMISSIONS TABLE (Global permission definitions)
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    category_id UUID REFERENCES permission_categories(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL UNIQUE, -- e.g., 'catalog:products:create'
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    resource VARCHAR(100), -- e.g., 'products'
    action VARCHAR(100),   -- e.g., 'create'
    is_sensitive BOOLEAN DEFAULT false,
    requires_2fa BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for permissions
CREATE INDEX IF NOT EXISTS idx_staff_permissions_category_id ON staff_permissions(category_id);
CREATE INDEX IF NOT EXISTS idx_staff_permissions_resource ON staff_permissions(resource);
CREATE INDEX IF NOT EXISTS idx_staff_permissions_is_active ON staff_permissions(is_active);

-- ============================================================================
-- STAFF ROLES TABLE (Tenant-specific custom roles)
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255), -- NULL means tenant-level role, otherwise vendor-specific
    name VARCHAR(100) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    priority_level INTEGER DEFAULT 0, -- Higher = more authority (Owner=100, Viewer=10)
    color VARCHAR(20) DEFAULT '#6B7280', -- Tailwind gray-500
    icon VARCHAR(50) DEFAULT 'UserCircle',
    is_system BOOLEAN DEFAULT false, -- System roles cannot be deleted
    is_template BOOLEAN DEFAULT false, -- Template roles for quick creation
    template_source VARCHAR(100), -- Which template this was created from
    can_manage_staff BOOLEAN DEFAULT false,
    can_create_roles BOOLEAN DEFAULT false,
    can_delete_roles BOOLEAN DEFAULT false,
    max_assignable_priority INTEGER, -- Max priority level this role can assign to others
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

-- Indexes for staff_roles
CREATE INDEX IF NOT EXISTS idx_staff_roles_tenant_id ON staff_roles(tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_roles_vendor_id ON staff_roles(vendor_id);
CREATE INDEX IF NOT EXISTS idx_staff_roles_tenant_vendor ON staff_roles(tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_staff_roles_priority_level ON staff_roles(priority_level);
CREATE INDEX IF NOT EXISTS idx_staff_roles_is_system ON staff_roles(is_system);
CREATE INDEX IF NOT EXISTS idx_staff_roles_is_active ON staff_roles(is_active);
CREATE INDEX IF NOT EXISTS idx_staff_roles_deleted_at ON staff_roles(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_roles_tenant_vendor_name ON staff_roles(tenant_id, COALESCE(vendor_id, ''), name) WHERE deleted_at IS NULL;

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_staff_roles_updated_at ON staff_roles;
CREATE TRIGGER update_staff_roles_updated_at
    BEFORE UPDATE ON staff_roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- STAFF ROLE PERMISSIONS (Junction table: which permissions each role has)
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_role_permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    role_id UUID NOT NULL REFERENCES staff_roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES staff_permissions(id) ON DELETE CASCADE,
    granted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    granted_by VARCHAR(255),
    UNIQUE(role_id, permission_id)
);

-- Indexes for role_permissions
CREATE INDEX IF NOT EXISTS idx_staff_role_permissions_role_id ON staff_role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_staff_role_permissions_permission_id ON staff_role_permissions(permission_id);

-- ============================================================================
-- STAFF ROLE ASSIGNMENTS (Which roles are assigned to which staff)
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_role_assignments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255), -- For vendor-specific role assignments
    staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES staff_roles(id) ON DELETE CASCADE,
    is_primary BOOLEAN DEFAULT false, -- Primary role for display purposes
    scope VARCHAR(100), -- Optional: 'global', 'department:xxx', 'team:xxx', 'vendor:xxx'
    scope_id UUID, -- Optional: department_id, team_id, or vendor_id for scoped roles
    assigned_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    assigned_by UUID REFERENCES staff(id) ON DELETE SET NULL,
    expires_at TIMESTAMP WITH TIME ZONE, -- For temporary role assignments
    notes TEXT,
    is_active BOOLEAN DEFAULT true
);

-- Unique constraint for role assignments (using index because COALESCE not allowed in table constraint)
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_role_assignments_unique
    ON staff_role_assignments(staff_id, role_id, COALESCE(vendor_id, ''), COALESCE(scope, ''), COALESCE(scope_id, '00000000-0000-0000-0000-000000000000'::uuid));

-- Indexes for role_assignments
CREATE INDEX IF NOT EXISTS idx_staff_role_assignments_tenant_id ON staff_role_assignments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_role_assignments_vendor_id ON staff_role_assignments(vendor_id);
CREATE INDEX IF NOT EXISTS idx_staff_role_assignments_tenant_vendor ON staff_role_assignments(tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_staff_role_assignments_staff_id ON staff_role_assignments(staff_id);
CREATE INDEX IF NOT EXISTS idx_staff_role_assignments_role_id ON staff_role_assignments(role_id);
CREATE INDEX IF NOT EXISTS idx_staff_role_assignments_is_primary ON staff_role_assignments(is_primary);
CREATE INDEX IF NOT EXISTS idx_staff_role_assignments_expires_at ON staff_role_assignments(expires_at);
CREATE INDEX IF NOT EXISTS idx_staff_role_assignments_is_active ON staff_role_assignments(is_active);

-- ============================================================================
-- STAFF DOCUMENTS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255), -- For vendor-specific document requirements
    staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    document_type staff_document_type NOT NULL,
    document_name VARCHAR(255) NOT NULL, -- Display name
    original_filename VARCHAR(500),
    document_number VARCHAR(255), -- ID number, passport number, etc.
    issuing_authority VARCHAR(255),
    issue_date DATE,
    expiry_date DATE,
    storage_path TEXT NOT NULL, -- Path in document-service/GCS
    file_size BIGINT,
    mime_type VARCHAR(100),
    verification_status document_verification_status NOT NULL DEFAULT 'pending',
    verified_at TIMESTAMP WITH TIME ZONE,
    verified_by UUID REFERENCES staff(id) ON DELETE SET NULL,
    verification_notes TEXT,
    rejection_reason TEXT,
    access_level document_access_level NOT NULL DEFAULT 'hr_only',
    is_mandatory BOOLEAN DEFAULT false,
    reminder_sent_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

-- Indexes for staff_documents
CREATE INDEX IF NOT EXISTS idx_staff_documents_tenant_id ON staff_documents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_documents_vendor_id ON staff_documents(vendor_id);
CREATE INDEX IF NOT EXISTS idx_staff_documents_tenant_vendor ON staff_documents(tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_staff_documents_staff_id ON staff_documents(staff_id);
CREATE INDEX IF NOT EXISTS idx_staff_documents_document_type ON staff_documents(document_type);
CREATE INDEX IF NOT EXISTS idx_staff_documents_verification_status ON staff_documents(verification_status);
CREATE INDEX IF NOT EXISTS idx_staff_documents_expiry_date ON staff_documents(expiry_date);
CREATE INDEX IF NOT EXISTS idx_staff_documents_deleted_at ON staff_documents(deleted_at);

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_staff_documents_updated_at ON staff_documents;
CREATE TRIGGER update_staff_documents_updated_at
    BEFORE UPDATE ON staff_documents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- STAFF EMERGENCY CONTACTS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_emergency_contacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255), -- Inherited from staff's vendor
    staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    relationship VARCHAR(100),
    phone_primary VARCHAR(50) NOT NULL,
    phone_secondary VARCHAR(50),
    email VARCHAR(255),
    address TEXT,
    is_primary BOOLEAN DEFAULT false,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for emergency_contacts
CREATE INDEX IF NOT EXISTS idx_staff_emergency_contacts_tenant_id ON staff_emergency_contacts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_emergency_contacts_vendor_id ON staff_emergency_contacts(vendor_id);
CREATE INDEX IF NOT EXISTS idx_staff_emergency_contacts_staff_id ON staff_emergency_contacts(staff_id);
CREATE INDEX IF NOT EXISTS idx_staff_emergency_contacts_is_primary ON staff_emergency_contacts(is_primary);

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_staff_emergency_contacts_updated_at ON staff_emergency_contacts;
CREATE TRIGGER update_staff_emergency_contacts_updated_at
    BEFORE UPDATE ON staff_emergency_contacts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- AUDIT LOG FOR RBAC CHANGES
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_rbac_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255), -- For vendor-scoped audit entries
    action VARCHAR(100) NOT NULL, -- 'role_assigned', 'role_removed', 'permission_granted', etc.
    entity_type VARCHAR(50) NOT NULL, -- 'staff', 'role', 'permission', 'document'
    entity_id UUID NOT NULL,
    target_staff_id UUID REFERENCES staff(id) ON DELETE SET NULL,
    performed_by UUID REFERENCES staff(id) ON DELETE SET NULL,
    old_value JSONB,
    new_value JSONB,
    ip_address INET,
    user_agent TEXT,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for audit log
CREATE INDEX IF NOT EXISTS idx_staff_rbac_audit_log_tenant_id ON staff_rbac_audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_rbac_audit_log_vendor_id ON staff_rbac_audit_log(vendor_id);
CREATE INDEX IF NOT EXISTS idx_staff_rbac_audit_log_tenant_vendor ON staff_rbac_audit_log(tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_staff_rbac_audit_log_action ON staff_rbac_audit_log(action);
CREATE INDEX IF NOT EXISTS idx_staff_rbac_audit_log_entity ON staff_rbac_audit_log(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_staff_rbac_audit_log_target_staff ON staff_rbac_audit_log(target_staff_id);
CREATE INDEX IF NOT EXISTS idx_staff_rbac_audit_log_performed_by ON staff_rbac_audit_log(performed_by);
CREATE INDEX IF NOT EXISTS idx_staff_rbac_audit_log_created_at ON staff_rbac_audit_log(created_at);

-- ============================================================================
-- UPDATE STAFF TABLE WITH NEW FK REFERENCES
-- ============================================================================

-- Add proper UUID references to departments and teams
ALTER TABLE staff ADD COLUMN IF NOT EXISTS department_uuid UUID REFERENCES departments(id) ON DELETE SET NULL;
ALTER TABLE staff ADD COLUMN IF NOT EXISTS team_uuid UUID REFERENCES teams(id) ON DELETE SET NULL;

-- Create indexes for the new columns
CREATE INDEX IF NOT EXISTS idx_staff_department_uuid ON staff(department_uuid);
CREATE INDEX IF NOT EXISTS idx_staff_team_uuid ON staff(team_uuid);

-- Add FK constraints for department_head_id and team_lead_id now that staff table exists
-- (These are circular references, so we add them after both tables exist)
ALTER TABLE departments
    ADD CONSTRAINT fk_departments_head
    FOREIGN KEY (department_head_id)
    REFERENCES staff(id)
    ON DELETE SET NULL;

ALTER TABLE teams
    ADD CONSTRAINT fk_teams_lead
    FOREIGN KEY (team_lead_id)
    REFERENCES staff(id)
    ON DELETE SET NULL;

-- ============================================================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================================================

COMMENT ON TABLE departments IS 'Organizational departments with hierarchical structure';
COMMENT ON TABLE teams IS 'Teams within departments for granular staff organization';
COMMENT ON TABLE permission_categories IS 'Categories for grouping related permissions';
COMMENT ON TABLE staff_permissions IS 'Global permission definitions available across all tenants';
COMMENT ON TABLE staff_roles IS 'Tenant-specific roles with customizable permissions';
COMMENT ON TABLE staff_role_permissions IS 'Junction table linking roles to their permissions';
COMMENT ON TABLE staff_role_assignments IS 'Assignments of roles to staff members';
COMMENT ON TABLE staff_documents IS 'Staff verification documents with approval workflow';
COMMENT ON TABLE staff_emergency_contacts IS 'Emergency contact information for staff';
COMMENT ON TABLE staff_rbac_audit_log IS 'Audit trail for all RBAC-related changes';

COMMENT ON COLUMN staff_roles.priority_level IS 'Higher values = more authority. Owner=100, Viewer=10. Staff can only assign roles with priority <= their own max priority.';
COMMENT ON COLUMN staff_roles.is_system IS 'System roles cannot be deleted or have their core permissions modified';
COMMENT ON COLUMN staff_role_assignments.scope IS 'Scope of the role: global, department:uuid, team:uuid';
COMMENT ON COLUMN staff_documents.access_level IS 'Who can view this document: self_only, manager, hr_only, admin_only';
