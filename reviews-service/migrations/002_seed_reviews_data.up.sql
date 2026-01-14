-- Seed data for reviews service
INSERT INTO reviews (
    id,
    tenant_id,
    application_id,
    entity_type,
    entity_id,
    user_id,
    user_name,
    user_email,
    title,
    content,
    status,
    ratings,
    tags,
    metadata,
    created_at,
    updated_at
) VALUES 
-- Product Reviews
(
    gen_random_uuid(),
    'default-tenant',
    'ecommerce-app',
    'product',
    'prod-1',
    'user-1',
    'Alice Johnson',
    'alice@example.com',
    'Excellent Product Quality!',
    'This product exceeded my expectations. The build quality is fantastic and it arrived exactly as described. Highly recommend to anyone looking for a reliable solution.',
    'APPROVED',
    '{"overall": 5, "quality": 5, "value": 4, "delivery": 5}',
    '["quality", "recommended", "fast-delivery"]',
    '{"verified_purchase": true, "helpful_votes": 15, "product_variant": "blue", "purchase_date": "2024-01-15"}',
    NOW() - INTERVAL '5 days',
    NOW() - INTERVAL '5 days'
),
(
    gen_random_uuid(),
    'default-tenant',
    'ecommerce-app',
    'product',
    'prod-1',
    'user-2',
    'Bob Smith',
    'bob@example.com',
    'Good value for money',
    'Decent product for the price point. Has some minor issues but overall satisfied with the purchase. Customer service was helpful when I had questions.',
    'APPROVED',
    '{"overall": 4, "quality": 3, "value": 5, "delivery": 4}',
    '["value", "customer-service"]',
    '{"verified_purchase": true, "helpful_votes": 8, "product_variant": "red", "purchase_date": "2024-01-18"}',
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '3 days'
),
(
    gen_random_uuid(),
    'default-tenant',
    'ecommerce-app',
    'product',
    'prod-2',
    'user-3',
    'Carol Williams',
    'carol@example.com',
    'Not what I expected',
    'The product description was misleading. While the product works fine, it does not match what was advertised. Would appreciate more accurate descriptions.',
    'APPROVED',
    '{"overall": 2, "quality": 3, "value": 2, "delivery": 4}',
    '["misleading", "description-issues"]',
    '{"verified_purchase": true, "helpful_votes": 3, "product_variant": "green", "purchase_date": "2024-01-20"}',
    NOW() - INTERVAL '2 days',
    NOW() - INTERVAL '2 days'
),
-- Service Reviews
(
    gen_random_uuid(),
    'default-tenant',
    'services-app',
    'service',
    'service-1',
    'user-4',
    'David Brown',
    'david@example.com',
    'Outstanding Customer Support',
    'The support team went above and beyond to help resolve my issue. Response time was quick and the solution provided was exactly what I needed. Will definitely use this service again.',
    'APPROVED',
    '{"overall": 5, "support": 5, "response_time": 5, "solution_quality": 5}',
    '["support", "quick-response", "helpful"]',
    '{"verified_customer": true, "helpful_votes": 22, "service_tier": "premium", "issue_resolved": true}',
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day'
),
(
    gen_random_uuid(),
    'default-tenant',
    'services-app',
    'service',
    'service-2',
    'user-5',
    'Emma Davis',
    'emma@example.com',
    'Mixed Experience',
    'The service itself is good but the onboarding process was confusing. Documentation could be improved. Once setup, everything works smoothly.',
    'APPROVED',
    '{"overall": 3, "ease_of_use": 2, "functionality": 4, "documentation": 2}',
    '["onboarding", "documentation", "functionality"]',
    '{"verified_customer": true, "helpful_votes": 5, "service_tier": "standard", "setup_time_hours": 4}',
    NOW() - INTERVAL '6 hours',
    NOW() - INTERVAL '6 hours'
),
-- Pending Review
(
    gen_random_uuid(),
    'default-tenant',
    'ecommerce-app',
    'product',
    'prod-3',
    'user-6',
    'Frank Miller',
    'frank@example.com',
    'Latest Purchase Review',
    'Just received this product and initial impressions are positive. Will update after using it for a few weeks. Packaging was excellent.',
    'PENDING',
    '{"overall": 4, "packaging": 5, "first_impression": 4}',
    '["new-purchase", "initial-review"]',
    '{"verified_purchase": true, "helpful_votes": 0, "product_variant": "black", "purchase_date": "2024-01-25"}',
    NOW() - INTERVAL '2 hours',
    NOW() - INTERVAL '2 hours'
),
-- Rejected Review (spam)
(
    gen_random_uuid(),
    'default-tenant',
    'ecommerce-app',
    'product',
    'prod-1',
    'user-7',
    'Spam User',
    'spam@example.com',
    'CHECK OUT MY AMAZING DEALS!!!',
    'Buy now at my website for 90% discount!!! Best deals ever!!! Click here now!!!',
    'REJECTED',
    '{"overall": 5}',
    '["spam", "promotional"]',
    '{"spam_score": 0.95, "auto_rejected": true, "rejection_reason": "spam_content"}',
    NOW() - INTERVAL '12 hours',
    NOW() - INTERVAL '12 hours'
);

-- Insert review comments
INSERT INTO review_comments (
    id,
    review_id,
    user_id,
    user_name,
    content,
    created_at,
    updated_at
) VALUES
(
    gen_random_uuid(),
    (SELECT id FROM reviews WHERE user_name = 'Alice Johnson' LIMIT 1),
    'user-8',
    'Grace Taylor',
    'Thanks for the detailed review! This helped me make my decision.',
    NOW() - INTERVAL '4 days',
    NOW() - INTERVAL '4 days'
),
(
    gen_random_uuid(),
    (SELECT id FROM reviews WHERE user_name = 'Bob Smith' LIMIT 1),
    'user-9',
    'Henry Wilson',
    'I had the same experience with customer service. They are really helpful!',
    NOW() - INTERVAL '2 days',
    NOW() - INTERVAL '2 days'
);

-- Insert review reactions
INSERT INTO review_reactions (
    id,
    review_id,
    user_id,
    reaction_type,
    created_at
) VALUES
(gen_random_uuid(), (SELECT id FROM reviews WHERE user_name = 'Alice Johnson' LIMIT 1), 'user-10', 'HELPFUL', NOW() - INTERVAL '4 days'),
(gen_random_uuid(), (SELECT id FROM reviews WHERE user_name = 'Alice Johnson' LIMIT 1), 'user-11', 'HELPFUL', NOW() - INTERVAL '4 days'),
(gen_random_uuid(), (SELECT id FROM reviews WHERE user_name = 'Bob Smith' LIMIT 1), 'user-12', 'HELPFUL', NOW() - INTERVAL '3 days'),
(gen_random_uuid(), (SELECT id FROM reviews WHERE user_name = 'David Brown' LIMIT 1), 'user-13', 'HELPFUL', NOW() - INTERVAL '1 day'),
(gen_random_uuid(), (SELECT id FROM reviews WHERE user_name = 'David Brown' LIMIT 1), 'user-14', 'THUMBS_UP', NOW() - INTERVAL '1 day');