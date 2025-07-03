package main

import (
	"log"

	"github.com/incident-sage/backend/internal/database"
	"github.com/incident-sage/backend/internal/models"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Connect to database
	database.Connect()

	// Run migrations first
	log.Println("Running database migrations...")
	database.AutoMigrate()

	// Seed database with sample data
	log.Println("Seeding database with sample data...")

	// Create admin user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	adminUser := models.User{
		Email:     "admin@incidentsage.com",
		Password:  string(hashedPassword),
		FirstName: "Admin",
		LastName:  "User",
		Role:      models.RoleAdmin,
	}

	// Create manager user
	managerPassword, _ := bcrypt.GenerateFromPassword([]byte("manager123"), bcrypt.DefaultCost)
	managerUser := models.User{
		Email:     "manager@incidentsage.com",
		Password:  string(managerPassword),
		FirstName: "John",
		LastName:  "Manager",
		Role:      models.RoleManager,
	}

	// Create responder user
	responderPassword, _ := bcrypt.GenerateFromPassword([]byte("responder123"), bcrypt.DefaultCost)
	responderUser := models.User{
		Email:     "responder@incidentsage.com",
		Password:  string(responderPassword),
		FirstName: "Sarah",
		LastName:  "Responder",
		Role:      models.RoleResponder,
	}

	// Create users
	users := []models.User{adminUser, managerUser, responderUser}
	for _, user := range users {
		var existingUser models.User
		if err := database.DB.Where("email = ?", user.Email).First(&existingUser).Error; err != nil {
			if err := database.DB.Create(&user).Error; err != nil {
				log.Printf("Error creating user %s: %v", user.Email, err)
			} else {
				log.Printf("✅ Created user: %s (%s)", user.Email, user.Role)
			}
		} else {
			log.Printf("⚠️  User already exists: %s", user.Email)
		}
	}

	// Create sample incidents
	sampleIncidents := []models.Incident{
		{
			Title:       "Server Outage - Production Environment",
			Description: "Production servers are experiencing high latency and intermittent connectivity issues. Multiple users are reporting slow response times.",
			Status:      models.StatusOpen,
			Priority:    models.PriorityHigh,
			Severity:    models.SeverityMajor,
			ReporterID:  1, // Admin user
			Tags:        []string{"server", "production", "latency"},
		},
		{
			Title:       "Database Connection Pool Exhausted",
			Description: "Application is unable to establish new database connections. Connection pool has reached maximum capacity.",
			Status:      models.StatusInProgress,
			Priority:    models.PriorityCritical,
			Severity:    models.SeverityCritical,
			ReporterID:  2,             // Manager user
			AssigneeID:  &[]uint{3}[0], // Assign to responder
			Tags:        []string{"database", "connection", "pool"},
		},
		{
			Title:       "Security Alert - Failed Login Attempts",
			Description: "Multiple failed login attempts detected from suspicious IP addresses. Potential brute force attack.",
			Status:      models.StatusResolved,
			Priority:    models.PriorityMedium,
			Severity:    models.SeverityModerate,
			ReporterID:  1,             // Admin user
			AssigneeID:  &[]uint{2}[0], // Assign to manager
			Tags:        []string{"security", "authentication", "brute-force"},
		},
	}

	// Create incidents
	for _, incident := range sampleIncidents {
		var existingIncident models.Incident
		if err := database.DB.Where("title = ?", incident.Title).First(&existingIncident).Error; err != nil {
			if err := database.DB.Create(&incident).Error; err != nil {
				log.Printf("Error creating incident %s: %v", incident.Title, err)
			} else {
				log.Printf("✅ Created incident: %s (%s)", incident.Title, incident.Status)
			}
		} else {
			log.Printf("⚠️  Incident already exists: %s", incident.Title)
		}
	}

	log.Println("✅ Database seeding completed successfully!")
	log.Println("")
	log.Println("📋 Sample Users Created:")
	log.Println("  Admin: admin@incidentsage.com / admin123")
	log.Println("  Manager: manager@incidentsage.com / manager123")
	log.Println("  Responder: responder@incidentsage.com / responder123")
	log.Println("")
	log.Println("🚨 Sample Incidents Created:")
	log.Println("  - Server Outage (Open)")
	log.Println("  - Database Connection Pool (In Progress)")
	log.Println("  - Security Alert (Resolved)")
}
