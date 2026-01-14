-- Create abandoned carts tables for cart recovery campaigns

-- Abandoned carts table (tracks carts that have been abandoned)
CREATE TABLE IF NOT EXISTS abandoned_carts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    cart_id UUID NOT NULL,
    customer_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- PENDING, REMINDED, RECOVERED, EXPIRED

    -- Cart snapshot at time of abandonment
    items JSONB NOT NULL DEFAULT '[]',
    subtotal DECIMAL(12,2) NOT NULL DEFAULT 0,
    item_count INT NOT NULL DEFAULT 0,

    -- Customer info for outreach
    customer_email VARCHAR(255),
    customer_first_name VARCHAR(100),
    customer_last_name VARCHAR(100),

    -- Abandonment tracking
    abandoned_at TIMESTAMP NOT NULL,
    last_cart_activity TIMESTAMP NOT NULL,

    -- Recovery tracking
    reminder_count INT DEFAULT 0,
    last_reminder_at TIMESTAMP,
    next_reminder_at TIMESTAMP,
    recovered_at TIMESTAMP,
    recovered_order_id UUID,
    expired_at TIMESTAMP,

    -- Analytics
    recovery_source VARCHAR(50), -- email_reminder, discount_offer, direct
    discount_used VARCHAR(50),
    recovered_value DECIMAL(12,2) DEFAULT 0,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Recovery attempts table (tracks individual outreach attempts)
CREATE TABLE IF NOT EXISTS abandoned_cart_recovery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    abandoned_cart_id UUID NOT NULL REFERENCES abandoned_carts(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    attempt_type VARCHAR(50) NOT NULL, -- email, sms, push
    attempt_number INT NOT NULL,
    status VARCHAR(20) NOT NULL, -- sent, delivered, opened, clicked, failed
    message_template VARCHAR(100),
    discount_offered VARCHAR(50),
    external_id VARCHAR(255),
    sent_at TIMESTAMP NOT NULL,
    delivered_at TIMESTAMP,
    opened_at TIMESTAMP,
    clicked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Tenant settings for abandoned cart recovery
CREATE TABLE IF NOT EXISTS abandoned_cart_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL UNIQUE,

    -- Abandonment detection
    abandonment_threshold_minutes INT DEFAULT 60,
    expiration_days INT DEFAULT 30,
    enabled BOOLEAN DEFAULT true,

    -- Reminder schedule (hours after abandonment)
    first_reminder_hours INT DEFAULT 1,
    second_reminder_hours INT DEFAULT 24,
    third_reminder_hours INT DEFAULT 72,
    max_reminders INT DEFAULT 3,

    -- Incentives
    offer_discount_on_reminder INT DEFAULT 2, -- Which reminder gets a discount offer (0=none)
    discount_type VARCHAR(20),
    discount_value DECIMAL(10,2),
    discount_code VARCHAR(50),

    -- Email templates
    reminder_email_template1 VARCHAR(100),
    reminder_email_template2 VARCHAR(100),
    reminder_email_template3 VARCHAR(100),

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_tenant_id ON abandoned_carts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_customer_id ON abandoned_carts(customer_id);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_cart_id ON abandoned_carts(cart_id);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_status ON abandoned_carts(status);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_abandoned_at ON abandoned_carts(abandoned_at);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_next_reminder_at ON abandoned_carts(next_reminder_at);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_deleted_at ON abandoned_carts(deleted_at);

-- Composite index for finding carts due for reminders
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_pending_reminders
    ON abandoned_carts(tenant_id, status, next_reminder_at)
    WHERE status IN ('PENDING', 'REMINDED') AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_abandoned_cart_recovery_attempts_abandoned_cart_id
    ON abandoned_cart_recovery_attempts(abandoned_cart_id);
CREATE INDEX IF NOT EXISTS idx_abandoned_cart_recovery_attempts_tenant_id
    ON abandoned_cart_recovery_attempts(tenant_id);

CREATE INDEX IF NOT EXISTS idx_abandoned_cart_settings_tenant_id ON abandoned_cart_settings(tenant_id);
