package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"

	"gorm.io/gorm"
)

type lineCategory int

const (
	validJSON lineCategory = iota
	fixableJSON
	unstructured
)

type lineInfo struct {
	lineNum      int
	content      string
	category     lineCategory
	fixAttempted bool
	fixSuccess   bool
	reason       string
}

type LogProcessor struct {
	db            *gorm.DB
	llmService    *LLMService
	logparserURL  string // Add logparser microservice URL
	dynamicParser *DynamicParserService
}

func NewLogProcessor(db *gorm.DB, llmService *LLMService) *LogProcessor {
	logparserURL := os.Getenv("LOGPARSER_URL")
	if logparserURL == "" {
		logparserURL = "http://localhost:8001"
	}

	logger.Info("LogProcessor initialized", map[string]interface{}{
		"logparser_url": logparserURL,
		"component":     "log_processor",
	})

	dynamicParser := NewDynamicParserService(llmService, db)

	return &LogProcessor{
		db:            db,
		llmService:    llmService,
		logparserURL:  logparserURL,
		dynamicParser: dynamicParser,
	}
}

// Helper to call logparser microservice with a file and return structured logs
func (lp *LogProcessor) callLogparserMicroservice(filePath string, logFormat string) ([]models.LogEntry, string, error) {
	logEntry := logger.WithContext(map[string]interface{}{
		"file_path":  filePath,
		"log_format": logFormat,
		"component":  "log_processor",
		"operation":  "call_logparser_microservice",
	})

	logEntry.Debug("Starting logparser microservice call")

	file, err := os.Open(filePath)
	if err != nil {
		logger.WithError(err, "log_processor").Error("Failed to open file for logparser")
		return nil, "failed to open file for logparser", fmt.Errorf("failed to open file for logparser: %w", err)
	}
	defer file.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		logger.WithError(err, "log_processor").Error("Failed to create form file")
		return nil, "failed to create form file", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(fw, file); err != nil {
		logger.WithError(err, "log_processor").Error("Failed to copy file")
		return nil, "failed to copy file", fmt.Errorf("failed to copy file: %w", err)
	}
	// Add log_format if provided
	if logFormat != "" {
		w.WriteField("log_format", logFormat)
	}
	w.Close()

	req, err := http.NewRequest("POST", lp.logparserURL+"/parse", &b)
	if err != nil {
		logger.WithError(err, "log_processor").Error("Failed to create HTTP request")
		return nil, "failed to create request", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	client := &http.Client{Timeout: 120 * time.Second}

	logEntry.Debug("Sending request to logparser microservice")
	resp, err := client.Do(req)
	if err != nil {
		logger.WithError(err, "log_processor").Error("Logparser microservice request failed")
		return nil, "logparser microservice request failed", fmt.Errorf("logparser microservice request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logger.WithError(fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody)), "log_processor").Error("Logparser microservice returned error status")
		return nil, string(respBody), fmt.Errorf("logparser microservice returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		logger.WithError(err, "log_processor").Error("Failed to decode logparser response")
		return nil, "failed to decode logparser response", fmt.Errorf("failed to decode logparser response: %w", err)
	}

	logEntry.Info("Successfully parsed log entries", map[string]interface{}{
		"parsed_count": len(parsed),
	})

	var entries []models.LogEntry
	for _, item := range parsed {
		entry := NormalizeToLogEntry(0, item) // Pass 0 for logFileID as it's not a file-specific entry
		entries = append(entries, entry)
	}
	return entries, "", nil
}

// Update ProcessLogFile to use logparser microservice for non-JSON logs
func (lp *LogProcessor) ProcessLogFile(logFileID uint, filePath string) error {
	logEntry := logger.WithLogFile(logFileID, filepath.Base(filePath))

	logEntry.Info("Starting log file processing", map[string]interface{}{
		"file_path": filePath,
		"file_size": func() int64 {
			if info, err := os.Stat(filePath); err == nil {
				return info.Size()
			}
			return 0
		}(),
	})

	// Database health check
	var count int64
	if err := lp.db.Model(&models.LogEntry{}).Count(&count).Error; err != nil {
		logger.WithError(err, "log_processor").Error("Database health check failed")
		return fmt.Errorf("database health check failed: %w", err)
	}
	logEntry.Debug("Database health check passed", map[string]interface{}{
		"current_log_entries_count": count,
	})

	// Test database write capability
	testEntry := models.LogEntry{
		LogFileID: logFileID,
		Timestamp: time.Now(),
		Level:     "TEST",
		Message:   "Test entry for database write verification",
	}
	if err := lp.db.Create(&testEntry).Error; err != nil {
		logger.WithError(err, "log_processor").Error("Database write test failed")
		return fmt.Errorf("database write test failed: %w", err)
	}
	logEntry.Debug("Database write test passed", map[string]interface{}{
		"test_entry_id": testEntry.ID,
	})

	// Clean up test entry
	if err := lp.db.Delete(&testEntry).Error; err != nil {
		logger.WithError(err, "log_processor").Warn("Failed to clean up test entry")
	}

	// Update status to processing
	if err := lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Update("status", "processing").Error; err != nil {
		logger.WithError(err, "log_processor").Error("Failed to update log file status to processing")
		return fmt.Errorf("failed to update log file status: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		logger.WithError(err, "log_processor").Error("Failed to open log file")
		lp.updateLogFileStatus(logFileID, "failed")
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Pre-processing: categorize each line

	var (
		lines                                       []lineInfo
		validCount, fixableCount, unstructuredCount int
	)

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Try direct JSON parse
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err == nil {
			lines = append(lines, lineInfo{lineNum, line, validJSON, false, false, ""})
			validCount++
			continue
		}
		// Try to auto-fix common JSON issues (e.g., trailing comma, single quotes)
		fixed, fixReason := attemptFixJSON(line)
		if fixed != "" {
			if err := json.Unmarshal([]byte(fixed), &parsed); err == nil {
				lines = append(lines, lineInfo{lineNum, fixed, fixableJSON, true, true, fixReason})
				fixableCount++
				continue
			}
			lines = append(lines, lineInfo{lineNum, line, fixableJSON, true, false, "Fix attempted but still invalid: " + fixReason})
			unstructuredCount++
			continue
		}
		lines = append(lines, lineInfo{lineNum, line, unstructured, false, false, "Not JSON and not fixable"})
		unstructuredCount++
	}
	if err := scanner.Err(); err != nil {
		logger.WithError(err, "log_processor").Error("Error reading log file")
		lp.updateLogFileStatus(logFileID, "failed")
		return fmt.Errorf("error reading log file: %w", err)
	}

	// Multi-line log entry handling
	lines = lp.handleMultiLineLogs(lines)

	totalLines := validCount + fixableCount + unstructuredCount
	logEntry.Info("Pre-processing complete", map[string]interface{}{
		"total_lines":  totalLines,
		"valid_json":   validCount,
		"fixable_json": fixableCount,
		"unstructured": unstructuredCount,
	})

	if totalLines == 0 {
		logEntry.Warn("No non-empty lines found in log file")
		lp.updateLogFileStatus(logFileID, "failed")
		return fmt.Errorf("no non-empty lines in log file")
	}

	jsonThreshold := 0.8
	jsonLines := validCount + fixableCount
	jsonRatio := float64(jsonLines) / float64(totalLines)
	var entries []models.LogEntry
	var errorCount, warningCount int
	var failedLines []lineInfo

	// Decide parsing strategy: robust hybrid mode
	if jsonRatio > 0 {
		logEntry.Info("Processing as JSON/hybrid log file (robust mode)", map[string]interface{}{
			"json_ratio":   jsonRatio,
			"threshold":    jsonThreshold,
			"valid_json":   validCount,
			"fixable_json": fixableCount,
			"unstructured": unstructuredCount,
		})
		// Parse all valid/fixed JSON lines
		for _, info := range lines {
			if info.category == validJSON || (info.category == fixableJSON && info.fixSuccess) {
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(info.content), &parsed); err == nil {
					entry := NormalizeToLogEntry(logFileID, parsed)
					entry.LogFileID = logFileID
					entries = append(entries, entry)
					if entry.Level == "ERROR" || entry.Level == "FATAL" {
						errorCount++
					} else if entry.Level == "WARN" {
						warningCount++
					}
				} else {
					failedLines = append(failedLines, info)
				}
			} else if info.category == unstructured || (info.category == fixableJSON && !info.fixSuccess) {
				failedLines = append(failedLines, info)
			}
		}
		// Fallback for failed/unstructured lines
		if len(failedLines) > 0 {
			logEntry.Info("Attempting ML/regex fallback for unstructured lines", map[string]interface{}{"count": len(failedLines)})
			var fallbackLines []string
			for _, info := range failedLines {
				fallbackLines = append(fallbackLines, info.content)
			}
			if len(fallbackLines) > 0 {
				tmpPath := filePath + ".unstructured.tmp"
				if err := os.WriteFile(tmpPath, []byte(strings.Join(fallbackLines, "\n")), 0644); err == nil {
					parsedEntries, parseErrMsg, err := lp.callLogparserMicroservice(tmpPath, "")
					if err == nil {
						for i := range parsedEntries {
							parsedEntries[i].LogFileID = logFileID
							entries = append(entries, parsedEntries[i])
							if parsedEntries[i].Level == "ERROR" || parsedEntries[i].Level == "FATAL" {
								errorCount++
							} else if parsedEntries[i].Level == "WARN" {
								warningCount++
							}
						}
						logEntry.Info("ML fallback parsed entries", map[string]interface{}{"count": len(parsedEntries)})
					} else {
						logEntry.Warn("ML fallback failed, using regex fallback", map[string]interface{}{"error": parseErrMsg})
						for _, info := range failedLines {
							parsed := regexFallback(info.content)
							if len(parsed) > 0 {
								entry := NormalizeToLogEntry(logFileID, parsed)
								entry.LogFileID = logFileID
								entries = append(entries, entry)
								if entry.Level == "ERROR" || entry.Level == "FATAL" {
									errorCount++
								} else if entry.Level == "WARN" {
									warningCount++
								}
							}
						}
					}
					os.Remove(tmpPath)
				}
			}
		}
	} else {
		logEntry.Info("Processing as unstructured log file (ML logparser, robust mode)", map[string]interface{}{
			"json_ratio":   jsonRatio,
			"valid_json":   validCount,
			"fixable_json": fixableCount,
			"unstructured": unstructuredCount,
		})
		parsedEntries, parseErrMsg, err := lp.callLogparserMicroservice(filePath, "")
		if err != nil {
			logger.WithError(err, "log_processor").Error("Logparser microservice failed", map[string]interface{}{
				"parse_error_message": parseErrMsg,
			})
			lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Updates(map[string]interface{}{
				"status":      "failed",
				"parse_error": parseErrMsg,
			})
			return fmt.Errorf("logparser microservice failed: %w", err)
		}
		for i := range parsedEntries {
			parsedEntries[i].LogFileID = logFileID
			entries = append(entries, parsedEntries[i])
			if parsedEntries[i].Level == "ERROR" || parsedEntries[i].Level == "FATAL" {
				errorCount++
			} else if parsedEntries[i].Level == "WARN" {
				warningCount++
			}
		}
		logEntry.Info("Unstructured processing complete", map[string]interface{}{"entries_parsed": len(parsedEntries)})
	}

	// Save entries in batches
	if len(entries) > 0 {
		logEntry.Info("Saving entries to database", map[string]interface{}{"entries_to_save": len(entries)})
		for i, entry := range entries {
			if entry.LogFileID == 0 {
				logEntry.Error("Entry has zero LogFileID", map[string]interface{}{"entry_index": i})
			}
			if entry.Timestamp.IsZero() {
				logEntry.Warn("Entry has zero timestamp", map[string]interface{}{"entry_index": i})
			}
			if entry.Message == "" {
				logEntry.Warn("Entry has empty message", map[string]interface{}{"entry_index": i})
			}
		}
		if err := lp.db.CreateInBatches(entries, 100).Error; err != nil {
			logger.WithError(err, "log_processor").Error("Database save error")
			lp.updateLogFileStatus(logFileID, "failed")
			return fmt.Errorf("failed to save log entries: %w", err)
		}
		var savedCount int64
		if err := lp.db.Model(&models.LogEntry{}).Where("log_file_id = ?", logFileID).Count(&savedCount).Error; err != nil {
			logger.WithError(err, "log_processor").Warn("Could not verify saved entries")
		} else {
			logEntry.Info("Verified entries saved to database", map[string]interface{}{"saved_count": savedCount})
		}
		logEntry.Info("Successfully saved entries to database", map[string]interface{}{"entries_saved": len(entries)})
	} else {
		logEntry.Warn("No entries to save")
	}

	// Log/report all failed/skipped lines
	if len(failedLines) > 0 {
		for _, info := range failedLines {
			logEntry.Warn("Line failed to parse", map[string]interface{}{
				"line_number": info.lineNum,
				"reason":      info.reason,
				"content":     info.content,
			})
		}
	}

	// Check if RCA is possible/useful
	isRCAPossible := true
	rcaNotPossibleReason := ""
	if errorCount == 0 && warningCount == 0 {
		isRCAPossible = false
		rcaNotPossibleReason = "No ERROR, FATAL, or WARNING entries found in the log file. RCA analysis is not needed as there are no issues to analyze."
	} else if errorCount == 0 {
		isRCAPossible = false
		rcaNotPossibleReason = "No ERROR or FATAL entries found in the log file. Only warnings are present, which typically don't require RCA analysis."
	}
	// Update log file with final stats
	now := time.Now()
	updateData := map[string]interface{}{
		"status":                  "completed",
		"entry_count":             len(entries),
		"error_count":             errorCount,
		"warning_count":           warningCount,
		"processed_at":            &now,
		"is_rca_possible":         isRCAPossible,
		"rca_not_possible_reason": rcaNotPossibleReason,
	}
	logEntry.Info("Updating log file stats", map[string]interface{}{
		"entries":              len(entries),
		"errors":               errorCount,
		"warnings":             warningCount,
		"status":               "completed",
		"isRCAPossible":        isRCAPossible,
		"rcaNotPossibleReason": rcaNotPossibleReason,
	})
	if err := lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Updates(updateData).Error; err != nil {
		logger.WithError(err, "log_processor").Error("Failed to update log file stats")
		return fmt.Errorf("failed to update log file stats: %w", err)
	}
	if err := os.Remove(filePath); err != nil {
		logger.WithError(err, "log_processor").Warn("Failed to delete uploaded file")
	} else {
		logEntry.Debug("Successfully deleted uploaded file", map[string]interface{}{"file_path": filePath})
	}
	logEntry.Info("Log file processing completed successfully")
	return nil
}

// ProcessLogFileWithShutdown processes a log file with shutdown support
func (lp *LogProcessor) ProcessLogFileWithShutdown(logFileID uint, filePath string, stopChan <-chan struct{}) error {
	completed := false
	defer func() {
		if !completed {
			lp.updateLogFileStatus(logFileID, "failed")
		}
	}()

	logEntry := logger.WithLogFile(logFileID, filepath.Base(filePath))
	logEntry.Info("Starting log file processing", map[string]interface{}{
		"file_path": filePath,
		"file_size": func() int64 {
			if info, err := os.Stat(filePath); err == nil {
				return info.Size()
			}
			return 0
		}(),
	})

	// Check for shutdown before starting
	select {
	case <-stopChan:
		return nil
	default:
	}

	// Database health check
	var count int64
	if err := lp.db.Model(&models.LogEntry{}).Count(&count).Error; err != nil {
		logger.WithError(err, "log_processor").Error("Database health check failed")
		return fmt.Errorf("database health check failed: %w", err)
	}
	logEntry.Debug("Database health check passed", map[string]interface{}{
		"current_log_entries_count": count,
	})

	// Test database write capability
	testEntry := models.LogEntry{
		LogFileID: logFileID,
		Timestamp: time.Now(),
		Level:     "TEST",
		Message:   "Test entry for database write verification",
	}
	if err := lp.db.Create(&testEntry).Error; err != nil {
		logger.WithError(err, "log_processor").Error("Database write test failed")
		return fmt.Errorf("database write test failed: %w", err)
	}
	logEntry.Debug("Database write test passed", map[string]interface{}{
		"test_entry_id": testEntry.ID,
	})

	// Clean up test entry
	if err := lp.db.Delete(&testEntry).Error; err != nil {
		logger.WithError(err, "log_processor").Warn("Failed to clean up test entry")
	}

	// Update status to processing
	if err := lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Update("status", "processing").Error; err != nil {
		logger.WithError(err, "log_processor").Error("Failed to update log file status to processing")
		return fmt.Errorf("failed to update log file status: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		logger.WithError(err, "log_processor").Error("Failed to open log file")
		lp.updateLogFileStatus(logFileID, "failed")
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Pre-processing: categorize each line

	var (
		lines                                       []lineInfo
		validCount, fixableCount, unstructuredCount int
	)

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Try direct JSON parse
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err == nil {
			lines = append(lines, lineInfo{lineNum, line, validJSON, false, false, ""})
			validCount++
			continue
		}
		// Try to auto-fix common JSON issues (e.g., trailing comma, single quotes)
		fixed, fixReason := attemptFixJSON(line)
		if fixed != "" {
			if err := json.Unmarshal([]byte(fixed), &parsed); err == nil {
				lines = append(lines, lineInfo{lineNum, fixed, fixableJSON, true, true, fixReason})
				fixableCount++
				continue
			}
			lines = append(lines, lineInfo{lineNum, line, fixableJSON, true, false, "Fix attempted but still invalid: " + fixReason})
			unstructuredCount++
			continue
		}
		lines = append(lines, lineInfo{lineNum, line, unstructured, false, false, "Not JSON and not fixable"})
		unstructuredCount++
	}
	if err := scanner.Err(); err != nil {
		logger.WithError(err, "log_processor").Error("Error reading log file")
		lp.updateLogFileStatus(logFileID, "failed")
		return fmt.Errorf("error reading log file: %w", err)
	}

	// Multi-line log entry handling
	lines = lp.handleMultiLineLogs(lines)

	totalLines := validCount + fixableCount + unstructuredCount
	logEntry.Info("Pre-processing complete", map[string]interface{}{
		"total_lines":  totalLines,
		"valid_json":   validCount,
		"fixable_json": fixableCount,
		"unstructured": unstructuredCount,
	})

	if totalLines == 0 {
		logEntry.Warn("No non-empty lines found in log file")
		lp.updateLogFileStatus(logFileID, "failed")
		return fmt.Errorf("no non-empty lines in log file")
	}

	jsonThreshold := 0.8
	jsonLines := validCount + fixableCount
	jsonRatio := float64(jsonLines) / float64(totalLines)
	var entries []models.LogEntry
	var errorCount, warningCount int
	var failedLines []lineInfo

	// Decide parsing strategy: robust hybrid mode
	if jsonRatio > 0 {
		logEntry.Info("Processing as JSON/hybrid log file (robust mode)", map[string]interface{}{
			"json_ratio":   jsonRatio,
			"threshold":    jsonThreshold,
			"valid_json":   validCount,
			"fixable_json": fixableCount,
			"unstructured": unstructuredCount,
		})
		// Parse all valid/fixed JSON lines
		for _, info := range lines {
			if info.category == validJSON || (info.category == fixableJSON && info.fixSuccess) {
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(info.content), &parsed); err == nil {
					entry := NormalizeToLogEntry(logFileID, parsed)
					entry.LogFileID = logFileID
					entries = append(entries, entry)
					if entry.Level == "ERROR" || entry.Level == "FATAL" {
						errorCount++
					} else if entry.Level == "WARN" {
						warningCount++
					}
				} else {
					failedLines = append(failedLines, info)
				}
			} else if info.category == unstructured || (info.category == fixableJSON && !info.fixSuccess) {
				failedLines = append(failedLines, info)
			}
		}
		// Fallback for failed/unstructured lines
		if len(failedLines) > 0 {
			logEntry.Info("Attempting ML/regex fallback for unstructured lines", map[string]interface{}{"count": len(failedLines)})
			var fallbackLines []string
			for _, info := range failedLines {
				fallbackLines = append(fallbackLines, info.content)
			}
			if len(fallbackLines) > 0 {
				tmpPath := filePath + ".unstructured.tmp"
				if err := os.WriteFile(tmpPath, []byte(strings.Join(fallbackLines, "\n")), 0644); err == nil {
					parsedEntries, parseErrMsg, err := lp.callLogparserMicroservice(tmpPath, "")
					if err == nil {
						for i := range parsedEntries {
							parsedEntries[i].LogFileID = logFileID
							entries = append(entries, parsedEntries[i])
							if parsedEntries[i].Level == "ERROR" || parsedEntries[i].Level == "FATAL" {
								errorCount++
							} else if parsedEntries[i].Level == "WARN" {
								warningCount++
							}
						}
						logEntry.Info("ML fallback parsed entries", map[string]interface{}{"count": len(parsedEntries)})
					} else {
						logEntry.Warn("ML fallback failed, using regex fallback", map[string]interface{}{"error": parseErrMsg})
						for _, info := range failedLines {
							parsed := regexFallback(info.content)
							if len(parsed) > 0 {
								entry := NormalizeToLogEntry(logFileID, parsed)
								entry.LogFileID = logFileID
								entries = append(entries, entry)
								if entry.Level == "ERROR" || entry.Level == "FATAL" {
									errorCount++
								} else if entry.Level == "WARN" {
									warningCount++
								}
							}
						}
					}
					os.Remove(tmpPath)
				}
			}
		}
	} else {
		logEntry.Info("Processing as unstructured log file (ML logparser, robust mode)", map[string]interface{}{
			"json_ratio":   jsonRatio,
			"valid_json":   validCount,
			"fixable_json": fixableCount,
			"unstructured": unstructuredCount,
		})
		parsedEntries, parseErrMsg, err := lp.callLogparserMicroservice(filePath, "")
		if err != nil {
			logger.WithError(err, "log_processor").Error("Logparser microservice failed", map[string]interface{}{
				"parse_error_message": parseErrMsg,
			})
			lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Updates(map[string]interface{}{
				"status":      "failed",
				"parse_error": parseErrMsg,
			})
			return fmt.Errorf("logparser microservice failed: %w", err)
		}
		for i := range parsedEntries {
			parsedEntries[i].LogFileID = logFileID
			entries = append(entries, parsedEntries[i])
			if parsedEntries[i].Level == "ERROR" || parsedEntries[i].Level == "FATAL" {
				errorCount++
			} else if parsedEntries[i].Level == "WARN" {
				warningCount++
			}
		}
		logEntry.Info("Unstructured processing complete", map[string]interface{}{"entries_parsed": len(parsedEntries)})
	}

	// Save entries in batches
	if len(entries) > 0 {
		logEntry.Info("Saving entries to database", map[string]interface{}{"entries_to_save": len(entries)})
		for i, entry := range entries {
			if entry.LogFileID == 0 {
				logEntry.Error("Entry has zero LogFileID", map[string]interface{}{"entry_index": i})
			}
			if entry.Timestamp.IsZero() {
				logEntry.Warn("Entry has zero timestamp", map[string]interface{}{"entry_index": i})
			}
			if entry.Message == "" {
				logEntry.Warn("Entry has empty message", map[string]interface{}{"entry_index": i})
			}
		}
		if err := lp.db.CreateInBatches(entries, 100).Error; err != nil {
			logger.WithError(err, "log_processor").Error("Database save error")
			lp.updateLogFileStatus(logFileID, "failed")
			return fmt.Errorf("failed to save log entries: %w", err)
		}
		var savedCount int64
		if err := lp.db.Model(&models.LogEntry{}).Where("log_file_id = ?", logFileID).Count(&savedCount).Error; err != nil {
			logger.WithError(err, "log_processor").Warn("Could not verify saved entries")
		} else {
			logEntry.Info("Verified entries saved to database", map[string]interface{}{"saved_count": savedCount})
		}
		logEntry.Info("Successfully saved entries to database", map[string]interface{}{"entries_saved": len(entries)})
	} else {
		logEntry.Warn("No entries to save")
	}

	// Log/report all failed/skipped lines
	if len(failedLines) > 0 {
		for _, info := range failedLines {
			logEntry.Warn("Line failed to parse", map[string]interface{}{
				"line_number": info.lineNum,
				"reason":      info.reason,
				"content":     info.content,
			})
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
	logEntry.Info("Updating log file stats", map[string]interface{}{
		"entries":  len(entries),
		"errors":   errorCount,
		"warnings": warningCount,
		"status":   "completed",
	})
	if err := lp.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Updates(updateData).Error; err != nil {
		logger.WithError(err, "log_processor").Error("Failed to update log file stats")
		return fmt.Errorf("failed to update log file stats: %w", err)
	}
	if err := os.Remove(filePath); err != nil {
		logger.WithError(err, "log_processor").Warn("Failed to delete uploaded file")
	} else {
		logEntry.Debug("Successfully deleted uploaded file", map[string]interface{}{"file_path": filePath})
	}
	logEntry.Info("Log file processing completed successfully")
	completed = true
	return nil
}

// attemptFixJSON tries to auto-fix common JSON issues in a line. Returns fixed string and reason if fix attempted.
func attemptFixJSON(line string) (string, string) {
	// Replace single quotes with double quotes if no double quotes present
	if strings.Contains(line, "'") && !strings.Contains(line, "\"") {
		fixed := strings.ReplaceAll(line, "'", "\"")
		return fixed, "Replaced single quotes with double quotes"
	}
	// Remove trailing comma
	if strings.HasSuffix(line, ",") {
		fixed := strings.TrimRight(line, ",")
		return fixed, "Removed trailing comma"
	}
	// Add missing closing brace(s)
	openBraces := strings.Count(line, "{")
	closeBraces := strings.Count(line, "}")
	if openBraces > closeBraces {
		fixed := line + strings.Repeat("}", openBraces-closeBraces)
		return fixed, "Added missing closing brace(s)"
	}
	// Remove extra closing brace(s)
	if closeBraces > openBraces {
		fixed := line
		for closeBraces > openBraces {
			idx := strings.LastIndex(fixed, "}")
			if idx != -1 {
				fixed = fixed[:idx] + fixed[idx+1:]
				closeBraces--
			}
		}
		return fixed, "Removed extra closing brace(s)"
	}
	// Remove extra comma before closing brace
	if strings.Contains(line, ",}") {
		fixed := strings.ReplaceAll(line, ",}", "}")
		return fixed, "Removed extra comma before closing brace"
	}
	// Remove non-printable/control characters
	clean := strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, line)
	if clean != line {
		return clean, "Removed non-printable/control characters"
	}
	// Attempt to close partial/truncated JSON (if starts with { and missing })
	if strings.HasPrefix(line, "{") && !strings.HasSuffix(line, "}") {
		fixed := line + "}"
		return fixed, "Closed partial/truncated JSON with }"
	}
	// Attempt to escape unescaped double quotes inside values
	if strings.Count(line, "\"")%2 != 0 {
		fixed := strings.ReplaceAll(line, "\"", "\\\"")
		return fixed, "Escaped unescaped double quotes"
	}
	return "", ""
}

// regexFallback attempts to extract timestamp, level, and message from a log line using regex
func regexFallback(line string) map[string]interface{} {
	patterns := []struct {
		name   string
		regex  string
		fields []string
	}{
		// RFC3339 timestamp with hostname, process, and level (systemd/rclone format)
		{"rfc3339_systemd", `^(?P<timestamp>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})) (?P<hostname>\S+) (?P<process>\w+)\[(?P<pid>\d+)\]: (?:(?P<level>INFO|DEBUG|WARN|WARNING|ERROR|FATAL|CRITICAL)[:\s-]*)?(?P<message>.*)$`, []string{"timestamp", "hostname", "process", "pid", "level", "message"}},
		// Apache/Nginx common log format
		{"apache_nginx", `^(?P<ip>\S+) \S+ \S+ \[(?P<timestamp>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) \S+" (?P<status>\d{3}) (?P<size>\d+|-)`, []string{"ip", "timestamp", "method", "path", "status", "size"}},
		// Syslog
		{"syslog", `^(?P<timestamp>[A-Z][a-z]{2} +\d{1,2} \d{2}:\d{2}:\d{2}) (?P<host>\S+) (?P<process>\S+): (?P<message>.*)$`, []string{"timestamp", "host", "process", "message"}},
		// Java stack trace (first line)
		{"java_stack", `^(?P<timestamp>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3}) (?P<level>\w+) \[(?P<thread>[^\]]+)\] (?P<class>\S+) - (?P<message>.*)$`, []string{"timestamp", "level", "thread", "class", "message"}},
		// RFC3339 timestamp with level and message
		{"rfc3339_level", `^(?P<timestamp>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})) (?P<level>INFO|DEBUG|WARN|WARNING|ERROR|FATAL|CRITICAL)[:\s-]*(?P<message>.*)$`, []string{"timestamp", "level", "message"}},
	}
	for _, pat := range patterns {
		re := regexp.MustCompile(pat.regex)
		match := re.FindStringSubmatch(line)
		if match != nil {
			result := make(map[string]interface{})
			for i, name := range re.SubexpNames() {
				if i != 0 && name != "" {
					result[name] = match[i]
				}
			}
			return result
		}
	}
	// Fallback: generic timestamp, level, message extraction
	timestampRegex := `([0-9]{4}-[0-9]{2}-[0-9]{2}[ T][0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?(?:Z|[+-][0-9]{2}:?[0-9]{2})?)`
	levelRegex := `\b(INFO|DEBUG|WARN|WARNING|ERROR|FATAL|CRITICAL)\b`
	msgRegex := `(?:INFO|DEBUG|WARN|WARNING|ERROR|FATAL|CRITICAL)[:\s-]*(.*)$`
	result := make(map[string]interface{})
	if ts := findFirstMatch(line, timestampRegex); ts != "" {
		result["timestamp"] = ts
	}
	if lvl := findFirstMatch(line, levelRegex); lvl != "" {
		result["level"] = lvl
	}
	if msg := findFirstMatch(line, msgRegex); msg != "" {
		result["message"] = strings.TrimSpace(msg)
	}
	return result
}

func findFirstMatch(line, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
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
		"02/Jan/2006:15:04:05 -0700", // Apache/Nginx log format
		"02/Jan/2006:15:04:05 +0000", // Apache/Nginx log format with UTC
		"02/Jan/2006:15:04:05 Z",     // Apache/Nginx log format with Z
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

// ProcessLogsWithDynamicParsing uses LLM to generate patterns and parse logs dynamically
func (lp *LogProcessor) ProcessLogsWithDynamicParsing(logLines []string, logFileID uint) ([]models.LogEntry, error) {
	logEntry := logger.WithContext(map[string]interface{}{
		"component":   "log_processor",
		"log_file_id": logFileID,
		"total_lines": len(logLines),
		"method":      "dynamic_parsing",
	})

	logEntry.Info("Starting dynamic log parsing with LLM")

	// Use dynamic parser to analyze patterns and parse logs
	patterns, err := lp.dynamicParser.AnalyzeLogPatterns(logLines, min(100, len(logLines)))
	if err != nil {
		logEntry.Error("Failed to analyze log patterns with LLM", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to analyze log patterns: %w", err)
	}

	if len(patterns) == 0 {
		logEntry.Warn("No valid patterns generated by LLM, falling back to regex")
		return lp.processLogsWithRegexFallback(logLines, logFileID)
	}

	logEntry.Info("Generated patterns with LLM", map[string]interface{}{
		"patterns_count": len(patterns),
	})

	// Parse logs using generated patterns
	entries, err := lp.dynamicParser.ParseLogsWithDynamicPatterns(logLines, patterns)
	if err != nil {
		logEntry.Error("Failed to parse logs with dynamic patterns", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to parse logs with dynamic patterns: %w", err)
	}

	// Set log file ID for all entries
	for i := range entries {
		entries[i].LogFileID = logFileID
	}

	logEntry.Info("Dynamic parsing completed", map[string]interface{}{
		"entries_parsed": len(entries),
		"patterns_used":  len(patterns),
	})

	return entries, nil
}

// processLogsWithRegexFallback processes logs using traditional regex patterns as fallback
func (lp *LogProcessor) processLogsWithRegexFallback(logLines []string, logFileID uint) ([]models.LogEntry, error) {
	logEntry := logger.WithContext(map[string]interface{}{
		"component":   "log_processor",
		"log_file_id": logFileID,
		"total_lines": len(logLines),
		"method":      "regex_fallback",
	})

	logEntry.Info("Using regex fallback for log parsing")

	var entries []models.LogEntry

	for _, line := range logLines {
		parsed := regexFallback(line)
		if len(parsed) > 0 {
			entry := NormalizeToLogEntry(logFileID, parsed)
			entries = append(entries, entry)
		}
	}

	logEntry.Info("Regex fallback parsing completed", map[string]interface{}{
		"entries_parsed": len(entries),
	})

	return entries, nil
}

// handleMultiLineLogs processes multi-line log entries and merges related lines
func (lp *LogProcessor) handleMultiLineLogs(lines []lineInfo) []lineInfo {
	// If all lines are valid or fixable JSON, do not merge—just return as-is
	allJSON := true
	for _, info := range lines {
		if info.category != validJSON && !(info.category == fixableJSON && info.fixSuccess) {
			allJSON = false
			break
		}
	}
	if allJSON {
		logger.Info("Multi-line log processing skipped: all lines are valid/fixable JSON", map[string]interface{}{
			"component":      "log_processor",
			"original_lines": len(lines),
		})
		return lines
	}

	if len(lines) == 0 {
		return lines
	}

	var processedLines []lineInfo
	var currentMultiLine *lineInfo
	var stackTraceLines []string

	for i, line := range lines {
		// Check if this line is part of a multi-line JSON
		if currentMultiLine != nil {
			// Try to merge with current multi-line JSON
			merged := currentMultiLine.content + "\n" + line.content
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(merged), &parsed); err == nil {
				// Successfully merged multi-line JSON
				currentMultiLine.content = merged
				currentMultiLine.reason = "Merged multi-line JSON"
				continue
			}
			// Try to auto-fix the merged JSON
			fixed, fixReason := attemptFixJSON(merged)
			if fixed != "" {
				if err := json.Unmarshal([]byte(fixed), &parsed); err == nil {
					currentMultiLine.content = fixed
					currentMultiLine.reason = "Merged and fixed multi-line JSON: " + fixReason
					continue
				}
			}
			// If we can't merge, finalize the current multi-line and start new
			processedLines = append(processedLines, *currentMultiLine)
			currentMultiLine = nil
		}

		// Check if this line starts a multi-line JSON (starts with { but doesn't end with })
		if strings.HasPrefix(line.content, "{") && !strings.HasSuffix(strings.TrimSpace(line.content), "}") {
			currentMultiLine = &lineInfo{
				lineNum:      line.lineNum,
				content:      line.content,
				category:     line.category,
				fixAttempted: line.fixAttempted,
				fixSuccess:   line.fixSuccess,
				reason:       "Started multi-line JSON",
			}
			continue
		}

		// Check if this line is part of a stack trace
		if lp.isStackTraceLine(line.content) {
			if len(stackTraceLines) == 0 {
				// Start new stack trace
				stackTraceLines = []string{line.content}
			} else {
				// Continue existing stack trace
				stackTraceLines = append(stackTraceLines, line.content)
			}
			continue
		} else if len(stackTraceLines) > 0 {
			// End of stack trace, merge with previous line
			if len(processedLines) > 0 {
				lastLine := &processedLines[len(processedLines)-1]
				lastLine.content = lastLine.content + "\n" + strings.Join(stackTraceLines, "\n")
				lastLine.reason = "Merged with stack trace"
			}
			stackTraceLines = nil
		}

		// Check if this line is a continuation of a structured log
		previousLine := ""
		if i > 0 {
			previousLine = lines[i-1].content
		}
		if lp.isLogContinuation(line.content, previousLine) {
			if len(processedLines) > 0 {
				lastLine := &processedLines[len(processedLines)-1]
				lastLine.content = lastLine.content + "\n" + line.content
				lastLine.reason = "Merged log continuation"
				continue
			}
		}

		// Regular line, add to processed lines
		processedLines = append(processedLines, line)
	}

	// Handle any remaining multi-line JSON or stack trace
	if currentMultiLine != nil {
		processedLines = append(processedLines, *currentMultiLine)
	}
	if len(stackTraceLines) > 0 {
		if len(processedLines) > 0 {
			lastLine := &processedLines[len(processedLines)-1]
			lastLine.content = lastLine.content + "\n" + strings.Join(stackTraceLines, "\n")
			lastLine.reason = "Merged with stack trace"
		}
	}

	logger.WithContext(map[string]interface{}{
		"original_lines":  len(lines),
		"processed_lines": len(processedLines),
		"component":       "log_processor",
	}).Info("Multi-line log processing completed")

	return processedLines
}

// isStackTraceLine checks if a line is part of a stack trace
func (lp *LogProcessor) isStackTraceLine(line string) bool {
	line = strings.TrimSpace(line)

	// Common stack trace patterns
	patterns := []string{
		`^\s*at\s+\w+\.\w+\([^)]*\)`,                // Java/C# stack trace
		`^\s*File\s+"[^"]*",\s+line\s+\d+`,          // Python stack trace
		`^\s*#\d+\s+\w+\s+in\s+`,                    // Ruby stack trace
		`^\s*from\s+\w+`,                            // Python import error
		`^\s*Caused by:`,                            // Java exception chain
		`^\s*Suppressed:`,                           // Java suppressed exceptions
		`^\s*\w+Exception:`,                         // Generic exception
		`^\s*\w+Error:`,                             // Generic error
		`^\s*\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`, // Timestamped error lines
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, line); matched {
			return true
		}
	}

	// Check for indented lines that might be stack trace continuation
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		// If it's indented and contains common stack trace keywords
		keywords := []string{"at ", "File ", "line ", "Exception", "Error", "Caused by", "from "}
		for _, keyword := range keywords {
			if strings.Contains(line, keyword) {
				return true
			}
		}
	}

	return false
}

// isLogContinuation checks if a line is a continuation of a previous log entry
func (lp *LogProcessor) isLogContinuation(currentLine, previousLine string) bool {
	currentLine = strings.TrimSpace(currentLine)
	previousLine = strings.TrimSpace(previousLine)

	// If previous line is empty, this can't be a continuation
	if previousLine == "" {
		return false
	}

	// Check if current line is indented (common continuation pattern)
	if strings.HasPrefix(currentLine, " ") || strings.HasPrefix(currentLine, "\t") {
		return true
	}

	// Check if previous line ends with continuation indicators
	continuationIndicators := []string{
		"...",
		"\\",
		"|",
		"->",
		"=>",
	}
	for _, indicator := range continuationIndicators {
		if strings.HasSuffix(previousLine, indicator) {
			return true
		}
	}

	// Check if current line starts with continuation indicators
	startIndicators := []string{
		"...",
		"->",
		"=>",
		"|",
	}
	for _, indicator := range startIndicators {
		if strings.HasPrefix(currentLine, indicator) {
			return true
		}
	}

	// Check for JSON continuation (starts with comma, bracket, or brace)
	if strings.HasPrefix(currentLine, ",") ||
		strings.HasPrefix(currentLine, "{") ||
		strings.HasPrefix(currentLine, "[") {
		return true
	}

	return false
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
			logger.WithError(err, "log_processor").Error("AI analysis failed for log file")
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
	// Apply user-defined parsing rules if available
	// Note: This would require passing userID to this function
	// For now, we'll use the built-in synonym mapping
	// Synonym mapping for canonical fields
	synonyms := map[string][]string{
		"timestamp":      {"timestamp", "ts", "time", "date", "datetime", "@timestamp"},
		"service":        {"service", "svc", "app", "application", "component"},
		"host":           {"host", "hostname", "node", "server"},
		"environment":    {"environment", "env", "stage"},
		"level":          {"level", "severity", "log_level", "lvl", "priority"},
		"error_code":     {"error_code", "err_code", "code", "errorCode"},
		"message":        {"message", "msg", "log", "log_message", "text", "body"},
		"exception":      {"exception", "error", "err", "exc"},
		"context":        {"context", "ctx"},
		"tags":           {"tags", "labels"},
		"correlation_id": {"correlation_id", "corr_id", "correlationID", "trace_id", "traceID"},
		"metadata":       {"metadata", "meta", "extra", "details"},
	}

	getString := func(keys ...string) string {
		for _, key := range keys {
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
		}
		return ""
	}
	getStringSlice := func(keys ...string) []string {
		for _, key := range keys {
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
		}
		return nil
	}
	getTime := func(keys ...string) time.Time {
		for _, key := range keys {
			if v, ok := parsed[key]; ok {
				if s, ok := v.(string); ok && s != "" {
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
						"02/Jan/2006:15:04:05 -0700", // Apache/Nginx log format
						"02/Jan/2006:15:04:05 +0000", // Apache/Nginx log format with UTC
						"02/Jan/2006:15:04:05 Z",     // Apache/Nginx log format with Z
					}
					for _, format := range formats {
						t, err := time.Parse(format, s)
						if err == nil {
							return t
						}
					}
					logger.WithContext(map[string]interface{}{
						"timestamp": s,
						"component": "log_processor",
					}).Errorf("Failed to parse timestamp: %q", s)
				}
			}
		}
		return time.Time{}
	}

	// Exception
	exception := models.Exception{}
	for _, key := range synonyms["exception"] {
		if exc, ok := parsed[key].(map[string]interface{}); ok {
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
			break
		}
	}
	// Context
	context := models.Context{}
	for _, key := range synonyms["context"] {
		if ctx, ok := parsed[key].(map[string]interface{}); ok {
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
			break
		}
	}

	// Extract metadata field if present, or fallback to unmapped fields
	var logMetadata map[string]interface{}
	for _, key := range synonyms["metadata"] {
		if meta, ok := parsed[key].(map[string]interface{}); ok {
			logMetadata = meta
			break
		}
	}
	if logMetadata == nil {
		// Only extract unmapped fields as metadata if there's no explicit metadata field
		logMetadata = extractMetadata(parsed)
	}

	// Canonical field extraction with synonym fallback
	timestamp := getTime(synonyms["timestamp"]...)
	service := getString(synonyms["service"]...)
	host := getString(synonyms["host"]...)
	environment := getString(synonyms["environment"]...)
	levelRaw := getString(synonyms["level"]...)
	errorCode := getString(synonyms["error_code"]...)
	message := getString(synonyms["message"]...)
	tags := getStringSlice(synonyms["tags"]...)
	correlationId := getString(synonyms["correlation_id"]...)

	// Normalize log level
	level := strings.ToUpper(strings.TrimSpace(levelRaw))
	switch level {
	case "DEBUG", "DBG", "TRACE":
		level = "DEBUG"
	case "INFO", "INF", "NOTICE":
		level = "INFO"
	case "WARN", "WARNING", "WARNG":
		level = "WARN"
	case "ERROR", "ERR":
		level = "ERROR"
	case "FATAL", "CRITICAL", "CRIT":
		level = "FATAL"
	default:
		if level == "" {
			level = "INFO"
		} else {
			level = levelRaw // fallback to original if unknown
		}
	}

	entry := models.LogEntry{
		LogFileID:     logFileID,
		Timestamp:     timestamp,
		Service:       service,
		Host:          host,
		Environment:   environment,
		Level:         level,
		ErrorCode:     errorCode,
		Message:       message,
		Exception:     exception,
		Context:       context,
		Tags:          tags,
		CorrelationId: correlationId,
		Metadata:      logMetadata, // Use the metadata field from the JSON or fallback
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
