package controllers

import (
	"net/http"
	"strconv"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"github.com/autolog/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LearningController struct {
	db              *gorm.DB
	learningService *services.LearningService
}

func NewLearningController(db *gorm.DB, learningService *services.LearningService) *LearningController {
	return &LearningController{
		db:              db,
		learningService: learningService,
	}
}

// GetLearningInsights returns learning insights for a specific log file
func (lc *LearningController) GetLearningInsights(c *gin.Context) {
	logFileIDStr := c.Param("logFileID")
	logFileID, err := strconv.ParseUint(logFileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Get log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		return
	}

	// Get log entries
	var entries []models.LogEntry
	if err := lc.db.Where("log_file_id = ?", logFileID).Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load log entries"})
		return
	}

	// Filter error entries
	var errorEntries []models.LogEntry
	for _, entry := range entries {
		if entry.Level == "ERROR" || entry.Level == "FATAL" {
			errorEntries = append(errorEntries, entry)
		}
	}

	// Get learning insights
	insights, err := lc.learningService.GetLearningInsights(&logFile, errorEntries)
	if err != nil {
		logger.Error("Failed to get learning insights", map[string]interface{}{
			"logFileID": logFileID,
			"error":     err,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get learning insights"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"insights": insights,
		"logFile":  logFile,
	})
}

// GetPatterns returns all learned patterns
func (lc *LearningController) GetPatterns(c *gin.Context) {
	patterns, err := lc.learningService.GetPatterns()
	if err != nil {
		logger.Error("Failed to get patterns", map[string]interface{}{"error": err})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get patterns"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"patterns": patterns,
	})
}

// GetPattern returns a specific pattern by ID
func (lc *LearningController) GetPattern(c *gin.Context) {
	patternIDStr := c.Param("patternID")
	patternID, err := strconv.ParseUint(patternIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pattern ID"})
		return
	}

	pattern, err := lc.learningService.GetPatternByID(uint(patternID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pattern not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pattern": pattern,
	})
}

// DeletePattern deletes a pattern
func (lc *LearningController) DeletePattern(c *gin.Context) {
	patternIDStr := c.Param("patternID")
	patternID, err := strconv.ParseUint(patternIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pattern ID"})
		return
	}

	if err := lc.learningService.DeletePattern(uint(patternID)); err != nil {
		logger.Error("Failed to delete pattern", map[string]interface{}{
			"patternID": patternID,
			"error":     err,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete pattern"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pattern deleted successfully"})
}

// GetLearningMetrics returns learning performance metrics
func (lc *LearningController) GetLearningMetrics(c *gin.Context) {
	metrics, err := lc.learningService.GetLearningMetrics()
	if err != nil {
		logger.Error("Failed to get learning metrics", map[string]interface{}{"error": err})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get learning metrics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics": metrics,
	})
}

// GetSimilarIncidents returns similar incidents for a log file
func (lc *LearningController) GetSimilarIncidents(c *gin.Context) {
	logFileIDStr := c.Param("logFileID")
	logFileID, err := strconv.ParseUint(logFileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file ID"})
		return
	}

	// Get log file
	var logFile models.LogFile
	if err := lc.db.First(&logFile, logFileID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		return
	}

	// Get log entries
	var entries []models.LogEntry
	if err := lc.db.Where("log_file_id = ?", logFileID).Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load log entries"})
		return
	}

	// Filter error entries
	var errorEntries []models.LogEntry
	for _, entry := range entries {
		if entry.Level == "ERROR" || entry.Level == "FATAL" {
			errorEntries = append(errorEntries, entry)
		}
	}

	// Get similar incidents
	insights, err := lc.learningService.GetLearningInsights(&logFile, errorEntries)
	if err != nil {
		logger.Error("Failed to get similar incidents", map[string]interface{}{
			"logFileID": logFileID,
			"error":     err,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get similar incidents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"similarIncidents": insights.SimilarIncidents,
		"logFile":          logFile,
	})
}
