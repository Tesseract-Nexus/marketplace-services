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

// ReviewNotification contains data for sending review email notifications
type ReviewNotification struct {
	TenantID       string
	ReviewID       string
	ProductID      string
	ProductName    string
	CustomerEmail  string
	CustomerName   string
	Rating         int
	Title          string
	Comment        string
	IsVerified     bool
	Status         string // PENDING, APPROVED, REJECTED
	RejectionReason string
	ReviewURL      string
	ProductURL     string
	AdminURL       string
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
		baseURL = "http://notification-service.marketplace.svc.cluster.local:8090"
	}

	return &NotificationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendReviewCreatedNotification sends notification when a review is submitted
func (c *NotificationClient) SendReviewCreatedNotification(ctx context.Context, notification *ReviewNotification) error {
	// Send customer confirmation
	if notification.CustomerEmail != "" {
		customerReq := &notificationRequest{
			To:       notification.CustomerEmail,
			Subject:  fmt.Sprintf("Thank you for your review of %s", notification.ProductName),
			Template: "review_customer",
			Variables: map[string]string{
				"reviewId":       notification.ReviewID,
				"productId":      notification.ProductID,
				"productName":    notification.ProductName,
				"customerName":   notification.CustomerName,
				"customerEmail":  notification.CustomerEmail,
				"rating":         fmt.Sprintf("%d", notification.Rating),
				"reviewTitle":    notification.Title,
				"reviewComment":  notification.Comment,
				"isVerified":     fmt.Sprintf("%t", notification.IsVerified),
				"reviewStatus":   "CREATED",
				"productUrl":     notification.ProductURL,
				"tenantId":       notification.TenantID,
			},
		}
		if err := c.sendNotification(ctx, notification.TenantID, customerReq); err != nil {
			log.Printf("[REVIEW] Failed to send customer notification: %v", err)
		}
	}

	// Send admin notification for moderation
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@tesserix.app"
	}

	adminReq := &notificationRequest{
		To:       adminEmail,
		Subject:  fmt.Sprintf("New Review Pending Moderation: %s", notification.ProductName),
		Template: "review_admin",
		Variables: map[string]string{
			"reviewId":      notification.ReviewID,
			"productId":     notification.ProductID,
			"productName":   notification.ProductName,
			"customerName":  notification.CustomerName,
			"customerEmail": notification.CustomerEmail,
			"rating":        fmt.Sprintf("%d", notification.Rating),
			"reviewTitle":   notification.Title,
			"reviewComment": notification.Comment,
			"isVerified":    fmt.Sprintf("%t", notification.IsVerified),
			"reviewStatus":  "CREATED",
			"adminUrl":      notification.AdminURL,
			"tenantId":      notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, adminReq)
}

// SendReviewApprovedNotification sends notification when a review is approved
func (c *NotificationClient) SendReviewApprovedNotification(ctx context.Context, notification *ReviewNotification) error {
	if notification.CustomerEmail == "" {
		log.Printf("[REVIEW] No customer email for review %s, skipping notification", notification.ReviewID)
		return nil
	}

	req := &notificationRequest{
		To:       notification.CustomerEmail,
		Subject:  fmt.Sprintf("Your review of %s is now live!", notification.ProductName),
		Template: "review_customer",
		Variables: map[string]string{
			"reviewId":      notification.ReviewID,
			"productId":     notification.ProductID,
			"productName":   notification.ProductName,
			"customerName":  notification.CustomerName,
			"customerEmail": notification.CustomerEmail,
			"rating":        fmt.Sprintf("%d", notification.Rating),
			"reviewTitle":   notification.Title,
			"reviewComment": notification.Comment,
			"isVerified":    fmt.Sprintf("%t", notification.IsVerified),
			"reviewStatus":  "APPROVED",
			"productUrl":    notification.ProductURL,
			"tenantId":      notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, req)
}

// SendReviewRejectedNotification sends notification when a review is rejected
func (c *NotificationClient) SendReviewRejectedNotification(ctx context.Context, notification *ReviewNotification) error {
	if notification.CustomerEmail == "" {
		log.Printf("[REVIEW] No customer email for review %s, skipping notification", notification.ReviewID)
		return nil
	}

	req := &notificationRequest{
		To:       notification.CustomerEmail,
		Subject:  fmt.Sprintf("Update on your review of %s", notification.ProductName),
		Template: "review_customer",
		Variables: map[string]string{
			"reviewId":        notification.ReviewID,
			"productId":       notification.ProductID,
			"productName":     notification.ProductName,
			"customerName":    notification.CustomerName,
			"customerEmail":   notification.CustomerEmail,
			"rating":          fmt.Sprintf("%d", notification.Rating),
			"reviewTitle":     notification.Title,
			"reviewComment":   notification.Comment,
			"isVerified":      fmt.Sprintf("%t", notification.IsVerified),
			"reviewStatus":    "REJECTED",
			"rejectionReason": notification.RejectionReason,
			"productUrl":      notification.ProductURL,
			"tenantId":        notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, req)
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
	httpReq.Header.Set("X-Internal-Service", "reviews-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	log.Printf("[REVIEW] Notification sent successfully to %s (template: %s)", req.To, req.Template)
	return nil
}
