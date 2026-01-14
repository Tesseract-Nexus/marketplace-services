# Payment Service - Production Integration Guide

This guide covers the complete process for integrating payment gateways in production.

## Pre-Production Checklist

Before going live, ensure:

- [ ] All DevTest testing completed successfully
- [ ] Business verification completed with payment gateway
- [ ] KYC documents submitted and approved
- [ ] Bank account linked for settlements
- [ ] Legal agreements signed
- [ ] SSL certificate configured
- [ ] Webhook endpoints accessible from internet
- [ ] Error monitoring configured (Sentry, etc.)
- [ ] Backup payment gateway configured (optional)

---

## Gateway Production Setup

### Razorpay (India) - Production

**Business Requirements:**
- Indian registered business (Pvt Ltd, LLP, Proprietorship)
- GST registration (recommended)
- PAN card
- Bank account in business name

**Steps:**

1. **Complete KYC Verification**
   - Login to dashboard.razorpay.com
   - Settings → Profile → Business Details
   - Upload:
     - PAN Card
     - GST Certificate
     - Cancelled cheque / Bank statement
     - Address proof
   - Wait for verification (1-3 business days)

2. **Get Live API Keys**
   - Settings → API Keys → Generate Key (Live Mode)
   - Copy:
     - Key ID: `rzp_live_XXXXXXXXXXXX`
     - Key Secret: `XXXXXXXXXXXXXXXXXXXXXXXX`

3. **Configure Live Webhook**
   - Settings → Webhooks → Add New Webhook
   - URL: `https://admin.yourdomain.com/webhooks/razorpay?tenant_id=YOUR_TENANT_ID`
   - Generate and save webhook secret
   - Enable events:
     - `payment.authorized`
     - `payment.captured`
     - `payment.failed`
     - `refund.created`
     - `refund.processed`
     - `refund.failed`
     - `order.paid`

4. **Configure Settlement Account**
   - Settings → Banking → Add Bank Account
   - Settlement cycle: T+2 (default) or T+1 (upon request)

5. **Enable Auto-Capture (Recommended)**
   - Settings → Auto Capture
   - Enable for immediate payment capture

**Fees (Live):**
- Cards: 2% per transaction
- UPI: 2% per transaction
- Net Banking: Flat Rs. 3-10 per transaction
- Settlement: Free

---

### Stripe (International) - Production

**Business Requirements:**
- Business registered in Stripe-supported country
- EIN/Tax ID
- Bank account
- Business website with privacy policy, terms of service

**Steps:**

1. **Complete Account Activation**
   - Dashboard → Get started
   - Fill business details
   - Add bank account for payouts
   - Verify identity (government ID)

2. **Get Live API Keys**
   - Developers → API Keys
   - Click "Reveal live key"
   - Copy both Publishable and Secret keys

3. **Configure Webhook**
   - Developers → Webhooks → Add endpoint
   - URL: `https://admin.yourdomain.com/webhooks/stripe`
   - Events:
     - `payment_intent.succeeded`
     - `payment_intent.payment_failed`
     - `charge.refunded`
     - `charge.dispute.created`
   - Copy Signing Secret

4. **Enable Radar for Fraud Protection**
   - Radar → Rules
   - Enable default rules or customize

5. **Configure Payout Schedule**
   - Settings → Payouts
   - Choose: Daily, Weekly, Monthly

**Fees (Standard):**
- Cards: 2.9% + $0.30 per transaction
- International cards: +1%
- ACH: 0.8% (max $5)

---

### PayPal (International) - Production

**Business Requirements:**
- Business PayPal account
- Business verification completed
- Linked bank account

**Steps:**

1. **Upgrade to Business Account**
   - PayPal.com → Upgrade to Business
   - Complete business verification

2. **Get Live API Credentials**
   - developer.paypal.com
   - My Apps → Create App (Live)
   - Copy Client ID and Secret

3. **Configure Webhook**
   - App Settings → Webhooks
   - Add webhook URL
   - Select events

4. **Enable Express Checkout**
   - Account Settings → Website Payments
   - Configure button appearance

---

## Production Configuration

### 1. Create Kubernetes Secrets

```bash
# Create sealed secret for gateway credentials
kubectl create secret generic payment-gateway-secrets \
  --from-literal=RAZORPAY_KEY_ID=rzp_live_XXX \
  --from-literal=RAZORPAY_KEY_SECRET=secret \
  --from-literal=RAZORPAY_WEBHOOK_SECRET=webhook_secret \
  --from-literal=STRIPE_SECRET_KEY=sk_live_XXX \
  --from-literal=STRIPE_WEBHOOK_SECRET=whsec_XXX \
  --dry-run=client -o yaml | kubeseal > payment-secrets-sealed.yaml

kubectl apply -f payment-secrets-sealed.yaml
```

### 2. Update Helm Values

```yaml
# values-production.yaml
gatewaySecrets:
  enabled: true
  existingSecret: "payment-gateway-secrets"

env:
  GIN_MODE: "release"
  PLATFORM_FEE_PERCENT: "0.05"
  PLATFORM_FEE_ENABLED: "true"

# Disable test mode
postgresql:
  enabled: true
  existingSecret: "postgresql-password"
```

### 3. Configure Gateway in Admin Portal

1. Login to Admin Portal (production)
2. Settings → Payments → Payment Gateways
3. Add production gateway:
   - **Disable** Test Mode
   - Enter live credentials
   - Set priority for multiple gateways

---

## Security Checklist for Production

### API Security
- [ ] HTTPS enforced on all endpoints
- [ ] Rate limiting enabled
- [ ] CORS configured for production domains only
- [ ] API secrets stored in sealed secrets
- [ ] Webhook signature verification enabled

### PCI Compliance
- [ ] Card data never stored (use gateway tokens)
- [ ] All logs sanitized (no card numbers)
- [ ] API secrets encrypted at rest
- [ ] Regular security audits scheduled

### Monitoring
- [ ] Payment success rate alerts (<95% triggers alert)
- [ ] Failed payment notifications
- [ ] Webhook failure alerts
- [ ] High refund rate alerts

---

## Production Webhook URLs

Configure these in each gateway's dashboard:

| Gateway | Webhook URL |
|---------|-------------|
| Razorpay | `https://admin.yourdomain.com/webhooks/razorpay?tenant_id={tenant}` |
| Stripe | `https://admin.yourdomain.com/webhooks/stripe` |
| PayPal | `https://admin.yourdomain.com/webhooks/paypal` |
| PayU | `https://admin.yourdomain.com/webhooks/payu` |
| Cashfree | `https://admin.yourdomain.com/webhooks/cashfree` |

---

## Multi-Gateway Strategy

### Recommended Setup for India + International

```
Primary (India):
├── Razorpay (Priority 1)
│   ├── UPI
│   ├── Cards (Indian)
│   ├── Net Banking
│   └── Wallets

Backup (India):
├── Cashfree (Priority 2)
│   └── All payment methods

International:
├── Stripe (Priority 1)
│   ├── Cards
│   ├── Apple Pay
│   └── Google Pay
├── PayPal (Priority 2)
    └── PayPal payments
```

### Failover Configuration

In Admin Portal → Settings → Payments:
1. Set priorities for each gateway
2. Enable "Automatic Failover"
3. Configure fallback gateway

---

## Settlement & Reconciliation

### Daily Reconciliation Process

1. **Download Settlement Reports**
   - Razorpay: Dashboard → Reports → Settlements
   - Stripe: Dashboard → Payments → Export

2. **Match with Database**
   ```sql
   SELECT
     pt.gateway_transaction_id,
     pt.amount,
     pt.status,
     pt.processed_at
   FROM payment_transactions pt
   WHERE pt.processed_at >= CURRENT_DATE - INTERVAL '1 day'
     AND pt.status = 'succeeded';
   ```

3. **Verify Platform Fees**
   ```sql
   SELECT
     SUM(platform_fee) as total_fees,
     COUNT(*) as transactions
   FROM payment_transactions
   WHERE processed_at >= CURRENT_DATE - INTERVAL '1 day'
     AND status = 'succeeded';
   ```

### Monthly Reporting

- Total transactions by gateway
- Success/failure rates
- Refund rates
- Platform fee collection
- Dispute rates

---

## Disaster Recovery

### Gateway Outage Response

1. **Detection**
   - Monitor gateway health endpoints
   - Alert on increased failure rates

2. **Failover**
   - Automatic: System switches to backup gateway
   - Manual: Disable failed gateway in Admin Portal

3. **Communication**
   - Notify customers if checkout is affected
   - Update status page

### Data Backup

- Database: Daily automated backups
- Transaction logs: Retained for 7 years (compliance)
- Webhook events: Retained for 90 days

---

## Compliance Requirements

### For India (Razorpay, PayU, etc.)

- RBI Payment Aggregator guidelines
- Data localization (card data must be stored in India)
- Two-factor authentication for cards
- Recurring payment mandate (if applicable)

### For International (Stripe, PayPal)

- PCI DSS compliance
- GDPR (for EU customers)
- Strong Customer Authentication (SCA) for EU

---

## Support Contacts

### Gateway Support

| Gateway | Support Email | Phone |
|---------|---------------|-------|
| Razorpay | support@razorpay.com | +91-76309-81080 |
| Stripe | support@stripe.com | - |
| PayPal | Business Support Portal | - |

### Internal Escalation

1. L1: Payment team - payment@yourcompany.com
2. L2: Engineering - engineering@yourcompany.com
3. L3: Finance - finance@yourcompany.com

---

## Go-Live Checklist

Before enabling live payments:

- [ ] Live credentials configured in production
- [ ] Webhooks verified working
- [ ] Test transaction successful (use small amount)
- [ ] Refund tested
- [ ] Settlement account verified
- [ ] Monitoring alerts configured
- [ ] Support team briefed
- [ ] Rollback plan documented
- [ ] Customer communication ready
