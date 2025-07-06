# Feedback Incorporation Plan for RCA Analysis

## Overview

This document outlines the comprehensive plan to incorporate user feedback from the `log_analyses_review` table into the Root Cause Analysis (RCA) system to improve accuracy and learning over time.

## Problem Statement

Currently, the system:
- ✅ Collects user feedback on RCA results
- ✅ Stores feedback in `log_analysis_feedback` table
- ❌ **Does NOT use feedback to improve future analyses**
- ❌ **Does NOT adjust pattern confidence based on user corrections**
- ❌ **Does NOT incorporate user corrections into LLM prompts**

## Solution Architecture

### Phase 1: Enhanced Data Models ✅

#### 1.1 Enhanced LogAnalysisFeedback Model
```go
type LogAnalysisFeedback struct {
    ID               uint      `gorm:"primaryKey" json:"id"`
    AnalysisMemoryID uint      `json:"analysisMemoryId"`
    UserID           *uint     `json:"userId"`
    IsCorrect        bool      `json:"isCorrect"`
    Correction       string    `json:"correction"`
    CreatedAt        time.Time `json:"createdAt"`
    
    // NEW: Enhanced fields for feedback processing
    FeedbackType     string    `json:"feedbackType" gorm:"default:'general'"`
    PatternName      *string   `json:"patternName"`
    RootCauseSection *string   `json:"rootCauseSection"`
    ConfidenceImpact float64   `json:"confidenceImpact" gorm:"default:0.0"`
    Processed        bool      `json:"processed" gorm:"default:false"`
    ProcessedAt      *time.Time `json:"processedAt"`
}
```

#### 1.2 New FeedbackInsight Model
```go
type FeedbackInsight struct {
    PatternName      string   `json:"patternName"`
    RootCause        string   `json:"rootCause"`
    PositiveFeedback int      `json:"positiveFeedback"`
    NegativeFeedback int      `json:"negativeFeedback"`
    Corrections      []string `json:"corrections"`
    ConfidenceScore  float64  `json:"confidenceScore"`
    LastUpdated      time.Time `json:"lastUpdated"`
}
```

### Phase 2: Feedback Service ✅

#### 2.1 Core Feedback Service Functions
- `GetFeedbackForSimilarIncidents()` - Retrieves feedback for similar past incidents
- `GetFeedbackInsights()` - Aggregates feedback insights for patterns and root causes
- `GetFeedbackForPattern()` - Gets feedback for specific patterns
- `GetFeedbackContext()` - Generates context string for LLM prompts
- `AdjustConfidenceBasedOnFeedback()` - Adjusts pattern confidence based on feedback

#### 2.2 Feedback Processing Logic
```go
// Example: Adjusting confidence based on feedback
func (fs *FeedbackService) AdjustConfidenceBasedOnFeedback(patternName string, baseConfidence float64) float64 {
    feedbacks, err := fs.GetFeedbackForPattern(patternName)
    if err != nil || len(feedbacks) == 0 {
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

    feedbackRatio := float64(positiveCount) / float64(total)
    adjustment := (feedbackRatio - 0.5) * 0.3 // Max ±15% adjustment
    return baseConfidence + adjustment
}
```

### Phase 3: Enhanced Learning Service ✅

#### 3.1 Integration Points
- **Pattern Confidence Adjustment**: Automatically adjust pattern confidence based on user feedback
- **Feedback Context**: Include feedback in learning insights
- **Similar Incident Enhancement**: Consider feedback when finding similar incidents

#### 3.2 Key Enhancements
```go
// Adjust pattern confidence with feedback
func (ls *LearningService) adjustPatternConfidenceWithFeedback(patternMatches []PatternMatch) {
    for i := range patternMatches {
        originalConfidence := patternMatches[i].Confidence
        adjustedConfidence := ls.feedbackService.AdjustConfidenceBasedOnFeedback(
            patternMatches[i].Pattern.Name, 
            originalConfidence,
        )
        patternMatches[i].Confidence = adjustedConfidence
        patternMatches[i].Relevance = ls.calculateRelevance(adjustedConfidence)
    }
}
```

### Phase 4: Enhanced LLM Prompts ✅

#### 4.1 New Prompt Types
- `CreateFeedbackEnhancedPrompt()` - Includes user feedback context
- Enhanced learning prompts with feedback integration

#### 4.2 Feedback Context in Prompts
```
USER FEEDBACK FROM SIMILAR INCIDENTS:
- Positive feedback: 3
- Negative feedback: 1
- User corrections:
  * Root cause was actually database connection timeout
  * Pattern should be classified as "network" not "application"

PATTERN 'Database Timeout' FEEDBACK:
- Positive: 2, Negative: 1
- WARNING: This pattern has received negative feedback

IMPORTANT: Consider the user feedback above when analyzing similar patterns and root causes.
```

### Phase 5: Enhanced Job Service ✅

#### 5.1 Feedback-Enhanced Analysis
- Retrieve feedback context for similar incidents
- Include feedback in chunk analysis
- Use feedback-enhanced prompts for better accuracy

#### 5.2 Implementation Flow
```go
func (js *JobService) performRCAAnalysisWithErrorTrackingAndChunkCount(...) {
    // 1. Get learning insights
    learningInsights, err := js.learningService.GetLearningInsights(logFile, errorEntries)
    
    // 2. Get feedback context
    feedbackContext := js.feedbackService.GetFeedbackContext(
        learningInsights.SimilarIncidents, 
        learningInsights.PatternMatches,
    )
    
    // 3. Use enhanced prompts
    prompt := js.llmService.CreateFeedbackEnhancedPrompt(
        request, errorEntries, learningInsights, feedbackContext,
    )
}
```

## API Endpoints

### New Feedback Endpoints
```
GET /api/v1/feedback/insights          - Get aggregated feedback insights
GET /api/v1/feedback/patterns/:name    - Get feedback for specific pattern
```

### Enhanced Existing Endpoints
- All RCA analysis endpoints now use feedback-enhanced prompts
- Learning insights include feedback context
- Pattern confidence automatically adjusted based on feedback

## Database Changes

### GORM Auto-Migration
The enhanced `LogAnalysisFeedback` model will be automatically migrated by GORM when the container restarts. The `AutoMigrate()` function in `backend/internal/db/connection.go` already includes:

```go
log.Println("Testing migration with LogAnalysisFeedback model...")
err = DB.AutoMigrate(&models.LogAnalysisFeedback{})
if err != nil {
    log.Printf("LogAnalysisFeedback migration failed: %v", err)
    return
}
log.Println("✅ LogAnalysisFeedback table migrated successfully")
```

This will automatically add the new fields:
- `feedback_type VARCHAR(50) DEFAULT 'general'`
- `pattern_name VARCHAR(255)`
- `root_cause_section TEXT`
- `confidence_impact DECIMAL(3,2) DEFAULT 0.0`
- `processed BOOLEAN DEFAULT FALSE`
- `processed_at TIMESTAMP`

## Benefits

### 1. Improved Accuracy
- **Pattern Confidence**: Automatically adjust based on user feedback
- **Root Cause Detection**: Learn from user corrections
- **Similar Incident Matching**: Consider feedback when finding similar cases

### 2. Continuous Learning
- **Feedback Loop**: Each user correction improves future analyses
- **Pattern Evolution**: Patterns evolve based on user feedback
- **Confidence Scoring**: Dynamic confidence based on user validation

### 3. Better User Experience
- **Contextual Prompts**: LLM considers past user feedback
- **Warnings**: Alert when patterns have received negative feedback
- **Transparency**: Show feedback impact on analysis

## Implementation Status

### ✅ Completed
- [x] Enhanced LogAnalysisFeedback model
- [x] FeedbackService implementation
- [x] Enhanced LearningService integration
- [x] Feedback-enhanced LLM prompts
- [x] Enhanced JobService integration
- [x] New API endpoints
- [x] GORM auto-migration support
- [x] Controller methods

### 🔄 Next Steps
1. **Restart Container**: The new fields will be automatically added via GORM migration
2. **Test Feedback Flow**: Verify feedback is being used in RCA analysis
3. **Monitor Performance**: Track improvement in RCA accuracy
4. **User Interface**: Consider adding feedback impact indicators in UI

## Usage Examples

### 1. Automatic Pattern Confidence Adjustment
```go
// Before feedback: Pattern confidence = 0.8
// After negative feedback: Pattern confidence = 0.65 (-15% adjustment)
// After positive feedback: Pattern confidence = 0.95 (+15% adjustment)
```

### 2. Feedback Context in LLM Prompts
```
When analyzing similar incidents, the LLM now sees:
"Users previously corrected this pattern's root cause to 'database connection timeout' 
instead of 'application error'. Consider this correction in your analysis."
```

### 3. Pattern Warnings
```
"WARNING: Pattern 'Database Timeout' has received 3 negative feedback vs 1 positive.
Consider alternative root causes."
```

## Monitoring and Metrics

### Key Metrics to Track
- **Feedback Utilization Rate**: How often feedback is used in analysis
- **Pattern Confidence Changes**: Track confidence adjustments over time
- **Accuracy Improvement**: Measure RCA accuracy before/after feedback integration
- **User Satisfaction**: Track if feedback incorporation improves user satisfaction

### Logging
All feedback operations are logged with appropriate context:
```go
logger.Info("Adjusted pattern confidence with feedback", map[string]interface{}{
    "pattern":           patternMatches[i].Pattern.Name,
    "originalConfidence": originalConfidence,
    "adjustedConfidence": adjustedConfidence,
    "relevance":         patternMatches[i].Relevance,
})
```

## Conclusion

This implementation creates a **true feedback loop** where:
1. Users provide feedback on RCA results
2. System learns from feedback and adjusts patterns
3. Future analyses incorporate past feedback
4. Accuracy improves over time through continuous learning

The system now **actively reuses feedback for similar patterns** and provides **contextual guidance** to the LLM based on user corrections, creating a more intelligent and adaptive RCA system.

**Note**: No manual database migration is required. Simply restart the container and GORM will automatically add the new feedback fields to the existing `log_analysis_feedback` table. 