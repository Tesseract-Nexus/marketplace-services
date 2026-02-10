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

	"coupons-service/internal/models"
)

// NotificationClient handles HTTP communication with notification-service
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// CouponNotification contains data for sending coupon email notifications
type CouponNotification struct {
	TenantID       string
	CouponID       string
	CouponCode     string
	DiscountType   string
	DiscountValue  float64
	DiscountAmount float64
	OrderValue     float64
	CustomerEmail  string
	CustomerName   string
	OrderID        string
	ValidFrom      string
	ValidUntil     string
	Status         string // CREATED, APPLIED, EXPIRED
	CouponsURL     string
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

// BuildFromCoupon creates a CouponNotification from a Coupon model
func BuildFromCoupon(coupon *models.Coupon) *CouponNotification {
	notification := &CouponNotification{
		TenantID:      coupon.TenantID,
		CouponID:      coupon.ID.String(),
		CouponCode:    coupon.Code,
		DiscountType:  string(coupon.DiscountType),
		DiscountValue: coupon.DiscountValue,
		ValidFrom:     coupon.ValidFrom.Format(time.RFC3339),
		Status:        string(coupon.Status),
	}
	if coupon.ValidUntil != nil {
		notification.ValidUntil = coupon.ValidUntil.Format(time.RFC3339)
	}
	return notification
}

// SendCouponCreatedNotification sends notification when a coupon is created
func (c *NotificationClient) SendCouponCreatedNotification(ctx context.Context, notification *CouponNotification) error {
	// Admin notification for new coupon creation
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@tesserix.app"
	}

	req := &notificationRequest{
		To:       adminEmail,
		Subject:  fmt.Sprintf("New Coupon Created: %s", notification.CouponCode),
		Template: "coupon_created",
		Variables: map[string]string{
			"couponId":      notification.CouponID,
			"couponCode":    notification.CouponCode,
			"discountType":  notification.DiscountType,
			"discountValue": fmt.Sprintf("%.2f", notification.DiscountValue),
			"validFrom":     notification.ValidFrom,
			"validUntil":    notification.ValidUntil,
			"couponsUrl":    notification.CouponsURL,
			"tenantId":      notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, req)
}

// SendCouponAppliedNotification sends notification when a coupon is applied to an order
func (c *NotificationClient) SendCouponAppliedNotification(ctx context.Context, notification *CouponNotification) error {
	if notification.CustomerEmail == "" {
		log.Printf("[COUPON] No customer email for coupon %s, skipping notification", notification.CouponID)
		return nil
	}

	req := &notificationRequest{
		To:       notification.CustomerEmail,
		Subject:  fmt.Sprintf("Coupon %s applied to your order!", notification.CouponCode),
		Template: "coupon_applied",
		Variables: map[string]string{
			"couponId":       notification.CouponID,
			"couponCode":     notification.CouponCode,
			"discountType":   notification.DiscountType,
			"discountValue":  fmt.Sprintf("%.2f", notification.DiscountValue),
			"discountAmount": fmt.Sprintf("%.2f", notification.DiscountAmount),
			"orderValue":     fmt.Sprintf("%.2f", notification.OrderValue),
			"customerName":   notification.CustomerName,
			"orderId":        notification.OrderID,
			"tenantId":       notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, req)
}

// SendCouponExpiredNotification sends notification when a coupon expires (admin notification)
func (c *NotificationClient) SendCouponExpiredNotification(ctx context.Context, notification *CouponNotification) error {
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@tesserix.app"
	}

	req := &notificationRequest{
		To:       adminEmail,
		Subject:  fmt.Sprintf("Coupon Expired: %s", notification.CouponCode),
		Template: "coupon_expired",
		Variables: map[string]string{
			"couponId":    notification.CouponID,
			"couponCode":  notification.CouponCode,
			"validUntil":  notification.ValidUntil,
			"couponsUrl":  notification.CouponsURL,
			"tenantId":    notification.TenantID,
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
	httpReq.Header.Set("X-Internal-Service", "coupons-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	log.Printf("[COUPON] Notification sent successfully to %s (template: %s)", req.To, req.Template)
	return nil
}
