-- =============================================================================
-- Marketplace Connector Service - HLD Services Migration
-- Migration: 003_hld_services
-- Adds: Event Ordering, Reconciliation, Inventory Mismatches, Order Imports
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Event Versions (Track entity versions for out-of-order event handling)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_event_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    version BIGINT NOT NULL DEFAULT 0,
    event_time TIMESTAMP WITH TIME ZONE NOT NULL,
    last_event_id VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT uq_event_version_entity UNIQUE (tenant_id, entity_type, entity_id)
);

CREATE INDEX idx_event_version_lookup ON marketplace_event_versions(tenant_id, entity_type, entity_id);
CREATE INDEX idx_event_version_time ON marketplace_event_versions(event_time DESC);

-- -----------------------------------------------------------------------------
-- Out of Order Events (Store events received out of order for later processing)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_out_of_order_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    event_id VARCHAR(255) NOT NULL,
    event_time TIMESTAMP WITH TIME ZONE NOT NULL,
    event_version BIGINT NOT NULL,
    event_payload BYTEA,
    received_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) DEFAULT 'PENDING',

    CONSTRAINT chk_ooo_event_status CHECK (status IN ('PENDING', 'PROCESSED', 'EXPIRED', 'FAILED'))
);

CREATE INDEX idx_ooo_events_tenant ON marketplace_out_of_order_events(tenant_id);
CREATE INDEX idx_ooo_events_entity ON marketplace_out_of_order_events(tenant_id, entity_type, entity_id);
CREATE INDEX idx_ooo_events_status ON marketplace_out_of_order_events(status) WHERE status = 'PENDING';
CREATE INDEX idx_ooo_events_time ON marketplace_out_of_order_events(event_time ASC);

-- -----------------------------------------------------------------------------
-- Reconciliation Jobs (Track reconciliation operations)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_reconciliation_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255) NOT NULL,
    connection_id UUID NOT NULL REFERENCES marketplace_connections(id) ON DELETE CASCADE,

    -- Job configuration
    reconciliation_type VARCHAR(50) NOT NULL,
    comparison_window_start TIMESTAMP WITH TIME ZONE,
    comparison_window_end TIMESTAMP WITH TIME ZONE,

    -- Status
    status VARCHAR(50) DEFAULT 'PENDING',

    -- Results
    total_compared INTEGER DEFAULT 0,
    discrepancies_found INTEGER DEFAULT 0,
    discrepancies_resolved INTEGER DEFAULT 0,
    discrepancies_pending INTEGER DEFAULT 0,

    -- Timing
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_recon_type CHECK (reconciliation_type IN ('INVENTORY', 'PRODUCTS', 'ORDERS', 'FULL')),
    CONSTRAINT chk_recon_status CHECK (status IN ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED'))
);

CREATE INDEX idx_recon_jobs_tenant ON marketplace_reconciliation_jobs(tenant_id);
CREATE INDEX idx_recon_jobs_connection ON marketplace_reconciliation_jobs(connection_id);
CREATE INDEX idx_recon_jobs_status ON marketplace_reconciliation_jobs(status);
CREATE INDEX idx_recon_jobs_created ON marketplace_reconciliation_jobs(created_at DESC);

-- -----------------------------------------------------------------------------
-- Reconciliation Discrepancies (Individual discrepancies found)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_reconciliation_discrepancies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reconciliation_job_id UUID NOT NULL REFERENCES marketplace_reconciliation_jobs(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,

    -- Entity reference
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    external_id VARCHAR(255),

    -- Discrepancy details
    discrepancy_type VARCHAR(50) NOT NULL,
    field_name VARCHAR(255),
    internal_value TEXT,
    external_value TEXT,
    severity VARCHAR(20) DEFAULT 'MEDIUM',

    -- Resolution
    status VARCHAR(50) DEFAULT 'OPEN',
    resolution TEXT,
    resolved_at TIMESTAMP WITH TIME ZONE,
    resolved_by VARCHAR(255),
    auto_resolved BOOLEAN DEFAULT false,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_discrepancy_type CHECK (discrepancy_type IN ('MISSING_INTERNAL', 'MISSING_EXTERNAL', 'VALUE_MISMATCH', 'QUANTITY_MISMATCH', 'STATUS_MISMATCH')),
    CONSTRAINT chk_discrepancy_severity CHECK (severity IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    CONSTRAINT chk_discrepancy_status CHECK (status IN ('OPEN', 'ACKNOWLEDGED', 'RESOLVED', 'IGNORED'))
);

CREATE INDEX idx_discrepancies_job ON marketplace_reconciliation_discrepancies(reconciliation_job_id);
CREATE INDEX idx_discrepancies_tenant ON marketplace_reconciliation_discrepancies(tenant_id);
CREATE INDEX idx_discrepancies_status ON marketplace_reconciliation_discrepancies(status);
CREATE INDEX idx_discrepancies_severity ON marketplace_reconciliation_discrepancies(severity) WHERE status = 'OPEN';

-- -----------------------------------------------------------------------------
-- Inventory Mismatches (Track inventory discrepancies between internal/external)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_inventory_mismatches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255) NOT NULL,
    connection_id UUID NOT NULL REFERENCES marketplace_connections(id) ON DELETE CASCADE,
    offer_id UUID NOT NULL REFERENCES marketplace_offers(id) ON DELETE CASCADE,
    inventory_id UUID NOT NULL,
    external_sku VARCHAR(255),

    -- Quantities
    internal_quantity INTEGER NOT NULL,
    external_quantity INTEGER NOT NULL,
    difference INTEGER NOT NULL,
    percentage_diff DECIMAL(10, 2),

    -- Status
    severity VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'OPEN',
    resolution TEXT,
    resolved_at TIMESTAMP WITH TIME ZONE,
    resolved_by VARCHAR(255),

    detected_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_mismatch_severity CHECK (severity IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    CONSTRAINT chk_mismatch_status CHECK (status IN ('OPEN', 'ACKNOWLEDGED', 'RESOLVED', 'IGNORED', 'AUTO_FIXED'))
);

CREATE INDEX idx_mismatches_tenant ON marketplace_inventory_mismatches(tenant_id);
CREATE INDEX idx_mismatches_connection ON marketplace_inventory_mismatches(connection_id);
CREATE INDEX idx_mismatches_offer ON marketplace_inventory_mismatches(offer_id);
CREATE INDEX idx_mismatches_status ON marketplace_inventory_mismatches(status) WHERE status = 'OPEN';
CREATE INDEX idx_mismatches_severity ON marketplace_inventory_mismatches(severity, detected_at DESC);
CREATE INDEX idx_mismatches_inventory ON marketplace_inventory_mismatches(inventory_id, connection_id);

-- -----------------------------------------------------------------------------
-- Imported Orders (Orders imported from marketplaces with PII encryption)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_imported_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255) NOT NULL,
    connection_id UUID NOT NULL REFERENCES marketplace_connections(id) ON DELETE CASCADE,
    external_order_id VARCHAR(255) NOT NULL,
    marketplace_type VARCHAR(50) NOT NULL,
    order_status VARCHAR(50) NOT NULL,
    resolution_status VARCHAR(50) DEFAULT 'PENDING',

    -- Encrypted PII fields (AES-256-GCM)
    customer_pii_ciphertext BYTEA,
    customer_pii_nonce BYTEA,
    customer_pii_key_version INTEGER DEFAULT 1,

    -- Non-PII order details
    currency VARCHAR(3) DEFAULT 'USD',
    total_amount DECIMAL(12, 2),
    subtotal_amount DECIMAL(12, 2),
    tax_amount DECIMAL(12, 2),
    shipping_amount DECIMAL(12, 2),
    discount_amount DECIMAL(12, 2),

    -- Timestamps
    order_date TIMESTAMP WITH TIME ZONE,
    imported_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_synced_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT uq_imported_order_external UNIQUE (connection_id, external_order_id),
    CONSTRAINT chk_order_status CHECK (order_status IN ('PENDING', 'PROCESSING', 'SHIPPED', 'DELIVERED', 'CANCELLED', 'REFUNDED')),
    CONSTRAINT chk_resolution_status CHECK (resolution_status IN ('PENDING', 'PARTIAL', 'RESOLVED', 'UNMAPPED'))
);

CREATE INDEX idx_imported_orders_tenant ON marketplace_imported_orders(tenant_id);
CREATE INDEX idx_imported_orders_vendor ON marketplace_imported_orders(vendor_id);
CREATE INDEX idx_imported_orders_connection ON marketplace_imported_orders(connection_id);
CREATE INDEX idx_imported_orders_status ON marketplace_imported_orders(order_status);
CREATE INDEX idx_imported_orders_resolution ON marketplace_imported_orders(resolution_status);
CREATE INDEX idx_imported_orders_date ON marketplace_imported_orders(order_date DESC);

-- -----------------------------------------------------------------------------
-- Imported Order Lines (Line items for imported orders)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marketplace_imported_order_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES marketplace_imported_orders(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    external_line_id VARCHAR(255),
    external_sku VARCHAR(255),
    external_name VARCHAR(500),

    -- Internal mapping
    internal_variant_id UUID,
    internal_offer_id UUID,
    mapping_status VARCHAR(50) DEFAULT 'UNRESOLVED',

    -- Line details
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(12, 2),
    total_price DECIMAL(12, 2),
    tax_amount DECIMAL(12, 2),

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_line_mapping_status CHECK (mapping_status IN ('RESOLVED', 'UNRESOLVED', 'MULTIPLE_MATCHES', 'NO_MATCH'))
);

CREATE INDEX idx_order_lines_order ON marketplace_imported_order_lines(order_id);
CREATE INDEX idx_order_lines_tenant ON marketplace_imported_order_lines(tenant_id);
CREATE INDEX idx_order_lines_sku ON marketplace_imported_order_lines(external_sku);
CREATE INDEX idx_order_lines_mapping ON marketplace_imported_order_lines(mapping_status);
CREATE INDEX idx_order_lines_variant ON marketplace_imported_order_lines(internal_variant_id) WHERE internal_variant_id IS NOT NULL;

-- -----------------------------------------------------------------------------
-- Enable RLS on new tables
-- -----------------------------------------------------------------------------
ALTER TABLE marketplace_event_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_out_of_order_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_reconciliation_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_reconciliation_discrepancies ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_inventory_mismatches ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_imported_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE marketplace_imported_order_lines ENABLE ROW LEVEL SECURITY;

-- RLS Policies
CREATE POLICY tenant_isolation_event_versions ON marketplace_event_versions
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_ooo_events ON marketplace_out_of_order_events
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_recon_jobs ON marketplace_reconciliation_jobs
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_discrepancies ON marketplace_reconciliation_discrepancies
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_mismatches ON marketplace_inventory_mismatches
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_imported_orders ON marketplace_imported_orders
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY tenant_isolation_order_lines ON marketplace_imported_order_lines
    FOR ALL USING (tenant_id = current_setting('app.tenant_id', true));

-- -----------------------------------------------------------------------------
-- Updated At Triggers
-- -----------------------------------------------------------------------------
CREATE TRIGGER update_marketplace_event_versions_updated_at
    BEFORE UPDATE ON marketplace_event_versions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_recon_jobs_updated_at
    BEFORE UPDATE ON marketplace_reconciliation_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_discrepancies_updated_at
    BEFORE UPDATE ON marketplace_reconciliation_discrepancies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_mismatches_updated_at
    BEFORE UPDATE ON marketplace_inventory_mismatches
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_imported_orders_updated_at
    BEFORE UPDATE ON marketplace_imported_orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_marketplace_order_lines_updated_at
    BEFORE UPDATE ON marketplace_imported_order_lines
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- -----------------------------------------------------------------------------
-- Comments for documentation
-- -----------------------------------------------------------------------------
COMMENT ON TABLE marketplace_event_versions IS 'Tracks entity versions for handling out-of-order webhook events';
COMMENT ON TABLE marketplace_out_of_order_events IS 'Stores events received out of order for deferred processing';
COMMENT ON TABLE marketplace_reconciliation_jobs IS 'Tracks reconciliation operations between internal and external systems';
COMMENT ON TABLE marketplace_reconciliation_discrepancies IS 'Individual discrepancies found during reconciliation';
COMMENT ON TABLE marketplace_inventory_mismatches IS 'Detected inventory quantity mismatches with severity levels';
COMMENT ON TABLE marketplace_imported_orders IS 'Orders imported from marketplaces with encrypted PII';
COMMENT ON TABLE marketplace_imported_order_lines IS 'Line items for imported orders with product mapping status';
