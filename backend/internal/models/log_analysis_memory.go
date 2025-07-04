package models

import (
	"time"
)

type LogAnalysisMemory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	LogFileID *uint     `json:"logFileId"`
	Summary   string    `json:"summary"`
	RootCause string    `json:"rootCause"`
	Embedding JSONB     `json:"embedding" gorm:"type:jsonb"` // Store as JSON array of floats
	Metadata  JSONB     `json:"metadata" gorm:"type:jsonb"`
	CreatedAt time.Time `json:"createdAt"`
}
