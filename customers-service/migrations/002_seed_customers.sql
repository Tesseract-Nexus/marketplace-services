-- Seed sample customer data for testing
-- Tenant ID: test-tenant

-- Insert sample customers
INSERT INTO customers (
    id,
    tenant_id,
    email,
    first_name,
    last_name,
    phone,
    status,
    customer_type,
    total_orders,
    total_spent,
    average_order_value,
    lifetime_value,
    last_order_date,
    first_order_date,
    tags,
    marketing_opt_in,
    created_at
) VALUES
-- VIP Customers
('11111111-1111-1111-1111-111111111111', 'test-tenant', 'sarah.johnson@example.com', 'Sarah', 'Johnson', '+1-415-555-0101', 'ACTIVE', 'VIP', 42, 15680.50, 373.35, 16200.00, NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 years', ARRAY['vip', 'high-value', 'loyal'], true, NOW() - INTERVAL '2 years'),
('11111111-1111-1111-1111-111111111112', 'test-tenant', 'michael.chen@example.com', 'Michael', 'Chen', '+1-415-555-0102', 'ACTIVE', 'VIP', 38, 14250.00, 375.00, 14800.00, NOW() - INTERVAL '5 days', NOW() - INTERVAL '18 months', ARRAY['vip', 'frequent-buyer'], true, NOW() - INTERVAL '18 months'),
('11111111-1111-1111-1111-111111111113', 'test-tenant', 'emma.rodriguez@example.com', 'Emma', 'Rodriguez', '+1-415-555-0103', 'ACTIVE', 'VIP', 35, 12890.75, 368.31, 13200.00, NOW() - INTERVAL '1 week', NOW() - INTERVAL '2 years 3 months', ARRAY['vip', 'early-adopter'], true, NOW() - INTERVAL '2 years 3 months'),

-- Wholesale Customers
('22222222-2222-2222-2222-222222222221', 'test-tenant', 'orders@techsolutions.com', 'David', 'Williams', '+1-415-555-0201', 'ACTIVE', 'WHOLESALE', 28, 45600.00, 1628.57, 46000.00, NOW() - INTERVAL '3 days', NOW() - INTERVAL '1 year', ARRAY['wholesale', 'bulk-buyer', 'technology'], true, NOW() - INTERVAL '1 year'),
('22222222-2222-2222-2222-222222222222', 'test-tenant', 'purchasing@retailplus.com', 'Jessica', 'Martinez', '+1-415-555-0202', 'ACTIVE', 'WHOLESALE', 22, 38900.00, 1768.18, 39500.00, NOW() - INTERVAL '1 week', NOW() - INTERVAL '9 months', ARRAY['wholesale', 'retail-partner'], true, NOW() - INTERVAL '9 months'),

-- Retail Customers (Active)
('33333333-3333-3333-3333-333333333331', 'test-tenant', 'alex.thompson@example.com', 'Alex', 'Thompson', '+1-415-555-0301', 'ACTIVE', 'RETAIL', 15, 3420.50, 228.03, 3600.00, NOW() - INTERVAL '10 days', NOW() - INTERVAL '8 months', ARRAY['active-shopper'], true, NOW() - INTERVAL '8 months'),
('33333333-3333-3333-3333-333333333332', 'test-tenant', 'olivia.davis@example.com', 'Olivia', 'Davis', '+1-415-555-0302', 'ACTIVE', 'RETAIL', 12, 2890.00, 240.83, 3000.00, NOW() - INTERVAL '2 weeks', NOW() - INTERVAL '6 months', ARRAY['fashion-lover'], true, NOW() - INTERVAL '6 months'),
('33333333-3333-3333-3333-333333333333', 'test-tenant', 'james.wilson@example.com', 'James', 'Wilson', '+1-415-555-0303', 'ACTIVE', 'RETAIL', 18, 4560.75, 253.38, 4700.00, NOW() - INTERVAL '4 days', NOW() - INTERVAL '1 year 2 months', ARRAY['tech-enthusiast'], true, NOW() - INTERVAL '1 year 2 months'),
('33333333-3333-3333-3333-333333333334', 'test-tenant', 'sophia.anderson@example.com', 'Sophia', 'Anderson', '+1-415-555-0304', 'ACTIVE', 'RETAIL', 10, 2150.00, 215.00, 2300.00, NOW() - INTERVAL '1 week', NOW() - INTERVAL '5 months', ARRAY['new-customer'], true, NOW() - INTERVAL '5 months'),
('33333333-3333-3333-3333-333333333335', 'test-tenant', 'william.taylor@example.com', 'William', 'Taylor', '+1-415-555-0305', 'ACTIVE', 'RETAIL', 8, 1890.50, 236.31, 2000.00, NOW() - INTERVAL '12 days', NOW() - INTERVAL '4 months', ARRAY['occasional-buyer'], false, NOW() - INTERVAL '4 months'),
('33333333-3333-3333-3333-333333333336', 'test-tenant', 'isabella.brown@example.com', 'Isabella', 'Brown', '+1-415-555-0306', 'ACTIVE', 'RETAIL', 20, 5670.25, 283.51, 5900.00, NOW() - INTERVAL '3 days', NOW() - INTERVAL '1 year 4 months', ARRAY['frequent-buyer', 'home-decor'], true, NOW() - INTERVAL '1 year 4 months'),
('33333333-3333-3333-3333-333333333337', 'test-tenant', 'ethan.garcia@example.com', 'Ethan', 'Garcia', '+1-415-555-0307', 'ACTIVE', 'RETAIL', 14, 3340.00, 238.57, 3500.00, NOW() - INTERVAL '8 days', NOW() - INTERVAL '7 months', ARRAY['sports-gear'], true, NOW() - INTERVAL '7 months'),
('33333333-3333-3333-3333-333333333338', 'test-tenant', 'mia.hernandez@example.com', 'Mia', 'Hernandez', '+1-415-555-0308', 'ACTIVE', 'RETAIL', 11, 2450.75, 222.80, 2600.00, NOW() - INTERVAL '6 days', NOW() - INTERVAL '5 months', ARRAY['beauty-products'], true, NOW() - INTERVAL '5 months'),
('33333333-3333-3333-3333-333333333339', 'test-tenant', 'noah.lopez@example.com', 'Noah', 'Lopez', '+1-415-555-0309', 'ACTIVE', 'RETAIL', 16, 4120.50, 257.53, 4300.00, NOW() - INTERVAL '11 days', NOW() - INTERVAL '10 months', ARRAY['electronics'], true, NOW() - INTERVAL '10 months'),
('33333333-3333-3333-3333-333333333340', 'test-tenant', 'ava.gonzalez@example.com', 'Ava', 'Gonzalez', '+1-415-555-0310', 'ACTIVE', 'RETAIL', 9, 2010.00, 223.33, 2100.00, NOW() - INTERVAL '15 days', NOW() - INTERVAL '3 months', ARRAY['new-customer'], true, NOW() - INTERVAL '3 months'),

-- Inactive Customers
('44444444-4444-4444-4444-444444444441', 'test-tenant', 'daniel.white@example.com', 'Daniel', 'White', '+1-415-555-0401', 'INACTIVE', 'RETAIL', 5, 890.00, 178.00, 900.00, NOW() - INTERVAL '6 months', NOW() - INTERVAL '1 year', ARRAY['inactive'], false, NOW() - INTERVAL '1 year'),
('44444444-4444-4444-4444-444444444442', 'test-tenant', 'emily.harris@example.com', 'Emily', 'Harris', '+1-415-555-0402', 'INACTIVE', 'RETAIL', 3, 450.00, 150.00, 500.00, NOW() - INTERVAL '8 months', NOW() - INTERVAL '1 year 2 months', ARRAY['inactive', 'win-back'], false, NOW() - INTERVAL '1 year 2 months'),

-- Blocked Customer
('55555555-5555-5555-5555-555555555551', 'test-tenant', 'blocked.user@example.com', 'Blocked', 'User', '+1-415-555-0501', 'BLOCKED', 'RETAIL', 2, 180.00, 90.00, 0.00, NOW() - INTERVAL '3 months', NOW() - INTERVAL '4 months', ARRAY['blocked', 'fraud'], false, NOW() - INTERVAL '4 months')

ON CONFLICT (tenant_id, email) DO NOTHING;

-- Insert sample addresses for some customers
INSERT INTO customer_addresses (
    customer_id,
    tenant_id,
    address_type,
    is_default,
    first_name,
    last_name,
    address_line_1,
    city,
    state,
    postal_code,
    country,
    phone
) VALUES
-- Sarah Johnson addresses
('11111111-1111-1111-1111-111111111111', 'test-tenant', 'BOTH', true, 'Sarah', 'Johnson', '123 Market Street, Apt 4B', 'San Francisco', 'CA', '94102', 'US', '+1-415-555-0101'),
('11111111-1111-1111-1111-111111111111', 'test-tenant', 'SHIPPING', false, 'Sarah', 'Johnson', '456 Oak Avenue', 'Palo Alto', 'CA', '94301', 'US', '+1-415-555-0101'),

-- Michael Chen addresses
('11111111-1111-1111-1111-111111111112', 'test-tenant', 'BOTH', true, 'Michael', 'Chen', '789 Tech Boulevard', 'Mountain View', 'CA', '94043', 'US', '+1-415-555-0102'),

-- David Williams (Wholesale)
('22222222-2222-2222-2222-222222222221', 'test-tenant', 'BOTH', true, 'David', 'Williams', 'TechSolutions Inc, 100 Enterprise Way', 'San Jose', 'CA', '95113', 'US', '+1-415-555-0201'),

-- Alex Thompson
('33333333-3333-3333-3333-333333333331', 'test-tenant', 'BOTH', true, 'Alex', 'Thompson', '234 Sunset Boulevard', 'San Francisco', 'CA', '94122', 'US', '+1-415-555-0301'),

-- Isabella Brown
('33333333-3333-3333-3333-333333333336', 'test-tenant', 'BOTH', true, 'Isabella', 'Brown', '567 Pine Street', 'Oakland', 'CA', '94612', 'US', '+1-415-555-0306')

ON CONFLICT DO NOTHING;

-- Insert some customer notes
INSERT INTO customer_notes (
    customer_id,
    tenant_id,
    note
) VALUES
('11111111-1111-1111-1111-111111111111', 'test-tenant', 'VIP customer - always provide expedited shipping'),
('11111111-1111-1111-1111-111111111112', 'test-tenant', 'Prefers email communication over phone'),
('22222222-2222-2222-2222-222222222221', 'test-tenant', 'Bulk orders - requires NET 30 payment terms'),
('55555555-5555-5555-5555-555555555551', 'test-tenant', 'Account blocked due to payment disputes - DO NOT FULFILL ORDERS')

ON CONFLICT DO NOTHING;

-- Create customer segments
INSERT INTO customer_segments (
    id,
    tenant_id,
    name,
    description,
    is_dynamic,
    customer_count
) VALUES
('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'test-tenant', 'VIP Customers', 'High-value customers with lifetime value > $10,000', false, 3),
('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'test-tenant', 'Frequent Buyers', 'Customers who made 10+ orders in the last year', false, 8),
('cccccccc-cccc-cccc-cccc-cccccccccccc', 'test-tenant', 'At-Risk', 'Customers who haven''t ordered in 6+ months', false, 2),
('dddddddd-dddd-dddd-dddd-dddddddddddd', 'test-tenant', 'New Customers', 'Customers who joined in the last 6 months', false, 5)

ON CONFLICT (tenant_id, name) DO NOTHING;

-- Assign customers to segments
INSERT INTO customer_segment_members (
    customer_id,
    segment_id,
    tenant_id,
    added_automatically
) VALUES
-- VIP Segment
('11111111-1111-1111-1111-111111111111', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'test-tenant', false),
('11111111-1111-1111-1111-111111111112', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'test-tenant', false),
('11111111-1111-1111-1111-111111111113', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'test-tenant', false),

-- Frequent Buyers
('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'test-tenant', true),
('11111111-1111-1111-1111-111111111112', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'test-tenant', true),
('33333333-3333-3333-3333-333333333333', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'test-tenant', true),
('33333333-3333-3333-3333-333333333336', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'test-tenant', true),

-- At-Risk
('44444444-4444-4444-4444-444444444441', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'test-tenant', true),
('44444444-4444-4444-4444-444444444442', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'test-tenant', true),

-- New Customers
('33333333-3333-3333-3333-333333333334', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'test-tenant', true),
('33333333-3333-3333-3333-333333333335', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'test-tenant', true),
('33333333-3333-3333-3333-333333333340', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'test-tenant', true)

ON CONFLICT (customer_id, segment_id) DO NOTHING;

-- Insert sample communication history
INSERT INTO customer_communications (
    customer_id,
    tenant_id,
    communication_type,
    direction,
    subject,
    content,
    status
) VALUES
('11111111-1111-1111-1111-111111111111', 'test-tenant', 'email', 'outbound', 'Your order has shipped!', 'Your VIP order #12345 has been shipped via FedEx Overnight...', 'opened'),
('11111111-1111-1111-1111-111111111112', 'test-tenant', 'email', 'outbound', 'Exclusive VIP Offer', 'As a valued VIP customer, enjoy 20% off your next order...', 'clicked'),
('33333333-3333-3333-3333-333333333331', 'test-tenant', 'sms', 'outbound', NULL, 'Your order is out for delivery today!', 'delivered'),
('44444444-4444-4444-4444-444444444441', 'test-tenant', 'email', 'outbound', 'We miss you!', 'It''s been a while since your last order. Here''s 15% off to welcome you back...', 'sent')

ON CONFLICT DO NOTHING;