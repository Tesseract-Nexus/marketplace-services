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

	"vendor-service/internal/models"
)

// NotificationClient handles HTTP communication with notification-service
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// VendorNotification contains data for sending vendor email notifications
type VendorNotification struct {
	TenantID         string
	VendorID         string
	VendorName       string
	BusinessName     string
	VendorEmail      string
	Status           string // CREATED, APPROVED, REJECTED, SUSPENDED, REACTIVATED
	PreviousStatus   string
	StatusReason     string
	ReviewedBy       string
	VendorURL        string
	AdminURL         string
}

// notificationRequest is the API request format for notification-service
type notificationRequest struct {
	To        string            `json:"to"`
	Subject   string            `json:"subject"`
	Template  string            `json:"template"`
	Variables map[string]string `json:"variables"`
}

// NewNotificationClient creates a new notification client
func NewNotificationClient() *NotificationClient {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://notification-service.devtest.svc.cluster.local:8090"
	}

	return &NotificationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// BuildFromVendor creates a VendorNotification from a Vendor model
func BuildFromVendor(vendor *models.Vendor) *VendorNotification {
	notification := &VendorNotification{
		TenantID:     vendor.TenantID,
		VendorID:     vendor.ID.String(),
		VendorName:   vendor.PrimaryContact,
		BusinessName: vendor.Name,
		VendorEmail:  vendor.Email,
		Status:       string(vendor.Status),
	}
	return notification
}

// SendVendorCreatedNotification sends notification when a vendor application is submitted
func (c *NotificationClient) SendVendorCreatedNotification(ctx context.Context, notification *VendorNotification) error {
	// Send vendor welcome/confirmation email
	if notification.VendorEmail != "" {
		vendorReq := &notificationRequest{
			To:       notification.VendorEmail,
			Subject:  "Your vendor application has been received",
			Template: "vendor_customer",
			Variables: map[string]string{
				"vendorId":         notification.VendorID,
				"vendorName":       notification.VendorName,
				"vendorBusinessName": notification.BusinessName,
				"vendorEmail":      notification.VendorEmail,
				"vendorStatus":     "CREATED",
				"tenantId":         notification.TenantID,
			},
		}
		if err := c.sendNotification(ctx, notification.TenantID, vendorReq); err != nil {
			log.Printf("[VENDOR] Failed to send vendor confirmation: %v", err)
		}
	}

	// Send admin notification
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@tesserix.app"
	}

	adminReq := &notificationRequest{
		To:       adminEmail,
		Subject:  fmt.Sprintf("New Vendor Application: %s", notification.BusinessName),
		Template: "vendor_admin",
		Variables: map[string]string{
			"vendorId":         notification.VendorID,
			"vendorName":       notification.VendorName,
			"vendorBusinessName": notification.BusinessName,
			"vendorEmail":      notification.VendorEmail,
			"vendorStatus":     "CREATED",
			"adminUrl":         notification.AdminURL,
			"tenantId":         notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, adminReq)
}

// SendVendorApprovedNotification sends notification when a vendor is approved
func (c *NotificationClient) SendVendorApprovedNotification(ctx context.Context, notification *VendorNotification) error {
	if notification.VendorEmail == "" {
		log.Printf("[VENDOR] No vendor email for vendor %s, skipping notification", notification.VendorID)
		return nil
	}

	req := &notificationRequest{
		To:       notification.VendorEmail,
		Subject:  "Congratulations! Your vendor application has been approved",
		Template: "vendor_customer",
		Variables: map[string]string{
			"vendorId":         notification.VendorID,
			"vendorName":       notification.VendorName,
			"vendorBusinessName": notification.BusinessName,
			"vendorEmail":      notification.VendorEmail,
			"vendorStatus":     "APPROVED",
			"vendorUrl":        notification.VendorURL,
			"tenantId":         notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, req)
}

// SendVendorRejectedNotification sends notification when a vendor is rejected
func (c *NotificationClient) SendVendorRejectedNotification(ctx context.Context, notification *VendorNotification) error {
	if notification.VendorEmail == "" {
		log.Printf("[VENDOR] No vendor email for vendor %s, skipping notification", notification.VendorID)
		return nil
	}

	req := &notificationRequest{
		To:       notification.VendorEmail,
		Subject:  "Update on your vendor application",
		Template: "vendor_customer",
		Variables: map[string]string{
			"vendorId":         notification.VendorID,
			"vendorName":       notification.VendorName,
			"vendorBusinessName": notification.BusinessName,
			"vendorEmail":      notification.VendorEmail,
			"vendorStatus":     "REJECTED",
			"statusReason":     notification.StatusReason,
			"tenantId":         notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, req)
}

// SendVendorSuspendedNotification sends notification when a vendor is suspended
func (c *NotificationClient) SendVendorSuspendedNotification(ctx context.Context, notification *VendorNotification) error {
	if notification.VendorEmail == "" {
		log.Printf("[VENDOR] No vendor email for vendor %s, skipping notification", notification.VendorID)
		return nil
	}

	req := &notificationRequest{
		To:       notification.VendorEmail,
		Subject:  "Important: Your vendor account has been suspended",
		Template: "vendor_customer",
		Variables: map[string]string{
			"vendorId":         notification.VendorID,
			"vendorName":       notification.VendorName,
			"vendorBusinessName": notification.BusinessName,
			"vendorEmail":      notification.VendorEmail,
			"vendorStatus":     "SUSPENDED",
			"statusReason":     notification.StatusReason,
			"tenantId":         notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, req)
}

// SendVendorUpdatedNotification sends notification when a vendor is updated
func (c *NotificationClient) SendVendorUpdatedNotification(ctx context.Context, notification *VendorNotification) error {
	// For simple updates, we may not want to send notifications
	// This is a placeholder for future use
	log.Printf("[VENDOR] Vendor %s updated, no notification sent for generic updates", notification.VendorID)
	return nil
}

// sendNotification sends a notification request to notification-service
func (c *NotificationClient) sendNotification(ctx context.Context, tenantID string, req *notificationRequest) error {
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
	httpReq.Header.Set("X-Internal-Service", "vendor-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	log.Printf("[VENDOR] Notification sent successfully to %s (template: %s)", req.To, req.Template)
	return nil
}
