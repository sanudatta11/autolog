package models

import (
	"time"

	"gorm.io/gorm"
)

type IncidentStatus string
type IncidentPriority string
type IncidentSeverity string

const (
	StatusOpen       IncidentStatus = "OPEN"
	StatusInProgress IncidentStatus = "IN_PROGRESS"
	StatusResolved   IncidentStatus = "RESOLVED"
	StatusClosed     IncidentStatus = "CLOSED"
	StatusCancelled  IncidentStatus = "CANCELLED"
)

const (
	PriorityLow      IncidentPriority = "LOW"
	PriorityMedium   IncidentPriority = "MEDIUM"
	PriorityHigh     IncidentPriority = "HIGH"
	PriorityCritical IncidentPriority = "CRITICAL"
)

const (
	SeverityMinor    IncidentSeverity = "MINOR"
	SeverityModerate IncidentSeverity = "MODERATE"
	SeverityMajor    IncidentSeverity = "MAJOR"
	SeverityCritical IncidentSeverity = "CRITICAL"
)

type Incident struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	Title       string           `json:"title" gorm:"not null"`
	Description string           `json:"description" gorm:"type:text;not null"`
	Status      IncidentStatus   `json:"status" gorm:"not null;default:'OPEN'"`
	Priority    IncidentPriority `json:"priority" gorm:"not null"`
	Severity    IncidentSeverity `json:"severity" gorm:"not null"`
	AssigneeID  *uint            `json:"assigneeId"`
	Assignee    *User            `json:"assignee" gorm:"foreignKey:AssigneeID"`
	ReporterID  uint             `json:"reporterId" gorm:"not null"`
	Reporter    User             `json:"reporter" gorm:"foreignKey:ReporterID"`
	Tags        []string         `json:"tags" gorm:"type:text[]"`
	ResolvedAt  *time.Time       `json:"resolvedAt"`
	ClosedAt    *time.Time       `json:"closedAt"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (Incident) TableName() string {
	return "incidents"
}
