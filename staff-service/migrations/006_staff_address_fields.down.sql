-- Migration: 006_staff_address_fields (rollback)
-- Description: Remove address fields from staff table

-- Drop indexes
DROP INDEX IF EXISTS idx_staff_country_code;
DROP INDEX IF EXISTS idx_staff_city;

-- Remove address columns
ALTER TABLE staff DROP COLUMN IF EXISTS street_address;
ALTER TABLE staff DROP COLUMN IF EXISTS street_address_2;
ALTER TABLE staff DROP COLUMN IF EXISTS city;
ALTER TABLE staff DROP COLUMN IF EXISTS state;
ALTER TABLE staff DROP COLUMN IF EXISTS state_code;
ALTER TABLE staff DROP COLUMN IF EXISTS postal_code;
ALTER TABLE staff DROP COLUMN IF EXISTS country;
ALTER TABLE staff DROP COLUMN IF EXISTS country_code;
ALTER TABLE staff DROP COLUMN IF EXISTS latitude;
ALTER TABLE staff DROP COLUMN IF EXISTS longitude;
ALTER TABLE staff DROP COLUMN IF EXISTS formatted_address;
ALTER TABLE staff DROP COLUMN IF EXISTS place_id;
