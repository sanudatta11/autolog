package models

import (
	"time"

	"gorm.io/gorm"
)

type UserRole string

const (
	RoleAdmin     UserRole = "ADMIN"
	RoleManager   UserRole = "MANAGER"
	RoleResponder UserRole = "RESPONDER"
	RoleViewer    UserRole = "VIEWER"
)

type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Email     string         `json:"email" gorm:"uniqueIndex;not null"`
	Password  string         `json:"-" gorm:"not null"`
	FirstName string         `json:"firstName" gorm:"not null"`
	LastName  string         `json:"lastName" gorm:"not null"`
	Role      UserRole       `json:"role" gorm:"not null;default:'VIEWER'"`
	Avatar    *string        `json:"avatar"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (User) TableName() string {
	return "users"
}
