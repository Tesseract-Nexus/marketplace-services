package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/clients"
	"marketplace-connector-service/internal/encryption"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
)

// OrderImportService handles order imports from marketplaces with PII protection
type OrderImportService struct {
	db              *gorm.DB
	piiEncryptor    *encryption.PIIEncryptor
	auditService    *AuditService
	concurrency     *TenantSemaphore
}

// NewOrderImportService creates a new order import service
func NewOrderImportService(
	db *gorm.DB,
	piiEncryptor *encryption.PIIEncryptor,
	auditService *AuditService,
	concurrency *TenantSemaphore,
) *OrderImportService {
	return &OrderImportService{
		db:           db,
		piiEncryptor: piiEncryptor,
		auditService: auditService,
		concurrency:  concurrency,
	}
}

// ImportedOrder represents an order after import with encrypted PII
type ImportedOrder struct {
	ID                uuid.UUID              `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID          string                 `gorm:"type:varchar(255);not null;index" json:"tenantId"`
	VendorID          string                 `gorm:"type:varchar(255);not null;index" json:"vendorId"`
	ConnectionID      uuid.UUID              `gorm:"type:uuid;not null;index" json:"connectionId"`
	ExternalOrderID   string                 `gorm:"type:varchar(255);not null" json:"externalOrderId"`
	MarketplaceType   models.MarketplaceType `gorm:"type:varchar(50);not null" json:"marketplaceType"`
	OrderStatus       OrderStatus            `gorm:"type:varchar(50);not null" json:"orderStatus"`
	ResolutionStatus  ResolutionStatus       `gorm:"type:varchar(50);default:'PENDING'" json:"resolutionStatus"`

	// Encrypted PII fields
	CustomerPIICiphertext []byte `gorm:"type:bytea" json:"-"`
	CustomerPIINonce      []byte `gorm:"type:bytea" json:"-"`
	CustomerPIIKeyVersion int    `gorm:"default:1" json:"-"`

	// Non-PII order details
	Currency       string    `gorm:"type:varchar(3);default:'USD'" json:"currency"`
	TotalAmount    float64   `gorm:"type:decimal(12,2)" json:"totalAmount"`
	SubtotalAmount float64   `gorm:"type:decimal(12,2)" json:"subtotalAmount"`
	TaxAmount      float64   `gorm:"type:decimal(12,2)" json:"taxAmount"`
	ShippingAmount float64   `gorm:"type:decimal(12,2)" json:"shippingAmount"`
	DiscountAmount float64   `gorm:"type:decimal(12,2)" json:"discountAmount"`

	// Timestamps
	OrderDate    time.Time  `json:"orderDate"`
	ImportedAt   time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"importedAt"`
	LastSyncedAt *time.Time `json:"lastSyncedAt,omitempty"`
	CreatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Lines []ImportedOrderLine `gorm:"foreignKey:OrderID" json:"lines,omitempty"`
}

// TableName specifies the table name
func (ImportedOrder) TableName() string {
	return "marketplace_imported_orders"
}

// OrderStatus represents the order status
type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "PENDING"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusShipped    OrderStatus = "SHIPPED"
	OrderStatusDelivered  OrderStatus = "DELIVERED"
	OrderStatusCancelled  OrderStatus = "CANCELLED"
	OrderStatusRefunded   OrderStatus = "REFUNDED"
)

// ResolutionStatus represents the order resolution status
type ResolutionStatus string

const (
	ResolutionPending   ResolutionStatus = "PENDING"
	ResolutionPartial   ResolutionStatus = "PARTIAL"
	ResolutionResolved  ResolutionStatus = "RESOLVED"
	ResolutionUnmapped  ResolutionStatus = "UNMAPPED"
)

// ImportedOrderLine represents an order line item
type ImportedOrderLine struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrderID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"orderId"`
	TenantID        string     `gorm:"type:varchar(255);not null" json:"tenantId"`
	ExternalLineID  string     `gorm:"type:varchar(255)" json:"externalLineId,omitempty"`
	ExternalSKU     string     `gorm:"type:varchar(255)" json:"externalSku"`
	ExternalName    string     `gorm:"type:varchar(500)" json:"externalName"`

	// Internal mapping
	InternalVariantID *uuid.UUID `gorm:"type:uuid" json:"internalVariantId,omitempty"`
	InternalOfferID   *uuid.UUID `gorm:"type:uuid" json:"internalOfferId,omitempty"`
	MappingStatus     string     `gorm:"type:varchar(50);default:'UNRESOLVED'" json:"mappingStatus"`

	// Line details
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `gorm:"type:decimal(12,2)" json:"unitPrice"`
	TotalPrice  float64 `gorm:"type:decimal(12,2)" json:"totalPrice"`
	TaxAmount   float64 `gorm:"type:decimal(12,2)" json:"taxAmount"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name
func (ImportedOrderLine) TableName() string {
	return "marketplace_imported_order_lines"
}

// CustomerPII contains customer PII fields
type CustomerPII struct {
	Email           string `json:"email,omitempty"`
	Phone           string `json:"phone,omitempty"`
	FirstName       string `json:"firstName,omitempty"`
	LastName        string `json:"lastName,omitempty"`
	ShippingAddress string `json:"shippingAddress,omitempty"`
	BillingAddress  string `json:"billingAddress,omitempty"`
}

// ImportOrderRequest represents a request to import an order
type ImportOrderRequest struct {
	ConnectionID    uuid.UUID
	ExternalOrder   *clients.ExternalOrder
	MarketplaceType models.MarketplaceType
	EncryptPII      bool
	ResolveProducts bool
}

// orderStrPtr returns a pointer to the string, or nil if empty
func orderStrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ImportOrder imports an order from an external marketplace
func (s *OrderImportService) ImportOrder(ctx context.Context, tenantID, vendorID string, req *ImportOrderRequest) (*ImportedOrder, error) {
	ext := req.ExternalOrder

	// Determine total amounts (handle different field names from various marketplaces)
	totalAmount := ext.TotalPrice
	if totalAmount == 0 && ext.Total > 0 {
		totalAmount = ext.Total
	}
	subtotalAmount := ext.SubtotalPrice
	if subtotalAmount == 0 && ext.Subtotal > 0 {
		subtotalAmount = ext.Subtotal
	}
	taxAmount := ext.TotalTax
	if taxAmount == 0 && ext.TaxTotal > 0 {
		taxAmount = ext.TaxTotal
	}
	shippingAmount := ext.TotalShipping
	if shippingAmount == 0 && ext.ShippingTotal > 0 {
		shippingAmount = ext.ShippingTotal
	}

	// Create imported order
	order := &ImportedOrder{
		ID:              uuid.New(),
		TenantID:        tenantID,
		VendorID:        vendorID,
		ConnectionID:    req.ConnectionID,
		ExternalOrderID: ext.ID,
		MarketplaceType: req.MarketplaceType,
		OrderStatus:     mapOrderStatus(ext.Status),
		ResolutionStatus: ResolutionPending,
		Currency:        ext.Currency,
		TotalAmount:     totalAmount,
		SubtotalAmount:  subtotalAmount,
		TaxAmount:       taxAmount,
		ShippingAmount:  shippingAmount,
		OrderDate:       ext.CreatedAt,
		ImportedAt:      time.Now(),
	}

	// Encrypt PII if enabled
	if req.EncryptPII && s.piiEncryptor != nil {
		// Build PII from order and customer data
		email := ext.Email
		phone := ext.Phone
		var firstName, lastName *string
		var address1, address2, city, state, postalCode, country *string

		if ext.Customer != nil {
			if ext.Customer.Email != "" {
				email = ext.Customer.Email
			}
			if ext.Customer.Phone != "" {
				phone = ext.Customer.Phone
			}
			if ext.Customer.FirstName != "" {
				firstName = &ext.Customer.FirstName
			}
			if ext.Customer.LastName != "" {
				lastName = &ext.Customer.LastName
			}
		}

		if ext.ShippingAddress != nil {
			if ext.ShippingAddress.Address1 != "" {
				address1 = &ext.ShippingAddress.Address1
			}
			if ext.ShippingAddress.Address2 != "" {
				address2 = &ext.ShippingAddress.Address2
			}
			if ext.ShippingAddress.City != "" {
				city = &ext.ShippingAddress.City
			}
			if ext.ShippingAddress.Province != "" {
				state = &ext.ShippingAddress.Province
			}
			if ext.ShippingAddress.Zip != "" {
				postalCode = &ext.ShippingAddress.Zip
			}
			if ext.ShippingAddress.Country != "" {
				country = &ext.ShippingAddress.Country
			}
		}

		pii := &encryption.PIIFields{
			Email:      orderStrPtr(email),
			Phone:      orderStrPtr(phone),
			FirstName:  firstName,
			LastName:   lastName,
			Address1:   address1,
			Address2:   address2,
			City:       city,
			State:      state,
			PostalCode: postalCode,
			Country:    country,
		}

		encrypted, err := s.piiEncryptor.EncryptPII(ctx, tenantID, pii)
		if err != nil {
			// Log encryption failure
			if s.auditService != nil {
				auditLog := models.NewAuditLog(tenantID, models.ActionOrderImport, models.ResourceOrder).
					WithActor(models.ActorSystem, "order-import", nil).
					WithResource(order.ID.String()).
					WithMetadata(models.JSONB{
						"operation":    "ENCRYPT",
						"success":      false,
						"error":        err.Error(),
						"customer_id":  ext.Customer.ID,
						"algorithm":    "AES-256-GCM",
					}).
					WithPIIAccess([]string{"email", "phone", "name", "address"}).
					Build()
				_ = s.auditService.LogAction(ctx, auditLog)
			}
			return nil, fmt.Errorf("failed to encrypt customer PII: %w", err)
		}

		// Decode base64 strings to []byte for storage
		ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
		}
		nonce, err := base64.StdEncoding.DecodeString(encrypted.Nonce)
		if err != nil {
			return nil, fmt.Errorf("failed to decode nonce: %w", err)
		}

		order.CustomerPIICiphertext = ciphertext
		order.CustomerPIINonce = nonce
		order.CustomerPIIKeyVersion = encrypted.KeyVersion

		// Log PII encryption success + PII access for audit
		if s.auditService != nil {
			customerID := ""
			if ext.Customer != nil {
				customerID = ext.Customer.ID
			}
			auditLog := models.NewAuditLog(tenantID, models.ActionOrderImport, models.ResourceOrder).
				WithActor(models.ActorSystem, "order-import", nil).
				WithResource(order.ID.String()).
				WithMetadata(models.JSONB{
					"operation":     "ENCRYPT",
					"success":       true,
					"customer_id":   customerID,
					"algorithm":     "AES-256-GCM",
					"key_version":   encrypted.KeyVersion,
					"fields_count":  len([]string{"email", "phone", "name", "address"}),
				}).
				WithPIIAccess([]string{"email", "phone", "name", "address"}).
				Build()
			_ = s.auditService.LogAction(ctx, auditLog)
		}
	}

	// Import order lines
	for _, extLine := range ext.LineItems {
		line := ImportedOrderLine{
			ID:             uuid.New(),
			OrderID:        order.ID,
			TenantID:       tenantID,
			ExternalLineID: extLine.ID,
			ExternalSKU:    extLine.SKU,
			ExternalName:   extLine.Title,
			Quantity:       extLine.Quantity,
			UnitPrice:      extLine.Price,
			TotalPrice:     extLine.TotalPrice,
			MappingStatus:  "UNRESOLVED",
		}
		order.Lines = append(order.Lines, line)
	}

	// Save to database
	if err := s.db.WithContext(ctx).Create(order).Error; err != nil {
		return nil, fmt.Errorf("failed to save imported order: %w", err)
	}

	// Attempt to resolve product mappings if requested
	if req.ResolveProducts {
		go s.resolveOrderLines(context.Background(), order)
	}

	return order, nil
}

// BatchImportOrders imports multiple orders efficiently
func (s *OrderImportService) BatchImportOrders(ctx context.Context, tenantID, vendorID string, connectionID uuid.UUID, orders []*clients.ExternalOrder) (*BatchImportResult, error) {
	result := &BatchImportResult{
		TotalOrders:   len(orders),
		ImportedAt:    time.Now(),
	}

	// Process in batches
	batchSize := 100
	for i := 0; i < len(orders); i += batchSize {
		end := i + batchSize
		if end > len(orders) {
			end = len(orders)
		}
		batch := orders[i:end]

		for _, extOrder := range batch {
			req := &ImportOrderRequest{
				ConnectionID:    connectionID,
				ExternalOrder:   extOrder,
				EncryptPII:      true,
				ResolveProducts: false, // Resolve in bulk after
			}

			_, err := s.ImportOrder(ctx, tenantID, vendorID, req)
			if err != nil {
				result.FailedOrders++
				result.Errors = append(result.Errors, ImportError{
					ExternalOrderID: extOrder.ID,
					Error:           err.Error(),
				})
			} else {
				result.SuccessfulOrders++
			}
		}
	}

	return result, nil
}

// BatchImportResult contains the result of a batch import
type BatchImportResult struct {
	TotalOrders      int           `json:"totalOrders"`
	SuccessfulOrders int           `json:"successfulOrders"`
	FailedOrders     int           `json:"failedOrders"`
	Errors           []ImportError `json:"errors,omitempty"`
	ImportedAt       time.Time     `json:"importedAt"`
}

// ImportError represents an import error
type ImportError struct {
	ExternalOrderID string `json:"externalOrderId"`
	Error           string `json:"error"`
}

// GetOrder retrieves an order with decrypted PII (requires audit)
func (s *OrderImportService) GetOrder(ctx context.Context, tenantID string, orderID uuid.UUID, includePII bool, actorID string) (*ImportedOrder, *CustomerPII, error) {
	var order ImportedOrder
	if err := s.db.WithContext(ctx).Preload("Lines").First(&order, "id = ? AND tenant_id = ?", orderID, tenantID).Error; err != nil {
		return nil, nil, err
	}

	var pii *CustomerPII
	if includePII && s.piiEncryptor != nil && len(order.CustomerPIICiphertext) > 0 {
		// Log PII access
		if s.auditService != nil {
			auditLog := models.NewAuditLog(tenantID, models.ActionPIIAccess, models.ResourceOrder).
				WithActor(models.ActorUser, actorID, nil).
				WithResource(orderID.String()).
				WithPIIAccess([]string{"email", "phone", "name", "address"}).
				Build()
			_ = s.auditService.LogAction(ctx, auditLog)
		}

		// Decrypt PII - encode []byte to base64 strings for EncryptedData
		encrypted := &encryption.EncryptedData{
			Ciphertext: base64.StdEncoding.EncodeToString(order.CustomerPIICiphertext),
			Nonce:      base64.StdEncoding.EncodeToString(order.CustomerPIINonce),
			KeyVersion: order.CustomerPIIKeyVersion,
		}

		decrypted, err := s.piiEncryptor.DecryptPII(ctx, tenantID, encrypted)
		if err != nil {
			// Log decryption failure
			if s.auditService != nil {
				failLog := models.NewAuditLog(tenantID, models.ActionPIIAccess, models.ResourceOrder).
					WithActor(models.ActorUser, actorID, nil).
					WithResource(orderID.String()).
					WithMetadata(models.JSONB{
						"operation":   "DECRYPT",
						"success":     false,
						"error":       err.Error(),
						"key_version": order.CustomerPIIKeyVersion,
					}).
					Build()
				_ = s.auditService.LogAction(ctx, failLog)
			}
			return &order, nil, fmt.Errorf("failed to decrypt PII: %w", err)
		}

		// Log decryption success
		if s.auditService != nil {
			decryptLog := models.NewAuditLog(tenantID, models.ActionPIIAccess, models.ResourceOrder).
				WithActor(models.ActorUser, actorID, nil).
				WithResource(orderID.String()).
				WithMetadata(models.JSONB{
					"operation":   "DECRYPT",
					"success":     true,
					"key_version": order.CustomerPIIKeyVersion,
				}).
				Build()
			_ = s.auditService.LogAction(ctx, decryptLog)
		}

		pii = &CustomerPII{}
		if decrypted.Email != nil {
			pii.Email = *decrypted.Email
		}
		if decrypted.Phone != nil {
			pii.Phone = *decrypted.Phone
		}
		if decrypted.FirstName != nil {
			pii.FirstName = *decrypted.FirstName
		}
		if decrypted.LastName != nil {
			pii.LastName = *decrypted.LastName
		}
		// Build shipping address from components
		if decrypted.Address1 != nil {
			pii.ShippingAddress = *decrypted.Address1
			if decrypted.Address2 != nil && *decrypted.Address2 != "" {
				pii.ShippingAddress += ", " + *decrypted.Address2
			}
			if decrypted.City != nil {
				pii.ShippingAddress += ", " + *decrypted.City
			}
			if decrypted.State != nil {
				pii.ShippingAddress += ", " + *decrypted.State
			}
			if decrypted.PostalCode != nil {
				pii.ShippingAddress += " " + *decrypted.PostalCode
			}
			if decrypted.Country != nil {
				pii.ShippingAddress += ", " + *decrypted.Country
			}
		}
	}

	return &order, pii, nil
}

// resolveOrderLines attempts to map order lines to internal products
func (s *OrderImportService) resolveOrderLines(ctx context.Context, order *ImportedOrder) {
	for i := range order.Lines {
		line := &order.Lines[i]

		// Try to find variant by SKU
		var variant models.CatalogVariant
		err := s.db.WithContext(ctx).
			Where("tenant_id = ? AND sku = ?", order.TenantID, line.ExternalSKU).
			First(&variant).Error

		if err == nil {
			line.InternalVariantID = &variant.ID
			line.MappingStatus = "RESOLVED"

			// Find offer for this vendor
			var offer models.Offer
			if err := s.db.WithContext(ctx).
				Where("tenant_id = ? AND vendor_id = ? AND catalog_variant_id = ?",
					order.TenantID, order.VendorID, variant.ID).
				First(&offer).Error; err == nil {
				line.InternalOfferID = &offer.ID
			}
		} else {
			line.MappingStatus = "UNRESOLVED"
		}

		// Update line
		s.db.WithContext(ctx).Save(line)
	}

	// Update order resolution status
	resolved := 0
	for _, line := range order.Lines {
		if line.MappingStatus == "RESOLVED" {
			resolved++
		}
	}

	if resolved == len(order.Lines) {
		order.ResolutionStatus = ResolutionResolved
	} else if resolved > 0 {
		order.ResolutionStatus = ResolutionPartial
	} else {
		order.ResolutionStatus = ResolutionUnmapped
	}
	s.db.WithContext(ctx).Save(order)
}

// mapOrderStatus maps external order status to internal status
func mapOrderStatus(external string) OrderStatus {
	switch external {
	case "pending", "Pending", "PENDING":
		return OrderStatusPending
	case "processing", "Processing", "PROCESSING", "Unshipped":
		return OrderStatusProcessing
	case "shipped", "Shipped", "SHIPPED":
		return OrderStatusShipped
	case "delivered", "Delivered", "DELIVERED":
		return OrderStatusDelivered
	case "cancelled", "Cancelled", "CANCELLED", "Canceled":
		return OrderStatusCancelled
	case "refunded", "Refunded", "REFUNDED":
		return OrderStatusRefunded
	default:
		return OrderStatusPending
	}
}

// ListOrders lists orders for a tenant with pagination
func (s *OrderImportService) ListOrders(ctx context.Context, tenantID string, limit, offset int) ([]ImportedOrder, int64, error) {
	var orders []ImportedOrder
	var total int64

	query := s.db.WithContext(ctx).Model(&ImportedOrder{}).Where("tenant_id = ?", tenantID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	if err := query.Order("order_date DESC").Preload("Lines").Find(&orders).Error; err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

// GetUnresolvedOrders gets orders with unresolved line items
func (s *OrderImportService) GetUnresolvedOrders(ctx context.Context, tenantID string) ([]ImportedOrder, error) {
	var orders []ImportedOrder
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND resolution_status IN ?", tenantID, []ResolutionStatus{ResolutionPending, ResolutionPartial, ResolutionUnmapped}).
		Preload("Lines").
		Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}
