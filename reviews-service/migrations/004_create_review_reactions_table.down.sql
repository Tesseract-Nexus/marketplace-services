-- Drop triggers first
DROP TRIGGER IF EXISTS update_review_reaction_counts_trigger ON review_reactions;
DROP TRIGGER IF EXISTS update_review_reactions_updated_at_trigger ON review_reactions;

-- Drop functions
DROP FUNCTION IF EXISTS update_review_reaction_counts();
DROP FUNCTION IF EXISTS update_review_reactions_updated_at();

-- Drop the table
DROP TABLE IF EXISTS review_reactions;

-- Remove the not_helpful_count column from reviews
ALTER TABLE reviews DROP COLUMN IF EXISTS not_helpful_count;
