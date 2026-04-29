package models

import (
	"time"

	"gorm.io/gorm"
)

type RoomReservation struct {
	ID                uint   `gorm:"primaryKey"`
	PublicID          string `gorm:"type:uuid;uniqueIndex;not null"`
	RoomID            uint   `gorm:"not null;index"`
	Provider          string `gorm:"size:32;not null;default:'tyys'"`
	SportType         string `gorm:"size:32;not null"`
	CampusName        string `gorm:"size:64;not null"`
	VenueName         string `gorm:"size:128;not null"`
	ReservationDate   string `gorm:"size:10;not null"`
	StartTime         string `gorm:"size:16;not null"`
	EndTime           string `gorm:"size:16;not null"`
	VenueID           *uint
	VenueSiteID       *uint
	SpaceID           *uint
	SpaceName         string `gorm:"size:64"`
	TimeID            *uint
	Token             string `gorm:"size:128"`
	WeekStartDate     string `gorm:"size:10"`
	BuddyCode         string `gorm:"size:32"`
	BuddyUserIDs      string `gorm:"type:text"`
	ReservationStatus string `gorm:"size:32;not null;default:'pending';index"`
	ScheduleStatus    string `gorm:"size:32;not null;default:'none';index"`
	ReserveOpenAt     *time.Time
	SubmitAttemptedAt *time.Time
	ExternalOrderID   string `gorm:"size:64"`
	ExternalTradeNo   string `gorm:"size:64"`
	RawResponse       string `gorm:"type:text"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Room              Room `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:RoomID;references:ID"`
}

func (r *RoomReservation) BeforeCreate(_ *gorm.DB) error {
	ensurePublicID(&r.PublicID)
	return nil
}
