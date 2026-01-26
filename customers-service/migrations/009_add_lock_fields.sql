-- Migration: Add lock/unlock metadata fields to customers table
-- Used for tracking when and why customers are locked/unlocked by admin

-- Add lock-related columns
ALTER TABLE customers ADD COLUMN IF NOT EXISTS lock_reason TEXT;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS locked_by UUID;

-- Add unlock-related columns
ALTER TABLE customers ADD COLUMN IF NOT EXISTS unlock_reason TEXT;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS unlocked_at TIMESTAMP;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS unlocked_by UUID;

-- Create index for efficient lookup of locked customers
CREATE INDEX IF NOT EXISTS idx_customers_locked_at ON customers(locked_at) WHERE locked_at IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN customers.lock_reason IS 'Reason provided by admin when locking the customer account';
COMMENT ON COLUMN customers.locked_at IS 'Timestamp when the customer account was locked';
COMMENT ON COLUMN customers.locked_by IS 'UUID of the staff member who locked the account';
COMMENT ON COLUMN customers.unlock_reason IS 'Reason provided by admin when unlocking the customer account';
COMMENT ON COLUMN customers.unlocked_at IS 'Timestamp when the customer account was unlocked';
COMMENT ON COLUMN customers.unlocked_by IS 'UUID of the staff member who unlocked the account';
