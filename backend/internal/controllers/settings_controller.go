package controllers

import (
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"github.com/autolog/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SettingsController struct {
	db *gorm.DB
}

func NewSettingsController(db *gorm.DB) *SettingsController {
	return &SettingsController{db: db}
}

type LLMEndpointRequest struct {
	LLMEndpoint string `json:"llm_endpoint" binding:"required"`
}

type TestLLMEndpointRequest struct {
	LLMEndpoint string `json:"llm_endpoint" binding:"required"`
}

// GetLLMEndpoint returns the current user's LLM endpoint
func (sc *SettingsController) GetLLMEndpoint(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get LLM endpoint", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get LLM endpoint request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	var user models.User
	if err := sc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "settings_controller").Error("User not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	logEntry.Info("LLM endpoint retrieved successfully", map[string]interface{}{
		"user_id":      user.ID,
		"has_endpoint": user.LLMEndpoint != nil,
		"endpoint":     user.LLMEndpoint,
	})

	c.JSON(http.StatusOK, gin.H{
		"llm_endpoint":       user.LLMEndpoint,
		"lastLLMStatusCheck": user.LLMStatusCheckedAt,
	})
}

// TestLLMEndpoint tests the provided LLM endpoint
func (sc *SettingsController) TestLLMEndpoint(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to test LLM endpoint", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Test LLM endpoint request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	var req TestLLMEndpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithError(err, "settings_controller").Error("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate URL format
	if _, err := url.ParseRequestURI(req.LLMEndpoint); err != nil {
		logger.WithError(err, "settings_controller").Error("Invalid LLM endpoint URL")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid LLM endpoint URL format"})
		return
	}

	// Create a temporary LLM service with the provided endpoint
	llmService := services.NewLLMServiceWithEndpoint(req.LLMEndpoint, "llama2:7b")

	// Test the endpoint
	if err := llmService.CheckLLMStatusWithEndpoint(req.LLMEndpoint); err != nil {
		logger.WithError(err, "settings_controller").Error("LLM endpoint test failed", map[string]interface{}{
			"endpoint": req.LLMEndpoint,
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "LLM endpoint test failed",
			"message": err.Error(),
		})
		return
	}

	// Update the user's last status check time
	var user models.User
	if err := sc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "settings_controller").Error("User not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	now := time.Now()
	user.LLMStatusCheckedAt = &now
	if err := sc.db.Save(&user).Error; err != nil {
		logger.WithError(err, "settings_controller").Error("Failed to update LLM status check time")
		// Don't fail the test if we can't update the timestamp
	}

	logEntry.Info("LLM endpoint test successful", map[string]interface{}{
		"user_id":  user.ID,
		"endpoint": req.LLMEndpoint,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "LLM endpoint test successful",
		"status":  "healthy",
	})
}

// UpdateLLMEndpoint updates the current user's LLM endpoint
func (sc *SettingsController) UpdateLLMEndpoint(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to update LLM endpoint", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Update LLM endpoint request received", map[string]interface{}{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
	})

	var req LLMEndpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithError(err, "settings_controller").Error("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate URL format
	if req.LLMEndpoint != "" {
		if _, err := url.ParseRequestURI(req.LLMEndpoint); err != nil {
			logger.WithError(err, "settings_controller").Error("Invalid LLM endpoint URL")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid LLM endpoint URL format"})
			return
		}
	}

	var user models.User
	if err := sc.db.First(&user, userID).Error; err != nil {
		logger.WithError(err, "settings_controller").Error("User not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Update the LLM endpoint
	if req.LLMEndpoint == "" {
		user.LLMEndpoint = nil
	} else {
		user.LLMEndpoint = &req.LLMEndpoint
	}

	if err := sc.db.Save(&user).Error; err != nil {
		logger.WithError(err, "settings_controller").Error("Failed to update LLM endpoint")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update LLM endpoint"})
		return
	}

	logEntry.Info("LLM endpoint updated successfully", map[string]interface{}{
		"user_id":  user.ID,
		"endpoint": user.LLMEndpoint,
	})

	c.JSON(http.StatusOK, gin.H{
		"message":      "LLM endpoint updated successfully",
		"llm_endpoint": user.LLMEndpoint,
	})
}

// GET /settings/llm-models
func (sc *SettingsController) GetLLMModels(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	// Get user's LLM endpoint
	var user models.User
	if err := sc.db.First(&user, userID).Error; err != nil || user.LLMEndpoint == nil || *user.LLMEndpoint == "" {
		c.JSON(400, gin.H{"error": "LLM endpoint not configured"})
		return
	}
	llmService := services.NewLLMServiceWithEndpoint(*user.LLMEndpoint, "llama2:7b")
	models, err := llmService.GetAvailableModelsWithEndpoint(*user.LLMEndpoint)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"models": models})
}

// POST /settings/llm-pull-model
func (sc *SettingsController) PullLLMModel(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	var req struct {
		Model string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
		c.JSON(400, gin.H{"error": "Model name required"})
		return
	}
	// Get user's LLM endpoint
	var user models.User
	if err := sc.db.First(&user, userID).Error; err != nil || user.LLMEndpoint == nil || *user.LLMEndpoint == "" {
		c.JSON(400, gin.H{"error": "LLM endpoint not configured"})
		return
	}
	llmService := services.NewLLMServiceWithEndpoint(*user.LLMEndpoint, "llama2:7b")
	err := llmService.PullModelWithEndpoint(req.Model, *user.LLMEndpoint)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// GetUploadLimit returns the current file upload size limit in bytes
func (sc *SettingsController) GetUploadLimit(c *gin.Context) {
	maxSizeStr := os.Getenv("MAX_FILE_SIZE")
	var maxSize int64 = 5 * 1024 * 1024 // Default 5MB
	if maxSizeStr != "" {
		if parsed, err := strconv.ParseInt(maxSizeStr, 10, 64); err == nil && parsed > 0 {
			maxSize = parsed
		}
	}
	c.JSON(http.StatusOK, gin.H{"uploadLimit": maxSize})
}
