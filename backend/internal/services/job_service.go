package services

import (
	"fmt"
	"log"
	"strings"
	"time"

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
		log.Printf("Failed to update job status to running: %v", err)
		return
	}

	// Get job details
	var job models.Job
	if err := js.db.Preload("LogFile").First(&job, jobID).Error; err != nil {
		log.Printf("Failed to get job details: %v", err)
		js.updateJobStatus(jobID, models.JobStatusFailed, "Failed to get job details", nil)
		return
	}

	// Update progress
	js.updateJobProgress(jobID, 20)

	// Check LLM health before starting analysis
	if err := js.llmService.CheckLLMHealth(); err != nil {
		log.Printf("LLM health check failed: %v", err)
		js.updateJobStatus(jobID, models.JobStatusFailed, fmt.Sprintf("LLM service unavailable: %v", err), nil)
		return
	}

	// Perform RCA analysis in chunks with error tracking
	var failedChunk int = -1
	var totalChunks int
	partials, err := js.performRCAAnalysisWithErrorTrackingAndChunkCount(job.LogFile, &failedChunk, &totalChunks, jobID)
	if err != nil {
		log.Printf("RCA analysis failed: %v", err)
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
		log.Printf("Failed to store partial RCA results: %v", err)
	}

	// Update progress after each chunk
	for i := range partials {
		progress := 20 + int(float64(i+1)/float64(len(partials))*60) // 20-80%
		js.updateJobProgress(jobID, progress)
	}

	// (Aggregation step)
	js.updateJobProgress(jobID, 85)

	// Aggregate partials into a final RCA report using LLM
	aggregated, err := js.aggregatePartialAnalyses(job.LogFile, partials)
	if err != nil {
		log.Printf("RCA aggregation failed: %v", err)
		js.updateJobStatus(jobID, models.JobStatusFailed, err.Error(), nil)
		// Also mark the log file as failed
		js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "failed")
		return
	}

	// Store final RCA in job.Result
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
		log.Printf("Failed to update job completion: %v", err)
		return
	}

	// Update log file status
	if err := js.db.Model(&models.LogFile{}).Where("id = ?", job.LogFileID).Update("rca_analysis_status", "completed").Error; err != nil {
		log.Printf("Failed to update log file status: %v", err)
	}

	log.Printf("RCA analysis completed for job %d", jobID)
}

// performRCAAnalysisWithErrorTrackingAndChunkCount is like performRCAAnalysisWithErrorTracking but also returns totalChunks and logs each chunk
func (js *JobService) performRCAAnalysisWithErrorTrackingAndChunkCount(logFile *models.LogFile, failedChunk *int, totalChunks *int, jobID uint) ([]*LogAnalysisResponse, error) {
	var entries []models.LogEntry
	if err := js.db.Where("log_file_id = ?", logFile.ID).Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to load log entries: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no log entries found for analysis")
	}
	chunkSize := 100
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
		log.Printf("[RCA] Processing chunk %d/%d for job %d", currentChunk, *totalChunks, jobID)
		log.Printf("[RCA] Analyzing chunk %d/%d (entries %d-%d) for job %d...", currentChunk, *totalChunks, i*chunkSize+1, i*chunkSize+len(chunk), jobID)
		analysis, err := js.llmService.AnalyzeLogsWithAI(logFile, chunk)
		if err != nil {
			log.Printf("[RCA] Chunk %d failed: %v", currentChunk, err)
			*failedChunk = currentChunk
			return nil, fmt.Errorf("LLM analysis failed for chunk %d: %w", currentChunk, err)
		}
		log.Printf("[RCA] Chunk %d succeeded", currentChunk)
		partialResults = append(partialResults, analysis)
	}
	return partialResults, nil
}

// aggregatePartialAnalyses aggregates chunk results into a final RCA report using the LLM
func (js *JobService) aggregatePartialAnalyses(logFile *models.LogFile, partials []*LogAnalysisResponse) (*LogAnalysisResponse, error) {
	if len(partials) == 0 {
		return nil, fmt.Errorf("no partial analyses to aggregate")
	}

	// Prepare aggregation prompt
	summaryParts := []string{}
	for i, p := range partials {
		summaryParts = append(summaryParts, fmt.Sprintf("Chunk %d: %s", i+1, p.Summary))
	}
	prompt := fmt.Sprintf(`You are an expert SRE. Given the following partial RCA analyses for log file '%s', produce a single, comprehensive root cause analysis report.\n\n%s\n\nOutput valid JSON in the same format as before.`, logFile.Filename, strings.Join(summaryParts, "\n"))

	response, err := js.llmService.callLLM(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM aggregation failed: %w", err)
	}

	aggregated, err := js.llmService.parseDetailedLLMResponse(response)
	if err != nil {
		log.Printf("LLM aggregation response parsing failed: %v", err)
		log.Printf("Raw LLM response: %q", response)
		return nil, fmt.Errorf("failed to parse LLM aggregation response: %w", err)
	}

	if aggregated.Summary == "" || aggregated.RootCause == "" {
		return nil, fmt.Errorf("LLM aggregation returned incomplete analysis")
	}

	return aggregated, nil
}

// updateJobProgress updates the job progress
func (js *JobService) updateJobProgress(jobID uint, progress int) {
	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Update("progress", progress).Error; err != nil {
		log.Printf("Failed to update job progress: %v", err)
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
		log.Printf("Failed to update job status: %v", err)
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
