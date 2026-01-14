# Inventory Service

Multi-tenant inventory management microservice for the Tesseract Hub platform. Manages warehouses, suppliers, purchase orders, inventory transfers, and stock levels.

## Features

- **Warehouse Management**: Create and manage multiple warehouses with priority and default settings
- **Supplier Management**: Track suppliers with performance metrics and payment terms
- **Purchase Orders**: Full lifecycle management with automatic stock updates on receipt
- **Inventory Transfers**: Move stock between warehouses with transactional integrity
- **Stock Tracking**: Real-time stock levels with reorder point alerts
- **Reservations**: Stock reservation system for pending orders

## Tech Stack

- **Language**: Go 1.25
- **Framework**: Gin v1.9.1
- **Database**: PostgreSQL with GORM v1.25.5
- **Port**: 8088

## API Endpoints

### Warehouses
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/warehouses` | Create warehouse |
| GET | `/api/v1/warehouses` | List warehouses |
| GET | `/api/v1/warehouses/:id` | Get warehouse |
| PUT | `/api/v1/warehouses/:id` | Update warehouse |
| DELETE | `/api/v1/warehouses/:id` | Delete warehouse |
| POST | `/api/v1/warehouses/bulk` | Bulk create warehouses (max 100) |
| DELETE | `/api/v1/warehouses/bulk` | Bulk delete warehouses |
| GET | `/api/v1/warehouses/import/template` | Download import template |
| POST | `/api/v1/warehouses/import` | Import warehouses from file |

### Suppliers
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/suppliers` | Create supplier |
| GET | `/api/v1/suppliers` | List suppliers |
| GET | `/api/v1/suppliers/:id` | Get supplier |
| PUT | `/api/v1/suppliers/:id` | Update supplier |
| DELETE | `/api/v1/suppliers/:id` | Delete supplier |
| POST | `/api/v1/suppliers/bulk` | Bulk create suppliers (max 100) |
| DELETE | `/api/v1/suppliers/bulk` | Bulk delete suppliers |
| GET | `/api/v1/suppliers/import/template` | Download import template |
| POST | `/api/v1/suppliers/import` | Import suppliers from file |

### Purchase Orders
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/purchase-orders` | Create purchase order |
| GET | `/api/v1/purchase-orders` | List purchase orders |
| GET | `/api/v1/purchase-orders/:id` | Get purchase order |
| PUT | `/api/v1/purchase-orders/:id/status` | Update PO status |
| POST | `/api/v1/purchase-orders/:id/receive` | Receive PO and update stock |

### Inventory Transfers
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/transfers` | Create transfer |
| GET | `/api/v1/transfers` | List transfers |
| GET | `/api/v1/transfers/:id` | Get transfer |
| PUT | `/api/v1/transfers/:id/status` | Update transfer status |
| POST | `/api/v1/transfers/:id/complete` | Complete transfer |

### Stock Levels
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/stock` | List all stock levels |
| GET | `/api/v1/stock/level` | Get stock for product/warehouse |
| GET | `/api/v1/stock/low` | Get low stock items |

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
DB_NAME=inventory_db
DB_SSL_MODE=disable

# Server
PORT=8088
ENVIRONMENT=development

# Authentication
JWT_SECRET=your-secret-key

# Pagination
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
```

## Data Models

### Warehouse
- UUID primary key with tenant isolation
- Status: ACTIVE, INACTIVE, CLOSED
- Complete address and contact management
- IsDefault flag (one per tenant)
- Priority for ordering

### Supplier
- Status: ACTIVE, INACTIVE, BLACKLISTED
- Payment terms and lead time configuration
- Performance metrics (rating, total orders, total spent)

### Purchase Order
- Auto-generated PO number: `PO-YYYYMM-000001`
- Status workflow: DRAFT → SUBMITTED → APPROVED → ORDERED → RECEIVED
- Financial tracking: subtotal, tax, shipping, total

### Inventory Transfer
- Auto-generated transfer number: `TR-YYYYMM-000001`
- Status: PENDING → IN_TRANSIT → COMPLETED
- Source and destination warehouse tracking

### Stock Level
- Composite unique: (warehouse, product, variant)
- Quantity tracking: on-hand, reserved, available
- Reorder point and quantity configuration

## API Request/Response Schemas

### Bulk Create Warehouses

**Request:**
```json
POST /api/v1/warehouses/bulk
{
  "warehouses": [
    {
      "name": "Main Warehouse",
      "code": "WH-001",
      "addressLine1": "123 Industrial Ave",
      "addressLine2": "Suite 100",
      "city": "Chicago",
      "state": "IL",
      "postalCode": "60601",
      "country": "US",
      "phone": "+1-555-123-4567",
      "email": "warehouse@company.com",
      "status": "ACTIVE",
      "isDefault": true,
      "priority": 1,
      "externalId": "ext-wh-001"
    }
  ],
  "skipDuplicates": true
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "created": 5,
    "skipped": 1,
    "failed": 0,
    "warehouses": [...],
    "errors": []
  }
}
```

### Import Warehouses Template

**Request:**
```
GET /api/v1/warehouses/import/template?format=csv
GET /api/v1/warehouses/import/template?format=xlsx
```

**Template Columns:**
| Column | Required | Type | Description |
|--------|----------|------|-------------|
| name | Yes | string | Warehouse name |
| code | Yes | string | Unique warehouse code |
| address_line_1 | Yes | string | Street address |
| address_line_2 | No | string | Additional address |
| city | Yes | string | City |
| state | Yes | string | State/Province |
| postal_code | Yes | string | ZIP/Postal code |
| country | Yes | string | Country code (US, CA, etc.) |
| phone | No | string | Contact phone |
| email | No | string | Contact email |
| status | No | string | ACTIVE, INACTIVE, CLOSED |
| is_default | No | boolean | true/false |
| priority | No | integer | Priority order |
| external_id | No | string | External system ID |

### Bulk Create Suppliers

**Request:**
```json
POST /api/v1/suppliers/bulk
{
  "suppliers": [
    {
      "name": "Acme Supplies",
      "code": "SUP-001",
      "email": "orders@acme.com",
      "phone": "+1-555-987-6543",
      "website": "https://acme.com",
      "contactName": "John Smith",
      "addressLine1": "456 Commerce St",
      "city": "New York",
      "state": "NY",
      "postalCode": "10001",
      "country": "US",
      "status": "ACTIVE",
      "paymentTerms": "NET30",
      "leadTimeDays": 7,
      "externalId": "ext-sup-001"
    }
  ],
  "skipDuplicates": true
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "created": 10,
    "skipped": 2,
    "failed": 0,
    "suppliers": [...],
    "errors": []
  }
}
```

### Import Suppliers Template

**Request:**
```
GET /api/v1/suppliers/import/template?format=csv
GET /api/v1/suppliers/import/template?format=xlsx
```

**Template Columns:**
| Column | Required | Type | Description |
|--------|----------|------|-------------|
| name | Yes | string | Supplier name |
| code | Yes | string | Unique supplier code |
| email | No | string | Email address |
| phone | No | string | Phone number |
| website | No | string | Website URL |
| contact_name | No | string | Primary contact |
| address_line_1 | No | string | Street address |
| city | No | string | City |
| state | No | string | State/Province |
| postal_code | No | string | ZIP/Postal code |
| country | No | string | Country code |
| status | No | string | ACTIVE, INACTIVE, BLACKLISTED |
| payment_terms | No | string | NET30, NET60, etc. |
| lead_time_days | No | integer | Lead time in days |
| external_id | No | string | External system ID |

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
