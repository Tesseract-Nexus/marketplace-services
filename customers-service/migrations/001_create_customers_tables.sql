-- Create customers database tables

-- Customers table (main customer entity)
CREATE TABLE IF NOT EXISTS customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    user_id UUID NULL, -- NULL for guest customers, references users table for registered
    email VARCHAR(255) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    phone VARCHAR(50),
    status VARCHAR(20) DEFAULT 'ACTIVE', -- ACTIVE, INACTIVE, BLOCKED
    customer_type VARCHAR(20) DEFAULT 'RETAIL', -- RETAIL, WHOLESALE, VIP

    -- Analytics fields
    total_orders INT DEFAULT 0,
    total_spent DECIMAL(12,2) DEFAULT 0,
    average_order_value DECIMAL(10,2) DEFAULT 0,
    lifetime_value DECIMAL(12,2) DEFAULT 0,
    last_order_date TIMESTAMP,
    first_order_date TIMESTAMP,

    -- Engagement
    tags TEXT[], -- Array of tags for segmentation
    notes TEXT,
    marketing_opt_in BOOLEAN DEFAULT false,

    -- Metadata
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP,

    UNIQUE(tenant_id, email)
);

-- Customer addresses table
-- Note: Column names use GORM's snake_case convention (address_line1, not address_line_1)
CREATE TABLE IF NOT EXISTS customer_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,

    -- Address type
    address_type VARCHAR(20) DEFAULT 'SHIPPING', -- SHIPPING, BILLING, BOTH
    is_default BOOLEAN DEFAULT false,
    label VARCHAR(50), -- User-defined label (e.g., "Home", "Work")

    -- Address fields
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    company VARCHAR(255),
    address_line1 VARCHAR(255) NOT NULL,
    address_line2 VARCHAR(255),
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100),
    postal_code VARCHAR(20) NOT NULL,
    country VARCHAR(2) NOT NULL, -- ISO 2-letter code
    phone VARCHAR(50),

    -- Metadata
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Customer payment methods table
CREATE TABLE IF NOT EXISTS customer_payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,

    -- Payment gateway info
    payment_gateway VARCHAR(50) NOT NULL, -- stripe, paypal, razorpay
    gateway_payment_method_id VARCHAR(255) NOT NULL, -- External ID from gateway

    -- Card/method details (last 4, type, etc - no sensitive data)
    payment_type VARCHAR(20) NOT NULL, -- card, bank_account, paypal, upi
    card_brand VARCHAR(20), -- visa, mastercard, amex
    last_four VARCHAR(4),
    expiry_month INT,
    expiry_year INT,

    is_default BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,

    -- Metadata
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Customer groups/segments table
CREATE TABLE IF NOT EXISTS customer_segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,

    -- Segment rules (stored as JSONB for flexibility)
    rules JSONB,

    -- Auto-update or manual
    is_dynamic BOOLEAN DEFAULT false,

    -- Stats
    customer_count INT DEFAULT 0,

    -- Metadata
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(tenant_id, name)
);

-- Customer segment membership (many-to-many)
CREATE TABLE IF NOT EXISTS customer_segment_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    segment_id UUID NOT NULL REFERENCES customer_segments(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,

    -- Auto-added or manual
    added_automatically BOOLEAN DEFAULT false,

    created_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(customer_id, segment_id)
);

-- Customer notes/comments table
CREATE TABLE IF NOT EXISTS customer_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,

    note TEXT NOT NULL,
    created_by UUID, -- User ID who created the note

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Customer communication history
CREATE TABLE IF NOT EXISTS customer_communications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,

    communication_type VARCHAR(50) NOT NULL, -- email, sms, call, chat
    direction VARCHAR(20) NOT NULL, -- inbound, outbound
    subject VARCHAR(255),
    content TEXT,
    status VARCHAR(20) DEFAULT 'sent', -- sent, delivered, failed, opened, clicked

    -- External references
    external_id VARCHAR(255), -- Email service ID, SMS ID, etc

    created_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_customers_tenant_id ON customers(tenant_id);
CREATE INDEX IF NOT EXISTS idx_customers_user_id ON customers(user_id);
CREATE INDEX IF NOT EXISTS idx_customers_email ON customers(email);
CREATE INDEX IF NOT EXISTS idx_customers_status ON customers(status);
CREATE INDEX IF NOT EXISTS idx_customers_customer_type ON customers(customer_type);
CREATE INDEX IF NOT EXISTS idx_customers_deleted_at ON customers(deleted_at);
CREATE INDEX IF NOT EXISTS idx_customers_tags ON customers USING GIN(tags);

CREATE INDEX IF NOT EXISTS idx_customer_addresses_customer_id ON customer_addresses(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_addresses_tenant_id ON customer_addresses(tenant_id);
CREATE INDEX IF NOT EXISTS idx_customer_addresses_is_default ON customer_addresses(is_default);

CREATE INDEX IF NOT EXISTS idx_customer_payment_methods_customer_id ON customer_payment_methods(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_payment_methods_tenant_id ON customer_payment_methods(tenant_id);
CREATE INDEX IF NOT EXISTS idx_customer_payment_methods_is_default ON customer_payment_methods(is_default);

CREATE INDEX IF NOT EXISTS idx_customer_segments_tenant_id ON customer_segments(tenant_id);

CREATE INDEX IF NOT EXISTS idx_customer_segment_members_customer_id ON customer_segment_members(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_segment_members_segment_id ON customer_segment_members(segment_id);

CREATE INDEX IF NOT EXISTS idx_customer_notes_customer_id ON customer_notes(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_notes_tenant_id ON customer_notes(tenant_id);

CREATE INDEX IF NOT EXISTS idx_customer_communications_customer_id ON customer_communications(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_communications_tenant_id ON customer_communications(tenant_id);
CREATE INDEX IF NOT EXISTS idx_customer_communications_type ON customer_communications(communication_type);
