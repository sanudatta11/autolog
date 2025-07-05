package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// CustomLoggerMiddleware creates a custom logging middleware that logs HTTP requests in simple text format
func CustomLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// End timer
		end := time.Now()
		latency := end.Sub(start)

		// Get user ID from context if available
		userID := uint(0)
		if user, exists := c.Get("user"); exists {
			if userModel, ok := user.(map[string]interface{}); ok {
				if id, ok := userModel["id"].(float64); ok {
					userID = uint(id)
				}
			}
		}

		// Log the request in simple text format
		fmt.Printf("[API] %s | %s | %d | %s | %s | User: %d\n",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			latency.String(),
			c.ClientIP(),
			userID,
		)
	}
}
