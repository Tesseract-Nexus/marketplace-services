-- Remove seed data for reviews service
DELETE FROM review_reactions WHERE user_id LIKE 'user-%';
DELETE FROM review_comments WHERE user_id LIKE 'user-%';
DELETE FROM reviews WHERE tenant_id = 'default-tenant';