package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	helpers "github.com/autolog/backend/internal/helpers"
	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"gorm.io/gorm"
)

// JobRequest represents a job request
type JobRequest struct {
	JobID uint
	Type  string
}

type JobService struct {
	db              *gorm.DB
	llmService      *LLMService
	learningService *LearningService
	jobQueue        chan JobRequest
	workerCount     int
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewJobService creates a new job service
func NewJobService(db *gorm.DB, llmService *LLMService) *JobService {
	learningService := NewLearningService(db, llmService)

	js := &JobService{
		db:              db,
		llmService:      llmService,
		learningService: learningService,
		jobQueue:        make(chan JobRequest, 100),
		workerCount:     2,
		stopChan:        make(chan struct{}),
	}

	// Start workers
	for i := 0; i < js.workerCount; i++ {
		js.wg.Add(1)
		go js.worker(i)
	}

	return js
}

// worker processes jobs from the queue
func (js *JobService) worker(id int) {
	defer js.wg.Done()

	for {
		select {
		case jobReq := <-js.jobQueue:
			logger.Info("Worker processing job", map[string]interface{}{
				"workerID": id,
				"jobID":    jobReq.JobID,
				"type":     jobReq.Type,
			})

			switch jobReq.Type {
			case "rca_analysis":
				js.ProcessRCAAnalysisJob(jobReq.JobID)
			default:
				logger.Error("Unknown job type", map[string]interface{}{
					"jobID": jobReq.JobID,
					"type":  jobReq.Type,
				})
			}

		case <-js.stopChan:
			logger.Info("Worker stopping", map[string]interface{}{"workerID": id})
			return
		}
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
	job := &models.Job{
		Type:      "rca_analysis",
		LogFileID: logFileID,
		Status:    models.JobStatusPending,
		Progress:  0,
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

	return job, nil
}

// ProcessRCAAnalysisJob processes an RCA analysis job in the background
func (js *JobService) ProcessRCAAnalysisJob(jobID uint) {
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
		return
	}

	// Check if RCA is possible for this log file
	if job.LogFile != nil && !job.LogFile.IsRCAPossible {
		logger.Info("RCA not possible for this log file, completing job with no RCA needed", map[string]interface{}{"jobID": jobID, "logFileID": job.LogFileID, "reason": job.LogFile.RCANotPossibleReason})
		finalResult := map[string]interface{}{
			"final": map[string]interface{}{
				"summary":           job.LogFile.RCANotPossibleReason,
				"severity":          "none",
				"rootCause":         job.LogFile.RCANotPossibleReason,
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
		if err := js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "completed").Error; err != nil {
			logger.Error("Failed to update log file status for no RCA needed", map[string]interface{}{"jobID": job.LogFileID, "error": err})
		}
		logger.Info("RCA job completed with no RCA needed", map[string]interface{}{"jobID": jobID})
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

	// Perform RCA analysis in chunks with error tracking
	var failedChunk int = -1
	var totalChunks int
	partials, err := js.performRCAAnalysisWithErrorTrackingAndChunkCount(job.LogFile, &failedChunk, &totalChunks, jobID)
	if err != nil {
		logger.Error("RCA analysis failed", map[string]interface{}{"jobID": jobID, "error": err})
		failMsg := err.Error()
		if failedChunk > 0 {
			failMsg = fmt.Sprintf("Chunk %d failed: %s", failedChunk, failMsg)
		}
		// Save failedChunk and totalChunks in job
		js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
			"failed_chunk": failedChunk,
			"total_chunks": totalChunks,
		})
		js.updateJobStatus(jobID, models.JobStatusFailed, failMsg, map[string]interface{}{
			"failedChunk": failedChunk,
			"totalChunks": totalChunks,
		})
		// Also mark the log file as failed
		js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "failed")
		return
	}
	// Save totalChunks in job
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
	}

	// (Aggregation step)
	js.updateJobProgress(jobID, 85)

	// Aggregate partials into a final RCA report using LLM
	var rawLLMResponse string
	aggregated, rawResp, err := js.aggregatePartialAnalysesWithRaw(job.LogFile, partials)
	if err != nil {
		logger.Error("RCA aggregation failed", map[string]interface{}{"jobID": jobID, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, err.Error(), nil)
		// Also mark the log file as failed
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

	// --- Save LogAnalysis and LogAnalysisMemory ---
	if aggregated != nil && job.LogFile != nil {
		// Save LogAnalysis
		analysis := &models.LogAnalysis{
			LogFileID:    job.LogFile.ID,
			Summary:      aggregated.Summary,
			Severity:     aggregated.Severity,
			ErrorCount:   aggregated.CriticalErrors + aggregated.NonCriticalErrors,
			WarningCount: job.LogFile.WarningCount,
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
			logger.Error("Failed to save LogAnalysis after RCA", map[string]interface{}{"jobID": jobID, "error": err})
		}

		// Save LogAnalysisMemory (with embedding)
		var embedding models.JSONB = nil
		if js.llmService != nil {
			prompt := aggregated.Summary + "\n" + aggregated.RootCause
			embed, err := js.llmService.GenerateEmbedding(prompt)
			if err != nil {
				logger.Warn("Failed to generate embedding for LogAnalysisMemory", map[string]interface{}{"jobID": jobID, "error": err})
			} else if embed != nil {
				embedding = models.JSONB{"embedding": embed}
			}
		}
		memory := &models.LogAnalysisMemory{
			LogFileID: &job.LogFile.ID,
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
			logger.Error("Failed to save LogAnalysisMemory after RCA", map[string]interface{}{"jobID": jobID, "error": err})
		}

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
	}

	logger.Info("RCA analysis completed for job", map[string]interface{}{"jobID": jobID})
}

func (js *JobService) CreateRCAAnalysisJobWithOptions(logFileID uint, timeout int, chunking bool) (*models.Job, error) {
	job := &models.Job{
		Type:      "rca_analysis",
		LogFileID: logFileID,
		Status:    models.JobStatusPending,
		Progress:  0,
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
	return job, nil
}

func (js *JobService) ProcessRCAAnalysisJobWithOptions(jobID uint, timeout int, chunking bool) {
	now := time.Now()
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":     models.JobStatusRunning,
		"started_at": &now,
		"progress":   10,
	}).Error; err != nil {
		logger.Error("Failed to update job status to running", map[string]interface{}{"jobID": jobID, "error": err})
		return
	}
	var job models.Job
	if err := js.db.Preload("LogFile").First(&job, jobID).Error; err != nil {
		logger.Error("Failed to get job details", map[string]interface{}{"jobID": jobID, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, "Failed to get job details", nil)
		return
	}
	js.updateJobProgress(jobID, 20)
	if err := js.llmService.CheckLLMHealth(); err != nil {
		logger.Error("LLM health check failed", map[string]interface{}{"jobID": jobID, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, fmt.Sprintf("LLM service unavailable: %v", err), nil)
		return
	}
	var failedChunk int = -1
	var totalChunks int
	var partials []*LogAnalysisResponse
	var err error
	if chunking {
		partials, err = js.performRCAAnalysisWithErrorTrackingAndChunkCountWithTimeout(job.LogFile, &failedChunk, &totalChunks, jobID, timeout)
	} else {
		partials, err = js.performRCAAnalysisNoChunking(job.LogFile, jobID, timeout)
		totalChunks = 1
	}
	if err != nil {
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
	result := map[string]interface{}{
		"partials": partials,
	}
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("result", result).Error; err != nil {
		logger.Error("Failed to store partial RCA results", map[string]interface{}{"jobID": jobID, "error": err})
	}
	for i := range partials {
		progress := 20 + int(float64(i+1)/float64(len(partials))*60)
		js.updateJobProgress(jobID, progress)
	}
	js.updateJobProgress(jobID, 85)
	aggregated, err := js.aggregatePartialAnalyses(job.LogFile, partials)
	if err != nil {
		logger.Error("RCA aggregation failed", map[string]interface{}{"jobID": jobID, "error": err})
		js.updateJobStatus(jobID, models.JobStatusFailed, err.Error(), nil)
		js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "failed")
		return
	}
	finalResult := map[string]interface{}{
		"partials": partials,
		"final":    aggregated,
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
	if err := js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "completed").Error; err != nil {
		logger.Error("Failed to update log file status", map[string]interface{}{"jobID": job.LogFileID, "error": err})
	}
	logger.Info("RCA analysis completed for job", map[string]interface{}{"jobID": jobID})
}

// ProcessRCAAnalysisJobWithShutdown processes an RCA analysis job with shutdown support
func (js *JobService) ProcessRCAAnalysisJobWithShutdown(jobID uint, stopChan <-chan struct{}) {
	completed := false
	defer func() {
		if !completed {
			js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
				"status": models.JobStatusFailed,
				"error":  "Job failed due to shutdown or unexpected exit",
			})
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
		return
	}

	// Check for shutdown before starting
	select {
	case <-stopChan:
		return
	default:
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
	partials, err := js.performRCAAnalysisWithErrorTrackingAndChunkCount(job.LogFile, &failedChunk, &totalChunks, jobID)
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

	completed = true
	logger.Info("RCA analysis completed for job", map[string]interface{}{"jobID": jobID})
}

func (js *JobService) performRCAAnalysisWithErrorTrackingAndChunkCount(logFile *models.LogFile, failedChunk *int, totalChunks *int, jobID uint) ([]*LogAnalysisResponse, error) {
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
	var partialResults []*LogAnalysisResponse
	for i, chunk := range chunks {
		currentChunk := i + 1
		// Update currentChunk in the job
		js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("current_chunk", currentChunk)
		logger.Info("[RCA] Processing chunk", map[string]interface{}{"jobID": jobID, "chunk": currentChunk, "totalChunks": *totalChunks})
		logger.Info("[RCA] Analyzing chunk", map[string]interface{}{"jobID": jobID, "chunk": currentChunk, "startEntry": i*chunkSize + 1, "endEntry": i*chunkSize + len(chunk)})

		// Use learning insights for better analysis
		analysis, err := js.analyzeChunkWithLearning(logFile, chunk, jobID, learningInsights)
		if err != nil {
			logger.Error("[RCA] Chunk failed", map[string]interface{}{"jobID": jobID, "chunk": currentChunk, "error": err})
			*failedChunk = currentChunk
			return nil, fmt.Errorf("LLM analysis failed for chunk %d: %w", currentChunk, err)
		}
		logger.Info("[RCA] Chunk succeeded", map[string]interface{}{"jobID": jobID, "chunk": currentChunk})
		partialResults = append(partialResults, analysis)
	}
	return partialResults, nil
}

// analyzeChunkWithLearning analyzes a chunk with learning insights
func (js *JobService) analyzeChunkWithLearning(logFile *models.LogFile, chunk []models.LogEntry, jobID uint, learningInsights *LearningInsights) (*LogAnalysisResponse, error) {
	// Filter error entries from this chunk
	errorEntries := js.filterErrorEntries(chunk)

	if len(errorEntries) == 0 {
		// No errors in this chunk, return basic analysis
		return js.llmService.generateNoErrorsAnalysis(logFile), nil
	}

	// Create analysis request
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

	// Use learning-enhanced prompt if we have insights
	var prompt string
	if learningInsights != nil && (len(learningInsights.SimilarIncidents) > 0 || len(learningInsights.PatternMatches) > 0) {
		prompt = js.llmService.CreateDetailedErrorAnalysisPromptWithLearning(request, errorEntries, learningInsights)
		logger.Debug("[RCA] Using learning-enhanced prompt", map[string]interface{}{
			"jobID":            jobID,
			"similarIncidents": len(learningInsights.SimilarIncidents),
			"patternMatches":   len(learningInsights.PatternMatches),
		})
	} else {
		prompt = js.llmService.CreateDetailedErrorAnalysisPrompt(request, errorEntries, "")
	}

	// Call LLM with the enhanced prompt
	response, err := js.llmService.callLLMWithContext(prompt, &logFile.ID, &jobID, "rca_analysis_with_learning")
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

func (js *JobService) performRCAAnalysisWithErrorTrackingAndChunkCountWithTimeout(logFile *models.LogFile, failedChunk *int, totalChunks *int, jobID uint, timeout int) ([]*LogAnalysisResponse, error) {
	var entries []models.LogEntry
	if err := js.db.Where("log_file_id = ?", logFile.ID).Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to load log entries: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no log entries found for analysis")
	}
	chunkSize := 25
	var chunks [][]models.LogEntry
	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}
		chunks = append(chunks, entries[i:end])
	}
	*totalChunks = len(chunks)
	var partialResults []*LogAnalysisResponse
	for i, chunk := range chunks {
		currentChunk := i + 1
		js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("current_chunk", currentChunk)
		logger.Info("[RCA] Processing chunk", map[string]interface{}{"jobID": jobID, "chunk": currentChunk, "totalChunks": *totalChunks})
		logger.Info("[RCA] Analyzing chunk", map[string]interface{}{"jobID": jobID, "chunk": currentChunk, "startEntry": i*chunkSize + 1, "endEntry": i*chunkSize + len(chunk)})
		logger.Info("[RCA] Starting LLM request for chunk", map[string]interface{}{"jobID": jobID, "chunk": currentChunk})
		analysis, err := js.llmService.AnalyzeLogsWithAIWithTimeout(logFile, chunk, timeout, &jobID)
		if err != nil {
			logger.Error("[RCA] Chunk failed", map[string]interface{}{"jobID": jobID, "chunk": currentChunk, "error": err})
			*failedChunk = currentChunk
			return nil, fmt.Errorf("LLM analysis failed for chunk %d: %w", currentChunk, err)
		}
		logger.Info("[RCA] Chunk succeeded", map[string]interface{}{"jobID": jobID, "chunk": currentChunk})
		partialResults = append(partialResults, analysis)
	}
	return partialResults, nil
}

func (js *JobService) performRCAAnalysisNoChunking(logFile *models.LogFile, jobID uint, timeout int) ([]*LogAnalysisResponse, error) {
	var entries []models.LogEntry
	if err := js.db.Where("log_file_id = ?", logFile.ID).Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to load log entries: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no log entries found for analysis")
	}
	analysis, err := js.llmService.AnalyzeLogsWithAIWithTimeout(logFile, entries, timeout, &jobID)
	if err != nil {
		return nil, err
	}
	return []*LogAnalysisResponse{analysis}, nil
}

// aggregatePartialAnalyses aggregates chunk results into a final RCA report using the LLM
func (js *JobService) aggregatePartialAnalyses(logFile *models.LogFile, partials []*LogAnalysisResponse) (*LogAnalysisResponse, error) {
	if len(partials) == 0 {
		return nil, fmt.Errorf("no partial analyses to aggregate")
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
		return &LogAnalysisResponse{
			Summary:           fmt.Sprintf("Log file '%s' contains no ERROR or FATAL entries across all analyzed chunks. System appears to be functioning normally.", logFile.Filename),
			Severity:          "low",
			RootCause:         "No errors detected in any analyzed log chunks",
			Recommendations:   []string{"Continue monitoring for any new errors", "Review INFO and WARNING logs for potential issues", "System is operating within normal parameters"},
			ErrorAnalysis:     []DetailedErrorAnalysis{},
			CriticalErrors:    0,
			NonCriticalErrors: 0,
		}, nil
	}

	// Prepare aggregation prompt
	summaryParts := []string{}
	for i, p := range partials {
		summaryParts = append(summaryParts, fmt.Sprintf("Chunk %d: %s", i+1, p.Summary))
	}
	prompt := fmt.Sprintf(`You are an expert SRE. Given the following partial RCA analyses for log file '%s', produce a single, comprehensive root cause analysis report.

%s

IMPORTANT: Return ONLY valid JSON in the exact format specified below. Do not include any explanatory text, introductions, or markdown formatting.

Required JSON format:
{
  "summary": "A concise summary focusing on the most critical errors and their impact (2-3 sentences)",
  "severity": "low|medium|high|critical",
  "rootCause": "The primary root cause that explains most of the errors, with step-by-step reasoning",
  "recommendations": ["specific_action1", "specific_action2", "specific_action3"],
  "errorAnalysis": [
    {
      "errorPattern": "The pattern or category of this error",
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

Return ONLY the JSON object, nothing else.`, logFile.Filename, strings.Join(summaryParts, "\n"))

	logger.Info("[RCA] Starting final LLM aggregation request...", nil)
	response, err := js.llmService.callLLM(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM aggregation failed: %w", err)
	}

	// Try to extract JSON from the response if it contains explanatory text
	cleanResponse := js.extractJSONFromResponse(response)

	aggregated, err := js.llmService.parseDetailedLLMResponse(cleanResponse)
	if err != nil {
		logger.Error("[RCA] LLM aggregation response parsing failed", map[string]interface{}{"error": err})
		logger.Info("[RCA] Raw LLM response", map[string]interface{}{"response": response})
		logger.Info("[RCA] Cleaned response", map[string]interface{}{"response": cleanResponse})
		return nil, fmt.Errorf("failed to parse LLM aggregation response: %w", err)
	}

	if aggregated.Summary == "" || aggregated.RootCause == "" {
		return nil, fmt.Errorf("LLM aggregation returned incomplete analysis")
	}

	return aggregated, nil
}

// New helper to aggregate and return raw LLM response
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

	// Prepare aggregation prompt
	summaryParts := []string{}
	for i, p := range partials {
		summaryParts = append(summaryParts, fmt.Sprintf("Chunk %d: %s", i+1, p.Summary))
	}
	prompt := fmt.Sprintf(`You are an expert SRE performing Root Cause Analysis (RCA) aggregation. 

Given the following partial RCA analyses for log file '%s', produce a single, comprehensive root cause analysis report.

PARTIAL ANALYSES:
%s

CRITICAL REQUIREMENT: You must return ONLY a valid JSON object in the exact format specified below. Do not include any explanatory text, introductions, markdown formatting, or notes.

If you cannot determine a single cumulative RCA, create a merged RCA by combining all partial analyses as a fallback. Do not return an empty or incomplete object.

REQUIRED JSON FORMAT (return exactly this structure):
{
  "summary": "A concise summary focusing on the most critical errors and their impact (2-3 sentences)",
  "severity": "low|medium|high|critical",
  "rootCause": "The primary root cause that explains most of the errors, with step-by-step reasoning",
  "recommendations": ["specific_action1", "specific_action2", "specific_action3"],
  "errorAnalysis": [
    {
      "errorPattern": "The pattern or category of this error",
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

IMPORTANT: Start your response with { and end with }. Do not include any text before or after the JSON object.`, logFile.Filename, strings.Join(summaryParts, "\n"))

	logger.Info("[RCA] Starting final LLM aggregation request...", nil)
	response, err := js.llmService.callLLM(prompt)
	if err != nil {
		return nil, response, fmt.Errorf("LLM aggregation failed: %w", err)
	}

	// Try to extract JSON from the response if it contains explanatory text
	cleanResponse := js.extractJSONFromResponse(response)
	aggregated, err := js.llmService.parseDetailedLLMResponse(cleanResponse)
	if err != nil {
		logger.Error("[RCA] LLM aggregation response parsing failed", map[string]interface{}{"error": err})
		logger.Info("[RCA] Raw LLM response", map[string]interface{}{"response": response})
		logger.Info("[RCA] Cleaned response", map[string]interface{}{"response": cleanResponse})

		// Try to parse the alternative format (rootCauses array)
		alternativeAnalysis, altErr := js.parseAlternativeLLMResponse(cleanResponse, partials)
		if altErr != nil {
			logger.Error("[RCA] Alternative parsing also failed", map[string]interface{}{"error": altErr})
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

	return aggregated, response, nil
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

	// Count errors from partials
	for _, partial := range partials {
		criticalErrors += partial.CriticalErrors
		nonCriticalErrors += partial.NonCriticalErrors
		errorAnalysis = append(errorAnalysis, partial.ErrorAnalysis...)
	}

	// Extract causes from the rootCauses format
	for _, rootCause := range rootCausesData.RootCauses {
		for _, cause := range rootCause.Causes {
			allCauses = append(allCauses, fmt.Sprintf("%s: %s", cause.Type, cause.Description))
		}
	}

	// Create a consolidated analysis
	summary := "System errors detected across multiple log chunks requiring attention."
	if len(allCauses) > 0 {
		summary = fmt.Sprintf("Multiple issues detected: %s", strings.Join(allCauses[:helpers.Min(3, len(allCauses))], "; "))
	}

	rootCause := "Multiple system issues identified across different log chunks."
	if len(allCauses) > 0 {
		rootCause = fmt.Sprintf("Primary issues: %s", strings.Join(allCauses[:helpers.Min(2, len(allCauses))], "; "))
	}

	// Determine severity based on error counts
	severity := "medium"
	if criticalErrors > 5 {
		severity = "critical"
	} else if criticalErrors > 2 {
		severity = "high"
	} else if criticalErrors == 0 && nonCriticalErrors == 0 {
		severity = "low"
	}

	// Generate recommendations based on the causes
	recommendations := []string{
		"Review and address the identified system issues",
		"Implement monitoring for the affected components",
		"Consider implementing automated recovery mechanisms",
	}

	return &LogAnalysisResponse{
		Summary:           summary,
		Severity:          severity,
		RootCause:         rootCause,
		Recommendations:   recommendations,
		ErrorAnalysis:     errorAnalysis,
		CriticalErrors:    criticalErrors,
		NonCriticalErrors: nonCriticalErrors,
	}, nil
}

// updateJobProgress updates the job progress
func (js *JobService) updateJobProgress(jobID uint, progress int) {
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("progress", progress).Error; err != nil {
		logger.Error("Failed to update job progress", map[string]interface{}{"jobID": jobID, "error": err})
	}
}

// updateJobStatus updates the job status
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
		logger.Error("Failed to update job status", map[string]interface{}{"jobID": jobID, "error": err})
	}
}

// GetJobStatus returns the current status of a job
func (js *JobService) GetJobStatus(jobID uint) (*models.Job, error) {
	var job models.Job
	if err := js.db.Preload("LogFile").First(&job, jobID).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// GetJobsByLogFile returns all jobs for a log file
func (js *JobService) GetJobsByLogFile(logFileID uint) ([]models.Job, error) {
	var jobs []models.Job
	if err := js.db.Where("log_file_id = ?", logFileID).Order("created_at DESC").Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}
