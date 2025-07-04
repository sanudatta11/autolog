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

type LogProcessor struct {
	db           *gorm.DB
	llmService   *LLMService
	logparserURL string // Add logparser microservice URL
}

type JSONLogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	// Additional fields that might be present
	Time     string                 `json:"time"`
	LogLevel string                 `json:"log_level"`
	Msg      string                 `json:"msg"`
	Data     map[string]interface{} `json:"data,omitempty"`
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
		var entry models.LogEntry
		// Prefer canonical fields if present
		if ts, ok := item["timestamp"]; ok {
			if tstr, ok := ts.(string); ok {
				if t, err := lp.parseTimestamp(tstr); err == nil {
					entry.Timestamp = t
				}
			}
		}
		if lvl, ok := item["level"]; ok {
			if lstr, ok := lvl.(string); ok {
				entry.Level = lp.normalizeLogLevel(lstr)
			}
		}
		if msg, ok := item["message"]; ok {
			if mstr, ok := msg.(string); ok {
				entry.Message = mstr
			}
		}
		if raw, ok := item["rawData"]; ok {
			if rstr, ok := raw.(string); ok {
				entry.RawData = rstr
			}
		}
		if meta, ok := item["metadata"]; ok {
			if m, ok := meta.(map[string]interface{}); ok {
				entry.Metadata = m
			}
		}
		// Fallback to legacy fields if canonical not present
		if entry.Timestamp.IsZero() {
			if ts, ok := item["Date"]; ok {
				if tstr, ok := ts.(string); ok {
					if t, err := lp.parseTimestamp(tstr); err == nil {
						entry.Timestamp = t
					}
				}
			}
		}
		if entry.Level == "" {
			if lvl, ok := item["Level"]; ok {
				if lstr, ok := lvl.(string); ok {
					entry.Level = lp.normalizeLogLevel(lstr)
				}
			}
		}
		if entry.Message == "" {
			if msg, ok := item["Content"]; ok {
				entry.Message = fmt.Sprintf("%v", msg)
			}
		}
		if entry.RawData == "" {
			entry.RawData = ""
		}
		if entry.Metadata == nil {
			entry.Metadata = item
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Update ProcessLogFile to use logparser microservice for non-JSON logs
func (lp *LogProcessor) ProcessLogFile(logFileID uint, filePath string) error {
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

	var entries []models.LogEntry
	var errorCount, warningCount int

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	isAllJSON := true
	var nonJSONLines []string
	const sampleLimit = 10

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		_, err := lp.parseLogLine(line)
		if err != nil {
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

	if isAllJSON {
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
			entry, err := lp.parseLogLine(line)
			if err != nil {
				log.Printf("Failed to parse line %d: %v", lineNumber, err)
				continue
			}
			entry.LogFileID = logFileID
			entries = append(entries, entry)
			switch entry.Level {
			case models.LogLevelError, models.LogLevelFatal:
				errorCount++
			case models.LogLevelWarning:
				warningCount++
			}
		}
	} else {
		// Infer log format from non-JSON samples using LLM
		logFormat := ""
		if len(nonJSONLines) > 0 {
			var err error
			logFormat, err = lp.llmService.InferLogFormatFromSamples(nonJSONLines)
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
			case models.LogLevelError, models.LogLevelFatal:
				errorCount++
			case models.LogLevelWarning:
				warningCount++
			}
		}
		entries = append(entries, parsedEntries...)
	}

	// Save entries in batches
	if len(entries) > 0 {
		if err := lp.db.CreateInBatches(entries, 100).Error; err != nil {
			lp.updateLogFileStatus(logFileID, "failed")
			return fmt.Errorf("failed to save log entries: %w", err)
		}
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

	if err := lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Updates(updateData).Error; err != nil {
		return fmt.Errorf("failed to update log file stats: %w", err)
	}

	// Delete the uploaded file after successful parsing and DB write
	if err := os.Remove(filePath); err != nil {
		log.Printf("Warning: failed to delete uploaded file %s: %v", filePath, err)
	}

	return nil
}

func (lp *LogProcessor) parseLogLine(line string) (models.LogEntry, error) {
	var jsonEntry JSONLogEntry
	if err := json.Unmarshal([]byte(line), &jsonEntry); err != nil {
		return models.LogEntry{}, fmt.Errorf("invalid JSON: %w", err)
	}

	entry := models.LogEntry{
		RawData: line,
	}

	// Parse timestamp
	if jsonEntry.Timestamp != "" {
		if timestamp, err := lp.parseTimestamp(jsonEntry.Timestamp); err == nil {
			entry.Timestamp = timestamp
		}
	} else if jsonEntry.Time != "" {
		if timestamp, err := lp.parseTimestamp(jsonEntry.Time); err == nil {
			entry.Timestamp = timestamp
		}
	}

	// Parse level
	if jsonEntry.Level != "" {
		entry.Level = lp.normalizeLogLevel(jsonEntry.Level)
	} else if jsonEntry.LogLevel != "" {
		entry.Level = lp.normalizeLogLevel(jsonEntry.LogLevel)
	}

	// Parse message
	if jsonEntry.Message != "" {
		entry.Message = jsonEntry.Message
	} else if jsonEntry.Msg != "" {
		entry.Message = jsonEntry.Msg
	}

	// Parse metadata
	if jsonEntry.Metadata != nil {
		entry.Metadata = jsonEntry.Metadata
	} else if jsonEntry.Data != nil {
		entry.Metadata = jsonEntry.Data
	}

	// Set default timestamp if not found
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Set default level if not found
	if entry.Level == "" {
		entry.Level = models.LogLevelInfo
	}

	return entry, nil
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

func (lp *LogProcessor) normalizeLogLevel(level string) models.LogLevel {
	level = strings.ToUpper(strings.TrimSpace(level))

	switch level {
	case "DEBUG", "DBG":
		return models.LogLevelDebug
	case "INFO", "INF":
		return models.LogLevelInfo
	case "WARN", "WARNING":
		return models.LogLevelWarning
	case "ERROR", "ERR":
		return models.LogLevelError
	case "FATAL", "CRITICAL", "CRIT":
		return models.LogLevelFatal
	default:
		return models.LogLevelInfo
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
		aiAnalysis, err := lp.llmService.AnalyzeLogsWithAI(&logFile, logFile.Entries)
		if err != nil {
			log.Printf("AI analysis failed, falling back to basic analysis: %v", err)
			// Fallback to basic analysis
			analysis.Severity = lp.determineSeverity(logFile.Entries)
			analysis.Summary = lp.generateSummary(logFile)
		} else {
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
		case models.LogLevelError:
			errorCount++
		case models.LogLevelFatal:
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
