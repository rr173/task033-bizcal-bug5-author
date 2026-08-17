// Package httpapi exposes the business-calendar service over HTTP+JSON.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"task033-bizcal/internal/calendar"
)

// Server serves the business-calendar API backed by a single immutable
// calendar loaded at startup.
type Server struct {
	cal *calendar.Calendar
}

// New returns a Server for the given calendar.
func New(cal *calendar.Calendar) *Server {
	return &Server{cal: cal}
}

// Handler returns the HTTP handler serving the API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /is-business-day", s.handleIsBusinessDay)
	mux.HandleFunc("POST /add-business-days", s.handleAddBusinessDays)
	mux.HandleFunc("POST /business-days-between", s.handleBusinessDaysBetween)
	mux.HandleFunc("POST /list-business-days", s.handleListBusinessDays)
	mux.HandleFunc("POST /resolve-timestamp", s.handleResolveTimestamp)
	return mux
}

// ---- request / response types ----

type dateReq struct {
	Date string `json:"date"`
}

type addReq struct {
	Date string `json:"date"`
	N    *int   `json:"n"` // pointer so a missing field is distinguishable from 0
}

type rangeReq struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type timestampReq struct {
	Timestamp string `json:"timestamp"`
}

type errResp struct {
	Error string `json:"error"`
}

// ---- handlers ----

func (s *Server) handleIsBusinessDay(w http.ResponseWriter, r *http.Request) {
	var req dateReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	d, err := calendar.ParseDate(req.Date)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid date")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"business_day": s.cal.IsBusinessDay(d)})
}

func (s *Server) handleAddBusinessDays(w http.ResponseWriter, r *http.Request) {
	var req addReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	d, err := calendar.ParseDate(req.Date)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid date")
		return
	}
	if req.N == nil {
		writeErr(w, http.StatusBadRequest, "missing n")
		return
	}
	res := s.cal.AddBusinessDays(d, *req.N)
	writeJSON(w, http.StatusOK, map[string]string{"date": res.String()})
}

func (s *Server) handleBusinessDaysBetween(w http.ResponseWriter, r *http.Request) {
	var req rangeReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	from, err := calendar.ParseDate(req.From)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid from date")
		return
	}
	to, err := calendar.ParseDate(req.To)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid to date")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": s.cal.BusinessDaysBetween(from, to)})
}

func (s *Server) handleListBusinessDays(w http.ResponseWriter, r *http.Request) {
	var req rangeReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	from, err := calendar.ParseDate(req.From)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid from date")
		return
	}
	to, err := calendar.ParseDate(req.To)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid to date")
		return
	}
	days := s.cal.ListBusinessDays(from, to)
	var out []string
	for _, d := range days {
		out = append(out, d.String())
	}
	writeJSON(w, http.StatusOK, map[string][]string{"dates": out})
}

func (s *Server) handleResolveTimestamp(w http.ResponseWriter, r *http.Request) {
	var req timestampReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	d, err := s.cal.ResolveTimestamp(req.Timestamp)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid timestamp")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"local_date":   d.String(),
		"business_day": s.cal.IsBusinessDay(d),
	})
}

// ---- helpers ----

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain exactly one JSON value")
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errResp{Error: msg})
}
