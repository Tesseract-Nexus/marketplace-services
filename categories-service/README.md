# Categories Service

A comprehensive microservice for managing hierarchical product categories with multi-tenant support.

## Features

- **Hierarchical Categories**: Parent-child relationships with automatic level calculation
- **Status Workflow**: Draft, Pending, Approved, and Rejected states
- **Audit Trail**: Complete change tracking for all category modifications
- **SEO Support**: Built-in SEO fields for search optimization
- **Position Management**: Flexible ordering within category levels
- **Multi-tenant Architecture**: Isolated data per tenant
- **Authentication**: JWT and Azure AD support
- **Circular Reference Prevention**: Database-level validation to prevent invalid hierarchies

## API Documentation

The service provides RESTful APIs with Swagger documentation available at `/swagger/index.html` when running.

### Key Endpoints

#### CRUD Operations
- `POST /api/v1/categories` - Create category
- `GET /api/v1/categories` - List categories with filters
- `GET /api/v1/categories/:id` - Get specific category
- `PUT /api/v1/categories/:id` - Update category
- `DELETE /api/v1/categories/:id` - Delete category
- `PUT /api/v1/categories/:id/status` - Update category status

#### Hierarchy & Organization
- `GET /api/v1/categories/tree` - Get hierarchical tree
- `POST /api/v1/categories/reorder` - Reorder positions

#### Bulk Operations
- `POST /api/v1/categories/bulk` - Bulk create categories (max 100 per request)
- `PUT /api/v1/categories/bulk` - Bulk update categories
- `DELETE /api/v1/categories/bulk` - Bulk delete categories

#### Import/Export
- `GET /api/v1/categories/import/template` - Download import template (CSV/XLSX)
- `POST /api/v1/categories/import` - Import categories from CSV/XLSX file

#### Export & Analytics
- `POST /api/v1/categories/export` - Export categories
- `GET /api/v1/categories/analytics` - Get category analytics

#### Audit
- `GET /api/v1/categories/:id/audit` - Get audit trail

## Docker Setup

### Using Docker Compose (Recommended)

```bash
# Start all services (PostgreSQL + API + Migrations)
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Manual Docker Build

```bash
# Build the image
docker build -t categories-service .

# Run with external PostgreSQL
docker run -p 8083:8083 \
  -e DB_HOST=localhost \
  -e DB_NAME=categories_db \
  -e DB_USER=postgres \
  -e DB_PASSWORD=password \
  categories-service
```

## Local Development

### Prerequisites

- Go 1.23+
- PostgreSQL 16+
- Git

### Setup

```bash
# Clone the repository
git clone <repository-url>
cd categories-service

# Install dependencies
go mod download

# Set up environment variables
cp .env.example .env

# Run database migrations
migrate -path ./migrations -database "postgres://postgres:password@localhost:5432/categories_db?sslmode=disable" up

# Run the service
go run cmd/main.go
```

### Environment Variables

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=categories_db
DB_SSL_MODE=disable

# Server
PORT=8083
ENVIRONMENT=development

# Authentication
JWT_SECRET=your-jwt-secret-key
AZURE_TENANT_ID=your-azure-tenant-id
AZURE_APPLICATION_ID=your-azure-app-id

# Pagination
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
```

## API Request/Response Schemas

### Bulk Create Categories

**Request:**
```json
POST /api/v1/categories/bulk
{
  "categories": [
    {
      "name": "Electronics",
      "description": "Electronic devices and accessories",
      "parentId": null,
      "position": 1,
      "status": "APPROVED",
      "isActive": true,
      "seoTitle": "Electronics - Shop Now",
      "seoDescription": "Browse our electronics collection",
      "seoKeywords": "electronics, gadgets, devices",
      "externalId": "ext-cat-001"
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
    "categories": [...],
    "errors": []
  }
}
```

### Import Categories from File

**Request:**
```
POST /api/v1/categories/import
Content-Type: multipart/form-data

file: <CSV or XLSX file>
skipDuplicates: true (optional)
```

**Response:**
```json
{
  "success": true,
  "data": {
    "imported": 25,
    "skipped": 3,
    "failed": 1,
    "errors": [
      {"row": 5, "field": "name", "message": "Name is required"}
    ]
  }
}
```

### Download Import Template

**Request:**
```
GET /api/v1/categories/import/template?format=csv
GET /api/v1/categories/import/template?format=xlsx
```

**Template Columns:**
| Column | Required | Type | Description |
|--------|----------|------|-------------|
| name | Yes | string | Category name |
| description | No | string | Category description |
| parent_id | No | uuid | Parent category UUID |
| position | No | integer | Sort order position |
| status | No | string | DRAFT, PENDING, APPROVED, REJECTED |
| is_active | No | boolean | true/false |
| seo_title | No | string | SEO page title |
| seo_description | No | string | SEO meta description |
| seo_keywords | No | string | Comma-separated keywords |
| external_id | No | string | External system ID |

## Database Schema

The service uses PostgreSQL with the following main tables:

- `categories` - Main category data with hierarchical structure
- `category_audit` - Audit trail for all category changes

Key features:
- Self-referencing foreign keys for hierarchy
- Automatic level calculation via triggers
- Circular reference prevention
- JSONB columns for flexible metadata
- GIN indexes for efficient querying
- Unique slug constraints per tenant

## Hierarchical Features

### Automatic Level Calculation
Categories automatically calculate their level based on parent relationships:
- Root categories: Level 0
- Child categories: Parent level + 1

### Circular Reference Prevention
Database triggers prevent:
- Self-referencing (category as its own parent)
- Circular references (A → B → A)

### Tree Operations
- **Get Tree**: Retrieve complete hierarchical structure
- **Reorder**: Change position within same level or move to different parent
- **Position Management**: Automatic position handling

## Architecture

```
categories-service/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── handlers/            # HTTP handlers
│   ├── middleware/          # Authentication & CORS
│   ├── models/              # Data models
│   └── repository/          # Database operations
├── migrations/              # Database migrations
├── Dockerfile               # Container definition
├── docker-compose.yml       # Local development setup
└── go.mod                   # Go dependencies
```

## Status Workflow

Categories follow a approval workflow:

1. **Draft** - Initial creation state
2. **Pending** - Submitted for approval
3. **Approved** - Ready for use
4. **Rejected** - Needs revision

## SEO Features

Built-in SEO support:
- **SEO Title**: Custom page title
- **SEO Description**: Meta description
- **SEO Keywords**: Searchable keywords
- **Slug**: URL-friendly identifier

## Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Test endpoints (after starting service)
curl http://localhost:8083/health
```

## Production Deployment

1. **Environment**: Set `ENVIRONMENT=production`
2. **Database**: Use managed PostgreSQL service
3. **Authentication**: Configure Azure AD properly
4. **Monitoring**: Monitor `/health` endpoint
5. **Logging**: Configure structured logging
6. **SSL**: Use HTTPS in production

## Contributing

1. Follow Go coding standards
2. Add tests for new features
3. Update documentation
4. Use conventional commit messages
5. Consider hierarchy implications for changes