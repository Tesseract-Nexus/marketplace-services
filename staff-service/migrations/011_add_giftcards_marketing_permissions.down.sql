-- Rollback: Remove Gift Cards and Marketing Permissions from Roles
-- Reverses migration 011

DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions
    WHERE name IN (
        'giftcards:view',
        'giftcards:create',
        'giftcards:edit',
        'giftcards:delete',
        'giftcards:redeem',
        'giftcards:balance:adjust',
        'giftcards:bulk:create',
        'giftcards:export',
        'giftcards:transactions:view',
        'marketing:coupons:view',
        'marketing:coupons:manage',
        'marketing:campaigns:view',
        'marketing:campaigns:manage',
        'marketing:email:send',
        'marketing:reviews:view',
        'marketing:reviews:moderate',
        'marketing:banners:manage'
    )
);
