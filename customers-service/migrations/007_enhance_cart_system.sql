-- Enhance cart system with 90-day expiration and item status tracking
-- This migration adds production-ready features for cart persistence and validation

-- Create customer_carts table if not exists (may have been auto-migrated by GORM)
CREATE TABLE IF NOT EXISTS customer_carts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    items JSONB NOT NULL DEFAULT '[]',
    subtotal DECIMAL(12,2) NOT NULL DEFAULT 0,
    item_count INT NOT NULL DEFAULT 0,
    last_item_change TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(customer_id, tenant_id)
);

-- Add new columns for cart expiration and validation
ALTER TABLE customer_carts
ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS last_validated_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS has_unavailable_items BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS has_price_changes BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS unavailable_count INT DEFAULT 0;

-- Set default expiration for existing carts (90 days from last update)
UPDATE customer_carts
SET expires_at = updated_at + INTERVAL '90 days'
WHERE expires_at IS NULL;

-- Create function to automatically set expiration on cart update
CREATE OR REPLACE FUNCTION set_cart_expiration()
RETURNS TRIGGER AS $$
BEGIN
    -- Reset expiration to 90 days from now on any cart update
    NEW.expires_at := NOW() + INTERVAL '90 days';
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for automatic expiration management
DROP TRIGGER IF EXISTS cart_expiration_trigger ON customer_carts;
CREATE TRIGGER cart_expiration_trigger
    BEFORE UPDATE ON customer_carts
    FOR EACH ROW
    EXECUTE FUNCTION set_cart_expiration();

-- Create trigger for new carts
CREATE OR REPLACE FUNCTION set_new_cart_expiration()
RETURNS TRIGGER AS $$
BEGIN
    NEW.expires_at := NOW() + INTERVAL '90 days';
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS new_cart_expiration_trigger ON customer_carts;
CREATE TRIGGER new_cart_expiration_trigger
    BEFORE INSERT ON customer_carts
    FOR EACH ROW
    EXECUTE FUNCTION set_new_cart_expiration();

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_customer_carts_customer_id ON customer_carts(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_carts_tenant_id ON customer_carts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_customer_carts_expires_at ON customer_carts(expires_at);
CREATE INDEX IF NOT EXISTS idx_customer_carts_last_validated_at ON customer_carts(last_validated_at);
CREATE INDEX IF NOT EXISTS idx_customer_carts_updated_at ON customer_carts(updated_at);

-- Composite index for finding expired carts efficiently
CREATE INDEX IF NOT EXISTS idx_customer_carts_expiration_cleanup
    ON customer_carts(tenant_id, expires_at)
    WHERE expires_at IS NOT NULL;

-- Composite index for finding carts needing validation
CREATE INDEX IF NOT EXISTS idx_customer_carts_validation_needed
    ON customer_carts(tenant_id, last_validated_at, item_count)
    WHERE item_count > 0;

-- Create a table to track product changes affecting carts (for audit and debugging)
CREATE TABLE IF NOT EXISTS cart_item_status_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    cart_id UUID NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    previous_status VARCHAR(50),
    new_status VARCHAR(50) NOT NULL,
    previous_price DECIMAL(10,2),
    new_price DECIMAL(10,2),
    reason VARCHAR(100), -- product_deleted, out_of_stock, price_changed, restocked
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cart_item_status_log_cart_id ON cart_item_status_log(cart_id);
CREATE INDEX IF NOT EXISTS idx_cart_item_status_log_product_id ON cart_item_status_log(product_id);
CREATE INDEX IF NOT EXISTS idx_cart_item_status_log_tenant_created
    ON cart_item_status_log(tenant_id, created_at DESC);

-- Add comment for documentation
COMMENT ON TABLE customer_carts IS 'Customer shopping carts with 90-day expiration and item validation tracking';
COMMENT ON COLUMN customer_carts.expires_at IS 'Cart expiration date, automatically set to 90 days from last update';
COMMENT ON COLUMN customer_carts.last_validated_at IS 'Timestamp of last product availability/price validation';
COMMENT ON COLUMN customer_carts.has_unavailable_items IS 'Flag indicating cart contains unavailable products';
COMMENT ON COLUMN customer_carts.has_price_changes IS 'Flag indicating cart contains items with price changes since added';
COMMENT ON COLUMN customer_carts.unavailable_count IS 'Count of unavailable items in cart';
