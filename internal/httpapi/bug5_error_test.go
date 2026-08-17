package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"task033-bizcal/internal/calendar"
)

func TestBug5RejectsTrailingJSON(t *testing.T) {
	cal, err := calendar.New(calendar.Config{Timezone: "Asia/Shanghai", Weekend: []int{6, 0}})
	if err != nil {
		t.Fatalf("calendar.New: %v", err)
	}
	payload := []byte(`{"date":"2026-08-14"}{"unexpected":true}`)
	req := httptest.NewRequest(http.MethodPost, "/is-business-day", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	New(cal).Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}
