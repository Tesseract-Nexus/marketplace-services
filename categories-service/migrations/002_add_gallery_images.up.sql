-- Migration: Add gallery images support to categories
-- Categories will support up to 3 images (icon, banner, gallery item)
-- This follows the same JSONB pattern used by products

-- Add images JSONB column for gallery (same pattern as products)
ALTER TABLE categories ADD COLUMN IF NOT EXISTS images JSONB DEFAULT '[]'::jsonb;

-- Add GIN index for efficient JSONB querying
CREATE INDEX IF NOT EXISTS idx_categories_images ON categories USING GIN (images);

-- Add comment for documentation
COMMENT ON COLUMN categories.images IS 'Array of category images (max 3) with id, url, altText, position, width, height';

-- Note: Keep existing image_url and banner_url for backward compatibility
-- These can be deprecated in future releases once clients migrate to images array
