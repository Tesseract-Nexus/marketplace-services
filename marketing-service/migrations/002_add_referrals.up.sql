-- Add referral_code column to customer_loyalties
ALTER TABLE customer_loyalties
ADD COLUMN IF NOT EXISTS referral_code VARCHAR(20) UNIQUE,
ADD COLUMN IF NOT EXISTS referred_by UUID;

CREATE INDEX IF NOT EXISTS idx_loyalty_referral_code ON customer_loyalties(referral_code);
CREATE INDEX IF NOT EXISTS idx_loyalty_referred_by ON customer_loyalties(referred_by);

-- Create referrals table to track referral relationships
CREATE TABLE IF NOT EXISTS referrals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(100) NOT NULL,

    -- Referrer (the person who referred)
    referrer_id UUID NOT NULL,
    referrer_loyalty_id UUID NOT NULL,

    -- Referred (the new customer)
    referred_id UUID NOT NULL,
    referred_loyalty_id UUID NOT NULL,

    -- Referral details
    referral_code VARCHAR(20) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',

    -- Bonus tracking
    referrer_bonus_points INTEGER DEFAULT 0,
    referred_bonus_points INTEGER DEFAULT 0,
    referrer_bonus_awarded_at TIMESTAMP,
    referred_bonus_awarded_at TIMESTAMP,

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_referral UNIQUE (tenant_id, referred_id)
);

CREATE INDEX IF NOT EXISTS idx_referrals_tenant ON referrals(tenant_id);
CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_id);
CREATE INDEX IF NOT EXISTS idx_referrals_referred ON referrals(referred_id);
CREATE INDEX IF NOT EXISTS idx_referrals_code ON referrals(referral_code);
CREATE INDEX IF NOT EXISTS idx_referrals_status ON referrals(status);
