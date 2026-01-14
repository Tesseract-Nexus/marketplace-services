-- Rollback: Remove gallery images support from categories

-- Drop the GIN index first
DROP INDEX IF EXISTS idx_categories_images;

-- Drop the images column
ALTER TABLE categories DROP COLUMN IF EXISTS images;
