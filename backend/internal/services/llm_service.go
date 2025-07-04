package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"math"
	"sync"

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
	Severity        string   `json:"severity"` // "critical" or "non-critical"
	RootCause       string   `json:"rootCause"`
	Impact          string   `json:"impact"`
	Fix             string   `json:"fix"`
	RelatedErrors   []string `json:"relatedErrors"`
}

// Embedding request/response for Ollama

type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// LLMAPI Call tracking
type LLMAPICall struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Endpoint  string                 `json:"endpoint"`
	Model     string                 `json:"model"`
	LogFileID *uint                  `json:"logFileId,omitempty"`
	JobID     *uint                  `json:"jobId,omitempty"`
	CallType  string                 `json:"callType"` // "format_inference", "rca_analysis", "rca_aggregation", "embedding", etc.
	Payload   map[string]interface{} `json:"payload"`
	Status    int                    `json:"status"`
	Duration  time.Duration          `json:"duration"`
	Response  string                 `json:"response"`
	Error     string                 `json:"error,omitempty"`
}

func NewLLMService(ollamaURL, llmModel string) *LLMService {
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if llmModel == "" {
		llmModel = "llama2:13b"
	}
	embedModel := os.Getenv("OLLAMA_EMBED_MODEL")
	if embedModel == "" {
		embedModel = "nomic-embed-text"
	}

	// Get timeout from environment or use default
	timeoutStr := os.Getenv("OLLAMA_TIMEOUT_SECONDS")
	timeout := 300 * time.Second // Default 5 minutes
	if timeoutStr != "" {
		if t, err := time.ParseDuration(timeoutStr + "s"); err == nil {
			timeout = t
		}
	}

	return &LLMService{
		baseURL:    ollamaURL,
		llmModel:   llmModel,
		embedModel: embedModel,
		client:     &http.Client{Timeout: timeout},
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

// CreateAPICall creates a new API call with context information
func (ls *LLMService) CreateAPICall(logFileID *uint, jobID *uint, callType string) *LLMAPICall {
	return &LLMAPICall{
		ID:        fmt.Sprintf("llm_%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Endpoint:  "/api/generate",
		Model:     ls.llmModel,
		LogFileID: logFileID,
		JobID:     jobID,
		CallType:  callType,
	}
}

// TrackAPICall tracks an API call with the given context
func (ls *LLMService) TrackAPICall(logFileID *uint, jobID *uint, callType string, payload map[string]interface{}, status int, duration time.Duration, response string, err string) {
	call := ls.CreateAPICall(logFileID, jobID, callType)
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
	if logFile == nil {
		return nil, fmt.Errorf("logFile is nil")
	}

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
	prompt := ls.createDetailedErrorAnalysisPrompt(request, errorEntries, "") // Pass an empty string for similarIncidents for now

	// Call the local LLM
	response, err := ls.callLLMWithContext(prompt, &logFile.ID, jobID, "rca_analysis")
	if err != nil {
		log.Printf("LLM analysis failed: %v", err)
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Parse the LLM response
	analysis, err := ls.parseDetailedLLMResponse(response)
	if err != nil {
		log.Printf("Failed to parse LLM response: %v", err)
		return nil, fmt.Errorf("Failed to parse LLM response: %w", err)
	}

	// Validate analysis (must have summary and root cause)
	if analysis.Summary == "" || analysis.RootCause == "" {
		return nil, fmt.Errorf("LLM returned incomplete analysis (missing summary or root cause)")
	}

	return analysis, nil
}

// AnalyzeLogsWithAIWithTimeout performs AI-powered analysis of log entries with focus on errors, with a per-request timeout.
func (ls *LLMService) AnalyzeLogsWithAIWithTimeout(logFile *models.LogFile, entries []models.LogEntry, timeout int, jobID *uint) (*LogAnalysisResponse, error) {
	if logFile == nil {
		return nil, fmt.Errorf("logFile is nil")
	}
	// Filter only ERROR and FATAL entries for analysis
	errorEntries := ls.filterErrorEntries(entries)
	if len(errorEntries) == 0 {
		log.Printf("[LLM] No ERROR/FATAL entries found in chunk (%d total entries), using fallback analysis", len(entries))
		return ls.generateNoErrorsAnalysis(logFile), nil
	}

	log.Printf("[LLM] Found %d ERROR/FATAL entries in chunk (%d total entries), proceeding with LLM analysis", len(errorEntries), len(entries))

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
		log.Printf("LLM analysis failed: %v", err)
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}
	analysis, err := ls.parseDetailedLLMResponse(response)
	if err != nil {
		log.Printf("Failed to parse LLM response: %v", err)
		return nil, fmt.Errorf("Failed to parse LLM response: %w", err)
	}
	if analysis.Summary == "" || analysis.RootCause == "" {
		return nil, fmt.Errorf("LLM returned incomplete analysis (missing summary or root cause)")
	}
	return analysis, nil
}

// Filter only ERROR and FATAL log entries
func (ls *LLMService) filterErrorEntries(entries []models.LogEntry) []models.LogEntry {
	var errorEntries []models.LogEntry
	for _, entry := range entries {
		if entry.Level == "ERROR" || entry.Level == "FATAL" {
			errorEntries = append(errorEntries, entry)
		}
	}
	return errorEntries
}

func (ls *LLMService) createDetailedErrorAnalysisPrompt(request LogAnalysisRequest, errorEntries []models.LogEntry, similarIncidents string) string {
	// Create a structured prompt focused on error analysis
	prompt := fmt.Sprintf(`You are an expert DevOps/SRE engineer performing detailed Root Cause Analysis (RCA) on system errors.

Analyze the following ERROR and FATAL log entries to provide a comprehensive error analysis:

LOG FILE: %s
TOTAL ERROR ENTRIES: %d
TIME RANGE: %s to %s

ERROR ENTRIES TO ANALYZE:
`, request.Filename, len(errorEntries),
		request.StartTime.Format("2006-01-02 15:04:05"), request.EndTime.Format("2006-01-02 15:04:05"))

	// Add error entries (limit to prevent timeout on very large files)
	maxEntries := 50 // Limit to prevent timeout
	if len(errorEntries) > maxEntries {
		prompt += fmt.Sprintf("NOTE: Showing first %d of %d error entries for analysis\n\n", maxEntries, len(errorEntries))
		errorEntries = errorEntries[:maxEntries]
	}

	for _, entry := range errorEntries {
		prompt += fmt.Sprintf("[%s] %s: %s\n",
			entry.Timestamp.Format("15:04:05"),
			entry.Level,
			entry.Message)

		// Add context information from normalized schema
		contextInfo := []string{}
		if entry.Service != "" {
			contextInfo = append(contextInfo, fmt.Sprintf("service=%s", entry.Service))
		}
		if entry.Host != "" {
			contextInfo = append(contextInfo, fmt.Sprintf("host=%s", entry.Host))
		}
		if entry.Environment != "" {
			contextInfo = append(contextInfo, fmt.Sprintf("env=%s", entry.Environment))
		}
		if entry.ErrorCode != "" {
			contextInfo = append(contextInfo, fmt.Sprintf("error_code=%s", entry.ErrorCode))
		}
		if entry.CorrelationId != "" {
			contextInfo = append(contextInfo, fmt.Sprintf("correlation_id=%s", entry.CorrelationId))
		}
		if len(entry.Tags) > 0 {
			contextInfo = append(contextInfo, fmt.Sprintf("tags=%v", entry.Tags))
		}
		if entry.Exception.Type != "" {
			contextInfo = append(contextInfo, fmt.Sprintf("exception_type=%s", entry.Exception.Type))
		}
		if entry.Context.TransactionId != "" {
			contextInfo = append(contextInfo, fmt.Sprintf("transaction_id=%s", entry.Context.TransactionId))
		}
		if entry.Context.UserId != "" {
			contextInfo = append(contextInfo, fmt.Sprintf("user_id=%s", entry.Context.UserId))
		}

		if len(contextInfo) > 0 {
			contextStr := strings.Join(contextInfo, ", ")
			if len(contextStr) > 500 { // Limit context size
				prompt += fmt.Sprintf("  Context: %s... (truncated)\n", contextStr[:500])
			} else {
				prompt += fmt.Sprintf("  Context: %s\n", contextStr)
			}
		}
		// Add metadata if present
		if entry.Metadata != nil && len(entry.Metadata) > 0 {
			metadata, _ := json.Marshal(entry.Metadata)
			if len(metadata) > 500 {
				prompt += fmt.Sprintf("  Metadata: %s... (truncated)\n", string(metadata[:500]))
			} else {
				prompt += fmt.Sprintf("  Metadata: %s\n", string(metadata))
			}
		}
	}

	if similarIncidents != "" {
		prompt += "\nSIMILAR PAST INCIDENTS (for reference):\n" + similarIncidents + "\n"
	}

	prompt += `

Perform a DEEP Root Cause Analysis and provide your findings in the following JSON format:

{
  "summary": "A concise summary focusing on the most critical errors and their impact (2-3 sentences)",
  "severity": "low|medium|high|critical",
  "rootCause": "The primary root cause that explains most of the errors, with step-by-step reasoning",
  "reasoning": "Step-by-step logical reasoning that led you to the root cause, referencing log evidence",
  "recommendations": ["specific_action1", "specific_action2", "specific_action3"],
  "furtherInvestigation": "What additional data or logs would help confirm the root cause?",
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
4. Provide a step-by-step logical reasoning for the root cause, referencing log evidence
5. Suggest what additional data or logs would help confirm the root cause
6. Be as specific and actionable as possible in your recommendations
7. Output valid JSON only
8. If you find similar past incidents, reference them in your reasoning
`

	return prompt
}

func (ls *LLMService) CreateDetailedErrorAnalysisPrompt(request LogAnalysisRequest, errorEntries []models.LogEntry, similarIncidents string) string {
	return ls.createDetailedErrorAnalysisPrompt(request, errorEntries, similarIncidents)
}

func (ls *LLMService) callLLM(prompt string) (string, error) {
	startTime := time.Now()

	request := OllamaGenerateRequest{
		Model:  ls.llmModel,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.2, // Even lower temperature for more consistent analysis
			"top_p":       0.8,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		ls.TrackAPICall(nil, nil, "general", map[string]interface{}{"prompt": prompt, "error": "marshal_failed"}, 0, time.Since(startTime), "", fmt.Sprintf("failed to marshal request: %v", err))
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", ls.baseURL)
	log.Printf("Making LLM request to %s with prompt length: %d characters", url, len(prompt))

	payload := map[string]interface{}{"prompt": prompt, "prompt_length": len(prompt)}

	resp, err := ls.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	elapsed := time.Since(startTime)

	if err != nil {
		log.Printf("LLM request failed after %v: %v", elapsed, err)
		ls.TrackAPICall(nil, nil, "general", payload, 0, elapsed, "", fmt.Sprintf("HTTP request failed: %v", err))
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("LLM request completed in %v with status: %d", elapsed, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		// Read and log the response body for diagnostics
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		log.Printf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
		ls.TrackAPICall(nil, nil, "general", payload, resp.StatusCode, elapsed, "", fmt.Sprintf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes)))
		return "", fmt.Errorf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		log.Printf("Failed to decode Ollama response: %v", err)
		ls.TrackAPICall(nil, nil, "general", payload, resp.StatusCode, elapsed, "", fmt.Sprintf("failed to decode Ollama response: %v", err))
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	ls.TrackAPICall(nil, nil, "general", payload, resp.StatusCode, elapsed, ollamaResp.Response, "")

	return ollamaResp.Response, nil
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
		ls.TrackAPICall(logFileID, jobID, callType, map[string]interface{}{"prompt": prompt, "error": "marshal_failed"}, 0, time.Since(startTime), "", fmt.Sprintf("failed to marshal request: %v", err))
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", ls.baseURL)
	log.Printf("Making LLM request to %s with prompt length: %d characters (context: %s)", url, len(prompt), callType)

	payload := map[string]interface{}{"prompt": prompt, "prompt_length": len(prompt)}

	resp, err := ls.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	elapsed := time.Since(startTime)

	if err != nil {
		log.Printf("LLM request failed after %v: %v", elapsed, err)
		ls.TrackAPICall(logFileID, jobID, callType, payload, 0, elapsed, "", fmt.Sprintf("HTTP request failed: %v", err))
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("LLM request completed in %v with status: %d", elapsed, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		log.Printf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
		ls.TrackAPICall(logFileID, jobID, callType, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes)))
		return "", fmt.Errorf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		log.Printf("Failed to decode Ollama response: %v", err)
		ls.TrackAPICall(logFileID, jobID, callType, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("failed to decode Ollama response: %v", err))
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	ls.TrackAPICall(logFileID, jobID, callType, payload, resp.StatusCode, elapsed, ollamaResp.Response, "")

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
		ls.TrackAPICall(logFileID, jobID, callType, map[string]interface{}{"prompt": prompt, "error": "marshal_failed"}, 0, time.Since(startTime), "", fmt.Sprintf("failed to marshal request: %v", err))
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", ls.baseURL)
	log.Printf("Making LLM request to %s with prompt length: %d characters (context: %s, timeout: %ds)", url, len(prompt), callType, timeout)

	payload := map[string]interface{}{"prompt": prompt, "prompt_length": len(prompt), "timeout": timeout}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		ls.TrackAPICall(logFileID, jobID, callType, payload, 0, time.Since(startTime), "", fmt.Sprintf("failed to create request: %v", err))
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ls.client.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		log.Printf("LLM request failed after %v: %v", elapsed, err)
		ls.TrackAPICall(logFileID, jobID, callType, payload, 0, elapsed, "", fmt.Sprintf("HTTP request failed: %v", err))
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("LLM request completed in %v with status: %d", elapsed, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		log.Printf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
		ls.TrackAPICall(logFileID, jobID, callType, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes)))
		return "", fmt.Errorf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		log.Printf("Failed to decode Ollama response: %v", err)
		ls.TrackAPICall(logFileID, jobID, callType, payload, resp.StatusCode, elapsed, "", fmt.Sprintf("failed to decode Ollama response: %v", err))
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	ls.TrackAPICall(logFileID, jobID, callType, payload, resp.StatusCode, elapsed, ollamaResp.Response, "")

	return ollamaResp.Response, nil
}

func (ls *LLMService) callLLMWithTimeout(prompt string, timeout int) (string, error) {
	callID := fmt.Sprintf("llm_timeout_%d", time.Now().UnixNano())
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
		ls.addAPICall(LLMAPICall{
			ID:        callID,
			Timestamp: startTime,
			Endpoint:  "/api/generate",
			Model:     ls.llmModel,
			Payload:   map[string]interface{}{"prompt": prompt, "timeout": timeout, "error": "marshal_failed"},
			Status:    0,
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("failed to marshal request: %v", err),
		})
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	url := fmt.Sprintf("%s/api/generate", ls.baseURL)
	log.Printf("Making LLM request to %s with prompt length: %d characters (timeout: %ds)", url, len(prompt), timeout)
	client := ls.client
	if timeout > 0 {
		client = &http.Client{Timeout: time.Duration(timeout) * time.Second}
	}

	apiCall := LLMAPICall{
		ID:        callID,
		Timestamp: startTime,
		Endpoint:  "/api/generate",
		Model:     ls.llmModel,
		Payload:   map[string]interface{}{"prompt": prompt, "prompt_length": len(prompt), "timeout": timeout},
		Duration:  time.Since(startTime),
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	elapsed := time.Since(startTime)

	if err != nil {
		log.Printf("LLM request failed after %v: %v", elapsed, err)
		apiCall.Status = 0
		apiCall.Error = fmt.Sprintf("HTTP request failed: %v", err)
		ls.addAPICall(apiCall)
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("LLM request completed in %v with status: %d", elapsed, resp.StatusCode)
	apiCall.Status = resp.StatusCode

	if resp.StatusCode != http.StatusOK {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(resp.Body)
		log.Printf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
		apiCall.Error = fmt.Sprintf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
		ls.addAPICall(apiCall)
		return "", fmt.Errorf("Ollama API returned status %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}
	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		log.Printf("Failed to decode Ollama response: %v", err)
		apiCall.Error = fmt.Sprintf("failed to decode Ollama response: %v", err)
		ls.addAPICall(apiCall)
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}
	apiCall.Response = ollamaResp.Response
	ls.addAPICall(apiCall)

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

	// Defensive: Check if the response looks like JSON
	if !strings.HasPrefix(cleanResponse, "{") && !strings.HasPrefix(cleanResponse, "[") {
		log.Printf("LLM returned non-JSON response: %q", cleanResponse)
		return nil, fmt.Errorf("LLM did not return valid JSON. Raw response: %q", cleanResponse)
	}

	var analysis LogAnalysisResponse
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		log.Printf("Failed to parse JSON from LLM: %q", cleanResponse)
		return nil, fmt.Errorf("failed to parse JSON response: %w. Raw response: %q", err, cleanResponse)
	}

	// Validate and normalize the response
	if analysis.Summary == "" {
		analysis.Summary = "Error analysis completed but no summary generated."
	}

	if analysis.RootCause == "" {
		analysis.RootCause = "Unable to determine root cause"
	}

	// Normalize severity
	analysis.Severity = ls.normalizeSeverity(analysis.Severity)

	// Validate error analysis
	if analysis.ErrorAnalysis == nil {
		analysis.ErrorAnalysis = []DetailedErrorAnalysis{}
	}

	// Normalize error analysis severity
	for i := range analysis.ErrorAnalysis {
		analysis.ErrorAnalysis[i].Severity = ls.normalizeErrorSeverity(analysis.ErrorAnalysis[i].Severity)
	}

	// Ensure recommendations is not nil
	if analysis.Recommendations == nil {
		analysis.Recommendations = []string{}
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

func (ls *LLMService) generateNoErrorsAnalysis(logFile *models.LogFile) *LogAnalysisResponse {
	return &LogAnalysisResponse{
		Summary:         fmt.Sprintf("Log file '%s' contains no ERROR or FATAL entries. System appears to be functioning normally.", logFile.Filename),
		Severity:        "low",
		RootCause:       "No errors detected",
		Recommendations: []string{"Continue monitoring for any new errors", "Review INFO and WARNING logs for potential issues"},

		ErrorAnalysis:     []DetailedErrorAnalysis{},
		CriticalErrors:    0,
		NonCriticalErrors: 0,
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
	prompt := fmt.Sprintf(`You are a log parsing expert. Analyze these log lines and return ONLY a logpai/logparser format string.

IMPORTANT: Return ONLY the format string, no explanations, no markdown, no extra text.

Example format strings:
- <Date> <Time> <Level>: <Content>
- <Level> <Time> <Content>
- <Date> <Time> <Level> <Content>

Log lines to analyze:
%s

Format string:`, strings.Join(samples, "\n"))

	resp, err := ls.callLLMWithContext(prompt, logFileID, nil, "format_inference")
	if err != nil {
		return "", err
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
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(resp, prefix) {
			resp = strings.TrimSpace(strings.TrimPrefix(resp, prefix))
			break
		}
	}

	// Take only the first line if multiple lines
	if idx := strings.Index(resp, "\n"); idx != -1 {
		resp = strings.TrimSpace(resp[:idx])
	}

	// Validate that it looks like a format string (contains angle brackets)
	if !strings.Contains(resp, "<") || !strings.Contains(resp, ">") {
		return "", fmt.Errorf("LLM did not return a valid format string: %q", resp)
	}

	return resp, nil
}
