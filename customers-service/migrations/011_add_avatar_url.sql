-- Migration: Add avatar_url field to customers table
-- Purpose: Store customer profile picture URL (GCS public bucket)

ALTER TABLE customers
ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(500);

COMMENT ON COLUMN customers.avatar_url IS 'Profile picture URL stored in GCS public bucket (e.g., https://storage.googleapis.com/bucket/marketplace/{tenantId}/customers/{customerId}/avatar/...)';
