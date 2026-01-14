-- Categories Service - Database Seed Script
-- This script seeds the database with sample categories for testing

-- Clean up existing data (optional - comment out if you want to keep existing data)
-- TRUNCATE TABLE category_audit CASCADE;
-- TRUNCATE TABLE categories CASCADE;

-- Insert root-level categories
INSERT INTO categories (id, tenant_id, created_by_id, updated_by_id, name, slug, description, level, position, is_active, status, created_at, updated_at)
VALUES
    -- Electronics
    ('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Electronics', 'electronics', 'Electronic devices, gadgets, and accessories',
     0, 1, true, 'APPROVED', NOW(), NOW()),

    -- Fashion
    ('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Fashion', 'fashion', 'Clothing, accessories, and footwear for men, women, and children',
     0, 2, true, 'APPROVED', NOW(), NOW()),

    -- Home & Garden
    ('33333333-3333-3333-3333-333333333333', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Home & Garden', 'home-garden', 'Furniture, decor, appliances, and gardening supplies',
     0, 3, true, 'APPROVED', NOW(), NOW()),

    -- Sports & Outdoors
    ('44444444-4444-4444-4444-444444444444', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Sports & Outdoors', 'sports-outdoors', 'Sporting goods, outdoor equipment, and fitness accessories',
     0, 4, true, 'APPROVED', NOW(), NOW()),

    -- Books & Media
    ('55555555-5555-5555-5555-555555555555', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Books & Media', 'books-media', 'Books, music, movies, and educational content',
     0, 5, true, 'APPROVED', NOW(), NOW()),

    -- Health & Beauty
    ('66666666-6666-6666-6666-666666666666', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Health & Beauty', 'health-beauty', 'Personal care, cosmetics, and wellness products',
     0, 6, true, 'APPROVED', NOW(), NOW()),

    -- Toys & Games
    ('77777777-7777-7777-7777-777777777777', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Toys & Games', 'toys-games', 'Toys, board games, video games, and hobby supplies',
     0, 7, true, 'APPROVED', NOW(), NOW()),

    -- Automotive
    ('88888888-8888-8888-8888-888888888888', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Automotive', 'automotive', 'Car parts, accessories, and maintenance supplies',
     0, 8, true, 'APPROVED', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert sub-categories for Electronics
INSERT INTO categories (id, tenant_id, created_by_id, updated_by_id, name, slug, description, parent_id, level, position, is_active, status, created_at, updated_at)
VALUES
    -- Electronics > Smartphones
    ('11111111-1111-1111-1111-111111111112', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Smartphones', 'electronics-smartphones', 'Mobile phones and accessories',
     '11111111-1111-1111-1111-111111111111', 1, 1, true, 'APPROVED', NOW(), NOW()),

    -- Electronics > Laptops
    ('11111111-1111-1111-1111-111111111113', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Laptops', 'electronics-laptops', 'Portable computers and notebooks',
     '11111111-1111-1111-1111-111111111111', 1, 2, true, 'APPROVED', NOW(), NOW()),

    -- Electronics > Tablets
    ('11111111-1111-1111-1111-111111111114', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Tablets', 'electronics-tablets', 'Tablet computers and e-readers',
     '11111111-1111-1111-1111-111111111111', 1, 3, true, 'APPROVED', NOW(), NOW()),

    -- Electronics > Cameras
    ('11111111-1111-1111-1111-111111111115', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Cameras', 'electronics-cameras', 'Digital cameras, DSLRs, and photography equipment',
     '11111111-1111-1111-1111-111111111111', 1, 4, true, 'APPROVED', NOW(), NOW()),

    -- Electronics > Audio Equipment
    ('11111111-1111-1111-1111-111111111116', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Audio Equipment', 'electronics-audio-equipment', 'Headphones, speakers, and sound systems',
     '11111111-1111-1111-1111-111111111111', 1, 5, true, 'APPROVED', NOW(), NOW()),

    -- Electronics > Smart Home
    ('11111111-1111-1111-1111-111111111117', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Smart Home', 'electronics-smart-home', 'Smart home devices and automation',
     '11111111-1111-1111-1111-111111111111', 1, 6, true, 'APPROVED', NOW(), NOW()),

    -- Electronics > Gaming Consoles
    ('11111111-1111-1111-1111-111111111118', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Gaming Consoles', 'electronics-gaming-consoles', 'Video game consoles and accessories',
     '11111111-1111-1111-1111-111111111111', 1, 7, true, 'APPROVED', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert sub-categories for Fashion
INSERT INTO categories (id, tenant_id, created_by_id, updated_by_id, name, slug, description, parent_id, level, position, is_active, status, created_at, updated_at)
VALUES
    -- Fashion > Men's Clothing
    ('22222222-2222-2222-2222-222222222223', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Men''s Clothing', 'fashion-mens-clothing', 'Clothing and apparel for men',
     '22222222-2222-2222-2222-222222222222', 1, 1, true, 'APPROVED', NOW(), NOW()),

    -- Fashion > Women's Clothing
    ('22222222-2222-2222-2222-222222222224', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Women''s Clothing', 'fashion-womens-clothing', 'Clothing and apparel for women',
     '22222222-2222-2222-2222-222222222222', 1, 2, true, 'APPROVED', NOW(), NOW()),

    -- Fashion > Shoes
    ('22222222-2222-2222-2222-222222222225', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Shoes', 'fashion-shoes', 'Footwear for all occasions',
     '22222222-2222-2222-2222-222222222222', 1, 3, true, 'APPROVED', NOW(), NOW()),

    -- Fashion > Accessories
    ('22222222-2222-2222-2222-222222222226', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Accessories', 'fashion-accessories', 'Bags, belts, jewelry, and fashion accessories',
     '22222222-2222-2222-2222-222222222222', 1, 4, true, 'APPROVED', NOW(), NOW()),

    -- Fashion > Kids' Clothing
    ('22222222-2222-2222-2222-222222222227', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Kids'' Clothing', 'fashion-kids-clothing', 'Clothing for children',
     '22222222-2222-2222-2222-222222222222', 1, 5, true, 'APPROVED', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert sub-categories for Home & Garden
INSERT INTO categories (id, tenant_id, created_by_id, updated_by_id, name, slug, description, parent_id, level, position, is_active, status, created_at, updated_at)
VALUES
    -- Home & Garden > Furniture
    ('33333333-3333-3333-3333-333333333334', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Furniture', 'home-garden-furniture', 'Indoor and outdoor furniture',
     '33333333-3333-3333-3333-333333333333', 1, 1, true, 'APPROVED', NOW(), NOW()),

    -- Home & Garden > Kitchen & Dining
    ('33333333-3333-3333-3333-333333333335', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Kitchen & Dining', 'home-garden-kitchen-dining', 'Cookware, dinnerware, and kitchen appliances',
     '33333333-3333-3333-3333-333333333333', 1, 2, true, 'APPROVED', NOW(), NOW()),

    -- Home & Garden > Bedding & Bath
    ('33333333-3333-3333-3333-333333333336', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Bedding & Bath', 'home-garden-bedding-bath', 'Bedding, towels, and bathroom accessories',
     '33333333-3333-3333-3333-333333333333', 1, 3, true, 'APPROVED', NOW(), NOW()),

    -- Home & Garden > Home Decor
    ('33333333-3333-3333-3333-333333333337', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Home Decor', 'home-garden-home-decor', 'Decorative items and home accessories',
     '33333333-3333-3333-3333-333333333333', 1, 4, true, 'APPROVED', NOW(), NOW()),

    -- Home & Garden > Gardening
    ('33333333-3333-3333-3333-333333333338', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Gardening', 'home-garden-gardening', 'Garden tools, plants, and outdoor supplies',
     '33333333-3333-3333-3333-333333333333', 1, 5, true, 'APPROVED', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert some third-level categories for demonstration
INSERT INTO categories (id, tenant_id, created_by_id, updated_by_id, name, slug, description, parent_id, level, position, is_active, status, created_at, updated_at)
VALUES
    -- Electronics > Smartphones > Android
    ('11111111-1111-1111-1111-111111111119', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Android Phones', 'electronics-smartphones-android', 'Android-based smartphones',
     '11111111-1111-1111-1111-111111111112', 2, 1, true, 'APPROVED', NOW(), NOW()),

    -- Electronics > Smartphones > iOS
    ('11111111-1111-1111-1111-111111111120', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'iPhones', 'electronics-smartphones-iphones', 'Apple iPhones',
     '11111111-1111-1111-1111-111111111112', 2, 2, true, 'APPROVED', NOW(), NOW()),

    -- Electronics > Laptops > Windows
    ('11111111-1111-1111-1111-111111111121', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Windows Laptops', 'electronics-laptops-windows', 'Windows-based laptops',
     '11111111-1111-1111-1111-111111111113', 2, 1, true, 'APPROVED', NOW(), NOW()),

    -- Electronics > Laptops > MacBooks
    ('11111111-1111-1111-1111-111111111122', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'MacBooks', 'electronics-laptops-macbooks', 'Apple MacBooks',
     '11111111-1111-1111-1111-111111111113', 2, 2, true, 'APPROVED', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert a few draft and pending categories for testing
INSERT INTO categories (id, tenant_id, created_by_id, updated_by_id, name, slug, description, level, position, is_active, status, created_at, updated_at)
VALUES
    ('99999999-9999-9999-9999-999999999991', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Pet Supplies', 'pet-supplies', 'Food, toys, and accessories for pets',
     0, 9, true, 'DRAFT', NOW(), NOW()),

    ('99999999-9999-9999-9999-999999999992', '00000000-0000-0000-0000-000000000001', 'admin-user', 'admin-user',
     'Office Supplies', 'office-supplies', 'Stationery, office equipment, and supplies',
     0, 10, false, 'PENDING', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Summary
SELECT 'Seed data inserted successfully!' as message;
SELECT
    status,
    COUNT(*) as count
FROM categories
WHERE tenant_id = '00000000-0000-0000-0000-000000000001'
GROUP BY status
ORDER BY status;

SELECT
    level,
    COUNT(*) as count
FROM categories
WHERE tenant_id = '00000000-0000-0000-0000-000000000001'
GROUP BY level
ORDER BY level;