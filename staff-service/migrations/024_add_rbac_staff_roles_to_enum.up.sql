-- Migration: Add RBAC role values to staff_role enum
-- The staff.role column uses an enum that needs to include all RBAC roles from staff_roles table
-- This allows staff members to be created with roles like store_admin, store_manager, etc.

-- Add new role values to the staff_role enum
-- Using IF NOT EXISTS pattern via DO block since ALTER TYPE ADD VALUE doesn't support IF NOT EXISTS in all PG versions
DO $$
BEGIN
    -- Store hierarchy roles
    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'store_owner' AND enumtypid = 'staff_role'::regtype) THEN
        ALTER TYPE staff_role ADD VALUE 'store_owner';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'store_admin' AND enumtypid = 'staff_role'::regtype) THEN
        ALTER TYPE staff_role ADD VALUE 'store_admin';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'store_manager' AND enumtypid = 'staff_role'::regtype) THEN
        ALTER TYPE staff_role ADD VALUE 'store_manager';
    END IF;

    -- Department-specific roles
    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'inventory_manager' AND enumtypid = 'staff_role'::regtype) THEN
        ALTER TYPE staff_role ADD VALUE 'inventory_manager';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'marketing_manager' AND enumtypid = 'staff_role'::regtype) THEN
        ALTER TYPE staff_role ADD VALUE 'marketing_manager';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'order_manager' AND enumtypid = 'staff_role'::regtype) THEN
        ALTER TYPE staff_role ADD VALUE 'order_manager';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'customer_support' AND enumtypid = 'staff_role'::regtype) THEN
        ALTER TYPE staff_role ADD VALUE 'customer_support';
    END IF;

    -- Read-only role
    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'viewer' AND enumtypid = 'staff_role'::regtype) THEN
        ALTER TYPE staff_role ADD VALUE 'viewer';
    END IF;
END$$;

-- Add comment explaining the enum values
COMMENT ON TYPE staff_role IS 'Staff role enum including legacy roles (super_admin, admin, etc.) and RBAC roles (store_owner, store_admin, etc.)';
