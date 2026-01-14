package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	Timestamp   time.Time         `json:"timestamp"`
	RequestID   string            `json:"requestId"`
	TenantID    string            `json:"tenantId"`
	UserID      string            `json:"userId"`
	VendorID    string            `json:"vendorId,omitempty"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	StatusCode  int               `json:"statusCode"`
	Duration    time.Duration     `json:"duration"`
	ClientIP    string            `json:"clientIp"`
	UserAgent   string            `json:"userAgent"`
	Action      string            `json:"action,omitempty"`
	Resource    string            `json:"resource,omitempty"`
	ResourceID  string            `json:"resourceId,omitempty"`
	Success     bool              `json:"success"`
	ErrorMsg    string            `json:"error,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AuditLogger defines the interface for audit logging
type AuditLogger interface {
	Log(entry *AuditLog)
}

// DefaultAuditLogger logs to stdout in JSON format
type DefaultAuditLogger struct{}

func (l *DefaultAuditLogger) Log(entry *AuditLog) {
	data, _ := json.Marshal(entry)
	log.Printf("[AUDIT] %s", string(data))
}

// AuditMiddleware logs all payment-related requests
func AuditMiddleware(logger AuditLogger) gin.HandlerFunc {
	if logger == nil {
		logger = &DefaultAuditLogger{}
	}

	return func(c *gin.Context) {
		start := time.Now()

		// Read request body for audit (only for POST/PUT)
		var requestBody []byte
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Process request
		c.Next()

		// Build audit entry
		entry := &AuditLog{
			Timestamp:  start,
			RequestID:  c.GetString("requestID"),
			TenantID:   c.GetString("tenantID"),
			UserID:     c.GetString("userID"),
			VendorID:   c.GetString("vendorID"),
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			StatusCode: c.Writer.Status(),
			Duration:   time.Since(start),
			ClientIP:   c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
			Success:    c.Writer.Status() < 400,
		}

		// Extract action and resource from path
		entry.Action, entry.Resource, entry.ResourceID = parsePaymentAction(c)

		// Add any error message
		if entry.StatusCode >= 400 {
			if errors, exists := c.Get("errors"); exists {
				entry.ErrorMsg = errors.(string)
			}
		}

		// Add payment-specific metadata
		entry.Metadata = extractPaymentMetadata(c, requestBody)

		// Log the entry
		logger.Log(entry)
	}
}

// parsePaymentAction extracts action and resource from the request
func parsePaymentAction(c *gin.Context) (action, resource, resourceID string) {
	path := c.Request.URL.Path
	method := c.Request.Method

	// Map paths to actions
	switch {
	case path == "/api/v1/payments/create-intent":
		return "create_payment_intent", "payment", ""
	case path == "/api/v1/payments/confirm":
		return "confirm_payment", "payment", ""
	case matchPath(path, "/api/v1/payments/*/refund"):
		return "create_refund", "refund", c.Param("id")
	case matchPath(path, "/api/v1/payments/*/cancel"):
		return "cancel_payment", "payment", c.Param("id")
	case matchPath(path, "/api/v1/payments/*"):
		return "get_payment", "payment", c.Param("id")
	case path == "/api/v1/gateway-configs" && method == "POST":
		return "create_gateway_config", "gateway_config", ""
	case matchPath(path, "/api/v1/gateway-configs/*") && method == "PUT":
		return "update_gateway_config", "gateway_config", c.Param("id")
	case matchPath(path, "/api/v1/gateway-configs/*") && method == "DELETE":
		return "delete_gateway_config", "gateway_config", c.Param("id")
	case path == "/api/v1/payment-settings" && method == "PUT":
		return "update_payment_settings", "payment_settings", ""
	case matchPath(path, "/webhooks/*"):
		return "webhook_received", "webhook", c.Param("*")
	default:
		return method, path, ""
	}
}

// matchPath checks if path matches a pattern with * wildcards
func matchPath(path, pattern string) bool {
	// Simple pattern matching - * matches any segment
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, part := range patternParts {
		if part != "*" && part != pathParts[i] {
			return false
		}
	}

	return true
}

func splitPath(path string) []string {
	parts := []string{}
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// extractPaymentMetadata extracts relevant metadata from request
func extractPaymentMetadata(c *gin.Context, body []byte) map[string]string {
	metadata := make(map[string]string)

	// For payment creation, extract amount and currency
	if c.Request.URL.Path == "/api/v1/payments/create-intent" && len(body) > 0 {
		var req struct {
			Amount      float64 `json:"amount"`
			Currency    string  `json:"currency"`
			GatewayType string  `json:"gatewayType"`
			OrderID     string  `json:"orderId"`
		}
		if json.Unmarshal(body, &req) == nil {
			metadata["amount"] = formatAmount(req.Amount)
			metadata["currency"] = req.Currency
			metadata["gateway"] = req.GatewayType
			metadata["order_id"] = req.OrderID
		}
	}

	// For refunds, extract amount
	if matchPath(c.Request.URL.Path, "/api/v1/payments/*/refund") && len(body) > 0 {
		var req struct {
			Amount float64 `json:"amount"`
			Reason string  `json:"reason"`
		}
		if json.Unmarshal(body, &req) == nil {
			metadata["refund_amount"] = formatAmount(req.Amount)
			metadata["reason"] = req.Reason
		}
	}

	return metadata
}

func formatAmount(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

// SensitiveFields are fields that should be masked in logs
var SensitiveFields = []string{
	"api_key",
	"api_secret",
	"secret_key",
	"webhook_secret",
	"password",
	"card_number",
	"cvv",
	"cvc",
	"expiry",
}

// MaskSensitiveData masks sensitive fields in a map
func MaskSensitiveData(data map[string]interface{}) map[string]interface{} {
	masked := make(map[string]interface{})
	for k, v := range data {
		isSensitive := false
		for _, sf := range SensitiveFields {
			if k == sf || containsIgnoreCase(k, sf) {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			masked[k] = "***MASKED***"
		} else if nestedMap, ok := v.(map[string]interface{}); ok {
			masked[k] = MaskSensitiveData(nestedMap)
		} else {
			masked[k] = v
		}
	}
	return masked
}

func containsIgnoreCase(s, substr string) bool {
	s = bytes.NewBuffer([]byte(s)).String()
	substr = bytes.NewBuffer([]byte(substr)).String()
	return bytes.Contains(bytes.ToLower([]byte(s)), bytes.ToLower([]byte(substr)))
}
