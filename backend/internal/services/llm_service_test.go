package services

import (
	"encoding/json"
	"testing"

	"github.com/autolog/backend/internal/models"
)

func TestFilterErrorEntries(t *testing.T) {
	// Create test entries
	entries := []models.LogEntry{
		{
			Level:   string(models.LogLevelInfo),
			Message: "System started",
		},
		{
			Level:   string(models.LogLevelError),
			Message: "Database connection failed",
		},
		{
			Level:   string(models.LogLevelDebug),
			Message: "Debug info",
		},
		{
			Level:   string(models.LogLevelFatal),
			Message: "Critical system failure",
		},
		{
			Level:   string(models.LogLevelWarning),
			Message: "Warning message",
		},
	}

	llmService := &LLMService{}
	errorEntries := llmService.filterErrorEntries(entries)

	// Should only have ERROR and FATAL entries
	if len(errorEntries) != 2 {
		t.Errorf("Expected 2 error entries, got %d", len(errorEntries))
	}

	// Check that only ERROR and FATAL entries are included
	for _, entry := range errorEntries {
		if entry.Level != string(models.LogLevelError) && entry.Level != string(models.LogLevelFatal) {
			t.Errorf("Expected ERROR or FATAL level, got %s", entry.Level)
		}
	}
}

func TestExtractErrorPattern(t *testing.T) {
	llmService := &LLMService{}

	tests := []struct {
		message string
		pattern string
	}{
		{
			message: "Database connection timeout",
			pattern: "Connection Timeout",
		},
		{
			message: "Authentication failed",
			pattern: "Authentication Error",
		},
		{
			message: "DB query failed",
			pattern: "Database Error",
		},
		{
			message: "Permission denied",
			pattern: "Permission/Access Error",
		},
		{
			message: "Resource not found",
			pattern: "Resource Not Found",
		},
		{
			message: "Request timeout",
			pattern: "Timeout Error",
		},
		{
			message: "Out of memory",
			pattern: "Memory Error",
		},
		{
			message: "Unknown error occurred",
			pattern: "General Error",
		},
	}

	for _, test := range tests {
		pattern := llmService.extractErrorPattern(test.message)
		if pattern != test.pattern {
			t.Errorf("For message '%s', expected pattern '%s', got '%s'", test.message, test.pattern, pattern)
		}
	}
}

func TestNormalizeErrorSeverity(t *testing.T) {
	llmService := &LLMService{}

	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "critical"},
		{"fatal", "critical"},
		{"severe", "critical"},
		{"non-critical", "non-critical"},
		{"noncritical", "non-critical"},
		{"minor", "non-critical"},
		{"low", "non-critical"},
		{"unknown", "non-critical"},
		{"", "non-critical"},
	}

	for _, test := range tests {
		result := llmService.normalizeErrorSeverity(test.input)
		if result != test.expected {
			t.Errorf("For input '%s', expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

func TestGenerateNoErrorsAnalysis(t *testing.T) {
	logFile := models.LogFile{
		Filename: "test.log",
	}

	llmService := &LLMService{}
	analysis := llmService.generateNoErrorsAnalysis(&logFile)

	if analysis.Severity != "low" {
		t.Errorf("Expected severity 'low', got '%s'", analysis.Severity)
	}

	if len(analysis.ErrorAnalysis) != 0 {
		t.Errorf("Expected no error analysis, got %d", len(analysis.ErrorAnalysis))
	}

	if analysis.CriticalErrors != 0 {
		t.Errorf("Expected 0 critical errors, got %d", analysis.CriticalErrors)
	}

	if analysis.NonCriticalErrors != 0 {
		t.Errorf("Expected 0 non-critical errors, got %d", analysis.NonCriticalErrors)
	}
}

func TestAttemptToFixJSON(t *testing.T) {
	llmService := &LLMService{}
	malformed := `{
"summary": "The system failed to connect to an external analytics service due to a temporary endpoint unavailability, with no data loss or security issues.",
"severity": "low",
"rootCause": "The primary root cause is the temporary unavailability of the external analytics service's endpoint, as indicated by the 503 error code and retry attempts. This is likely due to maintenance or infrastructure issues on the service provider's end.",
"reasoning": "Based on the log evidence, the system attempted to connect to the external analytics service at the specified endpoint but received a 503 error code, indicating that the endpoint is temporarily unavailable. The retry attempts suggest that the system is attempting to connect to the service despite the unavailability, which further supports the conclusion that the issue lies with the service provider's end. Additionally, there is no evidence of data loss or security issues, which suggests that the impact is minimal.",
"recommendations": ["Monitor the external analytics service's status and availability to ensure a smooth connection when it becomes available again."],
"furtherInvestigation": "To confirm the root cause, additional data such as the service provider's maintenance schedule or infrastructure issues would be helpful. Additionally, reviewing past incidents of similar nature could provide valuable insights into the service provider's reliability and potential mitigation strategies."
"errorAnalysis": [
{
"errorPattern": "External analytics service endpoint unavailable",
"errorCount": 1,
"firstOccurrence": "2025-07-04 15:30:30",
"lastOccurrence": "2025-07-04 15:30:30",
"severity": "low",
"rootCause": "Temporary unavailability of the external analytics service's endpoint",
"impact": "Minimal, as there is no data loss or security issues.",
"fix": "Monitor the service provider's status and availability to ensure a smooth connection when it becomes available again."
}
],
"criticalErrors": 0,
"nonCriticalErrors": 1
}`

	fixed := llmService.attemptToFixJSON(malformed)
	var result map[string]interface{}
	err := json.Unmarshal([]byte(fixed), &result)
	if err != nil {
		t.Errorf("attemptToFixJSON did not produce valid JSON. Error: %v\nFixed: %s", err, fixed)
	}
	if _, ok := result["summary"]; !ok {
		t.Error("Fixed JSON does not contain expected 'summary' field")
	}
}
