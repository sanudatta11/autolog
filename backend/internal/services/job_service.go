package services

import (
	"fmt"
	"log"
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

	// Perform RCA analysis
	analysis, err := js.performRCAAnalysis(job.LogFile)
	if err != nil {
		log.Printf("RCA analysis failed: %v", err)
		js.updateJobStatus(jobID, models.JobStatusFailed, err.Error(), nil)
		return
	}

	// Update progress
	js.updateJobProgress(jobID, 80)

	// Store results
	completedAt := time.Now()
	result := map[string]interface{}{
		"analysis": analysis,
	}

	if err := js.db.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":       models.JobStatusCompleted,
		"progress":     100,
		"result":       result,
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

// performRCAAnalysis performs the actual RCA analysis
func (js *JobService) performRCAAnalysis(logFile models.LogFile) (*LogAnalysisResponse, error) {
	// Load log entries
	var entries []models.LogEntry
	if err := js.db.Where("log_file_id = ?", logFile.ID).Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to load log entries: %w", err)
	}

	// Check if we have too many entries and warn
	if len(entries) > 1000 {
		log.Printf("Warning: Large log file detected with %d entries, analysis may take longer", len(entries))
	}

	// Perform AI analysis with retry logic
	var analysis *LogAnalysisResponse
	var err error

	// Try up to 3 times with exponential backoff
	for attempt := 1; attempt <= 3; attempt++ {
		analysis, err = js.llmService.AnalyzeLogsWithAI(logFile, entries)
		if err == nil {
			break
		}

		log.Printf("RCA analysis attempt %d failed: %v", attempt, err)

		if attempt < 3 {
			// Wait before retry (exponential backoff)
			waitTime := time.Duration(attempt) * 10 * time.Second
			log.Printf("Retrying in %v...", waitTime)
			time.Sleep(waitTime)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("AI analysis failed after 3 attempts: %w", err)
	}

	return analysis, nil
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
