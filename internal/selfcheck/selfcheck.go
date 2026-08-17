// Package selfcheck runs an end-to-end verification of the business-calendar
// service against an in-process HTTP server. It is invoked by the --smoke-test
// flag and exits the process on completion.
package selfcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"task033-bizcal/internal/calendar"
	"task033-bizcal/internal/httpapi"
)

// Request body shapes mirrored locally so call sites read as named struct
// literals; they marshal to the same JSON the handlers expect.
type dateReq struct {
	Date string `json:"date"`
}

type rangeReq struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type timestampReq struct {
	Timestamp string `json:"timestamp"`
}

// Run exercises the full HTTP API plus direct timezone-resolution checks and
// returns nil if every behavior matches the specification. On failure it
// returns an error describing the first mismatch.
func Run() error {
	cfg := calendar.Config{
		Timezone:          "Asia/Shanghai",
		Weekend:           []int{6, 0},
		FixedHolidays:     []string{"2026-01-01"},
		RecurringHolidays: []string{"01-01", "05-01", "10-01"},
	}
	cal, err := calendar.New(cfg)
	if err != nil {
		return fmt.Errorf("build calendar: %w", err)
	}

	ts := httptest.NewServer(httpapi.New(cal).Handler())
	defer ts.Close()
	c := ts.Client()

	// 1. is-business-day: weekend, weekday, holiday (fixed+recurring), recurring next year.
	if code, body, err := post(c, ts.URL+"/is-business-day", dateReq{Date: "2026-08-15"}); err != nil {
		return err
	} else if code != http.StatusOK || body["business_day"] != false {
		return fmt.Errorf("is-business-day 2026-08-15 (Sat): code=%d body=%v", code, body)
	}
	if code, body, err := post(c, ts.URL+"/is-business-day", dateReq{Date: "2026-08-14"}); err != nil {
		return err
	} else if code != http.StatusOK || body["business_day"] != true {
		return fmt.Errorf("is-business-day 2026-08-14 (Fri): code=%d body=%v", code, body)
	}
	if code, body, err := post(c, ts.URL+"/is-business-day", dateReq{Date: "2026-01-01"}); err != nil {
		return err
	} else if code != http.StatusOK || body["business_day"] != false {
		return fmt.Errorf("is-business-day 2026-01-01 (Thu holiday): code=%d body=%v", code, body)
	}
	if code, body, err := post(c, ts.URL+"/is-business-day", dateReq{Date: "2027-01-01"}); err != nil {
		return err
	} else if code != http.StatusOK || body["business_day"] != false {
		return fmt.Errorf("is-business-day 2027-01-01 (recurring): code=%d body=%v", code, body)
	}

	// 2. add-business-days: forward from weekday, forward from weekend, n==0, negative.
	if code, body, err := post(c, ts.URL+"/add-business-days", map[string]any{"date": "2026-08-14", "n": 1}); err != nil {
		return err
	} else if code != http.StatusOK || body["date"] != "2026-08-17" {
		return fmt.Errorf("add Fri +1: code=%d body=%v (want 2026-08-17)", code, body)
	}
	if code, body, err := post(c, ts.URL+"/add-business-days", map[string]any{"date": "2026-08-15", "n": 1}); err != nil {
		return err
	} else if code != http.StatusOK || body["date"] != "2026-08-17" {
		return fmt.Errorf("add Sat +1: code=%d body=%v (want 2026-08-17)", code, body)
	}
	if code, body, err := post(c, ts.URL+"/add-business-days", map[string]any{"date": "2026-08-15", "n": 0}); err != nil {
		return err
	} else if code != http.StatusOK || body["date"] != "2026-08-15" {
		return fmt.Errorf("add Sat +0: code=%d body=%v (want 2026-08-15)", code, body)
	}
	if code, body, err := post(c, ts.URL+"/add-business-days", map[string]any{"date": "2026-08-17", "n": -1}); err != nil {
		return err
	} else if code != http.StatusOK || body["date"] != "2026-08-14" {
		return fmt.Errorf("add Mon -1: code=%d body=%v (want 2026-08-14)", code, body)
	}

	// 3. business-days-between: half-open, equal, reversed.
	if code, body, err := post(c, ts.URL+"/business-days-between", rangeReq{From: "2026-08-10", To: "2026-08-20"}); err != nil {
		return err
	} else if code != http.StatusOK || body["count"] != float64(8) {
		return fmt.Errorf("between 08-10..08-20: code=%d body=%v (want 8)", code, body)
	}
	if code, body, err := post(c, ts.URL+"/business-days-between", rangeReq{From: "2026-08-10", To: "2026-08-10"}); err != nil {
		return err
	} else if code != http.StatusOK || body["count"] != float64(0) {
		return fmt.Errorf("between equal: code=%d body=%v (want 0)", code, body)
	}
	if code, body, err := post(c, ts.URL+"/business-days-between", rangeReq{From: "2026-08-20", To: "2026-08-10"}); err != nil {
		return err
	} else if code != http.StatusOK || body["count"] != float64(0) {
		return fmt.Errorf("between reversed: code=%d body=%v (want 0)", code, body)
	}

	// 4. list-business-days: contents, bounds, and empty-array on reversed range.
	if code, body, err := post(c, ts.URL+"/list-business-days", rangeReq{From: "2026-08-10", To: "2026-08-20"}); err != nil {
		return err
	} else if code != http.StatusOK {
		return fmt.Errorf("list 08-10..08-20: code=%d", code)
	} else {
		dates, _ := body["dates"].([]any)
		if len(dates) != 8 {
			return fmt.Errorf("list 08-10..08-20: got %d dates want 8: %v", len(dates), dates)
		}
		if dates[0] != "2026-08-10" || dates[7] != "2026-08-19" {
			return fmt.Errorf("list 08-10..08-20 bounds: %v", dates)
		}
	}
	if code, body, err := post(c, ts.URL+"/list-business-days", rangeReq{From: "2026-08-20", To: "2026-08-10"}); err != nil {
		return err
	} else if code != http.StatusOK {
		return fmt.Errorf("list reversed: code=%d", code)
	} else if dates, _ := body["dates"].([]any); len(dates) != 0 {
		return fmt.Errorf("list reversed: want empty array, got %v", dates)
	}

	// 5. resolve-timestamp: UTC instant and explicit-offset form, both landing on
	//    the configured timezone's local date.
	if code, body, err := post(c, ts.URL+"/resolve-timestamp", timestampReq{Timestamp: "2026-08-15T16:30:00Z"}); err != nil {
		return err
	} else if code != http.StatusOK || body["local_date"] != "2026-08-16" || body["business_day"] != false {
		return fmt.Errorf("resolve 16:30Z: code=%d body=%v (want 2026-08-16, non-business)", code, body)
	}
	if code, body, err := post(c, ts.URL+"/resolve-timestamp", timestampReq{Timestamp: "2026-08-15T07:00:00+08:00"}); err != nil {
		return err
	} else if code != http.StatusOK || body["local_date"] != "2026-08-15" {
		return fmt.Errorf("resolve +08:00: code=%d body=%v (want 2026-08-15)", code, body)
	}

	// 6. error cases return 400 (bad date, missing n, bad timestamp).
	if code, _, err := post(c, ts.URL+"/is-business-day", dateReq{Date: "not-a-date"}); err != nil {
		return err
	} else if code != http.StatusBadRequest {
		return fmt.Errorf("bad date: code=%d want 400", code)
	}
	if code, _, err := post(c, ts.URL+"/add-business-days", dateReq{Date: "2026-08-14"}); err != nil {
		return err
	} else if code != http.StatusBadRequest {
		return fmt.Errorf("missing n: code=%d want 400", code)
	}
	if code, _, err := post(c, ts.URL+"/resolve-timestamp", timestampReq{Timestamp: "nope"}); err != nil {
		return err
	} else if code != http.StatusBadRequest {
		return fmt.Errorf("bad timestamp: code=%d want 400", code)
	}

	// 7. Direct package check: a non-configured, non-local timezone still loads
	//    (the embedded tzdata is what makes this work in a bare container).
	//    America/New_York in August is EDT (UTC-4).
	nyCal, err := calendar.New(calendar.Config{Timezone: "America/New_York", Weekend: []int{6, 0}})
	if err != nil {
		return fmt.Errorf("new-york calendar: %w", err)
	}
	nyd, err := nyCal.ResolveTimestamp("2026-08-15T04:00:00Z")
	if err != nil {
		return fmt.Errorf("ny resolve 04:00Z: %w", err)
	}
	if nyd.String() != "2026-08-15" {
		return fmt.Errorf("ny resolve 04:00Z: got %s want 2026-08-15", nyd)
	}
	nyd2, err := nyCal.ResolveTimestamp("2026-08-15T03:00:00Z")
	if err != nil {
		return fmt.Errorf("ny resolve 03:00Z: %w", err)
	}
	if nyd2.String() != "2026-08-14" {
		return fmt.Errorf("ny resolve 03:00Z: got %s want 2026-08-14", nyd2)
	}

	return nil
}

// ---- HTTP helpers ----

// post sends a JSON POST and returns the status code and decoded body.
func post(c *http.Client, url string, body any) (int, map[string]any, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &out)
	}
	return resp.StatusCode, out, nil
}
