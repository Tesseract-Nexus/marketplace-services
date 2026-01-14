package repository

import (
	"time"

	"github.com/google/uuid"
	"staff-service/internal/models"
	"gorm.io/gorm"
)

// ============================================================================
// DOCUMENT REPOSITORY INTERFACE
// ============================================================================

type DocumentRepository interface {
	// Staff Documents
	CreateDocument(tenantID string, vendorID *string, doc *models.StaffDocument) error
	GetDocumentByID(tenantID string, vendorID *string, id uuid.UUID) (*models.StaffDocument, error)
	UpdateDocument(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateDocumentRequest) error
	DeleteDocument(tenantID string, vendorID *string, id uuid.UUID, deletedBy string) error
	ListDocumentsByStaff(tenantID string, vendorID *string, staffID uuid.UUID) ([]models.StaffDocument, error)
	VerifyDocument(tenantID string, vendorID *string, id uuid.UUID, req *models.VerifyDocumentRequest, verifiedBy uuid.UUID) error

	// Document Queries
	GetPendingDocuments(tenantID string, vendorID *string, page, limit int) ([]models.StaffDocument, *models.PaginationInfo, int, error)
	GetExpiringDocuments(tenantID string, vendorID *string, daysAhead int, page, limit int) ([]models.StaffDocument, *models.PaginationInfo, int, error)
	GetExpiredDocuments(tenantID string, vendorID *string, page, limit int) ([]models.StaffDocument, *models.PaginationInfo, int, error)
	GetStaffComplianceStatus(tenantID string, vendorID *string, staffID uuid.UUID) (*models.StaffComplianceStatus, error)

	// Emergency Contacts
	CreateEmergencyContact(tenantID string, vendorID *string, contact *models.EmergencyContact) error
	GetEmergencyContactByID(tenantID string, vendorID *string, id uuid.UUID) (*models.EmergencyContact, error)
	UpdateEmergencyContact(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateEmergencyContactRequest) error
	DeleteEmergencyContact(tenantID string, vendorID *string, id uuid.UUID) error
	ListEmergencyContactsByStaff(tenantID string, vendorID *string, staffID uuid.UUID) ([]models.EmergencyContact, error)
	SetPrimaryEmergencyContact(tenantID string, vendorID *string, staffID, contactID uuid.UUID) error

	// Batch Operations
	UpdateExpiredDocumentStatus(tenantID string, vendorID *string) (int64, error)
	SendExpiryReminders(tenantID string, vendorID *string, daysAhead int) ([]models.StaffDocument, error)
}

type documentRepository struct {
	db *gorm.DB
}

func NewDocumentRepository(db *gorm.DB) DocumentRepository {
	return &documentRepository{db: db}
}

// ============================================================================
// STAFF DOCUMENTS
// ============================================================================

func (r *documentRepository) CreateDocument(tenantID string, vendorID *string, doc *models.StaffDocument) error {
	doc.TenantID = tenantID
	doc.VendorID = vendorID
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = time.Now()
	return r.db.Create(doc).Error
}

func (r *documentRepository) GetDocumentByID(tenantID string, vendorID *string, id uuid.UUID) (*models.StaffDocument, error) {
	var doc models.StaffDocument
	query := r.db.Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)

	err := query.
		Preload("Staff").
		Preload("VerifiedByStaff").
		First(&doc).Error

	if err != nil {
		return nil, err
	}

	// Calculate expiry info
	r.enrichDocumentWithExpiryInfo(&doc)

	return &doc, nil
}

func (r *documentRepository) UpdateDocument(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateDocumentRequest) error {
	query := r.db.Model(&models.StaffDocument{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(updates).Error
}

func (r *documentRepository) DeleteDocument(tenantID string, vendorID *string, id uuid.UUID, deletedBy string) error {
	query := r.db.Model(&models.StaffDocument{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(map[string]interface{}{
		"deleted_at": time.Now(),
		"updated_by": deletedBy,
	}).Error
}

func (r *documentRepository) ListDocumentsByStaff(tenantID string, vendorID *string, staffID uuid.UUID) ([]models.StaffDocument, error) {
	var docs []models.StaffDocument

	query := r.db.Where("tenant_id = ? AND staff_id = ?", tenantID, staffID)
	query = r.applyVendorFilter(query, vendorID)

	err := query.
		Preload("VerifiedByStaff").
		Order("document_type, created_at DESC").
		Find(&docs).Error

	if err != nil {
		return nil, err
	}

	// Enrich with expiry info
	for i := range docs {
		r.enrichDocumentWithExpiryInfo(&docs[i])
	}

	return docs, nil
}

func (r *documentRepository) VerifyDocument(tenantID string, vendorID *string, id uuid.UUID, req *models.VerifyDocumentRequest, verifiedBy uuid.UUID) error {
	updates := map[string]interface{}{
		"verification_status": req.Status,
		"verified_at":         time.Now(),
		"verified_by":         verifiedBy,
		"updated_at":          time.Now(),
	}

	if req.Notes != nil {
		updates["verification_notes"] = *req.Notes
	}

	if req.Status == models.VerificationRejected && req.RejectionReason != nil {
		updates["rejection_reason"] = *req.RejectionReason
	}

	query := r.db.Model(&models.StaffDocument{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(updates).Error
}

// ============================================================================
// DOCUMENT QUERIES
// ============================================================================

func (r *documentRepository) GetPendingDocuments(tenantID string, vendorID *string, page, limit int) ([]models.StaffDocument, *models.PaginationInfo, int, error) {
	var docs []models.StaffDocument
	var total int64

	query := r.db.Model(&models.StaffDocument{}).
		Where("tenant_id = ? AND verification_status IN ?", tenantID,
			[]models.DocumentVerificationStatus{models.VerificationPending, models.VerificationUnderReview})
	query = r.applyVendorFilter(query, vendorID)

	if err := query.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("Staff").
		Order("created_at ASC").
		Find(&docs).Error; err != nil {
		return nil, nil, 0, err
	}

	pagination := r.buildPagination(page, limit, total)
	return docs, pagination, int(total), nil
}

func (r *documentRepository) GetExpiringDocuments(tenantID string, vendorID *string, daysAhead int, page, limit int) ([]models.StaffDocument, *models.PaginationInfo, int, error) {
	var docs []models.StaffDocument
	var total int64

	futureDate := time.Now().AddDate(0, 0, daysAhead)

	query := r.db.Model(&models.StaffDocument{}).
		Where("tenant_id = ? AND expiry_date IS NOT NULL AND expiry_date <= ? AND expiry_date > ? AND verification_status = ?",
			tenantID, futureDate, time.Now(), models.VerificationVerified)
	query = r.applyVendorFilter(query, vendorID)

	if err := query.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("Staff").
		Order("expiry_date ASC").
		Find(&docs).Error; err != nil {
		return nil, nil, 0, err
	}

	// Enrich with expiry info
	for i := range docs {
		r.enrichDocumentWithExpiryInfo(&docs[i])
	}

	pagination := r.buildPagination(page, limit, total)
	return docs, pagination, int(total), nil
}

func (r *documentRepository) GetExpiredDocuments(tenantID string, vendorID *string, page, limit int) ([]models.StaffDocument, *models.PaginationInfo, int, error) {
	var docs []models.StaffDocument
	var total int64

	query := r.db.Model(&models.StaffDocument{}).
		Where("tenant_id = ? AND expiry_date IS NOT NULL AND expiry_date < ?", tenantID, time.Now())
	query = r.applyVendorFilter(query, vendorID)

	if err := query.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("Staff").
		Order("expiry_date ASC").
		Find(&docs).Error; err != nil {
		return nil, nil, 0, err
	}

	for i := range docs {
		r.enrichDocumentWithExpiryInfo(&docs[i])
	}

	pagination := r.buildPagination(page, limit, total)
	return docs, pagination, int(total), nil
}

func (r *documentRepository) GetStaffComplianceStatus(tenantID string, vendorID *string, staffID uuid.UUID) (*models.StaffComplianceStatus, error) {
	// Get all documents for this staff member
	docs, err := r.ListDocumentsByStaff(tenantID, vendorID, staffID)
	if err != nil {
		return nil, err
	}

	// Get all document types info
	allDocTypes := models.GetAllDocumentTypes()

	// Build compliance status
	status := &models.StaffComplianceStatus{
		StaffID:             staffID,
		IsCompliant:         true,
		MissingMandatory:    make([]models.StaffDocumentType, 0),
		ExpiringDocuments:   make([]models.DocumentComplianceItem, 0),
		PendingVerification: 0,
		VerifiedCount:       0,
		TotalDocuments:      len(docs),
		Items:               make([]models.DocumentComplianceItem, 0),
		LastUpdated:         time.Now(),
	}

	// Create a map of submitted documents
	docMap := make(map[models.StaffDocumentType]*models.StaffDocument)
	for i := range docs {
		docMap[docs[i].DocumentType] = &docs[i]
	}

	// Check each document type
	for _, docType := range allDocTypes {
		item := models.DocumentComplianceItem{
			DocumentType:   docType.Type,
			DisplayName:    docType.DisplayName,
			IsMandatory:    docType.IsMandatory,
			IsSubmitted:    false,
			IsExpired:      false,
			IsExpiringSoon: false,
		}

		if doc, exists := docMap[docType.Type]; exists {
			item.IsSubmitted = true
			item.Status = doc.VerificationStatus
			item.ExpiryDate = doc.ExpiryDate
			item.IsExpired = doc.ExpiryDate != nil && doc.ExpiryDate.Before(time.Now())
			item.IsExpiringSoon = doc.IsExpiringSoon
			item.DaysUntilExpiry = doc.DaysUntilExpiry
			item.Document = doc

			// Count verified
			if doc.VerificationStatus == models.VerificationVerified {
				status.VerifiedCount++
			}

			// Count pending
			if doc.VerificationStatus == models.VerificationPending || doc.VerificationStatus == models.VerificationUnderReview {
				status.PendingVerification++
			}

			// Track expiring documents
			if item.IsExpiringSoon || item.IsExpired {
				status.ExpiringDocuments = append(status.ExpiringDocuments, item)
			}
		} else if docType.IsMandatory {
			// Missing mandatory document
			status.MissingMandatory = append(status.MissingMandatory, docType.Type)
			status.IsCompliant = false
		}

		status.Items = append(status.Items, item)
	}

	// Check for emergency contact
	var contactCount int64
	r.db.Model(&models.EmergencyContact{}).
		Where("tenant_id = ? AND staff_id = ?", tenantID, staffID).
		Count(&contactCount)
	status.HasEmergencyContact = contactCount > 0

	// Calculate compliance percentage
	if len(allDocTypes) > 0 {
		mandatoryCount := 0
		mandatorySubmitted := 0
		for _, docType := range allDocTypes {
			if docType.IsMandatory {
				mandatoryCount++
				if _, exists := docMap[docType.Type]; exists {
					mandatorySubmitted++
				}
			}
		}
		if mandatoryCount > 0 {
			status.CompliancePercentage = float64(mandatorySubmitted) / float64(mandatoryCount) * 100
		} else {
			status.CompliancePercentage = 100
		}
	}

	// Update compliance flag based on expiring documents too
	if len(status.ExpiringDocuments) > 0 {
		for _, doc := range status.ExpiringDocuments {
			if doc.IsExpired && doc.IsMandatory {
				status.IsCompliant = false
				break
			}
		}
	}

	return status, nil
}

// ============================================================================
// EMERGENCY CONTACTS
// ============================================================================

func (r *documentRepository) CreateEmergencyContact(tenantID string, vendorID *string, contact *models.EmergencyContact) error {
	contact.TenantID = tenantID
	contact.VendorID = vendorID
	contact.CreatedAt = time.Now()
	contact.UpdatedAt = time.Now()
	return r.db.Create(contact).Error
}

func (r *documentRepository) GetEmergencyContactByID(tenantID string, vendorID *string, id uuid.UUID) (*models.EmergencyContact, error) {
	var contact models.EmergencyContact
	query := r.db.Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)

	err := query.First(&contact).Error
	if err != nil {
		return nil, err
	}

	return &contact, nil
}

func (r *documentRepository) UpdateEmergencyContact(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateEmergencyContactRequest) error {
	query := r.db.Model(&models.EmergencyContact{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(updates).Error
}

func (r *documentRepository) DeleteEmergencyContact(tenantID string, vendorID *string, id uuid.UUID) error {
	query := r.db.Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Delete(&models.EmergencyContact{}).Error
}

func (r *documentRepository) ListEmergencyContactsByStaff(tenantID string, vendorID *string, staffID uuid.UUID) ([]models.EmergencyContact, error) {
	var contacts []models.EmergencyContact

	query := r.db.Where("tenant_id = ? AND staff_id = ?", tenantID, staffID)
	query = r.applyVendorFilter(query, vendorID)

	err := query.Order("is_primary DESC, name ASC").Find(&contacts).Error
	return contacts, err
}

func (r *documentRepository) SetPrimaryEmergencyContact(tenantID string, vendorID *string, staffID, contactID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Unset all primary flags for this staff member
		query := tx.Model(&models.EmergencyContact{}).
			Where("tenant_id = ? AND staff_id = ?", tenantID, staffID)
		if vendorID != nil {
			query = query.Where("vendor_id = ?", *vendorID)
		}
		if err := query.Update("is_primary", false).Error; err != nil {
			return err
		}

		// Set the specified contact as primary
		query = tx.Model(&models.EmergencyContact{}).
			Where("tenant_id = ? AND id = ?", tenantID, contactID)
		if vendorID != nil {
			query = query.Where("vendor_id = ?", *vendorID)
		}
		return query.Update("is_primary", true).Error
	})
}

// ============================================================================
// BATCH OPERATIONS
// ============================================================================

func (r *documentRepository) UpdateExpiredDocumentStatus(tenantID string, vendorID *string) (int64, error) {
	query := r.db.Model(&models.StaffDocument{}).
		Where("tenant_id = ? AND expiry_date IS NOT NULL AND expiry_date < ? AND verification_status = ?",
			tenantID, time.Now(), models.VerificationVerified)

	if vendorID != nil {
		query = query.Where("vendor_id = ?", *vendorID)
	}

	result := query.Update("verification_status", models.VerificationExpired)
	return result.RowsAffected, result.Error
}

func (r *documentRepository) SendExpiryReminders(tenantID string, vendorID *string, daysAhead int) ([]models.StaffDocument, error) {
	var docs []models.StaffDocument
	futureDate := time.Now().AddDate(0, 0, daysAhead)

	query := r.db.
		Where("tenant_id = ? AND expiry_date IS NOT NULL AND expiry_date <= ? AND expiry_date > ? AND verification_status = ? AND (reminder_sent_at IS NULL OR reminder_sent_at < ?)",
			tenantID, futureDate, time.Now(), models.VerificationVerified, time.Now().AddDate(0, 0, -7))

	if vendorID != nil {
		query = query.Where("vendor_id = ?", *vendorID)
	}

	if err := query.Preload("Staff").Find(&docs).Error; err != nil {
		return nil, err
	}

	// Update reminder_sent_at
	if len(docs) > 0 {
		ids := make([]uuid.UUID, len(docs))
		for i, doc := range docs {
			ids[i] = doc.ID
		}
		r.db.Model(&models.StaffDocument{}).
			Where("id IN ?", ids).
			Update("reminder_sent_at", time.Now())
	}

	return docs, nil
}

// ============================================================================
// HELPERS
// ============================================================================

func (r *documentRepository) applyVendorFilter(query *gorm.DB, vendorID *string) *gorm.DB {
	if vendorID != nil {
		return query.Where("vendor_id = ? OR vendor_id IS NULL", *vendorID)
	}
	return query.Where("vendor_id IS NULL")
}

func (r *documentRepository) buildPagination(page, limit int, total int64) *models.PaginationInfo {
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	return &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}
}

func (r *documentRepository) enrichDocumentWithExpiryInfo(doc *models.StaffDocument) {
	if doc.ExpiryDate != nil {
		now := time.Now()
		daysUntil := int(doc.ExpiryDate.Sub(now).Hours() / 24)
		doc.DaysUntilExpiry = &daysUntil
		doc.IsExpiringSoon = daysUntil > 0 && daysUntil <= 30
	}
}
