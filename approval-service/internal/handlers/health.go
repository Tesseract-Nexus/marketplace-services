package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck handles health check requests
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "approval-service",
	})
}

// ReadinessCheck handles readiness check requests
func ReadinessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ready",
		"service": "approval-service",
	})
}
