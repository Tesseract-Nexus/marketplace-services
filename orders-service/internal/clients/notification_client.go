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

// NotificationClient sends notifications via notification-service API
type NotificationClient interface {
	// SendOrderConfirmation sends order confirmation email to customer
	SendOrderConfirmation(ctx context.Context, order *OrderNotification) error
	// SendOrderShipped sends shipping notification email
	SendOrderShipped(ctx context.Context, order *OrderNotification) error
	// SendOrderDelivered sends delivery confirmation email
	SendOrderDelivered(ctx context.Context, order *OrderNotification) error
	// SendOrderCancelled sends cancellation email
	SendOrderCancelled(ctx context.Context, order *OrderNotification) error
	// SendOrderRefunded sends refund confirmation email
	SendOrderRefunded(ctx context.Context, order *OrderNotification) error
}

// notificationClient implements NotificationClient
type notificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewNotificationClient creates a new notification client
func NewNotificationClient() NotificationClient {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://notification-service.devtest.svc.cluster.local:8090"
	}

	return &notificationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendNotificationRequest represents the API request to notification-service
type SendNotificationRequest struct {
	Channel        string                 `json:"channel"`
	RecipientEmail string                 `json:"recipientEmail"`
	Subject        string                 `json:"subject"`
	TemplateName   string                 `json:"templateName,omitempty"`
	Variables      map[string]interface{} `json:"variables,omitempty"`
	BodyHTML       string                 `json:"bodyHtml,omitempty"`
}

// OrderNotification contains order details for notification
type OrderNotification struct {
	TenantID          string
	OrderID           string
	OrderNumber       string
	OrderDate         string
	CustomerEmail     string
	CustomerName      string
	OrderStatus       string
	TrackingURL       string
	OrderDetailsURL   string
	ShopURL           string
	ReviewURL         string
	Items             []OrderItem
	Currency          string
	Subtotal          string
	Discount          string
	Shipping          string
	Tax               string
	Total             string
	ShippingAddress   *Address
	PaymentMethod     string
	Carrier           string
	TrackingNumber    string
	EstimatedDelivery string
	DeliveryDate      string
	DeliveryLocation  string
	CancelledDate     string
	CancellationReason string
	RefundAmount      string
	RefundDays        string
	BusinessName      string
}

// OrderItem represents an item in an order notification
type OrderItem struct {
	Name     string `json:"name"`
	SKU      string `json:"sku"`
	ImageURL string `json:"imageUrl"`
	Quantity int    `json:"quantity"`
	Price    string `json:"price"`
	Currency string `json:"currency"`
}

// Address represents a shipping/billing address
type Address struct {
	Name       string `json:"name"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// SendOrderConfirmation sends order confirmation email to customer
func (c *notificationClient) SendOrderConfirmation(ctx context.Context, order *OrderNotification) error {
	if order == nil {
		log.Printf("[NotificationClient] Skipping order confirmation - order is nil")
		return nil
	}
	if order.CustomerEmail == "" {
		log.Printf("[NotificationClient] Skipping order confirmation - no customer email for order %s", order.OrderNumber)
		return nil
	}

	order.OrderStatus = "CONFIRMED"
	req := c.buildNotificationRequest(order)
	req.Subject = fmt.Sprintf("Order Confirmed - #%s", order.OrderNumber)
	req.TemplateName = "order_customer" // Unified template, uses OrderStatus to determine content

	if err := c.send(ctx, order.TenantID, req); err != nil {
		log.Printf("[NotificationClient] Failed to send order confirmation: %v", err)
		return err
	}

	log.Printf("[NotificationClient] Order confirmation sent for order %s to %s", order.OrderNumber, order.CustomerEmail)
	return nil
}

// SendOrderShipped sends shipping notification email
func (c *notificationClient) SendOrderShipped(ctx context.Context, order *OrderNotification) error {
	if order == nil {
		log.Printf("[NotificationClient] Skipping shipping notification - order is nil")
		return nil
	}
	if order.CustomerEmail == "" {
		log.Printf("[NotificationClient] Skipping shipping notification - no customer email for order %s", order.OrderNumber)
		return nil
	}

	order.OrderStatus = "SHIPPED"
	req := c.buildNotificationRequest(order)
	req.Subject = fmt.Sprintf("Your Order is On Its Way - #%s", order.OrderNumber)
	req.TemplateName = "order_customer" // Unified template, uses OrderStatus to determine content

	if err := c.send(ctx, order.TenantID, req); err != nil {
		log.Printf("[NotificationClient] Failed to send shipping notification: %v", err)
		return err
	}

	log.Printf("[NotificationClient] Shipping notification sent for order %s to %s", order.OrderNumber, order.CustomerEmail)
	return nil
}

// SendOrderDelivered sends delivery confirmation email
func (c *notificationClient) SendOrderDelivered(ctx context.Context, order *OrderNotification) error {
	if order == nil {
		log.Printf("[NotificationClient] Skipping delivery notification - order is nil")
		return nil
	}
	if order.CustomerEmail == "" {
		log.Printf("[NotificationClient] Skipping delivery notification - no customer email for order %s", order.OrderNumber)
		return nil
	}

	order.OrderStatus = "DELIVERED"
	req := c.buildNotificationRequest(order)
	req.Subject = fmt.Sprintf("Your Order Has Arrived - #%s", order.OrderNumber)
	req.TemplateName = "order_customer" // Unified template, uses OrderStatus to determine content

	if err := c.send(ctx, order.TenantID, req); err != nil {
		log.Printf("[NotificationClient] Failed to send delivery notification: %v", err)
		return err
	}

	log.Printf("[NotificationClient] Delivery notification sent for order %s to %s", order.OrderNumber, order.CustomerEmail)
	return nil
}

// SendOrderCancelled sends cancellation email
func (c *notificationClient) SendOrderCancelled(ctx context.Context, order *OrderNotification) error {
	if order == nil {
		log.Printf("[NotificationClient] Skipping cancellation notification - order is nil")
		return nil
	}
	if order.CustomerEmail == "" {
		log.Printf("[NotificationClient] Skipping cancellation notification - no customer email for order %s", order.OrderNumber)
		return nil
	}

	order.OrderStatus = "CANCELLED"
	req := c.buildNotificationRequest(order)
	req.Subject = fmt.Sprintf("Order Cancelled - #%s", order.OrderNumber)
	req.TemplateName = "order_customer" // Unified template, uses OrderStatus to determine content

	if err := c.send(ctx, order.TenantID, req); err != nil {
		log.Printf("[NotificationClient] Failed to send cancellation notification: %v", err)
		return err
	}

	log.Printf("[NotificationClient] Cancellation notification sent for order %s to %s", order.OrderNumber, order.CustomerEmail)
	return nil
}

// SendOrderRefunded sends refund confirmation email
func (c *notificationClient) SendOrderRefunded(ctx context.Context, order *OrderNotification) error {
	if order == nil {
		log.Printf("[NotificationClient] Skipping refund notification - order is nil")
		return nil
	}
	if order.CustomerEmail == "" {
		log.Printf("[NotificationClient] Skipping refund notification - no customer email for order %s", order.OrderNumber)
		return nil
	}

	order.OrderStatus = "REFUNDED"
	req := c.buildNotificationRequest(order)
	req.Subject = fmt.Sprintf("Refund Processed - #%s", order.OrderNumber)
	req.TemplateName = "order_customer" // Unified template, uses OrderStatus to determine content

	if err := c.send(ctx, order.TenantID, req); err != nil {
		log.Printf("[NotificationClient] Failed to send refund notification: %v", err)
		return err
	}

	log.Printf("[NotificationClient] Refund notification sent for order %s to %s", order.OrderNumber, order.CustomerEmail)
	return nil
}

// buildNotificationRequest builds the notification request from order data
func (c *notificationClient) buildNotificationRequest(order *OrderNotification) SendNotificationRequest {
	// Convert items to map format
	var items []map[string]interface{}
	for _, item := range order.Items {
		items = append(items, map[string]interface{}{
			"name":     item.Name,
			"sku":      item.SKU,
			"imageUrl": item.ImageURL,
			"quantity": item.Quantity,
			"price":    item.Price,
			"currency": item.Currency,
		})
	}

	// Build shipping address map
	var shippingAddress map[string]interface{}
	if order.ShippingAddress != nil {
		shippingAddress = map[string]interface{}{
			"name":       order.ShippingAddress.Name,
			"line1":      order.ShippingAddress.Line1,
			"line2":      order.ShippingAddress.Line2,
			"city":       order.ShippingAddress.City,
			"state":      order.ShippingAddress.State,
			"postalCode": order.ShippingAddress.PostalCode,
			"country":    order.ShippingAddress.Country,
		}
	}

	return SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: order.CustomerEmail,
		Variables: map[string]interface{}{
			"orderId":            order.OrderID,
			"orderNumber":        order.OrderNumber,
			"orderDate":          order.OrderDate,
			"orderStatus":        order.OrderStatus,
			"customerName":       order.CustomerName,
			"customerEmail":      order.CustomerEmail,
			"trackingUrl":        order.TrackingURL,
			"orderDetailsUrl":    order.OrderDetailsURL,
			"shopUrl":            order.ShopURL,
			"reviewUrl":          order.ReviewURL,
			"items":              items,
			"currency":           order.Currency,
			"subtotal":           order.Subtotal,
			"discount":           order.Discount,
			"shipping":           order.Shipping,
			"tax":                order.Tax,
			"total":              order.Total,
			"shippingAddress":    shippingAddress,
			"paymentMethod":      order.PaymentMethod,
			"carrier":            order.Carrier,
			"trackingNumber":     order.TrackingNumber,
			"estimatedDelivery":  order.EstimatedDelivery,
			"deliveryDate":       order.DeliveryDate,
			"deliveryLocation":   order.DeliveryLocation,
			"cancelledDate":      order.CancelledDate,
			"cancellationReason": order.CancellationReason,
			"refundAmount":       order.RefundAmount,
			"refundDays":         order.RefundDays,
			"businessName":       order.BusinessName,
		},
	}
}

func (c *notificationClient) send(ctx context.Context, tenantID string, req SendNotificationRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/notifications/send", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-Internal-Service", "orders-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification-service returned status %d", resp.StatusCode)
	}

	return nil
}
