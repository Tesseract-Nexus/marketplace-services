# Marketing Service API Documentation

## Overview

The Marketing Service is a comprehensive marketing automation platform for the Tesseract Hub e-commerce ecosystem. It provides campaign management, customer segmentation, loyalty programs, coupon management, and integration with Mautic for email marketing automation.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Marketing Service                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │  Campaigns  │  │  Segments   │  │   Loyalty Program       │  │
│  └──────┬──────┘  └──────┬──────┘  └───────────┬─────────────┘  │
│         │                │                     │                 │
│  ┌──────┴────────────────┴─────────────────────┴──────────────┐ │
│  │                    Marketing Service                        │ │
│  │              (Business Logic Layer)                         │ │
│  └──────┬────────────────┬─────────────────────┬──────────────┘ │
│         │                │                     │                 │
│  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────────┴────────────┐   │
│  │  PostgreSQL │  │   Mautic    │  │  GCP Secret Manager   │   │
│  │  (Database) │  │   (Email)   │  │    (Credentials)      │   │
│  └─────────────┘  └─────────────┘  └───────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Base URL

```
Production: https://api.tesserix.app/marketing
Internal:   http://marketing-service.marketplace.svc.cluster.local:8080
```

## Authentication

### Headers Required

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes* | Bearer JWT token for authenticated endpoints |
| `X-Tenant-ID` | Yes | Tenant/Vendor identifier for multi-tenancy |
| `X-Vendor-ID` | Alternative | Alternative to X-Tenant-ID |

*Public storefront endpoints don't require Authorization header.

### RBAC Permissions

The service uses Role-Based Access Control (RBAC) with the following permissions:

| Permission | Description |
|------------|-------------|
| `marketing:campaigns:view` | View campaigns and statistics |
| `marketing:campaigns:manage` | Create, update, delete campaigns |
| `marketing:segments:view` | View customer segments |
| `marketing:segments:manage` | Create, update, delete segments |
| `marketing:loyalty:view` | View loyalty program and customer data |
| `marketing:loyalty:manage` | Manage loyalty program settings |
| `marketing:loyalty:points:adjust` | Adjust customer loyalty points |
| `marketing:coupons:view` | View coupons |
| `marketing:coupons:manage` | Create, update, delete coupons |
| `marketing:carts:view` | View abandoned carts |
| `marketing:carts:recover` | Create cart recovery campaigns |
| `marketing:email:send` | Send marketing emails |

---

## API Endpoints

### Health & Status

#### Health Check
```http
GET /health
```

**Response:**
```json
{
  "service": "marketing-service",
  "status": "healthy",
  "version": "1.0.0"
}
```

#### Readiness Check
```http
GET /ready
```

---

### Campaigns

#### List Campaigns
```http
GET /api/v1/campaigns
```

**Permission:** `marketing:campaigns:view`

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `page` | int | Page number (default: 1) |
| `limit` | int | Items per page (default: 20) |
| `status` | string | Filter by status (DRAFT, SCHEDULED, ACTIVE, COMPLETED, PAUSED) |
| `type` | string | Filter by type (EMAIL, SMS, PUSH, IN_APP) |

**Response:**
```json
{
  "success": true,
  "data": {
    "campaigns": [
      {
        "id": "uuid",
        "tenantId": "tenant-001",
        "name": "Summer Sale Campaign",
        "description": "Promotional campaign for summer products",
        "type": "EMAIL",
        "status": "DRAFT",
        "subject": "Summer Sale - Up to 50% Off!",
        "content": "<html>...</html>",
        "segmentId": "uuid",
        "scheduledAt": "2024-07-01T10:00:00Z",
        "sentAt": null,
        "stats": {
          "sent": 0,
          "delivered": 0,
          "opened": 0,
          "clicked": 0,
          "converted": 0,
          "unsubscribed": 0
        },
        "createdAt": "2024-06-15T08:00:00Z",
        "updatedAt": "2024-06-15T08:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 45,
      "totalPages": 3
    }
  }
}
```

#### Create Campaign
```http
POST /api/v1/campaigns
```

**Permission:** `marketing:campaigns:manage`

**Request Body:**
```json
{
  "name": "Summer Sale Campaign",
  "description": "Promotional campaign for summer products",
  "type": "EMAIL",
  "subject": "Summer Sale - Up to 50% Off!",
  "content": "<html><body><h1>Summer Sale!</h1><p>Get up to 50% off on selected items.</p></body></html>",
  "segmentId": "uuid-of-target-segment"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "generated-uuid",
    "tenantId": "tenant-001",
    "name": "Summer Sale Campaign",
    "status": "DRAFT",
    "createdAt": "2024-06-15T08:00:00Z"
  }
}
```

#### Get Campaign
```http
GET /api/v1/campaigns/:id
```

**Permission:** `marketing:campaigns:view`

#### Update Campaign
```http
PUT /api/v1/campaigns/:id
```

**Permission:** `marketing:campaigns:manage`

#### Delete Campaign
```http
DELETE /api/v1/campaigns/:id
```

**Permission:** `marketing:campaigns:manage`

#### Send Campaign
```http
POST /api/v1/campaigns/:id/send
```

**Permission:** `marketing:email:send`

**Description:** Immediately sends the campaign to all recipients in the target segment.

#### Schedule Campaign
```http
POST /api/v1/campaigns/:id/schedule
```

**Permission:** `marketing:campaigns:manage`

**Request Body:**
```json
{
  "scheduledAt": "2024-07-01T10:00:00Z"
}
```

#### Get Campaign Statistics
```http
GET /api/v1/campaigns/stats
```

**Permission:** `marketing:campaigns:view`

**Response:**
```json
{
  "success": true,
  "data": {
    "totalCampaigns": 45,
    "activeCampaigns": 3,
    "totalSent": 125000,
    "totalDelivered": 122500,
    "totalOpened": 45000,
    "totalClicked": 12000,
    "averageOpenRate": 36.73,
    "averageClickRate": 9.79
  }
}
```

---

### Customer Segments

#### List Segments
```http
GET /api/v1/segments
```

**Permission:** `marketing:segments:view`

**Response:**
```json
{
  "success": true,
  "data": {
    "segments": [
      {
        "id": "uuid",
        "tenantId": "tenant-001",
        "name": "VIP Customers",
        "description": "Customers with lifetime value > $1000",
        "type": "DYNAMIC",
        "criteria": {
          "rules": [
            {
              "field": "lifetime_value",
              "operator": "greater_than",
              "value": 1000
            }
          ],
          "logic": "AND"
        },
        "customerCount": 1250,
        "mauticSegmentId": 15,
        "createdAt": "2024-01-15T08:00:00Z",
        "updatedAt": "2024-06-01T12:00:00Z"
      }
    ]
  }
}
```

#### Create Segment
```http
POST /api/v1/segments
```

**Permission:** `marketing:segments:manage`

**Request Body:**
```json
{
  "name": "High-Value Customers",
  "description": "Customers who spent more than $500",
  "type": "DYNAMIC",
  "criteria": {
    "rules": [
      {
        "field": "total_spent",
        "operator": "greater_than",
        "value": 500
      },
      {
        "field": "order_count",
        "operator": "greater_than",
        "value": 3
      }
    ],
    "logic": "AND"
  }
}
```

#### Get Segment
```http
GET /api/v1/segments/:id
```

#### Update Segment
```http
PUT /api/v1/segments/:id
```

#### Delete Segment
```http
DELETE /api/v1/segments/:id
```

---

### Loyalty Program

#### Get Loyalty Program
```http
GET /api/v1/loyalty/program
```

**Permission:** `marketing:loyalty:view`

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "tenantId": "tenant-001",
    "name": "Tesseract Rewards",
    "description": "Earn points on every purchase",
    "pointsPerDollar": 10,
    "minimumRedemption": 100,
    "pointValue": 0.01,
    "tiers": [
      {
        "name": "Bronze",
        "minPoints": 0,
        "multiplier": 1.0,
        "benefits": ["Free shipping on orders over $50"]
      },
      {
        "name": "Silver",
        "minPoints": 1000,
        "multiplier": 1.5,
        "benefits": ["Free shipping", "Early access to sales"]
      },
      {
        "name": "Gold",
        "minPoints": 5000,
        "multiplier": 2.0,
        "benefits": ["Free shipping", "Early access", "Exclusive discounts"]
      }
    ],
    "referralBonus": 500,
    "isActive": true
  }
}
```

#### Create/Update Loyalty Program
```http
POST /api/v1/loyalty/program
PUT /api/v1/loyalty/program
```

**Permission:** `marketing:loyalty:manage`

#### Get Customer Loyalty
```http
GET /api/v1/loyalty/customers/:customer_id
```

**Permission:** `marketing:loyalty:view`

**Response:**
```json
{
  "success": true,
  "data": {
    "customerId": "customer-uuid",
    "currentPoints": 2500,
    "lifetimePoints": 5000,
    "redeemedPoints": 2500,
    "currentTier": "Silver",
    "nextTier": "Gold",
    "pointsToNextTier": 2500,
    "referralCode": "CUST123ABC",
    "referralCount": 3,
    "enrolledAt": "2024-01-15T08:00:00Z"
  }
}
```

#### Enroll Customer
```http
POST /api/v1/loyalty/customers/:customer_id/enroll
```

**Permission:** `marketing:loyalty:manage`

#### Redeem Points
```http
POST /api/v1/loyalty/customers/:customer_id/redeem
```

**Permission:** `marketing:loyalty:points:adjust`

**Request Body:**
```json
{
  "points": 500,
  "reason": "Discount redemption",
  "orderId": "order-uuid"
}
```

#### Get Loyalty Transactions
```http
GET /api/v1/loyalty/customers/:customer_id/transactions
```

**Permission:** `marketing:loyalty:view`

#### Get Customer Referrals
```http
GET /api/v1/loyalty/customers/:customer_id/referrals
```

---

### Coupons

#### List Coupons
```http
GET /api/v1/coupons
```

**Permission:** `marketing:coupons:view`

**Response:**
```json
{
  "success": true,
  "data": {
    "coupons": [
      {
        "id": "uuid",
        "tenantId": "tenant-001",
        "code": "SUMMER2024",
        "name": "Summer Sale Coupon",
        "description": "20% off summer collection",
        "type": "PERCENTAGE",
        "discountValue": 20,
        "maxDiscount": 100,
        "minOrderAmount": 50,
        "maxUsage": 1000,
        "usagePerCustomer": 1,
        "currentUsage": 456,
        "validFrom": "2024-06-01T00:00:00Z",
        "validUntil": "2024-08-31T23:59:59Z",
        "isActive": true,
        "isPublic": true,
        "applicableProducts": null,
        "applicableCategories": ["summer-collection"],
        "excludedProducts": null
      }
    ]
  }
}
```

#### Create Coupon
```http
POST /api/v1/coupons
```

**Permission:** `marketing:coupons:manage`

**Request Body:**
```json
{
  "code": "WELCOME10",
  "name": "Welcome Discount",
  "description": "10% off for new customers",
  "type": "PERCENTAGE",
  "discountValue": 10,
  "maxDiscount": 50,
  "minOrderAmount": 25,
  "maxUsage": 0,
  "usagePerCustomer": 1,
  "validFrom": "2024-01-01T00:00:00Z",
  "validUntil": "2024-12-31T23:59:59Z",
  "isActive": true,
  "isPublic": false
}
```

#### Validate Coupon
```http
POST /api/v1/coupons/validate
```

**Permission:** `marketing:coupons:view`

**Request Body:**
```json
{
  "code": "SUMMER2024",
  "customerId": "customer-uuid",
  "orderTotal": 150.00,
  "products": ["product-uuid-1", "product-uuid-2"],
  "categories": ["summer-collection"]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "valid": true,
    "discountAmount": 30.00,
    "discountType": "PERCENTAGE",
    "message": "Coupon applied successfully"
  }
}
```

---

### Abandoned Carts

#### List Abandoned Carts
```http
GET /api/v1/abandoned-carts
```

**Permission:** `marketing:carts:view`

#### Create Abandoned Cart Record
```http
POST /api/v1/abandoned-carts
```

**Permission:** `marketing:carts:recover`

#### Get Abandoned Cart Statistics
```http
GET /api/v1/abandoned-carts/stats
```

**Permission:** `marketing:carts:view`

---

### Mautic Integration

#### Get Integration Status
```http
GET /api/v1/integrations/mautic/status
```

**Permission:** `marketing:campaigns:view`

**Response:**
```json
{
  "enabled": true,
  "connected": true,
  "url": "http://mautic.email.svc.cluster.local",
  "lastChecked": "2024-06-15T12:00:00Z"
}
```

#### Sync Campaign to Mautic
```http
POST /api/v1/integrations/mautic/sync/campaign
```

**Permission:** `marketing:campaigns:manage`

**Request Body:**
```json
{
  "campaignId": "campaign-uuid"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "mauticEmailId": 45,
    "mauticCampaignId": 12,
    "syncedAt": "2024-06-15T12:00:00Z"
  }
}
```

#### Sync Segment to Mautic
```http
POST /api/v1/integrations/mautic/sync/segment
```

**Permission:** `marketing:segments:manage`

**Request Body:**
```json
{
  "segmentId": "segment-uuid"
}
```

#### Create Contact in Mautic
```http
POST /api/v1/integrations/mautic/contacts
```

**Permission:** `marketing:campaigns:manage`

**Request Body:**
```json
{
  "email": "customer@example.com",
  "firstName": "John",
  "lastName": "Doe",
  "customFields": {
    "customer_id": "customer-uuid",
    "lifetime_value": 1500
  }
}
```

#### Add Contact to Segment
```http
POST /api/v1/integrations/mautic/segments/add-contact
```

**Permission:** `marketing:segments:manage`

**Request Body:**
```json
{
  "segmentId": "segment-uuid",
  "email": "customer@example.com"
}
```

#### Send Test Email
```http
POST /api/v1/integrations/mautic/test-email
```

**Permission:** `marketing:email:send`

**Request Body:**
```json
{
  "to": "test@example.com",
  "subject": "Test Email",
  "content": "<html><body><h1>Test</h1><p>This is a test email.</p></body></html>"
}
```

---

### Storefront (Public) Endpoints

These endpoints are for customer-facing applications and don't require JWT authentication, only tenant identification.

#### Get Loyalty Program Info
```http
GET /api/v1/storefront/loyalty/program
```

#### Get Customer Loyalty (via headers)
```http
GET /api/v1/storefront/loyalty/customer
```

**Headers:**
```
X-Tenant-ID: tenant-001
X-Customer-ID: customer-uuid
```

#### Enroll in Loyalty Program
```http
POST /api/v1/storefront/loyalty/enroll
```

#### Redeem Points
```http
POST /api/v1/storefront/loyalty/redeem
```

#### Get Transactions
```http
GET /api/v1/storefront/loyalty/transactions
```

#### Get Referrals
```http
GET /api/v1/storefront/loyalty/referrals
```

#### Validate Coupon (Public)
```http
POST /api/v1/storefront/coupons/validate
```

---

## Environment Configuration

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `PORT` | Service port | `8080` |
| `GIN_MODE` | Gin framework mode | `release` |
| `DB_HOST` | PostgreSQL host | `postgresql.postgresql-marketplace.svc.cluster.local` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | Database user | `postgres` |
| `DB_NAME` | Database name | `marketing_db` |
| `DB_SSLMODE` | SSL mode | `require` |

### GCP Secret Manager Integration

| Variable | Description |
|----------|-------------|
| `USE_GCP_SECRET_MANAGER` | Enable GCP secrets (`true`) |
| `GCP_PROJECT_ID` | GCP project ID |
| `GCP_SECRET_PREFIX` | Environment prefix (`devtest`, `pilot`, `prod`) |
| `DB_PASSWORD_SECRET_NAME` | Secret name for DB password |

### Mautic Integration

| Variable | Description |
|----------|-------------|
| `MAUTIC_ENABLED` | Enable Mautic (`true`/`false`) |
| `MAUTIC_URL` | Mautic service URL |
| `MAUTIC_USERNAME` | Mautic admin username |
| `MAUTIC_PASSWORD_SECRET_NAME` | GCP secret name for Mautic password |

### Email Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `FROM_EMAIL` | Sender email address | `noreply@mail.tesserix.app` |
| `FROM_NAME` | Sender display name | `Tesseract Hub` |

---

## Database Schema

### Tables

| Table | Description |
|-------|-------------|
| `campaigns` | Marketing campaigns |
| `customer_segments` | Customer segmentation rules |
| `campaign_recipients` | Campaign delivery tracking |
| `loyalty_programs` | Loyalty program configuration |
| `customer_loyalty` | Customer loyalty status |
| `loyalty_transactions` | Points transactions |
| `referrals` | Customer referral tracking |
| `coupon_codes` | Discount coupons |
| `coupon_usages` | Coupon usage tracking |
| `abandoned_carts` | Cart abandonment data |

---

## Error Responses

All errors follow a consistent format:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "field": "optional_field_name",
    "details": {}
  }
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `TENANT_REQUIRED` | 400 | Missing X-Tenant-ID header |
| `VALIDATION_ERROR` | 400 | Invalid request data |
| `UNAUTHORIZED` | 401 | Missing or invalid JWT token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource already exists |
| `INTERNAL_ERROR` | 500 | Server error |

---

## Swagger Documentation

Interactive API documentation is available at:

```
http://localhost:8080/swagger/index.html
```

---

## Examples

### cURL Examples

#### List Campaigns
```bash
curl -X GET "http://localhost:8080/api/v1/campaigns" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "X-Tenant-ID: tenant-001" \
  -H "Content-Type: application/json"
```

#### Create Campaign
```bash
curl -X POST "http://localhost:8080/api/v1/campaigns" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "X-Tenant-ID: tenant-001" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Welcome Campaign",
    "type": "EMAIL",
    "subject": "Welcome to Our Store!",
    "content": "<html><body><h1>Welcome!</h1></body></html>"
  }'
```

#### Validate Coupon
```bash
curl -X POST "http://localhost:8080/api/v1/storefront/coupons/validate" \
  -H "X-Tenant-ID: tenant-001" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "WELCOME10",
    "orderTotal": 100.00
  }'
```

#### Check Mautic Status
```bash
curl -X GET "http://localhost:8080/api/v1/integrations/mautic/status" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "X-Tenant-ID: tenant-001"
```

---

## Deployment

### Kubernetes

The service is deployed via ArgoCD with the following configuration:

- **Namespace:** `marketplace`
- **Replicas:** Auto-scaled (1-10)
- **Service Account:** `marketing-service-sa` (with Workload Identity)
- **Resources:**
  - CPU: 100m-500m
  - Memory: 128Mi-512Mi

### Health Probes

- **Liveness:** `GET /health`
- **Readiness:** `GET /ready`

---

## Support

For issues or questions, contact the platform team or create an issue in the repository.
