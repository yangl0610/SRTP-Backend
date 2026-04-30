package repository

import (
	"context"
	"fmt"
	"time"

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

// Update 全量保存预约记录（用于补全 slot 上下文或回写执行结果）。
func (r *ReservationRepository) Update(ctx context.Context, reservation *models.RoomReservation) error {
	return r.db.WithContext(ctx).Save(reservation).Error
}

// ListDueScheduled 返回所有 status=scheduled 且 reserve_open_at <= now 的计划，
// 供 materialize 调度器判断哪些计划已到开放时间。
func (r *ReservationRepository) ListDueScheduled(ctx context.Context, now time.Time) ([]*models.RoomReservation, error) {
	var reservations []*models.RoomReservation
	err := r.db.WithContext(ctx).
		Where("reservation_status = ? AND reserve_open_at <= ?", "scheduled", now).
		Find(&reservations).Error
	return reservations, err
}

// AtomicTransitionStatus 原子地将记录从 fromStatus 切换到 toStatus，防止并发双触发。
// 使用 public_id 定位记录，避免整数 ID 在内部流转。
// 返回 false 表示记录已被其他执行者抢占，调用方应直接放弃本次执行。
func (r *ReservationRepository) AtomicTransitionStatus(ctx context.Context, publicID, fromStatus, toStatus string) (bool, error) {
	result := r.db.WithContext(ctx).Model(&models.RoomReservation{}).
		Where("public_id = ? AND reservation_status = ?", publicID, fromStatus).
		Update("reservation_status", toStatus)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, fmt.Errorf("reservation %q not in status %q, may already be processing", publicID, fromStatus)
	}
	return true, nil
}
