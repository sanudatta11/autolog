package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/incident-sage/backend/internal/database"
	"github.com/incident-sage/backend/internal/models"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

// UserData represents the structure of users in the JSON file
type UserData struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      string `json:"role"`
}

// IncidentData represents the structure of incidents in the JSON file
type IncidentData struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Status        string   `json:"status"`
	Priority      string   `json:"priority"`
	Severity      string   `json:"severity"`
	ReporterEmail string   `json:"reporterEmail"`
	AssigneeEmail *string  `json:"assigneeEmail"`
	Tags          []string `json:"tags"`
}

// JSONData represents the structure of the JSON files
type JSONData struct {
	Users     []UserData     `json:"users"`
	Incidents []IncidentData `json:"incidents"`
}

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

	// Load and create users from JSON
	if err := seedUsers(); err != nil {
		log.Printf("Error seeding users: %v", err)
	}

	// Load and create incidents from JSON
	if err := seedIncidents(); err != nil {
		log.Printf("Error seeding incidents: %v", err)
	}

	log.Println("✅ Database seeding completed successfully!")
}

func seedUsers() error {
	// Read users JSON file
	usersData, err := os.ReadFile("data/initial-users.json")
	if err != nil {
		return err
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

		// Map role string to Role enum
		var role models.UserRole
		switch userData.Role {
		case "admin":
			role = models.RoleAdmin
		case "manager":
			role = models.RoleManager
		case "responder":
			role = models.RoleResponder
		case "viewer":
			role = models.RoleViewer
		default:
			log.Printf("Unknown role %s for user %s, defaulting to viewer", userData.Role, userData.Email)
			role = models.RoleViewer
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

	return nil
}

func seedIncidents() error {
	// Read incidents JSON file
	incidentsData, err := os.ReadFile("data/initial-incidents.json")
	if err != nil {
		return err
	}

	var jsonData JSONData
	if err := json.Unmarshal(incidentsData, &jsonData); err != nil {
		return err
	}

	// Create incidents
	for _, incidentData := range jsonData.Incidents {
		// Get reporter user ID
		var reporter models.User
		if err := database.DB.Where("email = ?", incidentData.ReporterEmail).First(&reporter).Error; err != nil {
			log.Printf("Error finding reporter %s: %v", incidentData.ReporterEmail, err)
			continue
		}

		// Get assignee user ID if specified
		var assigneeID *uint
		if incidentData.AssigneeEmail != nil {
			var assignee models.User
			if err := database.DB.Where("email = ?", *incidentData.AssigneeEmail).First(&assignee).Error; err != nil {
				log.Printf("Error finding assignee %s: %v", *incidentData.AssigneeEmail, err)
			} else {
				assigneeID = &assignee.ID
			}
		}

		// Map status string to Status enum
		var status models.IncidentStatus
		switch incidentData.Status {
		case "open":
			status = models.StatusOpen
		case "in_progress":
			status = models.StatusInProgress
		case "resolved":
			status = models.StatusResolved
		case "closed":
			status = models.StatusClosed
		default:
			log.Printf("Unknown status %s, defaulting to open", incidentData.Status)
			status = models.StatusOpen
		}

		// Map priority string to Priority enum
		var priority models.IncidentPriority
		switch incidentData.Priority {
		case "low":
			priority = models.PriorityLow
		case "medium":
			priority = models.PriorityMedium
		case "high":
			priority = models.PriorityHigh
		case "critical":
			priority = models.PriorityCritical
		default:
			log.Printf("Unknown priority %s, defaulting to medium", incidentData.Priority)
			priority = models.PriorityMedium
		}

		// Map severity string to Severity enum
		var severity models.IncidentSeverity
		switch incidentData.Severity {
		case "minor":
			severity = models.SeverityMinor
		case "moderate":
			severity = models.SeverityModerate
		case "major":
			severity = models.SeverityMajor
		case "critical":
			severity = models.SeverityCritical
		default:
			log.Printf("Unknown severity %s, defaulting to moderate", incidentData.Severity)
			severity = models.SeverityModerate
		}

		incident := models.Incident{
			Title:       incidentData.Title,
			Description: incidentData.Description,
			Status:      status,
			Priority:    priority,
			Severity:    severity,
			ReporterID:  reporter.ID,
			AssigneeID:  assigneeID,
			Tags:        incidentData.Tags,
		}

		// Check if incident already exists
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

	return nil
}
