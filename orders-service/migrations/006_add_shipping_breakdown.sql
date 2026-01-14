-- Migration 006: Add shipping rate breakdown fields
-- These fields store the breakdown of shipping costs for admin transparency:
-- baseRate: Original carrier rate before markup
-- markupAmount: Markup amount applied
-- markupPercent: Markup percentage applied (e.g., 10 for 10%)

-- Add shipping breakdown columns to order_shippings table
ALTER TABLE order_shippings
ADD COLUMN IF NOT EXISTS base_rate DECIMAL(10,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS markup_amount DECIMAL(10,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS markup_percent DECIMAL(5,2) DEFAULT 0;

-- Add comment for documentation
COMMENT ON COLUMN order_shippings.base_rate IS 'Original carrier rate before markup';
COMMENT ON COLUMN order_shippings.markup_amount IS 'Markup amount applied to carrier rate';
COMMENT ON COLUMN order_shippings.markup_percent IS 'Markup percentage applied (e.g., 10 for 10%)';
