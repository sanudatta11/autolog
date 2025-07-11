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
	"golang.org/x/sync/semaphore"
	"gorm.io/gorm"
)

// JobService handles job processing for log analysis
type JobService struct {
	db              *gorm.DB
	llmService      *LLMService
	learningService *LearningService
	feedbackService *FeedbackService
	stopChan        chan struct{}
	// Add cancellation tracking
	activeJobs map[uint]chan struct{} // jobID -> cancellation channel
	jobMutex   sync.RWMutex
}

// NewJobService creates a new job service
func NewJobService(db *gorm.DB, llmService *LLMService, learningService *LearningService, feedbackService *FeedbackService) *JobService {
	return &JobService{
		db:              db,
		llmService:      llmService,
		learningService: learningService,
		feedbackService: feedbackService,
		stopChan:        make(chan struct{}),
		activeJobs:      make(map[uint]chan struct{}),
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
	return js.CreateRCAAnalysisJobWithOptions(logFileID, 120, true, true, "") // Default timeout 120s, chunking enabled, smart chunking enabled, default model
}

// CreateRCAAnalysisJobWithOptions creates a new RCA analysis job with custom options
func (js *JobService) CreateRCAAnalysisJobWithOptions(logFileID uint, timeout int, chunking bool, smartChunking bool, model string) (*models.Job, error) {
	job := &models.Job{
		Type:      "rca_analysis",
		LogFileID: logFileID,
		Status:    models.JobStatusPending,
		Progress:  0,
		Result: map[string]interface{}{
			"timeout":       timeout,
			"chunking":      chunking,
			"smartChunking": smartChunking,
			"model":         model, // Add model to job result
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
		"jobID":         job.ID,
		"logFileID":     logFileID,
		"timeout":       timeout,
		"chunking":      chunking,
		"smartChunking": smartChunking,
		"model":         model,
	})

	return job, nil
}

// ProcessRCAAnalysisJobWithShutdown processes an RCA analysis job with shutdown support
func (js *JobService) ProcessRCAAnalysisJobWithShutdown(jobID uint, stopChan <-chan struct{}) {
	completed := false
	var logFileID *uint // Track logFileID for deferred error handling

	// Register this job for cancellation
	jobCancelChan := js.registerJob(jobID)
	defer func() {
		js.unregisterJob(jobID)
		if !completed {
			logger.Error("[RCA] Job failed due to shutdown or unexpected exit", map[string]interface{}{"jobID": jobID})
			js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
				"status":        models.JobStatusFailed,
				"error":         "Job failed due to shutdown or unexpected exit",
				"current_chunk": 0, // Reset current chunk when failed
			})
			// Also set log file rca_analysis_status to 'failed' if logFileID is known
			if logFileID != nil {
				js.db.Model(&models.LogFile{}).Where("id = ?", *logFileID).Update("rca_analysis_status", "failed")
			}
		}
	}()

	logger.Info("[RCA] Starting RCA analysis job", map[string]interface{}{"jobID": jobID})

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
	logger.Info("[RCA] Job status updated to running", map[string]interface{}{"jobID": jobID, "progress": 10})

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
	logger.Info("[RCA] Job details retrieved", map[string]interface{}{
		"jobID":     jobID,
		"logFileID": job.LogFileID,
		"filename":  job.LogFile.Filename,
	})

	// Check for shutdown before starting
	select {
	case <-stopChan:
		logger.Info("[RCA] Job cancelled before starting analysis", map[string]interface{}{"jobID": jobID})
		return
	case <-jobCancelChan:
		logger.Info("[RCA] Job cancelled by user before starting analysis", map[string]interface{}{"jobID": jobID})
		return
	default:
	}

	// Check if RCA is possible for this log file
	if job.LogFile != nil && !job.LogFile.IsRCAPossible {
		logger.Info("RCA not possible for this log file, completing job with no RCA needed", map[string]interface{}{
			"jobID":     jobID,
			"logFileID": job.LogFileID,
			"reason":    job.LogFile.RCANotPossibleReason,
		})
		js.completeJobWithNoRCANeeded(jobID, job.LogFileID, job.LogFile.RCANotPossibleReason)
		completed = true
		return
	}

	// Update progress
	js.updateJobProgress(jobID, 20)
	logger.Info("[RCA] Starting user configuration check", map[string]interface{}{"jobID": jobID, "progress": 20})

	// Get user's LLM endpoint from the log file's uploaded_by user
	var user models.User
	if err := js.db.First(&user, job.LogFile.UploadedBy).Error; err != nil {
		logger.Error("Failed to get user for LLM endpoint", map[string]interface{}{"jobID": jobID, "userID": job.LogFile.UploadedBy, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, "Failed to get user configuration", nil)
		return
	}
	logger.Info("[RCA] User configuration retrieved", map[string]interface{}{
		"jobID":       jobID,
		"userID":      user.ID,
		"hasEndpoint": user.LLMEndpoint != nil && *user.LLMEndpoint != "",
	})

	// Check if user has LLM endpoint configured
	if user.LLMEndpoint == nil || *user.LLMEndpoint == "" {
		logger.Error("User has no LLM endpoint configured", map[string]interface{}{"jobID": jobID, "userID": user.ID})
		js.updateJobStatus(jobID, models.JobStatusFailed, "LLM endpoint not configured for user", nil)
		return
	}

	// Check LLM health before starting analysis using user's endpoint
	logger.Info("[RCA] Checking LLM health", map[string]interface{}{"jobID": jobID, "endpoint": *user.LLMEndpoint})
	if err := js.llmService.CheckLLMStatusWithEndpoint(*user.LLMEndpoint); err != nil {
		logger.Error("LLM health check failed", map[string]interface{}{"jobID": jobID, "user_endpoint": *user.LLMEndpoint, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, fmt.Sprintf("LLM service unavailable at %s: %v", *user.LLMEndpoint, err), nil)
		return
	}
	logger.Info("[RCA] LLM health check passed", map[string]interface{}{"jobID": jobID, "endpoint": *user.LLMEndpoint})

	// Check for shutdown before analysis
	select {
	case <-stopChan:
		logger.Info("[RCA] Job cancelled before starting analysis", map[string]interface{}{"jobID": jobID})
		return
	case <-jobCancelChan:
		logger.Info("[RCA] Job cancelled by user before starting analysis", map[string]interface{}{"jobID": jobID})
		return
	default:
	}

	// Perform RCA analysis in chunks with error tracking
	var failedChunk int = -1
	var totalChunks int

	logger.Info("[RCA] Starting chunk analysis preparation", map[string]interface{}{"jobID": jobID})

	// Safely convert timeout from interface{} to int
	var timeout int
	if timeoutVal, ok := job.Result["timeout"]; ok {
		switch v := timeoutVal.(type) {
		case int:
			timeout = v
		case float64:
			timeout = int(v)
		default:
			timeout = 300 // default fallback
		}
	} else {
		timeout = 300 // default fallback
	}

	// Safely convert smartChunking from interface{} to bool
	var smartChunking bool = true // default to true
	if smartChunkingVal, ok := job.Result["smartChunking"]; ok {
		switch v := smartChunkingVal.(type) {
		case bool:
			smartChunking = v
		default:
			smartChunking = true // default fallback
		}
	}

	// Safely extract model from interface{} to string
	var selectedModel string = "" // default to empty string (will use default model)
	if modelVal, ok := job.Result["model"]; ok {
		switch v := modelVal.(type) {
		case string:
			selectedModel = v
		default:
			selectedModel = "" // default fallback
		}
	}

	logger.Info("[RCA] Analysis parameters configured", map[string]interface{}{
		"jobID":         jobID,
		"timeout":       timeout,
		"smartChunking": smartChunking,
		"model":         selectedModel,
	})

	logger.Info("[RCA] Starting chunk analysis", map[string]interface{}{"jobID": jobID})
	partials, err := js.performRCAAnalysisWithErrorTrackingAndChunkCount(job.LogFile, &failedChunk, &totalChunks, jobID, stopChan, jobCancelChan, timeout, smartChunking, selectedModel)
	if err != nil {
		logger.Error("RCA analysis failed", map[string]interface{}{"jobID": jobID, "error": err})
		failMsg := err.Error()
		if failedChunk > 0 {
			failMsg = fmt.Sprintf("Chunk %d failed: %s", failedChunk, failMsg)
		}
		js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
			"failed_chunk":  failedChunk,
			"total_chunks":  totalChunks,
			"current_chunk": 0, // Reset current chunk when failed
		})
		js.updateJobStatus(jobID, models.JobStatusFailed, failMsg, map[string]interface{}{
			"failedChunk": failedChunk,
			"totalChunks": totalChunks,
		})
		js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "failed")
		return
	}

	logger.Info("[RCA] Chunk analysis completed successfully", map[string]interface{}{
		"jobID":       jobID,
		"totalChunks": totalChunks,
		"partials":    len(partials),
	})

	js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("total_chunks", totalChunks)

	// Store partial results in job.Result
	result := map[string]interface{}{
		"partials": partials,
	}
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("result", result).Error; err != nil {
		logger.Error("Failed to store partial RCA results", map[string]interface{}{"jobID": jobID, "error": err})
	}
	logger.Info("[RCA] Partial results stored", map[string]interface{}{"jobID": jobID, "partialsCount": len(partials)})

	// Update progress after each chunk
	for i := range partials {
		progress := 20 + int(float64(i+1)/float64(len(partials))*60) // 20-80%
		js.updateJobProgress(jobID, progress)
		logger.Info("[RCA] Chunk progress updated", map[string]interface{}{
			"jobID":    jobID,
			"chunk":    i + 1,
			"progress": progress,
		})
		// Check for shutdown between chunks
		select {
		case <-stopChan:
			logger.Info("[RCA] Job cancelled during chunk processing", map[string]interface{}{"jobID": jobID})
			return
		case <-jobCancelChan:
			logger.Info("[RCA] Job cancelled by user during chunk processing", map[string]interface{}{"jobID": jobID})
			return
		default:
		}
	}

	// (Aggregation step)
	js.updateJobProgress(jobID, 85)
	logger.Info("[RCA] Starting aggregation phase", map[string]interface{}{"jobID": jobID, "progress": 85})

	// Check for shutdown before aggregation
	select {
	case <-stopChan:
		logger.Info("[RCA] Job cancelled before aggregation", map[string]interface{}{"jobID": jobID})
		return
	case <-jobCancelChan:
		logger.Info("[RCA] Job cancelled by user before aggregation", map[string]interface{}{"jobID": jobID})
		return
	default:
	}

	// Aggregate partials into a final RCA report using LLM
	var rawLLMResponse string
	logger.Info("[RCA] Calling LLM for aggregation", map[string]interface{}{"jobID": jobID})
	aggregated, rawResp, err := js.aggregatePartialAnalysesWithRaw(job.LogFile, partials, timeout)
	if err != nil {
		logger.Error("RCA aggregation failed", map[string]interface{}{"jobID": jobID, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, err.Error(), nil)
		js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "failed")
		return
	}
	logger.Info("[RCA] LLM aggregation completed successfully", map[string]interface{}{
		"jobID":    jobID,
		"response": rawResp,
	})
	rawLLMResponse = rawResp

	// Store final RCA in job.Result
	finalResult := map[string]interface{}{
		"partials":       partials,
		"final":          aggregated,
		"rawLLMResponse": rawLLMResponse,
	}
	completedAt := time.Now()
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":        models.JobStatusCompleted,
		"progress":      100,
		"result":        finalResult,
		"completed_at":  &completedAt,
		"current_chunk": 0, // Reset current chunk when completed
	}).Error; err != nil {
		logger.Error("Failed to update job completion", map[string]interface{}{"jobID": jobID, "error": err})
		return
	}
	logger.Info("[RCA] Job marked as completed", map[string]interface{}{"jobID": jobID, "progress": 100})

	// Update log file status
	if err := js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "completed").Error; err != nil {
		logger.Error("Failed to update log file status", map[string]interface{}{"jobID": job.LogFileID, "error": err})
	}
	logger.Info("[RCA] Log file status updated to completed", map[string]interface{}{"jobID": jobID, "logFileID": job.LogFileID})

	// Save LogAnalysis and LogAnalysisMemory
	logger.Info("[RCA] Saving analysis and memory records", map[string]interface{}{"jobID": jobID})
	js.saveAnalysisAndMemory(jobID, job.LogFile, aggregated)

	// Learn from the completed analysis
	if js.learningService != nil {
		logger.Info("[RCA] Starting learning from analysis", map[string]interface{}{"jobID": jobID})
		if err := js.learningService.LearnFromAnalysis(job.LogFile, aggregated); err != nil {
			logger.Error("Failed to learn from RCA analysis", map[string]interface{}{"jobID": jobID, "error": err})
		} else {
			logger.Info("Successfully learned from RCA analysis", map[string]interface{}{
				"jobID":     jobID,
				"logFileID": job.LogFile.ID,
			})
		}
	} else {
		logger.Info("[RCA] Learning service not available, skipping learning", map[string]interface{}{"jobID": jobID})
	}

	completed = true
	logger.Info("[RCA] RCA analysis completed successfully", map[string]interface{}{
		"jobID":       jobID,
		"totalChunks": totalChunks,
		"partials":    len(partials),
		"duration":    time.Since(now),
	})
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
		"status":        models.JobStatusCompleted,
		"progress":      100,
		"result":        finalResult,
		"completed_at":  &completedAt,
		"current_chunk": 0, // Reset current chunk when completed
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

func (js *JobService) performRCAAnalysisWithErrorTrackingAndChunkCount(logFile *models.LogFile, failedChunk *int, totalChunks *int, jobID uint, stopChan <-chan struct{}, jobCancelChan <-chan struct{}, timeout int, smartChunking bool, model string) ([]*LogAnalysisResponse, error) {
	logger.Info("[RCA] Starting chunk analysis preparation", map[string]interface{}{
		"jobID":         jobID,
		"logFileID":     logFile.ID,
		"filename":      logFile.Filename,
		"smartChunking": smartChunking,
		"timeout":       timeout,
	})

	var entries []models.LogEntry
	if err := js.db.Where("log_file_id = ?", logFile.ID).Find(&entries).Error; err != nil {
		logger.Error("[RCA] Failed to load log entries", map[string]interface{}{"jobID": jobID, "error": err})
		return nil, fmt.Errorf("failed to load log entries: %w", err)
	}
	if len(entries) == 0 {
		logger.Error("[RCA] No log entries found for analysis", map[string]interface{}{"jobID": jobID})
		return nil, fmt.Errorf("no log entries found for analysis")
	}

	logger.Info("[RCA] Log entries loaded", map[string]interface{}{
		"jobID":        jobID,
		"totalEntries": len(entries),
	})

	// Get learning insights for better analysis
	errorEntries := js.filterErrorEntries(entries)
	logger.Info("[RCA] Error entries filtered", map[string]interface{}{
		"jobID":        jobID,
		"errorEntries": len(errorEntries),
		"totalEntries": len(entries),
	})

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
		} else {
			logger.Info("[RCA] No feedback context available", map[string]interface{}{"jobID": jobID})
		}
	} else {
		logger.Info("[RCA] Feedback service not available", map[string]interface{}{"jobID": jobID})
	}

	// Declare chunks and chunkIndices variables at function scope
	var chunks [][]models.LogEntry
	var chunkIndices []int

	// Calculate dynamic chunk size based on total entries
	// Target: 2-5% of total entries, with minimum 5 and maximum 50 entries per chunk
	totalEntries := len(entries)
	targetPercentage := 0.035 // 3.5% (reduced from 7.5% to prevent memory issues)
	targetChunkSize := int(float64(totalEntries) * targetPercentage)

	// Apply min/max constraints
	if targetChunkSize < 5 {
		targetChunkSize = 5
	} else if targetChunkSize > 50 {
		targetChunkSize = 50
	}

	// Calculate number of chunks we'll create
	totalChunksToCreate := (totalEntries + targetChunkSize - 1) / targetChunkSize // Ceiling division

	logger.Info("[RCA] Dynamic chunk size calculation", map[string]interface{}{
		"jobID":               jobID,
		"totalEntries":        totalEntries,
		"targetPercentage":    targetPercentage,
		"targetChunkSize":     targetChunkSize,
		"totalChunksToCreate": totalChunksToCreate,
		"actualPercentage":    fmt.Sprintf("%.2f%%", float64(targetChunkSize)/float64(totalEntries)*100),
	})

	if smartChunking {
		logger.Info("[RCA] Using smart chunking strategy", map[string]interface{}{"jobID": jobID})
		// Smart chunking: Only create chunks that contain ERROR or FATAL entries
		var currentChunkIndex int

		for i := 0; i < len(entries); i += targetChunkSize {
			end := i + targetChunkSize
			if end > len(entries) {
				end = len(entries)
			}

			chunk := entries[i:end]
			// Check if this chunk contains any ERROR or FATAL entries
			hasErrors := false
			for _, entry := range chunk {
				if entry.Level == "ERROR" || entry.Level == "FATAL" {
					hasErrors = true
					break
				}
			}

			// Only include chunks that have errors
			if hasErrors {
				chunks = append(chunks, chunk)
				chunkIndices = append(chunkIndices, currentChunkIndex)
				logger.Info("[RCA] Smart chunking: Including chunk with errors", map[string]interface{}{
					"jobID":        jobID,
					"chunkIndex":   currentChunkIndex,
					"totalEntries": len(chunk),
					"hasErrors":    hasErrors,
				})
			} else {
				logger.Info("[RCA] Smart chunking: Skipping chunk without errors", map[string]interface{}{
					"jobID":        jobID,
					"chunkIndex":   currentChunkIndex,
					"totalEntries": len(chunk),
					"hasErrors":    hasErrors,
				})
			}
			currentChunkIndex++
		}

		// If no chunks have errors, create a single chunk for analysis
		if len(chunks) == 0 {
			logger.Info("[RCA] Smart chunking: No chunks with errors found, creating single chunk for analysis", map[string]interface{}{
				"jobID": jobID,
			})
			chunks = [][]models.LogEntry{entries}
			chunkIndices = []int{0}
		}

		*totalChunks = len(chunks)

		// Log smart chunking statistics
		totalOriginalChunks := currentChunkIndex
		skippedChunks := totalOriginalChunks - len(chunks)
		logger.Info("[RCA] Smart chunking completed", map[string]interface{}{
			"jobID":               jobID,
			"totalOriginalChunks": totalOriginalChunks,
			"chunksWithErrors":    len(chunks),
			"skippedChunks":       skippedChunks,
			"llmCallsSaved":       skippedChunks,
			"efficiencyGain":      fmt.Sprintf("%.1f%%", float64(skippedChunks)/float64(totalOriginalChunks)*100),
		})

		// Update TotalChunks in the database before starting LLM processing
		if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("total_chunks", len(chunks)).Error; err != nil {
			logger.Error("[RCA] Failed to update total chunks in database", map[string]interface{}{
				"jobID":       jobID,
				"totalChunks": len(chunks),
				"error":       err,
			})
		}
	} else {
		logger.Info("[RCA] Using regular chunking strategy", map[string]interface{}{"jobID": jobID})
		// Regular chunking: include all chunks regardless of error level
		for i := 0; i < len(entries); i += targetChunkSize {
			end := i + targetChunkSize
			if end > len(entries) {
				end = len(entries)
			}
			chunks = append(chunks, entries[i:end])
		}
		*totalChunks = len(chunks)

		// Initialize chunkIndices for regular chunking (sequential)
		chunkIndices = make([]int, len(chunks))
		for i := range chunkIndices {
			chunkIndices[i] = i
		}

		// Log regular chunking statistics
		logger.Info("[RCA] Regular chunking completed", map[string]interface{}{
			"jobID":               jobID,
			"totalOriginalChunks": len(entries),
			"chunksWithErrors":    len(chunks),
			"llmCallsSaved":       0,
			"efficiencyGain":      "0.0%",
		})

		// Update TotalChunks in the database before starting LLM processing
		if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("total_chunks", len(chunks)).Error; err != nil {
			logger.Error("[RCA] Failed to update total chunks in database", map[string]interface{}{
				"jobID":       jobID,
				"totalChunks": len(chunks),
				"error":       err,
			})
		}
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
	if concurrency > *totalChunks {
		concurrency = *totalChunks
	}
	logger.Info("[RCA] Dynamic concurrency calculation", map[string]interface{}{
		"jobID":            jobID,
		"cpuLimit":         cpuLimit,
		"memLimit":         memLimit,
		"finalConcurrency": concurrency,
		"availableMB":      memMB,
		"maxProcs":         maxProcs,
		"totalChunks":      *totalChunks,
	})

	logger.Info("[RCA] Starting parallel chunk processing", map[string]interface{}{
		"jobID":       jobID,
		"totalChunks": *totalChunks,
		"concurrency": concurrency,
		"timeout":     timeout,
	})

	results := make([]*LogAnalysisResponse, *totalChunks)
	errs := make([]error, *totalChunks)
	var errsMutex sync.Mutex // Add mutex for thread-safe error writing

	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(int64(concurrency))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a context that is cancelled by either the timeout or jobCancelChan
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	go func() {
		select {
		case <-jobCancelChan:
			cancelFunc()
		case <-ctx.Done():
		}
	}()
	defer cancelFunc()

	for i := 0; i < *totalChunks; i++ {
		wg.Add(1)
		if err := sem.Acquire(ctx, 1); err != nil {
			wg.Done()
			logger.Error("[RCA] Failed to acquire semaphore", map[string]interface{}{"jobID": jobID, "error": err})
			return nil, fmt.Errorf("failed to acquire semaphore: %w", err)
		}

		// Update current chunk in database before starting this chunk
		// Use the original chunk index for display purposes
		originalChunkIndex := i + 1
		if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("current_chunk", originalChunkIndex).Error; err != nil {
			logger.Error("[RCA] Failed to update current chunk in database", map[string]interface{}{
				"jobID":        jobID,
				"currentChunk": originalChunkIndex,
				"error":        err,
			})
		}

		logger.Info("[RCA] Starting chunk processing", map[string]interface{}{
			"jobID":       jobID,
			"chunkIndex":  i + 1,
			"totalChunks": *totalChunks,
			"chunkSize":   len(chunks[i]),
		})

		go func(idx int, chunk []models.LogEntry, originalChunkIdx int) {
			defer wg.Done()
			defer sem.Release(1)

			var analysis *LogAnalysisResponse
			var err error
			for attempt := 1; attempt <= 3; attempt++ {
				select {
				case <-ctx.Done():
					logger.Info("[RCA] Chunk cancelled during processing", map[string]interface{}{
						"jobID":      jobID,
						"chunkIndex": originalChunkIdx + 1,
					})
					errsMutex.Lock()
					errs[idx] = fmt.Errorf("job cancelled during chunk processing")
					errsMutex.Unlock()
					return
				case <-stopChan:
					logger.Info("[RCA] Chunk cancelled due to shutdown", map[string]interface{}{
						"jobID":      jobID,
						"chunkIndex": originalChunkIdx + 1,
					})
					errsMutex.Lock()
					errs[idx] = fmt.Errorf("job cancelled during chunk processing")
					errsMutex.Unlock()
					return
				case <-jobCancelChan:
					logger.Info("[RCA] Chunk cancelled by user during processing", map[string]interface{}{
						"jobID":      jobID,
						"chunkIndex": originalChunkIdx + 1,
					})
					errsMutex.Lock()
					errs[idx] = fmt.Errorf("job cancelled by user during chunk processing")
					errsMutex.Unlock()
					return
				default:
				}

				logger.Info("[RCA] Processing chunk attempt", map[string]interface{}{
					"jobID":      jobID,
					"chunkIndex": originalChunkIdx + 1,
					"attempt":    attempt,
				})

				analysis, err = js.analyzeChunkWithEnhancedContext(cancelCtx, logFile, chunk, jobID, learningInsights, feedbackContext, timeout, model)
				if err == nil {
					logger.Info("[RCA] Chunk processed successfully", map[string]interface{}{
						"jobID":      jobID,
						"chunkIndex": originalChunkIdx + 1,
						"attempt":    attempt,
					})
					errsMutex.Lock()
					results[idx] = analysis
					errsMutex.Unlock()
					return
				}

				logger.Warn("[RCA] Chunk processing failed, retrying", map[string]interface{}{
					"jobID":      jobID,
					"chunkIndex": originalChunkIdx + 1,
					"attempt":    attempt,
					"error":      err.Error(),
				})

				time.Sleep(500 * time.Millisecond)
			}

			logger.Error("[RCA] Chunk processing failed after all retries", map[string]interface{}{
				"jobID":      jobID,
				"chunkIndex": originalChunkIdx + 1,
				"error":      err.Error(),
			})

			errsMutex.Lock()
			errs[idx] = fmt.Errorf("chunk %d analysis failed after 3 retries: %w", originalChunkIdx+1, err)
			errsMutex.Unlock()
			cancel()
		}(i, chunks[i], i) // Pass the original chunk index
	}

	logger.Info("[RCA] Waiting for all chunks to complete", map[string]interface{}{"jobID": jobID})
	wg.Wait()

	// Check for errors and aggregate results
	var partialResults []*LogAnalysisResponse
	for i, res := range results {
		if errs[i] != nil {
			// Use the original chunk index for failed chunk reporting
			*failedChunk = i + 1
			logger.Error("[RCA] Chunk analysis failed after retries", map[string]interface{}{
				"jobID": jobID,
				"chunk": i + 1,
				"error": errs[i].Error(),
			})
			return nil, errs[i]
		}
		partialResults = append(partialResults, res)
	}

	logger.Info("[RCA] All chunks processed successfully", map[string]interface{}{
		"jobID":          jobID,
		"totalChunks":    *totalChunks,
		"partialResults": len(partialResults),
	})

	return partialResults, nil
}

// analyzeChunkWithEnhancedContext analyzes a chunk with learning insights and feedback context
func (js *JobService) analyzeChunkWithEnhancedContext(ctx context.Context, logFile *models.LogFile, entries []models.LogEntry, jobID uint, learningInsights *LearningInsights, feedbackContext string, timeout int, model string) (*LogAnalysisResponse, error) {
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

	// Get user's LLM endpoint
	var user models.User
	if err := js.db.First(&user, logFile.UploadedBy).Error; err != nil {
		return nil, fmt.Errorf("failed to get user configuration: %w", err)
	}

	if user.LLMEndpoint == nil || *user.LLMEndpoint == "" {
		return nil, fmt.Errorf("user has no LLM endpoint configured")
	}
	fmt.Println("[LLM] Starting LLM call with prompt", prompt)
	// Call LLM with the enhanced prompt and timeout using user's endpoint
	response, err := js.llmService.callLLMWithEndpointAndTimeout(ctx, prompt, *user.LLMEndpoint, &logFile.ID, &jobID, "rca_analysis_with_feedback", timeout, model)
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
func (js *JobService) aggregatePartialAnalysesWithRaw(logFile *models.LogFile, partials []*LogAnalysisResponse, timeout int) (*LogAnalysisResponse, string, error) {
	logger.Info("[RCA] Starting aggregation of partial analyses", map[string]interface{}{
		"logFileID": logFile.ID,
		"filename":  logFile.Filename,
		"partials":  len(partials),
		"timeout":   timeout,
	})

	if len(partials) == 0 {
		logger.Error("[RCA] No partial analyses to aggregate", map[string]interface{}{"logFileID": logFile.ID})
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
		logger.Info("[RCA] All chunks found no errors, returning consolidated no-errors analysis", map[string]interface{}{
			"logFileID": logFile.ID,
			"partials":  len(partials),
		})
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

	logger.Info("[RCA] Preparing aggregation prompt", map[string]interface{}{
		"logFileID": logFile.ID,
		"partials":  len(partials),
	})

	// Prepare aggregation prompt using the constant
	summaryParts := []string{}
	for i, p := range partials {
		summaryParts = append(summaryParts, fmt.Sprintf("Chunk %d: %s", i+1, p.Summary))
	}

	prompt := js.llmService.CreateAggregationPrompt(logFile.Filename, summaryParts)
	logger.Info("[RCA] Aggregation prompt created", map[string]interface{}{
		"logFileID":    logFile.ID,
		"promptLength": len(prompt),
		"summaryParts": len(summaryParts),
	})

	// Get user's LLM endpoint for aggregation
	var user models.User
	if err := js.db.First(&user, logFile.UploadedBy).Error; err != nil {
		logger.Error("[RCA] Failed to get user configuration for aggregation", map[string]interface{}{
			"logFileID": logFile.ID,
			"userID":    logFile.UploadedBy,
			"error":     err,
		})
		return nil, "", fmt.Errorf("failed to get user configuration for aggregation: %w", err)
	}

	if user.LLMEndpoint == nil || *user.LLMEndpoint == "" {
		logger.Error("[RCA] User has no LLM endpoint configured for aggregation", map[string]interface{}{
			"logFileID": logFile.ID,
			"userID":    user.ID,
		})
		return nil, "", fmt.Errorf("user has no LLM endpoint configured for aggregation")
	}

	logger.Info("[RCA] Calling LLM for aggregation", map[string]interface{}{
		"logFileID": logFile.ID,
		"endpoint":  *user.LLMEndpoint,
		"timeout":   timeout,
	})

	// Call LLM for aggregation using user's endpoint with timeout
	response, err := js.llmService.callLLMWithEndpointAndTimeout(context.Background(), prompt, *user.LLMEndpoint, &logFile.ID, nil, "rca_aggregation", timeout, "")
	if err != nil {
		logger.Error("[RCA] LLM aggregation failed", map[string]interface{}{
			"logFileID": logFile.ID,
			"endpoint":  *user.LLMEndpoint,
			"error":     err,
		})
		return nil, "", fmt.Errorf("LLM aggregation failed: %w", err)
	}

	logger.Info("[RCA] LLM aggregation response received", map[string]interface{}{
		"logFileID":      logFile.ID,
		"responseLength": len(response),
	})

	// Clean and parse the response
	logger.Info("[RCA] Cleaning and parsing LLM response", map[string]interface{}{"logFileID": logFile.ID})
	cleanResponse := js.extractJSONFromResponse(response)
	var aggregated LogAnalysisResponse
	if err := json.Unmarshal([]byte(cleanResponse), &aggregated); err != nil {
		logger.Error("[RCA] LLM aggregation response parsing failed", map[string]interface{}{
			"logFileID": logFile.ID,
			"error":     err,
		})
		logger.Info("[RCA] Raw LLM response", map[string]interface{}{"response": response})
		logger.Info("[RCA] Cleaned response", map[string]interface{}{"response": cleanResponse})

		// Try alternative parsing method
		logger.Info("[RCA] Trying alternative parsing method", map[string]interface{}{"logFileID": logFile.ID})
		alternativeAnalysis, err := js.parseAlternativeLLMResponse(cleanResponse, partials)
		if err != nil {
			logger.Error("[RCA] Alternative parsing also failed", map[string]interface{}{
				"logFileID": logFile.ID,
				"error":     err,
			})
			return nil, response, fmt.Errorf("failed to parse LLM aggregation response: %w", err)
		}

		if alternativeAnalysis.Summary == "" || alternativeAnalysis.RootCause == "" {
			logger.Error("[RCA] Alternative parsing returned incomplete analysis", map[string]interface{}{
				"logFileID": logFile.ID,
			})
			return nil, response, fmt.Errorf("LLM aggregation returned incomplete analysis")
		}

		logger.Info("[RCA] Alternative parsing successful", map[string]interface{}{
			"logFileID": logFile.ID,
			"summary":   alternativeAnalysis.Summary,
		})

		return alternativeAnalysis, response, nil
	}

	if aggregated.Summary == "" || aggregated.RootCause == "" {
		logger.Error("[RCA] Standard parsing returned incomplete analysis", map[string]interface{}{
			"logFileID": logFile.ID,
		})
		return nil, response, fmt.Errorf("LLM aggregation returned incomplete analysis")
	}

	logger.Info("[RCA] Aggregation completed successfully", map[string]interface{}{
		"logFileID": logFile.ID,
		"summary":   aggregated.Summary,
		"severity":  aggregated.Severity,
		"rootCause": aggregated.RootCause,
	})

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
		updates["current_chunk"] = 0 // Reset current chunk when job completes or fails
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

// GetJobsByLogFile returns all jobs for a specific log file
func (js *JobService) GetJobsByLogFile(logFileID uint) ([]models.Job, error) {
	var jobs []models.Job
	if err := js.db.Where("log_file_id = ?", logFileID).Order("created_at DESC").Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

// CancelJob cancels a running job
func (js *JobService) CancelJob(jobID uint) error {
	js.jobMutex.Lock()
	defer js.jobMutex.Unlock()

	// Check if job is active
	cancelChan, exists := js.activeJobs[jobID]
	if !exists {
		return fmt.Errorf("job %d is not active or already completed", jobID)
	}

	// Send cancellation signal
	close(cancelChan)

	// Update job status to cancelled
	now := time.Now()
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":        "cancelled",
		"error":         "Job cancelled by user",
		"completed_at":  &now,
		"current_chunk": 0, // Reset current chunk
	}).Error; err != nil {
		logger.Error("Failed to update job status to cancelled", map[string]interface{}{"jobID": jobID, "error": err})
		return err
	}

	// Update log file status
	var job models.Job
	if err := js.db.First(&job, jobID).Error; err == nil {
		js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "cancelled")
	}

	// Remove from active jobs
	delete(js.activeJobs, jobID)

	logger.Info("Job cancelled successfully", map[string]interface{}{"jobID": jobID})
	return nil
}

// IsJobActive checks if a job is currently running
func (js *JobService) IsJobActive(jobID uint) bool {
	js.jobMutex.RLock()
	defer js.jobMutex.RUnlock()
	_, exists := js.activeJobs[jobID]
	return exists
}

// registerJob registers a job as active and returns its cancellation channel
func (js *JobService) registerJob(jobID uint) chan struct{} {
	js.jobMutex.Lock()
	defer js.jobMutex.Unlock()

	cancelChan := make(chan struct{})
	js.activeJobs[jobID] = cancelChan
	return cancelChan
}

// unregisterJob removes a job from active tracking
func (js *JobService) unregisterJob(jobID uint) {
	js.jobMutex.Lock()
	defer js.jobMutex.Unlock()
	delete(js.activeJobs, jobID)
}
