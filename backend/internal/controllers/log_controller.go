package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"github.com/autolog/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const ENABLE_LOG_FILE_FLUSH = false

type LogController struct {
	db              *gorm.DB
	logProcessor    *services.LogProcessor
	llmService      *services.LLMService
	learningService *services.LearningService
	feedbackService *services.FeedbackService
	jobService      *services.JobService
	uploadDir       string
	stopChan        <-chan struct{} // Add stopChan for graceful shutdown
}

func NewLogController(db *gorm.DB, llmService *services.LLMService, stopChan <-chan struct{}) *LogController {
	// Initialize services
	feedbackService := services.NewFeedbackService(db)
	learningService := services.NewLearningService(db, llmService, feedbackService)
	jobService := services.NewJobService(db, llmService, learningService, feedbackService)

	logger.Info("LogController initialized", map[string]interface{}{
		"upload_dir": "uploads/logs",
		"component":  "log_controller",
	})

	return &LogController{
		db:              db,
		logProcessor:    services.NewLogProcessor(db, llmService),
		llmService:      llmService,
		learningService: learningService,
		feedbackService: feedbackService,
		jobService:      jobService,
		uploadDir:       "uploads/logs",
		stopChan:        stopChan,
	}
}

// UploadLogFile handles log file upload
func (lc *LogController) UploadLogFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to upload log file", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Info("Log file upload request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	file, err := c.FormFile("logfile")
	if err != nil {
		logger.WithError(err, "log_controller").Error("No file uploaded")
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	logEntry.Info("File received for upload", map[string]interface{}{
		"filename": file.Filename,
		"size":     file.Size,
	})

	// Validate file extension
	ext := filepath.Ext(file.Filename)
	if ext != ".json" && ext != ".log" && ext != ".txt" {
		logEntry.Warn("Invalid file extension", map[string]interface{}{
			"filename":     file.Filename,
			"extension":    ext,
			"allowed_exts": []string{".json", ".log", ".txt"},
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only JSON, LOG, and TXT files are supported"})
		return
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(lc.uploadDir, 0755); err != nil {
		logger.WithError(err, "log_controller").Error("Failed to create upload directory")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	// Generate unique filename
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s", timestamp, file.Filename)
	filepath := filepath.Join(lc.uploadDir, filename)

	logEntry.Debug("Saving uploaded file", map[string]interface{}{
		"original_filename": file.Filename,
		"stored_filename":   filename,
		"file_path":         filepath,
	})

	// Save file
	if err := c.SaveUploadedFile(file, filepath); err != nil {
		logger.WithError(err, "log_controller").Error("Failed to save uploaded file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Create log file record
	logFile := models.LogFile{
		Filename:   file.Filename,
		Size:       file.Size,
		UploadedBy: userID.(uint),
		Status:     "pending",
	}

	if err := lc.db.Create(&logFile).Error; err != nil {
		// Clean up file if database save fails
		os.Remove(filepath)
		logger.WithError(err, "log_controller").Error("Failed to save log file record to database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save log file record"})
		return
	}

	logEntry.Info("Log file record created", map[string]interface{}{
		"log_file_id": logFile.ID,
		"filename":    logFile.Filename,
		"status":      logFile.Status,
	})

	// Process log file in background
	go func() {
		logEntry.Info("Starting background processing", map[string]interface{}{
			"log_file_id": logFile.ID,
			"file_path":   filepath,
		})
		if err := lc.logProcessor.ProcessLogFileWithShutdown(logFile.ID, filepath, lc.stopChan); err != nil {
			logger.WithError(err, "log_controller").Error("Failed to process log file in background")
			// Update status to failed
			lc.db.Model(&models.LogFile{}).Where("id = ?", logFile.ID).Update("status", "failed")
		} else {
			logEntry.Info("Successfully processed log file in background", map[string]interface{}{
				"log_file_id": logFile.ID,
			})
		}
	}()

	c.JSON(http.StatusCreated, gin.H{
		"message": "Log file uploaded successfully",
		"logFile": logFile,
	})
}

// GetLogFiles returns all log files for the user
func (lc *LogController) GetLogFiles(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get log files", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get log files request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	var logFiles []models.LogFile
	query := lc.db.Where("uploaded_by = ?", userID).Order("created_at DESC")

	// Add pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	query = query.Offset(offset).Limit(limit)

	if err := query.Find(&logFiles).Error; err != nil {
		logger.WithError(err, "log_controller").Error("Failed to fetch log files from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log files"})
		return
	}

	// Get total count
	var total int64
	lc.db.Model(&models.LogFile{}).Where("uploaded_by = ?", userID).Count(&total)

	// Build response with hasReview for each log file
	logFilesWithReview := make([]map[string]interface{}, 0, len(logFiles))
	for _, logFile := range logFiles {
		logFileMap := map[string]interface{}{
			"id":                   logFile.ID,
			"filename":             logFile.Filename,
			"size":                 logFile.Size,
			"uploadedBy":           logFile.UploadedBy,
			"status":               logFile.Status,
			"entryCount":           logFile.EntryCount,
			"errorCount":           logFile.ErrorCount,
			"warningCount":         logFile.WarningCount,
			"processedAt":          logFile.ProcessedAt,
			"rcaAnalysisStatus":    logFile.RCAAnalysisStatus,
			"rcaAnalysisJobId":     logFile.RCAAnalysisJobID,
			"isRCAPossible":        logFile.IsRCAPossible,
			"rcaNotPossibleReason": logFile.RCANotPossibleReason,
			"createdAt":            logFile.CreatedAt,
			"updatedAt":            logFile.UpdatedAt,
		}

		hasReview := false
		// Find latest completed RCA job for this logFile
		var job models.Job
		err := lc.db.Where("log_file_id = ? AND type = ? AND status = ?", logFile.ID, "rca_analysis", "completed").Order("created_at desc").First(&job).Error
		if err == nil && job.ID != 0 {
			// Find LogAnalysisMemory for this logFile (latest)
			var memory models.LogAnalysisMemory
			errMem := lc.db.Where("log_file_id = ?", logFile.ID).Order("created_at desc").First(&memory).Error
			if errMem == nil && memory.ID != 0 {
				// Check for feedback
				var feedback models.LogAnalysisFeedback
				errFb := lc.db.Where("analysis_memory_id = ?", memory.ID).First(&feedback).Error
				hasReview = (errFb == nil && feedback.ID != 0)
			}
		}
		logFileMap["hasReview"] = hasReview
		logFilesWithReview = append(logFilesWithReview, logFileMap)
	}

	logEntry.Debug("Log files retrieved successfully", map[string]interface{}{
		"count": len(logFiles),
		"total": total,
		"page":  page,
		"limit": limit,
	})

	c.JSON(http.StatusOK, gin.H{
		"logFiles": logFilesWithReview,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

// GetLogFile returns a specific log file with its entries
func (lc *LogController) GetLogFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get log file details", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get log file details request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid log file ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	var logFile models.LogFile
	if err := lc.db.Preload("Entries").First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "log_controller").Error("Log file not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			logger.WithError(err, "log_controller").Error("Failed to fetch log file from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	// Check if user owns this log file
	if logFile.UploadedBy != userID.(uint) {
		logEntry.Warn("Access denied for log file", map[string]interface{}{
			"log_file_id": logFile.ID,
			"user_id":     userID.(uint),
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	logEntry.Info("Log file details retrieved successfully", map[string]interface{}{
		"log_file_id": logFile.ID,
		"filename":    logFile.Filename,
		"status":      logFile.Status,
	})

	c.JSON(http.StatusOK, gin.H{"logFile": logFile})
}

// GetRCAJobStatus returns the status of an RCA analysis job
func (lc *LogController) GetRCAJobStatus(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get RCA job status", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get RCA job status request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	jobID, err := strconv.ParseUint(c.Param("jobId"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid job ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	// Get job status
	job, err := lc.jobService.GetJobStatus(uint(jobID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "log_controller").Error("Job not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else {
			logger.WithError(err, "log_controller").Error("Failed to fetch job status from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch job status"})
		}
		return
	}

	// Check if user owns this log file
	if job.LogFile != nil && job.LogFile.UploadedBy != userID.(uint) {
		logEntry.Warn("Access denied for RCA job status", map[string]interface{}{
			"job_id":  job.ID,
			"user_id": userID.(uint),
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	logEntry.Info("RCA job status retrieved successfully", map[string]interface{}{
		"job_id": job.ID,
		"status": job.Status,
	})

	c.JSON(http.StatusOK, gin.H{
		"job":          job,
		"totalChunks":  job.TotalChunks,
		"failedChunk":  job.FailedChunk,
		"currentChunk": job.CurrentChunk,
	})
}

// GetRCAResults returns the RCA analysis results for a log file
func (lc *LogController) GetRCAResults(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get RCA analysis results", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get RCA analysis results request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid log file ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "log_controller").Error("Log file not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			logger.WithError(err, "log_controller").Error("Failed to fetch log file from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	if logFile.UploadedBy != userID.(uint) {
		logEntry.Warn("Access denied for RCA analysis results", map[string]interface{}{
			"log_file_id": logFile.ID,
			"user_id":     userID.(uint),
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if RCA analysis is completed
	if logFile.RCAAnalysisStatus != "completed" {
		logEntry.Warn("RCA analysis not completed for log file", map[string]interface{}{
			"log_file_id": logFile.ID,
			"status":      logFile.RCAAnalysisStatus,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "RCA analysis not completed"})
		return
	}

	// Get the completed job
	if logFile.RCAAnalysisJobID == nil {
		logEntry.Warn("RCA analysis job not found for log file", map[string]interface{}{
			"log_file_id": logFile.ID,
		})
		c.JSON(http.StatusNotFound, gin.H{"error": "RCA analysis job not found"})
		return
	}

	var job models.Job
	if err := lc.db.First(&job, *logFile.RCAAnalysisJobID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("RCA analysis job not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "RCA analysis job not found"})
		return
	}

	logEntry.Info("RCA analysis results retrieved successfully", map[string]interface{}{
		"log_file_id": logFile.ID,
		"analysis_id": job.ID,
	})

	c.JSON(http.StatusOK, gin.H{
		"analysis": job.Result,
		"job":      job,
	})
}

// GetAdminLogs returns logs for admin users during RCA analysis
func (lc *LogController) GetAdminLogs(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get admin logs", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get admin logs request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	// Check if user is admin
	var user models.User
	if err := lc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("User not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "ADMIN" {
		logEntry.Warn("Admin access required for admin logs", map[string]interface{}{
			"user_id": user.ID,
			"role":    user.Role,
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Get recent logs (last 100 entries)
	var entries []models.LogEntry
	if err := lc.db.Order("created_at DESC").Limit(100).Find(&entries).Error; err != nil {
		logger.WithError(err, "log_controller").Error("Failed to fetch logs from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch logs"})
		return
	}

	logEntry.Info("Admin logs retrieved successfully", map[string]interface{}{
		"count": len(entries),
	})

	c.JSON(http.StatusOK, gin.H{"logs": entries})
}

// AnalyzeLogFile triggers RCA analysis of a log file (background job)
func (lc *LogController) AnalyzeLogFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to analyze log file", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Analyze log file request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid log file ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Parse options from request body
	type AnalyzeOptions struct {
		Timeout  int  `json:"timeout"`
		Chunking bool `json:"chunking"`
	}
	var opts AnalyzeOptions
	if err := c.ShouldBindJSON(&opts); err != nil {
		// fallback to defaults if not provided
		opts.Timeout = 300
		opts.Chunking = true
	}

	// Enforce required fields and valid values
	if opts.Timeout <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Timeout is required and must be a positive integer"})
		return
	}
	// chunking is a bool, so no further validation needed

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("Log file not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		return
	}
	if logFile.UploadedBy != userID {
		logEntry.Warn("Access denied for log file analysis", map[string]interface{}{
			"log_file_id": logFile.ID,
			"user_id":     userID,
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	// Check if file is being processed or RCA is running
	if logFile.Status == "processing" || logFile.RCAAnalysisStatus == "pending" || logFile.RCAAnalysisStatus == "running" {
		logEntry.Warn("Cannot delete a log file that is currently processing or has RCA analysis in progress", map[string]interface{}{
			"log_file_id": logFile.ID,
			"status":      logFile.Status,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete a log file that is currently processing or has RCA analysis in progress."})
		return
	}

	// Get user's LLM endpoint
	var user models.User
	if err := lc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("User not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if user has LLM endpoint configured
	if user.LLMEndpoint == nil || *user.LLMEndpoint == "" {
		logEntry.Warn("User has no LLM endpoint configured for RCA job", map[string]interface{}{
			"user_id": user.ID,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "LLM endpoint not configured. Please configure your LLM endpoint in Settings before submitting RCA jobs."})
		return
	}

	// LLM health check before submitting RCA job using user's endpoint
	if err := lc.llmService.CheckLLMStatusWithEndpoint(*user.LLMEndpoint); err != nil {
		logger.Error("LLM service is not available for RCA job submission", map[string]interface{}{"error": err, "user_endpoint": *user.LLMEndpoint})
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM service is not available. Please check your LLM endpoint configuration and try again."})
		return
	}

	// Create and process RCA job with options
	job, err := lc.jobService.CreateRCAAnalysisJobWithOptions(logFile.ID, opts.Timeout, opts.Chunking)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Failed to create RCA analysis job", map[string]interface{}{
			"log_file_id": logFile.ID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	go lc.jobService.ProcessRCAAnalysisJobWithShutdown(job.ID, lc.stopChan)
	c.JSON(http.StatusAccepted, gin.H{"jobId": job.ID})
}

// GetDetailedErrorAnalysis returns detailed error analysis for a log file
func (lc *LogController) GetDetailedErrorAnalysis(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get detailed error analysis", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get detailed error analysis request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid log file ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.Preload("Entries").First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "log_controller").Error("Log file not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			logger.WithError(err, "log_controller").Error("Failed to fetch log file from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	if logFile.UploadedBy != userID.(uint) {
		logEntry.Warn("Access denied for detailed error analysis", map[string]interface{}{
			"log_file_id": logFile.ID,
			"user_id":     userID.(uint),
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if log file is processed
	if logFile.Status != "completed" {
		logEntry.Warn("Log file not yet processed for detailed error analysis", map[string]interface{}{
			"log_file_id": logFile.ID,
			"status":      logFile.Status,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Log file not yet processed"})
		return
	}

	// Get detailed error analysis
	errorAnalysis, err := lc.llmService.AnalyzeLogsWithAI(&logFile, logFile.Entries, nil) // No job ID for direct analysis
	if err != nil {
		logger.WithError(err, "log_controller").Error("AI analysis failed", map[string]interface{}{
			"log_file_id": logFile.ID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI analysis failed: %v", err)})
		return
	}

	// Filter only ERROR and FATAL entries for the response
	var errorEntries []models.LogEntry
	for _, entry := range logFile.Entries {
		if entry.Level == "ERROR" || entry.Level == "FATAL" {
			errorEntries = append(errorEntries, entry)
		}
	}

	logEntry.Info("Detailed error analysis retrieved successfully", map[string]interface{}{
		"log_file_id": logFile.ID,
		"error_count": len(errorEntries),
	})

	c.JSON(http.StatusOK, gin.H{
		"logFile":       logFile.Filename,
		"errorAnalysis": errorAnalysis,
		"errorEntries":  errorEntries,
		"totalErrors":   len(errorEntries),
	})
}

// GetLLMStatus returns the status of the LLM service and available models
func (lc *LogController) GetLLMStatus(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get LLM status", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get LLM status request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	// Get user's LLM endpoint
	var user models.User
	if err := lc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("User not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if user has LLM endpoint configured
	if user.LLMEndpoint == nil || *user.LLMEndpoint == "" {
		logEntry.Warn("User has no LLM endpoint configured", map[string]interface{}{
			"user_id": user.ID,
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "LLM endpoint not configured. Please configure your LLM endpoint in Settings.",
			"status": "unconfigured",
		})
		return
	}

	// Check LLM health using user's endpoint
	healthError := lc.llmService.CheckLLMStatusWithEndpoint(*user.LLMEndpoint)

	// If healthy, update last status check
	if healthError == nil {
		now := time.Now()
		user.LLMStatusCheckedAt = &now
		lc.db.Save(&user)
	}

	// Get available models from user's endpoint
	models, modelsError := lc.llmService.GetAvailableModelsWithEndpoint(*user.LLMEndpoint)

	// Get current model configuration
	currentModel := "llama3:8b" // Default, could be made configurable

	status := "healthy"
	if healthError != nil {
		status = "unhealthy"
	}

	// Convert errors to strings for JSON serialization
	var healthErrorStr *string
	if healthError != nil {
		errStr := healthError.Error()
		healthErrorStr = &errStr
	}

	var modelsErrorStr *string
	if modelsError != nil {
		errStr := modelsError.Error()
		modelsErrorStr = &errStr
	}

	logEntry.Info("LLM status retrieved successfully", map[string]interface{}{
		"status":            status,
		"health_error":      healthError,
		"current_model":     currentModel,
		"available_models":  models,
		"models_error":      modelsError,
		"user_endpoint":     *user.LLMEndpoint,
		"last_status_check": user.LLMStatusCheckedAt,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":             status,
		"healthError":        healthErrorStr,
		"currentModel":       currentModel,
		"availableModels":    models,
		"modelsError":        modelsErrorStr,
		"userEndpoint":       *user.LLMEndpoint,
		"lastLLMStatusCheck": user.LLMStatusCheckedAt,
	})
}

// GetLogAnalyses returns all RCA analysis jobs for a log file
func (lc *LogController) GetLogAnalyses(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get log analyses", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get log analyses request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid log file ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "log_controller").Error("Log file not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			logger.WithError(err, "log_controller").Error("Failed to fetch log file from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	if logFile.UploadedBy != userID.(uint) {
		logEntry.Warn("Access denied for log analyses", map[string]interface{}{
			"log_file_id": logFile.ID,
			"user_id":     userID.(uint),
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get RCA analysis jobs for this log file
	var jobs []models.Job
	if err := lc.db.Where("log_file_id = ? AND type = ?", logFileID, "rca_analysis").Order("created_at DESC").Find(&jobs).Error; err != nil {
		logger.WithError(err, "log_controller").Error("Failed to fetch analyses from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch analyses"})
		return
	}

	// Convert jobs to analysis format for backward compatibility
	var analyses []map[string]interface{}
	for _, job := range jobs {
		analysis := map[string]interface{}{
			"id":           job.ID,
			"logFileID":    job.LogFileID,
			"status":       job.Status,
			"progress":     job.Progress,
			"error":        job.Error,
			"createdAt":    job.CreatedAt,
			"updatedAt":    job.UpdatedAt,
			"totalChunks":  job.TotalChunks,
			"currentChunk": job.CurrentChunk,
			"failedChunk":  job.FailedChunk,
			"startedAt":    job.StartedAt,
			"completedAt":  job.CompletedAt,
		}

		// If job is completed, extract analysis results
		if job.Status == "completed" && job.Result != nil {
			if analysisData, ok := job.Result["analysis"].(map[string]interface{}); ok {
				analysis["summary"] = analysisData["summary"]
				analysis["severity"] = analysisData["severity"]
				analysis["metadata"] = analysisData["metadata"]
				analysis["errorCount"] = analysisData["errorCount"]
				analysis["warningCount"] = analysisData["warningCount"]
			}
			// Find LogAnalysisMemory for this log file that was created around the same time as job completion
			var memory models.LogAnalysisMemory
			// Look for memory created within 5 minutes of job completion
			jobCompletionTime := job.CompletedAt
			if jobCompletionTime != nil {
				timeWindow := 5 * time.Minute
				startTime := jobCompletionTime.Add(-timeWindow)
				endTime := jobCompletionTime.Add(timeWindow)

				errMem := lc.db.Where("log_file_id = ? AND created_at BETWEEN ? AND ?",
					job.LogFileID, startTime, endTime).Order("created_at desc").First(&memory).Error
				if errMem == nil && memory.ID != 0 {
					analysis["analysisMemoryId"] = memory.ID
				} else {
					// Fallback: get the latest memory for this log file
					errMem = lc.db.Where("log_file_id = ?", job.LogFileID).Order("created_at desc").First(&memory).Error
					if errMem == nil && memory.ID != 0 {
						analysis["analysisMemoryId"] = memory.ID
					}
				}
			} else {
				// Fallback: get the latest memory for this log file
				errMem := lc.db.Where("log_file_id = ?", job.LogFileID).Order("created_at desc").First(&memory).Error
				if errMem == nil && memory.ID != 0 {
					analysis["analysisMemoryId"] = memory.ID
				}
			}
		}

		analyses = append(analyses, analysis)
	}

	logEntry.Info("Log analyses retrieved successfully", map[string]interface{}{
		"log_file_id": logFile.ID,
		"count":       len(analyses),
	})

	c.JSON(http.StatusOK, gin.H{"analyses": analyses})
}

// DeleteLogFile deletes a log file and its associated data
func (lc *LogController) DeleteLogFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to delete log file", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Delete log file request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid log file ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "log_controller").Error("Log file not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			logger.WithError(err, "log_controller").Error("Failed to fetch log file from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	if logFile.UploadedBy != userID.(uint) {
		logEntry.Warn("Access denied for log file deletion", map[string]interface{}{
			"log_file_id": logFile.ID,
			"user_id":     userID.(uint),
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if file is being processed or RCA is running
	if logFile.Status == "processing" || logFile.RCAAnalysisStatus == "pending" || logFile.RCAAnalysisStatus == "running" {
		logEntry.Warn("Cannot delete a log file that is currently processing or has RCA analysis in progress", map[string]interface{}{
			"log_file_id": logFile.ID,
			"status":      logFile.Status,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete a log file that is currently processing or has RCA analysis in progress."})
		return
	}

	// Check for hardDelete query param
	hardDelete := c.DefaultQuery("hardDelete", "false") == "true"

	tx := lc.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if hardDelete {
		logEntry.Info("Performing HARD DELETE", map[string]interface{}{
			"log_file_id": logFileID,
		})

		// HARD DELETE: Remove all related data from all DB tables
		// 1. Delete all jobs for this log file
		if err := tx.Where("log_file_id = ?", logFileID).Unscoped().Delete(&models.Job{}).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "log_controller").Error("Failed to delete analysis jobs")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete analysis jobs"})
			return
		}
		logEntry.Info("Deleted jobs for log file", map[string]interface{}{"log_file_id": logFileID})

		// 2. Delete all log entries for this log file
		if err := tx.Where("log_file_id = ?", logFileID).Unscoped().Delete(&models.LogEntry{}).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "log_controller").Error("Failed to delete log entries")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete log entries"})
			return
		}
		logEntry.Info("Deleted log entries for log file", map[string]interface{}{"log_file_id": logFileID})

		// 3. Delete all log analyses for this log file
		if err := tx.Where("log_file_id = ?", logFileID).Unscoped().Delete(&models.LogAnalysis{}).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "log_controller").Error("Failed to delete log analyses")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete log analyses"})
			return
		}
		logEntry.Info("Deleted log analyses for log file", map[string]interface{}{"log_file_id": logFileID})

		// 4. Delete all log analysis memories for this log file
		var analysisMemories []models.LogAnalysisMemory
		if err := tx.Where("log_file_id = ?", logFileID).Find(&analysisMemories).Error; err == nil && len(analysisMemories) > 0 {
			var memoryIDs []uint
			for _, m := range analysisMemories {
				memoryIDs = append(memoryIDs, m.ID)
			}
			logEntry.Info("Found analysis memories to delete", map[string]interface{}{
				"log_file_id":  logFileID,
				"memory_count": len(memoryIDs),
			})

			// 5. Delete all feedback for these analysis memories
			if err := tx.Where("analysis_memory_id IN ?", memoryIDs).Delete(&models.LogAnalysisFeedback{}).Error; err != nil {
				logEntry.Warn("Failed to delete feedback for analysis memories", map[string]interface{}{
					"log_file_id": logFileID,
					"error":       err.Error(),
				})
				// Continue with deletion even if feedback deletion fails
			} else {
				logEntry.Info("Deleted feedback for analysis memories", map[string]interface{}{
					"log_file_id": logFileID,
				})
			}

			// 6. Delete the analysis memories themselves
			if err := tx.Where("log_file_id = ?", logFileID).Delete(&models.LogAnalysisMemory{}).Error; err != nil {
				tx.Rollback()
				logger.WithError(err, "log_controller").Error("Failed to delete log analysis memories")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete log analysis memories"})
				return
			}
			logEntry.Info("Deleted analysis memories for log file", map[string]interface{}{"log_file_id": logFileID})
		} else {
			logEntry.Info("No analysis memories found for log file", map[string]interface{}{"log_file_id": logFileID})
		}

		// 7. Delete the log file itself
		if ENABLE_LOG_FILE_FLUSH {
			// Remove the file from disk if enabled
			logFilePath := filepath.Join(lc.uploadDir, logFile.Filename)
			if err := os.Remove(logFilePath); err != nil && !os.IsNotExist(err) {
				logEntry.Warn("Failed to delete log file from disk", map[string]interface{}{"log_file_id": logFileID, "file_path": logFilePath, "error": err.Error()})
			} else {
				logEntry.Info("Deleted log file from disk", map[string]interface{}{"log_file_id": logFileID, "file_path": logFilePath})
			}
		}
		if err := tx.Unscoped().Delete(&logFile).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "log_controller").Error("Failed to delete log file")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete log file"})
			return
		}
		logEntry.Info("Deleted log file record", map[string]interface{}{"log_file_id": logFileID})
	} else {
		logEntry.Info("Performing SOFT DELETE", map[string]interface{}{
			"log_file_id": logFileID,
		})

		// SOFT DELETE: Only remove jobs, log entries, and the log file
		if err := tx.Where("log_file_id = ?", logFileID).Delete(&models.Job{}).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "log_controller").Error("Failed to delete analysis jobs")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete analysis jobs"})
			return
		}
		logEntry.Info("Deleted jobs for log file (soft delete)", map[string]interface{}{"log_file_id": logFileID})

		if err := tx.Where("log_file_id = ?", logFileID).Delete(&models.LogEntry{}).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "log_controller").Error("Failed to delete log entries")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete log entries"})
			return
		}
		logEntry.Info("Deleted log entries for log file (soft delete)", map[string]interface{}{"log_file_id": logFileID})

		if err := tx.Delete(&logFile).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "log_controller").Error("Failed to delete log file")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete log file"})
			return
		}
		logEntry.Info("Deleted log file record (soft delete)", map[string]interface{}{"log_file_id": logFileID})
	}

	logEntry.Info("Committing transaction", map[string]interface{}{"log_file_id": logFileID})
	if err := tx.Commit().Error; err != nil {
		logger.WithError(err, "log_controller").Error("Failed to commit transaction")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	logEntry.Info("Log file deleted successfully", map[string]interface{}{
		"log_file_id": logFile.ID,
		"hard_delete": hardDelete,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Log file deleted successfully"})
}

// GetLLMAPICalls returns the history of LLM API calls
func (lc *LogController) GetLLMAPICalls(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get LLM API calls", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get LLM API calls request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	// Check if user is admin
	var user models.User
	if err := lc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("User not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "ADMIN" {
		logEntry.Warn("Admin access required for LLM API calls", map[string]interface{}{
			"user_id": user.ID,
			"role":    user.Role,
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	apiCalls := lc.llmService.GetAPICalls()
	logEntry.Info("LLM API calls retrieved successfully", map[string]interface{}{
		"count": len(apiCalls),
	})

	c.JSON(http.StatusOK, gin.H{
		"apiCalls": apiCalls,
		"count":    len(apiCalls),
	})
}

// ClearLLMAPICalls clears the LLM API call history
func (lc *LogController) ClearLLMAPICalls(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to clear LLM API calls", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Clear LLM API calls request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	// Check if user is admin
	var user models.User
	if err := lc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("User not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "ADMIN" {
		logEntry.Warn("Admin access required to clear LLM API calls", map[string]interface{}{
			"user_id": user.ID,
			"role":    user.Role,
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	lc.llmService.ClearAPICalls()
	logEntry.Info("LLM API call history cleared")
	c.JSON(http.StatusOK, gin.H{"message": "LLM API call history cleared"})
}

// FeedbackRequest represents the payload for feedback submission
type FeedbackRequest struct {
	IsCorrect  bool   `json:"isCorrect"`
	Correction string `json:"correction"`
}

// AddFeedback handles feedback submission for a log analysis
func (lc *LogController) AddFeedback(c *gin.Context) {
	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("Invalid feedback request payload", nil)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	analysisID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid analysis ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid analysis ID"})
		return
	}

	userID, _ := c.Get("userID")
	var userIDPtr *uint
	if uid, ok := userID.(uint); ok {
		userIDPtr = &uid
	}

	feedback := models.LogAnalysisFeedback{
		AnalysisMemoryID: uint(analysisID),
		UserID:           userIDPtr,
		IsCorrect:        req.IsCorrect,
		Correction:       req.Correction,
		CreatedAt:        time.Now(),
	}

	if err := lc.db.Create(&feedback).Error; err != nil {
		logger.WithError(err, "log_controller").Error("Failed to store feedback to database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store feedback"})
		return
	}

	logger.Info("Feedback submitted successfully", map[string]interface{}{
		"analysis_memory_id": feedback.AnalysisMemoryID,
		"user_id":            feedback.UserID,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Feedback submitted successfully"})
}

// GetFeedbackForAnalysis returns all feedback for a given analysis
func (lc *LogController) GetFeedbackForAnalysis(c *gin.Context) {
	analysisID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid analysis ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid analysis ID"})
		return
	}
	var feedbacks []models.LogAnalysisFeedback
	if err := lc.db.Where("analysis_memory_id = ?", analysisID).Order("created_at DESC").Find(&feedbacks).Error; err != nil {
		logger.WithError(err, "log_controller").Error("Failed to fetch feedback from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch feedback"})
		return
	}
	logger.Info("Feedback retrieved successfully", map[string]interface{}{
		"analysis_memory_id": analysisID,
		"count":              len(feedbacks),
	})
	c.JSON(http.StatusOK, gin.H{"feedback": feedbacks})
}

// ExportAllFeedback returns all feedback as JSON (for admin/training use)
func (lc *LogController) ExportAllFeedback(c *gin.Context) {
	var feedbacks []models.LogAnalysisFeedback
	if err := lc.db.Order("created_at DESC").Find(&feedbacks).Error; err != nil {
		logger.WithError(err, "log_controller").Error("Failed to fetch feedback from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch feedback"})
		return
	}
	logger.Info("All feedback retrieved successfully", map[string]interface{}{
		"count": len(feedbacks),
	})
	c.JSON(http.StatusOK, gin.H{"feedback": feedbacks})
}

// GetAllRCAJobs returns all RCA jobs for a given log file
func (lc *LogController) GetAllRCAJobs(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get all RCA jobs", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get all RCA jobs request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	logFileID, err := strconv.ParseUint(c.Param("logFileId"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid log file ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("Log file not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		return
	}
	if logFile.UploadedBy != userID {
		logEntry.Warn("Access denied for RCA jobs", map[string]interface{}{
			"log_file_id": logFile.ID,
			"user_id":     userID.(uint),
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	// Fetch all jobs for this log file, most recent first
	var jobs []models.Job
	if err := lc.db.Where("log_file_id = ?", logFileID).Order("created_at desc").Find(&jobs).Error; err != nil {
		logger.WithError(err, "log_controller").Error("Failed to fetch jobs from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch jobs"})
		return
	}

	logEntry.Info("All RCA jobs retrieved successfully", map[string]interface{}{
		"log_file_id": logFile.ID,
		"count":       len(jobs),
	})

	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

// GetLogFileDetails returns log file details for admin (used in LLM API call context)
func (lc *LogController) GetLogFileDetails(c *gin.Context) {
	// Check if user is admin
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get log file details for admin", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get log file details for admin request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	// Get user to check if admin
	var user models.User
	if err := lc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("User not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "admin" {
		logEntry.Warn("Admin access required for log file details", map[string]interface{}{
			"user_id": user.ID,
			"role":    user.Role,
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid log file ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "log_controller").Error("Log file not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			logger.WithError(err, "log_controller").Error("Failed to fetch log file from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}
	logger.Info("Log file details retrieved successfully for admin", map[string]interface{}{
		"log_file_id": logFile.ID,
		"filename":    logFile.Filename,
	})

	c.JSON(http.StatusOK, gin.H{"logFile": logFile})
}

// GetJobDetails returns job details for admin (used in LLM API call context)
func (lc *LogController) GetJobDetails(c *gin.Context) {
	// Check if user is admin
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get job details for admin", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get job details for admin request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	// Get user to check if admin
	var user models.User
	if err := lc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "log_controller").Error("User not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "admin" {
		logEntry.Warn("Admin access required for job details", map[string]interface{}{
			"user_id": user.ID,
			"role":    user.Role,
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	jobID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Invalid job ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	job, err := lc.jobService.GetJobStatus(uint(jobID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "log_controller").Error("Job not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else {
			logger.WithError(err, "log_controller").Error("Failed to fetch job from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch job"})
		}
		return
	}

	logEntry.Info("Job details retrieved successfully for admin", map[string]interface{}{
		"job_id": job.ID,
		"status": job.Status,
	})

	c.JSON(http.StatusOK, gin.H{"job": job})
}

// GetFeedbackInsights returns aggregated feedback insights
func (lc *LogController) GetFeedbackInsights(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get feedback insights", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get feedback insights request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	// Create feedback service
	feedbackService := services.NewFeedbackService(lc.db)

	// Get feedback insights
	insights, err := feedbackService.GetFeedbackInsights()
	if err != nil {
		logger.WithError(err, "log_controller").Error("Failed to get feedback insights")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get feedback insights"})
		return
	}

	logEntry.Info("Feedback insights retrieved successfully", map[string]interface{}{
		"insights_count": len(insights),
	})

	c.JSON(http.StatusOK, gin.H{"insights": insights})
}

// GetFeedbackForPattern returns feedback for a specific pattern
func (lc *LogController) GetFeedbackForPattern(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get pattern feedback", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get pattern feedback request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	patternName := c.Param("patternName")
	if patternName == "" {
		logger.Error("Pattern name is required", nil)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pattern name is required"})
		return
	}

	// Create feedback service
	feedbackService := services.NewFeedbackService(lc.db)

	// Get feedback for pattern
	feedbacks, err := feedbackService.GetFeedbackForPattern(patternName)
	if err != nil {
		logger.WithError(err, "log_controller").Error("Failed to get pattern feedback")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pattern feedback"})
		return
	}

	logEntry.Info("Pattern feedback retrieved successfully", map[string]interface{}{
		"pattern_name":   patternName,
		"feedback_count": len(feedbacks),
	})

	c.JSON(http.StatusOK, gin.H{"pattern": patternName, "feedback": feedbacks})
}
