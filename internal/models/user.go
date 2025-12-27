package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a registered user
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Email        string         `gorm:"uniqueIndex;size:100;not null" json:"email"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Accounts []Account `gorm:"foreignKey:UserID" json:"accounts,omitempty"`
}

// TableName specifies the table name for User model
func (User) TableName() string {
	return "users"
}
