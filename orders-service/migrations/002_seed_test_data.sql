-- Seed test data for orders service
-- 3 test tenant IDs with sample orders

-- Test Tenant IDs (proper UUIDs):
-- 11111111-1111-1111-1111-111111111111: Electronics Store
-- 22222222-2222-2222-2222-222222222222: Fashion Boutique  
-- 33333333-3333-3333-3333-333333333333: Home & Garden

-- Clean existing data
TRUNCATE TABLE order_timelines, order_discounts, order_payments, order_shippings, order_customers, order_items, orders CASCADE;

-- Tenant 1: Electronics Store - Order 1 (PENDING)
INSERT INTO orders (id, tenant_id, order_number, customer_id, status, currency, subtotal, tax_amount, shipping_cost, discount_amount, total, notes, created_at, updated_at)
VALUES (
  'a0000001-1111-1111-1111-111111111111',
  '11111111-1111-1111-1111-111111111111',
  'ORD-1001',
  'c0000001-1111-1111-1111-111111111111',
  'PENDING',
  'USD',
  599.98,
  50.99,
  15.00,
  0.00,
  665.97,
  'Electronics order - Headphones bundle',
  NOW() - INTERVAL '2 days',
  NOW() - INTERVAL '2 days'
);

INSERT INTO order_items (id, order_id, product_id, product_name, sku, quantity, unit_price, total_price, created_at, updated_at)
VALUES
  ('10000001-1111-1111-1111-111111111111', 'a0000001-1111-1111-1111-111111111111', 'f0000001-1111-1111-1111-111111111111', 'Wireless Noise-Cancelling Headphones', 'WNC-HP-001', 1, 299.99, 299.99, NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days'),
  ('10000002-1111-1111-1111-111111111111', 'a0000001-1111-1111-1111-111111111111', 'f0000002-1111-1111-1111-111111111111', 'USB-C Fast Charging Cable', 'USB-C-FC-001', 2, 149.99, 299.99, NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

INSERT INTO order_customers (id, order_id, first_name, last_name, email, phone, created_at, updated_at)
VALUES ('20000001-1111-1111-1111-111111111111', 'a0000001-1111-1111-1111-111111111111', 'John', 'Electronics', 'john.electronics@test.com', '+1-555-0101', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

INSERT INTO order_shippings (id, order_id, method, carrier, cost, street, city, state, postal_code, country, estimated_delivery, created_at, updated_at)
VALUES ('30000001-1111-1111-1111-111111111111', 'a0000001-1111-1111-1111-111111111111', 'Standard Shipping', 'FedEx', 15.00, '123 Tech Street', 'San Francisco', 'CA', '94102', 'USA', NOW() + INTERVAL '5 days', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

INSERT INTO order_payments (id, order_id, method, status, amount, currency, transaction_id, created_at, updated_at)
VALUES ('40000001-1111-1111-1111-111111111111', 'a0000001-1111-1111-1111-111111111111', 'Credit Card', 'PENDING', 665.97, 'USD', 'txn-1111-1111-1111', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

INSERT INTO order_timelines (order_id, event, description, timestamp, created_by, created_at)
VALUES ('a0000001-1111-1111-1111-111111111111', 'ORDER_CREATED', 'Order has been created', NOW() - INTERVAL '2 days', 'system', NOW() - INTERVAL '2 days');

-- Tenant 1: Electronics Store - Order 2 (PROCESSING)
INSERT INTO orders (id, tenant_id, order_number, customer_id, status, currency, subtotal, tax_amount, shipping_cost, discount_amount, total, notes, created_at, updated_at)
VALUES (
  'a0000002-1111-1111-1111-111111111111',
  '11111111-1111-1111-1111-111111111111',
  'ORD-1002',
  'c0000002-1111-1111-1111-111111111111',
  'PROCESSING',
  'USD',
  899.99,
  76.49,
  20.00,
  50.00,
  946.48,
  'Webcam order with discount',
  NOW() - INTERVAL '1 day',
  NOW() - INTERVAL '6 hours'
);

INSERT INTO order_items (id, order_id, product_id, product_name, sku, quantity, unit_price, total_price, created_at, updated_at)
VALUES ('10000003-1111-1111-1111-111111111111', 'a0000002-1111-1111-1111-111111111111', 'f0000003-1111-1111-1111-111111111111', '4K Webcam', '4K-WC-001', 1, 899.99, 899.99, NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day');

INSERT INTO order_customers (id, order_id, first_name, last_name, email, phone, created_at, updated_at)
VALUES ('20000002-1111-1111-1111-111111111111', 'a0000002-1111-1111-1111-111111111111', 'Sarah', 'Techie', 'sarah.techie@test.com', '+1-555-0102', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day');

INSERT INTO order_shippings (id, order_id, method, carrier, tracking_number, cost, street, city, state, postal_code, country, estimated_delivery, created_at, updated_at)
VALUES ('30000002-1111-1111-1111-111111111111', 'a0000002-1111-1111-1111-111111111111', 'Express Shipping', 'UPS', 'UPS-TRACK-12345', 20.00, '456 Innovation Ave', 'Seattle', 'WA', '98101', 'USA', NOW() + INTERVAL '2 days', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day');

INSERT INTO order_payments (id, order_id, method, status, amount, currency, transaction_id, processed_at, created_at, updated_at)
VALUES ('40000002-1111-1111-1111-111111111111', 'a0000002-1111-1111-1111-111111111111', 'PayPal', 'COMPLETED', 946.48, 'USD', 'txn-1111-1111-1112', NOW() - INTERVAL '6 hours', NOW() - INTERVAL '1 day', NOW() - INTERVAL '6 hours');

INSERT INTO order_discounts (order_id, coupon_code, discount_type, amount, description, created_at)
VALUES ('a0000002-1111-1111-1111-111111111111', 'TECH50', 'fixed', 50.00, 'New customer discount', NOW() - INTERVAL '1 day');

INSERT INTO order_timelines (order_id, event, description, timestamp, created_by, created_at)
VALUES
  ('a0000002-1111-1111-1111-111111111111', 'ORDER_CREATED', 'Order has been created', NOW() - INTERVAL '1 day', 'system', NOW() - INTERVAL '1 day'),
  ('a0000002-1111-1111-1111-111111111111', 'STATUS_CHANGED', 'Order status changed to PROCESSING', NOW() - INTERVAL '6 hours', 'system', NOW() - INTERVAL '6 hours');

-- Tenant 1: Electronics Store - Order 3 (SHIPPED)
INSERT INTO orders (id, tenant_id, order_number, customer_id, status, currency, subtotal, tax_amount, shipping_cost, discount_amount, total, notes, created_at, updated_at)
VALUES (
  'a0000003-1111-1111-1111-111111111111',
  '11111111-1111-1111-1111-111111111111',
  'ORD-1003',
  'c0000003-1111-1111-1111-111111111111',
  'SHIPPED',
  'USD',
  1299.99,
  110.49,
  0.00,
  0.00,
  1410.48,
  'Gaming laptop with free shipping',
  NOW() - INTERVAL '3 days',
  NOW() - INTERVAL '1 day'
);

INSERT INTO order_items (id, order_id, product_id, product_name, sku, quantity, unit_price, total_price, created_at, updated_at)
VALUES ('10000004-1111-1111-1111-111111111111', 'a0000003-1111-1111-1111-111111111111', 'f0000004-1111-1111-1111-111111111111', 'Gaming Laptop', 'GM-LP-001', 1, 1299.99, 1299.99, NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days');

INSERT INTO order_customers (id, order_id, first_name, last_name, email, phone, created_at, updated_at)
VALUES ('20000003-1111-1111-1111-111111111111', 'a0000003-1111-1111-1111-111111111111', 'Mike', 'Gamer', 'mike.gamer@test.com', '+1-555-0103', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days');

INSERT INTO order_shippings (id, order_id, method, carrier, tracking_number, cost, street, city, state, postal_code, country, estimated_delivery, created_at, updated_at)
VALUES ('30000003-1111-1111-1111-111111111111', 'a0000003-1111-1111-1111-111111111111', 'Free Shipping', 'FedEx', 'FEDEX-TRACK-67890', 0.00, '789 Gaming Blvd', 'Austin', 'TX', '78701', 'USA', NOW() + INTERVAL '1 day', NOW() - INTERVAL '3 days', NOW() - INTERVAL '1 day');

INSERT INTO order_payments (id, order_id, method, status, amount, currency, transaction_id, processed_at, created_at, updated_at)
VALUES ('40000003-1111-1111-1111-111111111111', 'a0000003-1111-1111-1111-111111111111', 'Credit Card', 'COMPLETED', 1410.48, 'USD', 'txn-1111-1111-1113', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days');

INSERT INTO order_timelines (order_id, event, description, timestamp, created_by, created_at)
VALUES
  ('a0000003-1111-1111-1111-111111111111', 'ORDER_CREATED', 'Order has been created', NOW() - INTERVAL '3 days', 'system', NOW() - INTERVAL '3 days'),
  ('a0000003-1111-1111-1111-111111111111', 'STATUS_CHANGED', 'Order status changed to CONFIRMED', NOW() - INTERVAL '2 days', 'system', NOW() - INTERVAL '2 days'),
  ('a0000003-1111-1111-1111-111111111111', 'STATUS_CHANGED', 'Order status changed to PROCESSING', NOW() - INTERVAL '36 hours', 'system', NOW() - INTERVAL '36 hours'),
  ('a0000003-1111-1111-1111-111111111111', 'STATUS_CHANGED', 'Order status changed to SHIPPED', NOW() - INTERVAL '1 day', 'system', NOW() - INTERVAL '1 day');

-- Tenant 2: Fashion Boutique - Order 1 (CONFIRMED)
INSERT INTO orders (id, tenant_id, order_number, customer_id, status, currency, subtotal, tax_amount, shipping_cost, discount_amount, total, notes, created_at, updated_at)
VALUES (
  'a0000004-2222-2222-2222-222222222222',
  '22222222-2222-2222-2222-222222222222',
  'ORD-2001',
  'c0000004-2222-2222-2222-222222222222',
  'CONFIRMED',
  'USD',
  249.98,
  21.24,
  10.00,
  25.00,
  256.22,
  'Fashion items with VIP discount',
  NOW() - INTERVAL '1 day',
  NOW() - INTERVAL '12 hours'
);

INSERT INTO order_items (id, order_id, product_id, product_name, sku, quantity, unit_price, total_price, created_at, updated_at)
VALUES
  ('10000005-2222-2222-2222-222222222222', 'a0000004-2222-2222-2222-222222222222', 'f0000005-2222-2222-2222-222222222222', 'Designer Handbag', 'DES-HB-001', 1, 199.99, 199.99, NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day'),
  ('10000006-2222-2222-2222-222222222222', 'a0000004-2222-2222-2222-222222222222', 'f0000006-2222-2222-2222-222222222222', 'Silk Scarf', 'SLK-SC-001', 1, 49.99, 49.99, NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day');

INSERT INTO order_customers (id, order_id, first_name, last_name, email, phone, created_at, updated_at)
VALUES ('20000004-2222-2222-2222-222222222222', 'a0000004-2222-2222-2222-222222222222', 'Emma', 'Fashionista', 'emma.fashionista@test.com', '+1-555-0201', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day');

INSERT INTO order_shippings (id, order_id, method, carrier, cost, street, city, state, postal_code, country, estimated_delivery, created_at, updated_at)
VALUES ('30000004-2222-2222-2222-222222222222', 'a0000004-2222-2222-2222-222222222222', 'Standard Shipping', 'USPS', 10.00, '123 Fashion Ave', 'New York', 'NY', '10001', 'USA', NOW() + INTERVAL '4 days', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day');

INSERT INTO order_payments (id, order_id, method, status, amount, currency, transaction_id, processed_at, created_at, updated_at)
VALUES ('40000004-2222-2222-2222-222222222222', 'a0000004-2222-2222-2222-222222222222', 'Credit Card', 'COMPLETED', 256.22, 'USD', 'txn-2222-2222-2221', NOW() - INTERVAL '12 hours', NOW() - INTERVAL '1 day', NOW() - INTERVAL '12 hours');

INSERT INTO order_discounts (order_id, coupon_code, discount_type, amount, description, created_at)
VALUES ('a0000004-2222-2222-2222-222222222222', 'VIP25', 'fixed', 25.00, 'VIP member discount', NOW() - INTERVAL '1 day');

INSERT INTO order_timelines (order_id, event, description, timestamp, created_by, created_at)
VALUES
  ('a0000004-2222-2222-2222-222222222222', 'ORDER_CREATED', 'Order has been created', NOW() - INTERVAL '1 day', 'system', NOW() - INTERVAL '1 day'),
  ('a0000004-2222-2222-2222-222222222222', 'STATUS_CHANGED', 'Order status changed to CONFIRMED', NOW() - INTERVAL '12 hours', 'system', NOW() - INTERVAL '12 hours');

-- Tenant 2: Fashion Boutique - Order 2 (DELIVERED)
INSERT INTO orders (id, tenant_id, order_number, customer_id, status, currency, subtotal, tax_amount, shipping_cost, discount_amount, total, notes, created_at, updated_at)
VALUES (
  'a0000005-2222-2222-2222-222222222222',
  '22222222-2222-2222-2222-222222222222',
  'ORD-2002',
  'c0000005-2222-2222-2222-222222222222',
  'DELIVERED',
  'USD',
  399.99,
  33.99,
  15.00,
  0.00,
  448.98,
  'Evening dress - rush delivery',
  NOW() - INTERVAL '5 days',
  NOW() - INTERVAL '1 hour'
);

INSERT INTO order_items (id, order_id, product_id, product_name, sku, quantity, unit_price, total_price, created_at, updated_at)
VALUES ('10000007-2222-2222-2222-222222222222', 'a0000005-2222-2222-2222-222222222222', 'f0000007-2222-2222-2222-222222222222', 'Evening Dress', 'EVN-DR-001', 1, 399.99, 399.99, NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days');

INSERT INTO order_customers (id, order_id, first_name, last_name, email, phone, created_at, updated_at)
VALUES ('20000005-2222-2222-2222-222222222222', 'a0000005-2222-2222-2222-222222222222', 'Olivia', 'Style', 'olivia.style@test.com', '+1-555-0202', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days');

INSERT INTO order_shippings (id, order_id, method, carrier, tracking_number, cost, street, city, state, postal_code, country, estimated_delivery, actual_delivery, created_at, updated_at)
VALUES ('30000005-2222-2222-2222-222222222222', 'a0000005-2222-2222-2222-222222222222', 'Express Shipping', 'FedEx', 'FEDEX-TRACK-11111', 15.00, '456 Broadway St', 'Los Angeles', 'CA', '90012', 'USA', NOW() - INTERVAL '2 days', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '5 days', NOW() - INTERVAL '1 hour');

INSERT INTO order_payments (id, order_id, method, status, amount, currency, transaction_id, processed_at, created_at, updated_at)
VALUES ('40000005-2222-2222-2222-222222222222', 'a0000005-2222-2222-2222-222222222222', 'Apple Pay', 'COMPLETED', 448.98, 'USD', 'txn-2222-2222-2222', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days');

INSERT INTO order_timelines (order_id, event, description, timestamp, created_by, created_at)
VALUES
  ('a0000005-2222-2222-2222-222222222222', 'ORDER_CREATED', 'Order has been created', NOW() - INTERVAL '5 days', 'system', NOW() - INTERVAL '5 days'),
  ('a0000005-2222-2222-2222-222222222222', 'STATUS_CHANGED', 'Order status changed to CONFIRMED', NOW() - INTERVAL '4 days', 'system', NOW() - INTERVAL '4 days'),
  ('a0000005-2222-2222-2222-222222222222', 'STATUS_CHANGED', 'Order status changed to PROCESSING', NOW() - INTERVAL '3 days', 'system', NOW() - INTERVAL '3 days'),
  ('a0000005-2222-2222-2222-222222222222', 'STATUS_CHANGED', 'Order status changed to SHIPPED', NOW() - INTERVAL '2 days', 'system', NOW() - INTERVAL '2 days'),
  ('a0000005-2222-2222-2222-222222222222', 'STATUS_CHANGED', 'Order status changed to DELIVERED', NOW() - INTERVAL '1 hour', 'system', NOW() - INTERVAL '1 hour');

-- Tenant 3: Home & Garden - Order 1 (PENDING)
INSERT INTO orders (id, tenant_id, order_number, customer_id, status, currency, subtotal, tax_amount, shipping_cost, discount_amount, total, notes, created_at, updated_at)
VALUES (
  'a0000006-3333-3333-3333-333333333333',
  '33333333-3333-3333-3333-333333333333',
  'ORD-3001',
  'c0000006-3333-3333-3333-333333333333',
  'PENDING',
  'USD',
  149.98,
  12.74,
  12.00,
  0.00,
  174.72,
  'Garden tools and pots',
  NOW() - INTERVAL '6 hours',
  NOW() - INTERVAL '6 hours'
);

INSERT INTO order_items (id, order_id, product_id, product_name, sku, quantity, unit_price, total_price, created_at, updated_at)
VALUES
  ('10000008-3333-3333-3333-333333333333', 'a0000006-3333-3333-3333-333333333333', 'f0000008-3333-3333-3333-333333333333', 'Garden Tool Set', 'GRD-TS-001', 1, 89.99, 89.99, NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours'),
  ('10000009-3333-3333-3333-333333333333', 'a0000006-3333-3333-3333-333333333333', 'f0000009-3333-3333-3333-333333333333', 'Plant Pots Set of 6', 'PLT-PS-006', 1, 59.99, 59.99, NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours');

INSERT INTO order_customers (id, order_id, first_name, last_name, email, phone, created_at, updated_at)
VALUES ('20000006-3333-3333-3333-333333333333', 'a0000006-3333-3333-3333-333333333333', 'David', 'Gardener', 'david.gardener@test.com', '+1-555-0301', NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours');

INSERT INTO order_shippings (id, order_id, method, carrier, cost, street, city, state, postal_code, country, estimated_delivery, created_at, updated_at)
VALUES ('30000006-3333-3333-3333-333333333333', 'a0000006-3333-3333-3333-333333333333', 'Standard Shipping', 'UPS', 12.00, '123 Garden Lane', 'Portland', 'OR', '97201', 'USA', NOW() + INTERVAL '6 days', NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours');

INSERT INTO order_payments (id, order_id, method, status, amount, currency, transaction_id, created_at, updated_at)
VALUES ('40000006-3333-3333-3333-333333333333', 'a0000006-3333-3333-3333-333333333333', 'Debit Card', 'PENDING', 174.72, 'USD', 'txn-3333-3333-3331', NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours');

INSERT INTO order_timelines (order_id, event, description, timestamp, created_by, created_at)
VALUES ('a0000006-3333-3333-3333-333333333333', 'ORDER_CREATED', 'Order has been created', NOW() - INTERVAL '6 hours', 'system', NOW() - INTERVAL '6 hours');

-- Tenant 3: Home & Garden - Order 2 (CANCELLED)
INSERT INTO orders (id, tenant_id, order_number, customer_id, status, currency, subtotal, tax_amount, shipping_cost, discount_amount, total, notes, created_at, updated_at)
VALUES (
  'a0000007-3333-3333-3333-333333333333',
  '33333333-3333-3333-3333-333333333333',
  'ORD-3002',
  'c0000007-3333-3333-3333-333333333333',
  'CANCELLED',
  'USD',
  299.99,
  25.49,
  0.00,
  30.00,
  295.48,
  'Patio set - customer changed mind',
  NOW() - INTERVAL '2 days',
  NOW() - INTERVAL '1 day'
);

INSERT INTO order_items (id, order_id, product_id, product_name, sku, quantity, unit_price, total_price, created_at, updated_at)
VALUES ('1000000a-3333-3333-3333-333333333333', 'a0000007-3333-3333-3333-333333333333', 'f000000a-3333-3333-3333-333333333333', 'Outdoor Patio Set', 'OUT-PS-001', 1, 299.99, 299.99, NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

INSERT INTO order_customers (id, order_id, first_name, last_name, email, phone, created_at, updated_at)
VALUES ('20000007-3333-3333-3333-333333333333', 'a0000007-3333-3333-3333-333333333333', 'Lisa', 'Homeowner', 'lisa.homeowner@test.com', '+1-555-0302', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

INSERT INTO order_shippings (id, order_id, method, carrier, cost, street, city, state, postal_code, country, created_at, updated_at)
VALUES ('30000007-3333-3333-3333-333333333333', 'a0000007-3333-3333-3333-333333333333', 'Free Shipping', 'FedEx', 0.00, '456 Home St', 'Denver', 'CO', '80201', 'USA', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

INSERT INTO order_payments (id, order_id, method, status, amount, currency, transaction_id, processed_at, created_at, updated_at)
VALUES ('40000007-3333-3333-3333-333333333333', 'a0000007-3333-3333-3333-333333333333', 'Credit Card', 'PENDING', 295.48, 'USD', 'txn-3333-3333-3332', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

INSERT INTO order_discounts (order_id, coupon_code, discount_type, amount, description, created_at)
VALUES ('a0000007-3333-3333-3333-333333333333', 'HOME30', 'fixed', 30.00, 'Home improvement discount', NOW() - INTERVAL '2 days');

INSERT INTO order_timelines (order_id, event, description, timestamp, created_by, created_at)
VALUES
  ('a0000007-3333-3333-3333-333333333333', 'ORDER_CREATED', 'Order has been created', NOW() - INTERVAL '2 days', 'system', NOW() - INTERVAL '2 days'),
  ('a0000007-3333-3333-3333-333333333333', 'STATUS_CHANGED', 'Order status changed to CANCELLED. Notes: Customer changed mind', NOW() - INTERVAL '1 day', 'system', NOW() - INTERVAL '1 day');
