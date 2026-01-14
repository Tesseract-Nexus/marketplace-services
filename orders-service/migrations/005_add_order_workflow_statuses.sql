-- Migration: Add order workflow statuses
-- This migration adds payment_status and fulfillment_status to orders table
-- and migrates existing data to the new status values

-- Add new columns to orders table
ALTER TABLE orders ADD COLUMN IF NOT EXISTS payment_status VARCHAR(30) NOT NULL DEFAULT 'PENDING';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS fulfillment_status VARCHAR(30) NOT NULL DEFAULT 'UNFULFILLED';

-- Migrate existing order status values to new format
-- PENDING -> PLACED (order created, awaiting payment)
UPDATE orders SET status = 'PLACED' WHERE status = 'PENDING';

-- SHIPPED -> PROCESSING (we'll handle fulfillment separately)
UPDATE orders SET status = 'PROCESSING' WHERE status = 'SHIPPED';

-- DELIVERED -> COMPLETED
UPDATE orders SET status = 'COMPLETED' WHERE status = 'DELIVERED';

-- REFUNDED orders: keep as COMPLETED but update payment_status
UPDATE orders SET payment_status = 'REFUNDED' WHERE status = 'REFUNDED';
UPDATE orders SET status = 'COMPLETED' WHERE status = 'REFUNDED';

-- Migrate fulfillment status based on old order status
-- If order was SHIPPED, set fulfillment to DISPATCHED
UPDATE orders SET fulfillment_status = 'DISPATCHED'
WHERE status = 'PROCESSING' AND fulfillment_status = 'UNFULFILLED';

-- If order was DELIVERED (now COMPLETED), set fulfillment to DELIVERED
UPDATE orders SET fulfillment_status = 'DELIVERED'
WHERE status = 'COMPLETED' AND fulfillment_status = 'UNFULFILLED';

-- Sync payment_status from order_payments table
UPDATE orders o
SET payment_status = CASE
    WHEN op.status = 'COMPLETED' THEN 'PAID'
    WHEN op.status = 'FAILED' THEN 'FAILED'
    WHEN op.status = 'REFUNDED' THEN 'REFUNDED'
    ELSE 'PENDING'
END
FROM order_payments op
WHERE o.id = op.order_id AND o.payment_status = 'PENDING';

-- If payment is PAID, order should be at least CONFIRMED
UPDATE orders SET status = 'CONFIRMED'
WHERE payment_status = 'PAID' AND status = 'PLACED';

-- Create indexes for new columns
CREATE INDEX IF NOT EXISTS idx_orders_payment_status ON orders(payment_status);
CREATE INDEX IF NOT EXISTS idx_orders_fulfillment_status ON orders(fulfillment_status);
CREATE INDEX IF NOT EXISTS idx_orders_status_composite ON orders(tenant_id, status, payment_status, fulfillment_status);

-- Add comments for documentation
COMMENT ON COLUMN orders.status IS 'Overall order lifecycle: PLACED, CONFIRMED, PROCESSING, COMPLETED, CANCELLED';
COMMENT ON COLUMN orders.payment_status IS 'Payment flow: PENDING, PAID, FAILED, PARTIALLY_REFUNDED, REFUNDED';
COMMENT ON COLUMN orders.fulfillment_status IS 'Fulfillment tracking: UNFULFILLED, PROCESSING, PACKED, DISPATCHED, IN_TRANSIT, OUT_FOR_DELIVERY, DELIVERED, FAILED_DELIVERY, RETURNED';
