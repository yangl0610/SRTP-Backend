package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"

	"errors"
	"gorm.io/gorm"
)

type RoomService struct {
	repo        *repository.RoomRepository
	userService *UserService
}

func NewRoomService(repo *repository.RoomRepository, userService *UserService) *RoomService {
	return &RoomService{repo: repo, userService: userService}
}

type ListRoomsInput struct {
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

type CreateRoomInput struct {
	Name            string
	SportType       string
	CampusName      string
	VenueName       string
	Visibility      string
	JoinMode        string
	StartTime       time.Time
	EndTime         time.Time
	NeedReservation bool
	GenderRule      *string
	MemberLimit     *int32
	Organization    *string
	LevelDesc       *string
	Description     *string
}

type UpdateRoomInput struct {
	Name            *string
	Visibility      *string
	JoinMode        *string
	StartTime       *time.Time
	EndTime         *time.Time
	NeedReservation *bool
	GenderRule      *string
	MemberLimit     *int32
	Organization    *string
	LevelDesc       *string
	Description     *string
}

type JoinRoomByCodeInput struct {
	BuddyCode string
}

type CreateJoinRequestInput struct {
	Message string
}

type ReviewJoinRequestInput struct {
	RequestID uint
}

type InviteMemberInput struct {
	UserID uint
}

type RoomCardItem struct {
	Room               models.Room
	CurrentMemberCount int32
}

type ListRoomsOutput struct {
	Page     int32
	PageSize int32
	Total    int64
	Items    []RoomCardItem
}

type JoinRoomOutput struct {
	RoomID        uint
	RoomPublicID  string
	JoinResult    string
	MemberStatus  *string
	RequestStatus *string
}

type UserStatsOutput struct {
	CreatedRoomCount        int64
	JoinedRoomCount         int64
	PendingJoinRequestCount int64
}

/*基础功能：拿数据、加工成RoomCardItem*/
func (s *RoomService) List(ctx context.Context, input ListRoomsInput) (*ListRoomsOutput, error) {
	filter := repository.RoomFilter{
		Keyword:      input.Keyword,
		SportType:    input.SportType,
		Campus:       input.Campus,
		Date:         input.Date,
		TimeRange:    input.TimeRange,
		Organization: input.Organization,
		Level:        input.Level,
		Page:         int(input.Page),
		PageSize:     int(input.PageSize),
	}

	result, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	rooms := result.Items

	roomIDs := make([]uint, 0, len(rooms))
	for _, r := range rooms {
		roomIDs = append(roomIDs, r.ID)
	}

	countMap := make(map[uint]int64)
	if len(roomIDs) > 0 {
		countMap, err = s.repo.CountMembersByRoomIDs(ctx, roomIDs)
		if err != nil {
			return nil, err
		}
	}

	items := make([]RoomCardItem, 0, len(rooms))
	for _, r := range rooms {
		count := countMap[r.ID]

		items = append(items, RoomCardItem{
			Room:               r,
			CurrentMemberCount: int32(count),
		})
	}

	return &ListRoomsOutput{
		Page:     int32(input.Page),
		PageSize: int32(input.PageSize),
		Total:    result.Total,
		Items:    items,
	}, nil
}

func (s *RoomService) ListMineCreated(ctx context.Context, page, pageSize int) (*ListRoomsOutput, error) {
	return nil, fmt.Errorf("room service ListMineCreated not implemented")
}

func (s *RoomService) ListMineJoined(ctx context.Context, page, pageSize int) (*ListRoomsOutput, error) {
	return nil, fmt.Errorf("room service ListMineJoined not implemented")
}

func (s *RoomService) GetMyStats(ctx context.Context) (*UserStatsOutput, error) {
	return nil, fmt.Errorf("room service GetMyStats not implemented")
}

/*基础功能：拿数据、判断*/

func (s *RoomService) GetByPublicID(ctx context.Context, publicID string) (*models.Room, []models.RoomMember, error) {
    room, err := s.repo.GetByPublicID(ctx, publicID)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            var ErrRoomNotFound = errors.New("room not found")
            return nil, nil, ErrRoomNotFound
        }
        return nil, nil, err
    }

    members, err := s.repo.GetMembersByRoomID(ctx, room.ID)
    if err != nil {
        return nil, nil, err
    }

    return room, members, nil
}

func (s *RoomService) GetByID(ctx context.Context, id uint) (*models.Room, []models.RoomMember, error) {
    room, err := s.repo.GetByID(ctx, id)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            var ErrRoomNotFound = errors.New("room not found")
			return nil, nil, ErrRoomNotFound
        }
        return nil, nil, err
    }

    members, err := s.repo.GetMembersByRoomID(ctx, id)
    if err != nil {
        return nil, nil, err
    }

    return room, members, nil
}

func (s *RoomService) GetByPublicID(ctx context.Context, publicID string) (*models.Room, []models.RoomMember, error) {
	return nil, nil, fmt.Errorf("room service GetByPublicID not implemented")
}

func (s *RoomService) Create(ctx context.Context, input CreateRoomInput) (*models.Room, error) {
	return nil, fmt.Errorf("room service Create not implemented")
}

func (s *RoomService) Update(ctx context.Context, roomID uint, input UpdateRoomInput) (*models.Room, error) {
	return nil, fmt.Errorf("room service Update not implemented")
}

func (s *RoomService) Close(ctx context.Context, roomID uint) (*models.Room, error) {
	return nil, fmt.Errorf("room service Close not implemented")
}

func (s *RoomService) JoinByCode(ctx context.Context, input JoinRoomByCodeInput) (*JoinRoomOutput, error) {
	return nil, fmt.Errorf("room service JoinByCode not implemented")
}

func (s *RoomService) JoinDirectly(ctx context.Context, roomID uint) (*JoinRoomOutput, error) {
	return nil, fmt.Errorf("room service JoinDirectly not implemented")
}

func (s *RoomService) CreateJoinRequest(ctx context.Context, roomID uint, input CreateJoinRequestInput) (*models.JoinRequest, error) {
	return nil, fmt.Errorf("room service CreateJoinRequest not implemented")
}

func (s *RoomService) ApproveJoinRequest(ctx context.Context, roomID uint, input ReviewJoinRequestInput) (*models.JoinRequest, error) {
	return nil, fmt.Errorf("room service ApproveJoinRequest not implemented")
}

func (s *RoomService) RejectJoinRequest(ctx context.Context, roomID uint, input ReviewJoinRequestInput) (*models.JoinRequest, error) {
	return nil, fmt.Errorf("room service RejectJoinRequest not implemented")
}

func (s *RoomService) InviteMember(ctx context.Context, roomID uint, input InviteMemberInput) (*models.RoomMember, error) {
	return nil, fmt.Errorf("room service InviteMember not implemented")
}

func (s *RoomService) RemoveMember(ctx context.Context, roomID, userID uint) (*models.RoomMember, error) {
	return nil, fmt.Errorf("room service RemoveMember not implemented")
}
