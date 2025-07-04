package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/incident-sage/backend/internal/models"
)

type LLMService struct {
	baseURL string
	model   string
	client  *http.Client
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
	Summary           string                  `json:"summary"`
	Severity          string                  `json:"severity"`
	RootCause         string                  `json:"rootCause"`
	Recommendations   []string                `json:"recommendations"`
	IncidentType      string                  `json:"incidentType"`
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
	Severity        string   `json:"severity"` // "critical" or "non-critical"
	RootCause       string   `json:"rootCause"`
	Impact          string   `json:"impact"`
	Fix             string   `json:"fix"`
	RelatedErrors   []string `json:"relatedErrors"`
}

func NewLLMService(ollamaURL, model string) *LLMService {
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama2"
	}

	return &LLMService{
		baseURL: ollamaURL,
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// AnalyzeLogsWithAI performs AI-powered analysis of log entries with focus on errors
func (ls *LLMService) AnalyzeLogsWithAI(logFile models.LogFile, entries []models.LogEntry) (*LogAnalysisResponse, error) {
	// Filter only ERROR and FATAL entries for analysis
	errorEntries := ls.filterErrorEntries(entries)

	if len(errorEntries) == 0 {
		// No errors found, return basic analysis
		return ls.generateNoErrorsAnalysis(logFile), nil
	}

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
	prompt := ls.createDetailedErrorAnalysisPrompt(request, errorEntries)

	// Call the local LLM
	response, err := ls.callLLM(prompt)
	if err != nil {
		log.Printf("LLM analysis failed: %v", err)
		// Fallback to basic error analysis
		return ls.generateFallbackErrorAnalysis(request, errorEntries), nil
	}

	// Parse the LLM response
	analysis, err := ls.parseDetailedLLMResponse(response)
	if err != nil {
		log.Printf("Failed to parse LLM response: %v", err)
		// Fallback to basic error analysis
		return ls.generateFallbackErrorAnalysis(request, errorEntries), nil
	}

	return analysis, nil
}

// Filter only ERROR and FATAL log entries
func (ls *LLMService) filterErrorEntries(entries []models.LogEntry) []models.LogEntry {
	var errorEntries []models.LogEntry
	for _, entry := range entries {
		if entry.Level == models.LogLevelError || entry.Level == models.LogLevelFatal {
			errorEntries = append(errorEntries, entry)
		}
	}
	return errorEntries
}

func (ls *LLMService) createDetailedErrorAnalysisPrompt(request LogAnalysisRequest, errorEntries []models.LogEntry) string {
	// Create a structured prompt focused on error analysis
	prompt := fmt.Sprintf(`You are an expert DevOps/SRE engineer performing detailed Root Cause Analysis (RCA) on system errors. 

Analyze the following ERROR and FATAL log entries to provide a comprehensive error analysis:

LOG FILE: %s
TOTAL ERROR ENTRIES: %d
TIME RANGE: %s to %s

ERROR ENTRIES TO ANALYZE:
`, request.Filename, len(errorEntries),
		request.StartTime.Format("2006-01-02 15:04:05"), request.EndTime.Format("2006-01-02 15:04:05"))

	// Add all error entries (no limit for errors as they're critical)
	for _, entry := range errorEntries {
		prompt += fmt.Sprintf("[%s] %s: %s\n",
			entry.Timestamp.Format("15:04:05"),
			entry.Level,
			entry.Message)

		// Add metadata if present
		if entry.Metadata != nil && len(entry.Metadata) > 0 {
			metadata, _ := json.Marshal(entry.Metadata)
			prompt += fmt.Sprintf("  Metadata: %s\n", string(metadata))
		}
	}

	prompt += `

Perform a detailed Root Cause Analysis and provide your findings in the following JSON format:

{
  "summary": "A concise summary focusing on the most critical errors and their impact (2-3 sentences)",
  "severity": "low|medium|high|critical",
  "rootCause": "The primary root cause that explains most of the errors",
  "recommendations": ["specific_action1", "specific_action2", "specific_action3"],
  "incidentType": "The type of incident (e.g., 'Database Connection Failure', 'API Authentication Error', 'Service Outage')",
  "errorAnalysis": [
    {
      "errorPattern": "The pattern or category of this error (e.g., 'Database Connection Timeout', 'Authentication Failure')",
      "errorCount": 5,
      "firstOccurrence": "2024-01-15 10:30:00",
      "lastOccurrence": "2024-01-15 11:45:00",
      "severity": "critical|non-critical",
      "rootCause": "Specific root cause for this error pattern",
      "impact": "What is broken or affected by this error",
      "fix": "Specific fix or solution for this error pattern",
      "relatedErrors": ["related_error_message1", "related_error_message2"]
    }
  ],
  "criticalErrors": 3,
  "nonCriticalErrors": 2
}

ANALYSIS REQUIREMENTS:
1. Focus ONLY on ERROR and FATAL entries - ignore INFO and DEBUG logs
2. Group similar errors into patterns
3. Classify each error pattern as "critical" or "non-critical" based on:
   - Critical: Service outages, data loss, security issues, cascading failures
   - Non-critical: Temporary issues, retryable errors, minor performance issues
4. Provide specific, actionable fixes for each error pattern
5. Explain what exactly is broken for each error
6. Identify the root cause that explains the error pattern
7. Count total critical vs non-critical errors

Respond only with valid JSON.`

	return prompt
}

func (ls *LLMService) callLLM(prompt string) (string, error) {
	request := OllamaGenerateRequest{
		Model:  ls.model,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.2, // Even lower temperature for more consistent analysis
			"top_p":       0.8,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", ls.baseURL)
	resp, err := ls.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return ollamaResp.Response, nil
}

func (ls *LLMService) parseDetailedLLMResponse(response string) (*LogAnalysisResponse, error) {
	// Clean the response - remove any markdown formatting
	cleanResponse := strings.TrimSpace(response)

	// Remove markdown code blocks if present
	if strings.HasPrefix(cleanResponse, "```json") {
		cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	}
	if strings.HasPrefix(cleanResponse, "```") {
		cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	}
	if strings.HasSuffix(cleanResponse, "```") {
		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	}

	cleanResponse = strings.TrimSpace(cleanResponse)

	var analysis LogAnalysisResponse
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Validate and normalize the response
	if analysis.Summary == "" {
		analysis.Summary = "Error analysis completed but no summary generated."
	}

	// Normalize severity
	analysis.Severity = ls.normalizeSeverity(analysis.Severity)

	if analysis.IncidentType == "" {
		analysis.IncidentType = "System Error"
	}

	// Validate error analysis
	if analysis.ErrorAnalysis == nil {
		analysis.ErrorAnalysis = []DetailedErrorAnalysis{}
	}

	// Normalize error analysis severity
	for i := range analysis.ErrorAnalysis {
		analysis.ErrorAnalysis[i].Severity = ls.normalizeErrorSeverity(analysis.ErrorAnalysis[i].Severity)
	}

	return &analysis, nil
}

func (ls *LLMService) normalizeSeverity(severity string) string {
	severity = strings.ToLower(strings.TrimSpace(severity))

	switch severity {
	case "low", "minor":
		return "low"
	case "medium", "moderate":
		return "medium"
	case "high", "major":
		return "high"
	case "critical", "fatal":
		return "critical"
	default:
		return "medium"
	}
}

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

func (ls *LLMService) generateNoErrorsAnalysis(logFile models.LogFile) *LogAnalysisResponse {
	return &LogAnalysisResponse{
		Summary:           fmt.Sprintf("Log file '%s' contains no ERROR or FATAL entries. System appears to be functioning normally.", logFile.Filename),
		Severity:          "low",
		RootCause:         "No errors detected",
		Recommendations:   []string{"Continue monitoring for any new errors", "Review INFO and WARNING logs for potential issues"},
		IncidentType:      "No Incident",
		ErrorAnalysis:     []DetailedErrorAnalysis{},
		CriticalErrors:    0,
		NonCriticalErrors: 0,
	}
}

func (ls *LLMService) generateFallbackErrorAnalysis(request LogAnalysisRequest, errorEntries []models.LogEntry) *LogAnalysisResponse {
	// Generate a basic error analysis when LLM is not available
	summary := fmt.Sprintf("Log file '%s' contains %d error entries that require manual analysis.",
		request.Filename, len(errorEntries))

	severity := "medium"
	if len(errorEntries) > 10 {
		severity = "high"
	} else if len(errorEntries) > 5 {
		severity = "medium"
	}

	rootCause := "Multiple system errors detected requiring investigation"

	recommendations := []string{
		"Manually review each error entry for patterns",
		"Check system resources and dependencies",
		"Review recent deployments or configuration changes",
		"Monitor system metrics during error periods",
	}

	incidentType := "System Error Investigation"

	// Create basic error analysis
	var errorAnalysis []DetailedErrorAnalysis
	criticalCount := 0
	nonCriticalCount := 0

	// Group errors by message pattern
	errorPatterns := make(map[string][]models.LogEntry)
	for _, entry := range errorEntries {
		pattern := ls.extractErrorPattern(entry.Message)
		errorPatterns[pattern] = append(errorPatterns[pattern], entry)
	}

	for pattern, entries := range errorPatterns {
		errorSeverity := "non-critical"
		if len(entries) > 3 || strings.Contains(strings.ToLower(pattern), "fatal") {
			errorSeverity = "critical"
			criticalCount++
		} else {
			nonCriticalCount++
		}

		analysis := DetailedErrorAnalysis{
			ErrorPattern:    pattern,
			ErrorCount:      len(entries),
			FirstOccurrence: entries[0].Timestamp.Format("2006-01-02 15:04:05"),
			LastOccurrence:  entries[len(entries)-1].Timestamp.Format("2006-01-02 15:04:05"),
			Severity:        errorSeverity,
			RootCause:       "Requires manual investigation",
			Impact:          "System functionality may be affected",
			Fix:             "Investigate and resolve based on error pattern",
			RelatedErrors:   []string{},
		}
		errorAnalysis = append(errorAnalysis, analysis)
	}

	return &LogAnalysisResponse{
		Summary:           summary,
		Severity:          severity,
		RootCause:         rootCause,
		Recommendations:   recommendations,
		IncidentType:      incidentType,
		ErrorAnalysis:     errorAnalysis,
		CriticalErrors:    criticalCount,
		NonCriticalErrors: nonCriticalCount,
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

// CheckLLMHealth verifies if the local LLM is available
func (ls *LLMService) CheckLLMHealth() error {
	url := fmt.Sprintf("%s/api/tags", ls.baseURL)
	resp, err := ls.client.Get(url)
	if err != nil {
		return fmt.Errorf("LLM service not available: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LLM service returned status %d", resp.StatusCode)
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
