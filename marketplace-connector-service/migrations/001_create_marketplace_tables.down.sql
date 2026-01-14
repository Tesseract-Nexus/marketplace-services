-- =============================================================================
-- Marketplace Connector Service - Database Schema Rollback
-- Migration: 001_create_marketplace_tables (DOWN)
-- =============================================================================

-- Drop triggers
DROP TRIGGER IF EXISTS update_marketplace_inventory_mappings_updated_at ON marketplace_inventory_mappings;
DROP TRIGGER IF EXISTS update_marketplace_order_mappings_updated_at ON marketplace_order_mappings;
DROP TRIGGER IF EXISTS update_marketplace_product_mappings_updated_at ON marketplace_product_mappings;
DROP TRIGGER IF EXISTS update_marketplace_sync_jobs_updated_at ON marketplace_sync_jobs;
DROP TRIGGER IF EXISTS update_marketplace_credentials_updated_at ON marketplace_credentials;
DROP TRIGGER IF EXISTS update_marketplace_connections_updated_at ON marketplace_connections;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order (respect foreign key constraints)
DROP TABLE IF EXISTS marketplace_webhook_events;
DROP TABLE IF EXISTS marketplace_inventory_mappings;
DROP TABLE IF EXISTS marketplace_order_mappings;
DROP TABLE IF EXISTS marketplace_product_mappings;
DROP TABLE IF EXISTS marketplace_sync_logs;
DROP TABLE IF EXISTS marketplace_sync_jobs;
DROP TABLE IF EXISTS marketplace_credentials;
DROP TABLE IF EXISTS marketplace_connections;
