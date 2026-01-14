package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS returns a CORS middleware with default configuration
func CORS() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000", // Next.js storefront
			"http://localhost:4200", // Admin shell app
			"http://localhost:4201", // Tenant onboarding app
			"http://localhost:4301", // Categories MFE
			"http://localhost:4302", // Products MFE
			"http://localhost:4303", // Orders MFE
			"http://localhost:4304", // Coupons MFE
			"http://localhost:4305", // Reviews MFE
			"http://localhost:4306", // Staff MFE
			"http://localhost:4307", // Tickets MFE
			"http://localhost:4308", // User Management MFE
			"http://localhost:4309", // Vendor MFE
			"https://*.tesseract-hub.com",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-Tenant-ID", "X-User-ID", "X-Requested-With", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	return cors.New(config)
}
