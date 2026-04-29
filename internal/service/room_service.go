package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
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
	InviteCode string
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

func (s *RoomService) List(ctx context.Context, input ListRoomsInput) (*ListRoomsOutput, error) {
	return nil, fmt.Errorf("room service List not implemented")
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

func (s *RoomService) GetByID(ctx context.Context, id uint) (*models.Room, []models.RoomMember, error) {
	return nil, nil, fmt.Errorf("room service GetByID not implemented")
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
