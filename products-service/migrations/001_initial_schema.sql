-- Products Service Database Schema
-- This migration creates the initial schema for products, product variants, and categories

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Categories table
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE,
    description TEXT,
    parent_id VARCHAR(255),
    image_url TEXT,
    is_active BOOLEAN DEFAULT true,
    sort_order INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Products table
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255) NOT NULL,
    category_id VARCHAR(255) NOT NULL,
    created_by_id VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE,
    sku VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    price VARCHAR(50) NOT NULL,
    compare_price VARCHAR(50),
    cost_price VARCHAR(50),
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    inventory_status VARCHAR(50),
    quantity INTEGER,
    min_order_qty INTEGER,
    max_order_qty INTEGER,
    low_stock_threshold INTEGER,
    weight VARCHAR(50),
    dimensions JSONB,
    search_keywords TEXT,
    tags JSONB,
    currency_code VARCHAR(3),
    sync_status VARCHAR(50),
    synced_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    offline_id VARCHAR(255),
    localizations JSONB,
    attributes JSONB,
    images JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    metadata JSONB
);

-- Product variants table
CREATE TABLE product_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL,
    sku VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    price VARCHAR(50) NOT NULL,
    compare_price VARCHAR(50),
    cost_price VARCHAR(50),
    quantity INTEGER NOT NULL DEFAULT 0,
    low_stock_threshold INTEGER,
    weight VARCHAR(50),
    dimensions JSONB,
    inventory_status VARCHAR(50),
    sync_status VARCHAR(50),
    version INTEGER DEFAULT 1,
    offline_id VARCHAR(255),
    images JSONB,
    attributes JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for categories
CREATE INDEX idx_categories_tenant_id ON categories(tenant_id);
CREATE INDEX idx_categories_parent_id ON categories(parent_id);
CREATE INDEX idx_categories_is_active ON categories(is_active);
CREATE INDEX idx_categories_deleted_at ON categories(deleted_at);

-- Indexes for products
CREATE INDEX idx_products_tenant_id ON products(tenant_id);
CREATE INDEX idx_products_vendor_id ON products(vendor_id);
CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_products_status ON products(status);
CREATE INDEX idx_products_inventory_status ON products(inventory_status);
CREATE INDEX idx_products_created_at ON products(created_at);
CREATE INDEX idx_products_deleted_at ON products(deleted_at);
CREATE INDEX idx_products_name ON products USING gin(to_tsvector('english', name));
CREATE INDEX idx_products_description ON products USING gin(to_tsvector('english', description));
CREATE INDEX idx_products_search_keywords ON products USING gin(to_tsvector('english', search_keywords));

-- Indexes for product variants
CREATE INDEX idx_product_variants_product_id ON product_variants(product_id);
CREATE INDEX idx_product_variants_inventory_status ON product_variants(inventory_status);
CREATE INDEX idx_product_variants_deleted_at ON product_variants(deleted_at);

-- Foreign key constraints
ALTER TABLE product_variants ADD CONSTRAINT fk_product_variants_product_id 
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE;

-- Functions for updating timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updating timestamps
CREATE TRIGGER update_categories_updated_at BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_product_variants_updated_at BEFORE UPDATE ON product_variants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- NOTE: Sample/test data has been removed from this schema migration.
-- Categories and products are created per-tenant during onboarding or via the API.
-- For development test data, use SKIP_TEST_DATA=false with run-migrations.sh