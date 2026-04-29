package repository

import (
	"context"

	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/gorm"
)

type ReservationRepository struct {
	db *gorm.DB
}

func NewReservationRepository(db *gorm.DB) *ReservationRepository {
	return &ReservationRepository{db: db}
}

func (r *ReservationRepository) Create(ctx context.Context, reservation *models.RoomReservation) error {
	return r.db.WithContext(ctx).Create(reservation).Error
}

func (r *ReservationRepository) CreateAttemptLog(ctx context.Context, log *models.ReservationAttemptLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *ReservationRepository) GetLatestByRoomID(ctx context.Context, roomID uint) (*models.RoomReservation, error) {
	var reservation models.RoomReservation
	if err := r.db.WithContext(ctx).Where("room_id = ?", roomID).Order("id DESC").First(&reservation).Error; err != nil {
		return nil, err
	}
	return &reservation, nil
}

func (r *ReservationRepository) GetByID(ctx context.Context, id uint) (*models.RoomReservation, error) {
	var reservation models.RoomReservation
	if err := r.db.WithContext(ctx).First(&reservation, id).Error; err != nil {
		return nil, err
	}
	return &reservation, nil
}

func (r *ReservationRepository) GetByPublicID(ctx context.Context, publicID string) (*models.RoomReservation, error) {
	var reservation models.RoomReservation
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).First(&reservation).Error; err != nil {
		return nil, err
	}
	return &reservation, nil
}
