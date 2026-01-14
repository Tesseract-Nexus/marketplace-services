-- Migration 002: Update default markup percentage to 150%
-- This ensures existing tenants have a reasonable markup to cover actual shipping costs
-- Shiprocket API returns base rates without GST/fuel surcharge, so 150% markup is recommended

-- Update the default value for new records
ALTER TABLE shipping_settings
ALTER COLUMN handling_fee_percent SET DEFAULT 1.5;

-- Update existing records that have 0 or low markup (less than 50%) to use the new default
-- This prevents tenants from losing money on shipping
UPDATE shipping_settings
SET handling_fee_percent = 1.5, updated_at = NOW()
WHERE handling_fee_percent < 0.5;

-- Add a comment for documentation
COMMENT ON COLUMN shipping_settings.handling_fee_percent IS 'Markup percentage (1.5 = 150%). Default 150% covers GST, fuel surcharge, and handling fees not included in carrier API rates.';
