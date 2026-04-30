package repository_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
)

// connectTestDB 从环境变量构建 DSN 并返回 GORM 连接。
// 若 DB_HOST 未设置，测试将被跳过。
func connectTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	host := os.Getenv("DB_HOST")
	if host == "" {
		t.Skip("set DB_HOST to run repository integration tests (see .env.example)")
	}
	port, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	if port == 0 {
		port = 5432
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "srtp"
	}
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		host, port, user, password, dbName, sslmode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Room{}, &models.RoomReservation{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return db
}

// seedRoom 在测试库中创建一个最小化 User 和 Room，返回 Room.ID。
// t.Cleanup 中自动删除。
func seedRoom(t *testing.T, db *gorm.DB) uint {
	t.Helper()

	user := &models.User{
		AuthUID: fmt.Sprintf("test-uid-%d", time.Now().UnixNano()),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}

	room := &models.Room{
		OwnerID:    user.ID,
		Name:       "测试球场",
		SportType:  "篮球",
		CampusName: "紫金港",
		VenueName:  "篮球馆",
		StartTime:  time.Now().Add(24 * time.Hour),
		EndTime:    time.Now().Add(25 * time.Hour),
	}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("create test room: %v", err)
	}

	t.Cleanup(func() {
		db.Where("room_id = ?", room.ID).Delete(&models.RoomReservation{})
		db.Delete(room)
		db.Delete(user)
	})

	return room.ID
}

// seedReservation 创建一条预约记录并在测试结束时删除。
func seedReservation(t *testing.T, db *gorm.DB, r *models.RoomReservation) *models.RoomReservation {
	t.Helper()
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("create reservation: %v", err)
	}
	t.Cleanup(func() { db.Delete(r) })
	return r
}

// --- ListDueScheduled ---

func TestListDueScheduled_IncludesScheduledAndFailed(t *testing.T) {
	db := connectTestDB(t)
	repo := repository.NewReservationRepository(db)
	roomID := seedRoom(t, db)

	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	cases := []struct {
		status        string
		reserveOpenAt *time.Time
		wantIncluded  bool
		desc          string
	}{
		{"scheduled", &past, true, "scheduled + past open_at → 应被返回"},
		{"failed", &past, true, "failed + past open_at → 应被返回（新行为）"},
		{"expired", &past, false, "expired + past open_at → 不应被返回"},
		{"success", &past, false, "success + past open_at → 不应被返回"},
		{"scheduled", &future, false, "scheduled + 未来 open_at → 不应被返回"},
		{"failed", &future, false, "failed + 未来 open_at → 不应被返回"},
	}

	ids := make(map[uint]bool)
	for _, c := range cases {
		r := seedReservation(t, db, &models.RoomReservation{
			RoomID:            roomID,
			SportType:         "篮球",
			CampusName:        "紫金港",
			VenueName:         "篮球馆",
			ReservationDate:   "2099-01-01",
			StartTime:         "09:00",
			EndTime:           "10:00",
			ReservationStatus: c.status,
			ScheduleStatus:    "none",
			ReserveOpenAt:     c.reserveOpenAt,
		})
		if c.wantIncluded {
			ids[r.ID] = false // false = not yet seen
		}
	}

	results, err := repo.ListDueScheduled(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("ListDueScheduled: %v", err)
	}

	for _, r := range results {
		if _, ok := ids[r.ID]; ok {
			ids[r.ID] = true // seen
		}
	}

	for id, seen := range ids {
		if !seen {
			t.Errorf("reservation ID=%d expected in result but not found", id)
		}
	}

	// 验证不期望出现的 ID 确实没有出现
	for _, r := range results {
		for _, c := range cases {
			if !c.wantIncluded {
				// 检查返回结果中不含非预期状态（只检查我们创建的记录）
				// 通过 room_id 过滤避免误报其他测试的数据
				if r.RoomID == roomID && r.ReservationStatus == c.status {
					if c.reserveOpenAt != nil && c.reserveOpenAt.Before(time.Now()) {
						t.Errorf("status=%q 不应被 ListDueScheduled 返回", c.status)
					}
				}
			}
		}
	}
}

// --- MarkExpiredFailed ---

func TestMarkExpiredFailed_OnlyAffectsFailedAndPastDate(t *testing.T) {
	db := connectTestDB(t)
	repo := repository.NewReservationRepository(db)
	roomID := seedRoom(t, db)

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	cases := []struct {
		status      string
		date        string
		wantExpired bool
		desc        string
	}{
		{"failed", yesterday, true, "failed + 昨天 → 应标记为 expired"},
		{"failed", today, false, "failed + 今天 → 不应标记（需要 < today）"},
		{"failed", tomorrow, false, "failed + 明天 → 不应标记"},
		{"scheduled", yesterday, false, "scheduled + 昨天 → 不应标记"},
		{"success", yesterday, false, "success + 昨天 → 不应标记"},
	}

	reservations := make([]*models.RoomReservation, len(cases))
	for i, c := range cases {
		r := seedReservation(t, db, &models.RoomReservation{
			RoomID:            roomID,
			SportType:         "篮球",
			CampusName:        "紫金港",
			VenueName:         "篮球馆",
			ReservationDate:   c.date,
			StartTime:         "09:00",
			EndTime:           "10:00",
			ReservationStatus: c.status,
			ScheduleStatus:    "none",
		})
		reservations[i] = r
	}

	count, err := repo.MarkExpiredFailed(context.Background(), today)
	if err != nil {
		t.Fatalf("MarkExpiredFailed: %v", err)
	}

	// 统计本次应被标记的数量
	wantCount := 0
	for _, c := range cases {
		if c.wantExpired {
			wantCount++
		}
	}
	if int(count) < wantCount {
		t.Errorf("MarkExpiredFailed affected %d rows, want at least %d", count, wantCount)
	}

	// 验证每条记录的最终状态
	for i, c := range cases {
		var r models.RoomReservation
		if err := db.First(&r, reservations[i].ID).Error; err != nil {
			t.Fatalf("[%d] reload reservation: %v", i, err)
		}
		if c.wantExpired && r.ReservationStatus != "expired" {
			t.Errorf("[%d] %s: status = %q, want \"expired\"", i, c.desc, r.ReservationStatus)
		}
		if !c.wantExpired && r.ReservationStatus == "expired" {
			t.Errorf("[%d] %s: status unexpectedly set to \"expired\"", i, c.desc)
		}
	}
}

// --- AtomicTransitionStatus ---

func TestAtomicTransitionStatus_SuccessFromScheduled(t *testing.T) {
	db := connectTestDB(t)
	repo := repository.NewReservationRepository(db)
	roomID := seedRoom(t, db)

	r := seedReservation(t, db, &models.RoomReservation{
		RoomID:            roomID,
		SportType:         "篮球",
		CampusName:        "紫金港",
		VenueName:         "篮球馆",
		ReservationDate:   "2099-06-01",
		StartTime:         "09:00",
		EndTime:           "10:00",
		ReservationStatus: "scheduled",
		ScheduleStatus:    "none",
	})

	ok, err := repo.AtomicTransitionStatus(context.Background(), r.PublicID, "scheduled", "submitting")
	if err != nil {
		t.Fatalf("AtomicTransitionStatus: %v", err)
	}
	if !ok {
		t.Error("expected ok=true, got false")
	}

	var updated models.RoomReservation
	db.First(&updated, r.ID)
	if updated.ReservationStatus != "submitting" {
		t.Errorf("status = %q, want \"submitting\"", updated.ReservationStatus)
	}
}

func TestAtomicTransitionStatus_SuccessFromFailed(t *testing.T) {
	db := connectTestDB(t)
	repo := repository.NewReservationRepository(db)
	roomID := seedRoom(t, db)

	r := seedReservation(t, db, &models.RoomReservation{
		RoomID:            roomID,
		SportType:         "篮球",
		CampusName:        "紫金港",
		VenueName:         "篮球馆",
		ReservationDate:   "2099-06-01",
		StartTime:         "09:00",
		EndTime:           "10:00",
		ReservationStatus: "failed",
		ScheduleStatus:    "none",
	})

	// materializeOne 对 failed 状态也需要能原子切换
	ok, err := repo.AtomicTransitionStatus(context.Background(), r.PublicID, "failed", "submitting")
	if err != nil {
		t.Fatalf("AtomicTransitionStatus: %v", err)
	}
	if !ok {
		t.Error("expected ok=true for failed→submitting, got false")
	}

	var updated models.RoomReservation
	db.First(&updated, r.ID)
	if updated.ReservationStatus != "submitting" {
		t.Errorf("status = %q, want \"submitting\"", updated.ReservationStatus)
	}
}

func TestAtomicTransitionStatus_WrongFromStatus(t *testing.T) {
	db := connectTestDB(t)
	repo := repository.NewReservationRepository(db)
	roomID := seedRoom(t, db)

	r := seedReservation(t, db, &models.RoomReservation{
		RoomID:            roomID,
		SportType:         "篮球",
		CampusName:        "紫金港",
		VenueName:         "篮球馆",
		ReservationDate:   "2099-06-01",
		StartTime:         "09:00",
		EndTime:           "10:00",
		ReservationStatus: "submitting",
		ScheduleStatus:    "none",
	})

	// 尝试从 scheduled 切换，但实际是 submitting → 应该返回 false
	ok, _ := repo.AtomicTransitionStatus(context.Background(), r.PublicID, "scheduled", "submitting")
	if ok {
		t.Error("expected ok=false when fromStatus does not match, got true")
	}
}

func TestAtomicTransitionStatus_ConcurrentPreventsDuplicate(t *testing.T) {
	db := connectTestDB(t)
	repo := repository.NewReservationRepository(db)
	roomID := seedRoom(t, db)

	r := seedReservation(t, db, &models.RoomReservation{
		RoomID:            roomID,
		SportType:         "篮球",
		CampusName:        "紫金港",
		VenueName:         "篮球馆",
		ReservationDate:   "2099-06-01",
		StartTime:         "09:00",
		EndTime:           "10:00",
		ReservationStatus: "scheduled",
		ScheduleStatus:    "none",
	})

	// 第一次成功
	ok1, err1 := repo.AtomicTransitionStatus(context.Background(), r.PublicID, "scheduled", "submitting")
	if err1 != nil || !ok1 {
		t.Fatalf("first transition failed: ok=%v err=%v", ok1, err1)
	}

	// 第二次应被阻止（记录已变为 submitting）
	ok2, _ := repo.AtomicTransitionStatus(context.Background(), r.PublicID, "scheduled", "submitting")
	if ok2 {
		t.Error("second transition should be blocked, got ok=true")
	}
}
