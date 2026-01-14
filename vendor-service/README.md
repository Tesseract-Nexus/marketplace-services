# Vendor Service

Enterprise-grade vendor and storefront management microservice for the Tesseract Hub platform. Provides multi-tenant vendor lifecycle management, compliance document tracking, and storefront resolution capabilities.

## Features

- **Vendor Management**: Full CRUD with status and validation tracking
- **Storefront Management**: Multiple storefronts per vendor with custom domains
- **Document Management**: Compliance document tracking with expiry dates
- **Storefront Resolution**: Resolve tenant by slug or custom domain
- **Analytics**: Vendor performance metrics and statistics

## Tech Stack

- **Language**: Go 1.25
- **Framework**: Gin v1.10.0
- **Database**: PostgreSQL with GORM v1.25.7
- **Port**: 8081

## API Endpoints

### Vendors
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/vendors` | Create vendor |
| GET | `/api/v1/vendors` | List vendors with filters |
| GET | `/api/v1/vendors/:id` | Get vendor by ID |
| PUT | `/api/v1/vendors/:id` | Update vendor |
| DELETE | `/api/v1/vendors/:id` | Delete vendor (soft) |
| PUT | `/api/v1/vendors/:id/status` | Update vendor status |
| PUT | `/api/v1/vendors/:id/validationStatus` | Update validation status |
| GET | `/api/v1/vendors/analytics` | Get vendor analytics |
| POST | `/api/v1/vendors/bulk` | Bulk create vendors |

### Vendor Documents
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/vendors/documents/upload` | Upload document |
| GET | `/api/v1/vendors/:id/documents` | Get vendor documents |
| DELETE | `/api/v1/vendors/:id/documents/:bucket/*path` | Delete document |
| POST | `/api/v1/vendors/:id/documents/presigned-url` | Generate presigned URL |

### Storefronts
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/storefronts` | Create storefront |
| GET | `/api/v1/storefronts` | List storefronts |
| GET | `/api/v1/storefronts/:id` | Get storefront |
| PUT | `/api/v1/storefronts/:id` | Update storefront |
| DELETE | `/api/v1/storefronts/:id` | Delete storefront |
| GET | `/api/v1/storefronts/resolve/by-slug/:slug` | Resolve by slug |
| GET | `/api/v1/storefronts/resolve/by-domain/:domain` | Resolve by domain |
| GET | `/api/v1/vendors/:id/storefronts` | Get vendor's storefronts |

### Health
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |

## Environment Variables

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=vendor_db
DB_SSL_MODE=disable

# Server
PORT=8081
ENVIRONMENT=development

# Authentication
JWT_SECRET=your-secret-key
AZURE_TENANT_ID=your-azure-tenant-id
AZURE_APPLICATION_ID=your-azure-application-id

# External Services
DOCUMENT_SERVICE_URL=http://localhost:8082
STAFF_SERVICE_URL=http://localhost:8080

# Pagination
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
```

## Data Models

### Vendor
- Status: PENDING, ACTIVE, INACTIVE, SUSPENDED, TERMINATED
- Validation Status: NOT_STARTED, IN_PROGRESS, COMPLETED, FAILED, EXPIRED
- Business information: registration, tax ID, founded year
- Contract management: dates, value, payment terms
- Performance metrics: rating, review dates
- IsOwnerVendor flag for tenant's own vendor

### Storefront
- Unique slug (3-100 chars, globally unique)
- Optional custom domain (globally unique)
- Theme config and settings (JSONB)
- SEO metadata: meta title, description
- Logo and favicon URLs

### Document Types
- compliance, certification, insurance, contract
- tax_document, bank_statement, identity_proof, address_proof

## Running Locally

```bash
# Set environment variables
cp .env.example .env

# Run with Docker
docker-compose up

# Or run directly
go run cmd/main.go
```

## License

MIT
