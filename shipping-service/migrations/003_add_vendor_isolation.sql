-- Migration: Add vendor isolation to shipping tables for marketplace support

-- Add vendor_id to shipments
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Add tenant_id and vendor_id to shipment_tracking for efficient queries
ALTER TABLE shipment_tracking ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);
ALTER TABLE shipment_tracking ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Backfill shipment_tracking from parent shipments table
UPDATE shipment_tracking st
SET tenant_id = s.tenant_id, vendor_id = s.vendor_id
FROM shipments s
WHERE st.shipment_id = s.id AND (st.tenant_id IS NULL OR st.vendor_id IS NULL);

-- Create indexes for vendor isolation
CREATE INDEX IF NOT EXISTS idx_shipments_vendor ON shipments(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_shipments_tenant_vendor ON shipments(tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_shipment_tracking_tenant ON shipment_tracking(tenant_id);
CREATE INDEX IF NOT EXISTS idx_shipment_tracking_vendor ON shipment_tracking(vendor_id) WHERE vendor_id IS NOT NULL;

-- Comments
COMMENT ON COLUMN shipments.vendor_id IS 'Vendor ID for marketplace. Tracks which vendor fulfilled the shipment.';
COMMENT ON COLUMN shipment_tracking.tenant_id IS 'Denormalized tenant_id for efficient queries.';
COMMENT ON COLUMN shipment_tracking.vendor_id IS 'Denormalized vendor_id for marketplace filtering.';
