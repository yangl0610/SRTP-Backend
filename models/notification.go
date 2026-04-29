package models

import (
	"time"

	"gorm.io/gorm"
)

type Notification struct {
	ID          uint   `gorm:"primaryKey"`
	PublicID    string `gorm:"type:uuid;uniqueIndex;not null"`
	UserID      uint   `gorm:"not null;index;index:idx_notification_user_created,priority:1;index:idx_notification_user_status_created,priority:1"`
	Type        string `gorm:"size:32;not null;index"`
	Title       string `gorm:"size:100;not null"`
	Content     string `gorm:"size:500;not null"`
	Status      string `gorm:"size:32;not null;default:'pending';index;index:idx_notification_user_status_created,priority:2"`
	RelatedType string `gorm:"size:32"`
	RelatedID   *uint  `gorm:"index"`
	SentAt      *time.Time
	ReadAt      *time.Time
	CreatedAt   time.Time `gorm:"index:idx_notification_user_created,priority:2,sort:desc;index:idx_notification_user_status_created,priority:3,sort:desc"`
	UpdatedAt   time.Time
	User        User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:UserID;references:ID"`
}

func (n *Notification) BeforeCreate(_ *gorm.DB) error {
	ensurePublicID(&n.PublicID)
	return nil
}
