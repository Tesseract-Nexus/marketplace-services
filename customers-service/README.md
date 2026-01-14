# Customers Service

Customer management microservice for Tesseract Hub. Provides comprehensive customer data management, segmentation, and analytics capabilities.

## Features

- ✅ Customer CRUD operations
- ✅ Customer segmentation and tagging
- ✅ Address management (shipping, billing)
- ✅ Payment method storage (tokenized, no sensitive data)
- ✅ Customer notes and comments
- ✅ Communication history tracking
- ✅ Customer analytics (LTV, AOV, total orders, etc.)
- ✅ Multi-tenant support

## Tech Stack

- **Language**: Go 1.22
- **Framework**: Gin
- **Database**: PostgreSQL with GORM
- **Port**: 8089

## Getting Started

### Prerequisites

- Go 1.22+
- PostgreSQL 15+

### Installation

```bash
# Install dependencies
go mod tidy

# Set up environment variables
cp .env.example .env
# Edit .env with your database credentials

# Run database migration
make migrate

# Build the service
make build

# Run the service
make run
```

### Database Setup

The service uses PostgreSQL with the following tables:

- `customers` - Main customer entity
- `customer_addresses` - Customer addresses
- `customer_payment_methods` - Saved payment methods (tokenized)
- `customer_segments` - Customer segments/groups
- `customer_segment_members` - Segment membership (many-to-many)
- `customer_notes` - Internal notes about customers
- `customer_communications` - Communication history

## API Endpoints

### Customers

#### List Customers
```
GET /api/v1/customers?tenant_id={tenantId}&search={query}&status={status}&page=1&page_size=20
```

Query Parameters:
- `tenant_id` (required): Tenant ID
- `search`: Search by email, first name, or last name
- `status`: Filter by status (ACTIVE, INACTIVE, BLOCKED)
- `customer_type`: Filter by type (RETAIL, WHOLESALE, VIP)
- `page`: Page number (default: 1)
- `page_size`: Items per page (default: 20, max: 100)
- `sort_by`: Sort field (default: created_at)
- `sort_order`: Sort order (asc, desc)

Response:
```json
{
  "customers": [...],
  "total": 100,
  "page": 1,
  "pageSize": 20,
  "totalPages": 5
}
```

#### Get Customer
```
GET /api/v1/customers/:id?tenant_id={tenantId}
```

#### Create Customer
```
POST /api/v1/customers
```

Body:
```json
{
  "tenantId": "tenant-123",
  "email": "customer@example.com",
  "firstName": "John",
  "lastName": "Doe",
  "phone": "+1234567890",
  "customerType": "RETAIL",
  "marketingOptIn": true,
  "tags": ["vip", "early-adopter"]
}
```

#### Update Customer
```
PUT /api/v1/customers/:id?tenant_id={tenantId}
```

Body:
```json
{
  "firstName": "Jane",
  "status": "ACTIVE",
  "tags": ["vip"]
}
```

#### Delete Customer
```
DELETE /api/v1/customers/:id?tenant_id={tenantId}
```

### Addresses

#### Get Customer Addresses
```
GET /api/v1/customers/:id/addresses?tenant_id={tenantId}
```

#### Add Address
```
POST /api/v1/customers/:id/addresses
```

Body:
```json
{
  "tenantId": "tenant-123",
  "addressType": "SHIPPING",
  "isDefault": true,
  "addressLine1": "123 Main St",
  "city": "San Francisco",
  "state": "CA",
  "postalCode": "94102",
  "country": "US"
}
```

### Notes

#### Get Customer Notes
```
GET /api/v1/customers/:id/notes?tenant_id={tenantId}
```

#### Add Note
```
POST /api/v1/customers/:id/notes
```

Body:
```json
{
  "tenantId": "tenant-123",
  "note": "Customer requested expedited shipping"
}
```

### Communication History

#### Get Communications
```
GET /api/v1/customers/:id/communications?tenant_id={tenantId}&limit=50
```

## Customer Model

```go
type Customer struct {
    ID         uuid.UUID      // Customer ID
    TenantID   string         // Tenant ID for multi-tenancy
    UserID     *uuid.UUID     // Link to user account (NULL for guest customers)
    Email      string         // Email address
    FirstName  string         // First name
    LastName   string         // Last name
    Phone      string         // Phone number
    Status     CustomerStatus // ACTIVE, INACTIVE, BLOCKED
    CustomerType CustomerType // RETAIL, WHOLESALE, VIP

    // Analytics
    TotalOrders       int       // Total number of orders
    TotalSpent        float64   // Total amount spent
    AverageOrderValue float64   // Average order value
    LifetimeValue     float64   // Customer lifetime value
    LastOrderDate     *time.Time // Date of last order
    FirstOrderDate    *time.Time // Date of first order

    // Engagement
    Tags           []string // Tags for segmentation
    Notes          string   // Internal notes
    MarketingOptIn bool     // Marketing consent

    // Timestamps
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt *time.Time
}
```

## Integration with Orders Service

When an order is created/completed, the orders-service should call the customers-service to update customer statistics:

```go
// Update customer stats after order
customerService.RecordOrder(ctx, customerID, orderTotal, orderDate)
```

This will automatically update:
- `total_orders`
- `total_spent`
- `average_order_value`
- `lifetime_value`
- `last_order_date`
- `first_order_date` (if first order)

## Customer Segmentation

Customers can be organized into segments for targeted marketing and analysis. Segments can be:

- **Dynamic**: Auto-updated based on rules (e.g., "customers who spent > $1000")
- **Manual**: Manually assigned

Example segment rules (stored as JSONB):
```json
{
  "conditions": [
    { "field": "total_spent", "operator": "gt", "value": 1000 },
    { "field": "total_orders", "operator": "gte", "value": 5 }
  ],
  "logic": "AND"
}
```

## Security

- All endpoints require `tenant_id` for multi-tenant isolation
- Payment methods store only tokenized references (no card numbers)
- Customer data is soft-deleted (preserves order history)
- GDPR compliance: Customers can be fully deleted upon request

## Health Checks

- `GET /health` - Database connectivity check
- `GET /ready` - Readiness probe
- `GET /metrics` - Prometheus metrics

## Development

```bash
# Run tests
make test

# Build
make build

# Run locally
make run

# Clean build artifacts
make clean
```

## Docker

```bash
# Build image
docker build -t customers-service .

# Run container
docker run -p 8089:8089 \
  -e DATABASE_URL=postgres://user:pass@host:5432/db \
  customers-service
```

## Environment Variables

- `PORT`: Service port (default: 8089)
- `DATABASE_URL`: PostgreSQL connection string
- `ENVIRONMENT`: Environment (development, production)

## License

Copyright © 2026 Tesseract Hub
