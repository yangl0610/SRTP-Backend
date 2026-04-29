//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	zjulogin "github.com/QSCTech/SRTP-Backend/internal/zjulogin"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	auth, err := zjulogin.NewFromEnv()
	if err != nil {
		panic(err)
	}
	tyys, err := auth.TYYS()
	if err != nil {
		panic(err)
	}

	solver := zjulogin.TYYSPythonCaptchaSolver{
		PythonPath: firstNonEmpty(os.Getenv("TYYS_CAPTCHA_PYTHON"), "python"),
		ScriptPath: firstNonEmpty(os.Getenv("TYYS_CAPTCHA_SCRIPT"), "scripts/tyys_captcha_solver.py"),
	}

	reservationDate := "2026-04-25"
	weekStartDate := reservationDate
	venueID := "22"
	venueSiteID := "143"
	spaceID := "328"
	timeID := "22013"

	dayInfoParams := url.Values{}
	dayInfoParams.Set("venueId", venueID)
	dayInfoParams.Set("venueSiteId", venueSiteID)
	dayInfoParams.Set("siteId", venueSiteID)
	dayInfoParams.Set("date", reservationDate)
	dayInfoParams.Set("reservationDate", reservationDate)
	dayInfoParams.Set("searchDate", reservationDate)
	dayInfoParams.Set("weekStartDate", weekStartDate)

	dayInfo, err := tyys.ReservationDayInfo(ctx, dayInfoParams)
	if err != nil {
		panic(err)
	}

	token, err := extractReservationToken(dayInfo.Data)
	if err != nil {
		panic(err)
	}

	result, err := tyys.ReserveV2(ctx, zjulogin.TYYSReservationV2Request{
		ReservationDate: reservationDate,
		WeekStartDate:   weekStartDate,
		Token:           token,
		VenueSiteID:     venueSiteID,
		SpaceID:         spaceID,
		TimeID:          timeID,
		BuddyCode:       "", //同伴码
		Phone:           "",
		IsOfflineTicket: firstNonEmpty(os.Getenv("TYYS_IS_OFFLINE_TICKET"), "1"),
		CaptchaSolver:   solver,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("reservation_date=%s venue_site_id=%s space_id=%s time_id=%s token=%s\n", result.ReservationDate, result.VenueSiteID, result.SpaceID, result.TimeID, result.Token)
	fmt.Printf("order form: %s\n", result.OrderForm.Encode())
	if result.Submit != nil {
		fmt.Printf("submit code=%d message=%s\n", result.Submit.Code, result.Submit.Message)
	}
}

func extractReservationToken(data json.RawMessage) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("parse day/info data: %w", err)
	}
	token, ok := payload["token"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("token not found in day/info response")
	}
	return token, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
