-- Migration: 006_staff_address_fields
-- Description: Add address fields to staff table for location-based features
-- This aligns with the onboarding/settings address structure

-- Add address fields to staff table
ALTER TABLE staff ADD COLUMN IF NOT EXISTS street_address VARCHAR(500);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS street_address_2 VARCHAR(500);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS city VARCHAR(255);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS state VARCHAR(255);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS state_code VARCHAR(10);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS postal_code VARCHAR(20);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS country VARCHAR(255);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS country_code VARCHAR(3);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS latitude DECIMAL(10, 8);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS longitude DECIMAL(11, 8);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS formatted_address TEXT;
ALTER TABLE staff ADD COLUMN IF NOT EXISTS place_id VARCHAR(255);

-- Add index for country_code (useful for phone area code lookups and filtering)
CREATE INDEX IF NOT EXISTS idx_staff_country_code ON staff(country_code);

-- Add index for city (useful for filtering staff by city)
CREATE INDEX IF NOT EXISTS idx_staff_city ON staff(city);

-- Comment on columns
COMMENT ON COLUMN staff.street_address IS 'Primary street address';
COMMENT ON COLUMN staff.street_address_2 IS 'Secondary address line (apt, suite, etc.)';
COMMENT ON COLUMN staff.city IS 'City name';
COMMENT ON COLUMN staff.state IS 'State/Province full name';
COMMENT ON COLUMN staff.state_code IS 'State/Province code (e.g., NSW, CA)';
COMMENT ON COLUMN staff.postal_code IS 'Postal/ZIP code';
COMMENT ON COLUMN staff.country IS 'Country full name';
COMMENT ON COLUMN staff.country_code IS 'ISO 3166-1 alpha-2 country code (e.g., AU, US)';
COMMENT ON COLUMN staff.latitude IS 'GPS latitude coordinate';
COMMENT ON COLUMN staff.longitude IS 'GPS longitude coordinate';
COMMENT ON COLUMN staff.formatted_address IS 'Full formatted address string';
COMMENT ON COLUMN staff.place_id IS 'Google Places ID for the address';
