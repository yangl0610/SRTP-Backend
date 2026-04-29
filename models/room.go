package models

import (
	"time"

	"gorm.io/gorm"
)

type Room struct {
	ID                  uint      `gorm:"primaryKey"`
	PublicID            string    `gorm:"type:uuid;uniqueIndex;not null"`
	OwnerID             uint      `gorm:"not null;index"`
	Name                string    `gorm:"size:32;not null"`
	SportType           string    `gorm:"size:32;not null;index"`
	CampusName          string    `gorm:"size:64;not null;index"`
	VenueName           string    `gorm:"size:128;not null"`
	Visibility          string    `gorm:"size:32;not null;default:'public'"`
	JoinMode            string    `gorm:"size:32;not null;default:'direct'"`
	Status              string    `gorm:"size:32;not null;default:'recruiting';index"`
	ReservationStatus   string    `gorm:"size:32;not null;default:'not_required';index"`
	ReservationProvider string    `gorm:"size:32;not null;default:'tyys'"`
	NeedReservation     bool      `gorm:"not null;default:false"`
	StartTime           time.Time `gorm:"not null;index"`
	EndTime             time.Time `gorm:"not null"`
	GenderRule          string    `gorm:"size:32"`
	MemberLimit         *int
	Organization        string `gorm:"size:64"`
	LevelDesc           string `gorm:"size:64"`
	Description         string `gorm:"size:500"`
	InviteCode          string `gorm:"uniqueIndex;size:16"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Owner               User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;foreignKey:OwnerID;references:ID"`
}

func (r *Room) BeforeCreate(_ *gorm.DB) error {
	ensurePublicID(&r.PublicID)
	return nil
}
