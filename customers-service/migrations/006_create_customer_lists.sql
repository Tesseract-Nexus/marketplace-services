-- Create customer lists tables for multiple named wishlists/collections

-- Customer lists table (e.g., "My Wishlist", "Christmas", "Birthday")
CREATE TABLE IF NOT EXISTS customer_lists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,

    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,

    is_default BOOLEAN DEFAULT FALSE,
    item_count INTEGER DEFAULT 0,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(customer_id, slug)
);

-- Customer list items table
CREATE TABLE IF NOT EXISTS customer_list_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    list_id UUID NOT NULL REFERENCES customer_lists(id) ON DELETE CASCADE,

    product_id UUID NOT NULL,
    product_name VARCHAR(255),
    product_image VARCHAR(500),
    product_price DECIMAL(10,2),

    notes TEXT,
    added_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(list_id, product_id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_customer_lists_customer_id ON customer_lists(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_lists_tenant_id ON customer_lists(tenant_id);
CREATE INDEX IF NOT EXISTS idx_customer_list_items_list_id ON customer_list_items(list_id);
CREATE INDEX IF NOT EXISTS idx_customer_list_items_product_id ON customer_list_items(product_id);

-- Migrate existing wishlist items to default list
-- This will be done as a one-time migration when the service starts
-- For each customer with wishlist items, create a default "My Wishlist" list
-- and move their items to it
