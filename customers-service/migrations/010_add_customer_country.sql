-- Migration: Add country fields to customers table
-- Purpose: Store customer's primary country/region for payment method filtering and localization

-- Add country fields to customers table
ALTER TABLE customers
ADD COLUMN IF NOT EXISTS country VARCHAR(100),
ADD COLUMN IF NOT EXISTS country_code VARCHAR(2);

-- Create index for efficient country-based queries (multi-tenant)
CREATE INDEX IF NOT EXISTS idx_customers_tenant_country
ON customers (tenant_id, country_code)
WHERE country_code IS NOT NULL;

-- Add comment for documentation
COMMENT ON COLUMN customers.country IS 'Full country name (e.g., Australia, India, United States)';
COMMENT ON COLUMN customers.country_code IS 'ISO 3166-1 alpha-2 country code (e.g., AU, IN, US) - used for payment method filtering';

-- Update existing customers: Try to derive country from their default shipping address
UPDATE customers c
SET
    country_code = UPPER(a.country),
    country = CASE
        WHEN UPPER(a.country) = 'US' THEN 'United States'
        WHEN UPPER(a.country) = 'AU' THEN 'Australia'
        WHEN UPPER(a.country) = 'IN' THEN 'India'
        WHEN UPPER(a.country) = 'GB' THEN 'United Kingdom'
        WHEN UPPER(a.country) = 'CA' THEN 'Canada'
        WHEN UPPER(a.country) = 'NZ' THEN 'New Zealand'
        WHEN UPPER(a.country) = 'SG' THEN 'Singapore'
        WHEN UPPER(a.country) = 'DE' THEN 'Germany'
        WHEN UPPER(a.country) = 'FR' THEN 'France'
        WHEN UPPER(a.country) = 'JP' THEN 'Japan'
        ELSE a.country
    END
FROM customer_addresses a
WHERE a.customer_id = c.id
AND a.is_default = true
AND a.address_type IN ('SHIPPING', 'BOTH')
AND c.country_code IS NULL;
