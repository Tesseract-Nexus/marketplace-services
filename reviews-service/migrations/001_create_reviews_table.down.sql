-- Drop trigger and function
DROP TRIGGER IF EXISTS update_reviews_updated_at_trigger ON reviews;
DROP FUNCTION IF EXISTS update_reviews_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_reviews_target_status;
DROP INDEX IF EXISTS idx_reviews_status_created;
DROP INDEX IF EXISTS idx_reviews_metadata;
DROP INDEX IF EXISTS idx_reviews_tags;
DROP INDEX IF EXISTS idx_reviews_ratings;
DROP INDEX IF EXISTS idx_reviews_deleted_at;
DROP INDEX IF EXISTS idx_reviews_published_at;
DROP INDEX IF EXISTS idx_reviews_created_at;
DROP INDEX IF EXISTS idx_reviews_featured;
DROP INDEX IF EXISTS idx_reviews_type;
DROP INDEX IF EXISTS idx_reviews_status;
DROP INDEX IF EXISTS idx_reviews_user_id;
DROP INDEX IF EXISTS idx_reviews_target;
DROP INDEX IF EXISTS idx_reviews_tenant_id;

-- Drop table
DROP TABLE IF EXISTS reviews;