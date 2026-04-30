package service

import (
	"testing"
	"time"
)

// --- marshalPlanSlots / unmarshalPlanSlots ---

func TestMarshalUnmarshalPlanSlots_RoundTrip(t *testing.T) {
	venueID := uint(42)
	spaceIDStr := "篮球场A"
	original := []PlanSlotSelection{
		{
			CampusName:  "紫金港",
			VenueName:   "篮球馆",
			VenueID:     &venueID,
			SpaceID:     101,
			SpaceName:   &spaceIDStr,
			StartTime:   "09:00",
			EndTime:     "10:00",
		},
		{
			CampusName: "玉泉",
			VenueName:  "羽毛球馆",
			SpaceID:    202,
			StartTime:  "14:00",
			EndTime:    "15:00",
		},
	}

	encoded := marshalPlanSlots(original)
	if encoded == "" {
		t.Fatal("marshalPlanSlots returned empty string for non-empty input")
	}

	decoded := unmarshalPlanSlots(encoded)
	if len(decoded) != len(original) {
		t.Fatalf("round-trip length mismatch: got %d, want %d", len(decoded), len(original))
	}

	for i, got := range decoded {
		want := original[i]
		if got.CampusName != want.CampusName {
			t.Errorf("[%d] CampusName: got %q, want %q", i, got.CampusName, want.CampusName)
		}
		if got.VenueName != want.VenueName {
			t.Errorf("[%d] VenueName: got %q, want %q", i, got.VenueName, want.VenueName)
		}
		if got.SpaceID != want.SpaceID {
			t.Errorf("[%d] SpaceID: got %d, want %d", i, got.SpaceID, want.SpaceID)
		}
		if got.StartTime != want.StartTime {
			t.Errorf("[%d] StartTime: got %q, want %q", i, got.StartTime, want.StartTime)
		}
		if got.EndTime != want.EndTime {
			t.Errorf("[%d] EndTime: got %q, want %q", i, got.EndTime, want.EndTime)
		}
		if (got.VenueID == nil) != (want.VenueID == nil) {
			t.Errorf("[%d] VenueID nil mismatch", i)
		} else if want.VenueID != nil && *got.VenueID != *want.VenueID {
			t.Errorf("[%d] VenueID: got %d, want %d", i, *got.VenueID, *want.VenueID)
		}
	}
}

func TestMarshalPlanSlots_EmptySlice(t *testing.T) {
	if got := marshalPlanSlots(nil); got != "" {
		t.Errorf("nil input: got %q, want empty string", got)
	}
	if got := marshalPlanSlots([]PlanSlotSelection{}); got != "" {
		t.Errorf("empty slice: got %q, want empty string", got)
	}
}

func TestUnmarshalPlanSlots_InvalidJSON(t *testing.T) {
	cases := []string{"", "null", "{}", "not-json"}
	for _, s := range cases {
		got := unmarshalPlanSlots(s)
		if len(got) != 0 {
			t.Errorf("input %q: expected nil/empty, got %v", s, got)
		}
	}
}

// --- venueOpenHour ---

func TestVenueOpenHour(t *testing.T) {
	cases := []struct {
		campus    string
		sport     string
		wantHour  int
	}{
		{"紫金港校区", "网球", 8},
		{"玉泉校区", "羽毛球", 12},
		{"华家池校区", "羽毛球", 18},
		{"华家池校区", "网球", 18},
		// 默认：不匹配任何特殊规则
		{"西溪校区", "篮球", 9},
		{"玉泉校区", "网球", 9},
	}

	for _, c := range cases {
		got := venueOpenHour(c.campus, c.sport)
		if got != c.wantHour {
			t.Errorf("venueOpenHour(%q, %q) = %d, want %d", c.campus, c.sport, got, c.wantHour)
		}
	}
}

// --- reserveOpenAt ---

func TestReserveOpenAt_TwoDaysBefore(t *testing.T) {
	// 预约日期 2026-05-10（周日），紫金港网球（8:00 开放）
	// 期望开放时间：2026-05-08 08:00 Asia/Shanghai
	got, err := reserveOpenAt("2026-05-10", "紫金港校区", "网球")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	want := time.Date(2026, 5, 8, 8, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReserveOpenAt_DefaultHour(t *testing.T) {
	// 西溪校区篮球，默认 9:00
	got, err := reserveOpenAt("2026-06-01", "西溪校区", "篮球")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	want := time.Date(2026, 5, 30, 9, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReserveOpenAt_InvalidDate(t *testing.T) {
	_, err := reserveOpenAt("not-a-date", "紫金港", "网球")
	if err == nil {
		t.Error("expected error for invalid date, got nil")
	}
}

// --- coalesce ---

func TestCoalesce(t *testing.T) {
	if got := coalesce("a", "b"); got != "a" {
		t.Errorf("coalesce(\"a\",\"b\") = %q, want \"a\"", got)
	}
	if got := coalesce("", "b"); got != "b" {
		t.Errorf("coalesce(\"\",\"b\") = %q, want \"b\"", got)
	}
	if got := coalesce("", ""); got != "" {
		t.Errorf("coalesce(\"\",\"\") = %q, want \"\"", got)
	}
}

// --- stringVal ---

func TestStringVal(t *testing.T) {
	s := "hello"
	if got := stringVal(&s); got != "hello" {
		t.Errorf("stringVal(&\"hello\") = %q, want \"hello\"", got)
	}
	if got := stringVal(nil); got != "" {
		t.Errorf("stringVal(nil) = %q, want \"\"", got)
	}
}

// --- formatTimeOnly ---

func TestFormatTimeOnly(t *testing.T) {
	cases := []struct{ in, want string }{
		{"09:00", "09:00"},
		{"2026-05-01 09:00", "09:00"},
		{"18:30", "18:30"},
		{"2026-05-01 18:30", "18:30"},
	}
	for _, c := range cases {
		got := formatTimeOnly(c.in)
		if got != c.want {
			t.Errorf("formatTimeOnly(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
