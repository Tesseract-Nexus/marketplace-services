# Products Service

A comprehensive products and catalog management service with multi-tenant support, product variants, inventory management, and analytics.

## Features

- **Product Management**: Full CRUD operations for products
- **Product Variants**: Support for product variations (size, color, etc.)
- **Category Management**: Hierarchical product categories
- **Inventory Tracking**: Real-time inventory management with low stock alerts
- **Multi-tenant Support**: Isolated data per tenant
- **Search & Filtering**: Advanced product search with filters
- **Analytics**: Product statistics and trends
- **RESTful API**: OpenAPI 3.0 specification
- **Authentication**: Azure AD integration with development fallback

## Architecture

The service follows Clean Architecture principles:

```
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── config/                 # Configuration management
│   ├── handlers/               # HTTP handlers
│   ├── middleware/             # HTTP middleware
│   ├── models/                 # Domain models
│   └── repository/             # Data access layer
├── migrations/                 # Database migrations
├── Dockerfile                  # Container configuration
├── docker-compose.yml          # Local development setup
└── go.mod                      # Go module dependencies
```

## API Endpoints

### Products
- `POST /api/v1/products` - Create product
- `GET /api/v1/products` - List products with filters
- `GET /api/v1/products/{id}` - Get product details
- `PUT /api/v1/products/{id}` - Update product
- `DELETE /api/v1/products/{id}` - Delete product
- `PUT /api/v1/products/{id}/status` - Update product status
- `POST /api/v1/products/bulk/status` - Bulk status update

### Bulk Operations
- `POST /api/v1/products/bulk` - Bulk create products (max 100 per request)
- `DELETE /api/v1/products/bulk` - Bulk delete products

### Import/Export
- `GET /api/v1/products/import/template` - Download import template (CSV/XLSX)
- `POST /api/v1/products/import` - Import products from CSV/XLSX file

### Product Variants
- `POST /api/v1/products/{id}/variants` - Create variant
- `GET /api/v1/products/{id}/variants` - List variants
- `PUT /api/v1/products/{id}/variants/{variantId}` - Update variant
- `DELETE /api/v1/products/{id}/variants/{variantId}` - Delete variant

### Inventory
- `PUT /api/v1/products/{id}/inventory` - Update inventory
- `POST /api/v1/products/{id}/inventory/adjustment` - Adjust inventory
- `POST /api/v1/products/bulk/deduct` - Bulk deduct inventory (for order placement)
- `POST /api/v1/products/bulk/restore` - Bulk restore inventory (for order cancellation)
- `POST /api/v1/products/inventory/check` - Check stock availability

### Product Images
- `POST /api/v1/products/images/upload` - Upload product images
- `GET /api/v1/products/{id}/images` - Get product images
- `DELETE /api/v1/products/{id}/images/storage/:bucket/*path` - Delete stored image
- `POST /api/v1/products/{id}/images/presigned-url` - Generate presigned URL for uploads

### Categories
- `GET /api/v1/categories` - List categories
- `POST /api/v1/categories` - Create category
- `GET /api/v1/categories/{id}` - Get category
- `PUT /api/v1/categories/{id}` - Update category
- `DELETE /api/v1/categories/{id}` - Delete category
- `GET /api/v1/products/categories/{categoryId}` - Get products by category

### Analytics
- `GET /api/v1/products/analytics` - Product analytics
- `GET /api/v1/products/stats` - Product statistics
- `POST /api/v1/products/export` - Export products

### Search
- `POST /api/v1/products/search` - Advanced product search
- `GET /api/v1/products/trending` - Trending products
- `GET /api/v1/products/search/suggestions` - Get search suggestions
- `GET /api/v1/products/available-filters` - Get available filters for search
- `POST /api/v1/products/search/track` - Track search events
- `GET /api/v1/products/search/analytics` - Get search analytics

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Docker (optional)

### Local Development

1. **Clone and navigate to the service directory**
   ```bash
   cd domains/ecommerce/services/products-service
   ```

2. **Set up environment variables**
   Create a `.env` file:
   ```env
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=password
   DB_NAME=products_db
   PORT=8087
   ENVIRONMENT=development
   JWT_SECRET=your-secret-key
   ```

3. **Install dependencies**
   ```bash
   go mod download
   ```

4. **Run database migrations**
   ```bash
   # Apply the migration script to your PostgreSQL database
   psql -h localhost -U postgres -d products_db -f migrations/001_initial_schema.sql
   ```

5. **Run the service**
   ```bash
   go run cmd/main.go
   ```

### Docker Development

1. **Start the service with Docker Compose**
   ```bash
   docker-compose up -d
   ```

2. **View logs**
   ```bash
   docker-compose logs -f products-service
   ```

3. **Stop the service**
   ```bash
   docker-compose down
   ```

## Configuration

The service can be configured using environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | localhost | Database host |
| `DB_PORT` | 5432 | Database port |
| `DB_USER` | postgres | Database username |
| `DB_PASSWORD` | password | Database password |
| `DB_NAME` | products_db | Database name |
| `PORT` | 8087 | Service port |
| `ENVIRONMENT` | development | Environment (development/production) |
| `JWT_SECRET` | your-secret-key | JWT signing secret |
| `DEFAULT_PAGE_SIZE` | 20 | Default pagination size |
| `MAX_PAGE_SIZE` | 100 | Maximum pagination size |
| `MAX_PRODUCT_IMAGES` | 20 | Maximum images per product |
| `MAX_PRODUCT_VARIANTS` | 100 | Maximum variants per product |
| `DEFAULT_CURRENCY` | USD | Default currency code |
| `INVENTORY_TRACKING` | true | Enable inventory tracking |

## API Request/Response Schemas

### Bulk Create Products

**Request:**
```json
POST /api/v1/products/bulk
{
  "products": [
    {
      "name": "Product Name",
      "sku": "SKU-001",
      "price": "29.99",
      "comparePrice": "39.99",
      "costPrice": "15.00",
      "description": "Product description",
      "shortDescription": "Short desc",
      "categoryId": "uuid",
      "vendorId": "uuid",
      "status": "ACTIVE",
      "quantity": 100,
      "lowStockThreshold": 10,
      "weight": 1.5,
      "tags": ["tag1", "tag2"],
      "isTaxable": true,
      "externalId": "ext-001"
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
    "products": [...],
    "errors": []
  }
}
```

### Import Products from File

**Request:**
```
POST /api/v1/products/import
Content-Type: multipart/form-data

file: <CSV or XLSX file>
skipDuplicates: true (optional)
```

**Response:**
```json
{
  "success": true,
  "data": {
    "imported": 50,
    "skipped": 5,
    "failed": 2,
    "errors": [
      {"row": 3, "field": "price", "message": "Invalid price format"}
    ]
  }
}
```

### Download Import Template

**Request:**
```
GET /api/v1/products/import/template?format=csv
GET /api/v1/products/import/template?format=xlsx
```

**Template Columns:**
| Column | Required | Type | Description |
|--------|----------|------|-------------|
| name | Yes | string | Product name |
| sku | Yes | string | Unique SKU |
| price | Yes | decimal | Selling price |
| compare_price | No | decimal | Original/compare price |
| cost_price | No | decimal | Cost price |
| description | No | string | Full description |
| short_description | No | string | Brief description |
| category_id | No | uuid | Category UUID |
| vendor_id | No | uuid | Vendor UUID |
| status | No | string | DRAFT, ACTIVE, INACTIVE |
| quantity | No | integer | Stock quantity |
| low_stock_threshold | No | integer | Low stock alert level |
| weight | No | decimal | Weight in kg |
| tags | No | string | Comma-separated tags |
| is_taxable | No | boolean | true/false |
| external_id | No | string | External system ID |

## Database Schema

### Products Table
- Multi-tenant product storage
- JSONB fields for flexible attributes, images, and metadata
- Full-text search capabilities
- Inventory tracking
- Status management

### Product Variants Table
- Product variations (size, color, etc.)
- Individual inventory tracking
- JSONB attributes for flexibility

### Categories Table
- Hierarchical category structure
- Multi-tenant support
- Flexible metadata storage

## API Documentation

The service includes Swagger/OpenAPI documentation available at:
- `http://localhost:8087/swagger/index.html` (when running locally)

## Health Checks

- `GET /health` - Service health status
- `GET /ready` - Service readiness status

## Security

- **Authentication**: Azure AD JWT tokens (production) or simple header auth (development)
- **Authorization**: Tenant-based data isolation
- **CORS**: Configurable cross-origin resource sharing
- **Input Validation**: Request validation with detailed error messages

## Sample Data

The migration includes sample categories and products for development:
- Electronics category with wireless headphones
- Clothing category with cotton t-shirts
- Books category with programming guides
- Multiple product variants with different colors and sizes

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test ./internal/handlers -v
```

## Production Deployment

1. **Build the Docker image**
   ```bash
   docker build -t products-service:latest .
   ```

2. **Configure environment variables for production**
3. **Deploy with your orchestration platform** (Kubernetes, Docker Swarm, etc.)
4. **Run database migrations**
5. **Configure monitoring and logging**

## Monitoring

The service exposes standard HTTP metrics and logs structured JSON for monitoring integration.

## Contributing

1. Follow Go coding standards
2. Add tests for new features
3. Update documentation
4. Ensure all tests pass
5. Follow the existing code patterns

## License

MIT License - see LICENSE file for details.