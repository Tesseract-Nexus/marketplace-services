-- =============================================================================
-- Marketplace Connector Service - HLD Compliance Migration Rollback
-- Migration: 002_hld_compliance (DOWN)
-- =============================================================================

-- Drop RLS policies first
DROP POLICY IF EXISTS tenant_isolation_connections ON marketplace_connections;
DROP POLICY IF EXISTS tenant_isolation_credentials ON marketplace_credentials;
DROP POLICY IF EXISTS tenant_isolation_sync_jobs ON marketplace_sync_jobs;
DROP POLICY IF EXISTS tenant_isolation_sync_logs ON marketplace_sync_logs;
DROP POLICY IF EXISTS tenant_isolation_product_mappings ON marketplace_product_mappings;
DROP POLICY IF EXISTS tenant_isolation_order_mappings ON marketplace_order_mappings;
DROP POLICY IF EXISTS tenant_isolation_inventory_mappings ON marketplace_inventory_mappings;
DROP POLICY IF EXISTS tenant_isolation_webhook_events ON marketplace_webhook_events;
DROP POLICY IF EXISTS tenant_isolation_api_keys ON marketplace_api_keys;
DROP POLICY IF EXISTS tenant_isolation_catalog_items ON marketplace_catalog_items;
DROP POLICY IF EXISTS tenant_isolation_catalog_variants ON marketplace_catalog_variants;
DROP POLICY IF EXISTS tenant_isolation_offers ON marketplace_offers;
DROP POLICY IF EXISTS tenant_isolation_inventory_current ON marketplace_inventory_current;
DROP POLICY IF EXISTS tenant_isolation_inventory_ledgers ON marketplace_inventory_ledgers;
DROP POLICY IF EXISTS tenant_isolation_external_mappings ON marketplace_external_mappings;
DROP POLICY IF EXISTS tenant_isolation_raw_snapshots ON marketplace_raw_snapshots;
DROP POLICY IF EXISTS tenant_isolation_audit_logs ON marketplace_audit_logs;
DROP POLICY IF EXISTS tenant_isolation_encryption_keys ON marketplace_encryption_keys;

-- Disable RLS
ALTER TABLE marketplace_connections DISABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_credentials DISABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_sync_jobs DISABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_sync_logs DISABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_product_mappings DISABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_order_mappings DISABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_inventory_mappings DISABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_webhook_events DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_marketplace_api_keys_updated_at ON marketplace_api_keys;
DROP TRIGGER IF EXISTS update_marketplace_catalog_items_updated_at ON marketplace_catalog_items;
DROP TRIGGER IF EXISTS update_marketplace_catalog_variants_updated_at ON marketplace_catalog_variants;
DROP TRIGGER IF EXISTS update_marketplace_offers_updated_at ON marketplace_offers;
DROP TRIGGER IF EXISTS update_marketplace_inventory_current_updated_at ON marketplace_inventory_current;
DROP TRIGGER IF EXISTS update_marketplace_external_mappings_updated_at ON marketplace_external_mappings;
DROP TRIGGER IF EXISTS update_marketplace_encryption_keys_updated_at ON marketplace_encryption_keys;

-- Drop new tables (in correct order due to foreign keys)
DROP TABLE IF EXISTS marketplace_audit_logs;
DROP TABLE IF EXISTS marketplace_raw_snapshots;
DROP TABLE IF EXISTS marketplace_external_mappings;
DROP TABLE IF EXISTS marketplace_inventory_ledgers;
DROP TABLE IF EXISTS marketplace_inventory_current;
DROP TABLE IF EXISTS marketplace_offers;
DROP TABLE IF EXISTS marketplace_catalog_variants;
DROP TABLE IF EXISTS marketplace_catalog_items;
DROP TABLE IF EXISTS marketplace_encryption_keys;
DROP TABLE IF EXISTS marketplace_api_keys;

-- Remove added columns from sync_jobs
ALTER TABLE marketplace_sync_jobs
    DROP COLUMN IF EXISTS job_type,
    DROP COLUMN IF EXISTS cursor_position,
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS parent_job_id,
    DROP COLUMN IF EXISTS priority;

-- Remove added columns from credentials
ALTER TABLE marketplace_credentials
    DROP COLUMN IF EXISTS tenant_id;
