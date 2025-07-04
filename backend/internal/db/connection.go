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
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	log.Println("✅ Database connected successfully")
}

// AutoMigrate runs database migrations
func AutoMigrate() {
	err := DB.AutoMigrate(
		&models.User{},
		&models.LogFile{},
		&models.LogEntry{},
		&models.LogAnalysis{},
	)

	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	log.Println("✅ Database migrations completed")
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}
