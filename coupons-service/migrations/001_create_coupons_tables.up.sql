-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create coupons table
CREATE TABLE coupons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    created_by_id VARCHAR(255) NOT NULL,
    updated_by_id VARCHAR(255) NOT NULL,
    
    -- Basic Information
    code VARCHAR(255) NOT NULL,
    description TEXT,
    display_text TEXT,
    image_url TEXT,
    thumbnail_url TEXT,
    
    -- Scope and Status
    scope VARCHAR(50) NOT NULL DEFAULT 'APPLICATION',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    priority VARCHAR(50) NOT NULL DEFAULT 'MEDIUM',
    is_active BOOLEAN NOT NULL DEFAULT true,
    
    -- Discount Configuration
    discount_type VARCHAR(50) NOT NULL,
    discount_value DECIMAL(10,2) NOT NULL,
    max_discount DECIMAL(10,2),
    min_order_value DECIMAL(10,2),
    max_discount_per_vendor DECIMAL(10,2),
    
    -- Usage Limits
    max_usage_count INTEGER,
    current_usage_count INTEGER NOT NULL DEFAULT 0,
    max_usage_per_user INTEGER,
    max_usage_per_tenant INTEGER,
    max_usage_per_vendor INTEGER,
    
    -- Restrictions
    first_time_user_only BOOLEAN NOT NULL DEFAULT false,
    min_item_count INTEGER,
    max_item_count INTEGER,
    
    -- Target Criteria (JSONB arrays)
    excluded_tenants JSONB,
    excluded_vendors JSONB,
    category_ids JSONB,
    product_ids JSONB,
    user_group_ids JSONB,
    country_codes JSONB,
    region_codes JSONB,
    
    -- Time Restrictions
    valid_from TIMESTAMP WITH TIME ZONE NOT NULL,
    valid_until TIMESTAMP WITH TIME ZONE,
    days_of_week JSONB, -- [1,2,3,4,5] for Mon-Fri
    time_windows JSONB,
    
    -- Payment and Stacking
    allowed_payment_methods JSONB,
    stackable_with_other BOOLEAN NOT NULL DEFAULT false,
    stackable_priority INTEGER NOT NULL DEFAULT 0,
    combination VARCHAR(50) NOT NULL DEFAULT 'NONE',
    
    -- Metadata
    metadata JSONB,
    tags JSONB,
    
    -- Audit Fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create coupon_usage table
CREATE TABLE coupon_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    coupon_id UUID NOT NULL REFERENCES coupons(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    order_id VARCHAR(255),
    vendor_id VARCHAR(255),
    
    -- Usage Details
    discount_amount DECIMAL(10,2) NOT NULL,
    order_value DECIMAL(10,2) NOT NULL,
    payment_method VARCHAR(50),
    application_source VARCHAR(255),
    
    -- Metadata
    metadata JSONB,
    
    -- Audit Fields
    used_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for coupons table
CREATE INDEX idx_coupons_tenant_id ON coupons(tenant_id);
CREATE UNIQUE INDEX idx_coupons_tenant_code ON coupons(tenant_id, code) WHERE deleted_at IS NULL;
CREATE INDEX idx_coupons_status ON coupons(status);
CREATE INDEX idx_coupons_scope ON coupons(scope);
CREATE INDEX idx_coupons_discount_type ON coupons(discount_type);
CREATE INDEX idx_coupons_valid_from ON coupons(valid_from);
CREATE INDEX idx_coupons_valid_until ON coupons(valid_until);
CREATE INDEX idx_coupons_is_active ON coupons(is_active);
CREATE INDEX idx_coupons_deleted_at ON coupons(deleted_at);

-- Create GIN indexes for JSONB columns
CREATE INDEX idx_coupons_category_ids ON coupons USING GIN (category_ids);
CREATE INDEX idx_coupons_product_ids ON coupons USING GIN (product_ids);
CREATE INDEX idx_coupons_excluded_vendors ON coupons USING GIN (excluded_vendors);
CREATE INDEX idx_coupons_country_codes ON coupons USING GIN (country_codes);
CREATE INDEX idx_coupons_region_codes ON coupons USING GIN (region_codes);
CREATE INDEX idx_coupons_tags ON coupons USING GIN (tags);
CREATE INDEX idx_coupons_metadata ON coupons USING GIN (metadata);

-- Create indexes for coupon_usage table
CREATE INDEX idx_coupon_usage_tenant_id ON coupon_usage(tenant_id);
CREATE INDEX idx_coupon_usage_coupon_id ON coupon_usage(coupon_id);
CREATE INDEX idx_coupon_usage_user_id ON coupon_usage(user_id);
CREATE INDEX idx_coupon_usage_order_id ON coupon_usage(order_id);
CREATE INDEX idx_coupon_usage_vendor_id ON coupon_usage(vendor_id);
CREATE INDEX idx_coupon_usage_used_at ON coupon_usage(used_at);

-- Create trigger to update updated_at column
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_coupons_updated_at 
    BEFORE UPDATE ON coupons 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_coupon_usage_updated_at 
    BEFORE UPDATE ON coupon_usage 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();