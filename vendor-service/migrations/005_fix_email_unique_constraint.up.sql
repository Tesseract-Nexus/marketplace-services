-- Fix incorrect unique email constraint
-- GORM was creating idx_tenant_email on just email field
-- The correct constraint should be composite on (tenant_id, email)
-- This migration drops the incorrect index if it exists

-- Drop the incorrect email-only unique index created by GORM AutoMigrate
-- This index name was from the GORM tag: uniqueIndex:idx_tenant_email
DROP INDEX IF EXISTS idx_tenant_email;

-- The correct composite index already exists from migration 001:
-- idx_vendors_tenant_email ON vendors(tenant_id, email) WHERE deleted_at IS NULL
-- No need to recreate it

-- Add a comment explaining the constraint
COMMENT ON INDEX idx_vendors_tenant_email IS 'Ensures email uniqueness per tenant, allowing same email across different tenants';
