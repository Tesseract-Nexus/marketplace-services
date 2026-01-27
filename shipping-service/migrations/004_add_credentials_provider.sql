-- Migration: Add credential provider tracking to shipping_carrier_configs
-- This supports the transition from database-stored credentials to GCP Secret Manager

ALTER TABLE shipping_carrier_configs ADD COLUMN IF NOT EXISTS credentials_provisioned BOOLEAN DEFAULT false;
ALTER TABLE shipping_carrier_configs ADD COLUMN IF NOT EXISTS credential_provider VARCHAR(50) DEFAULT 'database';
