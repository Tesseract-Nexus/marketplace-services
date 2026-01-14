-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create categories table
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    created_by_id VARCHAR(255) NOT NULL,
    updated_by_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    image_url TEXT,
    banner_url TEXT,
    parent_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    level INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT true,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    tier VARCHAR(50),
    tags JSONB,
    seo_title TEXT,
    seo_description TEXT,
    seo_keywords JSONB,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create category_audit table
CREATE TABLE category_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,
    fields_changed JSONB,
    old_values JSONB,
    new_values JSONB,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    ip_address VARCHAR(45)
);

-- Create indexes for categories table
CREATE INDEX idx_categories_tenant_id ON categories(tenant_id);
CREATE UNIQUE INDEX idx_categories_tenant_slug ON categories(tenant_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_categories_parent_id ON categories(parent_id);
CREATE INDEX idx_categories_status ON categories(status);
CREATE INDEX idx_categories_level ON categories(level);
CREATE INDEX idx_categories_position ON categories(position);
CREATE INDEX idx_categories_is_active ON categories(is_active);
CREATE INDEX idx_categories_deleted_at ON categories(deleted_at);

-- Create GIN indexes for JSONB columns
CREATE INDEX idx_categories_tags ON categories USING GIN (tags);
CREATE INDEX idx_categories_seo_keywords ON categories USING GIN (seo_keywords);
CREATE INDEX idx_categories_metadata ON categories USING GIN (metadata);

-- Create indexes for category_audit table
CREATE INDEX idx_category_audit_category_id ON category_audit(category_id);
CREATE INDEX idx_category_audit_user_id ON category_audit(user_id);
CREATE INDEX idx_category_audit_action ON category_audit(action);
CREATE INDEX idx_category_audit_timestamp ON category_audit(timestamp);

-- Create trigger to update updated_at column
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_categories_updated_at 
    BEFORE UPDATE ON categories 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Create trigger to automatically calculate level based on parent hierarchy
CREATE OR REPLACE FUNCTION calculate_category_level()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.parent_id IS NULL THEN
        NEW.level = 0;
    ELSE
        SELECT level + 1 INTO NEW.level 
        FROM categories 
        WHERE id = NEW.parent_id;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER calculate_category_level_trigger
    BEFORE INSERT OR UPDATE ON categories
    FOR EACH ROW 
    EXECUTE FUNCTION calculate_category_level();

-- Create function to prevent circular references in parent-child relationships
CREATE OR REPLACE FUNCTION check_category_parent()
RETURNS TRIGGER AS $$
BEGIN
    -- Prevent self-reference
    IF NEW.parent_id = NEW.id THEN
        RAISE EXCEPTION 'Category cannot be its own parent';
    END IF;
    
    -- Prevent circular reference by checking if NEW.id exists in parent hierarchy of NEW.parent_id
    IF NEW.parent_id IS NOT NULL THEN
        WITH RECURSIVE parent_hierarchy AS (
            SELECT id, parent_id, 1 as depth
            FROM categories 
            WHERE id = NEW.parent_id
            
            UNION ALL
            
            SELECT c.id, c.parent_id, ph.depth + 1
            FROM categories c
            INNER JOIN parent_hierarchy ph ON c.id = ph.parent_id
            WHERE ph.depth < 10  -- Prevent infinite recursion
        )
        SELECT 1 FROM parent_hierarchy WHERE id = NEW.id;
        
        IF FOUND THEN
            RAISE EXCEPTION 'Circular reference detected in category hierarchy';
        END IF;
    END IF;
    
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER check_category_parent_trigger
    BEFORE INSERT OR UPDATE ON categories
    FOR EACH ROW 
    EXECUTE FUNCTION check_category_parent();