-- Migration: Seed Global Tax Data for India, EU, Canada, Australia, and other regions
-- This migration adds comprehensive tax jurisdictions and rates for global commerce
-- Run this migration to enable multi-country tax calculations

-- =============================================================================
-- INDIA GST (Goods and Services Tax)
-- =============================================================================
-- GST Rates: 0%, 5%, 12%, 18%, 28% + Cess for luxury goods
-- CGST + SGST for intrastate, IGST for interstate transactions

-- India Country
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'India', 'COUNTRY', 'IN', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

-- Indian States and Union Territories with GST State Codes
WITH india_country AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'IN'
)
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active)
SELECT 'global', data.name, 'STATE', data.code, data.state_code, india_country.id, true
FROM india_country
CROSS JOIN (
    VALUES
    -- States
    ('Andhra Pradesh', 'AP', '37'),
    ('Arunachal Pradesh', 'AR', '12'),
    ('Assam', 'AS', '18'),
    ('Bihar', 'BR', '10'),
    ('Chhattisgarh', 'CG', '22'),
    ('Goa', 'GA', '30'),
    ('Gujarat', 'GJ', '24'),
    ('Haryana', 'HR', '06'),
    ('Himachal Pradesh', 'HP', '02'),
    ('Jharkhand', 'JH', '20'),
    ('Karnataka', 'KA', '29'),
    ('Kerala', 'KL', '32'),
    ('Madhya Pradesh', 'MP', '23'),
    ('Maharashtra', 'MH', '27'),
    ('Manipur', 'MN', '14'),
    ('Meghalaya', 'ML', '17'),
    ('Mizoram', 'MZ', '15'),
    ('Nagaland', 'NL', '13'),
    ('Odisha', 'OD', '21'),
    ('Punjab', 'PB', '03'),
    ('Rajasthan', 'RJ', '08'),
    ('Sikkim', 'SK', '11'),
    ('Tamil Nadu', 'TN', '33'),
    ('Telangana', 'TS', '36'),
    ('Tripura', 'TR', '16'),
    ('Uttar Pradesh', 'UP', '09'),
    ('Uttarakhand', 'UK', '05'),
    ('West Bengal', 'WB', '19'),
    -- Union Territories
    ('Andaman and Nicobar Islands', 'AN', '35'),
    ('Chandigarh', 'CH', '04'),
    ('Dadra and Nagar Haveli and Daman and Diu', 'DD', '26'),
    ('Delhi', 'DL', '07'),
    ('Jammu and Kashmir', 'JK', '01'),
    ('Ladakh', 'LA', '38'),
    ('Lakshadweep', 'LD', '31'),
    ('Puducherry', 'PY', '34')
) AS data(name, code, state_code)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

-- India GST Rates (for Maharashtra example + national IGST)
WITH rate_data AS (
    SELECT * FROM (VALUES
        ('STATE', 'MH', 'CGST 9%', 9.0, 'CGST', 1, true, true, DATE '2017-07-01', false),
        ('STATE', 'MH', 'SGST 9%', 9.0, 'SGST', 2, true, true, DATE '2017-07-01', false),
        ('COUNTRY', 'IN', 'IGST 18%', 18.0, 'IGST', 1, true, true, DATE '2017-07-01', false),
        ('STATE', 'MH', 'CGST 2.5%', 2.5, 'CGST', 1, true, true, DATE '2017-07-01', false),
        ('STATE', 'MH', 'SGST 2.5%', 2.5, 'SGST', 2, true, true, DATE '2017-07-01', false),
        ('COUNTRY', 'IN', 'IGST 5%', 5.0, 'IGST', 1, true, true, DATE '2017-07-01', false),
        ('STATE', 'MH', 'CGST 6%', 6.0, 'CGST', 1, true, true, DATE '2017-07-01', false),
        ('STATE', 'MH', 'SGST 6%', 6.0, 'SGST', 2, true, true, DATE '2017-07-01', false),
        ('COUNTRY', 'IN', 'IGST 12%', 12.0, 'IGST', 1, true, true, DATE '2017-07-01', false),
        ('STATE', 'MH', 'CGST 14%', 14.0, 'CGST', 1, true, true, DATE '2017-07-01', false),
        ('STATE', 'MH', 'SGST 14%', 14.0, 'SGST', 2, true, true, DATE '2017-07-01', false),
        ('COUNTRY', 'IN', 'IGST 28%', 28.0, 'IGST', 1, true, true, DATE '2017-07-01', false)
    ) AS v(jurisdiction_type, code, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', j.id, v.name, v.rate, v.tax_type, v.priority,
       v.applies_to_shipping, v.applies_to_products, v.effective_from, v.is_compound
FROM rate_data v
JOIN tax_jurisdictions j
  ON j.tenant_id = 'global' AND j.type = v.jurisdiction_type AND j.code = v.code
LEFT JOIN tax_rates existing
  ON existing.tenant_id = 'global'
 AND existing.jurisdiction_id = j.id
 AND existing.name = v.name
WHERE existing.id IS NULL;

-- India Product Tax Categories with HSN/SAC codes
INSERT INTO product_tax_categories (tenant_id, name, description, tax_code, hsn_code, sac_code, gst_slab, is_tax_exempt, is_nil_rated, is_zero_rated) VALUES
-- Goods (HSN codes)
('global', 'Essential Commodities', 'Essential food items - 0% GST', 'IN-0', '0401', NULL, 0, false, true, false),
('global', 'Basic Necessities', 'Basic food and necessities - 5% GST', 'IN-5', '1001', NULL, 5, false, false, false),
('global', 'Processed Food', 'Processed food items - 12% GST', 'IN-12', '2106', NULL, 12, false, false, false),
('global', 'Standard Goods', 'Most goods and services - 18% GST', 'IN-18', '8471', NULL, 18, false, false, false),
('global', 'Luxury Goods', 'Luxury items, cars, tobacco - 28% GST', 'IN-28', '8703', NULL, 28, false, false, false),
('global', 'Automobiles with Cess', 'Luxury cars with additional cess', 'IN-28-CESS', '8703', NULL, 28, false, false, false),
-- Services (SAC codes)
('global', 'Basic Services', 'Accommodation, transport - 5% GST', 'IN-SVC-5', NULL, '9963', 5, false, false, false),
('global', 'Standard Services', 'IT, consulting, repairs - 18% GST', 'IN-SVC-18', NULL, '9983', 18, false, false, false),
('global', 'Entertainment Services', 'Movies, amusement parks - 28% GST', 'IN-SVC-28', NULL, '9996', 28, false, false, false),
('global', 'Healthcare Services', 'Medical services - Exempt', 'IN-SVC-0', NULL, '9993', 0, true, false, false),
('global', 'Educational Services', 'Educational services - Exempt', 'IN-SVC-EDU', NULL, '9992', 0, true, false, false),
-- Exports (Zero-rated)
('global', 'Export Goods', 'Goods for export - 0% with ITC', 'IN-EXPORT', NULL, NULL, 0, false, false, true)
ON CONFLICT (tenant_id, name) DO NOTHING;

-- =============================================================================
-- EUROPEAN UNION VAT
-- =============================================================================
-- Standard VAT rates vary by country, reverse charge applies to B2B cross-border

-- EU Countries
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'Germany', 'COUNTRY', 'DE', NULL, NULL, true),
('global', 'France', 'COUNTRY', 'FR', NULL, NULL, true),
('global', 'Italy', 'COUNTRY', 'IT', NULL, NULL, true),
('global', 'Spain', 'COUNTRY', 'ES', NULL, NULL, true),
('global', 'Netherlands', 'COUNTRY', 'NL', NULL, NULL, true),
('global', 'Belgium', 'COUNTRY', 'BE', NULL, NULL, true),
('global', 'Austria', 'COUNTRY', 'AT', NULL, NULL, true),
('global', 'Poland', 'COUNTRY', 'PL', NULL, NULL, true),
('global', 'Sweden', 'COUNTRY', 'SE', NULL, NULL, true),
('global', 'Denmark', 'COUNTRY', 'DK', NULL, NULL, true),
('global', 'Finland', 'COUNTRY', 'FI', NULL, NULL, true),
('global', 'Ireland', 'COUNTRY', 'IE', NULL, NULL, true),
('global', 'Portugal', 'COUNTRY', 'PT', NULL, NULL, true),
('global', 'Greece', 'COUNTRY', 'GR', NULL, NULL, true),
('global', 'Czech Republic', 'COUNTRY', 'CZ', NULL, NULL, true),
('global', 'Romania', 'COUNTRY', 'RO', NULL, NULL, true),
('global', 'Hungary', 'COUNTRY', 'HU', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

-- EU VAT Rates (2024 standard rates)
WITH eu_rates AS (
    SELECT * FROM (VALUES
        ('DE', 'Germany VAT', 19.0),
        ('FR', 'France VAT', 20.0),
        ('IT', 'Italy VAT', 22.0),
        ('ES', 'Spain VAT', 21.0),
        ('NL', 'Netherlands VAT', 21.0),
        ('BE', 'Belgium VAT', 21.0),
        ('AT', 'Austria VAT', 20.0),
        ('PL', 'Poland VAT', 23.0),
        ('SE', 'Sweden VAT', 25.0),
        ('DK', 'Denmark VAT', 25.0),
        ('FI', 'Finland VAT', 24.0),
        ('IE', 'Ireland VAT', 23.0),
        ('PT', 'Portugal VAT', 23.0),
        ('GR', 'Greece VAT', 24.0),
        ('CZ', 'Czech Republic VAT', 21.0),
        ('RO', 'Romania VAT', 19.0),
        ('HU', 'Hungary VAT', 27.0)
    ) AS v(code, name, rate)
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', j.id, v.name, v.rate, 'VAT', 1, true, true, DATE '2020-01-01', false
FROM eu_rates v
JOIN tax_jurisdictions j ON j.tenant_id = 'global' AND j.type = 'COUNTRY' AND j.code = v.code
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = j.id AND existing.name = v.name
WHERE existing.id IS NULL;

-- Reduced VAT rate for Germany
WITH germany AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'DE'
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', germany.id, 'Germany Reduced VAT', 7.0, 'VAT', 1, false, true, DATE '2021-01-01', false
FROM germany
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = germany.id AND existing.name = 'Germany Reduced VAT'
WHERE existing.id IS NULL;

-- =============================================================================
-- UNITED KINGDOM (Post-Brexit)
-- =============================================================================

-- UK Country and Home Nations
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'United Kingdom', 'COUNTRY', 'GB', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH uk_country AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'GB'
)
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active)
SELECT 'global', data.name, 'STATE', data.code, NULL, uk_country.id, true
FROM uk_country
CROSS JOIN (
    VALUES ('England', 'ENG'), ('Scotland', 'SCT'), ('Wales', 'WLS'), ('Northern Ireland', 'NIR')
) AS data(name, code)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH uk_rate_data AS (
    SELECT * FROM (VALUES
        ('GB', 'UK VAT Standard', 20.0, true, true),
        ('GB', 'UK VAT Reduced', 5.0, false, true),
        ('GB', 'UK VAT Zero', 0.0, false, true)
    ) AS v(code, name, rate, applies_to_shipping, applies_to_products)
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', j.id, v.name, v.rate, 'VAT', 1, v.applies_to_shipping, v.applies_to_products, DATE '2021-01-01', false
FROM uk_rate_data v
JOIN tax_jurisdictions j ON j.tenant_id = 'global' AND j.type = 'COUNTRY' AND j.code = v.code
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = j.id AND existing.name = v.name
WHERE existing.id IS NULL;

-- =============================================================================
-- CANADA (GST/HST/PST/QST)
-- =============================================================================

INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'Canada', 'COUNTRY', 'CA', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH canada_country AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'CA'
)
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active)
SELECT 'global', data.name, 'STATE', data.code, NULL, canada_country.id, true
FROM canada_country
CROSS JOIN (
    VALUES
    -- Provinces with HST (combined federal + provincial)
    ('Ontario', 'ON'),
    ('Nova Scotia', 'NS'),
    ('New Brunswick', 'NB'),
    ('Prince Edward Island', 'PE'),
    ('Newfoundland and Labrador', 'NL'),
    -- GST-only provinces/territories
    ('Alberta', 'AB'),
    ('Yukon', 'YT'),
    ('Northwest Territories', 'NT'),
    ('Nunavut', 'NU'),
    -- GST + PST provinces
    ('British Columbia', 'BC'),
    ('Saskatchewan', 'SK'),
    ('Manitoba', 'MB'),
    -- GST + QST
    ('Quebec', 'QC')
) AS data(name, code)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH canada_rates AS (
    SELECT * FROM (VALUES
        ('COUNTRY', 'CA', 'Federal GST', 5.0, 'GST', 1, true, true, false),
        ('STATE', 'ON', 'Ontario HST', 13.0, 'HST', 1, true, true, false),
        ('STATE', 'NS', 'Nova Scotia HST', 15.0, 'HST', 1, true, true, false),
        ('STATE', 'NB', 'New Brunswick HST', 15.0, 'HST', 1, true, true, false),
        ('STATE', 'PE', 'PEI HST', 15.0, 'HST', 1, true, true, false),
        ('STATE', 'NL', 'Newfoundland HST', 15.0, 'HST', 1, true, true, false),
        ('STATE', 'BC', 'BC PST', 7.0, 'PST', 2, true, true, false),
        ('STATE', 'SK', 'Saskatchewan PST', 6.0, 'PST', 2, true, true, false),
        ('STATE', 'MB', 'Manitoba PST', 7.0, 'PST', 2, true, true, false),
        ('STATE', 'QC', 'Quebec QST', 9.975, 'QST', 2, true, true, true)
    ) AS v(jurisdiction_type, code, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, is_compound)
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', j.id, v.name, v.rate, v.tax_type, v.priority,
       v.applies_to_shipping, v.applies_to_products, DATE '2020-01-01', v.is_compound
FROM canada_rates v
JOIN tax_jurisdictions j ON j.tenant_id = 'global' AND j.type = v.jurisdiction_type AND j.code = v.code
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = j.id AND existing.name = v.name
WHERE existing.id IS NULL;

-- =============================================================================
-- AUSTRALIA (GST)
-- =============================================================================

INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'Australia', 'COUNTRY', 'AU', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH australia AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'AU'
)
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active)
SELECT 'global', data.name, 'STATE', data.code, NULL, australia.id, true
FROM australia
CROSS JOIN (
    VALUES
    ('New South Wales', 'NSW'),
    ('Victoria', 'VIC'),
    ('Queensland', 'QLD'),
    ('Western Australia', 'WA'),
    ('South Australia', 'SA'),
    ('Tasmania', 'TAS'),
    ('Australian Capital Territory', 'ACT'),
    ('Northern Territory', 'NT')
) AS data(name, code)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH au_rate AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'AU'
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', au_rate.id, 'Australia GST', 10.0, 'GST', 1, true, true, DATE '2000-07-01', false
FROM au_rate
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = au_rate.id AND existing.name = 'Australia GST'
WHERE existing.id IS NULL;

-- =============================================================================
-- NEW ZEALAND (GST)
-- =============================================================================
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'New Zealand', 'COUNTRY', 'NZ', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH nz AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'NZ'
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', nz.id, 'New Zealand GST', 15.0, 'GST', 1, true, true, DATE '2010-10-01', false
FROM nz
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = nz.id AND existing.name = 'New Zealand GST'
WHERE existing.id IS NULL;

-- =============================================================================
-- SINGAPORE (GST)
-- =============================================================================
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'Singapore', 'COUNTRY', 'SG', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH sg AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'SG'
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', sg.id, 'Singapore GST', 9.0, 'GST', 1, true, true, DATE '2024-01-01', false
FROM sg
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = sg.id AND existing.name = 'Singapore GST'
WHERE existing.id IS NULL;

-- =============================================================================
-- JAPAN (Consumption Tax)
-- =============================================================================
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'Japan', 'COUNTRY', 'JP', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH jp_rates AS (
    SELECT * FROM (VALUES
        ('Japan Consumption Tax', 10.0, true, true),
        ('Japan Reduced Rate', 8.0, false, true)
    ) AS v(name, rate, applies_to_shipping, applies_to_products)
), japan AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'JP'
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', japan.id, v.name, v.rate, 'VAT', 1, v.applies_to_shipping, v.applies_to_products, DATE '2019-10-01', false
FROM jp_rates v
CROSS JOIN japan
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = japan.id AND existing.name = v.name
WHERE existing.id IS NULL;

-- =============================================================================
-- SOUTH KOREA (VAT)
-- =============================================================================
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'South Korea', 'COUNTRY', 'KR', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH kr AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'KR'
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', kr.id, 'South Korea VAT', 10.0, 'VAT', 1, true, true, DATE '2020-01-01', false
FROM kr
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = kr.id AND existing.name = 'South Korea VAT'
WHERE existing.id IS NULL;

-- =============================================================================
-- UNITED ARAB EMIRATES (VAT)
-- =============================================================================
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'United Arab Emirates', 'COUNTRY', 'AE', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH ae AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'AE'
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', ae.id, 'UAE VAT', 5.0, 'VAT', 1, true, true, DATE '2018-01-01', false
FROM ae
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = ae.id AND existing.name = 'UAE VAT'
WHERE existing.id IS NULL;

-- =============================================================================
-- SAUDI ARABIA (VAT)
-- =============================================================================
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'Saudi Arabia', 'COUNTRY', 'SA', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH sa AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'SA'
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', sa.id, 'Saudi Arabia VAT', 15.0, 'VAT', 1, true, true, DATE '2020-07-01', false
FROM sa
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = sa.id AND existing.name = 'Saudi Arabia VAT'
WHERE existing.id IS NULL;

-- =============================================================================
-- BRAZIL (Simplified multi-layer taxes)
-- =============================================================================
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'Brazil', 'COUNTRY', 'BR', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH brazil AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'BR'
)
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active)
SELECT 'global', data.name, 'STATE', data.code, NULL, brazil.id, true
FROM brazil
CROSS JOIN (
    VALUES ('Sao Paulo', 'SP'), ('Rio de Janeiro', 'RJ')
) AS data(name, code)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH br_rates AS (
    SELECT * FROM (VALUES
        ('STATE', 'SP', 'ICMS Sao Paulo', 18.0),
        ('STATE', 'RJ', 'ICMS Rio', 20.0)
    ) AS v(type, code, name, rate)
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', j.id, v.name, v.rate, 'STATE', 1, true, true, DATE '2020-01-01', false
FROM br_rates v
JOIN tax_jurisdictions j ON j.tenant_id = 'global' AND j.type = v.type AND j.code = v.code
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = j.id AND existing.name = v.name
WHERE existing.id IS NULL;

-- =============================================================================
-- MEXICO (IVA)
-- =============================================================================
INSERT INTO tax_jurisdictions (tenant_id, name, type, code, state_code, parent_id, is_active) VALUES
('global', 'Mexico', 'COUNTRY', 'MX', NULL, NULL, true)
ON CONFLICT (tenant_id, type, code) DO NOTHING;

WITH mx_rates AS (
    SELECT * FROM (VALUES
        ('Mexico IVA', 16.0, true, true),
        ('Mexico IVA Border Zone', 8.0, false, true)
    ) AS v(name, rate, applies_to_shipping, applies_to_products)
), mexico AS (
    SELECT id FROM tax_jurisdictions WHERE tenant_id = 'global' AND type = 'COUNTRY' AND code = 'MX'
)
INSERT INTO tax_rates (tenant_id, jurisdiction_id, name, rate, tax_type, priority, applies_to_shipping, applies_to_products, effective_from, is_compound)
SELECT 'global', mexico.id, v.name, v.rate, 'VAT', 1, v.applies_to_shipping, v.applies_to_products, DATE '2020-01-01', false
FROM mx_rates v
CROSS JOIN mexico
LEFT JOIN tax_rates existing ON existing.tenant_id = 'global' AND existing.jurisdiction_id = mexico.id AND existing.name = v.name
WHERE existing.id IS NULL;

-- =============================================================================
-- Comments
-- =============================================================================
COMMENT ON TABLE tax_jurisdictions IS 'Global tax jurisdictions including India GST states, EU countries, and other major markets';
COMMENT ON TABLE tax_rates IS 'Tax rates for all jurisdictions including India CGST/SGST/IGST slabs, EU VAT, and Canada GST/HST/PST/QST';
