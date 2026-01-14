-- Ad Billing & Commission Tables
-- Migration 005: Create tables for ad campaign payments and commission tracking

-- =============================================================================
-- Ad Commission Tiers Table
-- Defines commission rates based on campaign duration
-- =============================================================================
CREATE TABLE IF NOT EXISTS ad_commission_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    min_days INTEGER NOT NULL,
    max_days INTEGER,
    commission_rate DECIMAL(5,4) NOT NULL, -- e.g., 0.019 for 1.9%
    tax_inclusive BOOLEAN DEFAULT TRUE,
    is_active BOOLEAN DEFAULT TRUE,
    priority INTEGER DEFAULT 0,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ad_commission_tiers_tenant ON ad_commission_tiers(tenant_id);
CREATE INDEX idx_ad_commission_tiers_active ON ad_commission_tiers(tenant_id, is_active);

-- Insert default global commission tiers
-- Shorter campaigns = lower commission, Longer campaigns = higher commission
INSERT INTO ad_commission_tiers (tenant_id, name, min_days, max_days, commission_rate, priority, description) VALUES
('GLOBAL', 'Express (1-6 days)', 1, 6, 0.019, 4, 'Short-term campaigns with minimum 1.9% commission'),
('GLOBAL', 'Short-term (7-29 days)', 7, 29, 0.029, 3, 'Standard short campaigns with 2.9% commission'),
('GLOBAL', 'Medium-term (30-89 days)', 30, 89, 0.039, 2, 'Monthly campaigns with 3.9% commission'),
('GLOBAL', 'Long-term (90+ days)', 90, NULL, 0.049, 1, 'Extended campaigns with 4.9% commission');

-- =============================================================================
-- Ad Campaign Payments Table
-- Tracks payments for ad campaigns (both direct and sponsored)
-- =============================================================================
CREATE TABLE IF NOT EXISTS ad_campaign_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id UUID NOT NULL,
    campaign_id UUID NOT NULL,

    -- Payment type: DIRECT (full budget upfront) or SPONSORED (commission-based)
    payment_type VARCHAR(20) NOT NULL CHECK (payment_type IN ('DIRECT', 'SPONSORED')),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PROCESSING', 'PAID', 'FAILED', 'REFUNDED', 'CANCELLED')),

    -- Amounts
    budget_amount DECIMAL(12,2) NOT NULL,
    commission_rate DECIMAL(5,4),
    commission_amount DECIMAL(12,2) DEFAULT 0,
    tax_amount DECIMAL(12,2) DEFAULT 0,
    total_amount DECIMAL(12,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',

    -- Commission tier reference (for SPONSORED payments)
    commission_tier_id UUID REFERENCES ad_commission_tiers(id),
    campaign_days INTEGER,

    -- Link to payment transaction
    payment_transaction_id UUID REFERENCES payment_transactions(id),

    -- Gateway info
    gateway_type VARCHAR(50),
    gateway_transaction_id VARCHAR(255),

    -- Timestamps
    paid_at TIMESTAMP,
    refunded_at TIMESTAMP,

    -- Metadata for additional info
    metadata JSONB,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ad_campaign_payments_tenant ON ad_campaign_payments(tenant_id);
CREATE INDEX idx_ad_campaign_payments_vendor ON ad_campaign_payments(vendor_id);
CREATE INDEX idx_ad_campaign_payments_campaign ON ad_campaign_payments(campaign_id);
CREATE INDEX idx_ad_campaign_payments_status ON ad_campaign_payments(status);
CREATE INDEX idx_ad_campaign_payments_type ON ad_campaign_payments(payment_type);
CREATE INDEX idx_ad_campaign_payments_created ON ad_campaign_payments(created_at);

-- =============================================================================
-- Ad Billing Invoices Table
-- For periodic billing of sponsored ads (if applicable)
-- =============================================================================
CREATE TABLE IF NOT EXISTS ad_billing_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id UUID NOT NULL,
    invoice_number VARCHAR(50) NOT NULL UNIQUE,

    -- Billing period
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'INVOICED', 'PAID', 'OVERDUE', 'CANCELLED')),

    -- Amounts
    total_spend DECIMAL(12,2) NOT NULL,
    commission_rate DECIMAL(5,4) NOT NULL,
    commission_amount DECIMAL(12,2) NOT NULL,
    tax_amount DECIMAL(12,2) DEFAULT 0,
    total_due DECIMAL(12,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',

    -- Payment info
    due_date TIMESTAMP NOT NULL,
    paid_at TIMESTAMP,
    payment_transaction_id UUID REFERENCES payment_transactions(id),

    -- Line items breakdown (campaigns included in this invoice)
    line_items JSONB,

    -- Metadata
    metadata JSONB,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ad_billing_invoices_tenant ON ad_billing_invoices(tenant_id);
CREATE INDEX idx_ad_billing_invoices_vendor ON ad_billing_invoices(vendor_id);
CREATE INDEX idx_ad_billing_invoices_status ON ad_billing_invoices(status);
CREATE INDEX idx_ad_billing_invoices_due ON ad_billing_invoices(due_date);

-- =============================================================================
-- Ad Revenue Ledger Table
-- Tracks all ad revenue transactions for the platform
-- =============================================================================
CREATE TABLE IF NOT EXISTS ad_revenue_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id UUID NOT NULL,
    campaign_id UUID NOT NULL,

    -- Entry type: PAYMENT, SPEND, REFUND, ADJUSTMENT
    entry_type VARCHAR(30) NOT NULL CHECK (entry_type IN ('PAYMENT', 'SPEND', 'REFUND', 'ADJUSTMENT', 'COMMISSION')),

    -- Amounts
    amount DECIMAL(12,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',

    -- Running balance (per vendor per tenant)
    balance_after DECIMAL(12,2) NOT NULL,

    -- References
    campaign_payment_id UUID REFERENCES ad_campaign_payments(id),
    invoice_id UUID REFERENCES ad_billing_invoices(id),
    payment_transaction_id UUID REFERENCES payment_transactions(id),

    -- Description
    description TEXT,

    -- Metadata
    metadata JSONB,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ad_revenue_ledger_tenant ON ad_revenue_ledger(tenant_id);
CREATE INDEX idx_ad_revenue_ledger_vendor ON ad_revenue_ledger(vendor_id);
CREATE INDEX idx_ad_revenue_ledger_campaign ON ad_revenue_ledger(campaign_id);
CREATE INDEX idx_ad_revenue_ledger_type ON ad_revenue_ledger(entry_type);
CREATE INDEX idx_ad_revenue_ledger_created ON ad_revenue_ledger(created_at);

-- =============================================================================
-- Ad Vendor Balance Table
-- Tracks current balance for each vendor (for prepaid ad credits)
-- =============================================================================
CREATE TABLE IF NOT EXISTS ad_vendor_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id UUID NOT NULL,

    -- Balance
    current_balance DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_deposited DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_spent DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_refunded DECIMAL(12,2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'USD',

    -- Status
    is_active BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one balance per vendor per tenant
    UNIQUE(tenant_id, vendor_id)
);

CREATE INDEX idx_ad_vendor_balances_tenant ON ad_vendor_balances(tenant_id);
CREATE INDEX idx_ad_vendor_balances_vendor ON ad_vendor_balances(vendor_id);

-- =============================================================================
-- Helper function for updated_at timestamps
-- =============================================================================
CREATE OR REPLACE FUNCTION update_ad_billing_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers for updated_at
CREATE TRIGGER trigger_ad_commission_tiers_updated_at
    BEFORE UPDATE ON ad_commission_tiers
    FOR EACH ROW EXECUTE FUNCTION update_ad_billing_updated_at();

CREATE TRIGGER trigger_ad_campaign_payments_updated_at
    BEFORE UPDATE ON ad_campaign_payments
    FOR EACH ROW EXECUTE FUNCTION update_ad_billing_updated_at();

CREATE TRIGGER trigger_ad_billing_invoices_updated_at
    BEFORE UPDATE ON ad_billing_invoices
    FOR EACH ROW EXECUTE FUNCTION update_ad_billing_updated_at();

CREATE TRIGGER trigger_ad_vendor_balances_updated_at
    BEFORE UPDATE ON ad_vendor_balances
    FOR EACH ROW EXECUTE FUNCTION update_ad_billing_updated_at();
