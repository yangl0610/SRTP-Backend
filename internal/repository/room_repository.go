package repository

import (
	"context"
	"time"

	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/gorm"
)

type RoomFilter struct {
	Keyword      *string
	SportType    *string
	Campus       *string
	Date         *time.Time
	TimeRange    *string
	Organization *string
	Level        *string
	Page         int
	PageSize     int
}

type RoomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

func (r *RoomRepository) Create(ctx context.Context, room *models.Room) error {
	return r.db.WithContext(ctx).Create(room).Error
}

func (r *RoomRepository) Update(ctx context.Context, room *models.Room) error {
	return r.db.WithContext(ctx).Save(room).Error
}

func (r *RoomRepository) GetByID(ctx context.Context, id uint) (*models.Room, error) {
	var room models.Room
	if err := r.db.WithContext(ctx).Preload("Owner").First(&room, id).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepository) GetByPublicID(ctx context.Context, publicID string) (*models.Room, error) {
	var room models.Room
	if err := r.db.WithContext(ctx).Preload("Owner").Where("public_id = ?", publicID).First(&room).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepository) GetByInviteCode(ctx context.Context, code string) (*models.Room, error) {
	var room models.Room
	if err := r.db.WithContext(ctx).Preload("Owner").Where("invite_code = ?", code).First(&room).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

type RoomListResult struct {
	Items []models.Room
	Total int64
}

func (r *RoomRepository) List(ctx context.Context, f RoomFilter) (*RoomListResult, error) {
	q := r.db.WithContext(ctx).Model(&models.Room{}).Preload("Owner").
		Where("visibility = ?", "public").
		Where("status IN ?", []string{"recruiting", "full", "ongoing"})

	if f.Keyword != nil && *f.Keyword != "" {
		q = q.Where("name ILIKE ? OR campus_name ILIKE ? OR venue_name ILIKE ? OR organization ILIKE ?", "%"+*f.Keyword+"%", "%"+*f.Keyword+"%", "%"+*f.Keyword+"%", "%"+*f.Keyword+"%")
	}
	if f.SportType != nil && *f.SportType != "" {
		q = q.Where("sport_type = ?", *f.SportType)
	}
	if f.Campus != nil && *f.Campus != "" {
		q = q.Where("campus_name ILIKE ?", "%"+*f.Campus+"%")
	}
	if f.Organization != nil && *f.Organization != "" {
		q = q.Where("organization ILIKE ?", "%"+*f.Organization+"%")
	}
	if f.Level != nil && *f.Level != "" {
		q = q.Where("level_desc ILIKE ?", "%"+*f.Level+"%")
	}
	if f.Date != nil && !f.Date.IsZero() {
		start := time.Date(f.Date.Year(), f.Date.Month(), f.Date.Day(), 0, 0, 0, 0, f.Date.Location())
		end := start.Add(24 * time.Hour)
		q = q.Where("start_time >= ? AND start_time < ?", start, end)
	}
	if f.TimeRange != nil && *f.TimeRange != "" {
		switch *f.TimeRange {
		case "morning":
			q = q.Where("EXTRACT(HOUR FROM start_time AT TIME ZONE 'Asia/Shanghai') BETWEEN 6 AND 11")
		case "afternoon":
			q = q.Where("EXTRACT(HOUR FROM start_time AT TIME ZONE 'Asia/Shanghai') BETWEEN 12 AND 17")
		case "evening":
			q = q.Where("EXTRACT(HOUR FROM start_time AT TIME ZONE 'Asia/Shanghai') BETWEEN 18 AND 23")
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	var items []models.Room
	if err := q.Order("start_time ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}

	return &RoomListResult{Items: items, Total: total}, nil
}

func (r *RoomRepository) ListRoomsByOwner(ctx context.Context, ownerID uint, page, pageSize int) (*RoomListResult, error) {
	q := r.db.WithContext(ctx).Model(&models.Room{}).Preload("Owner").Where("owner_id = ?", ownerID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	var items []models.Room
	if err := q.Order("start_time DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}

	return &RoomListResult{Items: items, Total: total}, nil
}

func (r *RoomRepository) ListRoomsJoinedByUser(ctx context.Context, userID uint, page, pageSize int) (*RoomListResult, error) {
	q := r.db.WithContext(ctx).Model(&models.Room{}).Preload("Owner").
		Joins("JOIN room_members ON room_members.room_id = rooms.id").
		Where("room_members.user_id = ? AND room_members.status = ?", userID, "joined")

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	var items []models.Room
	if err := q.Order("rooms.start_time DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}

	return &RoomListResult{Items: items, Total: total}, nil
}

func (r *RoomRepository) GetMembersByRoomID(ctx context.Context, roomID uint) ([]models.RoomMember, error) {
	var members []models.RoomMember
	if err := r.db.WithContext(ctx).Preload("User").Where("room_id = ?", roomID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *RoomRepository) GetMember(ctx context.Context, roomID, userID uint) (*models.RoomMember, error) {
	var member models.RoomMember
	if err := r.db.WithContext(ctx).Where("room_id = ? AND user_id = ?", roomID, userID).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *RoomRepository) CreateMember(ctx context.Context, member *models.RoomMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *RoomRepository) UpdateMember(ctx context.Context, member *models.RoomMember) error {
	return r.db.WithContext(ctx).Save(member).Error
}

func (r *RoomRepository) CreateJoinRequest(ctx context.Context, req *models.JoinRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

func (r *RoomRepository) GetJoinRequestByID(ctx context.Context, requestID uint) (*models.JoinRequest, error) {
	var req models.JoinRequest
	if err := r.db.WithContext(ctx).First(&req, requestID).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *RoomRepository) GetJoinRequestByPublicID(ctx context.Context, publicID string) (*models.JoinRequest, error) {
	var req models.JoinRequest
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).First(&req).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *RoomRepository) UpdateJoinRequest(ctx context.Context, req *models.JoinRequest) error {
	return r.db.WithContext(ctx).Save(req).Error
}

func (r *RoomRepository) CountActiveMembers(ctx context.Context, roomID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.RoomMember{}).Where("room_id = ? AND status = 'joined'", roomID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *RoomRepository) CountRoomsByOwner(ctx context.Context, ownerID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Room{}).Where("owner_id = ?", ownerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *RoomRepository) CountJoinedRoomsByUser(ctx context.Context, userID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.RoomMember{}).Where("user_id = ? AND status = ?", userID, "joined").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *RoomRepository) CountPendingJoinRequestsByUser(ctx context.Context, userID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.JoinRequest{}).Where("user_id = ? AND status = ?", userID, "pending").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
