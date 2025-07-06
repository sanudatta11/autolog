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

	// Enhanced fields for feedback processing
	FeedbackType     string     `json:"feedbackType" gorm:"default:'general'"` // 'general', 'root_cause', 'pattern', 'severity'
	PatternName      *string    `json:"patternName"`                           // If feedback is about a specific pattern
	RootCauseSection *string    `json:"rootCauseSection"`                      // Which part of root cause was corrected
	ConfidenceImpact float64    `json:"confidenceImpact" gorm:"default:0.0"`   // How much this feedback should impact confidence
	Processed        bool       `json:"processed" gorm:"default:false"`        // Whether this feedback has been processed for learning
	ProcessedAt      *time.Time `json:"processedAt"`                           // When feedback was processed
}

// FeedbackInsight represents aggregated feedback insights
type FeedbackInsight struct {
	PatternName      string    `json:"patternName"`
	RootCause        string    `json:"rootCause"`
	PositiveFeedback int       `json:"positiveFeedback"`
	NegativeFeedback int       `json:"negativeFeedback"`
	Corrections      []string  `json:"corrections"`
	ConfidenceScore  float64   `json:"confidenceScore"`
	LastUpdated      time.Time `json:"lastUpdated"`
}
