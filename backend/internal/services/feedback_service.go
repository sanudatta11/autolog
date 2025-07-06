package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"gorm.io/gorm"
)

// FeedbackService handles feedback processing and retrieval for RCA analysis
type FeedbackService struct {
	db *gorm.DB
}

// NewFeedbackService creates a new feedback service
func NewFeedbackService(db *gorm.DB) *FeedbackService {
	return &FeedbackService{
		db: db,
	}
}

// GetFeedbackForSimilarIncidents retrieves feedback for similar past incidents
func (fs *FeedbackService) GetFeedbackForSimilarIncidents(similarIncidents []SimilarIncident) ([]models.LogAnalysisFeedback, error) {
	if len(similarIncidents) == 0 {
		return []models.LogAnalysisFeedback{}, nil
	}

	// Extract analysis memory IDs from similar incidents
	var memoryIDs []uint
	for _, incident := range similarIncidents {
		// Find the LogAnalysisMemory for this incident
		var memory models.LogAnalysisMemory
		if err := fs.db.Where("log_file_id = ?", incident.LogFileID).First(&memory).Error; err != nil {
			logger.Warn("Failed to find analysis memory for incident", map[string]interface{}{
				"logFileID": incident.LogFileID,
				"error":     err.Error(),
			})
			continue
		}
		memoryIDs = append(memoryIDs, memory.ID)
	}

	if len(memoryIDs) == 0 {
		return []models.LogAnalysisFeedback{}, nil
	}

	// Fetch feedback for these analysis memories
	var feedbacks []models.LogAnalysisFeedback
	if err := fs.db.Where("analysis_memory_id IN ?", memoryIDs).Find(&feedbacks).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch feedback: %w", err)
	}

	logger.Info("Retrieved feedback for similar incidents", map[string]interface{}{
		"similarIncidents": len(similarIncidents),
		"feedbacksFound":   len(feedbacks),
	})

	return feedbacks, nil
}

// GetFeedbackInsights aggregates feedback insights for patterns and root causes
func (fs *FeedbackService) GetFeedbackInsights() ([]models.FeedbackInsight, error) {
	var feedbacks []models.LogAnalysisFeedback
	if err := fs.db.Find(&feedbacks).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch all feedback: %w", err)
	}

	// Group feedback by pattern and root cause
	insightsMap := make(map[string]*models.FeedbackInsight)

	for _, feedback := range feedbacks {
		// Create a key for grouping (pattern + root cause)
		key := fs.createFeedbackKey(feedback)

		if insight, exists := insightsMap[key]; exists {
			// Update existing insight
			if feedback.IsCorrect {
				insight.PositiveFeedback++
			} else {
				insight.NegativeFeedback++
				if feedback.Correction != "" {
					insight.Corrections = append(insight.Corrections, feedback.Correction)
				}
			}
			insight.ConfidenceScore = fs.calculateConfidenceScore(insight.PositiveFeedback, insight.NegativeFeedback)
			insight.LastUpdated = time.Now()
		} else {
			// Create new insight
			insight := &models.FeedbackInsight{
				PatternName:      fs.getPatternName(feedback),
				RootCause:        fs.getRootCause(feedback),
				PositiveFeedback: 0,
				NegativeFeedback: 0,
				Corrections:      []string{},
				ConfidenceScore:  0.0,
				LastUpdated:      time.Now(),
			}

			if feedback.IsCorrect {
				insight.PositiveFeedback = 1
			} else {
				insight.NegativeFeedback = 1
				if feedback.Correction != "" {
					insight.Corrections = []string{feedback.Correction}
				}
			}

			insight.ConfidenceScore = fs.calculateConfidenceScore(insight.PositiveFeedback, insight.NegativeFeedback)
			insightsMap[key] = insight
		}
	}

	// Convert map to slice
	var insights []models.FeedbackInsight
	for _, insight := range insightsMap {
		insights = append(insights, *insight)
	}

	return insights, nil
}

// GetFeedbackForPattern retrieves feedback for a specific pattern
func (fs *FeedbackService) GetFeedbackForPattern(patternName string) ([]models.LogAnalysisFeedback, error) {
	var feedbacks []models.LogAnalysisFeedback
	if err := fs.db.Where("pattern_name = ? OR correction LIKE ?", patternName, "%"+patternName+"%").Find(&feedbacks).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch feedback for pattern: %w", err)
	}
	return feedbacks, nil
}

// ProcessFeedbackForLearning marks feedback as processed and updates learning metrics
func (fs *FeedbackService) ProcessFeedbackForLearning(feedbackID uint) error {
	now := time.Now()
	if err := fs.db.Model(&models.LogAnalysisFeedback{}).Where("id = ?", feedbackID).Updates(map[string]interface{}{
		"processed":    true,
		"processed_at": &now,
	}).Error; err != nil {
		return fmt.Errorf("failed to mark feedback as processed: %w", err)
	}
	return nil
}

// GetFeedbackContext generates context string for LLM prompts based on feedback
func (fs *FeedbackService) GetFeedbackContext(similarIncidents []SimilarIncident, patternMatches []PatternMatch) string {
	var context strings.Builder

	// Get feedback for similar incidents
	feedbacks, err := fs.GetFeedbackForSimilarIncidents(similarIncidents)
	if err != nil {
		logger.Warn("Failed to get feedback for similar incidents", map[string]interface{}{
			"error": err.Error(),
		})
		return ""
	}

	if len(feedbacks) > 0 {
		context.WriteString("USER FEEDBACK FROM SIMILAR INCIDENTS:\n")

		positiveCount := 0
		negativeCount := 0
		var corrections []string

		for _, feedback := range feedbacks {
			if feedback.IsCorrect {
				positiveCount++
			} else {
				negativeCount++
				if feedback.Correction != "" {
					corrections = append(corrections, feedback.Correction)
				}
			}
		}

		context.WriteString(fmt.Sprintf("- Positive feedback: %d\n", positiveCount))
		context.WriteString(fmt.Sprintf("- Negative feedback: %d\n", negativeCount))

		if len(corrections) > 0 {
			context.WriteString("- User corrections:\n")
			for i, correction := range corrections {
				if i >= 3 { // Limit to top 3 corrections
					break
				}
				context.WriteString(fmt.Sprintf("  * %s\n", correction))
			}
		}
		context.WriteString("\n")
	}

	// Get feedback for pattern matches
	for _, match := range patternMatches {
		patternFeedbacks, err := fs.GetFeedbackForPattern(match.Pattern.Name)
		if err != nil {
			continue
		}

		if len(patternFeedbacks) > 0 {
			positiveCount := 0
			negativeCount := 0
			for _, feedback := range patternFeedbacks {
				if feedback.IsCorrect {
					positiveCount++
				} else {
					negativeCount++
				}
			}

			context.WriteString(fmt.Sprintf("PATTERN '%s' FEEDBACK:\n", match.Pattern.Name))
			context.WriteString(fmt.Sprintf("- Positive: %d, Negative: %d\n", positiveCount, negativeCount))

			// Adjust confidence based on feedback
			if negativeCount > positiveCount {
				context.WriteString("- WARNING: This pattern has received negative feedback\n")
			} else if positiveCount > 0 {
				context.WriteString("- This pattern has received positive feedback\n")
			}
			context.WriteString("\n")
		}
	}

	return context.String()
}

// AdjustConfidenceBasedOnFeedback adjusts pattern confidence based on user feedback
func (fs *FeedbackService) AdjustConfidenceBasedOnFeedback(patternName string, baseConfidence float64) float64 {
	feedbacks, err := fs.GetFeedbackForPattern(patternName)
	if err != nil {
		return baseConfidence
	}

	if len(feedbacks) == 0 {
		return baseConfidence
	}

	positiveCount := 0
	negativeCount := 0

	for _, feedback := range feedbacks {
		if feedback.IsCorrect {
			positiveCount++
		} else {
			negativeCount++
		}
	}

	total := positiveCount + negativeCount
	if total == 0 {
		return baseConfidence
	}

	feedbackRatio := float64(positiveCount) / float64(total)

	// Adjust confidence: positive feedback boosts, negative feedback reduces
	adjustment := (feedbackRatio - 0.5) * 0.3 // Max ±15% adjustment
	adjustedConfidence := baseConfidence + adjustment

	// Clamp between 0.1 and 1.0
	if adjustedConfidence < 0.1 {
		adjustedConfidence = 0.1
	} else if adjustedConfidence > 1.0 {
		adjustedConfidence = 1.0
	}

	return adjustedConfidence
}

// Helper methods
func (fs *FeedbackService) createFeedbackKey(feedback models.LogAnalysisFeedback) string {
	pattern := fs.getPatternName(feedback)
	rootCause := fs.getRootCause(feedback)
	return fmt.Sprintf("%s|%s", pattern, rootCause)
}

func (fs *FeedbackService) getPatternName(feedback models.LogAnalysisFeedback) string {
	if feedback.PatternName != nil {
		return *feedback.PatternName
	}
	return "general"
}

func (fs *FeedbackService) getRootCause(feedback models.LogAnalysisFeedback) string {
	if feedback.RootCauseSection != nil {
		return *feedback.RootCauseSection
	}
	return "general"
}

func (fs *FeedbackService) calculateConfidenceScore(positive, negative int) float64 {
	total := positive + negative
	if total == 0 {
		return 0.5 // Neutral confidence
	}

	ratio := float64(positive) / float64(total)
	// Apply sigmoid-like function for smoother confidence
	return 0.5 + (ratio-0.5)*0.8
}
