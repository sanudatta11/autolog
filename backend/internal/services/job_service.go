package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"gorm.io/gorm"
)

type JobService struct {
	db         *gorm.DB
	llmService *LLMService
}

func NewJobService(db *gorm.DB, llmService *LLMService) *JobService {
	return &JobService{
		db:         db,
		llmService: llmService,
	}
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
		analysis, err := js.llmService.AnalyzeLogsWithAI(logFile, chunk, &jobID)
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
	prompt := fmt.Sprintf(`You are an expert SRE. Given the following partial RCA analyses for log file '%s', produce a single, comprehensive root cause analysis report.\n\n%s\n\nIMPORTANT: Return ONLY valid JSON in the exact format specified below. Do not include any explanatory text, introductions, or markdown formatting.\n\nRequired JSON format: ...`, logFile.Filename, strings.Join(summaryParts, "\n"))

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
		return nil, response, fmt.Errorf("failed to parse LLM aggregation response: %w", err)
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
