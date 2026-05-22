package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/internal/zjulogin"
	"github.com/QSCTech/SRTP-Backend/models"
)

type ReservationVenueItem struct {
	SportType  string
	CampusName string
	VenueName  string
}

type ReservationSlotItem struct {
	SlotKey     string
	StartTime   string
	EndTime     string
	Available   bool
	CampusName  string
	VenueName   string
	VenueID     *uint
	VenueSiteID uint
	SpaceID     uint
	SpaceName   *string
	TimeID      uint
	Token       string
	WeekStart   string
}

// SlotSelection 是单个候选场地时间段，实时路径和计划路径共用。
// 包含提交 TYYS 所需的执行上下文，以及写回 record 的 campus/venue/start/end 信息。
type SlotSelection struct {
	CampusName  string
	VenueName   string
	VenueID     *uint
	VenueSiteID uint
	SpaceID     uint
	SpaceName   *string
	StartTime   string
	EndTime     string
	TimeID      uint
	Token       string
	WeekStart   string
}

// TemplateSlot 是场馆固定场次（space × timeslot），不含实时的 TimeID/Token。
type TemplateSlot struct {
	SpaceID     uint
	SpaceName   string
	VenueID     *uint
	VenueSiteID uint
	CampusName  string
	VenueName   string
	StartTime   string
	EndTime     string
	DisplayLabel string
}

// ReservationTemplateOutput 是场馆固定结构信息，不依赖 TYYS 实时查询窗口。
type ReservationTemplateOutput struct {
	SportType   string
	CampusName  string
	VenueName   string
	VenueID     *uint
	VenueSiteID *uint
	Slots       []TemplateSlot
}

type ReservationPreviewInput struct {
	RoomID          uint
	SportType       string
	ReservationDate string
	BuddyCode       *string
	// Slots 是前端从 /reservations/slots 中选取的候选场地列表，campus/venue/start/end 均在每个 slot 中携带。
	Slots []SlotSelection
}

// PlanSlotSelection 是用户创建远期计划时指定的首选场次，供调度器在预约窗口期补全上下文。
// 与 SlotSelection 相比，不含需要 materialize 时才能确定的 TimeID/Token/WeekStart。
// CampusName/VenueName/StartTime/EndTime 若为空则继承计划顶层字段，允许跨场馆多选。
type PlanSlotSelection struct {
	CampusName  string
	VenueName   string
	VenueID     *uint
	VenueSiteID *uint
	SpaceID     uint
	SpaceName   *string
	StartTime   string
	EndTime     string
}

// ReservationPlanInput 仅含预约意图，不需要实时 slot 上下文。
// campus/venue/start/end 均在 PlanSlots 中按场次携带，顶层只保留跨 slot 共有的字段。
type ReservationPlanInput struct {
	RoomID          uint
	SportType       string
	ReservationDate string
	BuddyCode       *string
	// PlanSlots 是用户指定的首选场次列表，每个 slot 携带完整的 campus/venue/start/end 信息。
	PlanSlots []PlanSlotSelection
}

// SlotPreviewItem 是单个 slot 经 TYYS orderInfo 校验后的结果。
type SlotPreviewItem struct {
	Slot      SlotSelection
	Available bool
	Error     string
}

type ReservationPreviewOutput struct {
	RoomID          uint
	Provider        string
	SportType       string
	ReservationDate string
	BuddyCode       *string
	Slots           []SlotPreviewItem
}

// MaterializeResult 是 materialize 批量执行的汇总结果。
type MaterializeResult struct {
	Total     int
	Succeeded int
	Failed    int
	Expired   int
	Errors    []string
}

type venueInfoCache struct {
	mu        sync.Mutex
	data      json.RawMessage
	expiresAt time.Time
}

const venueInfoCacheTTL = 15 * time.Minute

type ReservationService struct {
	roomRepo        *repository.RoomRepository
	reservationRepo *repository.ReservationRepository
	tyys            *zjulogin.TYYS
	venueCache      venueInfoCache
}

func NewReservationService(roomRepo *repository.RoomRepository, reservationRepo *repository.ReservationRepository, tyys *zjulogin.TYYS) *ReservationService {
	return &ReservationService{roomRepo: roomRepo, reservationRepo: reservationRepo, tyys: tyys}
}

// cachedVenueInfo 返回带 TTL 缓存的 VenueInfo 数据，避免每次请求都打 TYYS。
func (s *ReservationService) cachedVenueInfo(ctx context.Context) (json.RawMessage, error) {
	s.venueCache.mu.Lock()
	defer s.venueCache.mu.Unlock()

	if s.venueCache.data != nil && time.Now().Before(s.venueCache.expiresAt) {
		return s.venueCache.data, nil
	}
	resp, err := s.tyys.VenueInfo(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("get venue info: %w", err)
	}
	s.venueCache.data = resp.Data
	s.venueCache.expiresAt = time.Now().Add(venueInfoCacheTTL)
	return s.venueCache.data, nil
}

func (s *ReservationService) ListVenues(ctx context.Context, sportType, campus *string) []ReservationVenueItem {
	data, err := s.cachedVenueInfo(ctx)
	if err != nil {
		return nil
	}

	var result []ReservationVenueItem
	walkVenues(data, func(obj map[string]any) {
		sport := trimString(obj["sportName"])
		camp := trimString(obj["campusName"])
		venue := trimString(obj["venueName"])

		if sport == "" || camp == "" || venue == "" {
			return
		}
		if sportType != nil && *sportType != "" && sport != *sportType {
			return
		}
		if campus != nil && *campus != "" && camp != *campus {
			return
		}
		result = append(result, ReservationVenueItem{
			SportType:  sport,
			CampusName: camp,
			VenueName:  venue,
		})
	})
	return result
}

// trimString converts an any value to string, returning empty string if not a string.
func trimString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatFloat(val, 'f', 0, 64)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	}
	return ""
}

// walkVenues parses TYYS venue data and visits each venue object that has sportName field.
// It recursively walks through the JSON data structure to find all venue objects.
func walkVenues(data []byte, visit func(map[string]any)) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}
	walkJSONObjects(payload, func(obj map[string]any) {
		if _, ok := obj["sportName"]; ok {
			visit(obj)
		}
	})
}

// walkJSONObjects recursively walks a parsed JSON structure and calls visit for each object.
// It handles both maps and arrays, drilling down into nested structures.
func walkJSONObjects(value any, visit func(map[string]any)) {
	switch typed := value.(type) {
	case map[string]any:
		visit(typed)
		for _, child := range typed {
			walkJSONObjects(child, visit)
		}
	case []any:
		for _, child := range typed {
			walkJSONObjects(child, visit)
		}
	}
}

// textMatches is a flexible string matcher for reservation fields.
// It returns true if want is empty or if got contains want or equals want.
func textMatches(got, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	got = strings.TrimSpace(got)
	return got == want || strings.Contains(got, want) || strings.Contains(want, got)
}

func (s *ReservationService) ListSlots(ctx context.Context, sportType, campusName, venueName, reservationDate string) ([]ReservationSlotItem, error) {
	// Step 1: 从 VenueInfo 找到 venueId / venueSiteId（使用缓存，避免重复打 TYYS）。
	venueData, err := s.cachedVenueInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get venue info: %w", err)
	}

	var venueIDStr, venueSiteIDStr string
	var resolvedCampus, resolvedVenue string
	walkVenues(venueData, func(obj map[string]any) {
		sportGot := trimString(obj["sportName"])
		campusGot := trimString(obj["campusName"])
		venueGot := trimString(obj["venueName"])
		if !textMatches(sportGot, sportType) || !textMatches(campusGot, campusName) || !textMatches(venueGot, venueName) {
			return
		}
		if venueIDStr == "" {
			venueIDStr = trimString(obj["venueId"])
			venueSiteIDStr = trimString(obj["id"])
			resolvedCampus = campusGot
			resolvedVenue = venueGot
		}
	})

	if venueIDStr == "" || venueSiteIDStr == "" {
		return nil, fmt.Errorf("venue not found for sport=%s campus=%s venue=%s", sportType, campusName, venueName)
	}

	venueIDUint := parseUint(venueIDStr)
	venueSiteIDUint := parseUint(venueSiteIDStr)

	// Step 2: 若预约窗口尚未开放，跳过实时 DayInfo 直接返回 template，避免无意义的 TYYS 请求。
	// reserveOpenAt 用最早开放时间（按校区+球类），确保保守判断。
	if openAt, oaErr := reserveOpenAt(reservationDate, campusName, sportType); oaErr == nil && time.Now().Before(openAt) {
		return s.listSlotsFromTemplate(ctx, resolvedCampus, resolvedVenue, venueIDUint, venueSiteIDUint, nil), nil
	}

	// Step 3: 尝试从 TYYS 获取实时 dayInfo。
	params := url.Values{}
	params.Set("venueId", venueIDStr)
	params.Set("venueSiteId", venueSiteIDStr)
	params.Set("siteId", venueSiteIDStr)
	params.Set("date", reservationDate)
	params.Set("reservationDate", reservationDate)
	params.Set("searchDate", reservationDate)

	dayResp, dayErr := s.tyys.ReservationDayInfo(ctx, params)

	// Step 4: 若 dayInfo 失败（预约窗口未开放），降级为 template 数据，标记 available=false。
	if dayErr != nil || dayResp == nil {
		return s.listSlotsFromTemplate(ctx, resolvedCampus, resolvedVenue, venueIDUint, venueSiteIDUint, nil), nil
	}

	// Step 5: 解析 dayInfo，提取顶层 token/weekStartDate。
	var topToken, topWeekStart string
	var topObj map[string]any
	if unmarshalErr := json.Unmarshal(dayResp.Data, &topObj); unmarshalErr == nil {
		topToken = trimString(topObj["token"])
		topWeekStart = trimString(topObj["weekStartDate"])
	}

	// Step 6: 按 slot 构建完整上下文。
	var slots []ReservationSlotItem
	walkSlots(dayResp.Data, func(slot map[string]any) {
		spaceID := parseUint(trimString(slot["spaceId"]))
		item := ReservationSlotItem{
			SlotKey:     trimString(slot["timeId"]),
			StartTime:   trimString(slot["startDate"]),
			EndTime:     trimString(slot["endDate"]),
			Available:   isSlotAvailable(slot),
			CampusName:  resolvedCampus,
			VenueName:   resolvedVenue,
			VenueID:     &venueIDUint,
			VenueSiteID: venueSiteIDUint,
			SpaceID:     spaceID,
			TimeID:      parseUint(trimString(slot["timeId"])),
			Token:       coalesce(trimString(slot["token"]), topToken),
			WeekStart:   coalesce(trimString(slot["weekStartDate"]), topWeekStart),
		}
		if name := trimString(slot["spaceName"]); name != "" {
			item.SpaceName = &name
		}
		slots = append(slots, item)
	})

	return slots, nil
}

// listSlotsFromTemplate 在 TYYS 预约窗口未开放时，基于 template 数据返回时间段骨架。
// 所有 slot 标记 available=false，token/time_id 为空，供前端构建计划预约的首选场次。
func (s *ReservationService) listSlotsFromTemplate(ctx context.Context, campusName, venueName string, venueID, venueSiteID uint, _ map[uint]string) []ReservationSlotItem {
	tmpl, err := s.ListTemplates(ctx, "", campusName, venueName)
	if err != nil || tmpl == nil {
		return nil
	}
	var slots []ReservationSlotItem
	for _, ts := range tmpl.Slots {
		vid := venueID
		if ts.VenueID != nil {
			vid = *ts.VenueID
		}
		sid := venueSiteID
		if ts.VenueSiteID != 0 {
			sid = ts.VenueSiteID
		}
		item := ReservationSlotItem{
			Available:   false,
			CampusName:  ts.CampusName,
			VenueName:   ts.VenueName,
			VenueID:     &vid,
			VenueSiteID: sid,
			SpaceID:     ts.SpaceID,
			StartTime:   ts.StartTime,
			EndTime:     ts.EndTime,
		}
		if ts.SpaceName != "" {
			sn := ts.SpaceName
			item.SpaceName = &sn
		}
		slots = append(slots, item)
	}
	return slots
}

// isSlotAvailable checks if a slot is available for booking.
func isSlotAvailable(slot map[string]any) bool {
	if status, ok := slot["reservationStatus"].(float64); ok && status != 1 {
		return false
	}
	if count, ok := slot["alreadyNum"].(float64); ok && count > 0 {
		return false
	}
	if tradeNo := trimString(slot["tradeNo"]); tradeNo != "" && tradeNo != "null" {
		return false
	}
	return true
}

// walkSlots walks through parsed TYYS dayInfo JSON and visits each slot with its space context.
//
// TYYS dayInfo structure: space objects contain child objects keyed by timeId, where each
// child has startDate/endDate. spaceId comes from the parent's "id" field; timeId is the key.
// This function merges spaceId/spaceName/timeId into each slot before calling visit.
func walkSlots(data []byte, visit func(map[string]any)) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}
	walkSlotsAny(payload, visit)
}

func walkSlotsAny(v any, visit func(map[string]any)) {
	switch node := v.(type) {
	case map[string]any:
		spaceID := trimString(node["id"])
		spaceName := trimString(node["spaceName"])
		foundSlot := false
		if spaceID != "" {
			for key, val := range node {
				child, ok := val.(map[string]any)
				if !ok {
					continue
				}
				if _, hasStart := child["startDate"]; !hasStart {
					continue
				}
				// merge space context into slot
				slot := make(map[string]any, len(child)+3)
				for k, v := range child {
					slot[k] = v
				}
				slot["spaceId"] = spaceID
				slot["spaceName"] = spaceName
				slot["timeId"] = key
				visit(slot)
				foundSlot = true
			}
		}
		if !foundSlot {
			for _, child := range node {
				walkSlotsAny(child, visit)
			}
		}
	case []any:
		for _, item := range node {
			walkSlotsAny(item, visit)
		}
	}
}

// venueOpenHour 返回指定校区+球类的 TYYS 预约开放小时（Asia/Shanghai）。
// 依据公共体育与艺术部 2024-10-17 公告，其余一律 09:00：
//   - 紫金港 + 网球:    08:00
//   - 玉泉   + 羽毛球:  12:00
//   - 华家池 + 羽毛球:  18:00
//   - 华家池 + 网球:    18:00（膜顶网球场）
//
// 注意：创建预约计划时尚不知道具体分场（spaceName），因此只能按校区+球类推断开放时间。
// 同一校区+球类下可能存在多个场馆，开放时间以该类型场馆中最早的为准。
// 如果后续需要按具体场馆细化，务必同时调整 reserveOpenAt 的调用方也传入 venueName。
func venueOpenHour(campusName, sportType string) int {
	switch {
	case strings.Contains(campusName, "紫金港") && strings.Contains(sportType, "网球"):
		return 8
	case strings.Contains(campusName, "玉泉") && strings.Contains(sportType, "羽毛球"):
		return 12
	case strings.Contains(campusName, "华家池") && strings.Contains(sportType, "羽毛球"):
		return 18
	case strings.Contains(campusName, "华家池") && strings.Contains(sportType, "网球"):
		return 18
	default:
		return 9
	}
}

// reserveOpenAt 计算给定预约日期、校区、球类对应的 TYYS 开放时间点（预约日期前2天）。
func reserveOpenAt(reservationDate, campusName, sportType string) (time.Time, error) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.UTC
	}
	date, err := time.ParseInLocation("2006-01-02", reservationDate, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid reservation_date %q: %w", reservationDate, err)
	}
	openDate := date.AddDate(0, 0, -2)
	hour := venueOpenHour(campusName, sportType)
	return time.Date(openDate.Year(), openDate.Month(), openDate.Day(), hour, 0, 0, 0, loc), nil
}

// ReservationSubmitOrPlanResult 是 SubmitOrPlan 的返回值，标明实际走了哪条路径。
type ReservationSubmitOrPlanResult struct {
	Record  *models.RoomReservation
	Planned bool // true=创建了计划，false=直接提交
}

// SubmitOrPlan 统一预约入口：对开放窗口内的 slot 依次尝试实时提交，全部失败（或无窗口内 slot）时，
// 仅将窗口尚未开放的 slot 降级为远期计划。已开放但提交失败的 slot 不进计划（场地可能已满或不可预约）。
func (s *ReservationService) SubmitOrPlan(ctx context.Context, input ReservationPreviewInput) (*ReservationSubmitOrPlanResult, error) {
	if len(input.Slots) == 0 {
		return nil, fmt.Errorf("submit or plan requires at least one slot selection")
	}

	now := time.Now()
	var openSlots, closedSlots []SlotSelection
	for _, slot := range input.Slots {
		openAt, err := reserveOpenAt(input.ReservationDate, slot.CampusName, input.SportType)
		if err != nil {
			continue
		}
		if now.Before(openAt) {
			closedSlots = append(closedSlots, slot)
		} else {
			openSlots = append(openSlots, slot)
		}
	}

	if len(openSlots) > 0 {
		record, err := s.Submit(ctx, ReservationPreviewInput{
			RoomID:          input.RoomID,
			SportType:       input.SportType,
			ReservationDate: input.ReservationDate,
			BuddyCode:       input.BuddyCode,
			Slots:           openSlots,
		})
		if err == nil && record.ReservationStatus == "success" {
			return &ReservationSubmitOrPlanResult{Record: record, Planned: false}, nil
		}
		// 实时提交失败：已开放的 slot 不落入计划，仅用 closedSlots 降级。
	}

	if len(closedSlots) == 0 {
		return nil, fmt.Errorf("real-time submission failed and no future slots available to plan")
	}

	planSlots := make([]PlanSlotSelection, 0, len(closedSlots))
	for _, sl := range closedSlots {
		siteID := sl.VenueSiteID
		planSlots = append(planSlots, PlanSlotSelection{
			CampusName:  sl.CampusName,
			VenueName:   sl.VenueName,
			VenueID:     sl.VenueID,
			VenueSiteID: &siteID,
			SpaceID:     sl.SpaceID,
			SpaceName:   sl.SpaceName,
			StartTime:   sl.StartTime,
			EndTime:     sl.EndTime,
		})
	}
	record, err := s.CreatePlan(ctx, ReservationPlanInput{
		RoomID:          input.RoomID,
		SportType:       input.SportType,
		ReservationDate: input.ReservationDate,
		BuddyCode:       input.BuddyCode,
		PlanSlots:       planSlots,
	})
	if err != nil {
		return nil, err
	}
	return &ReservationSubmitOrPlanResult{Record: record, Planned: true}, nil
}

// ListTemplates 查询场馆固定场次列表，不含实时的 TimeID/Token。
// 用明日日期调 ListSlots，剥离实时字段后返回。
func (s *ReservationService) ListTemplates(ctx context.Context, sportType, campusName, venueName string) (*ReservationTemplateOutput, error) {
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	items, err := s.ListSlots(ctx, sportType, campusName, venueName, tomorrow)
	if err != nil {
		return nil, err
	}

	out := &ReservationTemplateOutput{
		SportType:  sportType,
		CampusName: campusName,
		VenueName:  venueName,
	}
	if len(items) > 0 {
		out.CampusName = items[0].CampusName
		out.VenueName = items[0].VenueName
		out.VenueID = items[0].VenueID
		if items[0].VenueSiteID != 0 {
			v := items[0].VenueSiteID
			out.VenueSiteID = &v
		}
	}

	for _, item := range items {
		out.Slots = append(out.Slots, TemplateSlot{
			SpaceID:      item.SpaceID,
			SpaceName:    stringVal(item.SpaceName),
			VenueID:      item.VenueID,
			VenueSiteID:  item.VenueSiteID,
			CampusName:   item.CampusName,
			VenueName:    item.VenueName,
			StartTime:    formatTimeOnly(item.StartTime),
			EndTime:      formatTimeOnly(item.EndTime),
			DisplayLabel: formatTimeOnly(item.StartTime) + "-" + formatTimeOnly(item.EndTime),
		})
	}

	return out, nil
}

// CreatePlan 创建远期预约计划，只保存预约意图，不立即调 TYYS。
// 计划状态为 scheduled，reserve_open_at 根据首个 PlanSlot 的校区+球类自动计算。
func (s *ReservationService) CreatePlan(ctx context.Context, input ReservationPlanInput) (*models.RoomReservation, error) {
	if len(input.PlanSlots) == 0 {
		return nil, fmt.Errorf("plan requires at least one slot selection")
	}
	// 取所有场次中最早的预约开放时间，确保调度器能在第一个窗口开放时触发。
	var openAt time.Time
	for _, ps := range input.PlanSlots {
		t, err := reserveOpenAt(input.ReservationDate, ps.CampusName, input.SportType)
		if err != nil {
			return nil, err
		}
		if openAt.IsZero() || t.Before(openAt) {
			openAt = t
		}
	}

	planSlotsJSON := marshalPlanSlots(input.PlanSlots)
	record := &models.RoomReservation{
		RoomID:            input.RoomID,
		Provider:          "tyys",
		SportType:         input.SportType,
		ReservationDate:   input.ReservationDate,
		// CampusName/VenueName/StartTime/EndTime 在 materializeOne 成功后由 trySlots 写入。
		BuddyCode:         stringVal(input.BuddyCode),
		PlanSlots:         planSlotsJSON,
		ReservationStatus: "scheduled",
		ReserveOpenAt:     &openAt,
	}
	if err := s.reservationRepo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("create plan: %w", err)
	}
	return record, nil
}

// Preview 对每个候选 slot 并发调用 TYYS orderInfo 做预校验，不创建 DB 记录。
// 返回全量 slot 校验结果，前端可据此展示哪些场地可预约并让用户二次确认。
// 并发数限制为 3，避免对 TYYS 造成压力。
func (s *ReservationService) Preview(ctx context.Context, input ReservationPreviewInput) (*ReservationPreviewOutput, error) {
	if len(input.Slots) == 0 {
		return nil, fmt.Errorf("preview requires at least one slot selection")
	}

	out := &ReservationPreviewOutput{
		RoomID:          input.RoomID,
		Provider:        "tyys",
		SportType:       input.SportType,
		ReservationDate: input.ReservationDate,
		BuddyCode:       input.BuddyCode,
	}

	now := time.Now()
	items := make([]SlotPreviewItem, len(input.Slots))
	const maxConcurrent = 3
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, slot := range input.Slots {
		i, slot := i, slot
		wg.Add(1)
		go func() {
			defer wg.Done()
			openAt, err := reserveOpenAt(input.ReservationDate, slot.CampusName, input.SportType)
			if err == nil && now.Before(openAt) {
				items[i] = SlotPreviewItem{
					Slot:      slot,
					Available: false,
					Error:     fmt.Sprintf("reservation window not open until %s", openAt.Format("2006-01-02 15:04")),
				}
				return
			}
			sem <- struct{}{}
			defer func() { <-sem }()

			form := url.Values{}
			form.Set("venueSiteId", strconv.FormatUint(uint64(slot.VenueSiteID), 10))
			form.Set("reservationDate", input.ReservationDate)
			form.Set("weekStartDate", coalesce(slot.WeekStart, input.ReservationDate))
			form.Set("token", slot.Token)
			form.Set("reservationOrderJson", mustMarshalJSON([]map[string]any{{
				"spaceId":           strconv.FormatUint(uint64(slot.SpaceID), 10),
				"timeId":            strconv.FormatUint(uint64(slot.TimeID), 10),
				"venueSpaceGroupId": nil,
			}}))
			_, tyysErr := s.tyys.ReservationOrderInfo(ctx, form)
			item := SlotPreviewItem{Slot: slot, Available: tyysErr == nil}
			if tyysErr != nil {
				item.Error = tyysErr.Error()
			}
			items[i] = item
		}()
	}

	wg.Wait()
	out.Slots = items
	return out, nil
}

// Submit 创建预约记录并通过 trySlots 依次尝试候选场地提交 TYYS（实时路径 ≤2天）。
// 前端从 /reservations/slots 选取候选列表后传入，服务端自动选第一个成功的场地。
func (s *ReservationService) Submit(ctx context.Context, input ReservationPreviewInput) (*models.RoomReservation, error) {
	if len(input.Slots) == 0 {
		return nil, fmt.Errorf("submit requires at least one slot selection")
	}

	first := input.Slots[0]
	now := time.Now()
	venueSiteID, spaceID, timeID := first.VenueSiteID, first.SpaceID, first.TimeID
	record := &models.RoomReservation{
		RoomID:            input.RoomID,
		Provider:          "tyys",
		SportType:         input.SportType,
		CampusName:        first.CampusName,
		VenueName:         first.VenueName,
		ReservationDate:   input.ReservationDate,
		StartTime:         first.StartTime,
		EndTime:           first.EndTime,
		VenueID:           first.VenueID,
		VenueSiteID:       &venueSiteID,
		SpaceID:           &spaceID,
		SpaceName:         stringVal(first.SpaceName),
		TimeID:            &timeID,
		Token:             first.Token,
		WeekStartDate:     first.WeekStart,
		BuddyCode:         stringVal(input.BuddyCode),
		ReservationStatus: "submitting",
		SubmitAttemptedAt: &now,
	}
	if err := s.reservationRepo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("create reservation record: %w", err)
	}

	_ = s.trySlots(ctx, record, input.Slots)
	return record, nil
}

// trySlots 是实时路径和计划路径共用的 slot 执行引擎。
// 依次尝试每个候选 slot，首个成功即返回 nil；每次尝试前将 slot 的全部上下文（含 campus/venue/start/end）写入 record 并同步到 DB。
func (s *ReservationService) trySlots(ctx context.Context, record *models.RoomReservation, slots []SlotSelection) error {
	var lastErr error
	for _, slot := range slots {
		venueSiteID, spaceID, timeID := slot.VenueSiteID, slot.SpaceID, slot.TimeID
		record.VenueSiteID = &venueSiteID
		record.SpaceID = &spaceID
		record.TimeID = &timeID
		record.Token = slot.Token
		record.WeekStartDate = slot.WeekStart
		record.SpaceName = stringVal(slot.SpaceName)
		if slot.CampusName != "" {
			record.CampusName = slot.CampusName
		}
		if slot.VenueName != "" {
			record.VenueName = slot.VenueName
		}
		if slot.StartTime != "" {
			record.StartTime = slot.StartTime
		}
		if slot.EndTime != "" {
			record.EndTime = slot.EndTime
		}
		if slot.VenueID != nil {
			record.VenueID = slot.VenueID
		}
		if err := s.reservationRepo.Update(ctx, record); err != nil {
			lastErr = err
			continue
		}
		if err := s.executeReservation(ctx, record); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// resolvePreferredSlots 仅供计划路径（materializeOne）使用。
// 按 PlanSlots 中的 (campus, venue) 分组调用 ListSlots，过滤 space_id 后返回候选列表。
// 不按时间过滤；trySlots 会依次尝试所有候选 slot。
func (s *ReservationService) resolvePreferredSlots(ctx context.Context, plan *models.RoomReservation) ([]SlotSelection, error) {
	planSlots := unmarshalPlanSlots(plan.PlanSlots)
	if len(planSlots) == 0 {
		return nil, fmt.Errorf("plan has no slot selections")
	}

	type venueKey struct{ campus, venue string }
	byVenue := make(map[venueKey][]PlanSlotSelection)
	for _, ps := range planSlots {
		k := venueKey{ps.CampusName, ps.VenueName}
		byVenue[k] = append(byVenue[k], ps)
	}

	var candidates []SlotSelection
	for key, group := range byVenue {
		liveSlots, err := s.ListSlots(ctx, plan.SportType, key.campus, key.venue, plan.ReservationDate)
		if err != nil {
			continue // 该场馆暂不可查，跳过
		}
		// 同一 space 可能有多个时段，用 []PlanSlotSelection 保留全部意图
		spaceMap := make(map[uint][]PlanSlotSelection, len(group))
		for _, ps := range group {
			spaceMap[ps.SpaceID] = append(spaceMap[ps.SpaceID], ps)
		}
		for _, live := range liveSlots {
			if !live.Available {
				continue
			}
			preferred, ok := spaceMap[live.SpaceID]
			if !ok {
				continue
			}
			// 按时间段匹配：live slot 的 start/end 必须与某条意图一致
			for _, ps := range preferred {
				wantStart := formatTimeOnly(ps.StartTime)
				wantEnd := formatTimeOnly(ps.EndTime)
				if wantStart != "" && wantStart != formatTimeOnly(live.StartTime) {
					continue
				}
				if wantEnd != "" && wantEnd != formatTimeOnly(live.EndTime) {
					continue
				}
				candidates = append(candidates, SlotSelection{
					CampusName:  key.campus,
					VenueName:   key.venue,
					VenueID:     ps.VenueID,
					VenueSiteID: live.VenueSiteID,
					SpaceID:     live.SpaceID,
					SpaceName:   live.SpaceName,
					StartTime:   live.StartTime,
					EndTime:     live.EndTime,
					TimeID:      live.TimeID,
					Token:       live.Token,
					WeekStart:   live.WeekStart,
				})
				break // 一条 live slot 匹配一条意图即可
			}
		}
	}
	return candidates, nil
}

// TriggerReservation 由调度器通过 public_id 触发单条预约提交。
// 使用原子状态切换防止并发双触发。
func (s *ReservationService) TriggerReservation(ctx context.Context, publicID string) (*models.RoomReservation, error) {
	record, err := s.reservationRepo.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, fmt.Errorf("reservation not found: %w", err)
	}

	ok, err := s.reservationRepo.AtomicTransitionStatus(ctx, record.PublicID, "pending", "submitting")
	if err != nil || !ok {
		return nil, fmt.Errorf("reservation already processing or not in pending state: %w", err)
	}
	// 重新加载最新状态
	record, _ = s.reservationRepo.GetByID(ctx, record.ID)

	_ = s.executeReservation(ctx, record)
	record, _ = s.reservationRepo.GetByID(ctx, record.ID)
	return record, nil
}

// MaterializePlans 查找所有已到开放时间的 scheduled 计划，补全 slot 上下文后提交 TYYS。
// 并发执行，返回汇总结果。
func (s *ReservationService) MaterializePlans(ctx context.Context, dryRun bool) MaterializeResult {
	plans, err := s.reservationRepo.ListDueScheduled(ctx, time.Now())
	if err != nil {
		return MaterializeResult{Errors: []string{fmt.Sprintf("list scheduled: %s", err)}}
	}

	result := MaterializeResult{Total: len(plans)}
	if dryRun || len(plans) == 0 {
		return result
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, plan := range plans {
		wg.Add(1)
		go func(p *models.RoomReservation) {
			defer wg.Done()
			if materializeErr := s.materializeOne(ctx, p); materializeErr != nil {
				mu.Lock()
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("reservation %s: %s", p.PublicID, materializeErr))
				mu.Unlock()
				return
			}
			mu.Lock()
			result.Succeeded++
			mu.Unlock()
		}(plan)
	}
	wg.Wait()

	// 清理 failed 中预约时段已过期的记录，防止调度器继续无效重试。
	loc, _ := time.LoadLocation("Asia/Shanghai")
	today := time.Now().In(loc).Format("2006-01-02")
	if expired, cleanErr := s.reservationRepo.MarkExpiredFailed(ctx, today); cleanErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cleanup expired: %s", cleanErr))
	} else {
		result.Expired = int(expired)
	}

	return result
}

// materializeOne 补全单条计划的 slot 上下文并提交 TYYS。
// 仅在 reserve_open_at <= now 时被调用。failed 状态的计划每小时也会重试（部分场馆窗口尚未开放时的兜底）。
func (s *ReservationService) materializeOne(ctx context.Context, plan *models.RoomReservation) error {
	// 尝试从 scheduled 或 failed 原子切换到 submitting，防止并发双触发。
	ok, _ := s.reservationRepo.AtomicTransitionStatus(ctx, plan.PublicID, "scheduled", "submitting")
	if !ok {
		ok, _ = s.reservationRepo.AtomicTransitionStatus(ctx, plan.PublicID, "failed", "submitting")
	}
	if !ok {
		return fmt.Errorf("already processing")
	}

	candidates, resolveErr := s.resolvePreferredSlots(ctx, plan)
	if resolveErr != nil {
		s.failReservation(ctx, plan, resolveErr.Error())
		return resolveErr
	}
	if len(candidates) == 0 {
		err := fmt.Errorf("no available slot for %s-%s on %s matching preferred spaces", plan.StartTime, plan.EndTime, plan.ReservationDate)
		s.failReservation(ctx, plan, err.Error())
		return err
	}

	if err := s.trySlots(ctx, plan, candidates); err != nil {
		s.failReservation(ctx, plan, err.Error())
		return err
	}
	return nil
}

// executeReservation 调用 TYYS ReserveV2，对验证码失败最多重试 5 次。
// 无论成功还是失败都回写 reservation_status 并记录 attempt log。
func (s *ReservationService) executeReservation(ctx context.Context, record *models.RoomReservation) error {
	if record.VenueSiteID == nil || record.SpaceID == nil || record.TimeID == nil || record.Token == "" {
		err := fmt.Errorf("missing slot context (venue_site_id/space_id/time_id/token)")
		s.failReservation(ctx, record, err.Error())
		return err
	}

	solver := zjulogin.TYYSPythonCaptchaSolver{}
	req := zjulogin.TYYSReservationV2Request{
		ReservationDate: record.ReservationDate,
		WeekStartDate:   coalesce(record.WeekStartDate, record.ReservationDate),
		Token:           record.Token,
		VenueSiteID:     strconv.FormatUint(uint64(*record.VenueSiteID), 10),
		SpaceID:         strconv.FormatUint(uint64(*record.SpaceID), 10),
		TimeID:          strconv.FormatUint(uint64(*record.TimeID), 10),
		BuddyCode:       record.BuddyCode,
		CaptchaSolver:   solver,
	}

	const maxRetries = 5
	var lastErr error
	var lastResult *zjulogin.TYYSReservationV2Result

	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, execErr := s.tyys.ReserveV2(ctx, req)
		lastResult = result
		lastErr = execErr
		if execErr == nil {
			break
		}
		// 只对验证码失败重试。
		if !isCaptchaError(execErr) {
			break
		}
	}

	stage := "submit"
	success := lastErr == nil
	msg := ""
	if lastErr != nil {
		msg = lastErr.Error()
	}

	rawResp := ""
	orderID := ""
	tradeNo := ""
	if lastResult != nil && lastResult.Submit != nil {
		raw, _ := json.Marshal(lastResult.Submit)
		rawResp = string(raw)
		extractOrderFields(lastResult.Submit.Data, &orderID, &tradeNo)
	}

	now := time.Now()
	record.SubmitAttemptedAt = &now
	record.RawResponse = rawResp
	if success {
		record.ReservationStatus = "success"
		record.ExternalOrderID = orderID
		record.ExternalTradeNo = tradeNo
	} else {
		record.ReservationStatus = "failed"
	}
	_ = s.reservationRepo.Update(ctx, record)

	logEntry := &models.ReservationAttemptLog{
		RoomID:        &record.RoomID,
		ReservationID: &record.ID,
		Stage:         stage,
		Success:       success,
		Message:       msg,
	}
	if lastResult != nil && lastResult.Submit != nil {
		raw, _ := json.Marshal(lastResult.Submit)
		logEntry.ResponseSnapshot = string(raw)
	}
	_ = s.reservationRepo.CreateAttemptLog(ctx, logEntry)

	return lastErr
}

// failReservation 将预约状态标记为 failed 并记录日志。
func (s *ReservationService) failReservation(ctx context.Context, record *models.RoomReservation, msg string) {
	record.ReservationStatus = "failed"
	_ = s.reservationRepo.Update(ctx, record)
	_ = s.reservationRepo.CreateAttemptLog(ctx, &models.ReservationAttemptLog{
		RoomID:        &record.RoomID,
		ReservationID: &record.ID,
		Stage:         "materialize",
		Success:       false,
		Message:       msg,
	})
}

// isCaptchaError 判断错误是否属于验证码失败，只有这类错误才应重试。
func isCaptchaError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "captcha") || strings.Contains(msg, "repcode=6111") || strings.Contains(msg, "验证失败")
}

// marshalPlanSlots 将 []PlanSlotSelection 序列化为 JSON 字符串；空切片返回 ""。
func marshalPlanSlots(slots []PlanSlotSelection) string {
	if len(slots) == 0 {
		return ""
	}
	b, _ := json.Marshal(slots)
	return string(b)
}

// unmarshalPlanSlots 将 JSON 字符串反序列化为 []PlanSlotSelection；空或非法字符串返回 nil。
func unmarshalPlanSlots(s string) []PlanSlotSelection {
	if s == "" {
		return nil
	}
	var slots []PlanSlotSelection
	_ = json.Unmarshal([]byte(s), &slots)
	return slots
}

// formatTimeOnly 从 "YYYY-MM-DD HH:mm" 或 "HH:mm" 格式中提取 "HH:mm" 部分。
func formatTimeOnly(s string) string {
	if len(s) > 5 {
		return s[len(s)-5:]
	}
	return s
}

// parseUint 将字符串转换为 uint，失败返回 0。
func parseUint(s string) uint {
	v, _ := strconv.ParseUint(s, 10, 64)
	return uint(v)
}

// coalesce 返回第一个非空字符串。
func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// stringVal 从指针安全取值。
func stringVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// mustMarshalJSON 序列化为 JSON，失败返回 "null"。
func mustMarshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

// extractOrderFields 从 TYYS submit 响应 data 中提取订单 ID 和交易号。
func extractOrderFields(data json.RawMessage, orderID, tradeNo *string) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	if v := trimString(obj["orderId"]); v != "" {
		*orderID = v
	}
	if v := trimString(obj["tradeNo"]); v != "" {
		*tradeNo = v
	}
}
