# Content Service

Multi-tenant content management microservice for the Tesseract Hub platform. Manages content pages with SEO, publishing workflows, and analytics.

## Features

- **Content Management**: Full CRUD for content pages
- **Multiple Page Types**: Static, Blog, FAQ, Policy, Landing, Custom
- **Publishing Workflow**: Draft → Published → Archived
- **SEO Support**: Meta title, description, keywords, OG image
- **Menu/Footer Control**: Configurable navigation visibility
- **Analytics**: View tracking and comment counts
- **Categories & Tags**: Flexible content organization

## Tech Stack

- **Language**: Go
- **Framework**: Gin
- **Database**: PostgreSQL with GORM

## API Endpoints

### Content Pages
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/content-pages` | Create page |
| GET | `/api/v1/content-pages/:id` | Get by ID |
| GET | `/api/v1/content-pages/slug/:slug` | Get by slug |
| GET | `/api/v1/content-pages` | List with filters |
| PUT | `/api/v1/content-pages/:id` | Update page |
| DELETE | `/api/v1/content-pages/:id` | Delete page |

### Publishing
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/content-pages/:id/publish` | Publish page |
| POST | `/api/v1/content-pages/:id/unpublish` | Unpublish page |

### Navigation & Display
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/content-pages/menu-pages` | Get menu pages |
| GET | `/api/v1/content-pages/footer-pages` | Get footer pages |
| GET | `/api/v1/content-pages/featured-pages` | Get featured pages |
| GET | `/api/v1/content-pages/stats` | Get statistics |

### Utilities
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/content-pages/generate-slug` | Generate unique slug |

## Query Parameters

### Filtering
- `query` - Search term
- `type[]` - Page types filter
- `status[]` - Status filter
- `categoryId` - Category filter
- `tags[]` - Tags filter
- `authorId` - Author filter
- `isFeatured` - Featured filter
- `showInMenu` - Menu visibility
- `dateFrom`, `dateTo` - Date range

### Pagination
- `page` - Page number
- `limit` - Items per page
- `sortBy` - Sort field
- `sortOrder` - ASC or DESC

## Page Types

- **STATIC**: Standard static pages
- **BLOG**: Blog posts with author
- **FAQ**: Frequently asked questions
- **POLICY**: Policy documents (privacy, terms)
- **LANDING**: Marketing landing pages
- **CUSTOM**: Custom page types

## Page Status

- **DRAFT**: Work in progress
- **PUBLISHED**: Live and visible
- **ARCHIVED**: Hidden from public

## Data Model

### ContentPage
- UUID primary key with tenant isolation
- Type, status, title, slug (unique per tenant)
- Content and excerpt
- SEO fields: meta title, description, keywords, OG image
- Featured image with alt text
- Author info: ID and name
- Category: ID and name
- Tags (JSONB)
- Display: showInMenu, menuOrder, showInFooter, footerOrder
- Template type specification
- Analytics: viewCount, lastViewedAt
- Comments: allowComments flag
- Protection: isProtected, requiresAuth
- Custom CSS and JavaScript
- Metadata (JSONB)
- Audit: createdBy, updatedBy, timestamps

### ContentPageComment
- Threaded comments support
- Author info and status
- IP and user agent tracking

### ContentPageCategory
- Hierarchical categories
- Unique slug per tenant
- Sort order

## Environment Variables

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=content_db
DB_SSL_MODE=disable

# Server
SERVICE_PORT=8080
ENVIRONMENT=development
```

## License

MIT
