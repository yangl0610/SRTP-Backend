package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/api/gen"
	"github.com/QSCTech/SRTP-Backend/internal/service"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/QSCTech/SRTP-Backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"gorm.io/gorm"
)

type Handler struct {
	db                 *sql.DB
	userService        *service.UserService
	roomService        *service.RoomService
	reservationService *service.ReservationService
}

func NewHandler(db *sql.DB, userService *service.UserService, roomService *service.RoomService, reservationService *service.ReservationService) *Handler {
	return &Handler{db: db, userService: userService, roomService: roomService, reservationService: reservationService}
}

func (h *Handler) GetHealthz(c *gin.Context) {
	response.JSON(c, http.StatusOK, gen.HealthResponse{Service: "srtp-backend", Status: "ok"})
}

func (h *Handler) LoginWithWechat(c *gin.Context) {
	var req gen.WxLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	authUID := req.AuthUid
	if authUID == nil || *authUID == "" {
		fallback := "wx:" + req.Code
		authUID = &fallback
	}
	user, err := h.userService.LoginOrCreate(c.Request.Context(), *authUID, req.Code)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, buildUserResponse(user))
}

func (h *Handler) LogoutCurrentUser(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetReadyz(c *gin.Context) {
	if h.db == nil {
		response.Error(c, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		response.Error(c, http.StatusServiceUnavailable, "database down")
		return
	}
	response.JSON(c, http.StatusOK, gen.ReadyResponse{Database: "up", Status: "ready"})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req gen.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.userService.Create(c.Request.Context(), req.AuthUid)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, buildUserResponse(user))
}

func (h *Handler) GetUserById(c *gin.Context, id int64) {
	user, err := h.userService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "user not found" {
			response.Error(c, http.StatusNotFound, "user not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	response.JSON(c, http.StatusOK, buildUserResponse(user))
}

func (h *Handler) GetCurrentUser(c *gin.Context) {
	user, err := h.userService.GetCurrent(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusNotFound, "user not found")
		return
	}
	response.JSON(c, http.StatusOK, buildUserResponse(user))
}

func (h *Handler) UpdateCurrentUserProfile(c *gin.Context) {
	var req gen.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.userService.UpdateCurrentProfile(c.Request.Context(), service.UpdateProfileInput{
		Nickname:  req.Nickname,
		AvatarURL: req.AvatarUrl,
		Gender:    req.Gender,
		Bio:       req.Bio,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, buildUserResponse(user))
}

func (h *Handler) ListMyCreatedRooms(c *gin.Context, params gen.ListMyCreatedRoomsParams) {
	rooms, err := h.roomService.ListMineCreated(c.Request.Context(), optionalInt32(params.Page, 1), optionalInt32(params.PageSize, 20))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list created rooms")
		return
	}
	response.JSON(c, http.StatusOK, buildRoomCardPage(rooms))
}

func (h *Handler) ListMyJoinedRooms(c *gin.Context, params gen.ListMyJoinedRoomsParams) {
	rooms, err := h.roomService.ListMineJoined(c.Request.Context(), optionalInt32(params.Page, 1), optionalInt32(params.PageSize, 20))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list joined rooms")
		return
	}
	response.JSON(c, http.StatusOK, buildRoomCardPage(rooms))
}

func (h *Handler) GetMyStats(c *gin.Context) {
	stats, err := h.roomService.GetMyStats(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to get stats")
		return
	}
	response.JSON(c, http.StatusOK, gen.UserStatsResponse{
		CreatedRoomCount:        stats.CreatedRoomCount,
		JoinedRoomCount:         stats.JoinedRoomCount,
		PendingJoinRequestCount: stats.PendingJoinRequestCount,
	})
}

func (h *Handler) ListRooms(c *gin.Context, params gen.ListRoomsParams) {
	var date *time.Time
	if params.Date != nil {
		date = &params.Date.Time
	}
	rooms, err := h.roomService.List(c.Request.Context(), service.ListRoomsInput{
		Keyword:      params.Keyword,
		SportType:    params.SportType,
		Campus:       params.Campus,
		Date:         date,
		TimeRange:    params.TimeRange,
		Organization: params.Organization,
		Level:        params.Level,
		Page:         optionalInt32(params.Page, 1),
		PageSize:     optionalInt32(params.PageSize, 20),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list rooms")
		return
	}
	response.JSON(c, http.StatusOK, buildRoomCardPage(rooms))
}

func (h *Handler) CreateRoom(c *gin.Context) {
	var req gen.CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	room, err := h.roomService.Create(c.Request.Context(), service.CreateRoomInput{
		Name:            req.Name,
		SportType:       req.SportType,
		CampusName:      req.CampusName,
		VenueName:       req.VenueName,
		Visibility:      req.Visibility,
		JoinMode:        req.JoinMode,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		NeedReservation: req.NeedReservation != nil && *req.NeedReservation,
		GenderRule:      req.GenderRule,
		MemberLimit:     req.MemberLimit,
		Organization:    req.Organization,
		LevelDesc:       req.LevelDesc,
		Description:     req.Description,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	room, members, memberErr := h.roomService.GetByID(c.Request.Context(), room.ID)
	if memberErr != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch room")
		return
	}
	response.JSON(c, http.StatusCreated, h.buildRoomDetail(c.Request.Context(), room, members))
}

func (h *Handler) GetRoomById(c *gin.Context, roomId int64) {
	room, members, err := h.roomService.GetByID(c.Request.Context(), uint(roomId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "room not found" {
			response.Error(c, http.StatusNotFound, "room not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to fetch room")
		return
	}
	response.JSON(c, http.StatusOK, h.buildRoomDetail(c.Request.Context(), room, members))
}

func (h *Handler) UpdateRoom(c *gin.Context, roomId int64) {
	var req gen.UpdateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	room, err := h.roomService.Update(c.Request.Context(), uint(roomId), service.UpdateRoomInput{
		Name:            req.Name,
		Visibility:      req.Visibility,
		JoinMode:        req.JoinMode,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		NeedReservation: req.NeedReservation,
		GenderRule:      req.GenderRule,
		MemberLimit:     req.MemberLimit,
		Organization:    req.Organization,
		LevelDesc:       req.LevelDesc,
		Description:     req.Description,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	room, members, memberErr := h.roomService.GetByID(c.Request.Context(), room.ID)
	if memberErr != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch room")
		return
	}
	response.JSON(c, http.StatusOK, h.buildRoomDetail(c.Request.Context(), room, members))
}

func (h *Handler) CloseRoom(c *gin.Context, roomId int64) {
	room, err := h.roomService.Close(c.Request.Context(), uint(roomId))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	room, members, memberErr := h.roomService.GetByID(c.Request.Context(), room.ID)
	if memberErr != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch room")
		return
	}
	response.JSON(c, http.StatusOK, h.buildRoomDetail(c.Request.Context(), room, members))
}

func (h *Handler) JoinRoomByCode(c *gin.Context) {
	var req gen.JoinRoomByCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.roomService.JoinByCode(c.Request.Context(), service.JoinRoomByCodeInput{InviteCode: req.InviteCode})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, buildJoinRoomResult(result))
}

func (h *Handler) JoinRoomDirectly(c *gin.Context, roomId int64) {
	result, err := h.roomService.JoinDirectly(c.Request.Context(), uint(roomId))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, buildJoinRoomResult(result))
}

func (h *Handler) CreateJoinRequest(c *gin.Context, roomId int64) {
	var req gen.CreateJoinRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	joinRequest, err := h.roomService.CreateJoinRequest(c.Request.Context(), uint(roomId), service.CreateJoinRequestInput{Message: req.Message})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, gen.JoinRequestResponse{RequestId: int64(joinRequest.ID), Status: joinRequest.Status})
}

func (h *Handler) ApproveJoinRequest(c *gin.Context, roomId int64) {
	var req gen.ReviewJoinRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	joinRequest, err := h.roomService.ApproveJoinRequest(c.Request.Context(), uint(roomId), service.ReviewJoinRequestInput{RequestID: uint(req.RequestId)})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, gen.JoinRequestResponse{RequestId: int64(joinRequest.ID), Status: joinRequest.Status})
}

func (h *Handler) RejectJoinRequest(c *gin.Context, roomId int64) {
	var req gen.ReviewJoinRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	joinRequest, err := h.roomService.RejectJoinRequest(c.Request.Context(), uint(roomId), service.ReviewJoinRequestInput{RequestID: uint(req.RequestId)})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, gen.JoinRequestResponse{RequestId: int64(joinRequest.ID), Status: joinRequest.Status})
}

func (h *Handler) InviteRoomMember(c *gin.Context, roomId int64) {
	var req gen.InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	member, err := h.roomService.InviteMember(c.Request.Context(), uint(roomId), service.InviteMemberInput{UserID: uint(req.UserId)})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, gen.MemberActionResponse{RoomId: int64(member.RoomID), UserId: int64(member.UserID), Status: member.Status})
}

func (h *Handler) RemoveRoomMember(c *gin.Context, roomId int64, userId int64) {
	member, err := h.roomService.RemoveMember(c.Request.Context(), uint(roomId), uint(userId))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, gen.MemberActionResponse{RoomId: int64(member.RoomID), UserId: int64(member.UserID), Status: member.Status})
}

func (h *Handler) ListReservationVenues(c *gin.Context, params gen.ListReservationVenuesParams) {
	items := h.reservationService.ListVenues(c.Request.Context(), params.SportType, params.Campus)
	resp := gen.ReservationVenueListResponse{Items: make([]gen.ReservationVenue, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, gen.ReservationVenue{SportType: item.SportType, CampusName: item.CampusName, VenueName: item.VenueName})
	}
	response.JSON(c, http.StatusOK, resp)
}

func (h *Handler) ListReservationSlots(c *gin.Context, params gen.ListReservationSlotsParams) {
	items, err := h.reservationService.ListSlots(c.Request.Context(), params.SportType, params.CampusName, params.VenueName, params.ReservationDate.String())
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp := gen.ReservationSlotListResponse{Items: make([]gen.ReservationSlot, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, gen.ReservationSlot{SlotKey: item.SlotKey, StartTime: item.StartTime, EndTime: item.EndTime, Available: item.Available, SpaceName: item.SpaceName})
	}
	response.JSON(c, http.StatusOK, resp)
}

func (h *Handler) PreviewRoomReservation(c *gin.Context, roomId int64) {
	var req gen.ReservationSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	preview, err := h.reservationService.Preview(c.Request.Context(), buildReservationPreviewInput(uint(roomId), req))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	room, _, roomErr := h.roomService.GetByID(c.Request.Context(), uint(roomId))
	if roomErr != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch room")
		return
	}
	response.JSON(c, http.StatusOK, buildReservationPreviewResponse(preview, room.PublicID))
}

func (h *Handler) SubmitRoomReservation(c *gin.Context, roomId int64) {
	var req gen.ReservationSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	reservation, err := h.reservationService.Submit(c.Request.Context(), buildReservationPreviewInput(uint(roomId), req))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	room, _, roomErr := h.roomService.GetByID(c.Request.Context(), uint(roomId))
	if roomErr != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch room")
		return
	}
	response.JSON(c, http.StatusOK, buildReservationRecordResponse(reservation, room.PublicID))
}

func buildUserResponse(user *models.User) gen.UserResponse {
	return gen.UserResponse{Id: int64(user.ID), PublicId: mustParseUUID(user.PublicID), AuthUid: user.AuthUID, Nickname: user.Nickname, AvatarUrl: user.AvatarURL, Gender: user.Gender, Bio: user.Bio, ProfileStatus: user.ProfileStatus, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}

func buildRoomCardPage(rooms *service.ListRoomsOutput) gen.RoomCardPage {
	items := make([]gen.RoomCard, 0, len(rooms.Items))
	for _, item := range rooms.Items {
		items = append(items, buildRoomCard(item))
	}
	return gen.RoomCardPage{Page: rooms.Page, PageSize: rooms.PageSize, Total: rooms.Total, Items: items}
}

func buildRoomCard(item service.RoomCardItem) gen.RoomCard {
	room := item.Room
	return gen.RoomCard{
		Id:                 int64(room.ID),
		PublicId:           mustParseUUID(room.PublicID),
		Name:               room.Name,
		SportType:          room.SportType,
		CampusName:         room.CampusName,
		VenueName:          room.VenueName,
		StartTime:          room.StartTime,
		EndTime:            room.EndTime,
		OwnerNickname:      room.Owner.Nickname,
		OwnerAvatarUrl:     room.Owner.AvatarURL,
		CurrentMemberCount: item.CurrentMemberCount,
		MaxMemberCount:     int32Value(room.MemberLimit),
		JoinMode:           room.JoinMode,
		Visibility:         room.Visibility,
		Status:             room.Status,
		ReservationStatus:  room.ReservationStatus,
	}
}

func (h *Handler) buildRoomDetail(ctx context.Context, room *models.Room, members []models.RoomMember) gen.RoomDetail {
	memberItems := make([]gen.RoomMember, 0, len(members))
	for _, member := range members {
		memberItems = append(memberItems, gen.RoomMember{UserId: int64(member.UserID), UserPublicId: mustParseUUID(member.User.PublicID), Nickname: member.User.Nickname, AvatarUrl: member.User.AvatarURL, Role: member.Role, Status: member.Status})
	}
	currentUser, _ := h.userService.GetCurrent(ctx)
	isOwner := currentUser != nil && currentUser.ID == room.OwnerID
	return gen.RoomDetail{
		Id:                  int64(room.ID),
		PublicId:            mustParseUUID(room.PublicID),
		Name:                room.Name,
		SportType:           room.SportType,
		CampusName:          room.CampusName,
		VenueName:           room.VenueName,
		Visibility:          room.Visibility,
		JoinMode:            room.JoinMode,
		Status:              room.Status,
		ReservationStatus:   room.ReservationStatus,
		ReservationProvider: stringPtr(room.ReservationProvider),
		NeedReservation:     room.NeedReservation,
		StartTime:           room.StartTime,
		EndTime:             room.EndTime,
		GenderRule:          stringPtrOrNil(room.GenderRule),
		MemberLimit:         optionalInt(room.MemberLimit),
		Organization:        stringPtrOrNil(room.Organization),
		LevelDesc:           stringPtrOrNil(room.LevelDesc),
		Description:         stringPtrOrNil(room.Description),
		InviteCode:          stringPtrOrNil(room.InviteCode),
		Owner:               gen.RoomOwner{Id: int64(room.Owner.ID), PublicId: mustParseUUID(room.Owner.PublicID), Nickname: room.Owner.Nickname, AvatarUrl: room.Owner.AvatarURL},
		Members:             memberItems,
		CurrentMemberCount:  int32(countCurrentMembers(members)),
		IsOwner:             isOwner,
		Joinable:            room.Status == "recruiting",
	}
}

func buildJoinRoomResult(result *service.JoinRoomOutput) gen.JoinRoomResult {
	return gen.JoinRoomResult{RoomId: int64(result.RoomID), RoomPublicId: mustParseUUID(result.RoomPublicID), JoinResult: result.JoinResult, MemberStatus: result.MemberStatus, RequestStatus: result.RequestStatus}
}

func buildReservationPreviewInput(roomID uint, req gen.ReservationSubmitRequest) service.ReservationPreviewInput {
	return service.ReservationPreviewInput{
		RoomID:          roomID,
		SportType:       req.SportType,
		CampusName:      req.CampusName,
		VenueName:       req.VenueName,
		ReservationDate: req.ReservationDate.String(),
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		BuddyCode:       req.BuddyCode,
		VenueID:         int64PtrToUintPtr(req.VenueId),
		VenueSiteID:     int64PtrToUintPtr(req.VenueSiteId),
		SpaceID:         int64PtrToUintPtr(req.SpaceId),
		SpaceName:       req.SpaceName,
	}
}

func buildReservationPreviewResponse(preview *service.ReservationPreviewOutput, roomPublicID string) gen.ReservationPreviewResponse {
	return gen.ReservationPreviewResponse{
		RoomId:            int64(preview.RoomID),
		RoomPublicId:      mustParseUUID(roomPublicID),
		Provider:          preview.Provider,
		ReservationStatus: preview.ReservationStatus,
		SportType:         preview.SportType,
		CampusName:        preview.CampusName,
		VenueName:         preview.VenueName,
		ReservationDate:   openapi_types.Date{Time: mustParseDate(preview.ReservationDate)},
		StartTime:         preview.StartTime,
		EndTime:           preview.EndTime,
		BuddyCode:         preview.BuddyCode,
		VenueId:           uintPtrToInt64Ptr(preview.VenueID),
		VenueSiteId:       uintPtrToInt64Ptr(preview.VenueSiteID),
		SpaceId:           uintPtrToInt64Ptr(preview.SpaceID),
		SpaceName:         preview.SpaceName,
	}
}

func buildReservationRecordResponse(reservation *models.RoomReservation, roomPublicID string) gen.ReservationRecordResponse {
	return gen.ReservationRecordResponse{
		Id:                int64(reservation.ID),
		PublicId:          mustParseUUID(reservation.PublicID),
		RoomId:            int64(reservation.RoomID),
		RoomPublicId:      mustParseUUID(roomPublicID),
		Provider:          reservation.Provider,
		ReservationStatus: reservation.ReservationStatus,
		SportType:         reservation.SportType,
		CampusName:        reservation.CampusName,
		VenueName:         reservation.VenueName,
		ReservationDate:   openapi_types.Date{Time: mustParseDate(reservation.ReservationDate)},
		StartTime:         reservation.StartTime,
		EndTime:           reservation.EndTime,
		BuddyCode:         stringPtrOrNil(reservation.BuddyCode),
		VenueId:           uintPtrToInt64Ptr(reservation.VenueID),
		VenueSiteId:       uintPtrToInt64Ptr(reservation.VenueSiteID),
		SpaceId:           uintPtrToInt64Ptr(reservation.SpaceID),
		SpaceName:         stringPtrOrNil(reservation.SpaceName),
		ExternalOrderId:   stringPtrOrNil(reservation.ExternalOrderID),
		ExternalTradeNo:   stringPtrOrNil(reservation.ExternalTradeNo),
		CreatedAt:         reservation.CreatedAt,
		UpdatedAt:         reservation.UpdatedAt,
	}
}

func optionalInt(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func int32Value(value *int) int32 {
	if value == nil {
		return 0
	}
	return int32(*value)
}

func optionalInt32(value *int32, fallback int) int {
	if value == nil {
		return fallback
	}
	return int(*value)
}

func countCurrentMembers(members []models.RoomMember) int32 {
	var count int32
	for _, member := range members {
		if member.Status == "joined" {
			count++
		}
	}
	return count
}

func stringPtr(value string) *string {
	return &value
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}

func int64PtrToUintPtr(value *int64) *uint {
	if value == nil {
		return nil
	}
	v := uint(*value)
	return &v
}

func uintPtrToInt64Ptr(value *uint) *int64 {
	if value == nil {
		return nil
	}
	v := int64(*value)
	return &v
}

func mustParseDate(value string) time.Time {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func mustParseUUID(value string) openapi_types.UUID {
	if value == "" {
		return openapi_types.UUID{}
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return openapi_types.UUID{}
	}
	return openapi_types.UUID(parsed)
}
