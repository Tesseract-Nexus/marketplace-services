# Tax Service

Complete tax calculation and configuration service for global ecommerce platform with support for India GST, EU VAT, and international tax systems.

## Overview

The Tax Service handles:
- **Tax Rate Management**: Jurisdiction-based tax rates (country, state, city, zip)
- **Tax Calculation**: Real-time tax calculations for orders
- **India GST**: Full support for CGST, SGST, IGST, UTGST, and Cess
- **EU VAT**: VAT calculation with B2B reverse-charge mechanism
- **Canada Tax**: HST, PST, QST with compound tax support
- **Product Categories**: Different tax treatment for product types (HSN/SAC codes)
- **Tax Exemptions**: Customer exemption certificates (wholesale, resale, non-profit)
- **Tax Nexus**: Manage where business has tax collection obligations
- **Tax Reporting**: Compliance reports for tax filing

## Features

### Core Features
- [x] Multi-jurisdiction tax rates (country -> state -> city -> zip)
- [x] Compound tax calculation (state + local + special)
- [x] Product category-based taxation
- [x] Tax exemption certificates
- [x] Tax nexus management
- [x] Effective date handling
- [x] Tax calculation caching

### India GST Support
- [x] CGST (Central GST) - Intrastate transactions
- [x] SGST (State GST) - Intrastate transactions
- [x] IGST (Integrated GST) - Interstate transactions
- [x] UTGST (Union Territory GST) - UT transactions
- [x] GST Cess - Luxury goods additional tax
- [x] HSN Codes - Harmonized System of Nomenclature for goods
- [x] SAC Codes - Services Accounting Code for services
- [x] GST Slabs - 0%, 5%, 12%, 18%, 28%
- [x] B2B GSTIN validation
- [x] Interstate vs Intrastate detection

### EU VAT Support
- [x] Standard VAT calculation
- [x] B2B Reverse-charge mechanism
- [x] VAT number validation
- [x] Cross-border transactions

### Canada Tax Support
- [x] GST (Federal)
- [x] HST (Harmonized - Ontario, etc.)
- [x] PST (Provincial - BC, Saskatchewan, Manitoba)
- [x] QST (Quebec) - Compound tax on subtotal + GST

### Global Support
- [x] 20+ countries pre-configured
- [x] All Indian states with GST codes
- [x] State/Province code support
- [x] ISO country codes

## Database Schema

### Tax Jurisdictions
Hierarchical structure with state codes:
- Country -> State -> County -> City -> ZIP
- State codes for India GST (27=MH, 29=KA, etc.)
- ISO country codes (IN, US, GB, etc.)

### Tax Rates
- Support for multiple rates per jurisdiction
- Tax types: SALES, VAT, GST, CGST, SGST, IGST, UTGST, CESS, HST, PST, QST
- Compound tax calculation flag
- Effective date ranges
- Applies to products/shipping flags

### Product Tax Categories
- HSN codes for India goods
- SAC codes for India services
- GST slab assignment
- External tax code mapping

### Tax Nexus
- GSTIN for India
- VAT number for EU
- Composition scheme flag
- Country/state nexus tracking

## API Endpoints

### Tax Calculation
```
POST   /api/v1/tax/calculate           Calculate tax for order
POST   /api/v1/tax/validate-address    Validate & standardize address
```

### Tax Rates Management
```
GET    /api/v1/tax/rates               List tax rates
POST   /api/v1/tax/rates               Create tax rate
GET    /api/v1/tax/rates/:id           Get tax rate
PUT    /api/v1/tax/rates/:id           Update tax rate
DELETE /api/v1/tax/rates/:id           Delete tax rate
```

### Jurisdictions
```
GET    /api/v1/tax/jurisdictions       List jurisdictions
POST   /api/v1/tax/jurisdictions       Create jurisdiction
GET    /api/v1/tax/jurisdictions/:id   Get jurisdiction
```

### Product Categories
```
GET    /api/v1/tax/categories          List product tax categories
POST   /api/v1/tax/categories          Create category
PUT    /api/v1/tax/categories/:id      Update category
```

### Exemption Certificates
```
GET    /api/v1/tax/exemptions          List exemption certificates
POST   /api/v1/tax/exemptions          Create exemption
GET    /api/v1/tax/exemptions/:id      Get exemption
PUT    /api/v1/tax/exemptions/:id      Update exemption status
```

### Tax Nexus
```
GET    /api/v1/tax/nexus               List tax nexus locations
POST   /api/v1/tax/nexus               Add nexus
DELETE /api/v1/tax/nexus/:id           Remove nexus
```

## Tax Calculation Examples

### India GST - Intrastate (Same State)
```bash
curl -X POST http://localhost:8091/api/v1/tax/calculate \
  -H "Content-Type: application/json" \
  -d '{
    "tenantId": "test-tenant",
    "originAddress": {
      "stateCode": "27",
      "countryCode": "IN"
    },
    "shippingAddress": {
      "city": "Mumbai",
      "state": "Maharashtra",
      "stateCode": "27",
      "countryCode": "IN"
    },
    "lineItems": [
      {
        "productId": "prod-123",
        "hsnCode": "8471",
        "isService": false,
        "quantity": 1,
        "unitPrice": 10000.00,
        "subtotal": 10000.00
      }
    ],
    "isB2B": false
  }'
```

Response (Intrastate - CGST + SGST):
```json
{
  "subtotal": 10000.00,
  "taxAmount": 1800.00,
  "total": 11800.00,
  "taxBreakdown": [
    {
      "jurisdictionName": "India - Central",
      "taxType": "CGST",
      "rate": 0.09,
      "taxableAmount": 10000.00,
      "taxAmount": 900.00
    },
    {
      "jurisdictionName": "Maharashtra",
      "taxType": "SGST",
      "rate": 0.09,
      "taxableAmount": 10000.00,
      "taxAmount": 900.00
    }
  ],
  "gstSummary": {
    "cgst": 900.00,
    "sgst": 900.00,
    "igst": 0,
    "utgst": 0,
    "cess": 0,
    "totalGst": 1800.00,
    "isInterstate": false
  }
}
```

### India GST - Interstate (Different States)
```bash
curl -X POST http://localhost:8091/api/v1/tax/calculate \
  -H "Content-Type: application/json" \
  -d '{
    "tenantId": "test-tenant",
    "originAddress": {
      "stateCode": "27",
      "countryCode": "IN"
    },
    "shippingAddress": {
      "city": "Bangalore",
      "state": "Karnataka",
      "stateCode": "29",
      "countryCode": "IN"
    },
    "lineItems": [
      {
        "productId": "prod-123",
        "hsnCode": "8471",
        "quantity": 1,
        "unitPrice": 10000.00,
        "subtotal": 10000.00
      }
    ]
  }'
```

Response (Interstate - IGST):
```json
{
  "subtotal": 10000.00,
  "taxAmount": 1800.00,
  "total": 11800.00,
  "taxBreakdown": [
    {
      "jurisdictionName": "India",
      "taxType": "IGST",
      "rate": 0.18,
      "taxableAmount": 10000.00,
      "taxAmount": 1800.00
    }
  ],
  "gstSummary": {
    "cgst": 0,
    "sgst": 0,
    "igst": 1800.00,
    "utgst": 0,
    "cess": 0,
    "totalGst": 1800.00,
    "isInterstate": true
  }
}
```

### EU VAT with B2B Reverse-Charge
```bash
curl -X POST http://localhost:8091/api/v1/tax/calculate \
  -H "Content-Type: application/json" \
  -d '{
    "tenantId": "test-tenant",
    "originAddress": {
      "countryCode": "DE"
    },
    "shippingAddress": {
      "city": "Paris",
      "countryCode": "FR"
    },
    "lineItems": [
      {
        "productId": "prod-456",
        "quantity": 1,
        "unitPrice": 1000.00,
        "subtotal": 1000.00
      }
    ],
    "isB2B": true,
    "customerVatNumber": "FR12345678901"
  }'
```

Response (B2B Reverse-Charge - No VAT):
```json
{
  "subtotal": 1000.00,
  "taxAmount": 0,
  "total": 1000.00,
  "taxBreakdown": [],
  "vatSummary": {
    "vatAmount": 0,
    "isReverseCharge": true,
    "customerVatNumber": "FR12345678901"
  }
}
```

### Canada Quebec - Compound Tax (GST + QST)
```bash
curl -X POST http://localhost:8091/api/v1/tax/calculate \
  -H "Content-Type: application/json" \
  -d '{
    "tenantId": "test-tenant",
    "shippingAddress": {
      "city": "Montreal",
      "state": "Quebec",
      "stateCode": "QC",
      "countryCode": "CA"
    },
    "lineItems": [
      {
        "productId": "prod-789",
        "quantity": 1,
        "unitPrice": 100.00,
        "subtotal": 100.00
      }
    ]
  }'
```

Response (Quebec - GST + QST compound):
```json
{
  "subtotal": 100.00,
  "taxAmount": 14.98,
  "total": 114.98,
  "taxBreakdown": [
    {
      "jurisdictionName": "Canada",
      "taxType": "GST",
      "rate": 0.05,
      "taxableAmount": 100.00,
      "taxAmount": 5.00
    },
    {
      "jurisdictionName": "Quebec",
      "taxType": "QST",
      "rate": 0.09975,
      "taxableAmount": 105.00,
      "taxAmount": 9.98,
      "isCompound": true
    }
  ]
}
```

## India GST State Codes

| State | Code | State | Code |
|-------|------|-------|------|
| Andhra Pradesh | 37 | Maharashtra | 27 |
| Karnataka | 29 | Tamil Nadu | 33 |
| Kerala | 32 | Telangana | 36 |
| Gujarat | 24 | Rajasthan | 08 |
| Delhi | 07 | West Bengal | 19 |
| Uttar Pradesh | 09 | Punjab | 03 |

## Configuration

### Environment Variables
```bash
PORT=8091
DATABASE_URL=postgresql://user:pass@localhost:5432/tesseract_hub
ENVIRONMENT=development
LOG_LEVEL=info
CACHE_TTL_MINUTES=60
```

### Health Endpoints
```
GET /livez   - Liveness probe
GET /readyz  - Readiness probe (includes DB check)
```

## Development

### Run Migrations
```bash
# Create tables
psql -h localhost -U dev -d tesseract_hub -f migrations/001_create_tax_tables.sql
psql -h localhost -U dev -d tesseract_hub -f migrations/002_seed_tax_data.sql
psql -h localhost -U dev -d tesseract_hub -f migrations/003_add_gst_and_compound_tax.sql
psql -h localhost -U dev -d tesseract_hub -f migrations/004_seed_global_tax_data.sql
```

### Run Service
```bash
cd services/tax-service
go run cmd/main.go
```

### Build
```bash
go build -o tax-service ./cmd/main.go
```

### Docker
```bash
docker build -f services/tax-service/Dockerfile -t tax-service .
docker run -p 8091:8091 tax-service
```

## Deployment

### Kubernetes
The service is deployed via ArgoCD with Helm charts:
- Helm chart: `tesserix-k8s/charts/apps/tax-service/`
- ArgoCD app: `tesserix-k8s/argocd/devtest/apps/tax-service.yaml`

### Istio Routes
- `/api/v1/tax` - Tax calculation
- `/api/v1/tax-rates` - Rate management
- `/api/v1/tax-jurisdictions` - Jurisdictions
- `/api/v1/tax-exemptions` - Exemptions
- `/api/v1/tax-nexus` - Nexus configuration

## Compliance Notes

### India GST
- **GSTIN Format**: 2 digits state + 10 digits PAN + 1 digit entity + 1 check digit
- **HSN Codes**: Required for goods with turnover > 5 crore
- **SAC Codes**: Required for services
- **Invoice Requirements**: CGST/SGST or IGST based on transaction type

### EU VAT
- **VIES Validation**: Validate customer VAT numbers
- **Reverse-Charge**: B2B cross-border within EU
- **Place of Supply**: Destination-based for B2C

### General
- **Tax rates must be kept current** - Review quarterly
- **Exemption certificates** - Keep on file for audit trail
- **Tax nexus** - Update when expanding to new regions
- **Reports** - Generate monthly/quarterly for filing

## License

Proprietary - Tesseract Hub Platform
