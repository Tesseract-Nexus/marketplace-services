-- Drop triggers
DROP TRIGGER IF EXISTS check_category_parent_trigger ON categories;
DROP TRIGGER IF EXISTS calculate_category_level_trigger ON categories;
DROP TRIGGER IF EXISTS update_categories_updated_at ON categories;

-- Drop functions
DROP FUNCTION IF EXISTS check_category_parent();
DROP FUNCTION IF EXISTS calculate_category_level();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_category_audit_timestamp;
DROP INDEX IF EXISTS idx_category_audit_action;
DROP INDEX IF EXISTS idx_category_audit_user_id;
DROP INDEX IF EXISTS idx_category_audit_category_id;

DROP INDEX IF EXISTS idx_categories_metadata;
DROP INDEX IF EXISTS idx_categories_seo_keywords;
DROP INDEX IF EXISTS idx_categories_tags;

DROP INDEX IF EXISTS idx_categories_deleted_at;
DROP INDEX IF EXISTS idx_categories_is_active;
DROP INDEX IF EXISTS idx_categories_position;
DROP INDEX IF EXISTS idx_categories_level;
DROP INDEX IF EXISTS idx_categories_status;
DROP INDEX IF EXISTS idx_categories_parent_id;
DROP INDEX IF EXISTS idx_categories_tenant_slug;
DROP INDEX IF EXISTS idx_categories_tenant_id;

-- Drop tables
DROP TABLE IF EXISTS category_audit;
DROP TABLE IF EXISTS categories;