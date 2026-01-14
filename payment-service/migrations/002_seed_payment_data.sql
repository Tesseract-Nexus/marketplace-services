-- Seed Payment Gateway Data
-- Includes Indian payment gateways (Razorpay, PayU, Cashfree) and international options

-- Payment Settings for test tenant
INSERT INTO payment_settings (tenant_id, default_currency, supported_currencies, enable_express_checkout, enable_3d_secure) VALUES
('test-tenant', 'INR', ARRAY['INR', 'USD', 'EUR'], true, true)
ON CONFLICT (tenant_id) DO NOTHING;

-- Razorpay (Primary for India)
INSERT INTO payment_gateway_configs (
    tenant_id, gateway_type, display_name, is_enabled, is_test_mode,
    api_key_public, api_key_secret, webhook_secret,
    supports_payments, supports_refunds, supports_subscriptions,
    minimum_amount, maximum_amount, priority, description, config
) VALUES (
    'test-tenant',
    'RAZORPAY',
    'Razorpay',
    true,
    true,
    'rzp_test_XXXXXXXXXXXXXXXX', -- Test public key
    'ENCRYPTED:rzp_test_secret_XXXXXXXXXXXX', -- Test secret key (encrypted)
    'ENCRYPTED:webhook_secret_XXXXXXXXXXXX',
    true,
    true,
    true,
    1.00, -- ₹1 minimum
    1000000.00, -- ₹10 lakh maximum
    1,
    'Accept payments via UPI, Cards, Net Banking, Wallets (PhonePe, Paytm, Google Pay)',
    '{"payment_methods": ["card", "upi", "netbanking", "wallet"], "upi_enabled": true, "international_cards": false}'::jsonb
) ON CONFLICT (tenant_id, gateway_type) DO NOTHING;

-- PayU India
INSERT INTO payment_gateway_configs (
    tenant_id, gateway_type, display_name, is_enabled, is_test_mode,
    api_key_public, api_key_secret, webhook_secret,
    supports_payments, supports_refunds, supports_subscriptions,
    minimum_amount, maximum_amount, priority, description, config
) VALUES (
    'test-tenant',
    'PAYU_INDIA',
    'PayU India',
    false, -- Disabled by default
    true,
    'merchant_key_XXXXXXXX',
    'ENCRYPTED:salt_XXXXXXXXXXXXXXXX',
    'ENCRYPTED:webhook_secret_XXXX',
    true,
    true,
    false,
    10.00,
    500000.00,
    2,
    'PayU India - Cards, Net Banking, EMI, Wallets',
    '{"payment_methods": ["card", "netbanking", "emi", "wallet"]}'::jsonb
) ON CONFLICT (tenant_id, gateway_type) DO NOTHING;

-- Cashfree
INSERT INTO payment_gateway_configs (
    tenant_id, gateway_type, display_name, is_enabled, is_test_mode,
    api_key_public, api_key_secret, webhook_secret,
    supports_payments, supports_refunds, supports_subscriptions,
    minimum_amount, maximum_amount, priority, description, config
) VALUES (
    'test-tenant',
    'CASHFREE',
    'Cashfree',
    false,
    true,
    'app_id_test_XXXXXXXXXXXXXXXX',
    'ENCRYPTED:secret_key_test_XXXXXXXXXXXX',
    'ENCRYPTED:webhook_secret_XXXX',
    true,
    true,
    true,
    1.00,
    1000000.00,
    3,
    'Cashfree Payments - UPI, Cards, Net Banking, Paylater',
    '{"payment_methods": ["upi", "card", "netbanking", "paylater"], "auto_collect": true}'::jsonb
) ON CONFLICT (tenant_id, gateway_type) DO NOTHING;

-- Stripe (for international/US)
INSERT INTO payment_gateway_configs (
    tenant_id, gateway_type, display_name, is_enabled, is_test_mode,
    api_key_public, api_key_secret, webhook_secret,
    supports_payments, supports_refunds, supports_subscriptions,
    minimum_amount, maximum_amount, priority, description, config
) VALUES (
    'test-tenant',
    'STRIPE',
    'Stripe (International)',
    false, -- Disabled by default (not for India)
    true,
    'REDACTED',
    'ENCRYPTED:REDACTED',
    'ENCRYPTED:whsec_XXXXXXXXXXXXXXXXXX',
    true,
    true,
    true,
    0.50, -- $0.50 minimum USD
    999999.99,
    10, -- Lower priority
    'International credit/debit cards (Visa, Mastercard, Amex)',
    '{"payment_methods": ["card", "apple_pay", "google_pay"], "capture_method": "automatic"}'::jsonb
) ON CONFLICT (tenant_id, gateway_type) DO NOTHING;

-- PayPal (for international)
INSERT INTO payment_gateway_configs (
    tenant_id, gateway_type, display_name, is_enabled, is_test_mode,
    api_key_public, api_key_secret, webhook_secret,
    supports_payments, supports_refunds, supports_subscriptions,
    minimum_amount, maximum_amount, priority, description, config
) VALUES (
    'test-tenant',
    'PAYPAL',
    'PayPal',
    false,
    true,
    'client_id_XXXXXXXXXXXXXXXXXXXXXXXX',
    'ENCRYPTED:secret_XXXXXXXXXXXXXXXXXXXXXXXX',
    'ENCRYPTED:webhook_id_XXXX',
    true,
    true,
    true,
    1.00,
    10000.00,
    11,
    'Pay with PayPal balance or linked cards',
    '{"mode": "sandbox", "intent": "capture"}'::jsonb
) ON CONFLICT (tenant_id, gateway_type) DO NOTHING;

-- Paytm (India)
INSERT INTO payment_gateway_configs (
    tenant_id, gateway_type, display_name, is_enabled, is_test_mode,
    api_key_public, api_key_secret, webhook_secret,
    supports_payments, supports_refunds, supports_subscriptions,
    minimum_amount, maximum_amount, priority, description, config
) VALUES (
    'test-tenant',
    'PAYTM',
    'Paytm',
    false,
    true,
    'merchant_id_XXXXXXXX',
    'ENCRYPTED:merchant_key_XXXXXXXXXXXX',
    'ENCRYPTED:webhook_secret_XXXX',
    true,
    true,
    false,
    1.00,
    200000.00,
    4,
    'Paytm Wallet and UPI payments',
    '{"payment_methods": ["wallet", "upi", "card"], "channel_id": "WEB"}'::jsonb
) ON CONFLICT (tenant_id, gateway_type) DO NOTHING;

-- Sample Payment Transaction (test data)
INSERT INTO payment_transactions (
    tenant_id, order_id, customer_id, gateway_config_id, gateway_type,
    gateway_transaction_id, amount, currency, status, payment_method_type,
    card_brand, card_last_four, billing_email, billing_name, processed_at
) VALUES (
    'test-tenant',
    '11111111-1111-1111-1111-111111111111', -- Dummy order ID
    '11111111-1111-1111-1111-111111111111', -- VIP customer from customer seed
    (SELECT id FROM payment_gateway_configs WHERE gateway_type = 'RAZORPAY' AND tenant_id = 'test-tenant'),
    'RAZORPAY',
    'pay_test_KsGT4HlL6VZwRB',
    15680.50,
    'INR',
    'SUCCEEDED',
    'CARD',
    'Visa',
    '4242',
    'sarah.johnson@example.com',
    'Sarah Johnson',
    NOW() - INTERVAL '2 days'
);

-- Comments
COMMENT ON TABLE payment_gateway_configs IS 'Seeded with Razorpay (primary for India), PayU, Cashfree, Paytm, Stripe, PayPal';
