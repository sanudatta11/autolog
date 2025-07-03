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

type LogAnalysisResponse struct {
	Summary         string   `json:"summary"`
	Severity        string   `json:"severity"`
	RootCause       string   `json:"rootCause"`
	Recommendations []string `json:"recommendations"`
	IncidentType    string   `json:"incidentType"`
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

// AnalyzeLogsWithAI performs AI-powered analysis of log entries
func (ls *LLMService) AnalyzeLogsWithAI(logFile models.LogFile, entries []models.LogEntry) (*LogAnalysisResponse, error) {
	// Prepare the analysis request
	request := LogAnalysisRequest{
		LogEntries:   entries,
		ErrorCount:   logFile.ErrorCount,
		WarningCount: logFile.WarningCount,
		Filename:     logFile.Filename,
	}

	if len(entries) > 0 {
		request.StartTime = entries[0].Timestamp
		request.EndTime = entries[len(entries)-1].Timestamp
	}

	// Create the prompt for the LLM
	prompt := ls.createAnalysisPrompt(request)

	// Call the local LLM
	response, err := ls.callLLM(prompt)
	if err != nil {
		log.Printf("LLM analysis failed: %v", err)
		// Fallback to basic analysis
		return ls.generateFallbackAnalysis(request), nil
	}

	// Parse the LLM response
	analysis, err := ls.parseLLMResponse(response)
	if err != nil {
		log.Printf("Failed to parse LLM response: %v", err)
		// Fallback to basic analysis
		return ls.generateFallbackAnalysis(request), nil
	}

	return analysis, nil
}

func (ls *LLMService) createAnalysisPrompt(request LogAnalysisRequest) string {
	// Create a structured prompt for log analysis
	prompt := fmt.Sprintf(`You are an expert DevOps/SRE engineer analyzing system logs for incident detection. 

Analyze the following log file and provide a comprehensive incident analysis:

LOG FILE: %s
TOTAL ENTRIES: %d
ERROR COUNT: %d
WARNING COUNT: %d
TIME RANGE: %s to %s

LOG ENTRIES:
`, request.Filename, len(request.LogEntries), request.ErrorCount, request.WarningCount,
		request.StartTime.Format("2006-01-02 15:04:05"), request.EndTime.Format("2006-01-02 15:04:05"))

	// Add log entries (limit to first 50 to avoid token limits)
	maxEntries := 50
	if len(request.LogEntries) > maxEntries {
		prompt += fmt.Sprintf("(Showing first %d entries out of %d)\n", maxEntries, len(request.LogEntries))
	}

	for i, entry := range request.LogEntries {
		if i >= maxEntries {
			break
		}
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

Please provide your analysis in the following JSON format:
{
  "summary": "A concise summary of the incident (2-3 sentences)",
  "severity": "low|medium|high|critical",
  "rootCause": "The likely root cause of the issues",
  "recommendations": ["action1", "action2", "action3"],
  "incidentType": "The type of incident (e.g., 'Database Issue', 'API Failure', 'System Outage')"
}

Focus on:
1. Identifying patterns in errors and warnings
2. Determining the root cause
3. Assessing the severity based on impact
4. Providing actionable recommendations
5. Categorizing the incident type

Respond only with valid JSON.`

	return prompt
}

func (ls *LLMService) callLLM(prompt string) (string, error) {
	request := OllamaGenerateRequest{
		Model:  ls.model,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.3, // Lower temperature for more consistent analysis
			"top_p":       0.9,
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

func (ls *LLMService) parseLLMResponse(response string) (*LogAnalysisResponse, error) {
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
		analysis.Summary = "Analysis completed but no summary generated."
	}

	// Normalize severity
	analysis.Severity = ls.normalizeSeverity(analysis.Severity)

	if analysis.IncidentType == "" {
		analysis.IncidentType = "System Issue"
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

func (ls *LLMService) generateFallbackAnalysis(request LogAnalysisRequest) *LogAnalysisResponse {
	// Generate a basic analysis when LLM is not available
	summary := fmt.Sprintf("Log file '%s' contains %d entries with %d errors and %d warnings.",
		request.Filename, len(request.LogEntries), request.ErrorCount, request.WarningCount)

	severity := "low"
	if request.ErrorCount > 10 {
		severity = "high"
	} else if request.ErrorCount > 5 {
		severity = "medium"
	}

	rootCause := "Multiple system errors detected"
	if request.ErrorCount == 0 {
		rootCause = "No critical errors found"
	}

	recommendations := []string{
		"Review error patterns in the logs",
		"Check system resources and performance",
		"Monitor for similar issues in the future",
	}

	incidentType := "System Issue"
	if request.ErrorCount > 0 {
		incidentType = "Error Investigation"
	}

	return &LogAnalysisResponse{
		Summary:         summary,
		Severity:        severity,
		RootCause:       rootCause,
		Recommendations: recommendations,
		IncidentType:    incidentType,
	}
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
