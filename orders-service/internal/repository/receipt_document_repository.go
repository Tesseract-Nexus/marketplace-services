package repository

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"orders-service/internal/models"
)

// ReceiptDocumentRepository handles receipt document data persistence
type ReceiptDocumentRepository struct {
	db *gorm.DB
}

// NewReceiptDocumentRepository creates a new receipt document repository
func NewReceiptDocumentRepository(db *gorm.DB) *ReceiptDocumentRepository {
	return &ReceiptDocumentRepository{db: db}
}

// Create creates a new receipt document record
// Retries short code generation on unique constraint collision (up to 3 attempts)
func (r *ReceiptDocumentRepository) Create(doc *models.ReceiptDocument) error {
	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Generate short code if not provided or on retry
		if doc.ShortCode == "" || attempt > 0 {
			shortCode, err := generateShortCode()
			if err != nil {
				return fmt.Errorf("failed to generate short code: %w", err)
			}
			doc.ShortCode = shortCode
		}

		err := r.db.Create(doc).Error
		if err == nil {
			return nil
		}

		// Check if it's a unique constraint violation on short_code
		if attempt < maxRetries-1 && isUniqueViolation(err, "short_code") {
			continue // Retry with a new short code
		}
		return fmt.Errorf("failed to create receipt document: %w", err)
	}
	return fmt.Errorf("failed to create receipt document: exhausted short code generation retries")
}

// isUniqueViolation checks if the error is a unique constraint violation
func isUniqueViolation(err error, column string) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// PostgreSQL unique violation error code 23505
	return (strings.Contains(errStr, "23505") || strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "unique constraint")) &&
		strings.Contains(errStr, column)
}

// GetByID retrieves a receipt document by ID
func (r *ReceiptDocumentRepository) GetByID(id uuid.UUID, tenantID string) (*models.ReceiptDocument, error) {
	var doc models.ReceiptDocument
	err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get receipt document: %w", err)
	}
	return &doc, nil
}

// GetByOrderID retrieves all receipt documents for an order
func (r *ReceiptDocumentRepository) GetByOrderID(orderID uuid.UUID, tenantID string) ([]models.ReceiptDocument, error) {
	var docs []models.ReceiptDocument
	err := r.db.Where("order_id = ? AND tenant_id = ?", orderID, tenantID).
		Order("created_at DESC").
		Find(&docs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt documents: %w", err)
	}
	return docs, nil
}

// GetByShortCode retrieves a receipt document by its short code
// This is used for the public download endpoint
func (r *ReceiptDocumentRepository) GetByShortCode(shortCode string) (*models.ReceiptDocument, error) {
	var doc models.ReceiptDocument
	err := r.db.Where("short_code = ?", shortCode).First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get receipt document by short code: %w", err)
	}
	return &doc, nil
}

// GetByReceiptNumber retrieves a receipt document by receipt number
func (r *ReceiptDocumentRepository) GetByReceiptNumber(receiptNumber, tenantID string) (*models.ReceiptDocument, error) {
	var doc models.ReceiptDocument
	err := r.db.Where("receipt_number = ? AND tenant_id = ?", receiptNumber, tenantID).First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get receipt document: %w", err)
	}
	return &doc, nil
}

// GetLatestByOrderID retrieves the most recent receipt document for an order
func (r *ReceiptDocumentRepository) GetLatestByOrderID(orderID uuid.UUID, tenantID string) (*models.ReceiptDocument, error) {
	var doc models.ReceiptDocument
	err := r.db.Where("order_id = ? AND tenant_id = ?", orderID, tenantID).
		Order("created_at DESC").
		First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest receipt document: %w", err)
	}
	return &doc, nil
}

// Update updates a receipt document
func (r *ReceiptDocumentRepository) Update(doc *models.ReceiptDocument) error {
	if err := r.db.Save(doc).Error; err != nil {
		return fmt.Errorf("failed to update receipt document: %w", err)
	}
	return nil
}

// IncrementAccessCount increments the access count and updates last access time
// This is used to track downloads for audit purposes
func (r *ReceiptDocumentRepository) IncrementAccessCount(id uuid.UUID) error {
	now := time.Now()
	err := r.db.Model(&models.ReceiptDocument{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"access_count": gorm.Expr("access_count + 1"),
			"last_access":  now,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to increment access count: %w", err)
	}
	return nil
}

// UpdateEmailSent updates the email delivery tracking fields
func (r *ReceiptDocumentRepository) UpdateEmailSent(id uuid.UUID, email string) error {
	now := time.Now()
	err := r.db.Model(&models.ReceiptDocument{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"email_sent_at": now,
			"email_sent_to": email,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to update email sent: %w", err)
	}
	return nil
}

// Delete soft-deletes a receipt document
func (r *ReceiptDocumentRepository) Delete(id uuid.UUID, tenantID string) error {
	err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&models.ReceiptDocument{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete receipt document: %w", err)
	}
	return nil
}

// ListByTenant lists receipt documents for a tenant with pagination
func (r *ReceiptDocumentRepository) ListByTenant(tenantID string, page, limit int) ([]models.ReceiptDocument, int64, error) {
	var docs []models.ReceiptDocument
	var total int64

	// Count total
	if err := r.db.Model(&models.ReceiptDocument{}).Where("tenant_id = ?", tenantID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count receipt documents: %w", err)
	}

	// Get paginated results
	offset := (page - 1) * limit
	err := r.db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&docs).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list receipt documents: %w", err)
	}

	return docs, total, nil
}

// GetExpiredDocuments retrieves documents with expired short URLs
// Used for cleanup or regeneration
func (r *ReceiptDocumentRepository) GetExpiredDocuments(tenantID string) ([]models.ReceiptDocument, error) {
	var docs []models.ReceiptDocument
	now := time.Now()
	err := r.db.Where("tenant_id = ? AND expires_at IS NOT NULL AND expires_at < ?", tenantID, now).
		Find(&docs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get expired documents: %w", err)
	}
	return docs, nil
}

// generateShortCode generates a URL-safe short code for receipt access
// Uses crypto/rand for security
func generateShortCode() (string, error) {
	// Generate 9 random bytes = 12 base64 chars (72 bits of entropy)
	// This gives us ~4.7 sextillion combinations, sufficient for uniqueness
	bytes := make([]byte, 9)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Use URL-safe base64 encoding
	code := base64.RawURLEncoding.EncodeToString(bytes)
	return code, nil
}

// GenerateReceiptNumber generates a unique receipt number
// Format: RCP-{YYYYMMDD}-{sequence}
func GenerateReceiptNumber(orderNumber string) string {
	// Extract timestamp part from order number if available
	// OrderNumber format: ORD-{unix_timestamp}
	dateStr := time.Now().Format("20060102")

	// Use last 6 chars of order number for uniqueness
	suffix := orderNumber
	if len(orderNumber) > 6 {
		suffix = orderNumber[len(orderNumber)-6:]
	}

	return fmt.Sprintf("RCP-%s-%s", dateStr, suffix)
}

// GenerateInvoiceNumber generates a unique invoice number
// Format: INV-{YYYYMMDD}-{sequence}
func GenerateInvoiceNumber(orderNumber string) string {
	dateStr := time.Now().Format("20060102")

	suffix := orderNumber
	if len(orderNumber) > 6 {
		suffix = orderNumber[len(orderNumber)-6:]
	}

	return fmt.Sprintf("INV-%s-%s", dateStr, suffix)
}
