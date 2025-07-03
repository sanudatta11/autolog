package models

import (
	"time"

	"gorm.io/gorm"
)

type UpdateType string

const (
	UpdateTypeComment        UpdateType = "COMMENT"
	UpdateTypeStatusChange   UpdateType = "STATUS_CHANGE"
	UpdateTypeAssignment     UpdateType = "ASSIGNMENT"
	UpdateTypePriorityChange UpdateType = "PRIORITY_CHANGE"
)

type IncidentUpdate struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	IncidentID uint           `json:"incidentId" gorm:"not null"`
	Incident   Incident       `json:"incident" gorm:"foreignKey:IncidentID"`
	UserID     uint           `json:"userId" gorm:"not null"`
	User       User           `json:"user" gorm:"foreignKey:UserID"`
	Content    string         `json:"content" gorm:"type:text;not null"`
	Type       UpdateType     `json:"type" gorm:"not null"`
	Metadata   *string        `json:"metadata" gorm:"type:jsonb"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

func (IncidentUpdate) TableName() string {
	return "incident_updates"
}
