package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"school_district_reading/internal/engagement"
	"school_district_reading/internal/metrics"
)

func (s Server) agenda(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	writeJSON(w, 200, map[string]any{"session_started": s.Session.Snapshot().StartedAt, "state": s.Session.EngagementSnapshot(), "timezone": s.Session.EngagementTimezone(), "now": s.Session.Now().UTC(), "source": "This server session", "note": "Memory only. Restart clears engagement records. Availability requires staff confirmation."})
}
func (s Server) availability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	q := r.URL.Query()
	duration, e := strconv.Atoi(q.Get("duration"))
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "duration required"})
		return
	}
	slots, e := s.Session.Availability(q.Get("student_id"), q.Get("staff_id"), q.Get("date"), duration, q.Get("exclude"))
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": e.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"slots": slots, "timezone": s.Session.EngagementTimezone(), "note": "Suggestions from declared shifts minus structured blocks. Confirm staff/student availability; prose duties are not integrated."})
}

// A single command endpoint keeps all new mutations under the same coordinator.
func (s Server) engagementCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	raw, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var c engagement.Command
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&c); e != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid engagement command"})
		return
	}
	out, e := s.Session.EngagementCommand(r.Context(), c)
	if e != nil {
		code := 400
		if strings.Contains(e.Error(), "conflict") || strings.Contains(e.Error(), "already") || strings.Contains(e.Error(), "reused") {
			code = 409
		}
		writeJSON(w, code, map[string]any{"error": e.Error(), "revision": s.Session.EngagementSnapshot().Revision})
		return
	}
	writeJSON(w, 200, out)
}
func (s Server) engagementMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	q := r.URL.Query()
	if source := q.Get("source"); source != "" && source != "session" {
		writeJSON(w, 400, map[string]string{"error": "only session source is implemented"})
		return
	}
	loc, e := time.LoadLocation(s.Session.EngagementTimezone())
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "school timezone unavailable"})
		return
	}
	now := s.Session.Now().UTC()
	start := s.Session.Snapshot().StartedAt.In(loc)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	end := now.In(loc)
	end = time.Date(end.Year(), end.Month(), end.Day()+1, 0, 0, 0, 0, loc)
	if q.Get("start") != "" {
		start, e = time.ParseInLocation("2006-01-02", q.Get("start"), loc)
		if e != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid start date"})
			return
		}
	}
	if q.Get("end") != "" {
		end, e = time.ParseInLocation("2006-01-02", q.Get("end"), loc)
		if e != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid exclusive end date"})
			return
		}
	}
	if !start.Before(end) {
		writeJSON(w, 400, map[string]string{"error": "start must precede exclusive end"})
		return
	}
	staff := q.Get("staff_id")
	if staff != "" {
		if _, ok := s.snap().Engine.Store.LibrarianByID[staff]; !ok {
			writeJSON(w, 400, map[string]string{"error": "unknown staff"})
			return
		}
	}
	writeJSON(w, 200, metrics.Aggregate(s.Session.EngagementSnapshot(), metrics.Filter{Start: start, End: end, AsOf: now, StaffID: staff}))
}
