-- Create gift_cards table
CREATE TABLE IF NOT EXISTS gift_cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    code VARCHAR(50) NOT NULL UNIQUE,
    initial_balance DECIMAL(10,2) NOT NULL,
    current_balance DECIMAL(10,2) NOT NULL,
    currency_code VARCHAR(3) NOT NULL DEFAULT 'USD',
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',

    -- Recipient information
    recipient_email VARCHAR(255),
    recipient_name VARCHAR(255),
    sender_name VARCHAR(255),
    message TEXT,

    -- Purchase information
    purchased_by UUID,
    purchase_order_id UUID,
    purchase_date TIMESTAMP,

    -- Expiration
    expires_at TIMESTAMP,

    -- Usage tracking
    last_used_at TIMESTAMP,
    usage_count INTEGER DEFAULT 0,

    -- Metadata
    metadata JSONB,
    notes TEXT,

    -- Audit fields
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_gift_cards_tenant_id ON gift_cards(tenant_id);
CREATE INDEX IF NOT EXISTS idx_gift_cards_code ON gift_cards(code);
CREATE INDEX IF NOT EXISTS idx_gift_cards_status ON gift_cards(status);
CREATE INDEX IF NOT EXISTS idx_gift_cards_purchased_by ON gift_cards(purchased_by);
CREATE INDEX IF NOT EXISTS idx_gift_cards_expires_at ON gift_cards(expires_at);
CREATE INDEX IF NOT EXISTS idx_gift_cards_deleted_at ON gift_cards(deleted_at);

-- Create gift_card_transactions table
CREATE TABLE IF NOT EXISTS gift_card_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    gift_card_id UUID NOT NULL REFERENCES gift_cards(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    balance_before DECIMAL(10,2) NOT NULL,
    balance_after DECIMAL(10,2) NOT NULL,

    -- Related entities
    order_id UUID,
    user_id UUID,

    -- Transaction details
    description TEXT,
    metadata JSONB,

    -- Audit
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(255)
);

-- Create indexes for transactions
CREATE INDEX IF NOT EXISTS idx_gift_card_transactions_tenant_id ON gift_card_transactions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_gift_card_transactions_gift_card_id ON gift_card_transactions(gift_card_id);
CREATE INDEX IF NOT EXISTS idx_gift_card_transactions_order_id ON gift_card_transactions(order_id);
CREATE INDEX IF NOT EXISTS idx_gift_card_transactions_created_at ON gift_card_transactions(created_at);
