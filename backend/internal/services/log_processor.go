package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/autolog/backend/internal/models"

	"gorm.io/gorm"
)

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type LogProcessor struct {
	db           *gorm.DB
	llmService   *LLMService
	logparserURL string // Add logparser microservice URL
}

func NewLogProcessor(db *gorm.DB, llmService *LLMService) *LogProcessor {
	logparserURL := os.Getenv("LOGPARSER_URL")
	if logparserURL == "" {
		logparserURL = "http://localhost:8000"
	}
	return &LogProcessor{
		db:           db,
		llmService:   llmService,
		logparserURL: logparserURL,
	}
}

// Helper to call logparser microservice with a file and return structured logs
func (lp *LogProcessor) callLogparserMicroservice(filePath string, logFormat string) ([]models.LogEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for logparser: %w", err)
	}
	defer file.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(fw, file); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}
	// Add log_format if provided
	if logFormat != "" {
		w.WriteField("log_format", logFormat)
	}
	w.Close()

	req, err := http.NewRequest("POST", lp.logparserURL+"/parse", &b)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("logparser microservice request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("logparser microservice returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode logparser response: %w", err)
	}

	var entries []models.LogEntry
	for _, item := range parsed {
		entry := NormalizeToLogEntry(0, item) // Pass 0 for logFileID as it's not a file-specific entry
		entries = append(entries, entry)
	}
	return entries, nil
}

// Update ProcessLogFile to use logparser microservice for non-JSON logs
func (lp *LogProcessor) ProcessLogFile(logFileID uint, filePath string) error {
	log.Printf("[LOG PROCESSOR] Starting to process log file %d: %s", logFileID, filePath)

	// Database health check
	var count int64
	if err := lp.db.Model(&models.LogEntry{}).Count(&count).Error; err != nil {
		log.Printf("[LOG PROCESSOR] Database health check failed: %v", err)
		return fmt.Errorf("database health check failed: %w", err)
	}
	log.Printf("[LOG PROCESSOR] Database health check passed, current log_entries count: %d", count)

	// Test database write capability
	testEntry := models.LogEntry{
		LogFileID: logFileID,
		Timestamp: time.Now(),
		Level:     "TEST",
		Message:   "Test entry for database write verification",
	}
	if err := lp.db.Create(&testEntry).Error; err != nil {
		log.Printf("[LOG PROCESSOR] Database write test failed: %v", err)
		return fmt.Errorf("database write test failed: %w", err)
	}
	log.Printf("[LOG PROCESSOR] Database write test passed, created test entry with ID: %d", testEntry.ID)

	// Clean up test entry
	if err := lp.db.Delete(&testEntry).Error; err != nil {
		log.Printf("[LOG PROCESSOR] Warning: Failed to clean up test entry: %v", err)
	}

	// Update status to processing
	if err := lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Update("status", "processing").Error; err != nil {
		return fmt.Errorf("failed to update log file status: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		lp.updateLogFileStatus(logFileID, "failed")
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Test file reading - read first few lines to verify content
	log.Printf("[LOG PROCESSOR] Testing file reading for %s", filePath)
	file.Seek(0, 0)
	testScanner := bufio.NewScanner(file)
	lineCount := 0
	for testScanner.Scan() && lineCount < 3 {
		line := strings.TrimSpace(testScanner.Text())
		if line != "" {
			log.Printf("[LOG PROCESSOR] Test line %d: %s", lineCount+1, line[:min(len(line), 100)])
		}
		lineCount++
	}
	if err := testScanner.Err(); err != nil {
		log.Printf("[LOG PROCESSOR] Error during test reading: %v", err)
	}
	log.Printf("[LOG PROCESSOR] File test reading completed, found %d non-empty lines in first 3", lineCount)

	var entries []models.LogEntry
	var errorCount, warningCount int

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	isAllJSON := true
	var nonJSONLines []string
	const sampleLimit = 10

	log.Printf("[LOG PROCESSOR] Scanning file to determine format...")
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var parsedMap map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsedMap); err != nil {
			isAllJSON = false
			if len(nonJSONLines) < sampleLimit {
				nonJSONLines = append(nonJSONLines, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		lp.updateLogFileStatus(logFileID, "failed")
		return fmt.Errorf("error reading log file: %w", err)
	}

	log.Printf("[LOG PROCESSOR] File format determined: isAllJSON=%v, totalLines=%d", isAllJSON, lineNumber)

	if isAllJSON {
		log.Printf("[LOG PROCESSOR] Processing as JSON logs...")
		// Rewind file
		file.Seek(0, 0)
		scanner = bufio.NewScanner(file)
		lineNumber = 0
		for scanner.Scan() {
			lineNumber++
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var parsedMap map[string]interface{}
			if err := json.Unmarshal([]byte(line), &parsedMap); err != nil {
				log.Printf("Failed to parse line %d: %v", lineNumber, err)
				continue
			}
			log.Printf("[LOG PROCESSOR] Parsed line %d: %+v", lineNumber, parsedMap)
			entry := NormalizeToLogEntry(logFileID, parsedMap)
			log.Printf("[LOG PROCESSOR] LogEntry for line %d: %+v", lineNumber, entry)
			entry.LogFileID = logFileID
			entries = append(entries, entry)
			switch entry.Level {
			case "ERROR", "FATAL":
				errorCount++
			case "WARN":
				warningCount++
			}
		}
		log.Printf("[LOG PROCESSOR] JSON processing complete: %d entries parsed", len(entries))
	} else {
		log.Printf("[LOG PROCESSOR] Processing as unstructured logs...")
		// Infer log format from non-JSON samples using LLM
		logFormat := ""
		if len(nonJSONLines) > 0 {
			var err error
			logFormat, err = lp.llmService.InferLogFormatFromSamples(nonJSONLines, &logFileID)
			if err != nil {
				log.Printf("[LLM] Failed to infer log format: %v, falling back to default", err)
				logFormat = ""
			} else {
				log.Printf("[LLM] Inferred log format: %s", logFormat)
			}
		}
		parsedEntries, err := lp.callLogparserMicroservice(filePath, logFormat)
		if err != nil {
			lp.updateLogFileStatus(logFileID, "failed")
			return fmt.Errorf("logparser microservice failed: %w", err)
		}
		for i := range parsedEntries {
			parsedEntries[i].LogFileID = logFileID
			// Count errors and warnings
			switch parsedEntries[i].Level {
			case "ERROR", "FATAL":
				errorCount++
			case "WARN":
				warningCount++
			}
		}
		entries = append(entries, parsedEntries...)
		log.Printf("[LOG PROCESSOR] Unstructured processing complete: %d entries parsed", len(entries))
	}

	// Save entries in batches
	if len(entries) > 0 {
		log.Printf("[LOG PROCESSOR] Saving %d entries to database...", len(entries))

		// Validate entries before saving
		for i, entry := range entries {
			if entry.LogFileID == 0 {
				log.Printf("[LOG PROCESSOR] Error: Entry %d has zero LogFileID", i)
			}
			if entry.Timestamp.IsZero() {
				log.Printf("[LOG PROCESSOR] Warning: Entry %d has zero timestamp", i)
			}
			if entry.Message == "" {
				log.Printf("[LOG PROCESSOR] Warning: Entry %d has empty message", i)
			}
		}

		if err := lp.db.CreateInBatches(entries, 100).Error; err != nil {
			lp.updateLogFileStatus(logFileID, "failed")
			log.Printf("[LOG PROCESSOR] Database save error: %v", err)
			return fmt.Errorf("failed to save log entries: %w", err)
		}

		// Verify entries were saved
		var savedCount int64
		if err := lp.db.Model(&models.LogEntry{}).Where("log_file_id = ?", logFileID).Count(&savedCount).Error; err != nil {
			log.Printf("[LOG PROCESSOR] Warning: Could not verify saved entries: %v", err)
		} else {
			log.Printf("[LOG PROCESSOR] Verified %d entries saved to database", savedCount)
		}

		log.Printf("[LOG PROCESSOR] Successfully saved %d entries to database", len(entries))
	} else {
		log.Printf("[LOG PROCESSOR] Warning: No entries to save")
	}

	// Update log file with final stats
	now := time.Now()
	updateData := map[string]interface{}{
		"status":        "completed",
		"entry_count":   len(entries),
		"error_count":   errorCount,
		"warning_count": warningCount,
		"processed_at":  &now,
	}

	log.Printf("[LOG PROCESSOR] Updating log file stats: entries=%d, errors=%d, warnings=%d", len(entries), errorCount, warningCount)

	if err := lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Updates(updateData).Error; err != nil {
		return fmt.Errorf("failed to update log file stats: %w", err)
	}

	// Delete the uploaded file after successful parsing and DB write
	if err := os.Remove(filePath); err != nil {
		log.Printf("Warning: failed to delete uploaded file %s: %v", filePath, err)
	}

	log.Printf("[LOG PROCESSOR] Log file processing completed successfully")
	return nil
}

func (lp *LogProcessor) parseTimestamp(timestampStr string) (time.Time, error) {
	// Try common timestamp formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000",
		time.UnixDate,
		time.RFC822,
		time.RFC850,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timestampStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestampStr)
}

func (lp *LogProcessor) normalizeLogLevel(level string) string {
	level = strings.ToUpper(strings.TrimSpace(level))

	switch level {
	case "DEBUG", "DBG":
		return "DEBUG"
	case "INFO", "INF":
		return "INFO"
	case "WARN", "WARNING":
		return "WARN"
	case "ERROR", "ERR":
		return "ERROR"
	case "FATAL", "CRITICAL", "CRIT":
		return "FATAL"
	default:
		return "INFO"
	}
}

func (lp *LogProcessor) updateLogFileStatus(logFileID uint, status string) {
	lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Update("status", status)
}

// AnalyzeLogFile performs AI-powered log analysis and RCA generation on a processed log file
func (lp *LogProcessor) AnalyzeLogFile(logFileID uint) (*models.LogAnalysis, error) {
	var logFile models.LogFile
	if err := lp.db.Preload("Entries").First(&logFile, logFileID).Error; err != nil {
		return nil, fmt.Errorf("log file not found: %w", err)
	}

	if logFile.Status != "completed" {
		return nil, fmt.Errorf("log file not yet processed")
	}

	analysis := &models.LogAnalysis{
		LogFileID:    logFileID,
		ErrorCount:   logFile.ErrorCount,
		WarningCount: logFile.WarningCount,
	}

	// Find time range
	if len(logFile.Entries) > 0 {
		analysis.StartTime = logFile.Entries[0].Timestamp
		analysis.EndTime = logFile.Entries[len(logFile.Entries)-1].Timestamp
	}

	// Use AI-powered analysis if LLM service is available
	if lp.llmService != nil {
		// Perform AI analysis
		aiAnalysis, err := lp.llmService.AnalyzeLogsWithAI(&logFile, logFile.Entries, nil) // No job ID for direct processing
		if err != nil {
			log.Printf("AI analysis failed for log file %d: %v", logFileID, err)
			// Continue processing even if AI analysis fails
		}
		if err == nil {
			// Use AI-generated analysis
			analysis.Severity = aiAnalysis.Severity
			analysis.Summary = aiAnalysis.Summary

			// Add detailed error analysis to metadata
			analysis.Metadata = map[string]interface{}{
				"rootCause":       aiAnalysis.RootCause,
				"recommendations": aiAnalysis.Recommendations,

				"errorAnalysis":     aiAnalysis.ErrorAnalysis,
				"criticalErrors":    aiAnalysis.CriticalErrors,
				"nonCriticalErrors": aiAnalysis.NonCriticalErrors,
				"aiGenerated":       true,
			}
		} else {
			// Fallback to basic analysis when LLM service is not available
			analysis.Severity = lp.determineSeverity(logFile.Entries)
			analysis.Summary = lp.generateSummary(logFile)
		}
	} else {
		// Fallback to basic analysis when LLM service is not available
		analysis.Severity = lp.determineSeverity(logFile.Entries)
		analysis.Summary = lp.generateSummary(logFile)
	}

	// Save analysis
	if err := lp.db.Create(analysis).Error; err != nil {
		return nil, fmt.Errorf("failed to save analysis: %w", err)
	}

	return analysis, nil
}

func (lp *LogProcessor) determineSeverity(entries []models.LogEntry) string {
	errorCount := 0
	fatalCount := 0

	for _, entry := range entries {
		switch entry.Level {
		case "ERROR":
			errorCount++
		case "FATAL":
			fatalCount++
		}
	}

	if fatalCount > 0 {
		return "critical"
	} else if errorCount > 10 {
		return "high"
	} else if errorCount > 5 {
		return "medium"
	} else if errorCount > 0 {
		return "low"
	}
	return "low"
}

func (lp *LogProcessor) generateSummary(logFile models.LogFile) string {
	summary := fmt.Sprintf("Log file '%s' contains %d entries", logFile.Filename, logFile.EntryCount)

	if logFile.ErrorCount > 0 {
		summary += fmt.Sprintf(" with %d errors", logFile.ErrorCount)
	}

	if logFile.WarningCount > 0 {
		summary += fmt.Sprintf(" and %d warnings", logFile.WarningCount)
	}

	// Add time range if available
	if len(logFile.Entries) > 0 {
		start := logFile.Entries[0].Timestamp
		end := logFile.Entries[len(logFile.Entries)-1].Timestamp
		duration := end.Sub(start)
		summary += fmt.Sprintf(". Time range: %s to %s (duration: %s)",
			start.Format("2006-01-02 15:04:05"),
			end.Format("2006-01-02 15:04:05"),
			duration.String())
	}

	return summary
}

// NormalizeToLogEntry maps a parsed log (map[string]interface{}) to the canonical LogEntry struct
func NormalizeToLogEntry(logFileID uint, parsed map[string]interface{}) models.LogEntry {
	getString := func(key string) string {
		if v, ok := parsed[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			// Try to convert other types to string
			if f, ok := v.(float64); ok {
				return fmt.Sprintf("%.0f", f)
			}
			if i, ok := v.(int); ok {
				return fmt.Sprintf("%d", i)
			}
			if b, ok := v.(bool); ok {
				return fmt.Sprintf("%t", b)
			}
		}
		return ""
	}
	getStringSlice := func(key string) []string {
		if v, ok := parsed[key]; ok {
			if arr, ok := v.([]interface{}); ok {
				var out []string
				for _, item := range arr {
					if s, ok := item.(string); ok {
						out = append(out, s)
					}
				}
				return out
			}
			if arr, ok := v.([]string); ok {
				return arr
			}
		}
		return nil
	}
	getTime := func(key string) time.Time {
		if v, ok := parsed[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				// Try common timestamp formats
				formats := []string{
					time.RFC3339Nano,
					time.RFC3339,
					"2006-01-02T15:04:05Z07:00",
					"2006-01-02 15:04:05",
					"2006-01-02T15:04:05Z",
					"2006-01-02T15:04:05.000Z",
					"2006-01-02T15:04:05.000000Z",
					"2006-01-02 15:04:05.000",
					"2006-01-02 15:04:05.000000",
				}
				for _, format := range formats {
					t, err := time.Parse(format, s)
					if err == nil {
						return t
					}
				}
				log.Printf("[LOG PROCESSOR] Failed to parse timestamp: %q", s)
			}
		}
		return time.Time{}
	}
	// Exception
	exception := models.Exception{}
	if exc, ok := parsed["exception"].(map[string]interface{}); ok {
		exception.Type = getStringFromMap(exc, "type")
		if st, ok := exc["stack_trace"]; ok {
			if arr, ok := st.([]interface{}); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok {
						exception.StackTrace = append(exception.StackTrace, s)
					}
				}
			}
		}
	}
	// Context
	context := models.Context{}
	if ctx, ok := parsed["context"].(map[string]interface{}); ok {
		context.TransactionId = getStringFromMap(ctx, "transaction_id")
		context.UserId = getStringFromMap(ctx, "user_id")
		if req, ok := ctx["request"].(map[string]interface{}); ok {
			context.Request.Method = getStringFromMap(req, "method")
			context.Request.Url = getStringFromMap(req, "url")
			context.Request.Ip = getStringFromMap(req, "ip")
		}
		if cf, ok := ctx["custom_fields"].(map[string]interface{}); ok {
			if v, ok := cf["retry_attempt"].(float64); ok {
				context.CustomFields.RetryAttempt = int(v)
			}
			context.CustomFields.DatabaseName = getStringFromMap(cf, "database_name")
		}
	}

	// Extract metadata field if present
	var logMetadata map[string]interface{}
	if meta, ok := parsed["metadata"].(map[string]interface{}); ok {
		logMetadata = meta
	}

	entry := models.LogEntry{
		LogFileID:     logFileID,
		Timestamp:     getTime("timestamp"),
		Service:       getString("service"),
		Host:          getString("host"),
		Environment:   getString("environment"),
		Level:         getString("level"),
		ErrorCode:     getString("error_code"),
		Message:       getString("message"),
		Exception:     exception,
		Context:       context,
		Tags:          getStringSlice("tags"),
		CorrelationId: getString("correlation_id"),
		Metadata:      logMetadata, // Use the metadata field from the JSON
	}

	// Debug: Log if any critical fields are empty
	if entry.Timestamp.IsZero() {
		log.Printf("[LOG PROCESSOR] Warning: Timestamp is zero for entry")
	}
	if entry.Level == "" {
		log.Printf("[LOG PROCESSOR] Warning: Level is empty for entry")
	}
	if entry.Message == "" {
		log.Printf("[LOG PROCESSOR] Warning: Message is empty for entry")
	}

	return entry
}

// Helper to get string from map[string]interface{}
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Add helper to extract unmapped fields as metadata
func extractMetadata(parsed map[string]interface{}) map[string]interface{} {
	canonical := map[string]struct{}{
		"timestamp": {}, "service": {}, "host": {}, "environment": {}, "level": {}, "error_code": {}, "message": {}, "exception": {}, "context": {}, "tags": {}, "correlation_id": {}, "metadata": {},
	}
	metadata := make(map[string]interface{})
	for k, v := range parsed {
		if _, ok := canonical[k]; !ok {
			metadata[k] = v
		}
	}
	return metadata
}
