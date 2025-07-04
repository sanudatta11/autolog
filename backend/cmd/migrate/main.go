package main

import (
	"log"

	"github.com/autolog/backend/internal/db"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Connect to database
	db.Connect()

	// Run migrations
	log.Println("Running database migrations...")
	db.AutoMigrate()

	log.Println("✅ Database migrations completed successfully!")
}
