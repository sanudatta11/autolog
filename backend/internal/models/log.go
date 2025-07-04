package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// JSONB is a custom type for handling JSONB columns in GORM
// It implements the Scanner and Valuer interfaces
// so GORM can marshal/unmarshal JSON automatically
type JSONB map[string]interface{}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("Failed to unmarshal JSONB value: %v", value)
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

type LogLevel string

const (
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarning LogLevel = "WARN"
	LogLevelError   LogLevel = "ERROR"
	LogLevelFatal   LogLevel = "FATAL"
)

type LogEntry struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	LogFileID uint           `json:"logFileId" gorm:"not null"`
	Timestamp time.Time      `json:"timestamp"`
	Level     LogLevel       `json:"level"`
	Message   string         `json:"message" gorm:"type:text"`
	RawData   string         `json:"rawData" gorm:"type:text"`
	Metadata  JSONB          `json:"metadata" gorm:"type:jsonb"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type LogFile struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	Filename   string `json:"filename" gorm:"not null"`
	Size       int64  `json:"size"`
	UploadedBy uint   `json:"uploadedBy" gorm:"not null"`
	// Uploader     User           `json:"uploader" gorm:"foreignKey:UploadedBy"` // Temporarily disabled
	Status            string         `json:"status" gorm:"default:'pending'"` // pending, processing, completed, failed
	EntryCount        int            `json:"entryCount" gorm:"default:0"`
	ErrorCount        int            `json:"errorCount" gorm:"default:0"`
	WarningCount      int            `json:"warningCount" gorm:"default:0"`
	ProcessedAt       *time.Time     `json:"processedAt"`
	RCAAnalysisStatus string         `json:"rcaAnalysisStatus" gorm:"default:'not_started'"` // not_started, pending, running, completed, failed
	RCAAnalysisJobID  *uint          `json:"rcaAnalysisJobId"`                               // Just a pointer, no FK constraint
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	Entries        []LogEntry `json:"entries,omitempty" gorm:"foreignKey:LogFileID"`
	RCAAnalysisJob *Job       `json:"rcaAnalysisJob,omitempty" gorm:"-"` // No DB-level FK constraint
}

type LogAnalysis struct {
	ID        uint    `json:"id" gorm:"primaryKey"`
	LogFileID uint    `json:"logFileId" gorm:"not null"`
	LogFile   LogFile `json:"logFile" gorm:"foreignKey:LogFileID"`

	StartTime    time.Time      `json:"startTime"`
	EndTime      time.Time      `json:"endTime"`
	Summary      string         `json:"summary" gorm:"type:text"`
	Severity     string         `json:"severity"` // low, medium, high, critical
	ErrorCount   int            `json:"errorCount"`
	WarningCount int            `json:"warningCount"`
	Metadata     JSONB          `json:"metadata" gorm:"type:jsonb"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (LogEntry) TableName() string {
	return "log_entries"
}

func (LogFile) TableName() string {
	return "log_files"
}

func (LogAnalysis) TableName() string {
	return "log_analyses"
}
