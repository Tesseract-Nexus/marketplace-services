-- Payment Gateway Configuration Tables
-- Supports Stripe, PayPal, and other payment processors

-- Payment Gateway Configurations (per tenant)
CREATE TABLE IF NOT EXISTS payment_gateway_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    gateway_type VARCHAR(50) NOT NULL, -- STRIPE, PAYPAL, SQUARE, AUTHORIZE_NET
    display_name VARCHAR(255) NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    is_test_mode BOOLEAN DEFAULT true,

    -- API Credentials (encrypted in production)
    api_key_public TEXT, -- Stripe publishable key, PayPal client ID
    api_key_secret TEXT, -- Stripe secret key, PayPal secret (ENCRYPTED)
    webhook_secret TEXT, -- Webhook signing secret (ENCRYPTED)

    -- Configuration
    config JSONB, -- Additional gateway-specific settings

    -- Features
    supports_payments BOOLEAN DEFAULT true,
    supports_refunds BOOLEAN DEFAULT true,
    supports_subscriptions BOOLEAN DEFAULT false,

    -- Limits
    minimum_amount DECIMAL(10, 2), -- Minimum transaction amount
    maximum_amount DECIMAL(10, 2), -- Maximum transaction amount

    -- Display
    priority INT DEFAULT 0, -- Display order in checkout
    description TEXT,
    logo_url VARCHAR(500),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_gateway_per_tenant UNIQUE(tenant_id, gateway_type)
);

CREATE INDEX idx_payment_gateways_tenant ON payment_gateway_configs(tenant_id);
CREATE INDEX idx_payment_gateways_enabled ON payment_gateway_configs(is_enabled);

-- Payment Transactions
CREATE TABLE IF NOT EXISTS payment_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    order_id UUID NOT NULL,
    customer_id UUID,

    -- Gateway info
    gateway_config_id UUID NOT NULL REFERENCES payment_gateway_configs(id),
    gateway_type VARCHAR(50) NOT NULL,
    gateway_transaction_id VARCHAR(255), -- Stripe payment intent ID, PayPal order ID

    -- Amount
    amount DECIMAL(12, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',

    -- Status
    status VARCHAR(50) NOT NULL, -- PENDING, PROCESSING, SUCCEEDED, FAILED, CANCELED, REFUNDED
    payment_method_type VARCHAR(50), -- CARD, BANK_ACCOUNT, PAYPAL, APPLE_PAY, GOOGLE_PAY

    -- Card details (last 4 digits only, for display)
    card_brand VARCHAR(50),
    card_last_four VARCHAR(4),
    card_exp_month INT,
    card_exp_year INT,

    -- Customer info
    billing_email VARCHAR(255),
    billing_name VARCHAR(255),
    billing_address JSONB,

    -- Processing
    processed_at TIMESTAMP,
    failed_at TIMESTAMP,
    failure_code VARCHAR(100),
    failure_message TEXT,

    -- Metadata
    metadata JSONB,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_payment_transactions_tenant ON payment_transactions(tenant_id);
CREATE INDEX idx_payment_transactions_order ON payment_transactions(order_id);
CREATE INDEX idx_payment_transactions_customer ON payment_transactions(customer_id);
CREATE INDEX idx_payment_transactions_gateway_id ON payment_transactions(gateway_transaction_id);
CREATE INDEX idx_payment_transactions_status ON payment_transactions(status);
CREATE INDEX idx_payment_transactions_created ON payment_transactions(created_at DESC);

-- Refund Transactions
CREATE TABLE IF NOT EXISTS refund_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    payment_transaction_id UUID NOT NULL REFERENCES payment_transactions(id),

    -- Gateway info
    gateway_refund_id VARCHAR(255), -- Stripe refund ID, PayPal refund ID

    -- Amount
    amount DECIMAL(12, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',

    -- Status
    status VARCHAR(50) NOT NULL, -- PENDING, SUCCEEDED, FAILED, CANCELED
    reason VARCHAR(255), -- REQUESTED_BY_CUSTOMER, DUPLICATE, FRAUDULENT

    -- Processing
    processed_at TIMESTAMP,
    failed_at TIMESTAMP,
    failure_code VARCHAR(100),
    failure_message TEXT,

    -- Metadata
    metadata JSONB,
    notes TEXT,

    created_by UUID, -- Staff member who initiated refund
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_refunds_tenant ON refund_transactions(tenant_id);
CREATE INDEX idx_refunds_payment ON refund_transactions(payment_transaction_id);
CREATE INDEX idx_refunds_status ON refund_transactions(status);

-- Webhook Events (for auditing and debugging)
CREATE TABLE IF NOT EXISTS payment_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255),
    gateway_type VARCHAR(50) NOT NULL,
    event_id VARCHAR(255) NOT NULL, -- Stripe event ID, PayPal event ID
    event_type VARCHAR(100) NOT NULL, -- payment_intent.succeeded, checkout.session.completed

    -- Payload
    payload JSONB NOT NULL,

    -- Processing
    processed BOOLEAN DEFAULT false,
    processed_at TIMESTAMP,
    processing_error TEXT,
    retry_count INT DEFAULT 0,

    -- Related entities
    payment_transaction_id UUID REFERENCES payment_transactions(id),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_webhook_event UNIQUE(gateway_type, event_id)
);

CREATE INDEX idx_webhooks_gateway ON payment_webhook_events(gateway_type);
CREATE INDEX idx_webhooks_type ON payment_webhook_events(event_type);
CREATE INDEX idx_webhooks_processed ON payment_webhook_events(processed);
CREATE INDEX idx_webhooks_created ON payment_webhook_events(created_at DESC);

-- Saved Payment Methods (for recurring payments)
CREATE TABLE IF NOT EXISTS saved_payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    customer_id UUID NOT NULL,

    -- Gateway info
    gateway_config_id UUID NOT NULL REFERENCES payment_gateway_configs(id),
    gateway_type VARCHAR(50) NOT NULL,
    gateway_payment_method_id VARCHAR(255) NOT NULL, -- Stripe payment method ID

    -- Payment method details
    payment_method_type VARCHAR(50) NOT NULL, -- CARD, BANK_ACCOUNT, PAYPAL

    -- Card info (for display only)
    card_brand VARCHAR(50),
    card_last_four VARCHAR(4),
    card_exp_month INT,
    card_exp_year INT,

    -- Bank account info (for display only)
    bank_name VARCHAR(255),
    account_last_four VARCHAR(4),

    -- PayPal
    paypal_email VARCHAR(255),

    -- Status
    is_default BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,

    -- Billing address
    billing_name VARCHAR(255),
    billing_email VARCHAR(255),
    billing_address JSONB,

    -- Verification
    is_verified BOOLEAN DEFAULT false,
    verified_at TIMESTAMP,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_gateway_payment_method UNIQUE(gateway_payment_method_id)
);

CREATE INDEX idx_saved_payment_methods_tenant ON saved_payment_methods(tenant_id);
CREATE INDEX idx_saved_payment_methods_customer ON saved_payment_methods(customer_id);
CREATE INDEX idx_saved_payment_methods_gateway ON saved_payment_methods(gateway_config_id);

-- Payment Disputes/Chargebacks
CREATE TABLE IF NOT EXISTS payment_disputes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    payment_transaction_id UUID NOT NULL REFERENCES payment_transactions(id),

    -- Gateway info
    gateway_dispute_id VARCHAR(255) NOT NULL,

    -- Dispute details
    amount DECIMAL(12, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    reason VARCHAR(100), -- FRAUDULENT, UNRECOGNIZED, DUPLICATE, etc.
    status VARCHAR(50) NOT NULL, -- NEEDS_RESPONSE, UNDER_REVIEW, WON, LOST, ACCEPTED

    -- Evidence
    evidence JSONB,
    evidence_submitted_at TIMESTAMP,

    -- Deadlines
    respond_by TIMESTAMP,

    -- Resolution
    resolved_at TIMESTAMP,
    resolution VARCHAR(50), -- WON, LOST, ACCEPTED

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_disputes_tenant ON payment_disputes(tenant_id);
CREATE INDEX idx_disputes_payment ON payment_disputes(payment_transaction_id);
CREATE INDEX idx_disputes_status ON payment_disputes(status);

-- Payment Settings (per tenant)
CREATE TABLE IF NOT EXISTS payment_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL UNIQUE,

    -- Currency
    default_currency VARCHAR(3) DEFAULT 'USD',
    supported_currencies VARCHAR(3)[] DEFAULT ARRAY['USD'],

    -- Checkout
    enable_express_checkout BOOLEAN DEFAULT true, -- Apple Pay, Google Pay
    collect_billing_address BOOLEAN DEFAULT true,
    collect_shipping_address BOOLEAN DEFAULT true,

    -- 3D Secure
    enable_3d_secure BOOLEAN DEFAULT true,
    require_3d_secure_for_amounts_above DECIMAL(10, 2), -- Require 3DS for amounts above X

    -- Fraud prevention
    enable_fraud_detection BOOLEAN DEFAULT true,
    auto_cancel_risky_payments BOOLEAN DEFAULT false,

    -- Failed payments
    max_payment_retry_attempts INT DEFAULT 3,

    -- Notifications
    send_payment_receipts BOOLEAN DEFAULT true,
    send_refund_notifications BOOLEAN DEFAULT true,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_payment_settings_tenant ON payment_settings(tenant_id);

-- Comments
COMMENT ON TABLE payment_gateway_configs IS 'Payment gateway configurations per tenant (Stripe, PayPal, etc)';
COMMENT ON TABLE payment_transactions IS 'All payment transactions with gateway details';
COMMENT ON TABLE refund_transactions IS 'Refund transactions linked to original payments';
COMMENT ON TABLE payment_webhook_events IS 'Incoming webhook events from payment gateways';
COMMENT ON TABLE saved_payment_methods IS 'Customer saved payment methods for future use';
COMMENT ON TABLE payment_disputes IS 'Payment disputes and chargebacks';
COMMENT ON TABLE payment_settings IS 'Global payment settings per tenant';
