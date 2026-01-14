-- =============================================================================
-- Marketplace Connector Service - Database Schema
-- Migration: 001_create_marketplace_tables
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Marketplace Connections
-- Stores connection configuration for each tenant's marketplace integration
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255) NOT NULL,
    marketplace_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    secret_reference VARCHAR(512),
    sync_settings JSONB DEFAULT '{}',
    webhook_url VARCHAR(512),
    webhook_secret VARCHAR(255),
    last_sync_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_marketplace_type CHECK (marketplace_type IN ('AMAZON', 'SHOPIFY', 'DUKAAN')),
    CONSTRAINT chk_status CHECK (status IN ('pending', 'connected', 'disconnected', 'error'))
);

-- Index for tenant lookups
CREATE INDEX idx_marketplace_connections_tenant ON marketplace_connections(tenant_id);
CREATE INDEX idx_marketplace_connections_vendor ON marketplace_connections(vendor_id);
CREATE INDEX idx_marketplace_connections_type ON marketplace_connections(marketplace_type);
CREATE INDEX idx_marketplace_connections_status ON marketplace_connections(status);

-- Unique constraint for tenant + vendor + marketplace combination
CREATE UNIQUE INDEX idx_marketplace_connections_unique
    ON marketplace_connections(tenant_id, vendor_id, marketplace_type);

-- -----------------------------------------------------------------------------
-- Marketplace Credentials
-- Stores GCP Secret Manager references for credentials
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES marketplace_connections(id) ON DELETE CASCADE,
    gcp_secret_name VARCHAR(512) NOT NULL,
    credential_type VARCHAR(100) NOT NULL,
    version VARCHAR(50),
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_marketplace_credentials_connection ON marketplace_credentials(connection_id);

-- -----------------------------------------------------------------------------
-- Marketplace Sync Jobs
-- Tracks sync operations and their progress
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_sync_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    connection_id UUID NOT NULL REFERENCES marketplace_connections(id) ON DELETE CASCADE,
    sync_type VARCHAR(50) NOT NULL,
    direction VARCHAR(20) NOT NULL DEFAULT 'inbound',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    progress JSONB DEFAULT '{"current": 0, "total": 0, "percentage": 0}',
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_sync_type CHECK (sync_type IN ('full', 'incremental', 'products', 'orders', 'inventory')),
    CONSTRAINT chk_sync_direction CHECK (direction IN ('inbound', 'outbound')),
    CONSTRAINT chk_sync_status CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled'))
);

CREATE INDEX idx_marketplace_sync_jobs_tenant ON marketplace_sync_jobs(tenant_id);
CREATE INDEX idx_marketplace_sync_jobs_connection ON marketplace_sync_jobs(connection_id);
CREATE INDEX idx_marketplace_sync_jobs_status ON marketplace_sync_jobs(status);
CREATE INDEX idx_marketplace_sync_jobs_created ON marketplace_sync_jobs(created_at DESC);

-- -----------------------------------------------------------------------------
-- Marketplace Sync Logs
-- Audit trail for sync operations
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_sync_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES marketplace_sync_jobs(id) ON DELETE CASCADE,
    level VARCHAR(20) NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    details JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_log_level CHECK (level IN ('debug', 'info', 'warn', 'error'))
);

CREATE INDEX idx_marketplace_sync_logs_job ON marketplace_sync_logs(job_id);
CREATE INDEX idx_marketplace_sync_logs_created ON marketplace_sync_logs(created_at DESC);

-- -----------------------------------------------------------------------------
-- Marketplace Product Mappings
-- Maps internal product IDs to external marketplace product IDs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_product_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES marketplace_connections(id) ON DELETE CASCADE,
    internal_product_id UUID NOT NULL,
    external_product_id VARCHAR(255) NOT NULL,
    external_sku VARCHAR(255),
    external_asin VARCHAR(20),
    external_data JSONB,
    sync_status VARCHAR(50) DEFAULT 'synced',
    last_synced_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_marketplace_product_mappings_connection ON marketplace_product_mappings(connection_id);
CREATE INDEX idx_marketplace_product_mappings_internal ON marketplace_product_mappings(internal_product_id);
CREATE INDEX idx_marketplace_product_mappings_external ON marketplace_product_mappings(external_product_id);
CREATE UNIQUE INDEX idx_marketplace_product_mappings_unique
    ON marketplace_product_mappings(connection_id, external_product_id);

-- -----------------------------------------------------------------------------
-- Marketplace Order Mappings
-- Maps internal order IDs to external marketplace order IDs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_order_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES marketplace_connections(id) ON DELETE CASCADE,
    internal_order_id UUID NOT NULL,
    external_order_id VARCHAR(255) NOT NULL,
    external_status VARCHAR(100),
    external_data JSONB,
    sync_status VARCHAR(50) DEFAULT 'synced',
    last_synced_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_marketplace_order_mappings_connection ON marketplace_order_mappings(connection_id);
CREATE INDEX idx_marketplace_order_mappings_internal ON marketplace_order_mappings(internal_order_id);
CREATE INDEX idx_marketplace_order_mappings_external ON marketplace_order_mappings(external_order_id);
CREATE UNIQUE INDEX idx_marketplace_order_mappings_unique
    ON marketplace_order_mappings(connection_id, external_order_id);

-- -----------------------------------------------------------------------------
-- Marketplace Inventory Mappings
-- Maps internal inventory to external marketplace inventory
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_inventory_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES marketplace_connections(id) ON DELETE CASCADE,
    internal_product_id UUID NOT NULL,
    external_sku VARCHAR(255) NOT NULL,
    external_quantity INTEGER DEFAULT 0,
    internal_quantity INTEGER DEFAULT 0,
    sync_status VARCHAR(50) DEFAULT 'synced',
    last_synced_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_marketplace_inventory_mappings_connection ON marketplace_inventory_mappings(connection_id);
CREATE INDEX idx_marketplace_inventory_mappings_product ON marketplace_inventory_mappings(internal_product_id);
CREATE UNIQUE INDEX idx_marketplace_inventory_mappings_unique
    ON marketplace_inventory_mappings(connection_id, external_sku);

-- -----------------------------------------------------------------------------
-- Marketplace Webhook Events
-- Stores incoming webhook events for processing
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID REFERENCES marketplace_connections(id) ON DELETE SET NULL,
    marketplace_type VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_id VARCHAR(255),
    payload JSONB NOT NULL,
    headers JSONB,
    status VARCHAR(50) DEFAULT 'pending',
    processed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_webhook_status CHECK (status IN ('pending', 'processing', 'processed', 'failed'))
);

CREATE INDEX idx_marketplace_webhook_events_connection ON marketplace_webhook_events(connection_id);
CREATE INDEX idx_marketplace_webhook_events_type ON marketplace_webhook_events(marketplace_type);
CREATE INDEX idx_marketplace_webhook_events_status ON marketplace_webhook_events(status);
CREATE INDEX idx_marketplace_webhook_events_created ON marketplace_webhook_events(created_at DESC);

-- -----------------------------------------------------------------------------
-- Updated At Trigger Function
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at triggers
CREATE TRIGGER update_marketplace_connections_updated_at
    BEFORE UPDATE ON marketplace_connections
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_credentials_updated_at
    BEFORE UPDATE ON marketplace_credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_sync_jobs_updated_at
    BEFORE UPDATE ON marketplace_sync_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_product_mappings_updated_at
    BEFORE UPDATE ON marketplace_product_mappings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_order_mappings_updated_at
    BEFORE UPDATE ON marketplace_order_mappings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_inventory_mappings_updated_at
    BEFORE UPDATE ON marketplace_inventory_mappings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
