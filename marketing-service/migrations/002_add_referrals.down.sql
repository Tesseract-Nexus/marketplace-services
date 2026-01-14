-- Drop referrals table
DROP TABLE IF EXISTS referrals;

-- Remove referral columns from customer_loyalties
ALTER TABLE customer_loyalties
DROP COLUMN IF EXISTS referral_code,
DROP COLUMN IF EXISTS referred_by;
