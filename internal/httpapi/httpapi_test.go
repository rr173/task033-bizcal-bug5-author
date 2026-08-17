package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"task033-bizcal/internal/calendar"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cal, err := calendar.New(calendar.Config{
		Timezone:          "Asia/Shanghai",
		Weekend:           []int{6, 0},
		FixedHolidays:     []string{"2026-01-01"},
		RecurringHolidays: []string{"01-01", "05-01", "10-01"},
	})
	if err != nil {
		t.Fatalf("calendar.New: %v", err)
	}
	return httptest.NewServer(New(cal).Handler())
}

func post(t *testing.T, url string, body any) (int, map[string]any) {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &out)
	}
	return resp.StatusCode, out
}

func TestIsBusinessDayEndpoint(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	if code, body := post(t, ts.URL+"/is-business-day", map[string]string{"date": "2026-08-15"}); code != http.StatusOK || body["business_day"] != false {
		t.Errorf("Sat: code=%d body=%v", code, body)
	}
	if code, body := post(t, ts.URL+"/is-business-day", map[string]string{"date": "2026-08-14"}); code != http.StatusOK || body["business_day"] != true {
		t.Errorf("Fri: code=%d body=%v", code, body)
	}
}

func TestAddBusinessDaysEndpoint(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	if code, body := post(t, ts.URL+"/add-business-days", map[string]any{"date": "2026-08-14", "n": 1}); code != http.StatusOK || body["date"] != "2026-08-17" {
		t.Errorf("Fri+1: code=%d body=%v", code, body)
	}
	// n=0 is valid and returns the same date.
	if code, body := post(t, ts.URL+"/add-business-days", map[string]any{"date": "2026-08-14", "n": 0}); code != http.StatusOK || body["date"] != "2026-08-14" {
		t.Errorf("Fri+0: code=%d body=%v", code, body)
	}
}

func TestBusinessDaysBetweenEndpoint(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	if code, body := post(t, ts.URL+"/business-days-between", map[string]string{"from": "2026-08-10", "to": "2026-08-20"}); code != http.StatusOK || body["count"] != float64(8) {
		t.Errorf("between: code=%d body=%v", code, body)
	}
}

func TestListBusinessDaysEndpoint(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	code, body := post(t, ts.URL+"/list-business-days", map[string]string{"from": "2026-08-10", "to": "2026-08-17"})
	if code != http.StatusOK {
		t.Fatalf("list: code=%d", code)
	}
	dates, _ := body["dates"].([]any)
	if len(dates) != 5 {
		t.Fatalf("list: got %d dates want 5: %v", len(dates), dates)
	}
	if dates[0] != "2026-08-10" || dates[4] != "2026-08-14" {
		t.Errorf("list bounds: %v", dates)
	}
	// Reversed range yields an empty JSON array, not null.
	code, body = post(t, ts.URL+"/list-business-days", map[string]string{"from": "2026-08-17", "to": "2026-08-10"})
	if code != http.StatusOK {
		t.Fatalf("list reversed: code=%d", code)
	}
	if dates, _ := body["dates"].([]any); len(dates) != 0 {
		t.Errorf("list reversed: want empty array, got %v", dates)
	}
}

func TestResolveTimestampEndpoint(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	if code, body := post(t, ts.URL+"/resolve-timestamp", map[string]string{"timestamp": "2026-08-15T16:30:00Z"}); code != http.StatusOK || body["local_date"] != "2026-08-16" || body["business_day"] != false {
		t.Errorf("resolve: code=%d body=%v", code, body)
	}
}

func TestErrorCases(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	cases := []struct {
		name string
		path string
		body any
	}{
		{"bad date", "/is-business-day", map[string]string{"date": "nope"}},
		{"missing n", "/add-business-days", map[string]string{"date": "2026-08-14"}},
		{"bad from", "/business-days-between", map[string]string{"from": "nope", "to": "2026-08-20"}},
		{"bad to", "/list-business-days", map[string]string{"from": "2026-08-10", "to": "nope"}},
		{"bad timestamp", "/resolve-timestamp", map[string]string{"timestamp": "nope"}},
		{"empty body", "/is-business-day", map[string]string{}},
	}
	for _, tc := range cases {
		code, body := post(t, ts.URL+tc.path, tc.body)
		if code != http.StatusBadRequest {
			t.Errorf("%s: code=%d want 400 body=%v", tc.name, code, body)
		}
		if _, ok := body["error"]; !ok {
			t.Errorf("%s: response missing error field: %v", tc.name, body)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// GET on a POST-only endpoint is rejected.
	resp, err := http.Get(ts.URL + "/is-business-day")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /is-business-day: code=%d want 405", resp.StatusCode)
	}
}
