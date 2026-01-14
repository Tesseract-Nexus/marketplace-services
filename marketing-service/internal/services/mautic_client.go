package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"marketing-service/internal/config"
	"marketing-service/internal/models"
)

// MauticClient handles integration with Mautic marketing automation
type MauticClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	logger     *logrus.Logger
	enabled    bool
}

// MauticSegment represents a segment in Mautic
type MauticSegment struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name"`
	Alias       string `json:"alias,omitempty"`
	Description string `json:"description"`
	IsPublished bool   `json:"isPublished"`
	IsGlobal    bool   `json:"isGlobal"`
}

// MauticEmail represents an email template in Mautic
type MauticEmail struct {
	ID            int    `json:"id,omitempty"`
	Name          string `json:"name"`
	Subject       string `json:"subject"`
	FromAddress   string `json:"fromAddress,omitempty"`
	FromName      string `json:"fromName,omitempty"`
	ReplyToEmail  string `json:"replyToAddress,omitempty"`
	CustomHTML    string `json:"customHtml,omitempty"`
	PlainText     string `json:"plainText,omitempty"`
	IsPublished   bool   `json:"isPublished"`
	EmailType     string `json:"emailType"` // "template" or "list"
	Lists         []int  `json:"lists,omitempty"`
	Template      string `json:"template,omitempty"`
	Language      string `json:"language"`
}

// MauticContact represents a contact in Mautic
type MauticContact struct {
	ID        int               `json:"id,omitempty"`
	Email     string            `json:"email"`
	FirstName string            `json:"firstname,omitempty"`
	LastName  string            `json:"lastname,omitempty"`
	Phone     string            `json:"phone,omitempty"`
	Company   string            `json:"company,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// MauticCampaign represents a campaign in Mautic
type MauticCampaign struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublished bool   `json:"isPublished"`
}

// MauticAPIResponse represents a generic Mautic API response
type MauticAPIResponse struct {
	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"errors,omitempty"`
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Success     bool   `json:"success"`
	MauticID    int    `json:"mauticId,omitempty"`
	Error       string `json:"error,omitempty"`
	SyncedAt    time.Time `json:"syncedAt"`
}

// NewMauticClient creates a new Mautic client instance
func NewMauticClient(cfg *config.Config, logger *logrus.Logger) *MauticClient {
	return &MauticClient{
		baseURL:  cfg.MauticURL,
		username: cfg.MauticUsername,
		password: cfg.MauticPassword,
		enabled:  cfg.MauticEnabled,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// IsEnabled returns whether Mautic integration is enabled
func (c *MauticClient) IsEnabled() bool {
	return c.enabled && c.baseURL != "" && c.password != ""
}

// getAuthHeader returns the Basic Auth header value
func (c *MauticClient) getAuthHeader() string {
	credentials := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.password))
	return "Basic " + credentials
}

// doRequest performs an HTTP request to the Mautic API
func (c *MauticClient) doRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := fmt.Sprintf("%s/api%s", c.baseURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", c.getAuthHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		c.logger.WithFields(logrus.Fields{
			"status":   resp.StatusCode,
			"endpoint": endpoint,
			"response": string(respBody),
		}).Error("Mautic API error")
		return nil, fmt.Errorf("mautic API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ==================== SEGMENT OPERATIONS ====================

// CreateSegment creates a new segment in Mautic
func (c *MauticClient) CreateSegment(ctx context.Context, segment *MauticSegment) (int, error) {
	if !c.IsEnabled() {
		c.logger.Debug("Mautic integration disabled, skipping segment creation")
		return 0, nil
	}

	respBody, err := c.doRequest(ctx, "POST", "/segments/new", segment)
	if err != nil {
		return 0, err
	}

	var result struct {
		List struct {
			ID int `json:"id"`
		} `json:"list"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"segmentName": segment.Name,
		"mauticId":    result.List.ID,
	}).Info("Created segment in Mautic")

	return result.List.ID, nil
}

// SyncSegment syncs a local segment to Mautic
func (c *MauticClient) SyncSegment(ctx context.Context, segment *models.CustomerSegment) (*SyncResult, error) {
	if !c.IsEnabled() {
		return &SyncResult{Success: true, SyncedAt: time.Now()}, nil
	}

	mauticSegment := &MauticSegment{
		Name:        segment.Name,
		Description: segment.Description,
		IsPublished: segment.IsActive,
		IsGlobal:    false,
	}

	mauticID, err := c.CreateSegment(ctx, mauticSegment)
	if err != nil {
		return &SyncResult{
			Success:  false,
			Error:    err.Error(),
			SyncedAt: time.Now(),
		}, err
	}

	return &SyncResult{
		Success:  true,
		MauticID: mauticID,
		SyncedAt: time.Now(),
	}, nil
}

// AddContactToSegment adds a contact to a segment in Mautic
func (c *MauticClient) AddContactToSegment(ctx context.Context, segmentID, contactID int) error {
	if !c.IsEnabled() {
		return nil
	}

	endpoint := fmt.Sprintf("/segments/%d/contact/%d/add", segmentID, contactID)
	_, err := c.doRequest(ctx, "POST", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to add contact to segment: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"segmentId": segmentID,
		"contactId": contactID,
	}).Debug("Added contact to segment in Mautic")

	return nil
}

// ==================== EMAIL OPERATIONS ====================

// CreateEmail creates a new email template in Mautic
func (c *MauticClient) CreateEmail(ctx context.Context, email *MauticEmail) (int, error) {
	if !c.IsEnabled() {
		return 0, nil
	}

	respBody, err := c.doRequest(ctx, "POST", "/emails/new", email)
	if err != nil {
		return 0, err
	}

	var result struct {
		Email struct {
			ID int `json:"id"`
		} `json:"email"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"emailName": email.Name,
		"mauticId":  result.Email.ID,
	}).Info("Created email in Mautic")

	return result.Email.ID, nil
}

// SendEmailToContact sends an email to a specific contact
func (c *MauticClient) SendEmailToContact(ctx context.Context, emailID, contactID int) error {
	if !c.IsEnabled() {
		return nil
	}

	endpoint := fmt.Sprintf("/emails/%d/contact/%d/send", emailID, contactID)
	_, err := c.doRequest(ctx, "POST", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to send email to contact: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"emailId":   emailID,
		"contactId": contactID,
	}).Debug("Sent email to contact via Mautic")

	return nil
}

// SendEmailToSegment sends an email to all contacts in a segment
func (c *MauticClient) SendEmailToSegment(ctx context.Context, emailID, segmentID int) error {
	if !c.IsEnabled() {
		return nil
	}

	endpoint := fmt.Sprintf("/emails/%d/send", emailID)
	_, err := c.doRequest(ctx, "POST", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to send email to segment: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"emailId":   emailID,
		"segmentId": segmentID,
	}).Info("Sent email to segment via Mautic")

	return nil
}

// ==================== CONTACT OPERATIONS ====================

// CreateOrUpdateContact creates or updates a contact in Mautic
func (c *MauticClient) CreateOrUpdateContact(ctx context.Context, contact *MauticContact) (int, error) {
	if !c.IsEnabled() {
		return 0, nil
	}

	data := map[string]interface{}{
		"email":                contact.Email,
		"firstname":            contact.FirstName,
		"lastname":             contact.LastName,
		"phone":                contact.Phone,
		"company":              contact.Company,
		"overwriteWithBlank":   false,
	}

	respBody, err := c.doRequest(ctx, "POST", "/contacts/new", data)
	if err != nil {
		return 0, err
	}

	var result struct {
		Contact struct {
			ID int `json:"id"`
		} `json:"contact"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"email":    contact.Email,
		"mauticId": result.Contact.ID,
	}).Debug("Created/updated contact in Mautic")

	return result.Contact.ID, nil
}

// GetContactByEmail retrieves a contact by email address
func (c *MauticClient) GetContactByEmail(ctx context.Context, email string) (*MauticContact, error) {
	if !c.IsEnabled() {
		return nil, nil
	}

	endpoint := fmt.Sprintf("/contacts?search=%s", email)
	respBody, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Contacts map[string]struct {
			ID     int `json:"id"`
			Fields struct {
				Core map[string]struct {
					Value string `json:"value"`
				} `json:"core"`
			} `json:"fields"`
		} `json:"contacts"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	for _, contact := range result.Contacts {
		return &MauticContact{
			ID:        contact.ID,
			Email:     contact.Fields.Core["email"].Value,
			FirstName: contact.Fields.Core["firstname"].Value,
			LastName:  contact.Fields.Core["lastname"].Value,
		}, nil
	}

	return nil, nil
}

// ==================== CAMPAIGN OPERATIONS ====================

// SyncCampaign syncs a local campaign to Mautic (creates email template)
func (c *MauticClient) SyncCampaign(ctx context.Context, campaign *models.Campaign, fromEmail, fromName string) (*SyncResult, error) {
	if !c.IsEnabled() {
		return &SyncResult{Success: true, SyncedAt: time.Now()}, nil
	}

	// Create email template in Mautic for this campaign
	mauticEmail := &MauticEmail{
		Name:        campaign.Name,
		Subject:     campaign.Subject,
		FromAddress: fromEmail,
		FromName:    fromName,
		CustomHTML:  campaign.Content,
		IsPublished: campaign.Status != models.CampaignStatusDraft,
		EmailType:   "list",
		Language:    "en",
	}

	emailID, err := c.CreateEmail(ctx, mauticEmail)
	if err != nil {
		return &SyncResult{
			Success:  false,
			Error:    err.Error(),
			SyncedAt: time.Now(),
		}, err
	}

	return &SyncResult{
		Success:  true,
		MauticID: emailID,
		SyncedAt: time.Now(),
	}, nil
}

// SendCampaign sends a campaign via Mautic to contacts in a segment
func (c *MauticClient) SendCampaign(ctx context.Context, campaign *models.Campaign, mauticEmailID, mauticSegmentID int) error {
	if !c.IsEnabled() {
		return nil
	}

	// If we have a segment, link the email to the segment and send
	if mauticSegmentID > 0 {
		// Update email to target the segment
		updateData := map[string]interface{}{
			"lists": []int{mauticSegmentID},
		}
		endpoint := fmt.Sprintf("/emails/%d/edit", mauticEmailID)
		if _, err := c.doRequest(ctx, "PATCH", endpoint, updateData); err != nil {
			c.logger.WithError(err).Warn("Failed to link email to segment")
		}
	}

	// Send the email
	if err := c.SendEmailToSegment(ctx, mauticEmailID, mauticSegmentID); err != nil {
		return fmt.Errorf("failed to send campaign via Mautic: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"campaignId":    campaign.ID,
		"mauticEmailId": mauticEmailID,
		"segmentId":     mauticSegmentID,
	}).Info("Campaign sent via Mautic")

	return nil
}

// ==================== STATISTICS ====================

// GetEmailStats retrieves statistics for an email in Mautic
func (c *MauticClient) GetEmailStats(ctx context.Context, emailID int) (map[string]interface{}, error) {
	if !c.IsEnabled() {
		return nil, nil
	}

	endpoint := fmt.Sprintf("/emails/%d", emailID)
	respBody, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// GetSegmentStats retrieves statistics for a segment in Mautic
func (c *MauticClient) GetSegmentStats(ctx context.Context, segmentID int) (int, error) {
	if !c.IsEnabled() {
		return 0, nil
	}

	endpoint := fmt.Sprintf("/segments/%d", segmentID)
	respBody, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return 0, err
	}

	var result struct {
		List struct {
			ID    int `json:"id"`
			Leads int `json:"leads"`
		} `json:"list"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.List.Leads, nil
}

// ==================== HEALTH CHECK ====================

// HealthCheck verifies connectivity to Mautic API
func (c *MauticClient) HealthCheck(ctx context.Context) error {
	if !c.IsEnabled() {
		return fmt.Errorf("mautic integration is disabled")
	}

	_, err := c.doRequest(ctx, "GET", "/contacts?limit=1", nil)
	if err != nil {
		return fmt.Errorf("mautic health check failed: %w", err)
	}

	return nil
}
