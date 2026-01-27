package services

import (
	"strings"
	"time"

	"orders-service/internal/models"
)

// MaskEmail masks an email address: "john@example.com" -> "j***@example.com"
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	if len(local) <= 1 {
		return local + "***@" + parts[1]
	}
	return string(local[0]) + "***@" + parts[1]
}

// MaskPhone masks a phone number: "+919876543210" -> "+91****3210"
func MaskPhone(phone string) string {
	if phone == "" {
		return ""
	}
	if len(phone) <= 4 {
		return "****"
	}
	// Keep first 3 and last 4 characters
	prefix := phone[:3]
	suffix := phone[len(phone)-4:]
	masked := ""
	for i := 0; i < len(phone)-7; i++ {
		masked += "*"
	}
	return prefix + masked + suffix
}

// MaskOrderForPublic returns a JSON-safe map with PII masked for public guest order display.
func MaskOrderForPublic(order *models.Order) map[string]interface{} {
	result := map[string]interface{}{
		"orderNumber":   order.OrderNumber,
		"status":        order.Status,
		"paymentStatus": order.PaymentStatus,
		"currency":      order.Currency,
		"subtotal":      order.Subtotal,
		"shippingCost":  order.ShippingCost,
		"taxAmount":     order.TaxAmount,
		"total":         order.Total,
		"createdAt":     order.CreatedAt.Format(time.RFC3339),
	}

	// Items (public info only)
	var items []map[string]interface{}
	for _, item := range order.Items {
		items = append(items, map[string]interface{}{
			"productName": item.ProductName,
			"quantity":    item.Quantity,
			"unitPrice":   item.UnitPrice,
			"totalPrice":  item.TotalPrice,
			"image":       item.Image,
		})
	}
	result["items"] = items

	// Customer info (masked)
	if order.Customer != nil {
		result["customer"] = map[string]interface{}{
			"firstName": order.Customer.FirstName,
			"lastName":  order.Customer.LastName,
			"email":     MaskEmail(order.Customer.Email),
			"phone":     MaskPhone(order.Customer.Phone),
		}
	}

	// Shipping info (partially masked)
	if order.Shipping != nil {
		shipping := map[string]interface{}{
			"method":  order.Shipping.Method,
			"city":    order.Shipping.City,
			"state":   order.Shipping.State,
			"country": order.Shipping.Country,
		}
		// Mask street and postal code
		if order.Shipping.Street != "" {
			if len(order.Shipping.Street) > 3 {
				shipping["street"] = order.Shipping.Street[:3] + "***"
			} else {
				shipping["street"] = "***"
			}
		}
		if order.Shipping.PostalCode != "" {
			if len(order.Shipping.PostalCode) > 2 {
				shipping["postalCode"] = order.Shipping.PostalCode[:2] + "***"
			} else {
				shipping["postalCode"] = "***"
			}
		}

		// Tracking info
		if order.Shipping.Carrier != "" {
			shipping["carrier"] = order.Shipping.Carrier
		}
		if order.Shipping.TrackingNumber != "" {
			shipping["trackingNumber"] = order.Shipping.TrackingNumber
		}

		result["shipping"] = shipping
		result["fulfillmentStatus"] = order.FulfillmentStatus
	}

	// Computed: canCancel
	canCancel := order.Status == models.OrderStatusPlaced || order.Status == models.OrderStatusConfirmed
	result["canCancel"] = canCancel

	return result
}
