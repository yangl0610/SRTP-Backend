//go:build ignore

package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	zjulogin "github.com/QSCTech/SRTP-Backend/internal/zjulogin"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	auth, err := zjulogin.NewFromEnv()
	if err != nil {
		panic(err)
	}
	tyys, err := auth.TYYS()
	if err != nil {
		panic(err)
	}

	params := url.Values{}
	params.Set("venueId", "22")
	params.Set("venueSiteId", "23")
	params.Set("siteId", "23")
	params.Set("date", "2026-05-08")
	params.Set("reservationDate", "2026-05-08")
	params.Set("searchDate", "2026-05-08")
	params.Set("weekStartDate", "2026-05-08")

	resp, err := tyys.ReservationDayInfo(ctx, params)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", string(resp.Data))
}
