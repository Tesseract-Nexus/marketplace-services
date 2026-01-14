-- =============================================================================
-- Marketplace Connector Service - HLD Services Migration Rollback
-- Migration: 003_hld_services
-- =============================================================================

-- Drop triggers first
DROP TRIGGER IF EXISTS update_marketplace_order_lines_updated_at ON marketplace_imported_order_lines;
DROP TRIGGER IF EXISTS update_marketplace_imported_orders_updated_at ON marketplace_imported_orders;
DROP TRIGGER IF EXISTS update_marketplace_mismatches_updated_at ON marketplace_inventory_mismatches;
DROP TRIGGER IF EXISTS update_marketplace_discrepancies_updated_at ON marketplace_reconciliation_discrepancies;
DROP TRIGGER IF EXISTS update_marketplace_recon_jobs_updated_at ON marketplace_reconciliation_jobs;
DROP TRIGGER IF EXISTS update_marketplace_event_versions_updated_at ON marketplace_event_versions;

-- Drop RLS policies
DROP POLICY IF EXISTS tenant_isolation_order_lines ON marketplace_imported_order_lines;
DROP POLICY IF EXISTS tenant_isolation_imported_orders ON marketplace_imported_orders;
DROP POLICY IF EXISTS tenant_isolation_mismatches ON marketplace_inventory_mismatches;
DROP POLICY IF EXISTS tenant_isolation_discrepancies ON marketplace_reconciliation_discrepancies;
DROP POLICY IF EXISTS tenant_isolation_recon_jobs ON marketplace_reconciliation_jobs;
DROP POLICY IF EXISTS tenant_isolation_ooo_events ON marketplace_out_of_order_events;
DROP POLICY IF EXISTS tenant_isolation_event_versions ON marketplace_event_versions;

-- Drop tables (order matters due to foreign keys)
DROP TABLE IF EXISTS marketplace_imported_order_lines;
DROP TABLE IF EXISTS marketplace_imported_orders;
DROP TABLE IF EXISTS marketplace_inventory_mismatches;
DROP TABLE IF EXISTS marketplace_reconciliation_discrepancies;
DROP TABLE IF EXISTS marketplace_reconciliation_jobs;
DROP TABLE IF EXISTS marketplace_out_of_order_events;
DROP TABLE IF EXISTS marketplace_event_versions;
