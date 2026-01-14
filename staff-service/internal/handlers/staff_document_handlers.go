package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"staff-service/internal/models"
	"staff-service/internal/repository"
)

// StaffDocumentHandler handles staff document verification and compliance
type StaffDocumentHandler struct {
	repo      repository.DocumentRepository
	staffRepo repository.StaffRepository
}

func NewStaffDocumentHandler(repo repository.DocumentRepository, staffRepo repository.StaffRepository) *StaffDocumentHandler {
	return &StaffDocumentHandler{repo: repo, staffRepo: staffRepo}
}

// Helper functions
func (h *StaffDocumentHandler) getTenantAndVendor(c *gin.Context) (string, *string) {
	tenantID := c.GetString("tenant_id")
	vendorID := c.GetString("vendor_id")
	if vendorID == "" {
		return tenantID, nil
	}
	return tenantID, &vendorID
}

func (h *StaffDocumentHandler) getPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}

// ============================================================================
// STAFF DOCUMENTS (Database-backed with verification workflow)
// ============================================================================

// CreateStaffDocument creates a new document record for verification
func (h *StaffDocumentHandler) CreateStaffDocument(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.Param("id")
	userID := c.GetString("user_id")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	// Verify staff exists
	if _, err := h.staffRepo.GetByID(tenantID, staffID); err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "STAFF_NOT_FOUND", Message: "Staff member not found"},
		})
		return
	}

	var req models.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	doc := &models.StaffDocument{
		StaffID:          staffID,
		DocumentType:     req.DocumentType,
		DocumentName:     req.DocumentName,
		OriginalFilename: req.OriginalFilename,
		DocumentNumber:   req.DocumentNumber,
		IssuingAuthority: req.IssuingAuthority,
		IssueDate:        req.IssueDate,
		ExpiryDate:       req.ExpiryDate,
		StoragePath:      req.StoragePath,
		FileSize:         req.FileSize,
		MimeType:         req.MimeType,
		Metadata:         req.Metadata,
		CreatedBy:        &userID,
	}

	if req.AccessLevel != "" {
		doc.AccessLevel = req.AccessLevel
	} else {
		doc.AccessLevel = models.AccessLevelHROnly
	}

	if req.IsMandatory != nil {
		doc.IsMandatory = *req.IsMandatory
	}

	if err := h.repo.CreateDocument(tenantID, vendorID, doc); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "CREATE_FAILED", Message: "Failed to create document record"},
		})
		return
	}

	c.JSON(http.StatusCreated, models.StaffDocumentResponse{
		Success: true,
		Data:    doc,
	})
}

// GetStaffDocument retrieves a document by ID
func (h *StaffDocumentHandler) GetStaffDocument(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	docIDStr := c.Param("docId")

	docID, err := uuid.Parse(docIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid document ID format"},
		})
		return
	}

	doc, err := h.repo.GetDocumentByID(tenantID, vendorID, docID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NOT_FOUND", Message: "Document not found"},
		})
		return
	}

	c.JSON(http.StatusOK, models.StaffDocumentResponse{
		Success: true,
		Data:    doc,
	})
}

// UpdateStaffDocument updates a document
func (h *StaffDocumentHandler) UpdateStaffDocument(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	docIDStr := c.Param("docId")

	docID, err := uuid.Parse(docIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid document ID format"},
		})
		return
	}

	var req models.UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	if err := h.repo.UpdateDocument(tenantID, vendorID, docID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UPDATE_FAILED", Message: "Failed to update document"},
		})
		return
	}

	doc, _ := h.repo.GetDocumentByID(tenantID, vendorID, docID)
	c.JSON(http.StatusOK, models.StaffDocumentResponse{
		Success: true,
		Data:    doc,
	})
}

// DeleteStaffDocument soft deletes a document
func (h *StaffDocumentHandler) DeleteStaffDocument(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	docIDStr := c.Param("docId")
	userID := c.GetString("user_id")

	docID, err := uuid.Parse(docIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid document ID format"},
		})
		return
	}

	if err := h.repo.DeleteDocument(tenantID, vendorID, docID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "DELETE_FAILED", Message: "Failed to delete document"},
		})
		return
	}

	msg := "Document deleted successfully"
	c.JSON(http.StatusOK, models.StaffDocumentResponse{
		Success: true,
		Message: &msg,
	})
}

// ListStaffDocuments lists all documents for a staff member
func (h *StaffDocumentHandler) ListStaffDocuments(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.Param("id")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	docs, err := h.repo.ListDocumentsByStaff(tenantID, vendorID, staffID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to list documents"},
		})
		return
	}

	c.JSON(http.StatusOK, models.StaffDocumentListResponse{
		Success: true,
		Data:    docs,
	})
}

// VerifyStaffDocument verifies or rejects a document
func (h *StaffDocumentHandler) VerifyStaffDocument(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	docIDStr := c.Param("docId")
	verifierIDStr := c.GetString("staff_id")

	docID, err := uuid.Parse(docIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid document ID format"},
		})
		return
	}

	verifierID, err := uuid.Parse(verifierIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_VERIFIER", Message: "Verifier ID is required"},
		})
		return
	}

	var req models.VerifyDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	// Validate status
	validStatuses := map[models.DocumentVerificationStatus]bool{
		models.VerificationVerified:       true,
		models.VerificationRejected:       true,
		models.VerificationUnderReview:    true,
		models.VerificationRequiresUpdate: true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_STATUS", Message: "Invalid verification status"},
		})
		return
	}

	if err := h.repo.VerifyDocument(tenantID, vendorID, docID, &req, verifierID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "VERIFY_FAILED", Message: "Failed to verify document"},
		})
		return
	}

	doc, _ := h.repo.GetDocumentByID(tenantID, vendorID, docID)
	c.JSON(http.StatusOK, models.StaffDocumentResponse{
		Success: true,
		Data:    doc,
	})
}

// ============================================================================
// DOCUMENT QUERIES
// ============================================================================

// GetPendingDocuments gets all documents pending verification
func (h *StaffDocumentHandler) GetPendingDocuments(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	page, limit := h.getPagination(c)

	docs, pagination, count, err := h.repo.GetPendingDocuments(tenantID, vendorID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to get pending documents"},
		})
		return
	}

	c.JSON(http.StatusOK, models.PendingDocumentsResponse{
		Success:    true,
		Data:       docs,
		Pagination: pagination,
		Count:      count,
	})
}

// GetExpiringDocuments gets documents expiring within specified days
func (h *StaffDocumentHandler) GetExpiringDocuments(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	page, limit := h.getPagination(c)

	daysAhead, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if daysAhead < 1 {
		daysAhead = 30
	}

	docs, pagination, count, err := h.repo.GetExpiringDocuments(tenantID, vendorID, daysAhead, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to get expiring documents"},
		})
		return
	}

	c.JSON(http.StatusOK, models.ExpiringDocumentsResponse{
		Success:    true,
		Data:       docs,
		Pagination: pagination,
		Count:      count,
	})
}

// GetStaffComplianceStatus gets compliance status for a staff member
func (h *StaffDocumentHandler) GetStaffComplianceStatus(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.Param("id")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	status, err := h.repo.GetStaffComplianceStatus(tenantID, vendorID, staffID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "GET_FAILED", Message: "Failed to get compliance status"},
		})
		return
	}

	c.JSON(http.StatusOK, models.ComplianceStatusResponse{
		Success: true,
		Data:    status,
	})
}

// GetDocumentTypes returns all available document types
func (h *StaffDocumentHandler) GetDocumentTypes(c *gin.Context) {
	types := models.GetAllDocumentTypes()

	c.JSON(http.StatusOK, models.DocumentTypesResponse{
		Success: true,
		Data:    types,
	})
}

// ============================================================================
// EMERGENCY CONTACTS
// ============================================================================

// CreateEmergencyContact creates a new emergency contact
func (h *StaffDocumentHandler) CreateEmergencyContact(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.Param("id")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	var req models.CreateEmergencyContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	contact := &models.EmergencyContact{
		StaffID:        staffID,
		Name:           req.Name,
		Relationship:   req.Relationship,
		PhonePrimary:   req.PhonePrimary,
		PhoneSecondary: req.PhoneSecondary,
		Email:          req.Email,
		Address:        req.Address,
		Notes:          req.Notes,
	}

	if req.IsPrimary != nil {
		contact.IsPrimary = *req.IsPrimary
	}

	if err := h.repo.CreateEmergencyContact(tenantID, vendorID, contact); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "CREATE_FAILED", Message: "Failed to create emergency contact"},
		})
		return
	}

	c.JSON(http.StatusCreated, models.EmergencyContactResponse{
		Success: true,
		Data:    contact,
	})
}

// GetEmergencyContact retrieves an emergency contact by ID
func (h *StaffDocumentHandler) GetEmergencyContact(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	contactIDStr := c.Param("contactId")

	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid contact ID format"},
		})
		return
	}

	contact, err := h.repo.GetEmergencyContactByID(tenantID, vendorID, contactID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NOT_FOUND", Message: "Emergency contact not found"},
		})
		return
	}

	c.JSON(http.StatusOK, models.EmergencyContactResponse{
		Success: true,
		Data:    contact,
	})
}

// UpdateEmergencyContact updates an emergency contact
func (h *StaffDocumentHandler) UpdateEmergencyContact(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	contactIDStr := c.Param("contactId")

	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid contact ID format"},
		})
		return
	}

	var req models.UpdateEmergencyContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	if err := h.repo.UpdateEmergencyContact(tenantID, vendorID, contactID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UPDATE_FAILED", Message: "Failed to update emergency contact"},
		})
		return
	}

	contact, _ := h.repo.GetEmergencyContactByID(tenantID, vendorID, contactID)
	c.JSON(http.StatusOK, models.EmergencyContactResponse{
		Success: true,
		Data:    contact,
	})
}

// DeleteEmergencyContact deletes an emergency contact
func (h *StaffDocumentHandler) DeleteEmergencyContact(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	contactIDStr := c.Param("contactId")

	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid contact ID format"},
		})
		return
	}

	if err := h.repo.DeleteEmergencyContact(tenantID, vendorID, contactID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "DELETE_FAILED", Message: "Failed to delete emergency contact"},
		})
		return
	}

	msg := "Emergency contact deleted successfully"
	c.JSON(http.StatusOK, models.EmergencyContactResponse{
		Success: true,
		Message: &msg,
	})
}

// ListStaffEmergencyContacts lists all emergency contacts for a staff member
func (h *StaffDocumentHandler) ListStaffEmergencyContacts(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.Param("id")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	contacts, err := h.repo.ListEmergencyContactsByStaff(tenantID, vendorID, staffID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to list emergency contacts"},
		})
		return
	}

	c.JSON(http.StatusOK, models.EmergencyContactListResponse{
		Success: true,
		Data:    contacts,
	})
}

// SetPrimaryEmergencyContact sets a contact as the primary emergency contact
func (h *StaffDocumentHandler) SetPrimaryEmergencyContact(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.Param("id")
	contactIDStr := c.Param("contactId")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid contact ID format"},
		})
		return
	}

	if err := h.repo.SetPrimaryEmergencyContact(tenantID, vendorID, staffID, contactID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UPDATE_FAILED", Message: "Failed to set primary contact"},
		})
		return
	}

	msg := "Primary contact set successfully"
	c.JSON(http.StatusOK, models.EmergencyContactResponse{
		Success: true,
		Message: &msg,
	})
}

// ============================================================================
// BATCH OPERATIONS
// ============================================================================

// UpdateExpiredDocuments marks expired documents as expired
func (h *StaffDocumentHandler) UpdateExpiredDocuments(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)

	count, err := h.repo.UpdateExpiredDocumentStatus(tenantID, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UPDATE_FAILED", Message: "Failed to update expired documents"},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"updatedCount": count,
		"message":      "Expired documents updated",
	})
}
