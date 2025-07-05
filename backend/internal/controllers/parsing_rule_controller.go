package controllers

import (
	"net/http"
	"strconv"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"github.com/autolog/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type ParsingRuleController struct {
	parsingRuleService *services.ParsingRuleService
}

func NewParsingRuleController(parsingRuleService *services.ParsingRuleService) *ParsingRuleController {
	return &ParsingRuleController{
		parsingRuleService: parsingRuleService,
	}
}

// CreateParsingRule creates a new parsing rule
func (prc *ParsingRuleController) CreateParsingRule(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to create parsing rule", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Info("Create parsing rule request received")

	var rule models.ParsingRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := prc.parsingRuleService.CreateParsingRule(userID.(uint), &rule); err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Failed to create parsing rule")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create parsing rule"})
		return
	}

	logEntry.Info("Parsing rule created successfully", map[string]interface{}{
		"rule_id": rule.ID,
	})

	c.JSON(http.StatusCreated, gin.H{
		"message": "Parsing rule created successfully",
		"rule":    rule,
	})
}

// GetUserParsingRules returns all parsing rules for the current user
func (prc *ParsingRuleController) GetUserParsingRules(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get parsing rules", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get user parsing rules request received")

	rules, err := prc.parsingRuleService.GetUserParsingRules(userID.(uint))
	if err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Failed to get user parsing rules")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get parsing rules"})
		return
	}

	logEntry.Info("User parsing rules retrieved successfully", map[string]interface{}{
		"rules_count": len(rules),
	})

	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
	})
}

// GetParsingRule returns a specific parsing rule
func (prc *ParsingRuleController) GetParsingRule(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get parsing rule", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	ruleIDStr := c.Param("id")
	ruleID, err := strconv.ParseUint(ruleIDStr, 10, 32)
	if err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Invalid rule ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule ID"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Info("Get parsing rule request received", map[string]interface{}{
		"rule_id": ruleID,
	})

	rule, err := prc.parsingRuleService.GetParsingRule(uint(ruleID), userID.(uint))
	if err != nil {
		if err.Error() == "parsing rule not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Parsing rule not found"})
		} else {
			logger.WithError(err, "parsing_rule_controller").Error("Failed to get parsing rule")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get parsing rule"})
		}
		return
	}

	logEntry.Info("Parsing rule retrieved successfully", map[string]interface{}{
		"rule_id": rule.ID,
	})

	c.JSON(http.StatusOK, gin.H{
		"rule": rule,
	})
}

// UpdateParsingRule updates an existing parsing rule
func (prc *ParsingRuleController) UpdateParsingRule(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to update parsing rule", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	ruleIDStr := c.Param("id")
	ruleID, err := strconv.ParseUint(ruleIDStr, 10, 32)
	if err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Invalid rule ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule ID"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Info("Update parsing rule request received", map[string]interface{}{
		"rule_id": ruleID,
	})

	var updates models.ParsingRule
	if err := c.ShouldBindJSON(&updates); err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := prc.parsingRuleService.UpdateParsingRule(uint(ruleID), userID.(uint), &updates); err != nil {
		if err.Error() == "parsing rule not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Parsing rule not found"})
		} else {
			logger.WithError(err, "parsing_rule_controller").Error("Failed to update parsing rule")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update parsing rule"})
		}
		return
	}

	logEntry.Info("Parsing rule updated successfully", map[string]interface{}{
		"rule_id": ruleID,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Parsing rule updated successfully",
	})
}

// DeleteParsingRule deletes a parsing rule
func (prc *ParsingRuleController) DeleteParsingRule(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to delete parsing rule", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	ruleIDStr := c.Param("id")
	ruleID, err := strconv.ParseUint(ruleIDStr, 10, 32)
	if err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Invalid rule ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule ID"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Info("Delete parsing rule request received", map[string]interface{}{
		"rule_id": ruleID,
	})

	if err := prc.parsingRuleService.DeleteParsingRule(uint(ruleID), userID.(uint)); err != nil {
		if err.Error() == "parsing rule not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Parsing rule not found"})
		} else {
			logger.WithError(err, "parsing_rule_controller").Error("Failed to delete parsing rule")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete parsing rule"})
		}
		return
	}

	logEntry.Info("Parsing rule deleted successfully", map[string]interface{}{
		"rule_id": ruleID,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Parsing rule deleted successfully",
	})
}

// TestParsingRule tests a parsing rule against sample log data
func (prc *ParsingRuleController) TestParsingRule(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to test parsing rule", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var request struct {
		Rule       models.ParsingRule `json:"rule"`
		SampleLogs []string           `json:"sampleLogs"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Info("Test parsing rule request received", map[string]interface{}{
		"rule_name":    request.Rule.Name,
		"sample_count": len(request.SampleLogs),
	})

	result, err := prc.parsingRuleService.TestParsingRule(&request.Rule, request.SampleLogs)
	if err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Failed to test parsing rule")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to test parsing rule"})
		return
	}

	logEntry.Info("Parsing rule test completed", map[string]interface{}{
		"success_count": result.SuccessCount,
		"failure_count": result.FailureCount,
		"total_logs":    result.TotalLogs,
	})

	c.JSON(http.StatusOK, gin.H{
		"result": result,
	})
}

// GetActiveParsingRules returns all active parsing rules for the current user
func (prc *ParsingRuleController) GetActiveParsingRules(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		logger.Error("Unauthorized access attempt to get active parsing rules", nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	logEntry := logger.WithUser(userID.(uint))
	logEntry.Debug("Get active parsing rules request received")

	rules, err := prc.parsingRuleService.GetActiveParsingRules(userID.(uint))
	if err != nil {
		logger.WithError(err, "parsing_rule_controller").Error("Failed to get active parsing rules")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get active parsing rules"})
		return
	}

	logEntry.Info("Active parsing rules retrieved successfully", map[string]interface{}{
		"active_rules_count": len(rules),
	})

	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
	})
}
