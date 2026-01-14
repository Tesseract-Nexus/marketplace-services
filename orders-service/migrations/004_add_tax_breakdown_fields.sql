-- Migration: Add tax breakdown fields for India GST, EU VAT, and global tax support
-- This migration adds comprehensive tax tracking to orders and order items

-- Add tax breakdown to orders table
ALTER TABLE orders
ADD COLUMN IF NOT EXISTS tax_breakdown JSONB,
ADD COLUMN IF NOT EXISTS cgst DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS sgst DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS igst DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS utgst DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS gst_cess DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS is_interstate BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS customer_gstin VARCHAR(15),
ADD COLUMN IF NOT EXISTS vat_amount DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS is_reverse_charge BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS customer_vat_number VARCHAR(50);

-- Add tax fields to order_items table
ALTER TABLE order_items
ADD COLUMN IF NOT EXISTS tax_amount DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS tax_rate DECIMAL(5, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS hsn_code VARCHAR(10),
ADD COLUMN IF NOT EXISTS sac_code VARCHAR(10),
ADD COLUMN IF NOT EXISTS gst_slab DECIMAL(5, 2),
ADD COLUMN IF NOT EXISTS cgst_amount DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS sgst_amount DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS igst_amount DECIMAL(10, 2) DEFAULT 0;

-- Add state/country code to order_shipping for tax determination
ALTER TABLE order_shipping
ADD COLUMN IF NOT EXISTS state_code VARCHAR(10),
ADD COLUMN IF NOT EXISTS country_code VARCHAR(2);

-- Add indexes for tax-related queries
CREATE INDEX IF NOT EXISTS idx_orders_customer_gstin ON orders(customer_gstin) WHERE customer_gstin IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_orders_customer_vat ON orders(customer_vat_number) WHERE customer_vat_number IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_orders_interstate ON orders(is_interstate) WHERE is_interstate = true;
CREATE INDEX IF NOT EXISTS idx_order_items_hsn ON order_items(hsn_code) WHERE hsn_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_items_sac ON order_items(sac_code) WHERE sac_code IS NOT NULL;

-- Comments
COMMENT ON COLUMN orders.tax_breakdown IS 'Detailed tax breakdown as JSONB (jurisdiction, rate, amount per tax type)';
COMMENT ON COLUMN orders.cgst IS 'India Central GST amount';
COMMENT ON COLUMN orders.sgst IS 'India State GST amount';
COMMENT ON COLUMN orders.igst IS 'India Integrated GST (interstate) amount';
COMMENT ON COLUMN orders.utgst IS 'India Union Territory GST amount';
COMMENT ON COLUMN orders.gst_cess IS 'India GST Cess (luxury goods) amount';
COMMENT ON COLUMN orders.is_interstate IS 'True if order is interstate (different seller/buyer state)';
COMMENT ON COLUMN orders.customer_gstin IS 'Customer GSTIN for B2B invoices (India)';
COMMENT ON COLUMN orders.vat_amount IS 'EU VAT amount';
COMMENT ON COLUMN orders.is_reverse_charge IS 'True if EU VAT reverse charge applies (B2B cross-border)';
COMMENT ON COLUMN orders.customer_vat_number IS 'Customer VAT number for B2B invoices (EU)';
COMMENT ON COLUMN order_items.hsn_code IS 'Harmonized System of Nomenclature code (India goods)';
COMMENT ON COLUMN order_items.sac_code IS 'Services Accounting Code (India services)';
COMMENT ON COLUMN order_items.gst_slab IS 'GST slab rate (0, 5, 12, 18, 28)';
COMMENT ON COLUMN order_shipping.state_code IS 'State code for tax determination (MH, KA, CA, etc.)';
COMMENT ON COLUMN order_shipping.country_code IS 'ISO 3166-1 alpha-2 country code (IN, US, GB, etc.)';
