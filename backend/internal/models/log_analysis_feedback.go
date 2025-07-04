package models

import (
	"time"
)

type LogAnalysisFeedback struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	AnalysisMemoryID uint      `json:"analysisMemoryId"`
	UserID           *uint     `json:"userId"`
	IsCorrect        bool      `json:"isCorrect"`
	Correction       string    `json:"correction"`
	CreatedAt        time.Time `json:"createdAt"`
}
