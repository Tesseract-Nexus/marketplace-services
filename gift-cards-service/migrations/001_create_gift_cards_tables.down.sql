-- Drop indexes
DROP INDEX IF EXISTS idx_gift_card_transactions_created_at;
DROP INDEX IF EXISTS idx_gift_card_transactions_order_id;
DROP INDEX IF EXISTS idx_gift_card_transactions_gift_card_id;
DROP INDEX IF EXISTS idx_gift_card_transactions_tenant_id;

DROP INDEX IF EXISTS idx_gift_cards_deleted_at;
DROP INDEX IF EXISTS idx_gift_cards_expires_at;
DROP INDEX IF EXISTS idx_gift_cards_purchased_by;
DROP INDEX IF EXISTS idx_gift_cards_status;
DROP INDEX IF EXISTS idx_gift_cards_code;
DROP INDEX IF EXISTS idx_gift_cards_tenant_id;

-- Drop tables
DROP TABLE IF EXISTS gift_card_transactions;
DROP TABLE IF EXISTS gift_cards;
