// Package clients provides HTTP clients for service-to-service communication.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"customers-service/internal/models"
)

// NotificationClient handles sending notifications via the notification-service API.
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// CustomerNotification represents a customer-related notification payload.
type CustomerNotification struct {
	TenantID      string `json:"tenantId"`
	CustomerID    string `json:"customerId"`
	CustomerEmail string `json:"customerEmail"`
	CustomerName  string `json:"customerName"`
	EventType     string `json:"eventType"` // CREATED, UPDATED, DELETED, EMAIL_VERIFIED
	StorefrontURL string `json:"storefrontUrl,omitempty"`
	AdminURL      string `json:"adminUrl,omitempty"`
}

// notificationRequest is the payload sent to notification-service API.
type notificationRequest struct {
	Channel        string                 `json:"channel"`
	RecipientEmail string                 `json:"recipientEmail"`
	Subject        string                 `json:"subject"`
	TemplateName   string                 `json:"templateName"`
	Variables      map[string]interface{} `json:"variables"`
	TenantID       string                 `json:"tenantId"`
	UserID         string                 `json:"userId,omitempty"`
}

// NewNotificationClient creates a new notification client.
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

// SendCustomerWelcomeNotification sends a welcome email when a customer registers.
func (c *NotificationClient) SendCustomerWelcomeNotification(ctx context.Context, notification *CustomerNotification) error {
	req := notificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: notification.CustomerEmail,
		Subject:        "Welcome to our store!",
		TemplateName:   "customer_welcome",
		TenantID:       notification.TenantID,
		UserID:         notification.CustomerID,
		Variables: map[string]interface{}{
			"customerName":  notification.CustomerName,
			"customerEmail": notification.CustomerEmail,
			"storefrontUrl": notification.StorefrontURL,
		},
	}

	return c.sendNotification(ctx, req)
}

// SendEmailVerifiedNotification sends a confirmation when email is verified.
func (c *NotificationClient) SendEmailVerifiedNotification(ctx context.Context, notification *CustomerNotification) error {
	req := notificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: notification.CustomerEmail,
		Subject:        "Email Verified Successfully",
		TemplateName:   "email_verified",
		TenantID:       notification.TenantID,
		UserID:         notification.CustomerID,
		Variables: map[string]interface{}{
			"customerName":  notification.CustomerName,
			"customerEmail": notification.CustomerEmail,
			"storefrontUrl": notification.StorefrontURL,
		},
	}

	return c.sendNotification(ctx, req)
}

// EmailVerificationNotification represents an email verification request.
type EmailVerificationNotification struct {
	TenantID         string `json:"tenantId"`
	CustomerID       string `json:"customerId"`
	CustomerEmail    string `json:"customerEmail"`
	CustomerName     string `json:"customerName"`
	VerificationLink string `json:"verificationLink"`
	BusinessName     string `json:"businessName"`
	StorefrontURL    string `json:"storefrontUrl"`
	SupportEmail     string `json:"supportEmail"`
}

// SendEmailVerificationNotification sends a verification email to the customer.
func (c *NotificationClient) SendEmailVerificationNotification(ctx context.Context, notification *EmailVerificationNotification) error {
	req := notificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: notification.CustomerEmail,
		Subject:        "Please verify your email address",
		TemplateName:   "customer-verification",
		TenantID:       notification.TenantID,
		UserID:         notification.CustomerID,
		Variables: map[string]interface{}{
			"customerName":     notification.CustomerName,
			"customerEmail":    notification.CustomerEmail,
			"verificationLink": notification.VerificationLink,
			"businessName":     notification.BusinessName,
			"storefrontUrl":    notification.StorefrontURL,
			"supportEmail":     notification.SupportEmail,
		},
	}

	return c.sendNotification(ctx, req)
}

// sendNotification sends a notification request to the notification-service.
func (c *NotificationClient) sendNotification(ctx context.Context, req notificationRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/notifications/send", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", req.TenantID)
	httpReq.Header.Set("X-Internal-Service", "customers-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	return nil
}

// AbandonedCartReminderNotification represents an abandoned cart reminder payload.
type AbandonedCartReminderNotification struct {
	TenantID        string                 `json:"tenantId"`
	CustomerID      string                 `json:"customerId"`
	CustomerEmail   string                 `json:"customerEmail"`
	CustomerName    string                 `json:"customerName"`
	CartID          string                 `json:"cartId"`
	CartItems       []map[string]interface{} `json:"cartItems"`
	CartTotal       float64                `json:"cartTotal"`
	ReminderNumber  int                    `json:"reminderNumber"`
	DiscountCode    string                 `json:"discountCode,omitempty"`
	StorefrontURL   string                 `json:"storefrontUrl"`
	CartRecoveryURL string                 `json:"cartRecoveryUrl"`
}

// SendAbandonedCartReminder sends an abandoned cart reminder email.
func (c *NotificationClient) SendAbandonedCartReminder(ctx context.Context, notification *AbandonedCartReminderNotification) error {
	// Determine template based on reminder number
	templateName := "abandoned_cart_reminder_1"
	subject := "You left something behind!"

	switch notification.ReminderNumber {
	case 2:
		templateName = "abandoned_cart_reminder_2"
		subject = "Still thinking about it?"
	case 3:
		templateName = "abandoned_cart_reminder_3"
		subject = "Last chance to complete your order!"
	}

	variables := map[string]interface{}{
		"customerName":    notification.CustomerName,
		"customerEmail":   notification.CustomerEmail,
		"cartItems":       notification.CartItems,
		"cartTotal":       notification.CartTotal,
		"storefrontUrl":   notification.StorefrontURL,
		"cartRecoveryUrl": notification.CartRecoveryURL,
		"reminderNumber":  notification.ReminderNumber,
	}

	if notification.DiscountCode != "" {
		variables["discountCode"] = notification.DiscountCode
		subject = "Here's a special offer for you!"
	}

	req := notificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: notification.CustomerEmail,
		Subject:        subject,
		TemplateName:   templateName,
		TenantID:       notification.TenantID,
		UserID:         notification.CustomerID,
		Variables:      variables,
	}

	return c.sendNotification(ctx, req)
}

// BuildFromCustomer creates a CustomerNotification from a Customer model.
func BuildFromCustomer(customer *models.Customer) *CustomerNotification {
	name := customer.FirstName
	if customer.LastName != "" {
		name = customer.FirstName + " " + customer.LastName
	}

	return &CustomerNotification{
		TenantID:      customer.TenantID,
		CustomerID:    customer.ID.String(),
		CustomerEmail: customer.Email,
		CustomerName:  name,
	}
}

// AccountLockedNotification represents an account locked notification payload.
type AccountLockedNotification struct {
	TenantID      string `json:"tenantId"`
	CustomerID    string `json:"customerId"`
	CustomerEmail string `json:"customerEmail"`
	CustomerName  string `json:"customerName"`
	Reason        string `json:"reason"`
	StorefrontURL string `json:"storefrontUrl,omitempty"`
	SupportEmail  string `json:"supportEmail,omitempty"`
}

// SendAccountLockedNotification sends an email when a customer account is locked.
func (c *NotificationClient) SendAccountLockedNotification(ctx context.Context, notification *AccountLockedNotification) error {
	req := notificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: notification.CustomerEmail,
		Subject:        "Your Account Has Been Temporarily Locked",
		TemplateName:   "account_locked",
		TenantID:       notification.TenantID,
		UserID:         notification.CustomerID,
		Variables: map[string]interface{}{
			"customerName":  notification.CustomerName,
			"customerEmail": notification.CustomerEmail,
			"reason":        notification.Reason,
			"storefrontUrl": notification.StorefrontURL,
			"supportEmail":  notification.SupportEmail,
		},
	}

	return c.sendNotification(ctx, req)
}

// AccountUnlockedNotification represents an account unlocked notification payload.
type AccountUnlockedNotification struct {
	TenantID      string `json:"tenantId"`
	CustomerID    string `json:"customerId"`
	CustomerEmail string `json:"customerEmail"`
	CustomerName  string `json:"customerName"`
	StorefrontURL string `json:"storefrontUrl,omitempty"`
}

// SendAccountUnlockedNotification sends an email when a customer account is unlocked.
func (c *NotificationClient) SendAccountUnlockedNotification(ctx context.Context, notification *AccountUnlockedNotification) error {
	req := notificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: notification.CustomerEmail,
		Subject:        "Your Account Has Been Reactivated",
		TemplateName:   "account_unlocked",
		TenantID:       notification.TenantID,
		UserID:         notification.CustomerID,
		Variables: map[string]interface{}{
			"customerName":  notification.CustomerName,
			"customerEmail": notification.CustomerEmail,
			"storefrontUrl": notification.StorefrontURL,
		},
	}

	return c.sendNotification(ctx, req)
}
