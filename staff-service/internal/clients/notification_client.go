package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// NotificationClient handles HTTP communication with notification-service
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// StaffInvitationNotification contains data for sending staff invitation emails
type StaffInvitationNotification struct {
	TenantID       string
	VendorID       string
	StaffID        string
	StaffEmail     string
	StaffName      string
	Role           string
	InviterName    string
	InviterID      string // User ID of the person sending the invitation (for auth)
	BusinessName   string
	ActivationLink string
}

// notificationRequest is the API request format for notification-service
type notificationRequest struct {
	Channel        string                 `json:"channel"`
	RecipientEmail string                 `json:"recipientEmail"`
	Subject        string                 `json:"subject"`
	TemplateName   string                 `json:"templateName"`
	Variables      map[string]interface{} `json:"variables"`
}

// NewNotificationClient creates a new notification client
func NewNotificationClient() *NotificationClient {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://notification-service.marketplace.svc.cluster.local:8090"
	}

	return &NotificationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendStaffInvitation sends an invitation email to a new staff member
func (c *NotificationClient) SendStaffInvitation(ctx context.Context, notification *StaffInvitationNotification) error {
	if notification.StaffEmail == "" {
		log.Printf("[STAFF] No email for staff %s, skipping invitation email", notification.StaffID)
		return nil
	}

	businessName := notification.BusinessName
	if businessName == "" {
		businessName = "Tesseract Hub"
	}

	req := &notificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: notification.StaffEmail,
		Subject:        fmt.Sprintf("You're invited to join %s", businessName),
		TemplateName:   "staff_invitation",
		Variables: map[string]interface{}{
			"staffName":      notification.StaffName,
			"staffEmail":     notification.StaffEmail,
			"role":           notification.Role,
			"inviterName":    notification.InviterName,
			"businessName":   businessName,
			"activationLink": notification.ActivationLink,
			"tenantId":       notification.TenantID,
			"vendorId":       notification.VendorID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, notification.InviterID, req)
}

// sendNotification sends a notification request to notification-service
func (c *NotificationClient) sendNotification(ctx context.Context, tenantID, userID string, req *notificationRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/notifications/send", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-User-ID", userID)
	httpReq.Header.Set("X-Internal-Service", "staff-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	log.Printf("[STAFF] Invitation email sent successfully to %s (template: %s)", req.RecipientEmail, req.TemplateName)
	return nil
}
