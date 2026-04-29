package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID            uint   `gorm:"primaryKey"`
	PublicID      string `gorm:"type:uuid;uniqueIndex;not null"`
	AuthUID       string `gorm:"uniqueIndex;size:64;not null"`
	OpenID        string `gorm:"uniqueIndex;size:64"`
	Nickname      string `gorm:"size:30"`
	AvatarURL     string `gorm:"size:255"`
	Gender        string `gorm:"size:16"`
	Bio           string `gorm:"size:255"`
	ProfileStatus string `gorm:"size:32;not null;default:'pending'"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	ensurePublicID(&u.PublicID)
	return nil
}
