-- Add idempotency key column for duplicate order prevention
-- Nullable: backward compatible (old orders and admin-created orders won't have one)
-- Unique per tenant: composite index on (tenant_id, idempotency_key)
ALTER TABLE orders ADD COLUMN idempotency_key VARCHAR(255);

CREATE UNIQUE INDEX idx_orders_tenant_idempotency_key
  ON orders(tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
