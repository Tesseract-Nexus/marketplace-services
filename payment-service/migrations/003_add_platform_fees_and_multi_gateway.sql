-- Migration: Add Platform Fees and Multi-Gateway Support
-- Adds fee tracking, platform fee ledger, and geo-based gateway configuration

-- ============================================================================
-- Add fee tracking to payment_transactions
-- ============================================================================
ALTER TABLE payment_transactions
ADD COLUMN IF NOT EXISTS gross_amount DECIMAL(12, 2),
ADD COLUMN IF NOT EXISTS platform_fee DECIMAL(12, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS platform_fee_percent DECIMAL(5, 4) DEFAULT 0.0500,
ADD COLUMN IF NOT EXISTS gateway_fee DECIMAL(12, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS gateway_tax DECIMAL(12, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS net_amount DECIMAL(12, 2),
ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255),
ADD COLUMN IF NOT EXISTS retry_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_retry_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS country_code VARCHAR(2);

-- Backfill gross_amount for existing transactions
UPDATE payment_transactions SET gross_amount = amount WHERE gross_amount IS NULL;

-- Add unique constraint for idempotency
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_transactions_idempotency
ON payment_transactions(tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Add index for country-based queries
CREATE INDEX IF NOT EXISTS idx_payment_transactions_country ON payment_transactions(country_code);

-- ============================================================================
-- Add platform split and geo configuration to gateway configs
-- ============================================================================
ALTER TABLE payment_gateway_configs
ADD COLUMN IF NOT EXISTS merchant_account_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS platform_account_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS supports_platform_split BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS supported_countries TEXT[] DEFAULT ARRAY['US'],
ADD COLUMN IF NOT EXISTS supported_payment_methods TEXT[] DEFAULT ARRAY['CARD'],
ADD COLUMN IF NOT EXISTS fee_structure JSONB DEFAULT '{"fixed_fee": 0, "percent_fee": 0}'::jsonb;

-- ============================================================================
-- Add platform fee settings to payment_settings
-- ============================================================================
ALTER TABLE payment_settings
ADD COLUMN IF NOT EXISTS platform_fee_enabled BOOLEAN DEFAULT true,
ADD COLUMN IF NOT EXISTS platform_fee_percent DECIMAL(5, 4) DEFAULT 0.0500,
ADD COLUMN IF NOT EXISTS platform_account_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS fee_payer VARCHAR(20) DEFAULT 'merchant',
ADD COLUMN IF NOT EXISTS minimum_platform_fee DECIMAL(10, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS maximum_platform_fee DECIMAL(10, 2);

-- Add constraint for fee_payer
ALTER TABLE payment_settings
ADD CONSTRAINT IF NOT EXISTS check_fee_payer
CHECK (fee_payer IN ('merchant', 'customer', 'split'));

-- ============================================================================
-- Add fee tracking to refund_transactions
-- ============================================================================
ALTER TABLE refund_transactions
ADD COLUMN IF NOT EXISTS platform_fee_refund DECIMAL(12, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS gateway_fee_refund DECIMAL(12, 2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS net_refund_amount DECIMAL(12, 2),
ADD COLUMN IF NOT EXISTS refund_type VARCHAR(20) DEFAULT 'full';

-- Add constraint for refund_type
ALTER TABLE refund_transactions
ADD CONSTRAINT IF NOT EXISTS check_refund_type
CHECK (refund_type IN ('full', 'partial'));

-- ============================================================================
-- Platform Fee Ledger (for reconciliation)
-- ============================================================================
CREATE TABLE IF NOT EXISTS platform_fee_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    payment_transaction_id UUID REFERENCES payment_transactions(id) ON DELETE SET NULL,
    refund_transaction_id UUID REFERENCES refund_transactions(id) ON DELETE SET NULL,

    -- Entry details
    entry_type VARCHAR(20) NOT NULL,
    amount DECIMAL(12, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',

    -- Status tracking
    status VARCHAR(20) DEFAULT 'pending',

    -- Gateway transfer info
    gateway_type VARCHAR(50),
    gateway_transfer_id VARCHAR(255),
    gateway_payout_id VARCHAR(255),

    -- Settlement
    settled_at TIMESTAMP,
    settlement_batch_id VARCHAR(255),

    -- Error tracking
    error_code VARCHAR(100),
    error_message TEXT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT check_entry_type CHECK (entry_type IN ('collection', 'refund', 'adjustment', 'payout')),
    CONSTRAINT check_ledger_status CHECK (status IN ('pending', 'processing', 'collected', 'refunded', 'failed', 'settled'))
);

CREATE INDEX IF NOT EXISTS idx_platform_fee_ledger_tenant ON platform_fee_ledger(tenant_id);
CREATE INDEX IF NOT EXISTS idx_platform_fee_ledger_status ON platform_fee_ledger(status);
CREATE INDEX IF NOT EXISTS idx_platform_fee_ledger_created ON platform_fee_ledger(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_platform_fee_ledger_payment ON platform_fee_ledger(payment_transaction_id);
CREATE INDEX IF NOT EXISTS idx_platform_fee_ledger_refund ON platform_fee_ledger(refund_transaction_id);

-- ============================================================================
-- Gateway Region Mapping (for geo-based payment method filtering)
-- ============================================================================
CREATE TABLE IF NOT EXISTS payment_gateway_regions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_config_id UUID NOT NULL REFERENCES payment_gateway_configs(id) ON DELETE CASCADE,
    country_code VARCHAR(2) NOT NULL,
    is_primary BOOLEAN DEFAULT false,
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true,

    -- Region-specific settings
    supported_methods TEXT[],
    min_amount DECIMAL(10, 2),
    max_amount DECIMAL(10, 2),
    currency VARCHAR(3),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_gateway_country UNIQUE(gateway_config_id, country_code)
);

CREATE INDEX IF NOT EXISTS idx_gateway_regions_country ON payment_gateway_regions(country_code);
CREATE INDEX IF NOT EXISTS idx_gateway_regions_gateway ON payment_gateway_regions(gateway_config_id);
CREATE INDEX IF NOT EXISTS idx_gateway_regions_enabled ON payment_gateway_regions(enabled) WHERE enabled = true;

-- ============================================================================
-- Gateway Templates (for easy setup of new gateways)
-- ============================================================================
CREATE TABLE IF NOT EXISTS payment_gateway_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_type VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    logo_url VARCHAR(500),

    -- Supported features
    supports_payments BOOLEAN DEFAULT true,
    supports_refunds BOOLEAN DEFAULT true,
    supports_subscriptions BOOLEAN DEFAULT false,
    supports_platform_split BOOLEAN DEFAULT false,

    -- Regional support
    supported_countries TEXT[] NOT NULL,
    supported_payment_methods TEXT[] NOT NULL,

    -- Default configuration
    default_config JSONB,

    -- Required credentials
    required_credentials TEXT[] DEFAULT ARRAY['api_key_public', 'api_key_secret'],

    -- Documentation
    setup_instructions TEXT,
    documentation_url VARCHAR(500),

    -- Display
    priority INT DEFAULT 0,
    is_active BOOLEAN DEFAULT true,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- Seed Gateway Templates
-- ============================================================================
INSERT INTO payment_gateway_templates (
    gateway_type, display_name, description, logo_url,
    supports_payments, supports_refunds, supports_subscriptions, supports_platform_split,
    supported_countries, supported_payment_methods,
    default_config, required_credentials, setup_instructions, documentation_url, priority
) VALUES
-- Stripe (Global)
('STRIPE', 'Stripe',
 'Global payment processing with extensive payment method support including cards, wallets, and bank transfers.',
 '/icons/gateways/stripe.svg',
 true, true, true, true,
 ARRAY['US', 'GB', 'AU', 'CA', 'NZ', 'SG', 'DE', 'FR', 'IT', 'ES', 'NL', 'IE', 'AT', 'BE', 'CH', 'SE', 'NO', 'DK', 'FI', 'PT', 'PL', 'CZ', 'HU', 'RO', 'BG', 'HR', 'SK', 'SI', 'EE', 'LV', 'LT', 'LU', 'MT', 'CY', 'GR', 'JP', 'HK', 'MX', 'BR'],
 ARRAY['CARD', 'APPLE_PAY', 'GOOGLE_PAY', 'BANK_ACCOUNT', 'SEPA', 'IDEAL', 'GIROPAY', 'SOFORT', 'KLARNA'],
 '{"capture_method": "automatic", "statement_descriptor": "TESSERACT"}'::jsonb,
 ARRAY['api_key_public', 'api_key_secret', 'webhook_secret'],
 'Create a Stripe account at stripe.com, then enable Connect for platform fee collection.',
 'https://stripe.com/docs',
 1),

-- PayPal (Global)
('PAYPAL', 'PayPal',
 'Pay with PayPal balance, linked cards, or bank accounts. Available worldwide.',
 '/icons/gateways/paypal.svg',
 true, true, true, true,
 ARRAY['US', 'GB', 'AU', 'CA', 'NZ', 'DE', 'FR', 'IT', 'ES', 'NL', 'IE', 'AT', 'BE', 'CH', 'SE', 'NO', 'DK', 'FI', 'PT', 'PL', 'IN', 'SG', 'HK', 'JP', 'MX', 'BR'],
 ARRAY['PAYPAL', 'CARD'],
 '{"intent": "capture", "landing_page": "LOGIN"}'::jsonb,
 ARRAY['api_key_public', 'api_key_secret', 'webhook_secret'],
 'Create a PayPal Business account and generate REST API credentials.',
 'https://developer.paypal.com/docs',
 2),

-- Razorpay (India)
('RAZORPAY', 'Razorpay',
 'India''s leading payment gateway with UPI, cards, net banking, and wallets.',
 '/icons/gateways/razorpay.svg',
 true, true, true, true,
 ARRAY['IN'],
 ARRAY['CARD', 'UPI', 'NET_BANKING', 'WALLET', 'EMI', 'CARDLESS_EMI', 'PAY_LATER'],
 '{"payment_capture": "automatic", "upi_enabled": true, "international_cards": false}'::jsonb,
 ARRAY['api_key_public', 'api_key_secret', 'webhook_secret'],
 'Sign up at razorpay.com and complete KYC verification.',
 'https://razorpay.com/docs',
 3),

-- PhonePe (India)
('PHONEPE', 'PhonePe',
 'UPI-focused payment gateway for India with wallet support.',
 '/icons/gateways/phonepe.svg',
 true, true, false, false,
 ARRAY['IN'],
 ARRAY['UPI', 'WALLET', 'CARD'],
 '{"checkout_mode": "REDIRECT", "upi_intent_enabled": true}'::jsonb,
 ARRAY['api_key_public', 'api_key_secret', 'merchant_id'],
 'Sign up for PhonePe Business and complete merchant onboarding.',
 'https://developer.phonepe.com/docs',
 4),

-- BharatPay (India)
('BHARATPAY', 'BharatPay',
 'Indian payment gateway with RuPay and UPI support, powered by NPCI.',
 '/icons/gateways/bharatpay.svg',
 true, true, false, false,
 ARRAY['IN'],
 ARRAY['UPI', 'NET_BANKING', 'RUPAY'],
 '{"upi_enabled": true, "rupay_enabled": true}'::jsonb,
 ARRAY['api_key_public', 'api_key_secret', 'merchant_id'],
 'Register with BharatPay and complete bank verification.',
 'https://bharatpay.com/docs',
 5),

-- Afterpay (AU/US/GB/NZ)
('AFTERPAY', 'Afterpay',
 'Buy now, pay later in 4 interest-free installments.',
 '/icons/gateways/afterpay.svg',
 true, true, false, false,
 ARRAY['AU', 'NZ', 'US', 'GB', 'CA'],
 ARRAY['PAY_LATER'],
 '{"minimum_amount": 35, "maximum_amount": 2000, "currency": "AUD"}'::jsonb,
 ARRAY['api_key_public', 'api_key_secret', 'merchant_id'],
 'Sign up for Afterpay Merchant Portal and complete verification.',
 'https://developers.afterpay.com',
 6),

-- Zip (Australia/NZ)
('ZIP', 'Zip Pay',
 'Buy now, pay later with flexible payment plans for Australia and New Zealand.',
 '/icons/gateways/zip.svg',
 true, true, false, false,
 ARRAY['AU', 'NZ'],
 ARRAY['PAY_LATER'],
 '{"minimum_amount": 50, "maximum_amount": 1500, "currency": "AUD"}'::jsonb,
 ARRAY['api_key_public', 'api_key_secret', 'merchant_id'],
 'Apply for Zip merchant account and complete onboarding.',
 'https://zip.co/developers',
 7),

-- Linkt (Global)
('LINKT', 'Linkt',
 'Global payment solution with multi-currency support for cards, banks, and wallets.',
 '/icons/gateways/linkt.svg',
 true, true, true, true,
 ARRAY['US', 'GB', 'AU', 'EU', 'IN', 'SG', 'HK', 'JP', 'CA', 'NZ', 'AE', 'SA', 'KW', 'QA', 'BH', 'OM'],
 ARRAY['CARD', 'BANK_ACCOUNT', 'WALLET', 'APPLE_PAY', 'GOOGLE_PAY'],
 '{"multi_currency": true, "dynamic_conversion": true}'::jsonb,
 ARRAY['api_key_public', 'api_key_secret', 'webhook_secret', 'merchant_id'],
 'Contact Linkt sales for merchant onboarding and API access.',
 'https://linkt.com/docs',
 8)

ON CONFLICT (gateway_type) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    supported_countries = EXCLUDED.supported_countries,
    supported_payment_methods = EXCLUDED.supported_payment_methods,
    default_config = EXCLUDED.default_config,
    updated_at = CURRENT_TIMESTAMP;

-- ============================================================================
-- Update existing gateway configs with new fields
-- ============================================================================
UPDATE payment_gateway_configs
SET
    supported_countries = CASE gateway_type
        WHEN 'RAZORPAY' THEN ARRAY['IN']
        WHEN 'PAYU_INDIA' THEN ARRAY['IN']
        WHEN 'CASHFREE' THEN ARRAY['IN']
        WHEN 'PAYTM' THEN ARRAY['IN']
        WHEN 'STRIPE' THEN ARRAY['US', 'GB', 'AU', 'CA', 'EU']
        WHEN 'PAYPAL' THEN ARRAY['US', 'GB', 'AU', 'EU', 'IN']
        ELSE ARRAY['US']
    END,
    supported_payment_methods = CASE gateway_type
        WHEN 'RAZORPAY' THEN ARRAY['CARD', 'UPI', 'NET_BANKING', 'WALLET']
        WHEN 'PAYU_INDIA' THEN ARRAY['CARD', 'NET_BANKING', 'EMI', 'WALLET']
        WHEN 'CASHFREE' THEN ARRAY['CARD', 'UPI', 'NET_BANKING', 'PAY_LATER']
        WHEN 'PAYTM' THEN ARRAY['WALLET', 'UPI', 'CARD']
        WHEN 'STRIPE' THEN ARRAY['CARD', 'APPLE_PAY', 'GOOGLE_PAY']
        WHEN 'PAYPAL' THEN ARRAY['PAYPAL', 'CARD']
        ELSE ARRAY['CARD']
    END,
    supports_platform_split = CASE gateway_type
        WHEN 'STRIPE' THEN true
        WHEN 'RAZORPAY' THEN true
        WHEN 'PAYPAL' THEN true
        ELSE false
    END
WHERE supported_countries IS NULL OR supported_countries = ARRAY['US'];

-- ============================================================================
-- Comments
-- ============================================================================
COMMENT ON TABLE platform_fee_ledger IS 'Ledger for tracking platform fee collections, refunds, and payouts';
COMMENT ON TABLE payment_gateway_regions IS 'Country-specific configuration for payment gateways';
COMMENT ON TABLE payment_gateway_templates IS 'Pre-configured templates for easy gateway setup';

COMMENT ON COLUMN payment_transactions.platform_fee IS 'Platform commission (5% by default) deducted from merchant';
COMMENT ON COLUMN payment_transactions.gateway_fee IS 'Fee charged by payment gateway (e.g., Stripe fee)';
COMMENT ON COLUMN payment_transactions.net_amount IS 'Amount merchant receives after all fees';
COMMENT ON COLUMN payment_transactions.idempotency_key IS 'Unique key for preventing duplicate transactions';

COMMENT ON COLUMN payment_settings.platform_fee_enabled IS 'Whether to collect platform fee on transactions';
COMMENT ON COLUMN payment_settings.platform_fee_percent IS 'Platform fee percentage (0.0500 = 5%)';
COMMENT ON COLUMN payment_settings.fee_payer IS 'Who pays the platform fee: merchant, customer, or split';
