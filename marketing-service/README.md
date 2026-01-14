# Marketing Service

Complete marketing automation microservice for the Tesseract Hub platform. Provides campaigns, segmentation, abandoned cart recovery, loyalty programs, coupon management, and Mautic email marketing integration.

## Features

- **Campaign Management**: Multi-channel marketing campaigns with Mautic sync
- **Customer Segmentation**: Static and dynamic segments synced to Mautic
- **Abandoned Cart Recovery**: Automated recovery workflows
- **Loyalty Programs**: Points, tiers, and rewards
- **Coupon Management**: Discount codes with validation
- **Mautic Integration**: Email marketing automation via Mautic API
- **Multi-Tenant**: Full tenant isolation with RBAC security

## Tech Stack

- **Language**: Go 1.21+
- **Framework**: Gin
- **Database**: PostgreSQL with GORM
- **Secrets**: GCP Secret Manager (via Workload Identity)
- **Email**: Mautic API integration

## Documentation

- **[API Documentation](./API.md)** - Complete REST API reference
- **Swagger**: Available at `/swagger/index.html` when running

## API Endpoints

### Campaigns
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/campaigns` | Create campaign |
| GET | `/api/v1/campaigns` | List campaigns |
| GET | `/api/v1/campaigns/:id` | Get campaign |
| PUT | `/api/v1/campaigns/:id` | Update campaign |
| DELETE | `/api/v1/campaigns/:id` | Delete campaign |
| POST | `/api/v1/campaigns/:id/send` | Send immediately |
| POST | `/api/v1/campaigns/:id/schedule` | Schedule sending |

### Customer Segments
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/segments` | Create segment |
| GET | `/api/v1/segments` | List segments |
| GET | `/api/v1/segments/:id` | Get segment |
| PUT | `/api/v1/segments/:id` | Update segment |
| DELETE | `/api/v1/segments/:id` | Delete segment |

### Abandoned Carts
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/abandoned-carts` | Create cart record |
| GET | `/api/v1/abandoned-carts` | List abandoned carts |
| GET | `/api/v1/abandoned-carts/stats` | Cart statistics |

### Loyalty Program
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/loyalty/program` | Create program |
| GET | `/api/v1/loyalty/program` | Get program |
| PUT | `/api/v1/loyalty/program` | Update program |
| GET | `/api/v1/loyalty/customers/:id` | Get customer loyalty |
| POST | `/api/v1/loyalty/customers/:id/enroll` | Enroll customer |
| POST | `/api/v1/loyalty/customers/:id/redeem` | Redeem points |
| GET | `/api/v1/loyalty/customers/:id/transactions` | Transaction history |

### Coupons
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/coupons` | Create coupon |
| GET | `/api/v1/coupons` | List coupons |
| GET | `/api/v1/coupons/:id` | Get coupon |
| PUT | `/api/v1/coupons/:id` | Update coupon |
| DELETE | `/api/v1/coupons/:id` | Delete coupon |
| POST | `/api/v1/coupons/validate` | Validate coupon |

## Campaign Types

- PROMOTION, ABANDONED_CART, WELCOME
- WINBACK, PRODUCT_LAUNCH, NEWSLETTER
- TRANSACTIONAL, RE_ENGAGEMENT

## Campaign Channels

- EMAIL, SMS, PUSH, IN_APP, MULTI

## Campaign Status

- DRAFT → SCHEDULED → SENDING → SENT/COMPLETED

## Segment Types

- **Static**: Manual customer lists
- **Dynamic**: Rule-based, auto-updating

## Segment Operators

- `gt`, `lt`, `eq`, `between`, `in`

## Abandoned Cart Workflow

- PENDING → REMINDED → RECOVERED/EXPIRED/IGNORED
- Max 3 recovery attempts
- 24-hour minimum between reminders
- 7-day default expiration

## Loyalty Features

- Configurable points per dollar
- Multi-tier structure with benefits
- Signup, birthday, referral bonuses
- Points expiration management

## Coupon Types

- PERCENTAGE, FIXED_AMOUNT
- FREE_SHIPPING, BUY_X_GET_Y

## Coupon Validation

- Active status check
- Date range validation
- Global usage limits
- Per-customer usage limits
- Order amount constraints
- Product/category rules

## Data Models

### Campaign
- Multi-channel with templates
- Segment targeting or broadcast
- Analytics: sent, delivered, opened, clicked, converted

### CustomerSegment
- Rules stored as JSONB
- Customer count tracking

### AbandonedCart
- Cart items (JSONB)
- Recovery workflow status
- Recovered order tracking

### LoyaltyProgram
- Points configuration
- Tiers (JSONB)
- Bonus settings

### CustomerLoyalty
- Total and available points
- Current tier tracking

### CouponCode
- Discount configuration
- Usage limits
- Validity dates
- Product rules

## Mautic Integration

The service integrates with Mautic for email marketing automation.

### Mautic Endpoints
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/integrations/mautic/status` | Check integration status |
| POST | `/api/v1/integrations/mautic/sync/campaign` | Sync campaign to Mautic |
| POST | `/api/v1/integrations/mautic/sync/segment` | Sync segment to Mautic |
| POST | `/api/v1/integrations/mautic/contacts` | Create contact in Mautic |
| POST | `/api/v1/integrations/mautic/segments/add-contact` | Add contact to segment |
| POST | `/api/v1/integrations/mautic/test-email` | Send test email |

### How It Works

1. **Campaign Sync**: When a campaign is created/sent, it's automatically synced to Mautic
2. **Segment Sync**: Customer segments are mirrored as Mautic segments
3. **Contact Management**: Customer data is synced to Mautic contacts
4. **Email Delivery**: Emails are sent via Mautic's email infrastructure (Postal)

## Storefront (Public) Endpoints

These endpoints don't require JWT authentication, only tenant identification via headers.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/storefront/loyalty/program` | Get loyalty program info |
| GET | `/api/v1/storefront/loyalty/customer` | Get customer loyalty status |
| POST | `/api/v1/storefront/loyalty/enroll` | Enroll in loyalty program |
| POST | `/api/v1/storefront/loyalty/redeem` | Redeem loyalty points |
| GET | `/api/v1/storefront/loyalty/transactions` | Get point transactions |
| GET | `/api/v1/storefront/loyalty/referrals` | Get referral info |
| POST | `/api/v1/storefront/coupons/validate` | Validate coupon code |

## Environment Configuration

### Required Variables

```bash
# Server
PORT=8080
GIN_MODE=release

# Database
DB_HOST=postgresql.postgresql-marketplace.svc.cluster.local
DB_PORT=5432
DB_USER=postgres
DB_NAME=marketing_db
DB_SSLMODE=require

# GCP Secret Manager
USE_GCP_SECRET_MANAGER=true
GCP_PROJECT_ID=tesseracthub-480811
GCP_SECRET_PREFIX=devtest
DB_PASSWORD_SECRET_NAME=devtest-marketplace-postgresql-password

# Mautic
MAUTIC_ENABLED=true
MAUTIC_URL=http://mautic.email.svc.cluster.local
MAUTIC_USERNAME=admin
MAUTIC_PASSWORD_SECRET_NAME=devtest-mautic-api-password

# Email
FROM_EMAIL=noreply@mail.tesserix.app
FROM_NAME=Tesseract Hub

# Services
STAFF_SERVICE_URL=http://staff-service.marketplace.svc.cluster.local:8080
NATS_URL=nats://nats.nats.svc.cluster.local:4222
```

## GCP Secret Manager

The service uses GCP Secret Manager for credential management via Workload Identity.

### Secrets Used

| Secret Name | Description |
|-------------|-------------|
| `{prefix}-marketplace-postgresql-password` | Database password |
| `{prefix}-mautic-api-password` | Mautic admin API password |
| `{prefix}-jwt-secret` | JWT signing key |

### How It Works

1. Pod runs with Kubernetes Service Account (`marketing-service-sa`)
2. Service Account is bound to GCP Service Account via Workload Identity
3. GCP Service Account has `secretmanager.secretAccessor` role
4. Application fetches secrets at startup using GCP SDK

## RBAC Permissions

| Permission | Description |
|------------|-------------|
| `marketing:campaigns:view` | View campaigns |
| `marketing:campaigns:manage` | Manage campaigns |
| `marketing:segments:view` | View segments |
| `marketing:segments:manage` | Manage segments |
| `marketing:loyalty:view` | View loyalty data |
| `marketing:loyalty:manage` | Manage loyalty programs |
| `marketing:loyalty:points:adjust` | Adjust customer points |
| `marketing:coupons:view` | View coupons |
| `marketing:coupons:manage` | Manage coupons |
| `marketing:carts:view` | View abandoned carts |
| `marketing:carts:recover` | Create recovery campaigns |
| `marketing:email:send` | Send marketing emails |

## Running Locally

```bash
# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export MAUTIC_ENABLED=false

# Run the service
go run cmd/main.go
```

## Docker

```bash
# Build
docker build -t marketing-service .

# Run
docker run -p 8080:8080 \
  -e DB_HOST=host.docker.internal \
  -e MAUTIC_ENABLED=false \
  marketing-service
```

## Kubernetes Deployment

The service is deployed via ArgoCD with the following resources:

- **Namespace**: `marketplace`
- **Replicas**: 1-10 (auto-scaled via KEDA)
- **Service Account**: `marketing-service-sa`
- **Resources**: CPU 100m-500m, Memory 128Mi-512Mi

## License

MIT
