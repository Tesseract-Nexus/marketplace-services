-- Create campaigns table
CREATE TABLE IF NOT EXISTS campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',

    -- Segmentation
    segment_id UUID,
    target_all BOOLEAN DEFAULT FALSE,

    -- Content
    subject VARCHAR(500),
    content TEXT,
    template_id UUID,

    -- Scheduling
    scheduled_at TIMESTAMP,
    sent_at TIMESTAMP,

    -- Analytics
    total_recipients BIGINT DEFAULT 0,
    sent BIGINT DEFAULT 0,
    delivered BIGINT DEFAULT 0,
    opened BIGINT DEFAULT 0,
    clicked BIGINT DEFAULT 0,
    converted BIGINT DEFAULT 0,
    unsubscribed BIGINT DEFAULT 0,
    failed BIGINT DEFAULT 0,
    revenue DECIMAL(15,2) DEFAULT 0,

    -- Metadata
    metadata JSONB,
    created_by UUID,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_campaigns_tenant ON campaigns(tenant_id);
CREATE INDEX IF NOT EXISTS idx_campaigns_segment ON campaigns(segment_id);
CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status);
CREATE INDEX IF NOT EXISTS idx_campaigns_deleted_at ON campaigns(deleted_at);

-- Create customer_segments table
CREATE TABLE IF NOT EXISTS customer_segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,

    -- Rules
    rules JSONB NOT NULL,

    -- Statistics
    customer_count BIGINT DEFAULT 0,
    last_calculated TIMESTAMP,

    -- Metadata
    is_active BOOLEAN DEFAULT TRUE,
    created_by UUID,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_segments_tenant ON customer_segments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_segments_is_active ON customer_segments(is_active);
CREATE INDEX IF NOT EXISTS idx_segments_deleted_at ON customer_segments(deleted_at);

-- Create abandoned_carts table
CREATE TABLE IF NOT EXISTS abandoned_carts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) NOT NULL,
    customer_id UUID NOT NULL,
    session_id VARCHAR(255),

    -- Cart details
    cart_items JSONB NOT NULL,
    total_amount DECIMAL(15,2) NOT NULL,
    item_count INTEGER NOT NULL,

    -- Recovery
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    recovery_attempts INTEGER DEFAULT 0,
    last_reminder_sent TIMESTAMP,

    -- Outcome
    recovered_at TIMESTAMP,
    order_id UUID,
    recovered_amount DECIMAL(15,2) DEFAULT 0,

    -- Metadata
    abandoned_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_abandoned_carts_tenant ON abandoned_carts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_customer ON abandoned_carts(customer_id);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_status ON abandoned_carts(status);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_abandoned_at ON abandoned_carts(abandoned_at);

-- Create loyalty_programs table
CREATE TABLE IF NOT EXISTS loyalty_programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Points configuration
    points_per_dollar DECIMAL(10,2) NOT NULL DEFAULT 1,
    minimum_points INTEGER DEFAULT 0,
    points_expiry INTEGER DEFAULT 365,

    -- Tiers
    tiers JSONB,

    -- Settings
    is_active BOOLEAN DEFAULT TRUE,
    signup_bonus INTEGER DEFAULT 0,
    birthday_bonus INTEGER DEFAULT 0,
    referral_bonus INTEGER DEFAULT 0,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_loyalty_programs_tenant ON loyalty_programs(tenant_id);

-- Create customer_loyalty table
CREATE TABLE IF NOT EXISTS customer_loyalties (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) NOT NULL,
    customer_id UUID NOT NULL,

    -- Points
    total_points INTEGER DEFAULT 0,
    available_points INTEGER DEFAULT 0,
    lifetime_points INTEGER DEFAULT 0,

    -- Tier
    current_tier VARCHAR(100),
    tier_since TIMESTAMP,

    -- Metadata
    last_earned TIMESTAMP,
    last_redeemed TIMESTAMP,
    joined_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_tenant_customer UNIQUE (tenant_id, customer_id)
);

CREATE INDEX IF NOT EXISTS idx_loyalty_tenant ON customer_loyalties(tenant_id);
CREATE INDEX IF NOT EXISTS idx_loyalty_customer ON customer_loyalties(customer_id);

-- Create loyalty_transactions table
CREATE TABLE IF NOT EXISTS loyalty_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) NOT NULL,
    customer_id UUID NOT NULL,
    loyalty_id UUID NOT NULL,

    -- Transaction details
    type VARCHAR(50) NOT NULL,
    points INTEGER NOT NULL,
    description VARCHAR(500),

    -- Reference
    order_id UUID,
    reference_id UUID,
    reference_type VARCHAR(50),

    -- Expiry
    expires_at TIMESTAMP,
    expired_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_loyalty_txn_tenant ON loyalty_transactions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_loyalty_txn_customer ON loyalty_transactions(customer_id);
CREATE INDEX IF NOT EXISTS idx_loyalty_txn_created_at ON loyalty_transactions(created_at);

-- Create coupon_codes table
CREATE TABLE IF NOT EXISTS coupon_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Discount configuration
    type VARCHAR(50) NOT NULL,
    discount_value DECIMAL(15,2) NOT NULL,
    max_discount DECIMAL(15,2),

    -- Conditions
    min_order_amount DECIMAL(15,2),
    max_order_amount DECIMAL(15,2),
    applicable_products JSONB,
    applicable_categories JSONB,
    excluded_products JSONB,

    -- Usage limits
    max_usage INTEGER DEFAULT 0,
    usage_per_customer INTEGER DEFAULT 1,
    current_usage INTEGER DEFAULT 0,

    -- Validity
    valid_from TIMESTAMP NOT NULL,
    valid_until TIMESTAMP NOT NULL,

    -- Settings
    is_active BOOLEAN DEFAULT TRUE,
    is_public BOOLEAN DEFAULT TRUE,
    campaign_id UUID,

    -- Metadata
    created_by UUID,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,

    CONSTRAINT unique_tenant_code UNIQUE (tenant_id, code)
);

CREATE INDEX IF NOT EXISTS idx_coupons_tenant ON coupon_codes(tenant_id);
CREATE INDEX IF NOT EXISTS idx_coupons_code ON coupon_codes(code);
CREATE INDEX IF NOT EXISTS idx_coupons_is_active ON coupon_codes(is_active);
CREATE INDEX IF NOT EXISTS idx_coupons_deleted_at ON coupon_codes(deleted_at);

-- Create coupon_usages table
CREATE TABLE IF NOT EXISTS coupon_usages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) NOT NULL,
    coupon_id UUID NOT NULL REFERENCES coupon_codes(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL,
    order_id UUID NOT NULL UNIQUE,

    discount_amount DECIMAL(15,2) NOT NULL,
    order_total DECIMAL(15,2) NOT NULL,

    used_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_coupon_usages_tenant ON coupon_usages(tenant_id);
CREATE INDEX IF NOT EXISTS idx_coupon_usages_coupon ON coupon_usages(coupon_id);
CREATE INDEX IF NOT EXISTS idx_coupon_usages_customer ON coupon_usages(customer_id);

-- Create campaign_recipients table
CREATE TABLE IF NOT EXISTS campaign_recipients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL,

    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    sent_at TIMESTAMP,
    delivered_at TIMESTAMP,
    opened_at TIMESTAMP,
    clicked_at TIMESTAMP,
    converted_at TIMESTAMP,
    unsubscribed_at TIMESTAMP,

    error_message TEXT,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_recipients_campaign ON campaign_recipients(campaign_id);
CREATE INDEX IF NOT EXISTS idx_recipients_customer ON campaign_recipients(customer_id);
CREATE INDEX IF NOT EXISTS idx_recipients_status ON campaign_recipients(status);
