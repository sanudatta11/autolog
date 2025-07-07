package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"context"
	"runtime"
	"sync"
	"syscall"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"gorm.io/gorm"
)

// JobService handles job processing for log analysis
type JobService struct {
	db              *gorm.DB
	llmService      *LLMService
	learningService *LearningService
	feedbackService *FeedbackService
	stopChan        chan struct{}
}

// NewJobService creates a new job service
func NewJobService(db *gorm.DB, llmService *LLMService, learningService *LearningService, feedbackService *FeedbackService) *JobService {
	return &JobService{
		db:              db,
		llmService:      llmService,
		learningService: learningService,
		feedbackService: feedbackService,
		stopChan:        make(chan struct{}),
	}
}

// filterErrorEntries filters only ERROR and FATAL log entries
func (js *JobService) filterErrorEntries(entries []models.LogEntry) []models.LogEntry {
	var errorEntries []models.LogEntry
	for _, entry := range entries {
		if entry.Level == "ERROR" || entry.Level == "FATAL" {
			errorEntries = append(errorEntries, entry)
		}
	}
	return errorEntries
}

// CreateRCAAnalysisJob creates a new RCA analysis job
func (js *JobService) CreateRCAAnalysisJob(logFileID uint) (*models.Job, error) {
	return js.CreateRCAAnalysisJobWithOptions(logFileID, 300, true) // Default timeout 300s, chunking enabled
}

// CreateRCAAnalysisJobWithOptions creates a new RCA analysis job with custom options
func (js *JobService) CreateRCAAnalysisJobWithOptions(logFileID uint, timeout int, chunking bool) (*models.Job, error) {
	job := &models.Job{
		Type:      "rca_analysis",
		LogFileID: logFileID,
		Status:    models.JobStatusPending,
		Progress:  0,
		Result: map[string]interface{}{
			"timeout":  timeout,
			"chunking": chunking,
		},
	}

	if err := js.db.Create(job).Error; err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	// Update log file status
	if err := js.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Updates(map[string]interface{}{
		"rca_analysis_status": "pending",
		"rca_analysis_job_id": job.ID,
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to update log file status: %w", err)
	}

	logger.Info("RCA analysis job created with options", map[string]interface{}{
		"jobID":     job.ID,
		"logFileID": logFileID,
		"timeout":   timeout,
		"chunking":  chunking,
	})

	return job, nil
}

// ProcessRCAAnalysisJobWithShutdown processes an RCA analysis job with shutdown support
func (js *JobService) ProcessRCAAnalysisJobWithShutdown(jobID uint, stopChan <-chan struct{}) {
	completed := false
	var logFileID *uint // Track logFileID for deferred error handling
	defer func() {
		if !completed {
			js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
				"status": models.JobStatusFailed,
				"error":  "Job failed due to shutdown or unexpected exit",
			})
			// Also set log file rca_analysis_status to 'failed' if logFileID is known
			if logFileID != nil {
				js.db.Model(&models.LogFile{}).Where("id = ?", *logFileID).Update("rca_analysis_status", "failed")
			}
		}
	}()

	// Update job status to running
	now := time.Now()
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":     models.JobStatusRunning,
		"started_at": &now,
		"progress":   10,
	}).Error; err != nil {
		logger.Error("Failed to update job status to running", map[string]interface{}{"jobID": jobID, "error": err})
		return
	}

	// Get job details
	var job models.Job
	if err := js.db.Preload("LogFile").First(&job, jobID).Error; err != nil {
		logger.Error("Failed to get job details", map[string]interface{}{"jobID": jobID, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, "Failed to get job details", nil)
		// Try to get logFileID from job record if possible
		var jobRecord models.Job
		if err2 := js.db.First(&jobRecord, jobID).Error; err2 == nil {
			logFileID = &jobRecord.LogFileID
			js.db.Model(&models.LogFile{}).Where("id = ?", jobRecord.LogFileID).Update("rca_analysis_status", "failed")
		}
		return
	}
	logFileID = &job.LogFileID

	// Check for shutdown before starting
	select {
	case <-stopChan:
		return
	default:
	}

	// Check if RCA is possible for this log file
	if job.LogFile != nil && !job.LogFile.IsRCAPossible {
		logger.Info("RCA not possible for this log file, completing job with no RCA needed", map[string]interface{}{"jobID": jobID, "logFileID": job.LogFileID, "reason": job.LogFile.RCANotPossibleReason})
		js.completeJobWithNoRCANeeded(jobID, job.LogFileID, job.LogFile.RCANotPossibleReason)
		completed = true
		return
	}

	// Update progress
	js.updateJobProgress(jobID, 20)

	// Check LLM health before starting analysis
	if err := js.llmService.CheckLLMHealth(); err != nil {
		logger.Error("LLM health check failed", map[string]interface{}{"jobID": jobID, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, fmt.Sprintf("LLM service unavailable: %v", err), nil)
		return
	}

	// Check for shutdown before analysis
	select {
	case <-stopChan:
		return
	default:
	}

	// Perform RCA analysis in chunks with error tracking
	var failedChunk int = -1
	var totalChunks int
	partials, err := js.performRCAAnalysisWithErrorTrackingAndChunkCount(job.LogFile, &failedChunk, &totalChunks, jobID, stopChan)
	if err != nil {
		logger.Error("RCA analysis failed", map[string]interface{}{"jobID": jobID, "error": err})
		failMsg := err.Error()
		if failedChunk > 0 {
			failMsg = fmt.Sprintf("Chunk %d failed: %s", failedChunk, failMsg)
		}
		js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
			"failed_chunk": failedChunk,
			"total_chunks": totalChunks,
		})
		js.updateJobStatus(jobID, models.JobStatusFailed, failMsg, map[string]interface{}{
			"failedChunk": failedChunk,
			"totalChunks": totalChunks,
		})
		js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "failed")
		return
	}
	js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("total_chunks", totalChunks)

	// Store partial results in job.Result
	result := map[string]interface{}{
		"partials": partials,
	}
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("result", result).Error; err != nil {
		logger.Error("Failed to store partial RCA results", map[string]interface{}{"jobID": jobID, "error": err})
	}

	// Update progress after each chunk
	for i := range partials {
		progress := 20 + int(float64(i+1)/float64(len(partials))*60) // 20-80%
		js.updateJobProgress(jobID, progress)
		// Check for shutdown between chunks
		select {
		case <-stopChan:
			return
		default:
		}
	}

	// (Aggregation step)
	js.updateJobProgress(jobID, 85)

	// Check for shutdown before aggregation
	select {
	case <-stopChan:
		return
	default:
	}

	// Aggregate partials into a final RCA report using LLM
	var rawLLMResponse string
	aggregated, rawResp, err := js.aggregatePartialAnalysesWithRaw(job.LogFile, partials)
	if err != nil {
		logger.Error("RCA aggregation failed", map[string]interface{}{"jobID": jobID, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, err.Error(), nil)
		js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "failed")
		return
	}
	logger.Info("[RCA] Raw LLM aggregation response", map[string]interface{}{"jobID": jobID, "response": rawResp})
	rawLLMResponse = rawResp

	// Store final RCA in job.Result
	finalResult := map[string]interface{}{
		"partials":       partials,
		"final":          aggregated,
		"rawLLMResponse": rawLLMResponse,
	}
	completedAt := time.Now()
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":       models.JobStatusCompleted,
		"progress":     100,
		"result":       finalResult,
		"completed_at": &completedAt,
	}).Error; err != nil {
		logger.Error("Failed to update job completion", map[string]interface{}{"jobID": jobID, "error": err})
		return
	}

	// Update log file status
	if err := js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "completed").Error; err != nil {
		logger.Error("Failed to update log file status", map[string]interface{}{"jobID": job.LogFileID, "error": err})
	}

	// Save LogAnalysis and LogAnalysisMemory
	js.saveAnalysisAndMemory(jobID, job.LogFile, aggregated)

	// Learn from the completed analysis
	if js.learningService != nil {
		if err := js.learningService.LearnFromAnalysis(job.LogFile, aggregated); err != nil {
			logger.Error("Failed to learn from RCA analysis", map[string]interface{}{"jobID": jobID, "error": err})
		} else {
			logger.Info("Successfully learned from RCA analysis", map[string]interface{}{
				"jobID":     jobID,
				"logFileID": job.LogFile.ID,
			})
		}
	}

	completed = true
	logger.Info("[ProcessRCAAnalysisJobWithShutdown] RCA analysis completed for job", map[string]interface{}{"jobID": jobID})
}

// completeJobWithNoRCANeeded handles the case where RCA is not needed
func (js *JobService) completeJobWithNoRCANeeded(jobID uint, logFileID uint, reason string) {
	finalResult := map[string]interface{}{
		"final": map[string]interface{}{
			"summary":           reason,
			"severity":          "none",
			"rootCause":         reason,
			"recommendations":   []string{"No RCA needed. No errors or warnings to analyze."},
			"errorAnalysis":     []interface{}{},
			"criticalErrors":    0,
			"nonCriticalErrors": 0,
		},
	}
	completedAt := time.Now()
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":       models.JobStatusCompleted,
		"progress":     100,
		"result":       finalResult,
		"completed_at": &completedAt,
	}).Error; err != nil {
		logger.Error("Failed to update job completion for no RCA needed", map[string]interface{}{"jobID": jobID, "error": err})
		return
	}
	// Update log file status
	if err := js.db.Model(&models.LogFile{}).Where("id = ?", logFileID).Update("rca_analysis_status", "completed").Error; err != nil {
		logger.Error("Failed to update log file status for no RCA needed", map[string]interface{}{"jobID": logFileID, "error": err})
	}
	logger.Info("[ProcessRCAAnalysisJobWithShutdown] RCA job completed with no RCA needed", map[string]interface{}{"jobID": jobID})
}

// saveAnalysisAndMemory saves LogAnalysis and LogAnalysisMemory records
func (js *JobService) saveAnalysisAndMemory(jobID uint, logFile *models.LogFile, aggregated *LogAnalysisResponse) {
	logger.Info("[RCA] Checking conditions for LogAnalysis and LogAnalysisMemory creation", map[string]interface{}{
		"jobID":          jobID,
		"aggregated_nil": aggregated == nil,
		"jobLogFile_nil": logFile == nil,
		"logFileID":      logFile.ID,
	})

	if aggregated == nil {
		logger.Error("[RCA] Aggregated result is nil, will not create LogAnalysis or LogAnalysisMemory", map[string]interface{}{"jobID": jobID})
		return
	}

	if logFile == nil {
		logger.Error("[RCA] job.LogFile is nil, will not create LogAnalysis or LogAnalysisMemory", map[string]interface{}{"jobID": jobID})
		return
	}

	if aggregated.Summary == "" || aggregated.RootCause == "" {
		logger.Warn("[RCA] Skipping LogAnalysis and LogAnalysisMemory creation due to missing summary or rootCause", map[string]interface{}{
			"jobID":             jobID,
			"missing_summary":   aggregated.Summary == "",
			"missing_rootCause": aggregated.RootCause == "",
		})
		return
	}

	// Create LogAnalysis
	logger.Info("[RCA] Creating LogAnalysis record", map[string]interface{}{"jobID": jobID})
	analysis := &models.LogAnalysis{
		LogFileID:    logFile.ID,
		Summary:      aggregated.Summary,
		Severity:     aggregated.Severity,
		ErrorCount:   aggregated.CriticalErrors + aggregated.NonCriticalErrors,
		WarningCount: logFile.WarningCount,
		Metadata: map[string]interface{}{
			"rootCause":         aggregated.RootCause,
			"recommendations":   aggregated.Recommendations,
			"errorAnalysis":     aggregated.ErrorAnalysis,
			"criticalErrors":    aggregated.CriticalErrors,
			"nonCriticalErrors": aggregated.NonCriticalErrors,
			"aiGenerated":       true,
		},
	}
	if err := js.db.Create(analysis).Error; err != nil {
		logger.Error("[RCA] Failed to save LogAnalysis after RCA", map[string]interface{}{"jobID": jobID, "error": err, "analysis": analysis})
	} else {
		logger.Info("[RCA] Successfully created LogAnalysis", map[string]interface{}{"jobID": jobID, "analysisID": analysis.ID})
	}

	// Create LogAnalysisMemory (with embedding)
	var embedding models.JSONB = nil
	if js.llmService != nil {
		prompt := aggregated.Summary + "\n" + aggregated.RootCause
		embed, err := js.llmService.GenerateEmbedding(prompt)
		if err != nil {
			logger.Warn("[RCA] Failed to generate embedding for LogAnalysisMemory", map[string]interface{}{"jobID": jobID, "error": err})
		} else if embed != nil {
			embedding = models.JSONB{"embedding": embed}
		}
	}
	logger.Info("[RCA] Creating LogAnalysisMemory record", map[string]interface{}{"jobID": jobID})
	memory := &models.LogAnalysisMemory{
		LogFileID: &logFile.ID,
		Summary:   aggregated.Summary,
		RootCause: aggregated.RootCause,
		Embedding: embedding,
		Metadata: map[string]interface{}{
			"severity":        aggregated.Severity,
			"recommendations": aggregated.Recommendations,
			"errorAnalysis":   aggregated.ErrorAnalysis,
		},
		CreatedAt: time.Now(),
	}
	if err := js.db.Create(memory).Error; err != nil {
		logger.Error("[RCA] Failed to save LogAnalysisMemory after RCA", map[string]interface{}{"jobID": jobID, "error": err, "memory": memory})
	} else {
		logger.Info("[RCA] Successfully created LogAnalysisMemory", map[string]interface{}{"jobID": jobID, "memoryID": memory.ID})
	}
}

func getAvailableMemoryMB() int {
	var sysinfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysinfo); err != nil {
		return 1024 // fallback to 1GB if unknown
	}
	return int(sysinfo.Freeram / 1024 / 1024)
}

func (js *JobService) performRCAAnalysisWithErrorTrackingAndChunkCount(logFile *models.LogFile, failedChunk *int, totalChunks *int, jobID uint, stopChan <-chan struct{}) ([]*LogAnalysisResponse, error) {
	var entries []models.LogEntry
	if err := js.db.Where("log_file_id = ?", logFile.ID).Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to load log entries: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no log entries found for analysis")
	}

	// Get learning insights for better analysis
	errorEntries := js.filterErrorEntries(entries)
	learningInsights, err := js.learningService.GetLearningInsights(logFile, errorEntries)
	if err != nil {
		logger.Warn("[RCA] Failed to get learning insights, proceeding without them", map[string]interface{}{"jobID": jobID, "error": err})
		learningInsights = &LearningInsights{} // Empty insights
	} else {
		logger.Info("[RCA] Retrieved learning insights", map[string]interface{}{
			"jobID":            jobID,
			"similarIncidents": len(learningInsights.SimilarIncidents),
			"patternMatches":   len(learningInsights.PatternMatches),
			"confidenceBoost":  learningInsights.ConfidenceBoost,
		})
	}

	// Get feedback context for enhanced analysis
	var feedbackContext string
	if js.feedbackService != nil {
		feedbackContext = js.feedbackService.GetFeedbackContext(learningInsights.SimilarIncidents, learningInsights.PatternMatches)
		if feedbackContext != "" {
			logger.Info("[RCA] Retrieved feedback context", map[string]interface{}{
				"jobID":                 jobID,
				"feedbackContextLength": len(feedbackContext),
			})
		}
	}

	chunkSize := 25 // Reduced from 100 to 25 for faster fail/succeed
	var chunks [][]models.LogEntry
	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}
		chunks = append(chunks, entries[i:end])
	}
	*totalChunks = len(chunks)

	// Update TotalChunks in the database before starting LLM processing
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("total_chunks", len(chunks)).Error; err != nil {
		logger.Error("[RCA] Failed to update total chunks in database", map[string]interface{}{
			"jobID":       jobID,
			"totalChunks": len(chunks),
			"error":       err,
		})
	}

	// Calculate concurrency: 70% of available CPU cores and memory
	maxProcs := runtime.NumCPU()
	cpuLimit := int(float64(maxProcs) * 0.7)
	if cpuLimit < 1 {
		cpuLimit = 1
	}
	memMB := getAvailableMemoryMB()
	memLimit := memMB / 300 // assume each chunk/LLM call uses ~300MB
	if memLimit < 1 {
		memLimit = 1
	}
	concurrency := cpuLimit
	if memLimit < cpuLimit {
		concurrency = memLimit
	}
	if concurrency > len(chunks) {
		concurrency = len(chunks)
	}
	logger.Info("[RCA] Dynamic concurrency calculation", map[string]interface{}{"cpuLimit": cpuLimit, "memLimit": memLimit, "finalConcurrency": concurrency, "availableMB": memMB, "maxProcs": maxProcs, "chunks": len(chunks)})

	results := make([]*LogAnalysisResponse, len(chunks))
	errs := make([]error, len(chunks))

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, chunk []models.LogEntry) {
			defer wg.Done()
			logger.Info("[RCA] Waiting for worker slot", map[string]interface{}{"jobID": jobID, "chunk": idx + 1})
			sem <- struct{}{} // acquire
			logger.Info("[RCA] Worker acquired", map[string]interface{}{"jobID": jobID, "chunk": idx + 1})
			defer func() {
				<-sem // release
				logger.Info("[RCA] Worker released", map[string]interface{}{"jobID": jobID, "chunk": idx + 1})
			}()

			var analysis *LogAnalysisResponse
			var err error
			for attempt := 1; attempt <= 3; attempt++ {
				logger.Info("[RCA] Chunk analysis started", map[string]interface{}{"jobID": jobID, "chunk": idx + 1, "attempt": attempt})
				select {
				case <-ctx.Done():
					logger.Warn("[RCA] Chunk cancelled by context", map[string]interface{}{"jobID": jobID, "chunk": idx + 1})
					errs[idx] = fmt.Errorf("job cancelled during chunk processing")
					return
				case <-stopChan:
					logger.Warn("[RCA] Chunk cancelled by stopChan", map[string]interface{}{"jobID": jobID, "chunk": idx + 1})
					errs[idx] = fmt.Errorf("job cancelled during chunk processing")
					return
				default:
				}
				analysis, err = js.analyzeChunkWithEnhancedContext(logFile, chunk, jobID, learningInsights, feedbackContext)
				if err == nil {
					logger.Info("[RCA] Chunk analysis completed", map[string]interface{}{"jobID": jobID, "chunk": idx + 1, "attempt": attempt})
					results[idx] = analysis
					return
				}
				logger.Warn("[RCA] Chunk analysis failed, retrying", map[string]interface{}{"jobID": jobID, "chunk": idx + 1, "attempt": attempt, "error": err.Error()})
				time.Sleep(500 * time.Millisecond)
			}
			logger.Error("[RCA] Chunk analysis failed after retries", map[string]interface{}{"jobID": jobID, "chunk": idx + 1, "error": err.Error()})
			errs[idx] = fmt.Errorf("chunk %d analysis failed after 3 retries: %w", idx+1, err)
			// Cancel all other work if a chunk fails after retries
			cancel()
		}(i, chunk)
	}
	wg.Wait()

	// Check for errors and aggregate results
	var partialResults []*LogAnalysisResponse
	for i, res := range results {
		if errs[i] != nil {
			*failedChunk = i + 1
			logger.Error("[RCA] Chunk analysis failed after retries", map[string]interface{}{"jobID": jobID, "chunk": i + 1, "error": errs[i].Error()})
			return nil, errs[i]
		}
		partialResults = append(partialResults, res)
	}

	return partialResults, nil
}

// analyzeChunkWithEnhancedContext analyzes a chunk with learning insights and feedback context
func (js *JobService) analyzeChunkWithEnhancedContext(logFile *models.LogFile, entries []models.LogEntry, jobID uint, learningInsights *LearningInsights, feedbackContext string) (*LogAnalysisResponse, error) {
	// Filter only ERROR and FATAL entries for analysis
	errorEntries := js.filterErrorEntries(entries)
	if len(errorEntries) == 0 {
		logger.Info("[RCA] No ERROR/FATAL entries found in chunk, using fallback analysis", map[string]interface{}{
			"jobID":        jobID,
			"totalEntries": len(entries),
		})
		return js.llmService.generateNoErrorsAnalysis(logFile), nil
	}

	logger.Info("[RCA] Found ERROR/FATAL entries in chunk, proceeding with enhanced LLM analysis", map[string]interface{}{
		"jobID":        jobID,
		"errorEntries": len(errorEntries),
		"totalEntries": len(entries),
		"hasLearning":  learningInsights != nil,
		"hasFeedback":  feedbackContext != "",
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

	// Create enhanced prompt with learning insights and feedback
	var prompt string
	if learningInsights != nil && (len(learningInsights.SimilarIncidents) > 0 || len(learningInsights.PatternMatches) > 0) || feedbackContext != "" {
		prompt = js.llmService.CreateFeedbackEnhancedPrompt(request, errorEntries, learningInsights, feedbackContext)
		logger.Debug("[RCA] Using feedback-enhanced prompt", map[string]interface{}{
			"jobID":            jobID,
			"similarIncidents": len(learningInsights.SimilarIncidents),
			"patternMatches":   len(learningInsights.PatternMatches),
			"hasFeedback":      feedbackContext != "",
		})
	} else {
		prompt = js.llmService.createDetailedErrorAnalysisPrompt(request, errorEntries, "")
	}

	// Call LLM with the enhanced prompt
	response, err := js.llmService.callLLMWithContext(prompt, &logFile.ID, &jobID, "rca_analysis_with_feedback")
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Parse the response
	analysis, err := js.llmService.parseDetailedLLMResponse(response)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse LLM response: %w", err)
	}

	return analysis, nil
}

// aggregatePartialAnalysesWithRaw aggregates chunk results into a final RCA report using the LLM
func (js *JobService) aggregatePartialAnalysesWithRaw(logFile *models.LogFile, partials []*LogAnalysisResponse) (*LogAnalysisResponse, string, error) {
	if len(partials) == 0 {
		return nil, "", fmt.Errorf("no partial analyses to aggregate")
	}

	// Check if all partials indicate no errors found
	allNoErrors := true
	for _, p := range partials {
		if p.Severity != "low" || p.CriticalErrors > 0 || p.NonCriticalErrors > 0 || len(p.ErrorAnalysis) > 0 {
			allNoErrors = false
			break
		}
	}

	// If all chunks found no errors, return a consolidated "no errors" analysis
	if allNoErrors {
		logger.Info("[RCA] All chunks found no errors, returning consolidated no-errors analysis", nil)
		noErrorsAnalysis := &LogAnalysisResponse{
			Summary:           fmt.Sprintf("Log file '%s' contains no ERROR or FATAL entries across all analyzed chunks. System appears to be functioning normally.", logFile.Filename),
			Severity:          "low",
			RootCause:         "No errors detected in any analyzed log chunks",
			Recommendations:   []string{"Continue monitoring for any new errors", "Review INFO and WARNING logs for potential issues", "System is operating within normal parameters"},
			ErrorAnalysis:     []DetailedErrorAnalysis{},
			CriticalErrors:    0,
			NonCriticalErrors: 0,
		}
		return noErrorsAnalysis, "No errors found - consolidated analysis", nil
	}

	// Prepare aggregation prompt using the constant
	summaryParts := []string{}
	for i, p := range partials {
		summaryParts = append(summaryParts, fmt.Sprintf("Chunk %d: %s", i+1, p.Summary))
	}

	prompt := js.llmService.CreateAggregationPrompt(logFile.Filename, summaryParts)

	// Call LLM for aggregation
	response, err := js.llmService.callLLMWithContext(prompt, &logFile.ID, nil, "rca_aggregation")
	if err != nil {
		return nil, "", fmt.Errorf("LLM aggregation failed: %w", err)
	}

	// Clean and parse the response
	cleanResponse := js.extractJSONFromResponse(response)
	var aggregated LogAnalysisResponse
	if err := json.Unmarshal([]byte(cleanResponse), &aggregated); err != nil {
		logger.Error("[RCA] LLM aggregation response parsing failed", map[string]interface{}{"error": err})
		logger.Info("[RCA] Raw LLM response", map[string]interface{}{"response": response})
		logger.Info("[RCA] Cleaned response", map[string]interface{}{"response": cleanResponse})

		// Try alternative parsing method
		alternativeAnalysis, err := js.parseAlternativeLLMResponse(cleanResponse, partials)
		if err != nil {
			return nil, response, fmt.Errorf("failed to parse LLM aggregation response: %w", err)
		}

		if alternativeAnalysis.Summary == "" || alternativeAnalysis.RootCause == "" {
			return nil, response, fmt.Errorf("LLM aggregation returned incomplete analysis")
		}

		return alternativeAnalysis, response, nil
	}

	if aggregated.Summary == "" || aggregated.RootCause == "" {
		return nil, response, fmt.Errorf("LLM aggregation returned incomplete analysis")
	}

	return &aggregated, response, nil
}

// extractJSONFromResponse attempts to extract JSON from a response that may contain explanatory text
func (js *JobService) extractJSONFromResponse(response string) string {
	// First, try to find JSON blocks
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

	return strings.TrimSpace(response)
}

// parseAlternativeLLMResponse parses the alternative LLM response format with rootCauses array
func (js *JobService) parseAlternativeLLMResponse(response string, partials []*LogAnalysisResponse) (*LogAnalysisResponse, error) {
	// Try to parse the rootCauses format
	var rootCausesData struct {
		RootCauses []struct {
			Chunk  string `json:"chunk"`
			Causes []struct {
				Type        string `json:"type"`
				Description string `json:"description"`
			} `json:"causes"`
		} `json:"rootCauses"`
	}

	if err := json.Unmarshal([]byte(response), &rootCausesData); err != nil {
		return nil, fmt.Errorf("failed to parse rootCauses format: %w", err)
	}

	// Aggregate information from the rootCauses format
	var allCauses []string
	var criticalErrors, nonCriticalErrors int
	var errorAnalysis []DetailedErrorAnalysis

	for _, rootCause := range rootCausesData.RootCauses {
		for _, cause := range rootCause.Causes {
			allCauses = append(allCauses, cause.Description)
			if cause.Type == "critical" {
				criticalErrors++
			} else {
				nonCriticalErrors++
			}
		}
	}

	// Create a consolidated analysis
	summary := "Multiple root causes identified across log chunks"
	if len(allCauses) > 0 {
		summary = fmt.Sprintf("Analysis identified %d issues across %d chunks", len(allCauses), len(rootCausesData.RootCauses))
	}

	severity := "low"
	if criticalErrors > 0 {
		severity = "high"
	} else if nonCriticalErrors > 0 {
		severity = "medium"
	}

	rootCause := "Multiple issues detected"
	if len(allCauses) > 0 {
		rootCause = allCauses[0] // Use first cause as primary
	}

	return &LogAnalysisResponse{
		Summary:           summary,
		Severity:          severity,
		RootCause:         rootCause,
		Recommendations:   []string{"Review all identified issues", "Address critical errors first", "Implement monitoring for similar patterns"},
		ErrorAnalysis:     errorAnalysis,
		CriticalErrors:    criticalErrors,
		NonCriticalErrors: nonCriticalErrors,
	}, nil
}

func (js *JobService) updateJobProgress(jobID uint, progress int) {
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("progress", progress).Error; err != nil {
		logger.Error("Failed to update job progress", map[string]interface{}{"jobID": jobID, "progress": progress, "error": err})
	}
}

func (js *JobService) updateJobStatus(jobID uint, status models.JobStatus, errorMsg string, result map[string]interface{}) {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMsg != "" {
		updates["error"] = errorMsg
	}
	if result != nil {
		updates["result"] = result
	}
	if status == models.JobStatusFailed || status == models.JobStatusCompleted {
		now := time.Now()
		updates["completed_at"] = &now
	}

	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(updates).Error; err != nil {
		logger.Error("Failed to update job status", map[string]interface{}{"jobID": jobID, "status": status, "error": err})
	}
}

func (js *JobService) GetJobStatus(jobID uint) (*models.Job, error) {
	var job models.Job
	if err := js.db.First(&job, jobID).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (js *JobService) GetJobsByLogFile(logFileID uint) ([]models.Job, error) {
	var jobs []models.Job
	if err := js.db.Where("log_file_id = ?", logFileID).Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}
