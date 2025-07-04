package db

import (
	"fmt"
	"log"
	"os"

	"github.com/autolog/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Connect initializes the database connection
func Connect() {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_SSLMODE"),
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error), // Reduce logging to avoid issues
	})

	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	log.Println("✅ Database connected successfully")
}

// AutoMigrate runs database migrations
func AutoMigrate() {
	// Migrate User model
	log.Println("Testing migration with User model...")
	err := DB.AutoMigrate(&models.User{})
	if err != nil {
		log.Printf("User migration failed: %v", err)
		return
	}
	log.Println("✅ User table migrated successfully")

	// Then add LogFile
	log.Println("Testing migration with LogFile model...")
	err = DB.AutoMigrate(&models.LogFile{})
	if err != nil {
		log.Printf("LogFile migration failed: %v", err)
		return
	}
	log.Println("✅ LogFile table migrated successfully")

	// Then add LogEntry
	log.Println("Testing migration with LogEntry model...")
	err = DB.AutoMigrate(&models.LogEntry{})
	if err != nil {
		log.Printf("LogEntry migration failed: %v", err)
		return
	}
	log.Println("✅ LogEntry table migrated successfully")

	// Then add LogAnalysis
	log.Println("Testing migration with LogAnalysis model...")
	err = DB.AutoMigrate(&models.LogAnalysis{})
	if err != nil {
		log.Printf("LogAnalysis migration failed: %v", err)
		return
	}
	log.Println("✅ LogAnalysis table migrated successfully")

	// Add the new models for feedback functionality
	log.Println("Testing migration with LogAnalysisMemory model...")
	err = DB.AutoMigrate(&models.LogAnalysisMemory{})
	if err != nil {
		log.Printf("LogAnalysisMemory migration failed: %v", err)
		return
	}
	log.Println("✅ LogAnalysisMemory table migrated successfully")

	log.Println("Testing migration with LogAnalysisFeedback model...")
	err = DB.AutoMigrate(&models.LogAnalysisFeedback{})
	if err != nil {
		log.Printf("LogAnalysisFeedback migration failed: %v", err)
		return
	}
	log.Println("✅ LogAnalysisFeedback table migrated successfully")

	// Add the new Job model for background processing
	log.Println("Testing migration with Job model...")
	err = DB.AutoMigrate(&models.Job{})
	if err != nil {
		log.Printf("Job migration failed: %v", err)
		return
	}
	log.Println("✅ Job table migrated successfully")

	log.Println("✅ All database migrations completed successfully")
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}
