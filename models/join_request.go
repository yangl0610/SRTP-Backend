package models

import (
	"time"

	"gorm.io/gorm"
)

type JoinRequest struct {
	ID         uint   `gorm:"primaryKey"`
	PublicID   string `gorm:"type:uuid;uniqueIndex;not null"`
	RoomID     uint   `gorm:"not null;index;index:idx_join_request_room_status,priority:1"`
	UserID     uint   `gorm:"not null;index;index:idx_join_request_user_status,priority:1"`
	Status     string `gorm:"size:32;not null;default:'pending';index;index:idx_join_request_room_status,priority:2;index:idx_join_request_user_status,priority:2"`
	Message    string `gorm:"size:255"`
	ReviewedBy *uint  `gorm:"index"`
	ReviewedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Room       Room  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:RoomID;references:ID"`
	User       User  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;foreignKey:UserID;references:ID"`
	Reviewer   *User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;foreignKey:ReviewedBy;references:ID"`
}

func (j *JoinRequest) BeforeCreate(_ *gorm.DB) error {
	ensurePublicID(&j.PublicID)
	return nil
}
