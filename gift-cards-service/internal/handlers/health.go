package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck handles health check requests
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "gift-cards-service",
		"version": "1.0.0",
	})
}
