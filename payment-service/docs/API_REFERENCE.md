# Payment Service - API Reference

## Base URL

| Environment | URL |
|-------------|-----|
| DevTest | `https://dev-admin.tesserix.app/api/v1` |
| Production | `https://admin.yourdomain.com/api/v1` |

## Authentication

All API requests require the following headers:

```http
X-Tenant-ID: your-tenant-id
X-User-ID: user-uuid (optional, for audit)
X-Vendor-ID: vendor-uuid (optional, for multi-vendor)
Content-Type: application/json
```

## Rate Limits

| Operation | Rate | Burst |
|-----------|------|-------|
| Create Payment | 10/sec per tenant | 30 |
| Create Refund | 5/sec per tenant | 15 |
| General API | 100/sec per tenant | 200 |
| Webhooks | 500/sec per IP | 1000 |

---

## Payment Endpoints

### Create Payment Intent

Creates a new payment intent with the specified gateway.

```http
POST /payments/create-intent
```

**Request Body:**

```json
{
  "tenantId": "demo-store",
  "orderId": "order-uuid-here",
  "amount": 1500.00,
  "currency": "INR",
  "customerId": "customer-uuid",
  "gatewayType": "RAZORPAY",
  "paymentMethod": "CARD",
  "customerEmail": "customer@example.com",
  "customerPhone": "+919876543210",
  "customerName": "John Doe",
  "description": "Order #12345",
  "metadata": {
    "order_number": "12345",
    "product_name": "Widget Pro"
  }
}
```

**Gateway Types:**
- `RAZORPAY` - India (UPI, Cards, NetBanking, Wallets)
- `STRIPE` - International (Cards, Apple Pay, Google Pay)
- `PAYPAL` - International (PayPal, Cards)
- `PAYU` - India (Cards, UPI, EMI)
- `CASHFREE` - India (Cards, UPI, Paylater)
- `PHONEPE` - India (UPI, Wallet)
- `AFTERPAY` - AU/US/UK/NZ (BNPL)
- `ZIP` - AU/NZ (BNPL)

**Payment Method Types:**
- `CARD` - Credit/Debit Cards
- `UPI` - UPI (India)
- `NETBANKING` - Net Banking (India)
- `WALLET` - Digital Wallets
- `BANK_TRANSFER` - Bank Transfer
- `BNPL` - Buy Now Pay Later

**Response (Razorpay):**

```json
{
  "paymentIntentId": "pay_KsGT4HlL6VZwRB",
  "amount": 1500.00,
  "currency": "INR",
  "status": "created",
  "razorpayOrderId": "order_KsGT4HlL6VZwRB",
  "options": {
    "key": "rzp_test_XXXXXXXX",
    "order_id": "order_KsGT4HlL6VZwRB",
    "prefill": {
      "email": "customer@example.com",
      "contact": "+919876543210"
    }
  }
}
```

**Response (Stripe):**

```json
{
  "paymentIntentId": "pi_3L2XnG2eZvKYlo2C",
  "amount": 1500.00,
  "currency": "USD",
  "status": "requires_payment_method",
  "clientSecret": "pi_3L2XnG2eZvKYlo2C_secret_XXXX",
  "stripePublicKey": "pk_test_XXXX"
}
```

---

### Confirm Payment

Confirms a payment after customer completes on frontend.

```http
POST /payments/confirm
```

**Request Body:**

```json
{
  "paymentIntentId": "pay_KsGT4HlL6VZwRB",
  "gatewayTransactionId": "pay_KsGT4HlL6VZwRB",
  "signature": "razorpay_signature_here",
  "paymentDetails": {
    "razorpay_payment_id": "pay_KsGT4HlL6VZwRB",
    "razorpay_order_id": "order_KsGT4HlL6VZwRB",
    "razorpay_signature": "signature_here"
  }
}
```

**Response:**

```json
{
  "id": "uuid-here",
  "orderId": "order-uuid",
  "amount": 1500.00,
  "currency": "INR",
  "status": "succeeded",
  "paymentMethodType": "CARD",
  "gatewayTransactionId": "pay_KsGT4HlL6VZwRB",
  "cardBrand": "Visa",
  "cardLastFour": "4242",
  "processedAt": "2024-01-15T10:30:00Z",
  "createdAt": "2024-01-15T10:29:00Z"
}
```

---

### Get Payment Status

```http
GET /payments/:id
```

**Response:**

```json
{
  "id": "payment-uuid",
  "orderId": "order-uuid",
  "amount": 1500.00,
  "currency": "INR",
  "status": "succeeded",
  "paymentMethodType": "CARD",
  "gatewayTransactionId": "pay_KsGT4HlL6VZwRB",
  "cardBrand": "Visa",
  "cardLastFour": "4242",
  "billingEmail": "customer@example.com",
  "billingName": "John Doe",
  "processedAt": "2024-01-15T10:30:00Z",
  "createdAt": "2024-01-15T10:29:00Z"
}
```

**Payment Statuses:**
- `created` - Payment intent created
- `pending` - Awaiting payment
- `processing` - Payment being processed
- `requires_action` - 3D Secure required
- `succeeded` - Payment successful
- `failed` - Payment failed
- `cancelled` - Payment cancelled
- `refunded` - Fully refunded
- `partially_refunded` - Partially refunded

---

### Cancel Payment

```http
POST /payments/:id/cancel
```

**Response:**

```json
{
  "id": "payment-uuid",
  "status": "cancelled",
  "cancelledAt": "2024-01-15T10:35:00Z"
}
```

---

### Create Refund

```http
POST /payments/:id/refund
```

**Request Body:**

```json
{
  "amount": 500.00,
  "reason": "Customer requested refund",
  "notes": "Partial refund for damaged item"
}
```

**Response:**

```json
{
  "refundId": "refund-uuid",
  "paymentId": "payment-uuid",
  "amount": 500.00,
  "currency": "INR",
  "status": "pending",
  "gatewayRefundId": "rfnd_KsGT4HlL6VZwRB",
  "createdAt": "2024-01-15T10:40:00Z"
}
```

**Refund Statuses:**
- `pending` - Refund initiated
- `processing` - Being processed
- `succeeded` - Refund completed
- `failed` - Refund failed

---

### List Refunds by Payment

```http
GET /payments/:id/refunds
```

**Response:**

```json
{
  "refunds": [
    {
      "refundId": "refund-uuid",
      "amount": 500.00,
      "status": "succeeded",
      "createdAt": "2024-01-15T10:40:00Z"
    }
  ],
  "total": 1
}
```

---

### List Payments by Order

```http
GET /orders/:orderId/payments
```

---

## Gateway Configuration Endpoints

### List Gateway Configs

```http
GET /gateway-configs
```

**Response:**

```json
{
  "gateways": [
    {
      "id": "uuid",
      "gatewayType": "RAZORPAY",
      "displayName": "Razorpay India",
      "isEnabled": true,
      "isTestMode": true,
      "apiKeyPublic": "rzp_test_XXX",
      "supportsPayments": true,
      "supportsRefunds": true,
      "priority": 1
    }
  ]
}
```

---

### Create Gateway Config

```http
POST /gateway-configs
```

**Request Body:**

```json
{
  "tenantId": "demo-store",
  "gatewayType": "RAZORPAY",
  "displayName": "Razorpay India",
  "isEnabled": true,
  "isTestMode": true,
  "apiKeyPublic": "rzp_test_XXXXXXXXXXXX",
  "apiKeySecret": "secret_key_here",
  "webhookSecret": "webhook_secret_here",
  "supportsPayments": true,
  "supportsRefunds": true,
  "minimumAmount": 1.00,
  "maximumAmount": 500000.00,
  "priority": 1
}
```

---

### Get Gateway Templates

Returns pre-configured templates for each gateway type.

```http
GET /gateway-configs/templates
```

**Response:**

```json
{
  "templates": [
    {
      "gatewayType": "RAZORPAY",
      "displayName": "Razorpay",
      "description": "India's leading payment gateway",
      "supportedCountries": ["IN"],
      "supportedPaymentMethods": ["CARD", "UPI", "NETBANKING", "WALLET"],
      "requiredCredentials": ["apiKeyPublic", "apiKeySecret", "webhookSecret"],
      "setupInstructions": "1. Sign up at razorpay.com..."
    }
  ]
}
```

---

### Create Gateway from Template

```http
POST /gateway-configs/from-template/:gatewayType
```

**Request Body:**

```json
{
  "tenantId": "demo-store",
  "displayName": "My Razorpay",
  "isTestMode": true,
  "apiKeyPublic": "rzp_test_XXX",
  "apiKeySecret": "secret",
  "webhookSecret": "webhook_secret"
}
```

---

### Validate Gateway Credentials

```http
POST /gateway-configs/validate
```

**Request Body:**

```json
{
  "gatewayType": "RAZORPAY",
  "apiKeyPublic": "rzp_test_XXX",
  "apiKeySecret": "secret",
  "isTestMode": true
}
```

**Response:**

```json
{
  "valid": true,
  "message": "Credentials validated successfully"
}
```

---

## Gateway Selection Endpoints

### Get Available Gateways

Returns gateways available for a specific country.

```http
GET /gateways/available?countryCode=IN
```

---

### Get Payment Methods by Country

```http
GET /payment-methods/by-country/:countryCode
```

**Response:**

```json
{
  "countryCode": "IN",
  "paymentMethods": [
    {
      "type": "CARD",
      "displayName": "Credit/Debit Card",
      "gateways": ["RAZORPAY", "PAYU"]
    },
    {
      "type": "UPI",
      "displayName": "UPI",
      "gateways": ["RAZORPAY", "PHONEPE"]
    }
  ]
}
```

---

### Get Country Gateway Matrix

```http
GET /gateways/country-matrix
```

---

## Platform Fee Endpoints

### Calculate Platform Fees

```http
GET /platform-fees/calculate?amount=1000&currency=INR
```

**Response:**

```json
{
  "grossAmount": 1000.00,
  "platformFee": 50.00,
  "platformFeePercent": 0.05,
  "netAmount": 950.00,
  "currency": "INR"
}
```

---

### Get Fee Ledger

```http
GET /platform-fees/ledger?startDate=2024-01-01&endDate=2024-01-31
```

---

### Get Fee Summary

```http
GET /platform-fees/summary
```

---

## Payment Settings Endpoints

### Get Payment Settings

```http
GET /payment-settings
```

**Response:**

```json
{
  "defaultCurrency": "INR",
  "supportedCurrencies": ["INR", "USD"],
  "platformFeeEnabled": true,
  "platformFeePercent": 0.05,
  "enable3DSecure": true,
  "collectBillingAddress": true,
  "sendPaymentReceipts": true
}
```

---

### Update Payment Settings

```http
PUT /payment-settings
```

---

## Webhook Endpoints

Webhooks are called by payment gateways to notify of payment events.

| Gateway | Endpoint | Signature Header |
|---------|----------|------------------|
| Razorpay | `/webhooks/razorpay` | `X-Razorpay-Signature` |
| Stripe | `/webhooks/stripe` | `Stripe-Signature` |
| PayPal | `/webhooks/paypal` | `PAYPAL-TRANSMISSION-SIG` |
| PayU | `/webhooks/payu` | Hash in body |
| Cashfree | `/webhooks/cashfree` | `X-Cashfree-Signature` |
| PhonePe | `/webhooks/phonepe` | `X-VERIFY` |
| Afterpay | `/webhooks/afterpay` | Custom signature |
| Zip | `/webhooks/zip` | Custom signature |

---

## Error Responses

All errors follow this format:

```json
{
  "error": "Error type",
  "message": "Human readable message",
  "code": "ERROR_CODE"
}
```

**Common Error Codes:**

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Invalid request body |
| `UNAUTHORIZED` | 401 | Missing or invalid tenant ID |
| `PAYMENT_NOT_FOUND` | 404 | Payment doesn't exist |
| `GATEWAY_NOT_CONFIGURED` | 400 | Gateway not set up for tenant |
| `GATEWAY_ERROR` | 502 | Payment gateway returned error |
| `INSUFFICIENT_FUNDS` | 402 | Card has insufficient balance |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |

---

## Idempotency

For payment creation, you can pass an `Idempotency-Key` header to prevent duplicate payments:

```http
POST /payments/create-intent
Idempotency-Key: unique-key-per-request
```

If the same key is used within 24 hours, the original response will be returned.
