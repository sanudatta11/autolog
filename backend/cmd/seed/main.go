package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/autolog/backend/internal/database"
	"github.com/autolog/backend/internal/models"
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

// JSONData represents the structure of the JSON files
type JSONData struct {
	Users []UserData `json:"users"`
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
