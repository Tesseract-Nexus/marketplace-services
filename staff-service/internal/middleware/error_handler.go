package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error ErrorDetails `json:"error"`
}

// ErrorDetails contains the error information
type ErrorDetails struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	TraceID   string                 `json:"trace_id"`
}

// CustomError represents a custom application error
type CustomError struct {
	Code       string
	Message    string
	StatusCode int
	Details    map[string]interface{}
}

func (e CustomError) Error() string {
	return e.Message
}

// Common error codes
const (
	// Staff-specific errors
	ErrCodeStaffNotFound      = "STAFF_NOT_FOUND"
	ErrCodeStaffAlreadyExists = "STAFF_ALREADY_EXISTS"
	ErrCodeStaffInvalidData   = "STAFF_INVALID_DATA"

	// General errors
	ErrCodeInternalServer   = "INTERNAL_SERVER_ERROR"
	ErrCodeBadRequest       = "BAD_REQUEST"
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeForbidden        = "FORBIDDEN"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	ErrCodeConflict         = "CONFLICT"
	ErrCodeValidationFailed = "VALIDATION_FAILED"
	ErrCodeDatabaseError    = "DATABASE_ERROR"
	ErrCodeExternalService  = "EXTERNAL_SERVICE_ERROR"
)

// ErrorHandler is a middleware that handles errors in a consistent way
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			handleError(c, err.Err)
		}
	}
}

// handleError processes the error and sends appropriate response
func handleError(c *gin.Context, err error) {
	var response ErrorResponse
	var statusCode int

	// Get or generate trace ID
	traceID, exists := c.Get("trace_id")
	if !exists {
		traceID = uuid.New().String()
	}

	// Check if it's a custom error
	if customErr, ok := err.(CustomError); ok {
		statusCode = customErr.StatusCode
		response = ErrorResponse{
			Error: ErrorDetails{
				Code:      customErr.Code,
				Message:   customErr.Message,
				Details:   customErr.Details,
				Timestamp: time.Now().UTC(),
				TraceID:   traceID.(string),
			},
		}
	} else {
		// Default to internal server error
		statusCode = http.StatusInternalServerError
		response = ErrorResponse{
			Error: ErrorDetails{
				Code:      ErrCodeInternalServer,
				Message:   "An unexpected error occurred",
				Timestamp: time.Now().UTC(),
				TraceID:   traceID.(string),
			},
		}
	}

	// Log the error
	logError(c, err, response.Error)

	// Send response
	c.JSON(statusCode, response)
}

// logError logs the error details
func logError(c *gin.Context, err error, errorDetails ErrorDetails) {
	fmt.Printf("[ERROR] TraceID: %s, Code: %s, Message: %s, Path: %s, Method: %s, Error: %v\n",
		errorDetails.TraceID,
		errorDetails.Code,
		errorDetails.Message,
		c.Request.URL.Path,
		c.Request.Method,
		err,
	)
}

// NewBadRequestError creates a new bad request error
func NewBadRequestError(message string, details map[string]interface{}) CustomError {
	return CustomError{
		Code:       ErrCodeBadRequest,
		Message:    message,
		StatusCode: http.StatusBadRequest,
		Details:    details,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(details map[string]interface{}) CustomError {
	return CustomError{
		Code:       ErrCodeValidationFailed,
		Message:    "Validation failed",
		StatusCode: http.StatusBadRequest,
		Details:    details,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(resource string) CustomError {
	return CustomError{
		Code:       ErrCodeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		StatusCode: http.StatusNotFound,
	}
}

// NewUnauthorizedError creates a new unauthorized error
func NewUnauthorizedError(message string) CustomError {
	return CustomError{
		Code:       ErrCodeUnauthorized,
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

// NewForbiddenError creates a new forbidden error
func NewForbiddenError(message string) CustomError {
	return CustomError{
		Code:       ErrCodeForbidden,
		Message:    message,
		StatusCode: http.StatusForbidden,
	}
}

// NewConflictError creates a new conflict error
func NewConflictError(message string, details map[string]interface{}) CustomError {
	return CustomError{
		Code:       ErrCodeConflict,
		Message:    message,
		StatusCode: http.StatusConflict,
		Details:    details,
	}
}

// NewDatabaseError creates a new database error
func NewDatabaseError(message string) CustomError {
	return CustomError{
		Code:       ErrCodeDatabaseError,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// NewExternalServiceError creates a new external service error
func NewExternalServiceError(service string, err error) CustomError {
	return CustomError{
		Code:       ErrCodeExternalService,
		Message:    fmt.Sprintf("External service error: %s", service),
		StatusCode: http.StatusServiceUnavailable,
		Details: map[string]interface{}{
			"service": service,
			"error":   err.Error(),
		},
	}
}

// Staff-specific errors

// NewStaffNotFoundError creates a new staff not found error
func NewStaffNotFoundError(staffID string) CustomError {
	return CustomError{
		Code:       ErrCodeStaffNotFound,
		Message:    "Staff member not found",
		StatusCode: http.StatusNotFound,
		Details: map[string]interface{}{
			"staff_id": staffID,
		},
	}
}

// NewStaffAlreadyExistsError creates a new staff already exists error
func NewStaffAlreadyExistsError(email string) CustomError {
	return CustomError{
		Code:       ErrCodeStaffAlreadyExists,
		Message:    "Staff member with this email already exists",
		StatusCode: http.StatusConflict,
		Details: map[string]interface{}{
			"email": email,
		},
	}
}

// NewStaffInvalidDataError creates a new staff invalid data error
func NewStaffInvalidDataError(field string, reason string) CustomError {
	return CustomError{
		Code:       ErrCodeStaffInvalidData,
		Message:    "Invalid staff data",
		StatusCode: http.StatusBadRequest,
		Details: map[string]interface{}{
			"field":  field,
			"reason": reason,
		},
	}
}
