package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Canonical LogEntry JSON schema for log parsing and validation:
//
//	{
//	  "timestamp": string (RFC3339 or similar, required),
//	  "service": string (required),
//	  "host": string (optional),
//	  "environment": string (optional),
//	  "level": string (DEBUG|INFO|WARN|ERROR|FATAL, required),
//	  "error_code": string (optional),
//	  "message": string (required),
//	  "exception": {
//	    "type": string (optional),
//	    "stack_trace": [string] (optional)
//	  },
//	  "context": {
//	    "transaction_id": string (optional),
//	    "user_id": string (optional),
//	    "request": {
//	      "method": string (optional),
//	      "url": string (optional),
//	      "ip": string (optional)
//	    },
//	    "custom_fields": {
//	      "retry_attempt": int (optional),
//	      "database_name": string (optional)
//	    }
//	  },
//	  "tags": [string] (optional),
//	  "correlation_id": string (optional),
//	  "metadata": object (arbitrary JSON, optional)
//	}
//
// Example:
//
//	{
//	  "timestamp": "2024-06-01T12:34:56Z",
//	  "service": "auth-service",
//	  "host": "server-1",
//	  "environment": "production",
//	  "level": "ERROR",
//	  "error_code": "E401",
//	  "message": "Failed to authenticate user",
//	  "exception": { "type": "AuthError", "stack_trace": ["...stack..."] },
//	  "context": {
//	    "transaction_id": "abc123",
//	    "user_id": "user42",
//	    "request": { "method": "POST", "url": "/login", "ip": "1.2.3.4" },
//	    "custom_fields": { "retry_attempt": 1, "database_name": "users" }
//	  },
//	  "tags": ["auth", "login"],
//	  "correlation_id": "corr-xyz",
//	  "metadata": { "extra": "value" }
//	}
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

type Exception struct {
	Type       string   `json:"type" gorm:"type:text"`
	StackTrace []string `json:"stack_trace" gorm:"type:jsonb"`
}

type RequestContext struct {
	Method string `json:"method" gorm:"type:text"`
	Url    string `json:"url" gorm:"type:text"`
	Ip     string `json:"ip" gorm:"type:text"`
}

type CustomFields struct {
	RetryAttempt int    `json:"retry_attempt" gorm:"type:int"`
	DatabaseName string `json:"database_name" gorm:"type:text"`
}

type Context struct {
	TransactionId string         `json:"transaction_id" gorm:"type:text"`
	UserId        string         `json:"user_id" gorm:"type:text"`
	Request       RequestContext `json:"request" gorm:"embedded;embeddedPrefix:request_"`
	CustomFields  CustomFields   `json:"custom_fields" gorm:"embedded;embeddedPrefix:custom_"`
}

type LogEntry struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	LogFileID     uint           `json:"logFileId" gorm:"not null;index"`
	Timestamp     time.Time      `json:"timestamp" gorm:"not null"`
	Service       string         `json:"service" gorm:"type:text"`
	Host          string         `json:"host" gorm:"type:text"`
	Environment   string         `json:"environment" gorm:"type:text"`
	Level         string         `json:"level" gorm:"type:text"`
	ErrorCode     string         `json:"error_code" gorm:"type:text"`
	Message       string         `json:"message" gorm:"type:text"`
	Exception     Exception      `json:"exception" gorm:"embedded;embeddedPrefix:exception_"`
	Context       Context        `json:"context" gorm:"embedded;embeddedPrefix:context_"`
	Tags          []string       `json:"tags" gorm:"type:jsonb"`
	CorrelationId string         `json:"correlation_id" gorm:"type:text"`
	Metadata      JSONB          `json:"metadata" gorm:"type:jsonb"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
	LogFile       *LogFile       `json:"logFile,omitempty" gorm:"foreignKey:LogFileID"`
}

type LogFile struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	Filename   string `json:"filename" gorm:"not null"`
	Size       int64  `json:"size"`
	UploadedBy uint   `json:"uploadedBy" gorm:"not null"`
	// Uploader     User           `json:"uploader" gorm:"foreignKey:UploadedBy"` // Temporarily disabled
	Status               string         `json:"status" gorm:"default:'pending'"` // pending, processing, completed, failed
	EntryCount           int            `json:"entryCount" gorm:"default:0"`
	ErrorCount           int            `json:"errorCount" gorm:"default:0"`
	WarningCount         int            `json:"warningCount" gorm:"default:0"`
	ProcessedAt          *time.Time     `json:"processedAt"`
	RCAAnalysisStatus    string         `json:"rcaAnalysisStatus" gorm:"default:'not_started'"` // not_started, pending, running, completed, failed
	RCAAnalysisJobID     *uint          `json:"rcaAnalysisJobId"`                               // Just a pointer, no FK constraint
	IsRCAPossible        bool           `json:"isRCAPossible" gorm:"default:true"`              // Whether RCA analysis is possible/useful
	RCANotPossibleReason string         `json:"rcaNotPossibleReason" gorm:"type:text"`          // Reason why RCA is not possible
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
	DeletedAt            gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	Entries        []LogEntry `json:"entries,omitempty" gorm:"foreignKey:LogFileID"`
	RCAAnalysisJob *Job       `json:"rcaAnalysisJob,omitempty" gorm:"-"` // No DB-level FK constraint

	// ParseError stores error details if log parsing fails
	ParseError string `json:"parse_error,omitempty" gorm:"type:text"`
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
