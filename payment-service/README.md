

# Payment Service

Complete payment gateway integration and management service for ecommerce platform.

## Overview

The Payment Service handles:
- **Multi-Gateway Support**: Razorpay, PayU, Cashfree, Paytm (India) + Stripe, PayPal (International)
- **Payment Processing**: Accept payments from various methods
- **Refund Management**: Full and partial refunds
- **Webhook Handling**: Real-time payment status updates
- **Saved Payment Methods**: Store customer payment info securely
- **Dispute Management**: Handle chargebacks and disputes
- **PCI Compliance**: Secure payment data handling

## Supported Payment Gateways

### 🇮🇳 India-Specific Gateways (Primary)

#### 1. **Razorpay** (Recommended for India)
- ✅ UPI (PhonePe, Google Pay, Paytm, BHIM)
- ✅ Credit/Debit Cards (Visa, Mastercard, RuPay, Amex)
- ✅ Net Banking (50+ banks)
- ✅ Wallets (Paytm, PhonePe, Amazon Pay, Mobikwik)
- ✅ EMI Options
- ✅ Subscriptions/Recurring Payments
- ✅ International Cards
- **Fees**: 2% per transaction
- **Settlement**: T+3 days

#### 2. **PayU India**
- ✅ Cards, Net Banking, UPI, Wallets
- ✅ EMI (Bajaj Finserv, ZestMoney)
- ✅ Pay Later (LazyPay, Simpl)
- **Fees**: 2-3% per transaction

#### 3. **Cashfree**
- ✅ UPI, Cards, Net Banking
- ✅ Paylater (Simpl, LazyPay)
- ✅ Auto Collect
- ✅ Vendor Payouts
- **Fees**: 1.99% per transaction

#### 4. **Paytm**
- ✅ Paytm Wallet
- ✅ UPI, Cards, Net Banking
- **Fees**: 2% per transaction

### 🌍 International Gateways (Non-India Only)

#### 5. **Stripe** (US, Europe, 40+ countries - NOT India ❌)
- ✅ Credit/Debit Cards
- ✅ Apple Pay, Google Pay
- ✅ Bank Transfers (ACH, SEPA)
- ✅ Buy Now Pay Later (Klarna, Affirm)
- **Fees**: 2.9% + $0.30 per transaction
- **⚠️ IMPORTANT**: Stripe does NOT support India. Use Razorpay/PayU/Cashfree for Indian customers.

#### 6. **PayPal**
- ✅ PayPal Balance
- ✅ Linked Cards
- ✅ PayPal Credit
- **Fees**: 2.9% + fixed fee

## Features

### ✅ Core Features
- [x] Multi-gateway configuration per tenant
- [x] Payment intent creation and processing
- [x] Refund processing (full and partial)
- [x] Webhook event handling
- [x] Saved payment methods
- [x] 3D Secure authentication
- [x] Fraud detection integration
- [x] Multi-currency support

### ⚠️ Advanced Features
- [ ] Recurring payments/subscriptions
- [ ] Split payments
- [ ] Payment links
- [ ] Dynamic currency conversion
- [ ] Checkout.com integration
- [ ] Adyen integration

## Database Schema

### Payment Gateway Configs
Multi-tenant gateway configurations with credentials and settings.

### Payment Transactions
All payment attempts with status tracking.

### Refund Transactions
Refund records linked to original payments.

### Webhook Events
Incoming webhook events for audit and debugging.

### Saved Payment Methods
Customer payment methods for future use.

### Payment Disputes
Chargebacks and dispute management.

### Payment Settings
Global payment settings per tenant.

## API Endpoints

### Payment Processing
```
POST   /api/v1/payments/create-intent      Create payment intent
POST   /api/v1/payments/confirm             Confirm payment
GET    /api/v1/payments/:id                 Get payment status
POST   /api/v1/payments/:id/cancel          Cancel payment
POST   /api/v1/payments/:id/capture         Capture authorized payment
```

### Refunds
```
POST   /api/v1/payments/:id/refund          Create refund
GET    /api/v1/refunds/:id                  Get refund status
GET    /api/v1/refunds                      List refunds
```

### Payment Methods
```
GET    /api/v1/payment-methods              List saved payment methods
POST   /api/v1/payment-methods              Save payment method
DELETE /api/v1/payment-methods/:id          Delete payment method
PUT    /api/v1/payment-methods/:id/default  Set as default
```

### Gateway Configuration
```
GET    /api/v1/payment-gateways             List configured gateways
POST   /api/v1/payment-gateways             Add gateway
PUT    /api/v1/payment-gateways/:id         Update gateway
DELETE /api/v1/payment-gateways/:id         Remove gateway
POST   /api/v1/payment-gateways/:id/test    Test gateway connection
```

### Webhooks
```
POST   /webhooks/razorpay                   Razorpay webhook
POST   /webhooks/stripe                     Stripe webhook
POST   /webhooks/paypal                     PayPal webhook
POST   /webhooks/payu                       PayU webhook
POST   /webhooks/cashfree                   Cashfree webhook
```

## Usage Examples

### Create Payment with Razorpay (India)
```bash
curl -X POST http://localhost:8092/api/v1/payments/create-intent \
  -H "Content-Type: application/json" \
  -d '{
    "tenantId": "test-tenant",
    "orderId": "order-123",
    "amount": 15000.00,
    "currency": "INR",
    "customerId": "cust-456",
    "gatewayType": "RAZORPAY",
    "paymentMethod": "upi",
    "customerEmail": "customer@example.com",
    "customerPhone": "+919876543210"
  }'
```

Response:
```json
{
  "paymentIntentId": "pay_KsGT4HlL6VZwRB",
  "amount": 15000.00,
  "currency": "INR",
  "status": "created",
  "clientSecret": "pi_secret_XXXXXXXXXXXX",
  "razorpayOrderId": "order_KsGT4HlL6VZwRB",
  "options": {
    "key": "rzp_test_XXXXXXXX",
    "order_id": "order_KsGT4HlL6VZwRB",
    "prefill": {
      "email": "customer@example.com",
      "contact": "+919876543210"
    },
    "theme": {
      "color": "#3399cc"
    }
  }
}
```

### Process Refund
```bash
curl -X POST http://localhost:8092/api/v1/payments/pay-123/refund \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 5000.00,
    "reason": "REQUESTED_BY_CUSTOMER",
    "notes": "Customer requested partial refund"
  }'
```

## Integration

### Razorpay Integration (Recommended for India)

1. **Sign up**: https://razorpay.com
2. **Get API Keys**: Dashboard → Settings → API Keys
3. **Configure in Admin**:
   - Public Key: `rzp_test_XXXXXXXXXXXXXXXX`
   - Secret Key: `ENCRYPTED:rzp_test_secret_XXXXXXXXXXXX`
   - Enable webhook: `https://yoursite.com/webhooks/razorpay`

4. **Test Mode**:
   - Test UPI: Use VPA `success@razorpay`
   - Test Cards: `4111 1111 1111 1111`, CVV: `123`, Exp: any future date

5. **Go Live**:
   - Complete KYC verification
   - Switch to live API keys
   - Update webhook URL

### Payment Flow

```
1. Customer reaches checkout
2. Frontend calls /api/v1/payments/create-intent
3. Backend creates Razorpay order
4. Frontend displays Razorpay checkout modal
5. Customer completes payment
6. Razorpay sends webhook → /webhooks/razorpay
7. Backend updates order status
8. Customer sees success page
```

## Configuration

### Environment Variables
```bash
PORT=8092
DATABASE_URL=postgresql://user:pass@localhost:5432/tesseract_hub
ENVIRONMENT=development

# Razorpay (India)
RAZORPAY_KEY_ID=rzp_test_XXXXXXXXXXXXXXXX
RAZORPAY_KEY_SECRET=ENCRYPTED:secret
RAZORPAY_WEBHOOK_SECRET=ENCRYPTED:webhook_secret

# PayU India
PAYU_MERCHANT_KEY=XXXXXXXX
PAYU_MERCHANT_SALT=ENCRYPTED:salt

# Cashfree
CASHFREE_APP_ID=app_id
CASHFREE_SECRET_KEY=ENCRYPTED:secret

# Stripe (International)
STRIPE_PUBLIC_KEY=pk_test_XXXX
STRIPE_SECRET_KEY=ENCRYPTED:sk_test_XXXX
STRIPE_WEBHOOK_SECRET=ENCRYPTED:whsec_XXXX

# PayPal
PAYPAL_CLIENT_ID=client_id
PAYPAL_CLIENT_SECRET=ENCRYPTED:secret
```

## Security

### PCI Compliance
- ✅ Never store raw card numbers
- ✅ Use gateway tokens for saved cards
- ✅ Encrypt all API secrets
- ✅ HTTPS only
- ✅ Webhook signature verification

### Secrets Management
All sensitive data (API secrets, webhook secrets) should be:
1. Encrypted at rest
2. Never logged
3. Rotated periodically
4. Stored in secure vault (AWS Secrets Manager, HashiCorp Vault)

## Testing

### Test Cards (Razorpay)
```
Success: 4111 1111 1111 1111
3D Secure: 5104 0600 0000 0008
Failure: 4000 0000 0000 0002
```

### Test UPI (Razorpay)
```
Success: success@razorpay
Failure: failure@razorpay
```

### Webhook Testing
```bash
# Use Razorpay's test webhook generator
curl -X POST https://api.razorpay.com/v1/payments/test/webhook \
  -u rzp_test_XXX:secret
```

## Fees Comparison

| Gateway | Card | UPI | Net Banking | Wallets |
|---------|------|-----|-------------|---------|
| Razorpay | 2% | 2% | ₹3-10 flat | 2% |
| PayU | 2-3% | 2% | ₹3-15 flat | 2-3% |
| Cashfree | 1.99% | 1.99% | Flat ₹5 | 1.99% |
| Paytm | 2% | 2% | ₹5-15 flat | 1.99% |

## Error Handling

### Common Errors
- `INSUFFICIENT_FUNDS` - Card/account has insufficient balance
- `CARD_DECLINED` - Bank declined the transaction
- `INVALID_CVV` - Incorrect CVV
- `EXPIRED_CARD` - Card has expired
- `NETWORK_ERROR` - Gateway communication failed

### Retry Logic
- Failed payments: Automatic retry not recommended
- Webhook failures: Retry with exponential backoff (3 attempts)
- Refund failures: Manual retry after investigation

## Monitoring

### Metrics to Track
- Payment success rate (target: >95%)
- Average processing time (target: <3s)
- Webhook processing time
- Failed payment reasons
- Refund rate
- Dispute rate

## Support

### Razorpay
- Docs: https://razorpay.com/docs
- Support: support@razorpay.com
- Phone: +91-76309-81080

### Testing Credentials
All test credentials are available in the admin dashboard under Settings → Payment Gateways.

## License

Proprietary - Tesseract Hub Platform
