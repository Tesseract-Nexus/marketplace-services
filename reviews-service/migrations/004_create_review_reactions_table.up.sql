-- Add not_helpful_count column to reviews table
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS not_helpful_count INTEGER DEFAULT 0;

-- Create review_reactions table for tracking individual user reactions
CREATE TABLE IF NOT EXISTS review_reactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    review_id UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    reaction_type VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one reaction per user per review (user can only vote once)
    CONSTRAINT uq_review_reactions_user_review UNIQUE (review_id, user_id)
);

-- Add constraint for reaction types
ALTER TABLE review_reactions ADD CONSTRAINT chk_reaction_type
    CHECK (reaction_type IN ('HELPFUL', 'NOT_HELPFUL'));

-- Create indexes for performance
CREATE INDEX idx_review_reactions_tenant_id ON review_reactions(tenant_id);
CREATE INDEX idx_review_reactions_review_id ON review_reactions(review_id);
CREATE INDEX idx_review_reactions_user_id ON review_reactions(user_id);
CREATE INDEX idx_review_reactions_type ON review_reactions(reaction_type);

-- Create composite index for common queries
CREATE INDEX idx_review_reactions_review_type ON review_reactions(review_id, reaction_type);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_review_reactions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_review_reactions_updated_at_trigger
    BEFORE UPDATE ON review_reactions
    FOR EACH ROW
    EXECUTE FUNCTION update_review_reactions_updated_at();

-- Create function to update review counts when reactions change
CREATE OR REPLACE FUNCTION update_review_reaction_counts()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        -- Increment the appropriate count
        IF NEW.reaction_type = 'HELPFUL' THEN
            UPDATE reviews SET helpful_count = helpful_count + 1 WHERE id = NEW.review_id;
        ELSIF NEW.reaction_type = 'NOT_HELPFUL' THEN
            UPDATE reviews SET not_helpful_count = not_helpful_count + 1 WHERE id = NEW.review_id;
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        -- Decrement the appropriate count
        IF OLD.reaction_type = 'HELPFUL' THEN
            UPDATE reviews SET helpful_count = GREATEST(0, helpful_count - 1) WHERE id = OLD.review_id;
        ELSIF OLD.reaction_type = 'NOT_HELPFUL' THEN
            UPDATE reviews SET not_helpful_count = GREATEST(0, not_helpful_count - 1) WHERE id = OLD.review_id;
        END IF;
        RETURN OLD;
    ELSIF TG_OP = 'UPDATE' THEN
        -- Handle reaction type change
        IF OLD.reaction_type != NEW.reaction_type THEN
            -- Decrement old count
            IF OLD.reaction_type = 'HELPFUL' THEN
                UPDATE reviews SET helpful_count = GREATEST(0, helpful_count - 1) WHERE id = OLD.review_id;
            ELSIF OLD.reaction_type = 'NOT_HELPFUL' THEN
                UPDATE reviews SET not_helpful_count = GREATEST(0, not_helpful_count - 1) WHERE id = OLD.review_id;
            END IF;
            -- Increment new count
            IF NEW.reaction_type = 'HELPFUL' THEN
                UPDATE reviews SET helpful_count = helpful_count + 1 WHERE id = NEW.review_id;
            ELSIF NEW.reaction_type = 'NOT_HELPFUL' THEN
                UPDATE reviews SET not_helpful_count = not_helpful_count + 1 WHERE id = NEW.review_id;
            END IF;
        END IF;
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update counts
CREATE TRIGGER update_review_reaction_counts_trigger
    AFTER INSERT OR UPDATE OR DELETE ON review_reactions
    FOR EACH ROW
    EXECUTE FUNCTION update_review_reaction_counts();
