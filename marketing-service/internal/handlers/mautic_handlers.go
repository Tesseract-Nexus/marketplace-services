package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"marketing-service/internal/services"
)

// MauticHandlers handles HTTP requests for Mautic integration
type MauticHandlers struct {
	mauticClient *services.MauticClient
	marketing    *services.MarketingService
	logger       *logrus.Logger
}

// NewMauticHandlers creates a new mautic handlers instance
func NewMauticHandlers(mauticClient *services.MauticClient, marketing *services.MarketingService, logger *logrus.Logger) *MauticHandlers {
	return &MauticHandlers{
		mauticClient: mauticClient,
		marketing:    marketing,
		logger:       logger,
	}
}

// MauticStatus represents the Mautic integration status
type MauticStatus struct {
	Enabled     bool   `json:"enabled"`
	Connected   bool   `json:"connected"`
	URL         string `json:"url,omitempty"`
	Error       string `json:"error,omitempty"`
	LastChecked string `json:"lastChecked,omitempty"`
}

// GetIntegrationStatus returns the current Mautic integration status
// GET /api/v1/integrations/mautic/status
func (h *MauticHandlers) GetIntegrationStatus(c *gin.Context) {
	status := &MauticStatus{
		Enabled: h.mauticClient.IsEnabled(),
	}

	if !status.Enabled {
		status.Error = "Mautic integration is disabled or credentials are not configured"
		c.JSON(http.StatusOK, status)
		return
	}

	// Try to connect to Mautic
	if err := h.mauticClient.HealthCheck(c.Request.Context()); err != nil {
		status.Connected = false
		status.Error = err.Error()
	} else {
		status.Connected = true
	}

	c.JSON(http.StatusOK, status)
}

// SyncCampaignRequest represents a request to sync a campaign to Mautic
type SyncCampaignRequest struct {
	CampaignID string `json:"campaignId" binding:"required"`
}

// SyncCampaign manually syncs a campaign to Mautic
// POST /api/v1/integrations/mautic/sync/campaign
func (h *MauticHandlers) SyncCampaign(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var req SyncCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	campaignID, err := uuid.Parse(req.CampaignID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	// Get campaign
	campaign, err := h.marketing.GetCampaign(c.Request.Context(), tenantID, campaignID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	// Sync to Mautic
	result, err := h.mauticClient.SyncCampaign(c.Request.Context(), campaign, "noreply@mail.tesserix.app", "Tesseract Hub")
	if err != nil {
		h.logger.WithError(err).Error("Failed to sync campaign to Mautic")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to sync campaign to Mautic",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  result.Success,
		"mauticId": result.MauticID,
		"syncedAt": result.SyncedAt,
	})
}

// SyncSegmentRequest represents a request to sync a segment to Mautic
type SyncSegmentRequest struct {
	SegmentID string `json:"segmentId" binding:"required"`
}

// SyncSegment manually syncs a segment to Mautic
// POST /api/v1/integrations/mautic/sync/segment
func (h *MauticHandlers) SyncSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var req SyncSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	segmentID, err := uuid.Parse(req.SegmentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid segment ID"})
		return
	}

	// Get segment
	segment, err := h.marketing.GetSegment(c.Request.Context(), tenantID, segmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Segment not found"})
		return
	}

	// Sync to Mautic
	result, err := h.mauticClient.SyncSegment(c.Request.Context(), segment)
	if err != nil {
		h.logger.WithError(err).Error("Failed to sync segment to Mautic")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to sync segment to Mautic",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  result.Success,
		"mauticId": result.MauticID,
		"syncedAt": result.SyncedAt,
	})
}

// CreateContactRequest represents a request to create a contact in Mautic
type CreateContactRequest struct {
	Email     string `json:"email" binding:"required,email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Phone     string `json:"phone"`
	Company   string `json:"company"`
}

// CreateContact creates a contact in Mautic
// POST /api/v1/integrations/mautic/contacts
func (h *MauticHandlers) CreateContact(c *gin.Context) {
	var req CreateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact := &services.MauticContact{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		Company:   req.Company,
	}

	mauticID, err := h.mauticClient.CreateOrUpdateContact(c.Request.Context(), contact)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create contact in Mautic")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to create contact in Mautic",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":  true,
		"mauticId": mauticID,
		"email":    req.Email,
	})
}

// AddToSegmentRequest represents a request to add a contact to a segment
type AddToSegmentRequest struct {
	ContactID int `json:"contactId" binding:"required"`
	SegmentID int `json:"segmentId" binding:"required"`
}

// AddContactToSegment adds a contact to a Mautic segment
// POST /api/v1/integrations/mautic/segments/add-contact
func (h *MauticHandlers) AddContactToSegment(c *gin.Context) {
	var req AddToSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.mauticClient.AddContactToSegment(c.Request.Context(), req.SegmentID, req.ContactID); err != nil {
		h.logger.WithError(err).Error("Failed to add contact to segment")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to add contact to segment",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"contactId": req.ContactID,
		"segmentId": req.SegmentID,
	})
}

// TestEmailRequest represents a request to send a test email
type TestEmailRequest struct {
	To      string `json:"to" binding:"required,email"`
	Subject string `json:"subject" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// SendTestEmail sends a test email via Mautic
// POST /api/v1/integrations/mautic/test-email
func (h *MauticHandlers) SendTestEmail(c *gin.Context) {
	var req TestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create contact for recipient
	contact := &services.MauticContact{
		Email: req.To,
	}
	contactID, err := h.mauticClient.CreateOrUpdateContact(c.Request.Context(), contact)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to create contact",
			"detail": err.Error(),
		})
		return
	}

	// Create email
	email := &services.MauticEmail{
		Name:        "Test Email - " + req.Subject,
		Subject:     req.Subject,
		CustomHTML:  req.Content,
		IsPublished: true,
		EmailType:   "template",
		Language:    "en",
	}
	emailID, err := h.mauticClient.CreateEmail(c.Request.Context(), email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to create email template",
			"detail": err.Error(),
		})
		return
	}

	// Send email
	if err := h.mauticClient.SendEmailToContact(c.Request.Context(), emailID, contactID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to send email",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Test email sent successfully",
		"to":        req.To,
		"emailId":   emailID,
		"contactId": contactID,
	})
}
