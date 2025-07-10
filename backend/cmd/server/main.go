package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/autolog/backend/internal/db"
	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/middleware"
	"github.com/autolog/backend/internal/models"
	"github.com/autolog/backend/internal/routes"

	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set CORS headers for all requests
		origin := "http://localhost:5173"
		if os.Getenv("ENV") != "local" && os.Getenv("ENV") != "" {
			if corsOrigin := os.Getenv("CORS_ORIGIN"); corsOrigin != "" {
				origin = corsOrigin
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight request
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

func main() {
	// Initialize logger first
	logger.Initialize()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, using environment variables", nil)
	}

	// Connect to database
	db.Connect()
	db.AutoMigrate()

	// Setup graceful shutdown
	stopChan := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		logger.Warn("Received shutdown signal, stopping background workers...", nil)
		close(stopChan)
	}()

	// Seed database with initial data if in development
	if os.Getenv("ENV") == "development" {
		log.Println("🌱 Seeding database with initial data...")
		if err := seedDatabase(); err != nil {
			log.Printf("Warning: Failed to seed database: %v", err)
		}
	}

	// Set Gin mode
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router without default middleware
	r := gin.New()

	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false

	// Use our custom logging middleware instead of gin.Default()
	r.Use(middleware.CustomLoggerMiddleware())
	r.Use(CORSMiddleware())
	r.Use(gin.Recovery())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		// Check database connectivity
		var dbStatus string
		var dbError error

		if db.DB != nil {
			sqlDB, err := db.DB.DB()
			if err != nil {
				dbStatus = "error"
				dbError = err
			} else {
				err = sqlDB.Ping()
				if err != nil {
					dbStatus = "error"
					dbError = err
				} else {
					dbStatus = "ok"
				}
			}
		} else {
			dbStatus = "error"
			dbError = fmt.Errorf("database connection not initialized")
		}

		// Determine overall health
		overallStatus := "ok"
		statusCode := 200

		if dbStatus != "ok" {
			overallStatus = "error"
			statusCode = 503
		}

		response := gin.H{
			"status":    overallStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
			"services": gin.H{
				"database": gin.H{
					"status": dbStatus,
					"error":  dbError,
				},
			},
		}

		c.JSON(statusCode, response)
	})

	// Setup routes
	routes.SetupRoutes(r, db.DB, stopChan)

	// Start server with graceful shutdown
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	logger.Info("Starting AutoLog backend server", map[string]interface{}{
		"port":     port,
		"gin_mode": gin.Mode(),
	})

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Wait for shutdown signal
	<-stopChan
	logger.Info("Shutting down server gracefully...", nil)

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		logger.Info("Server exited gracefully", nil)
	}
}

// UserData represents the structure of users in the JSON file
type UserData struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      string `json:"role"`
}

// JSONData represents the structure of the JSON files
type JSONData struct {
	Users []UserData `json:"users"`
}

func seedDatabase() error {
	// Load and create users from JSON
	if err := seedUsers(); err != nil {
		return err
	}

	log.Println("✅ Database seeding completed successfully!")
	return nil
}

func seedUsers() error {
	// Read users JSON file
	log.Println("🔍 Looking for users file at ../../data/initial-users.json")
	usersData, err := os.ReadFile("../../data/initial-users.json")
	if err != nil {
		log.Printf("⚠️ First path failed: %v", err)
		// Try alternative path
		log.Println("🔍 Looking for users file at data/initial-users.json")
		usersData, err = os.ReadFile("data/initial-users.json")
		if err != nil {
			log.Printf("❌ Both paths failed: %v", err)
			return fmt.Errorf("failed to read users file: %w", err)
		}
		log.Println("✅ Found users file at data/initial-users.json")
	} else {
		log.Println("✅ Found users file at ../../data/initial-users.json")
	}

	var jsonData JSONData
	if err := json.Unmarshal(usersData, &jsonData); err != nil {
		return err
	}

	// Create users
	for _, userData := range jsonData.Users {
		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userData.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Error hashing password for %s: %v", userData.Email, err)
			continue
		}

		// Map role string to string
		var role string
		switch userData.Role {
		case "admin":
			role = "ADMIN"
		case "manager":
			role = "MANAGER"
		case "responder":
			role = "RESPONDER"
		case "viewer":
			role = "VIEWER"
		default:
			log.Printf("Unknown role %s for user %s, defaulting to viewer", userData.Role, userData.Email)
			role = "VIEWER"
		}

		user := models.User{
			Email:     userData.Email,
			Password:  string(hashedPassword),
			FirstName: userData.FirstName,
			LastName:  userData.LastName,
			Role:      role,
		}

		// Check if user already exists
		var existingUser models.User
		if err := db.DB.Where("email = ?", user.Email).First(&existingUser).Error; err != nil {
			if err := db.DB.Create(&user).Error; err != nil {
				log.Printf("Error creating user %s: %v", user.Email, err)
			} else {
				log.Printf("✅ Created user: %s (%s)", user.Email, user.Role)
			}
		} else {
			log.Printf("⚠️  User already exists: %s", user.Email)
		}
	}

	return nil
}
