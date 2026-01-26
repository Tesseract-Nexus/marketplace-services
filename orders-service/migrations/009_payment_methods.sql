-- Migration: 009_payment_methods
-- Description: Add payment methods master table and tenant payment configurations
-- Based on industry patterns from Shopify, Dukaan, and WooCommerce

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Payment methods master table (seeded reference data)
-- Stores available payment methods with their provider, regional support, and fees
CREATE TABLE IF NOT EXISTS payment_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    provider VARCHAR(50) NOT NULL,
    type VARCHAR(30) NOT NULL, -- card, wallet, bnpl, upi, gateway
    supported_regions TEXT[] NOT NULL,
    supported_currencies TEXT[] NOT NULL,
    icon_url TEXT,
    transaction_fee_percent DECIMAL(5,2) DEFAULT 0,
    transaction_fee_fixed DECIMAL(10,2) DEFAULT 0,
    display_order INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Tenant payment configurations
-- Stores per-tenant enabled payment methods with encrypted credentials
CREATE TABLE IF NOT EXISTS tenant_payment_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    payment_method_code VARCHAR(50) NOT NULL REFERENCES payment_methods(code) ON DELETE CASCADE,
    is_enabled BOOLEAN DEFAULT FALSE,
    is_test_mode BOOLEAN DEFAULT TRUE,
    display_order INT DEFAULT 0,
    -- Encrypted credentials stored as JSON
    credentials_encrypted BYTEA,
    -- Non-sensitive settings stored as JSONB
    settings JSONB DEFAULT '{}',
    -- Test connection tracking
    last_test_at TIMESTAMP WITH TIME ZONE,
    last_test_success BOOLEAN,
    last_test_message TEXT,
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    UNIQUE(tenant_id, payment_method_code)
);

-- Payment configuration audit log
-- Tracks all changes to payment configurations for compliance
CREATE TABLE IF NOT EXISTS payment_config_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    payment_method_code VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL, -- enable, disable, configure, test
    user_id VARCHAR(255),
    ip_address VARCHAR(45),
    changes JSONB, -- Old and new values (credentials masked)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_tenant_payment_configs_tenant ON tenant_payment_configs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_payment_configs_enabled ON tenant_payment_configs(tenant_id, is_enabled) WHERE is_enabled = TRUE;
CREATE INDEX IF NOT EXISTS idx_payment_config_audit_tenant ON payment_config_audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_payment_config_audit_created ON payment_config_audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_methods_active ON payment_methods(is_active) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_payment_methods_regions ON payment_methods USING GIN (supported_regions);

-- Seed payment methods with multi-region support
-- Based on research: Shopify, Dukaan (India), and best practices
INSERT INTO payment_methods (code, name, description, provider, type, supported_regions, supported_currencies, transaction_fee_percent, transaction_fee_fixed, display_order, icon_url) VALUES
-- Global payment methods
('stripe', 'Credit/Debit Cards', 'Accept Visa, Mastercard, American Express, and more via Stripe', 'Stripe', 'card',
 ARRAY['AU','NZ','US','GB','CA','DE','FR','SG','HK','GLOBAL'], ARRAY['AUD','NZD','USD','GBP','CAD','EUR','SGD','HKD'],
 1.75, 0.30, 1, '/icons/payment/stripe.svg'),

('paypal', 'PayPal', 'PayPal checkout with buyer protection', 'PayPal', 'wallet',
 ARRAY['AU','NZ','US','GB','CA','DE','FR','IN','SG','GLOBAL'], ARRAY['AUD','NZD','USD','GBP','CAD','EUR','INR','SGD'],
 2.60, 0.30, 2, '/icons/payment/paypal.svg'),

-- Australia/NZ specific - Buy Now Pay Later
('afterpay', 'Afterpay', 'Buy now, pay in 4 interest-free installments', 'Afterpay', 'bnpl',
 ARRAY['AU','NZ','US','GB','CA'], ARRAY['AUD','NZD','USD','GBP','CAD'],
 5.00, 0.30, 3, '/icons/payment/afterpay.svg'),

('zip', 'Zip Pay', 'Flexible buy now pay later with interest-free options', 'Zip', 'bnpl',
 ARRAY['AU','NZ','US','GB'], ARRAY['AUD','NZD','USD','GBP'],
 4.00, 0.00, 4, '/icons/payment/zip.svg'),

-- India specific - Razorpay ecosystem
('razorpay', 'Razorpay', 'Accept cards, UPI, netbanking, wallets in India', 'Razorpay', 'gateway',
 ARRAY['IN'], ARRAY['INR'],
 2.00, 0.00, 1, '/icons/payment/razorpay.svg'),

('razorpay_upi', 'UPI', 'Instant UPI payments via Razorpay (Google Pay, PhonePe, Paytm)', 'Razorpay', 'upi',
 ARRAY['IN'], ARRAY['INR'],
 0.00, 0.00, 2, '/icons/payment/upi.svg'),

('razorpay_netbanking', 'Net Banking', 'Direct bank transfers via Razorpay', 'Razorpay', 'netbanking',
 ARRAY['IN'], ARRAY['INR'],
 1.80, 0.00, 3, '/icons/payment/netbanking.svg'),

('razorpay_wallet', 'Digital Wallets', 'Paytm, PhonePe, Amazon Pay wallets via Razorpay', 'Razorpay', 'wallet',
 ARRAY['IN'], ARRAY['INR'],
 2.00, 0.00, 4, '/icons/payment/wallet.svg'),

-- Cash on Delivery (no fees)
('cod', 'Cash on Delivery', 'Pay with cash when your order arrives', 'Manual', 'cod',
 ARRAY['IN','AU','NZ','SG'], ARRAY['INR','AUD','NZD','SGD'],
 0.00, 0.00, 10, '/icons/payment/cod.svg'),

-- Bank Transfer
('bank_transfer', 'Bank Transfer', 'Direct bank transfer payment', 'Manual', 'bank',
 ARRAY['AU','NZ','US','GB','IN','GLOBAL'], ARRAY['AUD','NZD','USD','GBP','INR'],
 0.00, 0.00, 11, '/icons/payment/bank.svg')

ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    supported_regions = EXCLUDED.supported_regions,
    supported_currencies = EXCLUDED.supported_currencies,
    transaction_fee_percent = EXCLUDED.transaction_fee_percent,
    transaction_fee_fixed = EXCLUDED.transaction_fee_fixed,
    updated_at = NOW();

-- Function to update timestamp on modification
CREATE OR REPLACE FUNCTION update_payment_config_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for auto-updating timestamps
DROP TRIGGER IF EXISTS trigger_update_payment_config_timestamp ON tenant_payment_configs;
CREATE TRIGGER trigger_update_payment_config_timestamp
    BEFORE UPDATE ON tenant_payment_configs
    FOR EACH ROW
    EXECUTE FUNCTION update_payment_config_timestamp();

DROP TRIGGER IF EXISTS trigger_update_payment_methods_timestamp ON payment_methods;
CREATE TRIGGER trigger_update_payment_methods_timestamp
    BEFORE UPDATE ON payment_methods
    FOR EACH ROW
    EXECUTE FUNCTION update_payment_config_timestamp();
