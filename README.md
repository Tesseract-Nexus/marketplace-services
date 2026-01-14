# Marketplace Services

Business and marketplace-specific microservices for the Tesseract multi-tenant e-commerce platform. These services handle core marketplace functionality including orders, products, payments, inventory, and more.

## Architecture

```
marketplace-services/
├── approval-service/        # Workflow approval management
├── bergamot-service/        # Mozilla Bergamot translation (Python)
├── categories-service/      # Product category hierarchy
├── content-service/         # CMS content management
├── coupons-service/         # Discount coupon management
├── customers-service/       # Customer profiles & management
├── gift-cards-service/      # Gift card issuance & redemption
├── huggingface-mt-service/  # HuggingFace ML translation (Python)
├── inventory-service/       # Stock & inventory management
├── marketing-service/       # Marketing campaigns & automation
├── marketplace-connector-service/  # External marketplace integrations
├── orders-service/          # Order lifecycle management
├── payment-service/         # Payment processing & gateways
├── products-service/        # Product catalog management
├── reviews-service/         # Product reviews & ratings
├── shipping-service/        # Shipping & fulfillment
├── staff-service/           # Staff management & permissions
├── tax-service/             # Tax calculation & compliance
├── tickets-service/         # Customer support tickets
└── vendor-service/          # Vendor/seller management
```

## Services Overview

### Core Commerce Services

| Service | Description | Port | Tech |
|---------|-------------|------|------|
| **products-service** | Product catalog, variants, attributes, pricing | 8080 | Go |
| **orders-service** | Order creation, lifecycle, fulfillment tracking | 8080 | Go |
| **inventory-service** | Stock levels, reservations, warehouse management | 8088 | Go |
| **payment-service** | Payment processing, Stripe/PayPal integration | 8080 | Go |
| **shipping-service** | Shipping rates, carrier integration, tracking | 8080 | Go |

### Customer & Vendor Services

| Service | Description | Port | Tech |
|---------|-------------|------|------|
| **customers-service** | Customer profiles, addresses, preferences | 8080 | Go |
| **vendor-service** | Vendor onboarding, management, payouts | 8080 | Go |
| **staff-service** | Staff accounts, roles, permissions | 8080 | Go |
| **reviews-service** | Product reviews, ratings, moderation | 8080 | Go |

### Marketing & Promotions

| Service | Description | Port | Tech |
|---------|-------------|------|------|
| **coupons-service** | Coupon creation, validation, redemption | 8080 | Go |
| **gift-cards-service** | Gift card issuance, balance, redemption | 8080 | Go |
| **marketing-service** | Email campaigns, automation, analytics | 8080 | Go |

### Content & Categories

| Service | Description | Port | Tech |
|---------|-------------|------|------|
| **categories-service** | Product category hierarchy, attributes | 8080 | Go |
| **content-service** | CMS pages, blocks, media management | 8080 | Go |

### Operations & Support

| Service | Description | Port | Tech |
|---------|-------------|------|------|
| **approval-service** | Workflow approvals, delegation | 8099 | Go |
| **tickets-service** | Support tickets, SLA management | 8080 | Go |
| **tax-service** | Tax calculation, rates, compliance | 8091 | Go |

### Translation Services

| Service | Description | Port | Tech |
|---------|-------------|------|------|
| **bergamot-service** | Mozilla Bergamot neural translation | 8080 | Python |
| **huggingface-mt-service** | HuggingFace machine translation | 8080 | Python |

### Integration Services

| Service | Description | Port | Tech |
|---------|-------------|------|------|
| **marketplace-connector-service** | Amazon, eBay, Shopify integrations | 8099 | Go |

## Tech Stack

### Go Services
- **Language**: Go 1.25
- **Framework**: Gin HTTP framework
- **ORM**: GORM with PostgreSQL
- **Messaging**: NATS for event-driven communication
- **Caching**: Redis
- **Shared Library**: [github.com/Tesseract-Nexus/go-shared](https://github.com/Tesseract-Nexus/go-shared)

### Python Services
- **Language**: Python 3.11
- **Framework**: FastAPI
- **ML**: HuggingFace Transformers, Mozilla Bergamot

## Development

### Prerequisites
- Go 1.25+
- Python 3.11+ (for translation services)
- Docker & Docker Compose
- PostgreSQL 15+
- Redis 7+
- NATS Server

### Running a Service Locally

```bash
# Go service
cd products-service
go mod download
go run ./cmd

# Python service
cd bergamot-service
pip install -r requirements.txt
python main.py
```

### Building Docker Images

```bash
# Build a specific service
docker build -t products-service ./products-service

# Or use docker-compose
docker-compose up products-service
```

## CI/CD

Each service has its own GitHub Actions workflow that:
1. Runs on push to main or feature branches
2. Builds and tests the service
3. Builds Docker image
4. Pushes to GitHub Container Registry (ghcr.io)
5. Runs Trivy security scanning

## Related Repositories

- [go-shared](https://github.com/Tesseract-Nexus/go-shared) - Shared Go library
- [global-services](https://github.com/Tesseract-Nexus/global-services) - Platform infrastructure services
- [marketplace-clients](https://github.com/Tesseract-Nexus/marketplace-clients) - Frontend applications

## License

Proprietary - Tesseract Nexus
