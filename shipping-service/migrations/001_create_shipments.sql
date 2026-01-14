-- Create shipments table
CREATE TABLE IF NOT EXISTS shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    order_id UUID NOT NULL,
    order_number VARCHAR(100),
    carrier VARCHAR(50) NOT NULL,
    carrier_shipment_id VARCHAR(255),
    tracking_number VARCHAR(255),
    tracking_url VARCHAR(500),
    label_url VARCHAR(500),
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',

    -- From Address
    from_name VARCHAR(255),
    from_street VARCHAR(500),
    from_city VARCHAR(100),
    from_state VARCHAR(100),
    from_postal_code VARCHAR(20),
    from_country VARCHAR(10),
    from_email VARCHAR(255),
    from_phone VARCHAR(50),

    -- To Address
    to_name VARCHAR(255),
    to_street VARCHAR(500),
    to_city VARCHAR(100),
    to_state VARCHAR(100),
    to_postal_code VARCHAR(20),
    to_country VARCHAR(10),
    to_email VARCHAR(255),
    to_phone VARCHAR(50),

    -- Package Details
    weight DECIMAL(10,2),
    length DECIMAL(10,2),
    width DECIMAL(10,2),
    height DECIMAL(10,2),

    -- Pricing
    shipping_cost DECIMAL(10,2),
    currency VARCHAR(10) DEFAULT 'USD',

    -- Dates
    estimated_delivery TIMESTAMP,
    actual_delivery TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create shipment_tracking table
CREATE TABLE IF NOT EXISTS shipment_tracking (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shipment_id UUID NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    status VARCHAR(100) NOT NULL,
    location VARCHAR(255),
    description TEXT,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for shipments
CREATE INDEX IF NOT EXISTS idx_shipments_tenant_id ON shipments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_shipments_order_id ON shipments(order_id);
CREATE INDEX IF NOT EXISTS idx_shipments_tracking_number ON shipments(tracking_number);
CREATE INDEX IF NOT EXISTS idx_shipments_status ON shipments(status);
CREATE INDEX IF NOT EXISTS idx_shipments_created_at ON shipments(created_at);

-- Create indexes for shipment_tracking
CREATE INDEX IF NOT EXISTS idx_shipment_tracking_shipment_id ON shipment_tracking(shipment_id);
CREATE INDEX IF NOT EXISTS idx_shipment_tracking_timestamp ON shipment_tracking(timestamp);
