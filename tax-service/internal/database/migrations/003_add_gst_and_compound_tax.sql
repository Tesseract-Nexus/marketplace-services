-- Migration: Add compound tax and India GST support
-- This migration adds:
-- 1. is_compound column for compound tax calculations
-- 2. HSN/SAC codes for India GST
-- 3. GST slab rate for categories
-- 4. Nil-rated and zero-rated flags for special tax treatment

-- Add is_compound to tax_rates for compound tax (tax on tax)
ALTER TABLE tax_rates
ADD COLUMN IF NOT EXISTS is_compound BOOLEAN DEFAULT false;

-- Add India GST fields to product_tax_categories
ALTER TABLE product_tax_categories
ADD COLUMN IF NOT EXISTS hsn_code VARCHAR(10),  -- Harmonized System Nomenclature (goods)
ADD COLUMN IF NOT EXISTS sac_code VARCHAR(10),  -- Services Accounting Code
ADD COLUMN IF NOT EXISTS gst_slab DECIMAL(5, 2) DEFAULT 0,  -- GST slab: 0, 5, 12, 18, 28
ADD COLUMN IF NOT EXISTS is_nil_rated BOOLEAN DEFAULT false,  -- 0% GST but not exempt
ADD COLUMN IF NOT EXISTS is_zero_rated BOOLEAN DEFAULT false;  -- EU VAT zero-rated

-- Add indexes for HSN/SAC code lookups
CREATE INDEX IF NOT EXISTS idx_product_tax_categories_hsn ON product_tax_categories(hsn_code) WHERE hsn_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_product_tax_categories_sac ON product_tax_categories(sac_code) WHERE sac_code IS NOT NULL;

-- Add performance index for compound tax calculation
CREATE INDEX IF NOT EXISTS idx_tax_rates_active_compound ON tax_rates(is_active, is_compound, priority);

-- Add state code to jurisdictions for IGST determination (interstate vs intrastate)
ALTER TABLE tax_jurisdictions
ADD COLUMN IF NOT EXISTS state_code VARCHAR(10);  -- India state code (MH, KA, TN, etc.)

-- Add tax registration fields to tax_nexus for GSTIN
ALTER TABLE tax_nexus
ADD COLUMN IF NOT EXISTS gstin VARCHAR(15),  -- India - Goods and Services Tax Identification Number
ADD COLUMN IF NOT EXISTS vat_number VARCHAR(50),  -- EU VAT number
ADD COLUMN IF NOT EXISTS is_composition_scheme BOOLEAN DEFAULT false;  -- India GST composition scheme

-- Comment updates
COMMENT ON COLUMN tax_rates.is_compound IS 'If true, tax is calculated on subtotal + previous taxes (compound). Used for Quebec QST on GST.';
COMMENT ON COLUMN product_tax_categories.hsn_code IS 'India HSN code (Harmonized System Nomenclature) for goods - 4 to 8 digits';
COMMENT ON COLUMN product_tax_categories.sac_code IS 'India SAC code (Services Accounting Code) for services - 6 digits';
COMMENT ON COLUMN product_tax_categories.gst_slab IS 'India GST slab rate (0, 5, 12, 18, 28). Used when no specific rate is configured.';
COMMENT ON COLUMN product_tax_categories.is_nil_rated IS 'India: 0% GST but seller cannot claim input tax credit';
COMMENT ON COLUMN product_tax_categories.is_zero_rated IS 'EU VAT: 0% rate but seller can claim input tax credit (exports, etc.)';
COMMENT ON COLUMN tax_jurisdictions.state_code IS 'State code for interstate tax determination (India: MH, KA, etc.)';
COMMENT ON COLUMN tax_nexus.gstin IS 'India Goods and Services Tax Identification Number (15 characters)';
COMMENT ON COLUMN tax_nexus.vat_number IS 'EU VAT registration number';
COMMENT ON COLUMN tax_nexus.is_composition_scheme IS 'India: Business opted for GST composition scheme (limited to intrastate B2C)';
