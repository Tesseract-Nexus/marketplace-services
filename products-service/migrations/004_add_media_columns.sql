-- Migration: Add media columns to products and categories
-- This migration adds logo_url, banner_url, and videos columns for enhanced media support

-- Add media columns to products table
ALTER TABLE products ADD COLUMN IF NOT EXISTS logo_url VARCHAR(1000);
ALTER TABLE products ADD COLUMN IF NOT EXISTS banner_url VARCHAR(1000);
ALTER TABLE products ADD COLUMN IF NOT EXISTS videos JSONB DEFAULT '[]'::jsonb;

-- Add brand column if not exists (some deployments might be missing it)
ALTER TABLE products ADD COLUMN IF NOT EXISTS brand VARCHAR(255);

-- Add banner_url to categories table
ALTER TABLE categories ADD COLUMN IF NOT EXISTS banner_url VARCHAR(1000);

-- Add position column to categories for ordering
ALTER TABLE categories ADD COLUMN IF NOT EXISTS position INTEGER DEFAULT 0;

-- Add comments for documentation
COMMENT ON COLUMN products.logo_url IS 'Product logo/icon URL (512x512 max)';
COMMENT ON COLUMN products.banner_url IS 'Product banner URL (1920x480 max)';
COMMENT ON COLUMN products.videos IS 'Array of promotional videos (max 2) with id, url, title, thumbnailUrl, duration, position';
COMMENT ON COLUMN categories.banner_url IS 'Category banner URL for storefront display';
COMMENT ON COLUMN categories.position IS 'Display order position for category sorting';

-- Create index for category position ordering
CREATE INDEX IF NOT EXISTS idx_categories_position ON categories(position);
CREATE INDEX IF NOT EXISTS idx_categories_tenant_position ON categories(tenant_id, position);
