package services

import (
	"fmt"
	"regexp"

	"github.com/autolog/backend/internal/logger"
	"github.com/autolog/backend/internal/models"
	"gorm.io/gorm"
)

type ParsingRuleService struct {
	db *gorm.DB
}

func NewParsingRuleService(db *gorm.DB) *ParsingRuleService {
	return &ParsingRuleService{
		db: db,
	}
}

// CreateParsingRule creates a new parsing rule with field mappings and regex patterns
func (prs *ParsingRuleService) CreateParsingRule(userID uint, rule *models.ParsingRule) error {
	logEntry := logger.WithUser(userID)
	logEntry.Info("Creating parsing rule", map[string]interface{}{
		"rule_name":   rule.Name,
		"is_template": rule.IsTemplate,
	})

	// Validate the rule
	if err := rule.Validate(); err != nil {
		logger.WithError(err, "parsing_rule_service").Error("Parsing rule validation failed")
		return fmt.Errorf("parsing rule validation failed: %w", err)
	}

	// Set user ID
	rule.UserID = userID

	// Start transaction
	tx := prs.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create the parsing rule
	if err := tx.Create(rule).Error; err != nil {
		tx.Rollback()
		logger.WithError(err, "parsing_rule_service").Error("Failed to create parsing rule")
		return fmt.Errorf("failed to create parsing rule: %w", err)
	}

	// Create field mappings
	for i := range rule.FieldMappings {
		rule.FieldMappings[i].ParsingRuleID = rule.ID
		if err := rule.FieldMappings[i].Validate(); err != nil {
			tx.Rollback()
			logger.WithError(err, "parsing_rule_service").Error("Field mapping validation failed")
			return fmt.Errorf("field mapping validation failed: %w", err)
		}
		if err := tx.Create(&rule.FieldMappings[i]).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "parsing_rule_service").Error("Failed to create field mapping")
			return fmt.Errorf("failed to create field mapping: %w", err)
		}
	}

	// Create regex patterns
	for i := range rule.RegexPatterns {
		rule.RegexPatterns[i].ParsingRuleID = rule.ID
		if err := rule.RegexPatterns[i].Validate(); err != nil {
			tx.Rollback()
			logger.WithError(err, "parsing_rule_service").Error("Regex pattern validation failed")
			return fmt.Errorf("regex pattern validation failed: %w", err)
		}
		if err := tx.Create(&rule.RegexPatterns[i]).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "parsing_rule_service").Error("Failed to create regex pattern")
			return fmt.Errorf("failed to create regex pattern: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		logger.WithError(err, "parsing_rule_service").Error("Failed to commit parsing rule creation")
		return fmt.Errorf("failed to commit parsing rule creation: %w", err)
	}

	logEntry.Info("Parsing rule created successfully", map[string]interface{}{
		"rule_id":        rule.ID,
		"field_mappings": len(rule.FieldMappings),
		"regex_patterns": len(rule.RegexPatterns),
	})

	return nil
}

// GetUserParsingRules returns all parsing rules for a user
func (prs *ParsingRuleService) GetUserParsingRules(userID uint) ([]models.ParsingRule, error) {
	var rules []models.ParsingRule
	err := prs.db.Where("user_id = ?", userID).
		Preload("FieldMappings").
		Preload("RegexPatterns").
		Order("created_at DESC").
		Find(&rules).Error

	if err != nil {
		logger.WithError(err, "parsing_rule_service").Error("Failed to get user parsing rules")
		return nil, fmt.Errorf("failed to get user parsing rules: %w", err)
	}

	return rules, nil
}

// GetParsingRule returns a specific parsing rule with its mappings and patterns
func (prs *ParsingRuleService) GetParsingRule(ruleID, userID uint) (*models.ParsingRule, error) {
	var rule models.ParsingRule
	err := prs.db.Where("id = ? AND user_id = ?", ruleID, userID).
		Preload("FieldMappings").
		Preload("RegexPatterns").
		First(&rule).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "parsing_rule_service").Error("Parsing rule not found")
			return nil, fmt.Errorf("parsing rule not found")
		}
		logger.WithError(err, "parsing_rule_service").Error("Failed to get parsing rule")
		return nil, fmt.Errorf("failed to get parsing rule: %w", err)
	}

	return &rule, nil
}

// UpdateParsingRule updates an existing parsing rule
func (prs *ParsingRuleService) UpdateParsingRule(ruleID, userID uint, updates *models.ParsingRule) error {
	logEntry := logger.WithUser(userID)
	logEntry.Info("Updating parsing rule", map[string]interface{}{
		"rule_id": ruleID,
	})

	// Verify ownership
	var existingRule models.ParsingRule
	if err := prs.db.Where("id = ? AND user_id = ?", ruleID, userID).First(&existingRule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "parsing_rule_service").Error("Parsing rule not found")
			return fmt.Errorf("parsing rule not found")
		}
		logger.WithError(err, "parsing_rule_service").Error("Failed to verify parsing rule ownership")
		return fmt.Errorf("failed to verify parsing rule ownership: %w", err)
	}

	// Start transaction
	tx := prs.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update the parsing rule
	if err := tx.Model(&existingRule).Updates(updates).Error; err != nil {
		tx.Rollback()
		logger.WithError(err, "parsing_rule_service").Error("Failed to update parsing rule")
		return fmt.Errorf("failed to update parsing rule: %w", err)
	}

	// Update field mappings if provided
	if len(updates.FieldMappings) > 0 {
		// Delete existing mappings
		if err := tx.Where("parsing_rule_id = ?", ruleID).Delete(&models.FieldMapping{}).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "parsing_rule_service").Error("Failed to delete existing field mappings")
			return fmt.Errorf("failed to delete existing field mappings: %w", err)
		}

		// Create new mappings
		for i := range updates.FieldMappings {
			updates.FieldMappings[i].ParsingRuleID = ruleID
			if err := updates.FieldMappings[i].Validate(); err != nil {
				tx.Rollback()
				logger.WithError(err, "parsing_rule_service").Error("Field mapping validation failed")
				return fmt.Errorf("field mapping validation failed: %w", err)
			}
			if err := tx.Create(&updates.FieldMappings[i]).Error; err != nil {
				tx.Rollback()
				logger.WithError(err, "parsing_rule_service").Error("Failed to create field mapping")
				return fmt.Errorf("failed to create field mapping: %w", err)
			}
		}
	}

	// Update regex patterns if provided
	if len(updates.RegexPatterns) > 0 {
		// Delete existing patterns
		if err := tx.Where("parsing_rule_id = ?", ruleID).Delete(&models.RegexPattern{}).Error; err != nil {
			tx.Rollback()
			logger.WithError(err, "parsing_rule_service").Error("Failed to delete existing regex patterns")
			return fmt.Errorf("failed to delete existing regex patterns: %w", err)
		}

		// Create new patterns
		for i := range updates.RegexPatterns {
			updates.RegexPatterns[i].ParsingRuleID = ruleID
			if err := updates.RegexPatterns[i].Validate(); err != nil {
				tx.Rollback()
				logger.WithError(err, "parsing_rule_service").Error("Regex pattern validation failed")
				return fmt.Errorf("regex pattern validation failed: %w", err)
			}
			if err := tx.Create(&updates.RegexPatterns[i]).Error; err != nil {
				tx.Rollback()
				logger.WithError(err, "parsing_rule_service").Error("Failed to create regex pattern")
				return fmt.Errorf("failed to create regex pattern: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		logger.WithError(err, "parsing_rule_service").Error("Failed to commit parsing rule update")
		return fmt.Errorf("failed to commit parsing rule update: %w", err)
	}

	logEntry.Info("Parsing rule updated successfully", map[string]interface{}{
		"rule_id": ruleID,
	})

	return nil
}

// DeleteParsingRule deletes a parsing rule
func (prs *ParsingRuleService) DeleteParsingRule(ruleID, userID uint) error {
	logEntry := logger.WithUser(userID)
	logEntry.Info("Deleting parsing rule", map[string]interface{}{
		"rule_id": ruleID,
	})

	// Verify ownership
	var rule models.ParsingRule
	if err := prs.db.Where("id = ? AND user_id = ?", ruleID, userID).First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.WithError(err, "parsing_rule_service").Error("Parsing rule not found")
			return fmt.Errorf("parsing rule not found")
		}
		logger.WithError(err, "parsing_rule_service").Error("Failed to verify parsing rule ownership")
		return fmt.Errorf("failed to verify parsing rule ownership: %w", err)
	}

	// Start transaction
	tx := prs.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete field mappings
	if err := tx.Where("parsing_rule_id = ?", ruleID).Delete(&models.FieldMapping{}).Error; err != nil {
		tx.Rollback()
		logger.WithError(err, "parsing_rule_service").Error("Failed to delete field mappings")
		return fmt.Errorf("failed to delete field mappings: %w", err)
	}

	// Delete regex patterns
	if err := tx.Where("parsing_rule_id = ?", ruleID).Delete(&models.RegexPattern{}).Error; err != nil {
		tx.Rollback()
		logger.WithError(err, "parsing_rule_service").Error("Failed to delete regex patterns")
		return fmt.Errorf("failed to delete regex patterns: %w", err)
	}

	// Delete the parsing rule
	if err := tx.Delete(&rule).Error; err != nil {
		tx.Rollback()
		logger.WithError(err, "parsing_rule_service").Error("Failed to delete parsing rule")
		return fmt.Errorf("failed to delete parsing rule: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		logger.WithError(err, "parsing_rule_service").Error("Failed to commit parsing rule deletion")
		return fmt.Errorf("failed to commit parsing rule deletion: %w", err)
	}

	logEntry.Info("Parsing rule deleted successfully", map[string]interface{}{
		"rule_id": ruleID,
	})

	return nil
}

// GetActiveParsingRules returns all active parsing rules for a user
func (prs *ParsingRuleService) GetActiveParsingRules(userID uint) ([]models.ParsingRule, error) {
	var rules []models.ParsingRule
	err := prs.db.Where("user_id = ? AND is_active = ?", userID, true).
		Preload("FieldMappings", "is_active = ?", true).
		Preload("RegexPatterns", "is_active = ?", true).
		Order("created_at DESC").
		Find(&rules).Error

	if err != nil {
		logger.WithError(err, "parsing_rule_service").Error("Failed to get active parsing rules")
		return nil, fmt.Errorf("failed to get active parsing rules: %w", err)
	}

	return rules, nil
}

// ApplyParsingRules applies user-defined parsing rules to a log entry
func (prs *ParsingRuleService) ApplyParsingRules(userID uint, parsed map[string]interface{}) (map[string]interface{}, error) {
	// Get active parsing rules for the user
	rules, err := prs.GetActiveParsingRules(userID)
	if err != nil {
		return parsed, err // Return original if rules can't be loaded
	}

	if len(rules) == 0 {
		return parsed, nil // No rules to apply
	}

	// Apply field mappings from all active rules
	for _, rule := range rules {
		for _, mapping := range rule.FieldMappings {
			if value, exists := parsed[mapping.SourceField]; exists {
				parsed[mapping.TargetField] = value
				// Keep original field as well for backward compatibility
			}
		}
	}

	return parsed, nil
}

// TestParsingRule tests a parsing rule against sample log data
func (prs *ParsingRuleService) TestParsingRule(rule *models.ParsingRule, sampleLogs []string) (*ParsingRuleTestResult, error) {
	result := &ParsingRuleTestResult{
		RuleID:       rule.ID,
		RuleName:     rule.Name,
		TotalLogs:    len(sampleLogs),
		SuccessCount: 0,
		FailureCount: 0,
		Details:      make([]LogTestDetail, 0),
	}

	for i, logLine := range sampleLogs {
		detail := LogTestDetail{
			LogIndex: i,
			LogLine:  logLine,
			Success:  false,
			Errors:   make([]string, 0),
		}

		// Test field mappings
		parsed := make(map[string]interface{})
		// Simulate parsing (this would be the actual parsing logic)
		parsed["raw"] = logLine

		// Apply field mappings
		for _, mapping := range rule.FieldMappings {
			if value, exists := parsed[mapping.SourceField]; exists {
				parsed[mapping.TargetField] = value
			}
		}

		// Test regex patterns
		for _, pattern := range rule.RegexPatterns {
			re, err := regexp.Compile(pattern.Pattern)
			if err != nil {
				detail.Errors = append(detail.Errors, fmt.Sprintf("Invalid regex pattern '%s': %v", pattern.Name, err))
				continue
			}

			if re.MatchString(logLine) {
				detail.Success = true
				detail.MatchedPattern = pattern.Name
				break
			}
		}

		if detail.Success {
			result.SuccessCount++
		} else {
			result.FailureCount++
		}

		result.Details = append(result.Details, detail)
	}

	return result, nil
}

// ParsingRuleTestResult represents the result of testing a parsing rule
type ParsingRuleTestResult struct {
	RuleID       uint            `json:"ruleId"`
	RuleName     string          `json:"ruleName"`
	TotalLogs    int             `json:"totalLogs"`
	SuccessCount int             `json:"successCount"`
	FailureCount int             `json:"failureCount"`
	Details      []LogTestDetail `json:"details"`
}

// LogTestDetail represents the result of testing a single log line
type LogTestDetail struct {
	LogIndex       int      `json:"logIndex"`
	LogLine        string   `json:"logLine"`
	Success        bool     `json:"success"`
	MatchedPattern string   `json:"matchedPattern,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}
