package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/incident-sage/backend/internal/database"
	"github.com/incident-sage/backend/internal/handlers"
	"github.com/incident-sage/backend/internal/middleware"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Connect to database
	database.Connect()
	database.AutoMigrate()

	// Set Gin mode
	if os.Getenv("ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	r := gin.Default()

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{os.Getenv("CORS_ORIGIN")}
	config.AllowCredentials = true
	config.AddAllowHeaders("Authorization")
	r.Use(cors.New(config))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API routes
	api := r.Group("/api/v1")
	{
		// Auth routes
		auth := api.Group("/auth")
		{
			auth.POST("/login", handlers.Login)
			auth.POST("/register", handlers.Register)
			auth.POST("/refresh", handlers.RefreshToken)
		}

		// Protected routes
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			// Users
			users := protected.Group("/users")
			{
				users.GET("/me", handlers.GetCurrentUser)
				users.PUT("/me", handlers.UpdateCurrentUser)
				users.GET("/", handlers.GetUsers)
			}

			// Incidents
			incidents := protected.Group("/incidents")
			{
				incidents.GET("/", handlers.GetIncidents)
				incidents.POST("/", handlers.CreateIncident)
				incidents.GET("/:id", handlers.GetIncident)
				incidents.PUT("/:id", handlers.UpdateIncident)
				incidents.DELETE("/:id", handlers.DeleteIncident)

				// Incident updates
				incidents.GET("/:id/updates", handlers.GetIncidentUpdates)
				incidents.POST("/:id/updates", handlers.CreateIncidentUpdate)
			}
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
