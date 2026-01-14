# Payment Service - DevTest Integration Guide

This guide walks you through setting up and testing payment integrations in the DevTest environment.

## Environment Details

| Component | URL |
|-----------|-----|
| Admin Portal | https://dev-admin.tesserix.app |
| Storefront | https://dev-store.tesserix.app |
| Payment API | https://dev-admin.tesserix.app/api/v1/payments |
| Webhooks | https://dev-admin.tesserix.app/webhooks/{gateway} |

---

## Step 1: Create Test Gateway Accounts

### Razorpay (Recommended for India)

**What you need to do:**

1. **Sign up for Razorpay Test Account**
   - Go to: https://dashboard.razorpay.com/signup
   - Select "I want to start with Test Mode"
   - Complete email verification

2. **Get Test API Keys**
   - Dashboard → Settings → API Keys → Generate Key
   - Copy:
     - Key ID: `rzp_test_XXXXXXXXXXXX`
     - Key Secret: `XXXXXXXXXXXXXXXXXXXXXXXX`

3. **Configure Webhook**
   - Dashboard → Settings → Webhooks → Add New Webhook
   - URL: `https://dev-admin.tesserix.app/webhooks/razorpay?tenant_id=YOUR_TENANT_ID`
   - Secret: Generate and save
   - Events to enable:
     - `payment.authorized`
     - `payment.captured`
     - `payment.failed`
     - `refund.created`
     - `refund.processed`
     - `refund.failed`

4. **Test Credentials to Use:**
   ```
   Test Card: 4111 1111 1111 1111
   CVV: Any 3 digits
   Expiry: Any future date
   OTP: 1234 (for 3D Secure)

   Test UPI: success@razorpay
   Failure UPI: failure@razorpay
   ```

---

### Stripe (International Markets)

**What you need to do:**

1. **Sign up for Stripe Test Account**
   - Go to: https://dashboard.stripe.com/register
   - Complete verification

2. **Get Test API Keys**
   - Dashboard → Developers → API Keys
   - Copy:
     - Publishable Key: `pk_test_XXXXXXXXXXXX`
     - Secret Key: `sk_test_XXXXXXXXXXXX`

3. **Configure Webhook**
   - Dashboard → Developers → Webhooks → Add Endpoint
   - URL: `https://dev-admin.tesserix.app/webhooks/stripe`
   - Events to enable:
     - `payment_intent.succeeded`
     - `payment_intent.payment_failed`
     - `charge.refunded`
   - Copy Webhook Signing Secret: `whsec_XXXXXXXXXXXX`

4. **Test Credentials:**
   ```
   Success Card: 4242 4242 4242 4242
   Decline Card: 4000 0000 0000 0002
   3D Secure: 4000 0027 6000 3184

   CVV: Any 3 digits
   Expiry: Any future date
   ZIP: Any 5 digits
   ```

---

### PayPal (International)

**What you need to do:**

1. **Create PayPal Developer Account**
   - Go to: https://developer.paypal.com
   - Sign in with PayPal account

2. **Create Sandbox App**
   - Dashboard → My Apps & Credentials → Create App
   - App Type: Merchant
   - Copy:
     - Client ID
     - Secret

3. **Configure Webhook**
   - Dashboard → My Apps → Your App → Webhooks
   - URL: `https://dev-admin.tesserix.app/webhooks/paypal`
   - Events:
     - `PAYMENT.CAPTURE.COMPLETED`
     - `PAYMENT.CAPTURE.DENIED`

4. **Sandbox Test Accounts**
   - Dashboard → Sandbox → Accounts
   - Create buyer/seller test accounts

---

## Step 2: Configure Gateway in Admin Portal

1. **Login to Admin Portal**
   - URL: https://dev-admin.tesserix.app
   - Use your tenant credentials

2. **Navigate to Payment Settings**
   - Settings → Payments → Payment Gateways

3. **Add New Gateway**
   - Click "Add Gateway"
   - Select gateway type (e.g., RAZORPAY)
   - Fill in credentials from Step 1
   - Enable "Test Mode"
   - Save

4. **Verify Configuration**
   - Click "Test Connection" button
   - Should show "Connection successful"

---

## Step 3: Test Payment Flow

### Using Admin Portal

1. **Create Test Order**
   - Orders → Create New Order
   - Add test products
   - Set customer info

2. **Process Payment**
   - Select payment method
   - Complete checkout

### Using API Directly

```bash
# 1. Create Payment Intent
curl -X POST https://dev-admin.tesserix.app/api/v1/payments/create-intent \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-store" \
  -d '{
    "tenantId": "demo-store",
    "orderId": "test-order-001",
    "amount": 100.00,
    "currency": "INR",
    "gatewayType": "RAZORPAY",
    "customerEmail": "test@example.com",
    "customerPhone": "+919876543210"
  }'

# Response will include razorpayOrderId and options for checkout

# 2. After customer completes payment, confirm it
curl -X POST https://dev-admin.tesserix.app/api/v1/payments/confirm \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-store" \
  -d '{
    "paymentIntentId": "pay_xxx",
    "gatewayTransactionId": "pay_xxx",
    "signature": "razorpay_signature_from_frontend"
  }'
```

### Using Storefront

1. **Go to Storefront**
   - https://demo-store.tesserix.app (or your tenant storefront)

2. **Add Products to Cart**

3. **Checkout**
   - Enter shipping info
   - Select payment method
   - Use test card numbers from Step 1

---

## Step 4: Test Webhooks

### Verify Webhook Delivery

1. **Check Razorpay Dashboard**
   - Dashboard → Settings → Webhooks → View logs
   - Should see successful deliveries

2. **Check Payment Service Logs**
   ```bash
   kubectl logs deployment/payment-service -n devtest --tail=100 | grep webhook
   ```

### Simulate Webhooks (Razorpay)

```bash
# Use Razorpay's test webhook generator
curl -X POST "https://api.razorpay.com/v1/webhooks/test" \
  -u "rzp_test_XXX:secret" \
  -H "Content-Type: application/json" \
  -d '{
    "event": "payment.captured",
    "payment_id": "pay_test123"
  }'
```

---

## Step 5: Test Refunds

```bash
# Create a refund for a successful payment
curl -X POST https://dev-admin.tesserix.app/api/v1/payments/{payment-id}/refund \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-store" \
  -d '{
    "amount": 50.00,
    "reason": "Customer requested refund"
  }'
```

---

## Test Checklist

### Gateway Configuration
- [ ] Razorpay test credentials obtained
- [ ] Gateway added in Admin Portal
- [ ] Connection test passed
- [ ] Webhook URL configured in gateway dashboard

### Payment Flow
- [ ] Payment intent creation works
- [ ] Razorpay checkout modal opens
- [ ] Test card payment succeeds
- [ ] Payment confirmation works
- [ ] Order status updated

### Webhooks
- [ ] Webhook receives payment events
- [ ] Payment status updates automatically
- [ ] Webhook signature verification works

### Refunds
- [ ] Full refund works
- [ ] Partial refund works
- [ ] Refund status updates via webhook

---

## Troubleshooting

### "Gateway not configured for tenant"

**Cause:** No gateway config exists for your tenant.

**Fix:** Add gateway in Admin Portal → Settings → Payments → Add Gateway

### "Webhook signature verification failed"

**Cause:** Webhook secret mismatch.

**Fix:**
1. Verify webhook secret in gateway dashboard matches the one in your config
2. Check if tenant_id is passed in webhook URL

### "Payment creation failed"

**Cause:** Invalid credentials or gateway issue.

**Fix:**
1. Run "Test Connection" in Admin Portal
2. Check API keys are correct
3. Ensure test mode is enabled

### Payments stuck in "pending"

**Cause:** Webhook not being received.

**Fix:**
1. Check webhook URL is accessible from internet
2. Verify webhook events are enabled in gateway dashboard
3. Check payment service logs for errors

---

## Gateway-Specific Test Data

### Razorpay Test Cards

| Card Number | Description |
|-------------|-------------|
| 4111 1111 1111 1111 | Success |
| 5104 0600 0000 0008 | 3D Secure |
| 4000 0000 0000 0002 | Decline |

### Razorpay Test UPI

| VPA | Description |
|-----|-------------|
| success@razorpay | Payment succeeds |
| failure@razorpay | Payment fails |

### Stripe Test Cards

| Card Number | Description |
|-------------|-------------|
| 4242 4242 4242 4242 | Success |
| 4000 0000 0000 0002 | Decline |
| 4000 0027 6000 3184 | 3D Secure required |

---

## Environment Variables for DevTest

These are already configured in the devtest cluster:

```yaml
env:
  PORT: "8080"
  GIN_MODE: "release"
  PLATFORM_FEE_PERCENT: "0.05"
  PLATFORM_FEE_ENABLED: "true"
```

Database connection uses the existing `postgresql-password` secret.

---

## Next Steps

Once testing is complete:
1. Document any issues encountered
2. Verify all test cases pass
3. Proceed to Production Integration Guide
