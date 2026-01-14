-- Tax Configuration Tables
-- Supports location-based tax rates, product categories, exemptions, and reporting

-- Tax Jurisdictions (Countries, States, Counties, Cities)
CREATE TABLE IF NOT EXISTS tax_jurisdictions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- COUNTRY, STATE, COUNTY, CITY, ZIP
    code VARCHAR(50) NOT NULL, -- ISO code or zip code
    parent_id UUID REFERENCES tax_jurisdictions(id),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_jurisdiction UNIQUE(tenant_id, type, code)
);

CREATE INDEX idx_tax_jurisdictions_tenant ON tax_jurisdictions(tenant_id);
CREATE INDEX idx_tax_jurisdictions_code ON tax_jurisdictions(code);
CREATE INDEX idx_tax_jurisdictions_parent ON tax_jurisdictions(parent_id);

-- Tax Rates
CREATE TABLE IF NOT EXISTS tax_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    jurisdiction_id UUID NOT NULL REFERENCES tax_jurisdictions(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL, -- e.g., "CA State Sales Tax", "San Francisco City Tax"
    rate DECIMAL(10, 6) NOT NULL, -- 8.5% = 0.085000
    tax_type VARCHAR(50) NOT NULL, -- SALES, VAT, GST, CITY, COUNTY, STATE, SPECIAL
    priority INT DEFAULT 0, -- For compound tax calculation order

    -- Applicability
    applies_to_shipping BOOLEAN DEFAULT false,
    applies_to_products BOOLEAN DEFAULT true,

    -- Effective dates
    effective_from DATE NOT NULL,
    effective_to DATE,

    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tax_rates_tenant ON tax_rates(tenant_id);
CREATE INDEX idx_tax_rates_jurisdiction ON tax_rates(jurisdiction_id);
CREATE INDEX idx_tax_rates_effective ON tax_rates(effective_from, effective_to);

-- Product Tax Categories
CREATE TABLE IF NOT EXISTS product_tax_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL, -- e.g., "Clothing", "Food", "Electronics"
    description TEXT,
    tax_code VARCHAR(50), -- External tax code (e.g., TaxJar, Avalara)
    is_tax_exempt BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_tax_category UNIQUE(tenant_id, name)
);

CREATE INDEX idx_product_tax_categories_tenant ON product_tax_categories(tenant_id);

-- Tax Rate Overrides by Category
CREATE TABLE IF NOT EXISTS tax_rate_category_overrides (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tax_rate_id UUID NOT NULL REFERENCES tax_rates(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES product_tax_categories(id) ON DELETE CASCADE,
    override_rate DECIMAL(10, 6), -- NULL means exempt
    is_exempt BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_rate_category UNIQUE(tax_rate_id, category_id)
);

CREATE INDEX idx_tax_rate_overrides_rate ON tax_rate_category_overrides(tax_rate_id);
CREATE INDEX idx_tax_rate_overrides_category ON tax_rate_category_overrides(category_id);

-- Tax Exemption Certificates (for wholesale/resale customers)
CREATE TABLE IF NOT EXISTS tax_exemption_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    customer_id UUID NOT NULL, -- References customers table
    certificate_number VARCHAR(100) NOT NULL,
    certificate_type VARCHAR(50) NOT NULL, -- RESALE, GOVERNMENT, NON_PROFIT, DIPLOMATIC

    -- Jurisdiction scope
    jurisdiction_id UUID REFERENCES tax_jurisdictions(id),
    applies_to_all_jurisdictions BOOLEAN DEFAULT false,

    -- Validity
    issued_date DATE NOT NULL,
    expiry_date DATE,

    -- Document storage
    document_url VARCHAR(500),

    -- Status
    status VARCHAR(50) DEFAULT 'ACTIVE', -- ACTIVE, EXPIRED, REVOKED, PENDING
    verified_at TIMESTAMP,
    verified_by UUID,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_certificate UNIQUE(tenant_id, customer_id, certificate_number)
);

CREATE INDEX idx_tax_exemptions_tenant ON tax_exemption_certificates(tenant_id);
CREATE INDEX idx_tax_exemptions_customer ON tax_exemption_certificates(customer_id);
CREATE INDEX idx_tax_exemptions_status ON tax_exemption_certificates(status);

-- Tax Calculation Cache (for performance)
CREATE TABLE IF NOT EXISTS tax_calculation_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cache_key VARCHAR(255) NOT NULL, -- Hash of calculation inputs
    jurisdiction_ids UUID[],
    subtotal DECIMAL(12, 2) NOT NULL,
    shipping_amount DECIMAL(12, 2),
    tax_amount DECIMAL(12, 2) NOT NULL,
    tax_breakdown JSONB, -- Detailed breakdown by jurisdiction/rate

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,

    CONSTRAINT unique_cache_key UNIQUE(cache_key)
);

CREATE INDEX idx_tax_cache_key ON tax_calculation_cache(cache_key);
CREATE INDEX idx_tax_cache_expiry ON tax_calculation_cache(expires_at);

-- Tax Nexus (where business has tax obligation)
CREATE TABLE IF NOT EXISTS tax_nexus (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    jurisdiction_id UUID NOT NULL REFERENCES tax_jurisdictions(id),
    nexus_type VARCHAR(50) NOT NULL, -- PHYSICAL, ECONOMIC, AFFILIATE
    registration_number VARCHAR(100),
    effective_date DATE NOT NULL,
    notes TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT unique_nexus UNIQUE(tenant_id, jurisdiction_id)
);

CREATE INDEX idx_tax_nexus_tenant ON tax_nexus(tenant_id);
CREATE INDEX idx_tax_nexus_jurisdiction ON tax_nexus(jurisdiction_id);

-- Tax Reports (for compliance and filing)
CREATE TABLE IF NOT EXISTS tax_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    report_type VARCHAR(50) NOT NULL, -- MONTHLY, QUARTERLY, ANNUAL
    jurisdiction_id UUID REFERENCES tax_jurisdictions(id),

    -- Period
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,

    -- Totals
    total_sales DECIMAL(12, 2) NOT NULL DEFAULT 0,
    taxable_sales DECIMAL(12, 2) NOT NULL DEFAULT 0,
    exempt_sales DECIMAL(12, 2) NOT NULL DEFAULT 0,
    tax_collected DECIMAL(12, 2) NOT NULL DEFAULT 0,

    -- Breakdown
    tax_breakdown JSONB,

    -- Status
    status VARCHAR(50) DEFAULT 'DRAFT', -- DRAFT, FILED, PAID
    filed_at TIMESTAMP,
    payment_due_date DATE,
    paid_at TIMESTAMP,

    -- Document
    report_url VARCHAR(500),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tax_reports_tenant ON tax_reports(tenant_id);
CREATE INDEX idx_tax_reports_period ON tax_reports(period_start, period_end);
CREATE INDEX idx_tax_reports_jurisdiction ON tax_reports(jurisdiction_id);
CREATE INDEX idx_tax_reports_status ON tax_reports(status);

-- Comments
COMMENT ON TABLE tax_jurisdictions IS 'Tax jurisdictions (countries, states, cities, zip codes)';
COMMENT ON TABLE tax_rates IS 'Tax rates for each jurisdiction and type';
COMMENT ON TABLE product_tax_categories IS 'Product categories with different tax treatment';
COMMENT ON TABLE tax_rate_category_overrides IS 'Override tax rates for specific product categories';
COMMENT ON TABLE tax_exemption_certificates IS 'Customer tax exemption certificates (wholesale, non-profit, etc)';
COMMENT ON TABLE tax_calculation_cache IS 'Cache for tax calculations to improve performance';
COMMENT ON TABLE tax_nexus IS 'Locations where business has tax collection obligation';
COMMENT ON TABLE tax_reports IS 'Tax reports for compliance and filing';
