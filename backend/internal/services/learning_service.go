package services

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"gorm.io/gorm"
)

// LearningService provides comprehensive learning capabilities from RCA analyses
type LearningService struct {
	db                  *gorm.DB
	llmService          *LLMService
	similarityThreshold float64
	maxMemoriesToUse    int
}

// Pattern represents a learned error pattern from past analyses
type Pattern struct {
	ID              uint                   `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	ErrorKeywords   []string               `json:"errorKeywords"`
	RootCause       string                 `json:"rootCause"`
	CommonFixes     []string               `json:"commonFixes"`
	Severity        string                 `json:"severity"`
	OccurrenceCount int                    `json:"occurrenceCount"`
	LastSeen        time.Time              `json:"lastSeen"`
	Examples        []PatternExample       `json:"examples"`
	Confidence      float64                `json:"confidence"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// PatternExample represents a specific example of a pattern
type PatternExample struct {
	LogFileID  uint      `json:"logFileId"`
	Summary    string    `json:"summary"`
	RootCause  string    `json:"rootCause"`
	Timestamp  time.Time `json:"timestamp"`
	ErrorCount int       `json:"errorCount"`
	Severity   string    `json:"severity"`
}

// SimilarIncident represents a similar past incident for context
type SimilarIncident struct {
	LogFileID  uint      `json:"logFileId"`
	Filename   string    `json:"filename"`
	Summary    string    `json:"summary"`
	RootCause  string    `json:"rootCause"`
	Severity   string    `json:"severity"`
	Timestamp  time.Time `json:"timestamp"`
	Similarity float64   `json:"similarity"`
	Relevance  string    `json:"relevance"` // "high", "medium", "low"
}

// LearningInsights provides insights for improving analysis
type LearningInsights struct {
	SimilarIncidents   []SimilarIncident `json:"similarIncidents"`
	IdentifiedPatterns []Pattern         `json:"identifiedPatterns"`
	ConfidenceBoost    float64           `json:"confidenceBoost"`
	SuggestedContext   string            `json:"suggestedContext"`
	PatternMatches     []PatternMatch    `json:"patternMatches"`
	LearningMetrics    LearningMetrics   `json:"learningMetrics"`
}

// PatternMatch represents a match between current errors and known patterns
type PatternMatch struct {
	Pattern     Pattern `json:"pattern"`
	Confidence  float64 `json:"confidence"`
	MatchReason string  `json:"matchReason"`
	Relevance   string  `json:"relevance"`
}

// LearningMetrics tracks learning performance
type LearningMetrics struct {
	TotalAnalyses       int     `json:"totalAnalyses"`
	PatternMatches      int     `json:"patternMatches"`
	AccuracyImprovement float64 `json:"accuracyImprovement"`
	AverageConfidence   float64 `json:"averageConfidence"`
	LearningRate        float64 `json:"learningRate"`
}

// NewLearningService creates a new learning service
func NewLearningService(db *gorm.DB, llmService *LLMService) *LearningService {
	return &LearningService{
		db:                  db,
		llmService:          llmService,
		similarityThreshold: 0.7, // 70% similarity threshold
		maxMemoriesToUse:    5,   // Use top 5 similar memories
	}
}

// LearnFromAnalysis learns from a completed RCA analysis
func (ls *LearningService) LearnFromAnalysis(logFile *models.LogFile, analysis *LogAnalysisResponse) error {
	logger.Info("Learning from RCA analysis", map[string]interface{}{
		"logFileID": logFile.ID,
		"filename":  logFile.Filename,
		"severity":  analysis.Severity,
	})

	// 1. Extract patterns from the analysis
	patterns := ls.extractPatternsFromAnalysis(analysis, logFile)

	// 2. Update or create pattern records
	for _, pattern := range patterns {
		if err := ls.updatePattern(pattern); err != nil {
			logger.Error("Failed to update pattern", map[string]interface{}{
				"pattern": pattern.Name,
				"error":   err,
			})
		}
	}

	// 3. Generate and store embedding for future similarity search
	if err := ls.storeAnalysisEmbedding(logFile.ID, analysis); err != nil {
		logger.Error("Failed to store analysis embedding", map[string]interface{}{
			"logFileID": logFile.ID,
			"error":     err,
		})
	}

	// 4. Update learning metrics
	if err := ls.updateLearningMetrics(analysis); err != nil {
		logger.Error("Failed to update learning metrics", map[string]interface{}{
			"error": err,
		})
	}

	logger.Info("Successfully learned from RCA analysis", map[string]interface{}{
		"logFileID":     logFile.ID,
		"patternsFound": len(patterns),
		"severity":      analysis.Severity,
	})

	return nil
}

// GetLearningInsights provides insights for a new analysis
func (ls *LearningService) GetLearningInsights(logFile *models.LogFile, errorEntries []models.LogEntry) (*LearningInsights, error) {
	logger.Info("Getting learning insights", map[string]interface{}{
		"logFileID":    logFile.ID,
		"errorEntries": len(errorEntries),
	})

	insights := &LearningInsights{
		SimilarIncidents:   []SimilarIncident{},
		IdentifiedPatterns: []Pattern{},
		PatternMatches:     []PatternMatch{},
		ConfidenceBoost:    0.0,
	}

	// 1. Find similar past incidents
	similarIncidents, err := ls.findSimilarIncidents(logFile, errorEntries)
	if err != nil {
		logger.Error("Failed to find similar incidents", map[string]interface{}{
			"error": err,
		})
	} else {
		insights.SimilarIncidents = similarIncidents
	}

	// 2. Identify patterns in current errors
	patternMatches, err := ls.identifyPatterns(errorEntries)
	if err != nil {
		logger.Error("Failed to identify patterns", map[string]interface{}{
			"error": err,
		})
	} else {
		insights.PatternMatches = patternMatches
		insights.IdentifiedPatterns = ls.extractPatternsFromMatches(patternMatches)
	}

	// 3. Calculate confidence boost based on learning
	insights.ConfidenceBoost = ls.calculateConfidenceBoost(insights)

	// 4. Generate suggested context for LLM
	insights.SuggestedContext = ls.generateSuggestedContext(insights)

	// 5. Get learning metrics
	metrics, err := ls.GetLearningMetrics()
	if err != nil {
		logger.Error("Failed to get learning metrics", map[string]interface{}{
			"error": err,
		})
	} else {
		insights.LearningMetrics = *metrics
	}

	logger.Info("Generated learning insights", map[string]interface{}{
		"logFileID":        logFile.ID,
		"similarIncidents": len(insights.SimilarIncidents),
		"patternMatches":   len(insights.PatternMatches),
		"confidenceBoost":  insights.ConfidenceBoost,
	})

	return insights, nil
}

// extractPatternsFromAnalysis extracts patterns from a completed analysis
func (ls *LearningService) extractPatternsFromAnalysis(analysis *LogAnalysisResponse, logFile *models.LogFile) []Pattern {
	var patterns []Pattern

	// Extract patterns from error analysis
	for _, errorAnalysis := range analysis.ErrorAnalysis {
		pattern := Pattern{
			Name:            errorAnalysis.ErrorPattern,
			Description:     fmt.Sprintf("Pattern: %s", errorAnalysis.ErrorPattern),
			ErrorKeywords:   ls.extractKeywords(errorAnalysis.ErrorPattern),
			RootCause:       errorAnalysis.RootCause,
			CommonFixes:     []string{errorAnalysis.Fix},
			Severity:        errorAnalysis.Severity,
			OccurrenceCount: errorAnalysis.ErrorCount,
			LastSeen:        time.Now(),
			Examples: []PatternExample{
				{
					LogFileID:  logFile.ID,
					Summary:    analysis.Summary,
					RootCause:  errorAnalysis.RootCause,
					Timestamp:  time.Now(),
					ErrorCount: errorAnalysis.ErrorCount,
					Severity:   errorAnalysis.Severity,
				},
			},
			Confidence: 0.8, // Initial confidence
			Metadata: map[string]interface{}{
				"firstSeen": time.Now(),
				"source":    "rca_analysis",
			},
		}
		patterns = append(patterns, pattern)
	}

	return patterns
}

// findSimilarIncidents finds similar past incidents using embedding similarity
func (ls *LearningService) findSimilarIncidents(logFile *models.LogFile, errorEntries []models.LogEntry) ([]SimilarIncident, error) {
	// Generate embedding for current error context
	errorContext := ls.buildErrorContext(errorEntries)
	embedding, err := ls.llmService.GenerateEmbedding(errorContext)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Find similar analyses using embedding similarity
	similarMemories, err := ls.llmService.FindSimilarAnalyses(ls.db, embedding, ls.maxMemoriesToUse)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar analyses: %w", err)
	}

	var incidents []SimilarIncident
	for _, memory := range similarMemories {
		similarity := ls.calculateSimilarity(embedding, memory.Embedding)
		if similarity >= ls.similarityThreshold {
			incident := SimilarIncident{
				LogFileID:  *memory.LogFileID,
				Summary:    memory.Summary,
				RootCause:  memory.RootCause,
				Timestamp:  memory.CreatedAt,
				Similarity: similarity,
				Relevance:  ls.calculateRelevance(similarity),
			}

			// Get filename for the incident
			var logFile models.LogFile
			if err := ls.db.Select("filename").First(&logFile, memory.LogFileID).Error; err == nil {
				incident.Filename = logFile.Filename
			}

			// Get severity from metadata if available
			if memory.Metadata != nil {
				if severity, ok := memory.Metadata["severity"].(string); ok {
					incident.Severity = severity
				}
			}

			incidents = append(incidents, incident)
		}
	}

	// Sort by similarity (highest first)
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].Similarity > incidents[j].Similarity
	})

	return incidents, nil
}

// identifyPatterns identifies known patterns in current error entries
func (ls *LearningService) identifyPatterns(errorEntries []models.LogEntry) ([]PatternMatch, error) {
	var matches []PatternMatch

	// Get all known patterns from database
	patterns, err := ls.GetPatterns()
	if err != nil {
		return nil, fmt.Errorf("failed to load patterns: %w", err)
	}

	// Build error context
	errorContext := ls.buildErrorContext(errorEntries)

	for _, pattern := range patterns {
		confidence := ls.calculatePatternMatchConfidence(pattern, errorContext)
		if confidence > 0.5 { // 50% confidence threshold
			match := PatternMatch{
				Pattern:     pattern,
				Confidence:  confidence,
				MatchReason: ls.generateMatchReason(pattern, errorContext),
				Relevance:   ls.calculateRelevance(confidence),
			}
			matches = append(matches, match)
		}
	}

	// Sort by confidence (highest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Confidence > matches[j].Confidence
	})

	return matches, nil
}

// buildErrorContext builds a context string from error entries
func (ls *LearningService) buildErrorContext(errorEntries []models.LogEntry) string {
	var context strings.Builder
	context.WriteString("Error Analysis Context:\n")

	for _, entry := range errorEntries {
		context.WriteString(fmt.Sprintf("- %s: %s\n", entry.Level, entry.Message))
		if entry.ErrorCode != "" {
			context.WriteString(fmt.Sprintf("  Error Code: %s\n", entry.ErrorCode))
		}
		if entry.Service != "" {
			context.WriteString(fmt.Sprintf("  Service: %s\n", entry.Service))
		}
	}

	return context.String()
}

// calculateSimilarity calculates cosine similarity between two embeddings
func (ls *LearningService) calculateSimilarity(embedding1 []float32, embedding2 models.JSONB) float64 {
	// Extract embedding from JSONB
	var emb2 []float32
	if embedding2 != nil {
		if embData, ok := embedding2["embedding"].([]interface{}); ok {
			for _, v := range embData {
				if f, ok := v.(float64); ok {
					emb2 = append(emb2, float32(f))
				}
			}
		}
	}

	if len(embedding1) == 0 || len(emb2) == 0 {
		return 0.0
	}

	// Ensure both embeddings have the same length
	minLen := len(embedding1)
	if len(emb2) < minLen {
		minLen = len(emb2)
	}

	// Calculate cosine similarity
	var dotProduct float64
	var norm1 float64
	var norm2 float64

	for i := 0; i < minLen; i++ {
		dotProduct += float64(embedding1[i] * emb2[i])
		norm1 += float64(embedding1[i] * embedding1[i])
		norm2 += float64(emb2[i] * emb2[i])
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

// calculateRelevance converts similarity/confidence to relevance level
func (ls *LearningService) calculateRelevance(value float64) string {
	if value >= 0.8 {
		return "high"
	} else if value >= 0.6 {
		return "medium"
	}
	return "low"
}

// calculatePatternMatchConfidence calculates how well a pattern matches current errors
func (ls *LearningService) calculatePatternMatchConfidence(pattern Pattern, errorContext string) float64 {
	errorContext = strings.ToLower(errorContext)
	confidence := 0.0

	// Check keyword matches
	for _, keyword := range pattern.ErrorKeywords {
		if strings.Contains(errorContext, strings.ToLower(keyword)) {
			confidence += 0.2
		}
	}

	// Check root cause similarity
	if strings.Contains(errorContext, strings.ToLower(pattern.RootCause)) {
		confidence += 0.3
	}

	// Normalize confidence
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// generateMatchReason explains why a pattern matched
func (ls *LearningService) generateMatchReason(pattern Pattern, errorContext string) string {
	var reasons []string
	errorContext = strings.ToLower(errorContext)

	for _, keyword := range pattern.ErrorKeywords {
		if strings.Contains(errorContext, strings.ToLower(keyword)) {
			reasons = append(reasons, fmt.Sprintf("contains keyword '%s'", keyword))
		}
	}

	if strings.Contains(errorContext, strings.ToLower(pattern.RootCause)) {
		reasons = append(reasons, "similar root cause")
	}

	if len(reasons) == 0 {
		return "pattern similarity based on historical analysis"
	}

	return strings.Join(reasons, ", ")
}

// extractKeywords extracts keywords from a pattern name
func (ls *LearningService) extractKeywords(patternName string) []string {
	keywords := []string{patternName}

	// Add common variations
	lowerPattern := strings.ToLower(patternName)
	if strings.Contains(lowerPattern, "timeout") {
		keywords = append(keywords, "timeout", "connection timeout", "request timeout")
	}
	if strings.Contains(lowerPattern, "connection") {
		keywords = append(keywords, "connection", "connect", "network")
	}
	if strings.Contains(lowerPattern, "database") {
		keywords = append(keywords, "database", "db", "sql")
	}
	if strings.Contains(lowerPattern, "authentication") {
		keywords = append(keywords, "authentication", "auth", "login")
	}

	return keywords
}

// updatePattern updates or creates a pattern in the database
func (ls *LearningService) updatePattern(pattern Pattern) error {
	// Convert to database model
	dbPattern := &models.Pattern{
		Name:            pattern.Name,
		Description:     pattern.Description,
		ErrorKeywords:   models.JSONB{"keywords": pattern.ErrorKeywords},
		RootCause:       pattern.RootCause,
		CommonFixes:     models.JSONB{"fixes": pattern.CommonFixes},
		Severity:        pattern.Severity,
		OccurrenceCount: pattern.OccurrenceCount,
		LastSeen:        pattern.LastSeen,
		Confidence:      pattern.Confidence,
		Metadata:        models.JSONB(pattern.Metadata),
	}

	// Check if pattern already exists
	var existingPattern models.Pattern
	err := ls.db.Where("name = ?", pattern.Name).First(&existingPattern).Error

	if err == gorm.ErrRecordNotFound {
		// Create new pattern
		return ls.db.Create(dbPattern).Error
	} else if err != nil {
		return fmt.Errorf("failed to check existing pattern: %w", err)
	}

	// Update existing pattern
	existingPattern.OccurrenceCount++
	existingPattern.LastSeen = time.Now()
	existingPattern.Confidence = math.Min(existingPattern.Confidence+0.1, 1.0) // Increase confidence

	// Update common fixes if new ones are provided
	if len(pattern.CommonFixes) > 0 {
		// Merge fixes
		var existingFixes []string
		if existingPattern.CommonFixes != nil {
			if fixes, ok := existingPattern.CommonFixes["fixes"].([]interface{}); ok {
				for _, fix := range fixes {
					if fixStr, ok := fix.(string); ok {
						existingFixes = append(existingFixes, fixStr)
					}
				}
			}
		}
		existingFixes = append(existingFixes, pattern.CommonFixes...)
		existingPattern.CommonFixes = models.JSONB{"fixes": existingFixes}
	}

	return ls.db.Save(&existingPattern).Error
}

// storeAnalysisEmbedding stores embedding for future similarity search
func (ls *LearningService) storeAnalysisEmbedding(logFileID uint, analysis *LogAnalysisResponse) error {
	// Generate embedding from summary and root cause
	embeddingText := fmt.Sprintf("%s\n%s", analysis.Summary, analysis.RootCause)
	embedding, err := ls.llmService.GenerateEmbedding(embeddingText)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Store in LogAnalysisMemory
	memory := &models.LogAnalysisMemory{
		LogFileID: &logFileID,
		Summary:   analysis.Summary,
		RootCause: analysis.RootCause,
		Embedding: models.JSONB{"embedding": embedding},
		Metadata: map[string]interface{}{
			"severity":        analysis.Severity,
			"recommendations": analysis.Recommendations,
			"errorCount":      analysis.CriticalErrors + analysis.NonCriticalErrors,
		},
	}

	return ls.db.Create(memory).Error
}

// calculateConfidenceBoost calculates how much confidence to boost based on learning
func (ls *LearningService) calculateConfidenceBoost(insights *LearningInsights) float64 {
	boost := 0.0

	// Boost from similar incidents
	for _, incident := range insights.SimilarIncidents {
		if incident.Relevance == "high" {
			boost += 0.2
		} else if incident.Relevance == "medium" {
			boost += 0.1
		}
	}

	// Boost from pattern matches
	for _, match := range insights.PatternMatches {
		if match.Relevance == "high" {
			boost += 0.15
		} else if match.Relevance == "medium" {
			boost += 0.08
		}
	}

	// Cap the boost
	if boost > 0.5 {
		boost = 0.5
	}

	return boost
}

// generateSuggestedContext generates context for LLM based on learning insights
func (ls *LearningService) generateSuggestedContext(insights *LearningInsights) string {
	var context strings.Builder

	if len(insights.SimilarIncidents) > 0 {
		context.WriteString("SIMILAR PAST INCIDENTS:\n")
		for i, incident := range insights.SimilarIncidents {
			if i >= 3 { // Limit to top 3
				break
			}
			context.WriteString(fmt.Sprintf("- %s (%.1f%% similar): %s\n  Root Cause: %s\n",
				incident.Filename, incident.Similarity*100, incident.Summary, incident.RootCause))
		}
		context.WriteString("\n")
	}

	if len(insights.PatternMatches) > 0 {
		context.WriteString("IDENTIFIED PATTERNS:\n")
		for i, match := range insights.PatternMatches {
			if i >= 3 { // Limit to top 3
				break
			}
			context.WriteString(fmt.Sprintf("- %s (%.1f%% confidence): %s\n  Common Fix: %s\n",
				match.Pattern.Name, match.Confidence*100, match.Pattern.RootCause,
				strings.Join(match.Pattern.CommonFixes, ", ")))
		}
		context.WriteString("\n")
	}

	if insights.ConfidenceBoost > 0 {
		context.WriteString(fmt.Sprintf("LEARNING CONFIDENCE BOOST: %.1f%%\n", insights.ConfidenceBoost*100))
	}

	return context.String()
}

// extractPatternsFromMatches extracts patterns from pattern matches
func (ls *LearningService) extractPatternsFromMatches(matches []PatternMatch) []Pattern {
	var patterns []Pattern
	for _, match := range matches {
		patterns = append(patterns, match.Pattern)
	}
	return patterns
}

// updateLearningMetrics updates learning performance metrics
func (ls *LearningService) updateLearningMetrics(analysis *LogAnalysisResponse) error {
	// This would update metrics in a separate metrics table
	// For now, we'll just log the metrics
	logger.Info("Learning metrics updated", map[string]interface{}{
		"severity": analysis.Severity,
		"patterns": len(analysis.ErrorAnalysis),
	})
	return nil
}

// GetLearningMetrics retrieves current learning metrics
func (ls *LearningService) GetLearningMetrics() (*LearningMetrics, error) {
	// This would retrieve metrics from a metrics table
	// For now, return default metrics
	return &LearningMetrics{
		TotalAnalyses:       0,
		PatternMatches:      0,
		AccuracyImprovement: 0.0,
		AverageConfidence:   0.0,
		LearningRate:        0.0,
	}, nil
}

// GetPatterns returns all learned patterns
func (ls *LearningService) GetPatterns() ([]Pattern, error) {
	var dbPatterns []models.Pattern
	err := ls.db.Find(&dbPatterns).Error
	if err != nil {
		return nil, err
	}

	// Convert to service pattern format
	var patterns []Pattern
	for _, dbPattern := range dbPatterns {
		pattern := Pattern{
			ID:              dbPattern.ID,
			Name:            dbPattern.Name,
			Description:     dbPattern.Description,
			RootCause:       dbPattern.RootCause,
			Severity:        dbPattern.Severity,
			OccurrenceCount: dbPattern.OccurrenceCount,
			LastSeen:        dbPattern.LastSeen,
			Confidence:      dbPattern.Confidence,
		}

		// Extract keywords from JSONB
		if dbPattern.ErrorKeywords != nil {
			if keywords, ok := dbPattern.ErrorKeywords["keywords"].([]interface{}); ok {
				for _, keyword := range keywords {
					if keywordStr, ok := keyword.(string); ok {
						pattern.ErrorKeywords = append(pattern.ErrorKeywords, keywordStr)
					}
				}
			}
		}

		// Extract fixes from JSONB
		if dbPattern.CommonFixes != nil {
			if fixes, ok := dbPattern.CommonFixes["fixes"].([]interface{}); ok {
				for _, fix := range fixes {
					if fixStr, ok := fix.(string); ok {
						pattern.CommonFixes = append(pattern.CommonFixes, fixStr)
					}
				}
			}
		}

		// Extract metadata
		if dbPattern.Metadata != nil {
			pattern.Metadata = map[string]interface{}(dbPattern.Metadata)
		}

		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

// GetPatternByID returns a specific pattern by ID
func (ls *LearningService) GetPatternByID(patternID uint) (*Pattern, error) {
	var dbPattern models.Pattern
	err := ls.db.First(&dbPattern, patternID).Error
	if err != nil {
		return nil, err
	}

	// Convert to service pattern format
	pattern := Pattern{
		ID:              dbPattern.ID,
		Name:            dbPattern.Name,
		Description:     dbPattern.Description,
		RootCause:       dbPattern.RootCause,
		Severity:        dbPattern.Severity,
		OccurrenceCount: dbPattern.OccurrenceCount,
		LastSeen:        dbPattern.LastSeen,
		Confidence:      dbPattern.Confidence,
	}

	// Extract keywords from JSONB
	if dbPattern.ErrorKeywords != nil {
		if keywords, ok := dbPattern.ErrorKeywords["keywords"].([]interface{}); ok {
			for _, keyword := range keywords {
				if keywordStr, ok := keyword.(string); ok {
					pattern.ErrorKeywords = append(pattern.ErrorKeywords, keywordStr)
				}
			}
		}
	}

	// Extract fixes from JSONB
	if dbPattern.CommonFixes != nil {
		if fixes, ok := dbPattern.CommonFixes["fixes"].([]interface{}); ok {
			for _, fix := range fixes {
				if fixStr, ok := fix.(string); ok {
					pattern.CommonFixes = append(pattern.CommonFixes, fixStr)
				}
			}
		}
	}

	// Extract metadata
	if dbPattern.Metadata != nil {
		pattern.Metadata = map[string]interface{}(dbPattern.Metadata)
	}

	return &pattern, nil
}

// DeletePattern deletes a pattern
func (ls *LearningService) DeletePattern(patternID uint) error {
	return ls.db.Delete(&models.Pattern{}, patternID).Error
}
