-- =============================================================================
-- Marketplace Connector Service - HLD Compliance Migration
-- Migration: 002_hld_compliance
-- Adds: RLS, Catalog, Offers, Inventory Ledger, Audit Logs, API Keys, Partitioning
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Enable Row Level Security Extensions
-- -----------------------------------------------------------------------------

-- Add vendor_id to marketplace_connections if not exists (already has it)
-- Add display_name column rename if needed
DO $$
BEGIN
    -- Only rename if 'name' column exists and 'display_name' doesn't
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'marketplace_connections' AND column_name = 'name')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'marketplace_connections' AND column_name = 'display_name') THEN
        ALTER TABLE marketplace_connections RENAME COLUMN name TO display_name;
    END IF;

    -- Add missing columns only if they don't exist
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'marketplace_connections' AND column_name = 'external_store_id') THEN
        ALTER TABLE marketplace_connections ADD COLUMN external_store_id VARCHAR(255);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'marketplace_connections' AND column_name = 'external_store_url') THEN
        ALTER TABLE marketplace_connections ADD COLUMN external_store_url VARCHAR(500);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'marketplace_connections' AND column_name = 'is_enabled') THEN
        ALTER TABLE marketplace_connections ADD COLUMN is_enabled BOOLEAN DEFAULT true;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'marketplace_connections' AND column_name = 'error_count') THEN
        ALTER TABLE marketplace_connections ADD COLUMN error_count INTEGER DEFAULT 0;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'marketplace_connections' AND column_name = 'created_by') THEN
        ALTER TABLE marketplace_connections ADD COLUMN created_by VARCHAR(255);
    END IF;
END $$;

-- Add tenant_id to tables that might be missing it
ALTER TABLE marketplace_credentials
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

UPDATE marketplace_credentials mc
SET tenant_id = (SELECT tenant_id FROM marketplace_connections c WHERE c.id = mc.connection_id)
WHERE tenant_id IS NULL;

-- -----------------------------------------------------------------------------
-- API Keys Table (for tenant API key management)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,

    -- Key identification
    key_prefix VARCHAR(12) NOT NULL,  -- First 12 chars for identification
    key_hash VARCHAR(64) NOT NULL,     -- SHA-256 hash of full key

    -- Key metadata
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Permissions
    scopes JSONB DEFAULT '["read", "write"]',

    -- Lifecycle
    is_active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,

    -- Rotation support
    previous_key_hash VARCHAR(64),
    rotation_grace_period_ends TIMESTAMP WITH TIME ZONE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255),

    CONSTRAINT uq_api_key_prefix UNIQUE (key_prefix)
);

CREATE INDEX idx_api_keys_tenant ON marketplace_api_keys(tenant_id);
CREATE INDEX idx_api_keys_hash ON marketplace_api_keys(key_hash);
CREATE INDEX idx_api_keys_active ON marketplace_api_keys(is_active) WHERE is_active = true;

-- -----------------------------------------------------------------------------
-- Catalog Items (Unified product catalog per tenant)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_catalog_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,

    -- Product identification
    name VARCHAR(500) NOT NULL,
    description TEXT,
    brand VARCHAR(255),

    -- Universal identifiers (for matching)
    gtin VARCHAR(14),        -- Global Trade Item Number (UPC/EAN)
    upc VARCHAR(12),         -- Universal Product Code
    ean VARCHAR(13),         -- European Article Number
    isbn VARCHAR(13),        -- International Standard Book Number
    mpn VARCHAR(100),        -- Manufacturer Part Number

    -- Categorization
    category_id UUID,
    category_path VARCHAR(1000),

    -- Status
    status VARCHAR(50) DEFAULT 'ACTIVE',

    -- Metadata
    attributes JSONB DEFAULT '{}',
    images JSONB DEFAULT '[]',

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_catalog_status CHECK (status IN ('ACTIVE', 'INACTIVE', 'DISCONTINUED'))
);

CREATE INDEX idx_catalog_items_tenant ON marketplace_catalog_items(tenant_id);
CREATE INDEX idx_catalog_items_gtin ON marketplace_catalog_items(gtin) WHERE gtin IS NOT NULL;
CREATE INDEX idx_catalog_items_upc ON marketplace_catalog_items(upc) WHERE upc IS NOT NULL;
CREATE INDEX idx_catalog_items_ean ON marketplace_catalog_items(ean) WHERE ean IS NOT NULL;
CREATE INDEX idx_catalog_items_brand ON marketplace_catalog_items(brand);

-- -----------------------------------------------------------------------------
-- Catalog Variants (SKU-level records)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_catalog_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    catalog_item_id UUID NOT NULL REFERENCES marketplace_catalog_items(id) ON DELETE CASCADE,

    -- Variant identification
    sku VARCHAR(255) NOT NULL,
    name VARCHAR(500),

    -- Identifiers
    gtin VARCHAR(14),
    upc VARCHAR(12),
    ean VARCHAR(13),
    barcode VARCHAR(50),
    barcode_type VARCHAR(20),

    -- Variant attributes
    options JSONB DEFAULT '{}',  -- e.g., {"size": "M", "color": "Blue"}

    -- Pricing (base price, vendors set their own via offers)
    cost_price DECIMAL(12, 2),

    -- Physical attributes
    weight DECIMAL(10, 3),
    weight_unit VARCHAR(10) DEFAULT 'kg',
    length DECIMAL(10, 2),
    width DECIMAL(10, 2),
    height DECIMAL(10, 2),
    dimension_unit VARCHAR(10) DEFAULT 'cm',

    -- Status
    status VARCHAR(50) DEFAULT 'ACTIVE',

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT uq_catalog_variant_sku UNIQUE (tenant_id, sku),
    CONSTRAINT chk_variant_status CHECK (status IN ('ACTIVE', 'INACTIVE', 'DISCONTINUED'))
);

CREATE INDEX idx_catalog_variants_tenant ON marketplace_catalog_variants(tenant_id);
CREATE INDEX idx_catalog_variants_item ON marketplace_catalog_variants(catalog_item_id);
CREATE INDEX idx_catalog_variants_sku ON marketplace_catalog_variants(tenant_id, sku);
CREATE INDEX idx_catalog_variants_gtin ON marketplace_catalog_variants(gtin) WHERE gtin IS NOT NULL;

-- -----------------------------------------------------------------------------
-- Offers (Vendor-specific pricing and availability)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_offers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255) NOT NULL,
    catalog_variant_id UUID NOT NULL REFERENCES marketplace_catalog_variants(id) ON DELETE CASCADE,
    connection_id UUID REFERENCES marketplace_connections(id) ON DELETE SET NULL,

    -- Pricing
    price DECIMAL(12, 2) NOT NULL,
    compare_at_price DECIMAL(12, 2),
    currency VARCHAR(3) DEFAULT 'USD',

    -- Availability
    is_available BOOLEAN DEFAULT true,
    available_quantity INTEGER DEFAULT 0,

    -- Fulfillment
    fulfillment_type VARCHAR(50) DEFAULT 'VENDOR',  -- VENDOR, FBA, FBM, DROPSHIP
    lead_time_days INTEGER DEFAULT 0,

    -- External mapping
    external_offer_id VARCHAR(255),
    external_listing_id VARCHAR(255),

    -- Status
    status VARCHAR(50) DEFAULT 'ACTIVE',

    -- Metadata
    metadata JSONB DEFAULT '{}',

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT uq_offer_vendor_variant UNIQUE (tenant_id, vendor_id, catalog_variant_id),
    CONSTRAINT chk_offer_status CHECK (status IN ('ACTIVE', 'INACTIVE', 'SUSPENDED', 'OUT_OF_STOCK')),
    CONSTRAINT chk_fulfillment_type CHECK (fulfillment_type IN ('VENDOR', 'FBA', 'FBM', 'DROPSHIP', 'MARKETPLACE'))
);

CREATE INDEX idx_offers_tenant ON marketplace_offers(tenant_id);
CREATE INDEX idx_offers_vendor ON marketplace_offers(vendor_id);
CREATE INDEX idx_offers_variant ON marketplace_offers(catalog_variant_id);
CREATE INDEX idx_offers_connection ON marketplace_offers(connection_id);
CREATE INDEX idx_offers_status ON marketplace_offers(status);

-- -----------------------------------------------------------------------------
-- Inventory Current (Current inventory levels)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_inventory_current (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255) NOT NULL,
    offer_id UUID NOT NULL REFERENCES marketplace_offers(id) ON DELETE CASCADE,

    -- Location
    location_id VARCHAR(255),
    location_name VARCHAR(255),
    location_type VARCHAR(50) DEFAULT 'WAREHOUSE',  -- WAREHOUSE, STORE, FBA, FBM

    -- Quantities
    quantity_on_hand INTEGER DEFAULT 0,
    quantity_reserved INTEGER DEFAULT 0,
    quantity_available INTEGER GENERATED ALWAYS AS (quantity_on_hand - quantity_reserved) STORED,
    quantity_incoming INTEGER DEFAULT 0,

    -- Thresholds
    low_stock_threshold INTEGER DEFAULT 10,
    reorder_point INTEGER DEFAULT 20,

    -- External sync
    external_location_id VARCHAR(255),
    last_synced_at TIMESTAMP WITH TIME ZONE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT uq_inventory_offer_location UNIQUE (offer_id, location_id),
    CONSTRAINT chk_location_type CHECK (location_type IN ('WAREHOUSE', 'STORE', 'FBA', 'FBM', 'DROPSHIP', 'VIRTUAL'))
);

CREATE INDEX idx_inventory_current_tenant ON marketplace_inventory_current(tenant_id);
CREATE INDEX idx_inventory_current_vendor ON marketplace_inventory_current(vendor_id);
CREATE INDEX idx_inventory_current_offer ON marketplace_inventory_current(offer_id);
CREATE INDEX idx_inventory_current_low_stock ON marketplace_inventory_current(quantity_available)
    WHERE quantity_available <= low_stock_threshold;

-- -----------------------------------------------------------------------------
-- Inventory Ledger (Partitioned - audit trail for inventory changes)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_inventory_ledgers (
    id UUID DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255) NOT NULL,
    offer_id UUID NOT NULL,
    inventory_id UUID NOT NULL,

    -- Transaction details
    transaction_type VARCHAR(50) NOT NULL,
    quantity_change INTEGER NOT NULL,
    quantity_before INTEGER NOT NULL,
    quantity_after INTEGER NOT NULL,

    -- Reference
    reference_type VARCHAR(50),  -- ORDER, SYNC, ADJUSTMENT, RESERVATION, TRANSFER
    reference_id VARCHAR(255),

    -- Source
    source VARCHAR(50) NOT NULL,  -- MARKETPLACE, MANUAL, SYNC, WEBHOOK
    source_connection_id UUID,

    -- Metadata
    notes TEXT,
    metadata JSONB DEFAULT '{}',

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255),

    PRIMARY KEY (id, created_at),
    CONSTRAINT chk_transaction_type CHECK (transaction_type IN ('RECEIVE', 'SELL', 'ADJUST', 'RESERVE', 'RELEASE', 'TRANSFER_IN', 'TRANSFER_OUT', 'SYNC')),
    CONSTRAINT chk_reference_type CHECK (reference_type IN ('ORDER', 'SYNC', 'ADJUSTMENT', 'RESERVATION', 'TRANSFER', 'RETURN', 'DAMAGE'))
) PARTITION BY RANGE (created_at);

-- Create partitions for inventory ledgers (monthly)
CREATE TABLE IF NOT EXISTS marketplace_inventory_ledgers_2026_01
    PARTITION OF marketplace_inventory_ledgers
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

CREATE TABLE IF NOT EXISTS marketplace_inventory_ledgers_2026_02
    PARTITION OF marketplace_inventory_ledgers
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE TABLE IF NOT EXISTS marketplace_inventory_ledgers_2026_03
    PARTITION OF marketplace_inventory_ledgers
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE IF NOT EXISTS marketplace_inventory_ledgers_default
    PARTITION OF marketplace_inventory_ledgers
    DEFAULT;

CREATE INDEX idx_inventory_ledgers_tenant ON marketplace_inventory_ledgers(tenant_id);
CREATE INDEX idx_inventory_ledgers_offer ON marketplace_inventory_ledgers(offer_id);
CREATE INDEX idx_inventory_ledgers_created ON marketplace_inventory_ledgers(created_at DESC);

-- -----------------------------------------------------------------------------
-- External Mappings (Unified mapping table)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_external_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255) NOT NULL,
    connection_id UUID NOT NULL REFERENCES marketplace_connections(id) ON DELETE CASCADE,

    -- Entity type
    entity_type VARCHAR(50) NOT NULL,  -- PRODUCT, VARIANT, ORDER, CUSTOMER, CATEGORY

    -- Internal reference
    internal_id UUID NOT NULL,

    -- External reference
    external_id VARCHAR(255) NOT NULL,
    external_sku VARCHAR(255),
    external_asin VARCHAR(20),
    external_parent_id VARCHAR(255),  -- For variants pointing to parent product

    -- Match quality
    match_type VARCHAR(50) DEFAULT 'EXACT',  -- EXACT, GTIN, FUZZY, MANUAL
    match_confidence DECIMAL(3, 2) DEFAULT 1.0,

    -- Sync status
    sync_status VARCHAR(50) DEFAULT 'SYNCED',
    last_synced_at TIMESTAMP WITH TIME ZONE,
    sync_error TEXT,

    -- Version tracking
    internal_version INTEGER DEFAULT 1,
    external_version VARCHAR(100),

    -- External data snapshot
    external_data JSONB,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT uq_external_mapping UNIQUE (connection_id, entity_type, external_id),
    CONSTRAINT chk_entity_type CHECK (entity_type IN ('PRODUCT', 'VARIANT', 'ORDER', 'ORDER_LINE', 'CUSTOMER', 'CATEGORY', 'LOCATION')),
    CONSTRAINT chk_match_type CHECK (match_type IN ('EXACT', 'GTIN', 'UPC', 'EAN', 'SKU', 'FUZZY', 'MANUAL')),
    CONSTRAINT chk_mapping_sync_status CHECK (sync_status IN ('SYNCED', 'PENDING', 'ERROR', 'CONFLICT', 'DELETED'))
);

CREATE INDEX idx_external_mappings_tenant ON marketplace_external_mappings(tenant_id);
CREATE INDEX idx_external_mappings_connection ON marketplace_external_mappings(connection_id);
CREATE INDEX idx_external_mappings_entity ON marketplace_external_mappings(entity_type);
CREATE INDEX idx_external_mappings_internal ON marketplace_external_mappings(internal_id);
CREATE INDEX idx_external_mappings_external ON marketplace_external_mappings(external_id);
CREATE INDEX idx_external_mappings_status ON marketplace_external_mappings(sync_status);

-- -----------------------------------------------------------------------------
-- Raw Snapshots (Partitioned - stores raw API responses)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_raw_snapshots (
    id UUID DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    connection_id UUID NOT NULL,

    -- Entity reference
    entity_type VARCHAR(50) NOT NULL,
    external_id VARCHAR(255) NOT NULL,

    -- Snapshot data
    raw_data JSONB NOT NULL,
    data_hash VARCHAR(64),  -- SHA-256 for change detection

    -- Source
    source_endpoint VARCHAR(500),
    source_sync_job_id UUID,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create partitions for raw snapshots (monthly)
CREATE TABLE IF NOT EXISTS marketplace_raw_snapshots_2026_01
    PARTITION OF marketplace_raw_snapshots
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

CREATE TABLE IF NOT EXISTS marketplace_raw_snapshots_2026_02
    PARTITION OF marketplace_raw_snapshots
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE TABLE IF NOT EXISTS marketplace_raw_snapshots_2026_03
    PARTITION OF marketplace_raw_snapshots
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE IF NOT EXISTS marketplace_raw_snapshots_default
    PARTITION OF marketplace_raw_snapshots
    DEFAULT;

CREATE INDEX idx_raw_snapshots_tenant ON marketplace_raw_snapshots(tenant_id);
CREATE INDEX idx_raw_snapshots_connection ON marketplace_raw_snapshots(connection_id);
CREATE INDEX idx_raw_snapshots_entity ON marketplace_raw_snapshots(entity_type, external_id);
CREATE INDEX idx_raw_snapshots_hash ON marketplace_raw_snapshots(data_hash);

-- -----------------------------------------------------------------------------
-- Audit Logs (For PII access and sensitive operations)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,

    -- Actor
    actor_type VARCHAR(50) NOT NULL,  -- USER, SYSTEM, API_KEY, SERVICE
    actor_id VARCHAR(255) NOT NULL,
    actor_ip VARCHAR(45),

    -- Action
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255),

    -- Details
    old_value JSONB,
    new_value JSONB,
    metadata JSONB DEFAULT '{}',

    -- PII tracking
    pii_accessed BOOLEAN DEFAULT false,
    pii_fields TEXT[],

    -- Request context
    request_id VARCHAR(255),
    session_id VARCHAR(255),

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_tenant ON marketplace_audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_actor ON marketplace_audit_logs(actor_id);
CREATE INDEX idx_audit_logs_action ON marketplace_audit_logs(action);
CREATE INDEX idx_audit_logs_resource ON marketplace_audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_logs_created ON marketplace_audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_pii ON marketplace_audit_logs(pii_accessed) WHERE pii_accessed = true;

-- -----------------------------------------------------------------------------
-- PII Encryption Keys (Per-tenant encryption key references)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_encryption_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,

    -- Key identification
    key_id VARCHAR(255) NOT NULL,  -- GCP KMS key reference
    key_version INTEGER NOT NULL DEFAULT 1,

    -- Key type
    key_type VARCHAR(50) NOT NULL DEFAULT 'DATA_ENCRYPTION',

    -- Status
    status VARCHAR(50) DEFAULT 'ACTIVE',

    -- Rotation
    rotated_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT uq_encryption_key_tenant_type UNIQUE (tenant_id, key_type),
    CONSTRAINT chk_key_type CHECK (key_type IN ('DATA_ENCRYPTION', 'PII_ENCRYPTION', 'API_KEY_ENCRYPTION')),
    CONSTRAINT chk_key_status CHECK (status IN ('ACTIVE', 'ROTATING', 'DEPRECATED', 'DESTROYED'))
);

CREATE INDEX idx_encryption_keys_tenant ON marketplace_encryption_keys(tenant_id);
CREATE INDEX idx_encryption_keys_status ON marketplace_encryption_keys(status);

-- -----------------------------------------------------------------------------
-- Update sync_jobs for HLD job types
-- -----------------------------------------------------------------------------
ALTER TABLE marketplace_sync_jobs
    ADD COLUMN IF NOT EXISTS job_type VARCHAR(50) DEFAULT 'FULL_IMPORT',
    ADD COLUMN IF NOT EXISTS cursor_position JSONB,
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255),
    ADD COLUMN IF NOT EXISTS parent_job_id UUID,
    ADD COLUMN IF NOT EXISTS priority INTEGER DEFAULT 5;

-- Add check constraint for new job types
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_job_type'
    ) THEN
        ALTER TABLE marketplace_sync_jobs
            ADD CONSTRAINT chk_job_type
            CHECK (job_type IN ('FULL_IMPORT', 'DELTA_SYNC', 'FETCH_ENTITY', 'RECONCILE', 'REPAIR'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_sync_jobs_idempotency ON marketplace_sync_jobs(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_priority ON marketplace_sync_jobs(priority, created_at);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_parent ON marketplace_sync_jobs(parent_job_id);

-- -----------------------------------------------------------------------------
-- Row Level Security Policies
-- -----------------------------------------------------------------------------

-- Enable RLS on all tenant tables
ALTER TABLE marketplace_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_sync_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_sync_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_product_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_order_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_inventory_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_webhook_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_catalog_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_catalog_variants ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_offers ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_inventory_current ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_inventory_ledgers ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_external_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_raw_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_encryption_keys ENABLE ROW LEVEL SECURITY;

-- Create RLS policies for tenant isolation
-- Policy: Users can only access their own tenant's data

CREATE POLICY tenant_isolation_connections ON marketplace_connections
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_credentials ON marketplace_credentials
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_sync_jobs ON marketplace_sync_jobs
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_sync_logs ON marketplace_sync_logs
    FOR ALL USING (
        sync_job_id IN (
            SELECT id FROM marketplace_sync_jobs
            WHERE tenant_id = current_setting('app.tenant_id', true)
        )
    );

CREATE POLICY tenant_isolation_product_mappings ON marketplace_product_mappings
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_order_mappings ON marketplace_order_mappings
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_inventory_mappings ON marketplace_inventory_mappings
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_webhook_events ON marketplace_webhook_events
    FOR ALL USING (
        tenant_id = current_setting('app.tenant_id', true)
        OR tenant_id IS NULL  -- Allow unassigned webhooks for processing
    );

CREATE POLICY tenant_isolation_api_keys ON marketplace_api_keys
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_catalog_items ON marketplace_catalog_items
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_catalog_variants ON marketplace_catalog_variants
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_offers ON marketplace_offers
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_inventory_current ON marketplace_inventory_current
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_inventory_ledgers ON marketplace_inventory_ledgers
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_external_mappings ON marketplace_external_mappings
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_raw_snapshots ON marketplace_raw_snapshots
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_audit_logs ON marketplace_audit_logs
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_encryption_keys ON marketplace_encryption_keys
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

-- -----------------------------------------------------------------------------
-- Updated At Triggers for new tables
-- -----------------------------------------------------------------------------

CREATE TRIGGER update_marketplace_api_keys_updated_at
    BEFORE UPDATE ON marketplace_api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_catalog_items_updated_at
    BEFORE UPDATE ON marketplace_catalog_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_catalog_variants_updated_at
    BEFORE UPDATE ON marketplace_catalog_variants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_offers_updated_at
    BEFORE UPDATE ON marketplace_offers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_inventory_current_updated_at
    BEFORE UPDATE ON marketplace_inventory_current
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_external_mappings_updated_at
    BEFORE UPDATE ON marketplace_external_mappings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_encryption_keys_updated_at
    BEFORE UPDATE ON marketplace_encryption_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- -----------------------------------------------------------------------------
-- Comments for documentation
-- -----------------------------------------------------------------------------
COMMENT ON TABLE marketplace_api_keys IS 'Stores hashed API keys for tenant authentication';
COMMENT ON TABLE marketplace_catalog_items IS 'Unified product catalog per tenant with GTIN/UPC/EAN for matching';
COMMENT ON TABLE marketplace_catalog_variants IS 'SKU-level variants linked to catalog items';
COMMENT ON TABLE marketplace_offers IS 'Vendor-specific pricing and availability for catalog variants';
COMMENT ON TABLE marketplace_inventory_current IS 'Current inventory levels per offer and location';
COMMENT ON TABLE marketplace_inventory_ledgers IS 'Partitioned audit trail for inventory changes';
COMMENT ON TABLE marketplace_external_mappings IS 'Unified mapping between internal and external entity IDs';
COMMENT ON TABLE marketplace_raw_snapshots IS 'Partitioned storage for raw API responses';
COMMENT ON TABLE marketplace_audit_logs IS 'Security audit trail for sensitive operations and PII access';
COMMENT ON TABLE marketplace_encryption_keys IS 'Per-tenant encryption key references for PII protection';
