-- Seed test data for products service
-- Using the same 3 tenant IDs as orders-service for consistency

-- Define our tenant IDs
-- Tenant 1: 11111111-1111-1111-1111-111111111111
-- Tenant 2: 22222222-2222-2222-2222-222222222222
-- Tenant 3: 33333333-3333-3333-3333-333333333333

-- Clear existing sample data
DELETE FROM product_variants;
DELETE FROM products;
DELETE FROM categories;

-- ============================================
-- CATEGORIES - Shared across all tenants
-- ============================================

INSERT INTO categories (id, tenant_id, name, slug, description, is_active, sort_order) VALUES
-- Tenant 1 Categories
('10000000-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 'Electronics', 'electronics', 'Electronic devices and accessories', true, 1),
('10000000-1111-1111-1111-111111111112', '11111111-1111-1111-1111-111111111111', 'Clothing', 'clothing', 'Apparel and fashion items', true, 2),
('10000000-1111-1111-1111-111111111113', '11111111-1111-1111-1111-111111111111', 'Books', 'books', 'Books and educational materials', true, 3),
('10000000-1111-1111-1111-111111111114', '11111111-1111-1111-1111-111111111111', 'Home & Garden', 'home-garden', 'Home improvement and garden supplies', true, 4),
('10000000-1111-1111-1111-111111111115', '11111111-1111-1111-1111-111111111111', 'Sports', 'sports', 'Sports equipment and outdoor gear', true, 5),

-- Tenant 2 Categories
('20000000-2222-2222-2222-222222222221', '22222222-2222-2222-2222-222222222222', 'Electronics', 'electronics-t2', 'Electronic devices and accessories', true, 1),
('20000000-2222-2222-2222-222222222222', '22222222-2222-2222-2222-222222222222', 'Furniture', 'furniture-t2', 'Home and office furniture', true, 2),
('20000000-2222-2222-2222-222222222223', '22222222-2222-2222-2222-222222222222', 'Beauty', 'beauty-t2', 'Beauty and personal care products', true, 3),

-- Tenant 3 Categories
('30000000-3333-3333-3333-333333333331', '33333333-3333-3333-3333-333333333333', 'Toys', 'toys-t3', 'Toys and games for all ages', true, 1),
('30000000-3333-3333-3333-333333333332', '33333333-3333-3333-3333-333333333333', 'Pet Supplies', 'pet-supplies-t3', 'Pet food, toys, and accessories', true, 2);

-- ============================================
-- PRODUCTS - Tenant 1
-- ============================================

INSERT INTO products (id, tenant_id, vendor_id, category_id, name, slug, sku, description, price, compare_price, cost_price, status, inventory_status, quantity, min_order_qty, max_order_qty, low_stock_threshold, currency_code, tags, attributes, images, created_by, updated_by) VALUES
-- Electronics Products
('a0000001-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 'vendor-001', '10000000-1111-1111-1111-111111111111',
'Wireless Bluetooth Headphones', 'wireless-bluetooth-headphones', 'WBH-001',
'Premium wireless headphones with active noise cancellation and 30-hour battery life',
'299.99', '349.99', '150.00', 'ACTIVE', 'IN_STOCK', 50, 1, 10, 10, 'USD',
'{"tags": ["wireless", "bluetooth", "headphones", "audio", "noise-cancelling"]}'::jsonb,
null,
'[{"id": "img1", "url": "https://example.com/headphones-1.jpg", "altText": "Black wireless headphones", "position": 1}]'::jsonb,
'admin-user-1', 'admin-user-1'),

('a0000002-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 'vendor-001', '10000000-1111-1111-1111-111111111111',
'Smart Watch Pro', 'smart-watch-pro', 'SWP-001',
'Advanced smartwatch with fitness tracking, heart rate monitor, and GPS',
'399.99', '499.99', '200.00', 'ACTIVE', 'IN_STOCK', 35, 1, 5, 5, 'USD',
null,
null,
'[{"id": "img2", "url": "https://example.com/smartwatch-1.jpg", "altText": "Smart watch with black band", "position": 1}]'::jsonb,
'admin-user-1', 'admin-user-1'),

('a0000003-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 'vendor-002', '10000000-1111-1111-1111-111111111111',
'4K Webcam', '4k-webcam', 'WC4K-001',
'Ultra HD 4K webcam with auto-focus and built-in microphone for professional video calls',
'149.99', '199.99', '75.00', 'ACTIVE', 'LOW_STOCK', 8, 1, 20, 5, 'USD',
null,
null,
'[{"id": "img3", "url": "https://example.com/webcam-1.jpg", "altText": "4K webcam", "position": 1}]'::jsonb,
'admin-user-1', 'admin-user-1'),

-- Clothing Products
('a0000004-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 'vendor-003', '10000000-1111-1111-1111-111111111112',
'Premium Cotton T-Shirt', 'premium-cotton-tshirt', 'PCT-001',
'100% organic cotton t-shirt with comfortable fit and premium quality',
'29.99', '39.99', '12.00', 'ACTIVE', 'IN_STOCK', 150, 1, 50, 20, 'USD',
null,
null,
'[{"id": "img4", "url": "https://example.com/tshirt-1.jpg", "altText": "White cotton t-shirt", "position": 1}]'::jsonb,
'admin-user-1', 'admin-user-1'),

('a0000005-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 'vendor-003', '10000000-1111-1111-1111-111111111112',
'Denim Jeans Classic', 'denim-jeans-classic', 'DJC-001',
'Classic fit denim jeans with stretch comfort and durable construction',
'79.99', '99.99', '35.00', 'ACTIVE', 'IN_STOCK', 80, 1, 20, 15, 'USD',
null,
null,
'[{"id": "img5", "url": "https://example.com/jeans-1.jpg", "altText": "Blue denim jeans", "position": 1}]'::jsonb,
'admin-user-1', 'admin-user-1'),

-- Books
('a0000006-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 'vendor-004', '10000000-1111-1111-1111-111111111113',
'JavaScript: The Complete Guide', 'javascript-complete-guide', 'JSG-001',
'Comprehensive guide to modern JavaScript from basics to advanced concepts',
'49.99', '59.99', '20.00', 'ACTIVE', 'IN_STOCK', 30, 1, 10, 5, 'USD',
null,
null,
'[{"id": "img6", "url": "https://example.com/book-js-1.jpg", "altText": "JavaScript book cover", "position": 1}]'::jsonb,
'admin-user-1', 'admin-user-1'),

-- Sports
('a0000007-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 'vendor-005', '10000000-1111-1111-1111-111111111115',
'Yoga Mat Pro', 'yoga-mat-pro', 'YMP-001',
'Professional non-slip yoga mat with extra cushioning and carrying strap',
'39.99', '49.99', '15.00', 'ACTIVE', 'IN_STOCK', 60, 1, 20, 10, 'USD',
null,
null,
'[{"id": "img7", "url": "https://example.com/yoga-mat-1.jpg", "altText": "Purple yoga mat", "position": 1}]'::jsonb,
'admin-user-1', 'admin-user-1');

-- ============================================
-- PRODUCTS - Tenant 2
-- ============================================

INSERT INTO products (id, tenant_id, vendor_id, category_id, name, slug, sku, description, price, status, inventory_status, quantity, currency_code, tags, attributes, created_by, updated_by) VALUES
('a0000008-2222-2222-2222-222222222222', '22222222-2222-2222-2222-222222222222', 'vendor-101', '20000000-2222-2222-2222-222222222221',
'Wireless Gaming Mouse', 'wireless-gaming-mouse-t2', 'WGM-T2-001',
'High-precision wireless gaming mouse with RGB lighting and programmable buttons',
'79.99', 'ACTIVE', 'IN_STOCK', 40, 'USD',
null,
null,
'admin-user-2', 'admin-user-2'),

('a0000009-2222-2222-2222-222222222222', '22222222-2222-2222-2222-222222222222', 'vendor-102', '20000000-2222-2222-2222-222222222222',
'Executive Office Chair', 'executive-office-chair-t2', 'EOC-T2-001',
'Ergonomic executive chair with lumbar support and premium leather upholstery',
'349.99', 'ACTIVE', 'IN_STOCK', 15, 'USD',
null,
null,
'admin-user-2', 'admin-user-2');

-- ============================================
-- PRODUCTS - Tenant 3
-- ============================================

INSERT INTO products (id, tenant_id, vendor_id, category_id, name, slug, sku, description, price, status, inventory_status, quantity, currency_code, tags, attributes, created_by, updated_by) VALUES
('a0000010-3333-3333-3333-333333333333', '33333333-3333-3333-3333-333333333333', 'vendor-201', '30000000-3333-3333-3333-333333333331',
'Educational Building Blocks', 'educational-building-blocks-t3', 'EBB-T3-001',
'Colorful building blocks set for children ages 3+ with 200 pieces',
'34.99', 'ACTIVE', 'IN_STOCK', 55, 'USD',
null,
null,
'admin-user-3', 'admin-user-3'),

('a0000011-3333-3333-3333-333333333333', '33333333-3333-3333-3333-333333333333', 'vendor-202', '30000000-3333-3333-3333-333333333332',
'Premium Dog Food', 'premium-dog-food-t3', 'PDF-T3-001',
'Natural grain-free dog food with real chicken and vegetables - 25lb bag',
'54.99', 'ACTIVE', 'IN_STOCK', 25, 'USD',
null,
null,
'admin-user-3', 'admin-user-3');

-- ============================================
-- PRODUCT VARIANTS - For products with variations
-- ============================================

INSERT INTO product_variants (id, product_id, sku, name, price, compare_price, quantity, low_stock_threshold, inventory_status, attributes) VALUES
-- Wireless Headphones Variants (Color variations)
('b0000001-1111-1111-1111-111111111111', 'a0000001-1111-1111-1111-111111111111', 'WBH-001-BLACK', 'Wireless Bluetooth Headphones - Black', '299.99', '349.99', 30, 5, 'IN_STOCK',
'[{"name": "Color", "value": "Black", "type": "color"}]'::jsonb),

('b0000002-1111-1111-1111-111111111111', 'a0000001-1111-1111-1111-111111111111', 'WBH-001-WHITE', 'Wireless Bluetooth Headphones - White', '299.99', '349.99', 20, 5, 'IN_STOCK',
'[{"name": "Color", "value": "White", "type": "color"}]'::jsonb),

-- T-Shirt Variants (Size and Color combinations)
('b0000003-1111-1111-1111-111111111111', 'a0000004-1111-1111-1111-111111111111', 'PCT-001-S-WHITE', 'Premium Cotton T-Shirt - Small White', '29.99', '39.99', 30, 5, 'IN_STOCK',
'[{"name": "Size", "value": "S", "type": "size"}, {"name": "Color", "value": "White", "type": "color"}]'::jsonb),

('b0000004-1111-1111-1111-111111111111', 'a0000004-1111-1111-1111-111111111111', 'PCT-001-M-WHITE', 'Premium Cotton T-Shirt - Medium White', '29.99', '39.99', 40, 5, 'IN_STOCK',
'[{"name": "Size", "value": "M", "type": "size"}, {"name": "Color", "value": "White", "type": "color"}]'::jsonb),

('b0000005-1111-1111-1111-111111111111', 'a0000004-1111-1111-1111-111111111111', 'PCT-001-L-WHITE', 'Premium Cotton T-Shirt - Large White', '29.99', '39.99', 35, 5, 'IN_STOCK',
'[{"name": "Size", "value": "L", "type": "size"}, {"name": "Color", "value": "White", "type": "color"}]'::jsonb),

('b0000006-1111-1111-1111-111111111111', 'a0000004-1111-1111-1111-111111111111', 'PCT-001-M-BLACK', 'Premium Cotton T-Shirt - Medium Black', '29.99', '39.99', 45, 5, 'IN_STOCK',
'[{"name": "Size", "value": "M", "type": "size"}, {"name": "Color", "value": "Black", "type": "color"}]'::jsonb),

-- Jeans Variants (Size variations)
('b0000007-1111-1111-1111-111111111111', 'a0000005-1111-1111-1111-111111111111', 'DJC-001-30', 'Denim Jeans Classic - W30', '79.99', '99.99', 20, 3, 'IN_STOCK',
'[{"name": "Waist", "value": "30", "type": "size"}]'::jsonb),

('b0000008-1111-1111-1111-111111111111', 'a0000005-1111-1111-1111-111111111111', 'DJC-001-32', 'Denim Jeans Classic - W32', '79.99', '99.99', 25, 3, 'IN_STOCK',
'[{"name": "Waist", "value": "32", "type": "size"}]'::jsonb),

('b0000009-1111-1111-1111-111111111111', 'a0000005-1111-1111-1111-111111111111', 'DJC-001-34', 'Denim Jeans Classic - W34', '79.99', '99.99', 20, 3, 'IN_STOCK',
'[{"name": "Waist", "value": "34", "type": "size"}]'::jsonb),

('b0000010-1111-1111-1111-111111111111', 'a0000005-1111-1111-1111-111111111111', 'DJC-001-36', 'Denim Jeans Classic - W36', '79.99', '99.99', 15, 3, 'LOW_STOCK',
'[{"name": "Waist", "value": "36", "type": "size"}]'::jsonb);

COMMIT;
