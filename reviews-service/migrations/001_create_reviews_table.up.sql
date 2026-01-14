-- Create UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create reviews table
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    application_id VARCHAR(255) NOT NULL,
    target_id VARCHAR(255) NOT NULL,
    target_type VARCHAR(100) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    title TEXT,
    content TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    type VARCHAR(50) NOT NULL,
    visibility VARCHAR(50) NOT NULL DEFAULT 'PUBLIC',
    ratings JSONB,
    comments JSONB,
    reactions JSONB,
    media JSONB,
    tags JSONB,
    helpful_count INTEGER DEFAULT 0,
    report_count INTEGER DEFAULT 0,
    featured BOOLEAN DEFAULT FALSE,
    verified_purchase BOOLEAN DEFAULT FALSE,
    language VARCHAR(10),
    ip_address INET,
    user_agent TEXT,
    spam_score DECIMAL(3,2),
    sentiment_score DECIMAL(3,2),
    moderation_notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    metadata JSONB
);

-- Create indexes for performance
CREATE INDEX idx_reviews_tenant_id ON reviews(tenant_id);
CREATE INDEX idx_reviews_target ON reviews(tenant_id, target_type, target_id);
CREATE INDEX idx_reviews_user_id ON reviews(tenant_id, user_id);
CREATE INDEX idx_reviews_status ON reviews(tenant_id, status);
CREATE INDEX idx_reviews_type ON reviews(tenant_id, type);
CREATE INDEX idx_reviews_featured ON reviews(tenant_id, featured);
CREATE INDEX idx_reviews_created_at ON reviews(created_at);
CREATE INDEX idx_reviews_published_at ON reviews(published_at);
CREATE INDEX idx_reviews_deleted_at ON reviews(deleted_at);

-- Create GIN indexes for JSONB columns
CREATE INDEX idx_reviews_ratings ON reviews USING GIN(ratings);
CREATE INDEX idx_reviews_tags ON reviews USING GIN(tags);
CREATE INDEX idx_reviews_metadata ON reviews USING GIN(metadata);

-- Create composite indexes for common queries
CREATE INDEX idx_reviews_status_created ON reviews(tenant_id, status, created_at DESC);
CREATE INDEX idx_reviews_target_status ON reviews(tenant_id, target_type, target_id, status);

-- Add constraints
ALTER TABLE reviews ADD CONSTRAINT chk_reviews_status 
    CHECK (status IN ('DRAFT', 'PENDING', 'APPROVED', 'REJECTED', 'FLAGGED', 'ARCHIVED'));

ALTER TABLE reviews ADD CONSTRAINT chk_reviews_type 
    CHECK (type IN ('PRODUCT', 'SERVICE', 'VENDOR', 'EXPERIENCE'));

ALTER TABLE reviews ADD CONSTRAINT chk_reviews_visibility 
    CHECK (visibility IN ('PUBLIC', 'PRIVATE', 'INTERNAL'));

ALTER TABLE reviews ADD CONSTRAINT chk_reviews_spam_score 
    CHECK (spam_score >= 0.0 AND spam_score <= 1.0);

ALTER TABLE reviews ADD CONSTRAINT chk_reviews_sentiment_score 
    CHECK (sentiment_score >= -1.0 AND sentiment_score <= 1.0);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_reviews_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_reviews_updated_at_trigger
    BEFORE UPDATE ON reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_reviews_updated_at();