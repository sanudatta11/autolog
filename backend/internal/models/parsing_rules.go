package models

import (
	"fmt"
	"regexp"
	"time"

	"gorm.io/gorm"
)

// ParsingRule represents a user-defined parsing rule
type ParsingRule struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UserID      uint           `json:"userId" gorm:"not null;index"`
	Name        string         `json:"name" gorm:"not null"`
	Description string         `json:"description" gorm:"type:text"`
	IsActive    bool           `json:"isActive" gorm:"default:true"`
	IsTemplate  bool           `json:"isTemplate" gorm:"default:false"` // Template for sharing
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	User          *User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	FieldMappings []FieldMapping `json:"fieldMappings,omitempty" gorm:"foreignKey:ParsingRuleID"`
	RegexPatterns []RegexPattern `json:"regexPatterns,omitempty" gorm:"foreignKey:ParsingRuleID"`
}

// FieldMapping defines custom field mappings for log parsing
type FieldMapping struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	ParsingRuleID uint           `json:"parsingRuleId" gorm:"not null;index"`
	SourceField   string         `json:"sourceField" gorm:"not null"` // Field name in source log
	TargetField   string         `json:"targetField" gorm:"not null"` // Field name in canonical schema
	Description   string         `json:"description" gorm:"type:text"`
	IsActive      bool           `json:"isActive" gorm:"default:true"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationship
	ParsingRule *ParsingRule `json:"parsingRule,omitempty" gorm:"foreignKey:ParsingRuleID"`
}

// RegexPattern defines custom regex patterns for unstructured log parsing
type RegexPattern struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	ParsingRuleID uint           `json:"parsingRuleId" gorm:"not null;index"`
	Name          string         `json:"name" gorm:"not null"`
	Pattern       string         `json:"pattern" gorm:"not null;type:text"`
	Description   string         `json:"description" gorm:"type:text"`
	Priority      int            `json:"priority" gorm:"default:0"` // Higher priority patterns are tried first
	IsActive      bool           `json:"isActive" gorm:"default:true"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationship
	ParsingRule *ParsingRule `json:"parsingRule,omitempty" gorm:"foreignKey:ParsingRuleID"`
}

// ParsingRuleTemplate represents a shared template for parsing rules
type ParsingRuleTemplate struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"not null"`
	Description string         `json:"description" gorm:"type:text"`
	Category    string         `json:"category" gorm:"not null"` // e.g., "apache", "nginx", "java", "python"
	IsPublic    bool           `json:"isPublic" gorm:"default:true"`
	CreatedBy   uint           `json:"createdBy" gorm:"not null"`
	UsageCount  int            `json:"usageCount" gorm:"default:0"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// Template data (JSON)
	FieldMappings []FieldMapping `json:"fieldMappings,omitempty" gorm:"-"`
	RegexPatterns []RegexPattern `json:"regexPatterns,omitempty" gorm:"-"`
}

// ParsingRuleUsage tracks usage of parsing rules
type ParsingRuleUsage struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	ParsingRuleID uint      `json:"parsingRuleId" gorm:"not null;index"`
	LogFileID     uint      `json:"logFileId" gorm:"not null;index"`
	SuccessCount  int       `json:"successCount" gorm:"default:0"`
	FailureCount  int       `json:"failureCount" gorm:"default:0"`
	LastUsed      time.Time `json:"lastUsed"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`

	// Relationships
	ParsingRule *ParsingRule `json:"parsingRule,omitempty" gorm:"foreignKey:ParsingRuleID"`
	LogFile     *LogFile     `json:"logFile,omitempty" gorm:"foreignKey:LogFileID"`
}

// Table names
func (ParsingRule) TableName() string {
	return "parsing_rules"
}

func (FieldMapping) TableName() string {
	return "field_mappings"
}

func (RegexPattern) TableName() string {
	return "regex_patterns"
}

func (ParsingRuleTemplate) TableName() string {
	return "parsing_rule_templates"
}

func (ParsingRuleUsage) TableName() string {
	return "parsing_rule_usage"
}

// Validation methods
func (pr *ParsingRule) Validate() error {
	if pr.Name == "" {
		return fmt.Errorf("parsing rule name is required")
	}
	if pr.UserID == 0 {
		return fmt.Errorf("user ID is required")
	}
	return nil
}

func (fm *FieldMapping) Validate() error {
	if fm.SourceField == "" {
		return fmt.Errorf("source field is required")
	}
	if fm.TargetField == "" {
		return fmt.Errorf("target field is required")
	}
	if fm.ParsingRuleID == 0 {
		return fmt.Errorf("parsing rule ID is required")
	}
	return nil
}

func (rp *RegexPattern) Validate() error {
	if rp.Name == "" {
		return fmt.Errorf("regex pattern name is required")
	}
	if rp.Pattern == "" {
		return fmt.Errorf("regex pattern is required")
	}
	if rp.ParsingRuleID == 0 {
		return fmt.Errorf("parsing rule ID is required")
	}
	// Validate regex pattern
	if _, err := regexp.Compile(rp.Pattern); err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	return nil
}
