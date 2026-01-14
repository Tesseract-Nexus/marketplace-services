-- Fix column naming mismatch between old SQL migration and GORM conventions
-- Old migration used address_line_1/address_line_2, GORM uses address_line1/address_line2
-- Also adds label column for user-defined address labels

-- Drop old columns if they exist (GORM created the correct ones)
ALTER TABLE customer_addresses DROP COLUMN IF EXISTS address_line_1;
ALTER TABLE customer_addresses DROP COLUMN IF EXISTS address_line_2;

-- Add label column if it doesn't exist
ALTER TABLE customer_addresses ADD COLUMN IF NOT EXISTS label VARCHAR(50);

-- Ensure address_line1 has NOT NULL constraint
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'customer_addresses' AND column_name = 'address_line1') THEN
        ALTER TABLE customer_addresses ALTER COLUMN address_line1 SET NOT NULL;
    END IF;
END $$;
