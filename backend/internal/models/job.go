package models

import (
	"time"

	"gorm.io/gorm"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type Job struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Type        string         `json:"type" gorm:"not null"`            // "rca_analysis"
	LogFileID   uint           `json:"logFileId" gorm:"not null;index"` // Foreign key to LogFile
	Status      JobStatus      `json:"status" gorm:"not null;default:'pending'"`
	Progress    int            `json:"progress" gorm:"default:0"`
	Result      JSONB          `json:"result" gorm:"type:jsonb"`
	Error       string         `json:"error"`
	StartedAt   *time.Time     `json:"startedAt"`
	CompletedAt *time.Time     `json:"completedAt"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	TotalChunks  int `json:"totalChunks" gorm:"default:0"`
	FailedChunk  int `json:"failedChunk" gorm:"default:0"`
	CurrentChunk int `json:"currentChunk" gorm:"default:0"`

	// Relationship (optional, not a DB constraint)
	LogFile *LogFile `json:"logFile,omitempty" gorm:"foreignKey:LogFileID;references:ID"`
}

func (Job) TableName() string {
	return "jobs"
}
