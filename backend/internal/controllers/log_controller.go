package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/autolog/backend/internal/models"
	"github.com/autolog/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LogController struct {
	db           *gorm.DB
	logProcessor *services.LogProcessor
	llmService   *services.LLMService
	uploadDir    string
}

func NewLogController(db *gorm.DB, llmService *services.LLMService) *LogController {
	return &LogController{
		db:           db,
		logProcessor: services.NewLogProcessor(db, llmService),
		llmService:   llmService,
		uploadDir:    "uploads/logs",
	}
}

// UploadLogFile handles log file upload
func (lc *LogController) UploadLogFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	file, err := c.FormFile("logfile")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Validate file extension
	ext := filepath.Ext(file.Filename)
	if ext != ".json" && ext != ".log" && ext != ".txt" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only JSON, LOG, and TXT files are supported"})
		return
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(lc.uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	// Generate unique filename
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s", timestamp, file.Filename)
	filepath := filepath.Join(lc.uploadDir, filename)

	// Save file
	if err := c.SaveUploadedFile(file, filepath); err != nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save log file record"})
		return
	}

	// Process log file in background
	go func() {
		if err := lc.logProcessor.ProcessLogFile(logFile.ID, filepath); err != nil {
			fmt.Printf("Failed to process log file %d: %v\n", logFile.ID, err)
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var logFiles []models.LogFile
	query := lc.db.Preload("Uploader").Where("uploaded_by = ?", userID).Order("created_at DESC")

	// Add pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	query = query.Offset(offset).Limit(limit)

	if err := query.Find(&logFiles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log files"})
		return
	}

	// Get total count
	var total int64
	lc.db.Model(&models.LogFile{}).Where("uploaded_by = ?", userID).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"logFiles": logFiles,
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	var logFile models.LogFile
	if err := lc.db.Preload("Uploader").Preload("Entries").First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	// Check if user owns this log file
	if logFile.UploadedBy != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logFile": logFile})
}

// AnalyzeLogFile triggers analysis of a log file
func (lc *LogController) AnalyzeLogFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	if logFile.UploadedBy != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Perform analysis
	analysis, err := lc.logProcessor.AnalyzeLogFile(uint(logFileID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Analysis failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Log analysis completed",
		"analysis": analysis,
	})
}

// GetDetailedErrorAnalysis returns detailed error analysis for a log file
func (lc *LogController) GetDetailedErrorAnalysis(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.Preload("Entries").First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	if logFile.UploadedBy != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if log file is processed
	if logFile.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Log file not yet processed"})
		return
	}

	// Get detailed error analysis
	errorAnalysis, err := lc.llmService.AnalyzeLogsWithAI(logFile, logFile.Entries)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error analysis failed: %v", err)})
		return
	}

	// Filter only ERROR and FATAL entries for the response
	var errorEntries []models.LogEntry
	for _, entry := range logFile.Entries {
		if entry.Level == models.LogLevelError || entry.Level == models.LogLevelFatal {
			errorEntries = append(errorEntries, entry)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"logFile":       logFile.Filename,
		"errorAnalysis": errorAnalysis,
		"errorEntries":  errorEntries,
		"totalErrors":   len(errorEntries),
	})
}

// GetLLMStatus returns the status of the LLM service and available models
func (lc *LogController) GetLLMStatus(c *gin.Context) {
	_, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Check LLM health
	healthError := lc.llmService.CheckLLMHealth()

	// Get available models
	models, modelsError := lc.llmService.GetAvailableModels()

	// Get current model configuration
	currentModel := "llama2" // Default, could be made configurable

	status := "healthy"
	if healthError != nil {
		status = "unhealthy"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          status,
		"healthError":     healthError,
		"currentModel":    currentModel,
		"availableModels": models,
		"modelsError":     modelsError,
		"ollamaUrl":       "http://localhost:11434",
	})
}

// GetLogAnalyses returns all analyses for a log file
func (lc *LogController) GetLogAnalyses(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	if logFile.UploadedBy != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var analyses []models.LogAnalysis
	if err := lc.db.Where("log_file_id = ?", logFileID).Order("created_at DESC").Find(&analyses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch analyses"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"analyses": analyses})
}

// DeleteLogFile deletes a log file and its associated data
func (lc *LogController) DeleteLogFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logFileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Check if user owns this log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log file"})
		}
		return
	}

	if logFile.UploadedBy != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Delete in transaction
	tx := lc.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete analyses
	if err := tx.Where("log_file_id = ?", logFileID).Delete(&models.LogAnalysis{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete analyses"})
		return
	}

	// Delete log entries
	if err := tx.Where("log_file_id = ?", logFileID).Delete(&models.LogEntry{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete log entries"})
		return
	}

	// Delete log file
	if err := tx.Delete(&logFile).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete log file"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Log file deleted successfully"})
}
