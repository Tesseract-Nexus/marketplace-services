package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthCheck returns the health status of the service
// @Summary Health check
// @Description Returns the health status of the vendor service
// @Tags health
// @Produce json
// @Success 200 {object} object{status=string,timestamp=string,service=string}
// @Router /health [get]
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "vendor-service",
		"version":   "1.0.0",
	})
}

// ReadinessCheck returns the readiness status of the service
// @Summary Readiness check
// @Description Returns the readiness status of the vendor service
// @Tags health
// @Produce json
// @Success 200 {object} object{status=string,timestamp=string,service=string}
// @Router /ready [get]
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	// Add database connectivity check here if needed
	c.JSON(http.StatusOK, gin.H{
		"status":    "ready",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "vendor-service",
		"version":   "1.0.0",
	})
}
