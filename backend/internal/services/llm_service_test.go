package services

import (
	"testing"
	"time"

	"github.com/autolog/backend/internal/models"
)

func TestFilterErrorEntries(t *testing.T) {
	// Create test entries
	entries := []models.LogEntry{
		{
			Level:   models.LogLevelInfo,
			Message: "System started",
		},
		{
			Level:   models.LogLevelError,
			Message: "Database connection failed",
		},
		{
			Level:   models.LogLevelDebug,
			Message: "Debug info",
		},
		{
			Level:   models.LogLevelFatal,
			Message: "Critical system failure",
		},
		{
			Level:   models.LogLevelWarning,
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
		if entry.Level != models.LogLevelError && entry.Level != models.LogLevelFatal {
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
	analysis := llmService.generateNoErrorsAnalysis(logFile)

	if analysis.Severity != "low" {
		t.Errorf("Expected severity 'low', got '%s'", analysis.Severity)
	}

	if analysis.IncidentType != "No Incident" {
		t.Errorf("Expected incident type 'No Incident', got '%s'", analysis.IncidentType)
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

func TestGenerateFallbackErrorAnalysis(t *testing.T) {
	errorEntries := []models.LogEntry{
		{
			Timestamp: time.Now().Add(-time.Hour),
			Level:     models.LogLevelError,
			Message:   "Database connection failed",
		},
		{
			Timestamp: time.Now(),
			Level:     models.LogLevelError,
			Message:   "Database connection failed",
		},
		{
			Timestamp: time.Now(),
			Level:     models.LogLevelFatal,
			Message:   "System crash",
		},
	}

	request := LogAnalysisRequest{
		LogEntries: errorEntries,
		Filename:   "test.log",
	}

	llmService := &LLMService{}
	analysis := llmService.generateFallbackErrorAnalysis(request, errorEntries)

	if len(analysis.ErrorAnalysis) == 0 {
		t.Error("Expected error analysis to be generated")
	}

	// Should have grouped similar errors
	if len(analysis.ErrorAnalysis) < 1 {
		t.Errorf("Expected at least 1 error pattern, got %d", len(analysis.ErrorAnalysis))
	}

	// Check that critical errors are counted
	if analysis.CriticalErrors == 0 && analysis.NonCriticalErrors == 0 {
		t.Error("Expected error counts to be calculated")
	}
}
