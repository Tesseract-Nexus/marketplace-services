-- Seed Tax Data for Testing
-- Includes common US jurisdictions and tax rates

-- United States
INSERT INTO tax_jurisdictions (id, tenant_id, name, type, code, parent_id, is_active) VALUES
('00000000-0000-0000-0000-000000000001', 'test-tenant', 'United States', 'COUNTRY', 'US', NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

-- California
INSERT INTO tax_jurisdictions (id, tenant_id, name, type, code, parent_id, is_active) VALUES
('10000000-0000-0000-0000-000000000001', 'test-tenant', 'California', 'STATE', 'CA', '00000000-0000-0000-0000-000000000001', true),
('10000000-0000-0000-0000-000000000002', 'test-tenant', 'San Francisco', 'CITY', 'SF', '10000000-0000-0000-0000-000000000001', true),
('10000000-0000-0000-0000-000000000003', 'test-tenant', 'Los Angeles', 'CITY', 'LA', '10000000-0000-0000-0000-000000000001', true),
('10000000-0000-0000-0000-000000000004', 'test-tenant', 'San Diego', 'CITY', 'SD', '10000000-0000-0000-0000-000000000001', true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

-- New York
INSERT INTO tax_jurisdictions (id, tenant_id, name, type, code, parent_id, is_active) VALUES
('20000000-0000-0000-0000-000000000001', 'test-tenant', 'New York', 'STATE', 'NY', '00000000-0000-0000-0000-000000000001', true),
('20000000-0000-0000-0000-000000000002', 'test-tenant', 'New York City', 'CITY', 'NYC', '20000000-0000-0000-0000-000000000001', true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

-- Texas
INSERT INTO tax_jurisdictions (id, tenant_id, name, type, code, parent_id, is_active) VALUES
('30000000-0000-0000-0000-000000000001', 'test-tenant', 'Texas', 'STATE', 'TX', '00000000-0000-0000-0000-000000000001', true),
('30000000-0000-0000-0000-000000000002', 'test-tenant', 'Austin', 'CITY', 'ATX', '30000000-0000-0000-0000-000000000001', true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

-- Florida (no state income tax, but has sales tax)
INSERT INTO tax_jurisdictions (id, tenant_id, name, type, code, parent_id, is_active) VALUES
('40000000-0000-0000-0000-000000000001', 'test-tenant', 'Florida', 'STATE', 'FL', '00000000-0000-0000-0000-000000000001', true),
('40000000-0000-0000-0000-000000000002', 'test-tenant', 'Miami', 'CITY', 'MIA', '40000000-0000-0000-0000-000000000001', true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

-- Tax Rates

-- California State Tax: 7.25%
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from) VALUES
('test-tenant', '10000000-0000-0000-0000-000000000001', 'CA State Sales Tax', 0.0725, 'STATE', 1, false, true, '2020-01-01');

-- San Francisco: 8.625% total (7.25% state + 1.375% local)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from) VALUES
('test-tenant', '10000000-0000-0000-0000-000000000002', 'San Francisco Sales Tax', 0.01375, 'CITY', 2, false, true, '2020-01-01');

-- Los Angeles: 9.5% total (7.25% state + 2.25% local)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from) VALUES
('test-tenant', '10000000-0000-0000-0000-000000000003', 'Los Angeles Sales Tax', 0.0225, 'CITY', 2, false, true, '2020-01-01');

-- New York State Tax: 4%
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from) VALUES
('test-tenant', '20000000-0000-0000-0000-000000000001', 'NY State Sales Tax', 0.04, 'STATE', 1, false, true, '2020-01-01');

-- NYC: 8.875% total (4% state + 4.5% city + 0.375% MTA)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from) VALUES
('test-tenant', '20000000-0000-0000-0000-000000000002', 'NYC Sales Tax', 0.045, 'CITY', 2, false, true, '2020-01-01'),
('test-tenant', '20000000-0000-0000-0000-000000000002', 'MTA Surcharge', 0.00375, 'SPECIAL', 3, false, true, '2020-01-01');

-- Texas State Tax: 6.25%
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from) VALUES
('test-tenant', '30000000-0000-0000-0000-000000000001', 'TX State Sales Tax', 0.0625, 'STATE', 1, false, true, '2020-01-01');

-- Austin: 8.25% total (6.25% state + 2% local)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from) VALUES
('test-tenant', '30000000-0000-0000-0000-000000000002', 'Austin Sales Tax', 0.02, 'CITY', 2, false, true, '2020-01-01');

-- Florida State Tax: 6%
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from) VALUES
('test-tenant', '40000000-0000-0000-0000-000000000001', 'FL State Sales Tax', 0.06, 'STATE', 1, false, true, '2020-01-01');

-- Miami: 7% total (6% state + 1% county)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from) VALUES
('test-tenant', '40000000-0000-0000-0000-000000000002', 'Miami-Dade County Tax', 0.01, 'COUNTY', 2, false, true, '2020-01-01');

-- Product Tax Categories

INSERT INTO product_tax_categories (tenant_id, name, description, tax_code, is_tax_exempt) VALUES
('test-tenant', 'General Merchandise', 'Standard taxable goods', 'P0000000', false),
('test-tenant', 'Clothing', 'Apparel and accessories', 'P0001000', false),
('test-tenant', 'Food & Groceries', 'Unprepared food items', 'P0002000', true),
('test-tenant', 'Electronics', 'Consumer electronics', 'P0003000', false),
('test-tenant', 'Books & Media', 'Books, magazines, digital media', 'P0004000', false),
('test-tenant', 'Medical Supplies', 'OTC and prescription items', 'P0005000', true),
('test-tenant', 'Software (Digital)', 'Downloadable software', 'P0006000', false),
('test-tenant', 'Services', 'Professional and personal services', 'S0000000', false)
ON CONFLICT (tenant_id, name) DO NOTHING;

-- Tax Exemption Certificate Example
INSERT INTO tax_exemption_certificates (tenant_id, customer_id, certificate_number, certificate_type, applies_to_all_jurisdictions, issued_date, status) VALUES
('test-tenant', '11111111-1111-1111-1111-111111111111', 'RESALE-001-2024', 'RESALE', true, '2024-01-01', 'ACTIVE')
ON CONFLICT (tenant_id, customer_id, certificate_number) DO NOTHING;

-- Tax Nexus (where business must collect tax)
INSERT INTO tax_nexus (tenant_id, jurisdiction_id, nexus_type, registration_number, effective_date, is_active) VALUES
('test-tenant', '10000000-0000-0000-0000-000000000001', 'PHYSICAL', 'CA-123456789', '2020-01-01', true),
('test-tenant', '20000000-0000-0000-0000-000000000001', 'ECONOMIC', 'NY-987654321', '2021-01-01', true),
('test-tenant', '30000000-0000-0000-0000-000000000001', 'ECONOMIC', 'TX-456789123', '2021-06-01', true)
ON CONFLICT (tenant_id, jurisdiction_id) DO NOTHING;

-- Comments
COMMENT ON TABLE tax_jurisdictions IS 'Seeded with US, CA, NY, TX, FL jurisdictions for testing';
COMMENT ON TABLE tax_rates IS 'Seeded with realistic 2024 US sales tax rates';
COMMENT ON TABLE product_tax_categories IS 'Common product categories with tax treatment';
