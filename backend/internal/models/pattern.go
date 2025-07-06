package models

import (
	"time"

	"gorm.io/gorm"
)

// Pattern represents a learned error pattern from past analyses
type Pattern struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	Name            string         `json:"name" gorm:"uniqueIndex;not null"`
	Description     string         `json:"description" gorm:"type:text"`
	ErrorKeywords   JSONB          `json:"errorKeywords" gorm:"type:jsonb"` // Store as JSON array
	RootCause       string         `json:"rootCause" gorm:"type:text"`
	CommonFixes     JSONB          `json:"commonFixes" gorm:"type:jsonb"` // Store as JSON array
	Severity        string         `json:"severity" gorm:"type:varchar(20)"`
	OccurrenceCount int            `json:"occurrenceCount" gorm:"default:1"`
	LastSeen        time.Time      `json:"lastSeen"`
	Confidence      float64        `json:"confidence" gorm:"default:0.8"`
	Metadata        JSONB          `json:"metadata" gorm:"type:jsonb"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

// PatternExample represents a specific example of a pattern
type PatternExample struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	PatternID  uint           `json:"patternId" gorm:"not null"`
	Pattern    Pattern        `json:"pattern" gorm:"foreignKey:PatternID"`
	LogFileID  uint           `json:"logFileId" gorm:"not null"`
	LogFile    LogFile        `json:"logFile" gorm:"foreignKey:LogFileID"`
	Summary    string         `json:"summary" gorm:"type:text"`
	RootCause  string         `json:"rootCause" gorm:"type:text"`
	Timestamp  time.Time      `json:"timestamp"`
	ErrorCount int            `json:"errorCount"`
	Severity   string         `json:"severity" gorm:"type:varchar(20)"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Pattern) TableName() string {
	return "patterns"
}

func (PatternExample) TableName() string {
	return "pattern_examples"
}
