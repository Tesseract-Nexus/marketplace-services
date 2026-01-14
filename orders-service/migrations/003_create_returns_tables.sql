-- Create returns management tables

-- Returns table
CREATE TABLE IF NOT EXISTS returns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    rma_number VARCHAR(50) UNIQUE NOT NULL,
    order_id UUID NOT NULL,
    customer_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    reason VARCHAR(30) NOT NULL,
    return_type VARCHAR(20) NOT NULL DEFAULT 'REFUND',
    customer_notes TEXT,
    admin_notes TEXT,

    -- Financial details
    refund_amount DECIMAL(10,2),
    refund_method VARCHAR(30),
    refund_processed_at TIMESTAMP,
    restocking_fee DECIMAL(10,2) DEFAULT 0,

    -- Shipping details
    return_shipping_cost DECIMAL(10,2) DEFAULT 0,
    return_tracking_number VARCHAR(100),
    return_carrier VARCHAR(100),
    return_shipping_label_url TEXT,

    -- Exchange details
    exchange_order_id UUID,
    exchange_product_id UUID,

    -- Approval details
    approved_by UUID,
    approved_at TIMESTAMP,
    rejected_by UUID,
    rejected_at TIMESTAMP,
    rejection_reason TEXT,

    -- Inspection details
    inspected_by UUID,
    inspected_at TIMESTAMP,
    inspection_notes TEXT,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP,

    CONSTRAINT fk_returns_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);

-- Return items table
CREATE TABLE IF NOT EXISTS return_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id UUID NOT NULL,
    order_item_id UUID NOT NULL,
    product_id UUID NOT NULL,
    product_name VARCHAR(255) NOT NULL,
    sku VARCHAR(100) NOT NULL,
    quantity INT NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    refund_amount DECIMAL(10,2),
    reason VARCHAR(30),
    item_notes TEXT,

    -- Condition tracking
    received_condition VARCHAR(50),
    is_defective BOOLEAN DEFAULT FALSE,
    can_resell BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT fk_return_items_return FOREIGN KEY (return_id) REFERENCES returns(id) ON DELETE CASCADE,
    CONSTRAINT fk_return_items_order_item FOREIGN KEY (order_item_id) REFERENCES order_items(id) ON DELETE CASCADE
);

-- Return timeline table
CREATE TABLE IF NOT EXISTS return_timeline (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    notes TEXT,
    created_by UUID, -- Staff user ID, null for system events
    created_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT fk_return_timeline_return FOREIGN KEY (return_id) REFERENCES returns(id) ON DELETE CASCADE
);

-- Return policies table
CREATE TABLE IF NOT EXISTS return_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) UNIQUE NOT NULL,
    return_window_days INT DEFAULT 30,
    allow_exchange BOOLEAN DEFAULT TRUE,
    allow_store_credit BOOLEAN DEFAULT TRUE,
    restocking_fee_percent DECIMAL(5,2) DEFAULT 0,
    free_return_shipping BOOLEAN DEFAULT FALSE,
    auto_approve_returns BOOLEAN DEFAULT FALSE,
    require_photos BOOLEAN DEFAULT FALSE,
    policy_text TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_returns_tenant_id ON returns(tenant_id);
CREATE INDEX IF NOT EXISTS idx_returns_order_id ON returns(order_id);
CREATE INDEX IF NOT EXISTS idx_returns_customer_id ON returns(customer_id);
CREATE INDEX IF NOT EXISTS idx_returns_status ON returns(status);
CREATE INDEX IF NOT EXISTS idx_returns_rma_number ON returns(rma_number);
CREATE INDEX IF NOT EXISTS idx_returns_created_at ON returns(created_at);
CREATE INDEX IF NOT EXISTS idx_returns_deleted_at ON returns(deleted_at);

CREATE INDEX IF NOT EXISTS idx_return_items_return_id ON return_items(return_id);
CREATE INDEX IF NOT EXISTS idx_return_items_order_item_id ON return_items(order_item_id);
CREATE INDEX IF NOT EXISTS idx_return_items_product_id ON return_items(product_id);

CREATE INDEX IF NOT EXISTS idx_return_timeline_return_id ON return_timeline(return_id);
CREATE INDEX IF NOT EXISTS idx_return_timeline_created_at ON return_timeline(created_at);

-- Insert default return policy for default tenant
INSERT INTO return_policies (tenant_id, return_window_days, allow_exchange, allow_store_credit, free_return_shipping, policy_text)
VALUES (
    'default-tenant',
    30,
    TRUE,
    TRUE,
    TRUE,
    'We accept returns within 30 days of purchase. Items must be in original condition with tags attached. Free return shipping is provided for all returns.'
) ON CONFLICT (tenant_id) DO NOTHING;
