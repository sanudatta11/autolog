package database

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

func Connect() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	fmt.Println("Database connected successfully")
}

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

	fmt.Println("Database migrated successfully")
}
