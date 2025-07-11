package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"math"
	"sync"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"gorm.io/gorm"
)

type LLMService struct {
	baseURL    string
	llmModel   string
	embedModel string
	client     *http.Client
	apiCalls   []LLMAPICall
	callMutex  sync.RWMutex
}

type OllamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type OllamaGenerateResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

type LogAnalysisRequest struct {
	LogEntries   []models.LogEntry `json:"logEntries"`
	ErrorCount   int               `json:"errorCount"`
	WarningCount int               `json:"warningCount"`
	StartTime    time.Time         `json:"startTime"`
	EndTime      time.Time         `json:"endTime"`
	Filename     string            `json:"filename"`
}

// Enhanced response structure for detailed error analysis
type LogAnalysisResponse struct {
	Summary         string   `json:"summary"`
	Severity        string   `json:"severity"`
	RootCause       string   `json:"rootCause"`
	Recommendations []string `json:"recommendations"`

	ErrorAnalysis     []DetailedErrorAnalysis `json:"errorAnalysis"`
	CriticalErrors    int                     `json:"criticalErrors"`
	NonCriticalErrors int                     `json:"nonCriticalErrors"`
}

// New structure for detailed error analysis
type DetailedErrorAnalysis struct {
	ErrorPattern    string   `json:"errorPattern"`
	ErrorCount      int      `json:"errorCount"`
	FirstOccurrence string   `json:"firstOccurrence"`
	LastOccurrence  string   `json:"lastOccurrence"`
	Severity        string   `json:"severity"`
	RootCause       string   `json:"rootCause"`
	Impact          string   `json:"impact"`
	Fix             string   `json:"fix"`
	RelatedErrors   []string `json:"relatedErrors"`
}

// LLMAPICall tracks individual LLM API calls for monitoring and debugging
type LLMAPICall struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Endpoint  string                 `json:"endpoint"`
	Model     string                 `json:"model"`
	LogFileID *uint                  `json:"logFileId,omitempty"`
	JobID     *uint                  `json:"jobId,omitempty"`
	CallType  string                 `json:"callType"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Status    int                    `json:"status"`
	Duration  time.Duration          `json:"duration"`
	Response  string                 `json:"response,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// OllamaEmbeddingRequest represents a request to generate embeddings
type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbeddingResponse represents the response from the embedding API
type OllamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

func NewLLMService(baseURL, llmModel string) *LLMService {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if llmModel == "" {
		llmModel = "llama2:7b"
	}

	logger.Info("LLMService initialized", map[string]interface{}{
		"base_url":  baseURL,
		"llm_model": llmModel,
		"component": "llm_service",
	})

	return &LLMService{
		baseURL:    baseURL,
		llmModel:   llmModel,
		embedModel: "nomic-embed-text:latest",
		client:     &http.Client{Timeout: 300 * time.Second},
		apiCalls:   make([]LLMAPICall, 0),
	}
}

// NewLLMServiceWithEndpoint creates a new LLM service with a specific endpoint
func NewLLMServiceWithEndpoint(baseURL, llmModel string) *LLMService {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if llmModel == "" {
		llmModel = "llama2:7b"
	}

	logger.Info("LLMService initialized with custom endpoint", map[string]interface{}{
		"base_url":  baseURL,
		"llm_model": llmModel,
		"component": "llm_service",
	})

	return &LLMService{
		baseURL:    baseURL,
		llmModel:   llmModel,
		embedModel: "nomic-embed-text:latest",
		client:     &http.Client{Timeout: 300 * time.Second},
		apiCalls:   make([]LLMAPICall, 0),
	}
}

// GetAPICalls returns all tracked LLM API calls
func (ls *LLMService) GetAPICalls() []LLMAPICall {
	ls.callMutex.RLock()
	defer ls.callMutex.RUnlock()

	// Return a copy to avoid race conditions
	calls := make([]LLMAPICall, len(ls.apiCalls))
	copy(calls, ls.apiCalls)
	return calls
}

// ClearAPICalls clears the API call history
func (ls *LLMService) ClearAPICalls() {
	ls.callMutex.Lock()
	defer ls.callMutex.Unlock()
	ls.apiCalls = make([]LLMAPICall, 0)
}

// addAPICall adds a new API call to the tracking list
func (ls *LLMService) addAPICall(call LLMAPICall) {
	ls.callMutex.Lock()
	defer ls.callMutex.Unlock()

	// Keep only last 100 calls to prevent memory issues
	if len(ls.apiCalls) >= 100 {
		ls.apiCalls = ls.apiCalls[1:]
	}
	ls.apiCalls = append(ls.apiCalls, call)
}

// Update LLMAPICall to accept model as a parameter
func (ls *LLMService) CreateAPICall(logFileID *uint, jobID *uint, callType string, model string) *LLMAPICall {
	return &LLMAPICall{
		ID:        fmt.Sprintf("llm_%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Endpoint:  "/api/generate",
		Model:     model,
		LogFileID: logFileID,
		JobID:     jobID,
		CallType:  callType,
	}
}

// Update TrackAPICall to accept model as a parameter
func (ls *LLMService) TrackAPICall(logFileID *uint, jobID *uint, callType string, model string, payload map[string]interface{}, status int, duration time.Duration, response string, err string) {
	call := ls.CreateAPICall(logFileID, jobID, callType, model)
	call.Payload = payload
	call.Status = status
	call.Duration = duration
	call.Response = response
	if err != "" {
		call.Error = err
	}
	ls.addAPICall(*call)
}

// AnalyzeLogsWithAI performs AI-powered analysis of log entries with focus on errors
func (ls *LLMService) AnalyzeLogsWithAI(logFile *models.LogFile, entries []models.LogEntry, jobID *uint) (*LogAnalysisResponse, error) {
	logEntry := logger.WithLLM(&logFile.ID, jobID, "rca_analysis")

	if logFile == nil {
		logEntry.Error("LogFile is nil")
		return nil, fmt.Errorf("logFile is nil")
	}

	// Filter only ERROR and FATAL entries for analysis
	errorEntries := ls.filterErrorEntries(entries)
	if len(errorEntries) == 0 {
		logEntry.Info("No error entries found, returning basic analysis")
		// No errors found, return basic analysis
		return ls.generateNoErrorsAnalysis(logFile), nil
	}

	logEntry.Info("Starting AI analysis", map[string]interface{}{
		"total_entries": len(entries),
		"error_entries": len(errorEntries),
		"log_file_id":   logFile.ID,
		"filename":      logFile.Filename,
	})

	// Prepare the analysis request
	request := LogAnalysisRequest{
		LogEntries:   errorEntries, // Only error entries
		ErrorCount:   logFile.ErrorCount,
		WarningCount: logFile.WarningCount,
		Filename:     logFile.Filename,
	}

	if len(errorEntries) > 0 {
		request.StartTime = errorEntries[0].Timestamp
		request.EndTime = errorEntries[len(errorEntries)-1].Timestamp
	}

	// Create the prompt for the LLM
	prompt := ls.createDetailedErrorAnalysisPrompt(request, errorEntries, "") // Pass an empty string for similarIncidents for now

	logEntry.Debug("Generated analysis prompt", map[string]interface{}{
		"prompt_length": len(prompt),
	})

	// Call the local LLM
	response, err := ls.callLLMWithContext(prompt, &logFile.ID, jobID, "rca_analysis")
	if err != nil {
		logger.WithError(err, "llm_service").Error("LLM analysis failed")
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	logEntry.Debug("Received LLM response", map[string]interface{}{
		"response_length": len(response),
	})

	// Parse the LLM response
	analysis, err := ls.parseDetailedLLMResponse(response)
	if err != nil {
		logger.WithError(err, "llm_service").Error("Failed to parse LLM response")
		return nil, fmt.Errorf("Failed to parse LLM response: %w", err)
	}

	// Validate analysis (must have summary and root cause)
	if analysis.Summary == "" || analysis.RootCause == "" {
		logEntry.Error("LLM returned incomplete analysis", map[string]interface{}{
			"has_summary":    analysis.Summary != "",
			"has_root_cause": analysis.RootCause != "",
		})
		return nil, fmt.Errorf("LLM returned incomplete analysis (missing summary or root cause)")
	}

	logEntry.Info("AI analysis completed successfully", map[string]interface{}{
		"severity":          analysis.Severity,
		"summary_length":    len(analysis.Summary),
		"root_cause_length": len(analysis.RootCause),
	})

	return analysis, nil
}

// AnalyzeLogsWithAIWithTimeout performs AI-powered analysis of log entries with focus on errors, with a per-request timeout.
func (ls *LLMService) AnalyzeLogsWithAIWithTimeout(logFile *models.LogFile, entries []models.LogEntry, timeout int, jobID *uint) (*LogAnalysisResponse, error) {
	logEntry := logger.WithLLM(&logFile.ID, jobID, "rca_analysis_timeout")

	if logFile == nil {
		logEntry.Error("LogFile is nil")
		return nil, fmt.Errorf("logFile is nil")
	}
	// Filter only ERROR and FATAL entries for analysis
	errorEntries := ls.filterErrorEntries(entries)
	if len(errorEntries) == 0 {
		logEntry.Info("No ERROR/FATAL entries found in chunk, using fallback analysis", map[string]interface{}{
			"total_entries": len(entries),
		})
		return ls.generateNoErrorsAnalysis(logFile), nil
	}

	logEntry.Info("Found ERROR/FATAL entries in chunk, proceeding with LLM analysis", map[string]interface{}{
		"error_entries": len(errorEntries),
		"total_entries": len(entries),
		"timeout":       timeout,
	})

	request := LogAnalysisRequest{
		LogEntries:   errorEntries,
		ErrorCount:   logFile.ErrorCount,
		WarningCount: logFile.WarningCount,
		Filename:     logFile.Filename,
	}
	if len(errorEntries) > 0 {
		request.StartTime = errorEntries[0].Timestamp
		request.EndTime = errorEntries[len(errorEntries)-1].Timestamp
	}
	prompt := ls.createDetailedErrorAnalysisPrompt(request, errorEntries, "")
	response, err := ls.callLLMWithContextAndTimeout(prompt, &logFile.ID, jobID, "rca_analysis", timeout)
	if err != nil {
		logger.WithError(err, "llm_service").Error("LLM analysis failed")
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}
	analysis, err := ls.parseDetailedLLMResponse(response)
	if err != nil {
		logger.WithError(err, "llm_service").Error("Failed to parse LLM response")
		return nil, fmt.Errorf("Failed to parse LLM response: %w", err)
	}
	if analysis.Summary == "" || analysis.RootCause == "" {
		logEntry.Error("LLM returned incomplete analysis")
		return nil, fmt.Errorf("LLM returned incomplete analysis (missing summary or root cause)")
	}

	logEntry.Info("AI analysis completed successfully", map[string]interface{}{
		"severity":          analysis.Severity,
		"summary_length":    len(analysis.Summary),
		"root_cause_length": len(analysis.RootCause),
	})

	return analysis, nil
}

// filterErrorEntries filters only ERROR and FATAL log entries
func (ls *LLMService) filterErrorEntries(entries []models.LogEntry) []models.LogEntry {
	var errorEntries []models.LogEntry
	for _, entry := range entries {
		if entry.Level == "ERROR" || entry.Level == "FATAL" {
			errorEntries = append(errorEntries, entry)
		}
	}
	return errorEntries
}

// generateNoErrorsAnalysis creates a basic analysis when no errors are found
func (ls *LLMService) generateNoErrorsAnalysis(logFile *models.LogFile) *LogAnalysisResponse {
	return &LogAnalysisResponse{
		Summary:           fmt.Sprintf("Log file '%s' contains no ERROR or FATAL entries. System appears to be functioning normally.", logFile.Filename),
		Severity:          "low",
		RootCause:         "No errors detected in the log file",
		Recommendations:   []string{"Continue monitoring for any new errors", "Review INFO and WARNING logs for potential issues", "System is operating within normal parameters"},
		ErrorAnalysis:     []DetailedErrorAnalysis{},
		CriticalErrors:    0,
		NonCriticalErrors: 0,
	}
}

// createDetailedErrorAnalysisPrompt creates a comprehensive prompt for error analysis
func (ls *LLMService) createDetailedErrorAnalysisPrompt(request LogAnalysisRequest, errorEntries []models.LogEntry, similarIncidents string) string {
	// Build the error entries section
	var errorEntriesText strings.Builder
	for i, entry := range errorEntries {
		errorEntriesText.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, entry.Timestamp.Format("2006-01-02 15:04:05"), entry.Message))
	}

	// Build the prompt using the constant
	prompt := fmt.Sprintf(RCA_ANALYSIS_PROMPT,
		request.Filename,
		request.ErrorCount,
		request.WarningCount,
		request.StartTime.Format("2006-01-02 15:04:05"),
		request.EndTime.Format("2006-01-02 15:04:05"),
		errorEntriesText.String(),
		similarIncidents)

	return prompt
}

// CreateDetailedErrorAnalysisPrompt is the public version of createDetailedErrorAnalysisPrompt
func (ls *LLMService) CreateDetailedErrorAnalysisPrompt(request LogAnalysisRequest, errorEntries []models.LogEntry, similarIncidents string) string {
	return ls.createDetailedErrorAnalysisPrompt(request, errorEntries, similarIncidents)
}

// CreateDetailedErrorAnalysisPromptWithLearning creates a prompt with learning insights and feedback
func (ls *LLMService) CreateDetailedErrorAnalysisPromptWithLearning(request LogAnalysisRequest, errorEntries []models.LogEntry, insights *LearningInsights) string {
	var result strings.Builder

	// Add similar incidents
	if len(insights.SimilarIncidents) > 0 {
		result.WriteString("SIMILAR PAST INCIDENTS:\n")
		for i, incident := range insights.SimilarIncidents {
			result.WriteString(fmt.Sprintf("%d. File: %s (Similarity: %.2f%%)\n", i+1, incident.Filename, incident.Similarity*100))
			result.WriteString(fmt.Sprintf("   Summary: %s\n", incident.Summary))
			result.WriteString(fmt.Sprintf("   Root Cause: %s\n", incident.RootCause))
			result.WriteString(fmt.Sprintf("   Severity: %s\n", incident.Severity))
			result.WriteString(fmt.Sprintf("   Relevance: %s\n\n", incident.Relevance))
		}
	}

	// Add pattern matches
	if len(insights.PatternMatches) > 0 {
		result.WriteString("IDENTIFIED PATTERNS:\n")
		for i, match := range insights.PatternMatches {
			result.WriteString(fmt.Sprintf("%d. Pattern: %s (Confidence: %.2f%%)\n", i+1, match.Pattern.Name, match.Confidence*100))
			result.WriteString(fmt.Sprintf("   Description: %s\n", match.Pattern.Description))
			result.WriteString(fmt.Sprintf("   Root Cause: %s\n", match.Pattern.RootCause))
			result.WriteString(fmt.Sprintf("   Common Fixes: %s\n", strings.Join(match.Pattern.CommonFixes, "; ")))
			result.WriteString(fmt.Sprintf("   Match Reason: %s\n", match.MatchReason))
			result.WriteString(fmt.Sprintf("   Relevance: %s\n\n", match.Relevance))
		}
	}

	// Add confidence boost
	if insights.ConfidenceBoost > 0 {
		result.WriteString(fmt.Sprintf("CONFIDENCE BOOST: %.2f%%\n", insights.ConfidenceBoost*100))
	}

	// Add suggested context
	if insights.SuggestedContext != "" {
		result.WriteString(fmt.Sprintf("SUGGESTED CONTEXT: %s\n", insights.SuggestedContext))
	}

	// Add learning metrics
	if insights.LearningMetrics.TotalAnalyses > 0 {
		result.WriteString(fmt.Sprintf("LEARNING METRICS:\n"))
		result.WriteString(fmt.Sprintf("- Total Analyses: %d\n", insights.LearningMetrics.TotalAnalyses))
		result.WriteString(fmt.Sprintf("- Pattern Matches: %d\n", insights.LearningMetrics.PatternMatches))
		result.WriteString(fmt.Sprintf("- Accuracy Improvement: %.2f%%\n", insights.LearningMetrics.AccuracyImprovement*100))
		result.WriteString(fmt.Sprintf("- Average Confidence: %.2f%%\n", insights.LearningMetrics.AverageConfidence*100))
	}

	return result.String()
}

// CreateFeedbackEnhancedPrompt creates a prompt that includes user feedback for better analysis
func (ls *LLMService) CreateFeedbackEnhancedPrompt(request LogAnalysisRequest, errorEntries []models.LogEntry, insights *LearningInsights, feedbackContext string) string {
	var result strings.Builder

	// Start with the base prompt
	basePrompt := ls.createDetailedErrorAnalysisPrompt(request, errorEntries, "")
	result.WriteString(basePrompt)

	// Add learning insights
	if insights != nil {
		learningContext := ls.CreateDetailedErrorAnalysisPromptWithLearning(request, errorEntries, insights)
		if learningContext != "" {
			result.WriteString("\n\nLEARNING INSIGHTS:\n")
			result.WriteString(learningContext)
		}
	}

	// Add feedback context
	if feedbackContext != "" {
		result.WriteString("\n\nUSER FEEDBACK CONTEXT:\n")
		result.WriteString(feedbackContext)
		result.WriteString("\n\nIMPORTANT: Consider the user feedback above when analyzing similar patterns and root causes. If users have corrected similar analyses in the past, incorporate those corrections into your analysis.")
	}

	return result.String()
}

// formatLearningInsights converts LearningInsights to a formatted string for LLM consumption
func (ls *LLMService) formatLearningInsights(insights *LearningInsights) string {
	if insights == nil {
		return ""
	}

	var result strings.Builder

	// Add similar incidents
	if len(insights.SimilarIncidents) > 0 {
		result.WriteString("SIMILAR PAST INCIDENTS:\n")
		for i, incident := range insights.SimilarIncidents {
			result.WriteString(fmt.Sprintf("%d. File: %s (Similarity: %.2f%%)\n", i+1, incident.Filename, incident.Similarity*100))
			result.WriteString(fmt.Sprintf("   Summary: %s\n", incident.Summary))
			result.WriteString(fmt.Sprintf("   Root Cause: %s\n", incident.RootCause))
			result.WriteString(fmt.Sprintf("   Severity: %s\n", incident.Severity))
			result.WriteString(fmt.Sprintf("   Relevance: %s\n\n", incident.Relevance))
		}
	}

	// Add pattern matches
	if len(insights.PatternMatches) > 0 {
		result.WriteString("IDENTIFIED PATTERNS:\n")
		for i, match := range insights.PatternMatches {
			result.WriteString(fmt.Sprintf("%d. Pattern: %s (Confidence: %.2f%%)\n", i+1, match.Pattern.Name, match.Confidence*100))
			result.WriteString(fmt.Sprintf("   Description: %s\n", match.Pattern.Description))
			result.WriteString(fmt.Sprintf("   Root Cause: %s\n", match.Pattern.RootCause))
			result.WriteString(fmt.Sprintf("   Common Fixes: %s\n", strings.Join(match.Pattern.CommonFixes, "; ")))
			result.WriteString(fmt.Sprintf("   Match Reason: %s\n", match.MatchReason))
			result.WriteString(fmt.Sprintf("   Relevance: %s\n\n", match.Relevance))
		}
	}

	// Add confidence boost
	if insights.ConfidenceBoost > 0 {
		result.WriteString(fmt.Sprintf("CONFIDENCE BOOST: %.2f%%\n", insights.ConfidenceBoost*100))
	}

	// Add suggested context
	if insights.SuggestedContext != "" {
		result.WriteString(fmt.Sprintf("SUGGESTED CONTEXT: %s\n", insights.SuggestedContext))
	}

	// Add learning metrics
	if insights.LearningMetrics.TotalAnalyses > 0 {
		result.WriteString(fmt.Sprintf("LEARNING METRICS:\n"))
		result.WriteString(fmt.Sprintf("- Total Analyses: %d\n", insights.LearningMetrics.TotalAnalyses))
		result.WriteString(fmt.Sprintf("- Pattern Matches: %d\n", insights.LearningMetrics.PatternMatches))
		result.WriteString(fmt.Sprintf("- Accuracy Improvement: %.2f%%\n", insights.LearningMetrics.AccuracyImprovement*100))
		result.WriteString(fmt.Sprintf("- Average Confidence: %.2f%%\n", insights.LearningMetrics.AverageConfidence*100))
	}

	return result.String()
}

// callLLMWithContext makes an LLM call with context information for tracking
func (ls *LLMService) callLLMWithContext(prompt string, logFileID *uint, jobID *uint, callType string) (string, error) {
	startTime := time.Now()

	request := OllamaGenerateRequest{
		Model:  ls.llmModel,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.2,
			"top_p":       0.8,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, map[string]interface{}{"prompt": prompt, "error": "marshal_failed"}, 0, time.Since(startTime), "", fmt.Sprintf("failed to marshal request: %v", err))
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", ls.baseURL)
	logger.Debug("Making LLM request", map[string]interface{}{
		"url":           url,
		"prompt_length": len(prompt),
		"model":         ls.llmModel,
		"call_type":     callType,
	})

	payload := map[string]interface{}{"prompt": prompt, "prompt_length": len(prompt)}

	resp, err := ls.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	elapsed := time.Since(startTime)

	if err != nil {
		logger.WithError(err, "llm_service").Error("LLM request failed", map[string]interface{}{
			"elapsed": elapsed,
		})
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, 0, elapsed, "", fmt.Sprintf("HTTP request failed: %v", err))
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	logger.Debug("LLM request completed", map[string]interface{}{
		"elapsed":     elapsed,
		"status_code": resp.StatusCode,
	})

	if resp.StatusCode != http.StatusOK {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		logger.WithError(fmt.Errorf("status %d: %s", resp.StatusCode, string(respBodyBytes)), "llm_service").Error("Ollama API returned error status")
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes)))
		return "", fmt.Errorf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		logger.WithError(err, "llm_service").Error("Failed to decode Ollama response")
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("failed to decode Ollama response: %v", err))
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	logger.Debug("LLM response decoded successfully", map[string]interface{}{
		"response_length": len(ollamaResp.Response),
		"model":           ollamaResp.Model,
	})

	ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, resp.StatusCode, elapsed, ollamaResp.Response, "")

	return ollamaResp.Response, nil
}

// callLLMWithContextAndTimeout makes an LLM call with context information and timeout
func (ls *LLMService) callLLMWithContextAndTimeout(prompt string, logFileID *uint, jobID *uint, callType string, timeout int) (string, error) {
	startTime := time.Now()

	request := OllamaGenerateRequest{
		Model:  ls.llmModel,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.2,
			"top_p":       0.8,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, map[string]interface{}{"prompt": prompt, "error": "marshal_failed"}, 0, time.Since(startTime), "", fmt.Sprintf("failed to marshal request: %v", err))
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", ls.baseURL)
	logger.Debug("Making LLM request", map[string]interface{}{
		"url":           url,
		"prompt_length": len(prompt),
		"model":         ls.llmModel,
		"call_type":     callType,
		"timeout":       timeout,
	})

	payload := map[string]interface{}{"prompt": prompt, "prompt_length": len(prompt), "timeout": timeout}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, 0, time.Since(startTime), "", fmt.Sprintf("failed to create request: %v", err))
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ls.client.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		logger.WithError(err, "llm_service").Error("LLM request failed", map[string]interface{}{
			"elapsed": elapsed,
		})
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, 0, elapsed, "", fmt.Sprintf("HTTP request failed: %v", err))
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	logger.Debug("LLM request completed", map[string]interface{}{
		"elapsed":     elapsed,
		"status_code": resp.StatusCode,
	})

	if resp.StatusCode != http.StatusOK {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		logger.WithError(fmt.Errorf("status %d: %s", resp.StatusCode, string(respBodyBytes)), "llm_service").Error("Ollama API returned error status")
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes)))
		return "", fmt.Errorf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		logger.WithError(err, "llm_service").Error("Failed to decode Ollama response")
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("failed to decode Ollama response: %v", err))
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	logger.Debug("LLM response decoded successfully", map[string]interface{}{
		"response_length": len(ollamaResp.Response),
		"model":           ollamaResp.Model,
	})

	ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, resp.StatusCode, elapsed, ollamaResp.Response, "")

	return ollamaResp.Response, nil
}

// callLLMWithEndpoint makes an LLM call to a specific endpoint
func (ls *LLMService) callLLMWithEndpoint(prompt string, endpoint string, logFileID *uint, jobID *uint, callType string) (string, error) {
	startTime := time.Now()

	request := OllamaGenerateRequest{
		Model:  ls.llmModel,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.2,
			"top_p":       0.8,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, map[string]interface{}{"prompt": prompt, "error": "marshal_failed"}, 0, time.Since(startTime), "", fmt.Sprintf("failed to marshal request: %v", err))
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", endpoint)
	logger.Debug("Making LLM request to custom endpoint", map[string]interface{}{
		"url":           url,
		"prompt_length": len(prompt),
		"model":         ls.llmModel,
		"call_type":     callType,
	})

	payload := map[string]interface{}{"prompt": prompt, "prompt_length": len(prompt)}

	resp, err := ls.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	elapsed := time.Since(startTime)

	if err != nil {
		logger.WithError(err, "llm_service").Error("LLM request failed", map[string]interface{}{
			"elapsed": elapsed,
		})
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, 0, elapsed, "", fmt.Sprintf("HTTP request failed: %v", err))
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	logger.Debug("LLM request completed", map[string]interface{}{
		"elapsed":     elapsed,
		"status_code": resp.StatusCode,
	})

	if resp.StatusCode != http.StatusOK {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		logger.WithError(fmt.Errorf("status %d: %s", resp.StatusCode, string(respBodyBytes)), "llm_service").Error("Ollama API returned error status")
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes)))
		return "", fmt.Errorf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		logger.WithError(err, "llm_service").Error("Failed to decode Ollama response")
		ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("failed to decode Ollama response: %v", err))
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	logger.Debug("LLM response decoded successfully", map[string]interface{}{
		"response_length": len(ollamaResp.Response),
		"model":           ollamaResp.Model,
	})

	ls.TrackAPICall(logFileID, jobID, callType, ls.llmModel, payload, resp.StatusCode, elapsed, ollamaResp.Response, "")

	return ollamaResp.Response, nil
}

// callLLMWithEndpointAndTimeout makes an LLM call to a specific endpoint with timeout and context
func (ls *LLMService) callLLMWithEndpointAndTimeout(ctx context.Context, prompt string, endpoint string, logFileID *uint, jobID *uint, callType string, timeout int, model string) (string, error) {
	startTime := time.Now()

	// Use the provided model if specified, otherwise use the default model
	selectedModel := model
	if selectedModel == "" {
		selectedModel = ls.llmModel
	}

	request := OllamaGenerateRequest{
		Model:  selectedModel,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.2,
			"top_p":       0.8,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		ls.TrackAPICall(logFileID, jobID, callType, selectedModel, map[string]interface{}{"prompt": prompt, "error": "marshal_failed"}, 0, time.Since(startTime), "", fmt.Sprintf("failed to marshal request: %v", err))
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", endpoint)
	logger.Debug("Making LLM request to custom endpoint with timeout", map[string]interface{}{
		"url":           url,
		"prompt_length": len(prompt),
		"model":         selectedModel,
		"call_type":     callType,
		"timeout":       timeout,
	})

	payload := map[string]interface{}{"prompt": prompt, "prompt_length": len(prompt), "timeout": timeout}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		ls.TrackAPICall(logFileID, jobID, callType, selectedModel, payload, 0, time.Since(startTime), "", fmt.Sprintf("failed to create request: %v", err))
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a custom HTTP client with the specified timeout
	customClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := customClient.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		logger.WithError(err, "llm_service").Error("LLM request failed", map[string]interface{}{
			"elapsed": elapsed,
		})
		ls.TrackAPICall(logFileID, jobID, callType, selectedModel, payload, 0, elapsed, "", fmt.Sprintf("HTTP request failed: %v", err))
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	logger.Debug("LLM request completed", map[string]interface{}{
		"elapsed":     elapsed,
		"status_code": resp.StatusCode,
	})

	if resp.StatusCode != http.StatusOK {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		logger.WithError(fmt.Errorf("status %d: %s", resp.StatusCode, string(respBodyBytes)), "llm_service").Error("Ollama API returned error status")
		ls.TrackAPICall(logFileID, jobID, callType, selectedModel, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes)))
		return "", fmt.Errorf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		logger.WithError(err, "llm_service").Error("Failed to decode Ollama response")
		ls.TrackAPICall(logFileID, jobID, callType, selectedModel, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("failed to decode Ollama response: %v", err))
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	logger.Debug("LLM response decoded successfully", map[string]interface{}{
		"response_length": len(ollamaResp.Response),
		"model":           ollamaResp.Model,
	})

	ls.TrackAPICall(logFileID, jobID, callType, selectedModel, payload, resp.StatusCode, elapsed, ollamaResp.Response, "")

	return ollamaResp.Response, nil
}

// extractAndCleanJSON attempts to extract valid JSON from a potentially malformed response
func (ls *LLMService) extractAndCleanJSON(response string) string {
	response = strings.TrimSpace(response)

	// Remove markdown code blocks if present
	if strings.Contains(response, "```json") {
		start := strings.Index(response, "```json")
		end := strings.LastIndex(response, "```")
		if start != -1 && end != -1 && end > start {
			response = response[start+7 : end]
		}
	} else if strings.Contains(response, "```") {
		start := strings.Index(response, "```")
		end := strings.LastIndex(response, "```")
		if start != -1 && end != -1 && end > start {
			response = response[start+3 : end]
		}
	}

	// Try to find the first { and last } to extract JSON
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start != -1 && end != -1 && end > start {
		response = response[start : end+1]
	}

	// Clean up common issues
	response = strings.TrimSpace(response)

	// Fix double-escaped quotes and other escape sequences
	response = strings.ReplaceAll(response, `\"`, `"`)
	response = strings.ReplaceAll(response, `\\n`, `\n`)
	response = strings.ReplaceAll(response, `\\t`, `\t`)
	response = strings.ReplaceAll(response, `\\`, `\`)

	return response
}

// createFallbackAnalysis creates a basic analysis when JSON parsing completely fails
func (ls *LLMService) createFallbackAnalysis(response string) *LogAnalysisResponse {
	// Try to extract basic information from the response text
	response = strings.ToLower(response)

	// Look for common patterns in the response
	var summary, rootCause string
	var severity string = "medium"
	var recommendations []string

	// Extract summary if present
	if strings.Contains(response, "summary") {
		// Try to find content after "summary"
		parts := strings.Split(response, "summary")
		if len(parts) > 1 {
			summary = strings.TrimSpace(parts[1])
			// Clean up the summary
			if strings.Contains(summary, "rootcause") || strings.Contains(summary, "recommendations") {
				summary = strings.Split(summary, "rootcause")[0]
				summary = strings.Split(summary, "recommendations")[0]
			}
			summary = strings.TrimSpace(summary)
			// Remove quotes and extra punctuation
			summary = strings.Trim(summary, `"':,.`)
		}
	}

	// Extract root cause if present
	if strings.Contains(response, "rootcause") {
		parts := strings.Split(response, "rootcause")
		if len(parts) > 1 {
			rootCause = strings.TrimSpace(parts[1])
			if strings.Contains(rootCause, "recommendations") {
				rootCause = strings.Split(rootCause, "recommendations")[0]
			}
			rootCause = strings.TrimSpace(rootCause)
			rootCause = strings.Trim(rootCause, `"':,.`)
		}
	}

	// Determine severity based on keywords
	if strings.Contains(response, "critical") || strings.Contains(response, "fatal") {
		severity = "critical"
	} else if strings.Contains(response, "high") || strings.Contains(response, "major") {
		severity = "high"
	} else if strings.Contains(response, "low") || strings.Contains(response, "minor") {
		severity = "low"
	}

	// Add basic recommendations
	recommendations = append(recommendations, "Review the log entries for more details")
	recommendations = append(recommendations, "Check system configuration and dependencies")

	// If we couldn't extract meaningful information, provide defaults
	if summary == "" {
		summary = "Analysis completed but response format was invalid"
	}
	if rootCause == "" {
		rootCause = "Unable to determine root cause due to response parsing issues"
	}

	return &LogAnalysisResponse{
		Summary:           summary,
		Severity:          severity,
		RootCause:         rootCause,
		Recommendations:   recommendations,
		ErrorAnalysis:     []DetailedErrorAnalysis{},
		CriticalErrors:    0,
		NonCriticalErrors: 0,
	}
}

// parseDetailedLLMResponse parses the LLM response into a LogAnalysisResponse
func (ls *LLMService) parseDetailedLLMResponse(response string) (*LogAnalysisResponse, error) {
	// Clean the response - remove any markdown formatting
	cleanResponse := ls.extractAndCleanJSON(response)

	// Try to parse the JSON as-is first
	var analysis LogAnalysisResponse
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		logger.WithError(err, "llm_service").Error("Failed to parse LLM response as JSON")
		logger.Debug("Raw LLM response", map[string]interface{}{"response": response})
		logger.Debug("Cleaned response", map[string]interface{}{"response": cleanResponse})

		// Try to create a fallback analysis from the text
		fallbackAnalysis := ls.createFallbackAnalysis(response)
		if fallbackAnalysis.Summary == "" || fallbackAnalysis.RootCause == "" {
			return nil, fmt.Errorf("failed to parse LLM response and fallback analysis is incomplete: %w", err)
		}
		return fallbackAnalysis, nil
	}

	// Validate the parsed analysis
	if analysis.Summary == "" || analysis.RootCause == "" {
		logger.WithError(fmt.Errorf("incomplete analysis"), "llm_service").Error("LLM returned incomplete analysis")
		return nil, fmt.Errorf("LLM returned incomplete analysis (missing summary or root cause)")
	}

	// Normalize severity
	analysis.Severity = ls.normalizeSeverity(analysis.Severity)

	// Process error analysis entries
	for i := range analysis.ErrorAnalysis {
		analysis.ErrorAnalysis[i].Severity = ls.normalizeErrorSeverity(analysis.ErrorAnalysis[i].Severity)
		analysis.ErrorAnalysis[i].ErrorPattern = ls.extractErrorPattern(analysis.ErrorAnalysis[i].ErrorPattern)
	}

	return &analysis, nil
}

// normalizeSeverity normalizes severity values to standard format
func (ls *LLMService) normalizeSeverity(severity string) string {
	severity = strings.ToLower(strings.TrimSpace(severity))
	switch severity {
	case "critical", "fatal", "severe":
		return "high"
	case "high", "major":
		return "high"
	case "medium", "moderate":
		return "medium"
	case "low", "minor", "info":
		return "low"
	default:
		return "medium"
	}
}

// normalizeErrorSeverity normalizes error severity values
func (ls *LLMService) normalizeErrorSeverity(severity string) string {
	severity = strings.ToLower(strings.TrimSpace(severity))
	switch severity {
	case "critical", "fatal", "severe":
		return "critical"
	case "non-critical", "noncritical", "minor", "low":
		return "non-critical"
	default:
		return "non-critical"
	}
}

func (ls *LLMService) extractErrorPattern(message string) string {
	// Extract a simplified pattern from error message
	message = strings.ToLower(message)

	// Common error patterns
	if strings.Contains(message, "connection") && strings.Contains(message, "timeout") {
		return "Connection Timeout"
	}
	if strings.Contains(message, "authentication") || strings.Contains(message, "auth") {
		return "Authentication Error"
	}
	if strings.Contains(message, "database") || strings.Contains(message, "db") {
		return "Database Error"
	}
	if strings.Contains(message, "permission") || strings.Contains(message, "access") {
		return "Permission/Access Error"
	}
	if strings.Contains(message, "not found") || strings.Contains(message, "404") {
		return "Resource Not Found"
	}
	if strings.Contains(message, "timeout") {
		return "Timeout Error"
	}
	if strings.Contains(message, "memory") || strings.Contains(message, "oom") {
		return "Memory Error"
	}

	// Default pattern
	return "General Error"
}

// CheckLLMStatus performs a lightweight health check (status endpoint)
func (ls *LLMService) CheckLLMStatus() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := fmt.Sprintf("%s/api/health", ls.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create status check request: %w", err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("LLM status check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LLM status check failed: status %d", resp.StatusCode)
	}
	return nil
}

// CheckLLMGenerate verifies if the local LLM can generate (test prompt)
func (ls *LLMService) CheckLLMGenerate() error {
	// Use a simple health check prompt to verify LLM functionality
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	request := OllamaGenerateRequest{
		Model:  ls.llmModel,
		Prompt: HEALTH_CHECK_PROMPT,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.2,
			"top_p":       0.8,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal health check request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", ls.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use a local client with a 3s timeout for health check only
	healthClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("LLM service not available: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		return fmt.Errorf("LLM health check failed: status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return fmt.Errorf("failed to decode LLM health check response: %w", err)
	}

	if strings.TrimSpace(ollamaResp.Response) != "OK" {
		return fmt.Errorf("LLM health check did not return OK: %s", ollamaResp.Response)
	}

	return nil
}

// CheckLLMStatusWithEndpoint performs a lightweight health check for a specific endpoint
func (ls *LLMService) CheckLLMStatusWithEndpoint(endpoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := fmt.Sprintf("%s/api/tags", endpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create status check request: %w", err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("LLM status check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LLM status check failed: status %d", resp.StatusCode)
	}
	return nil
}

// CheckLLMGenerateWithEndpoint verifies if the LLM can generate for a specific endpoint
func (ls *LLMService) CheckLLMGenerateWithEndpoint(endpoint string) error {
	// Use a simple health check prompt to verify LLM functionality
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	request := OllamaGenerateRequest{
		Model:  ls.llmModel,
		Prompt: HEALTH_CHECK_PROMPT,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.2,
			"top_p":       0.8,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal health check request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use a local client with a 3s timeout for health check only
	healthClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("LLM service not available: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		return fmt.Errorf("LLM health check failed: status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return fmt.Errorf("failed to decode LLM health check response: %w", err)
	}

	if strings.TrimSpace(ollamaResp.Response) != "OK" {
		return fmt.Errorf("LLM health check did not return OK: %s", ollamaResp.Response)
	}

	return nil
}

type OllamaModelsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// GetAvailableModels returns the list of available models
func (ls *LLMService) GetAvailableModels() ([]string, error) {
	url := fmt.Sprintf("%s/api/tags", ls.baseURL)
	resp, err := ls.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models: status %d", resp.StatusCode)
	}

	var modelsResp OllamaModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	var modelNames []string
	for _, model := range modelsResp.Models {
		modelNames = append(modelNames, model.Name)
	}

	return modelNames, nil
}

// GetAvailableModelsWithEndpoint returns the list of available models from a specific endpoint
func (ls *LLMService) GetAvailableModelsWithEndpoint(endpoint string) ([]string, error) {
	url := fmt.Sprintf("%s/api/tags", endpoint)

	// Create a client with timeout for this specific request
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models: status %d", resp.StatusCode)
	}

	var modelsResp OllamaModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	var modelNames []string
	for _, model := range modelsResp.Models {
		modelNames = append(modelNames, model.Name)
	}

	return modelNames, nil
}

// GenerateEmbedding generates an embedding for the given text using Ollama
func (ls *LLMService) GenerateEmbedding(text string) ([]float32, error) {
	url := ls.baseURL + "/api/embeddings"
	request := OllamaEmbeddingRequest{
		Model:  ls.embedModel, // Use embedding model
		Prompt: text,
	}
	body, _ := json.Marshal(request)
	resp, err := ls.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("embedding API returned status %d", resp.StatusCode)
	}
	var embeddingResp OllamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, err
	}
	return embeddingResp.Embedding, nil
}

// GenerateEmbeddingForAnalysis generates an embedding specifically for log analysis using the optimized prompt
func (ls *LLMService) GenerateEmbeddingForAnalysis(summary, rootCause, severity string, errorPatterns []string) ([]float32, error) {
	patternsText := strings.Join(errorPatterns, ", ")
	prompt := fmt.Sprintf(EMBEDDING_PROMPT, summary, rootCause, severity, patternsText)
	return ls.GenerateEmbedding(prompt)
}

// CreateAggregationPrompt creates a prompt for aggregating multiple chunk analyses
func (ls *LLMService) CreateAggregationPrompt(logFileName string, chunkAnalyses []string) string {
	analysesText := strings.Join(chunkAnalyses, "\n\n---\n\n")
	return fmt.Sprintf(RCA_AGGREGATION_PROMPT, logFileName, analysesText)
}

// FindSimilarAnalyses finds the top-N most similar past analyses by embedding cosine similarity
func (ls *LLMService) FindSimilarAnalyses(db *gorm.DB, embedding []float32, topN int) ([]models.LogAnalysisMemory, error) {
	var memories []models.LogAnalysisMemory
	if err := db.Find(&memories).Error; err != nil {
		return nil, err
	}
	type scored struct {
		mem   models.LogAnalysisMemory
		score float64
	}
	var scoredList []scored
	for _, mem := range memories {
		var emb []float32
		if mem.Embedding != nil {
			if embBytes, err := json.Marshal(mem.Embedding); err == nil {
				if err := json.Unmarshal(embBytes, &emb); err == nil && len(emb) == len(embedding) {
					score := cosineSimilarity(embedding, emb)
					scoredList = append(scoredList, scored{mem, score})
				}
			}
		}
	}
	// Sort by descending similarity
	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].score > scoredList[j].score })
	var top []models.LogAnalysisMemory
	for i := 0; i < len(scoredList) && i < topN; i++ {
		top = append(top, scoredList[i].mem)
	}
	return top, nil
}

// cosineSimilarity computes cosine similarity between two float32 slices
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// InferLogFormatFromSamples asks the LLM to infer a log format string from sample log lines
func (ls *LLMService) InferLogFormatFromSamples(samples []string, logFileID *uint) (string, error) {
	prompt := fmt.Sprintf(LOG_FORMAT_INFERENCE_PROMPT, strings.Join(samples, "\n"))
	logger.Debug("Prompting LLM for log format inference", map[string]interface{}{
		"prompt_length": len(prompt),
	})
	resp, err := ls.callLLMWithContext(prompt, logFileID, nil, "format_inference")
	if err != nil {
		return "", fmt.Errorf("LLM format inference failed: %w", err)
	}

	// Clean the response - remove any explanatory text
	resp = strings.TrimSpace(resp)

	// Remove common prefixes that LLMs might add
	prefixes := []string{
		"The log format string for logpai/logparser can be inferred as follows:",
		"The log format is:",
		"Based on the log lines, the format is:",
		"Format string:",
		"Log format:",
		"Here is the logpai/logparser format string for the given log lines:",
		"The format string is:",
		"Logpai/logparser format string:",
		"Here is the format string:",
		"The format is:",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(resp, prefix) {
			resp = strings.TrimSpace(strings.TrimPrefix(resp, prefix))
			break
		}
	}

	// Remove quotes if present
	resp = strings.Trim(resp, `"'`)

	// Take only the first line if multiple lines
	if idx := strings.Index(resp, "\n"); idx != -1 {
		resp = strings.TrimSpace(resp[:idx])
	}

	// Validate that it looks like a format string (contains angle brackets)
	if !strings.Contains(resp, "<") || !strings.Contains(resp, ">") {
		logger.WithError(fmt.Errorf("LLM did not return a valid format string: %q", resp), "llm_service").Error("LLM did not return a valid format string")
		return "", fmt.Errorf("LLM did not return a valid format string: %q", resp)
	}

	logger.Debug("LLM returned valid format string", map[string]interface{}{
		"format_string": resp,
	})

	return resp, nil
}

func (ls *LLMService) PullModelWithEndpoint(model, endpoint string) error {
	url := endpoint + "/api/pull"
	payload := map[string]string{"name": model}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Failed to pull model: %s", string(body))
	}
	return nil
}
