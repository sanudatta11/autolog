package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/autolog/backend/internal/models"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ensureDatabaseExists() error {
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	port := os.Getenv("DB_PORT")
	sslmode := os.Getenv("DB_SSLMODE")
	dbName := os.Getenv("DB_NAME")

	// Connect to the default 'postgres' database
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=postgres port=%s sslmode=%s",
		host, user, password, port, sslmode)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	// Check if the database exists
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	if err := db.QueryRow(query, dbName).Scan(&exists); err != nil {
		return err
	}

	if !exists {
		// Create the database
		_, err = db.Exec("CREATE DATABASE " + dbName)
		if err != nil {
			return err
		}
		log.Printf("✅ Database %s created", dbName)
	} else {
		log.Printf("Database %s already exists", dbName)
	}
	return nil
}

// Connect initializes the database connection
func Connect() {
	if err := ensureDatabaseExists(); err != nil {
		log.Fatalf("Failed to ensure database exists: %v", err)
	}

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

	// Migrate Job model FIRST to break circular dependency
	log.Println("Testing migration with Job model...")
	err = DB.AutoMigrate(&models.Job{})
	if err != nil {
		log.Printf("Job migration failed: %v", err)
		return
	}
	log.Println("✅ Job table migrated successfully")

	// Then add LogFile
	log.Println("Testing migration with LogFile model...")
	err = DB.AutoMigrate(&models.LogFile{}) // Includes ParseError field
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

	// Add parsing rule models
	log.Println("Testing migration with ParsingRule model...")
	err = DB.AutoMigrate(&models.ParsingRule{})
	if err != nil {
		log.Printf("ParsingRule migration failed: %v", err)
		return
	}
	log.Println("✅ ParsingRule table migrated successfully")

	log.Println("Testing migration with FieldMapping model...")
	err = DB.AutoMigrate(&models.FieldMapping{})
	if err != nil {
		log.Printf("FieldMapping migration failed: %v", err)
		return
	}
	log.Println("✅ FieldMapping table migrated successfully")

	log.Println("Testing migration with RegexPattern model...")
	err = DB.AutoMigrate(&models.RegexPattern{})
	if err != nil {
		log.Printf("RegexPattern migration failed: %v", err)
		return
	}
	log.Println("✅ RegexPattern table migrated successfully")

	log.Println("Testing migration with ParsingRuleTemplate model...")
	err = DB.AutoMigrate(&models.ParsingRuleTemplate{})
	if err != nil {
		log.Printf("ParsingRuleTemplate migration failed: %v", err)
		return
	}
	log.Println("✅ ParsingRuleTemplate table migrated successfully")

	log.Println("Testing migration with ParsingRuleUsage model...")
	err = DB.AutoMigrate(&models.ParsingRuleUsage{})
	if err != nil {
		log.Printf("ParsingRuleUsage migration failed: %v", err)
		return
	}
	log.Println("✅ ParsingRuleUsage table migrated successfully")

	// Add pattern models for learning functionality
	log.Println("Testing migration with Pattern model...")
	err = DB.AutoMigrate(&models.Pattern{})
	if err != nil {
		log.Printf("Pattern migration failed: %v", err)
		return
	}
	log.Println("✅ Pattern table migrated successfully")

	log.Println("Testing migration with PatternExample model...")
	err = DB.AutoMigrate(&models.PatternExample{})
	if err != nil {
		log.Printf("PatternExample migration failed: %v", err)
		return
	}
	log.Println("✅ PatternExample table migrated successfully")

	log.Println("✅ All database migrations completed successfully")
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}
