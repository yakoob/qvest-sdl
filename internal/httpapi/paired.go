package httpapi

import (
	"net/http"
	"school_district_reading/internal/metrics"
	"time"
)

func (s Server) pairedOutcomes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	q := r.URL.Query()
	source := q.Get("source")
	if source == "" {
		source = "historical"
	}
	loc, err := time.LoadLocation(s.Session.EngagementTimezone())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "school timezone unavailable"})
		return
	}
	now := time.Now().UTC()
	asof := now
	if value := q.Get("as_of"); value != "" {
		asof, err = time.Parse(time.RFC3339, value)
		if err != nil || asof.After(now) {
			writeJSON(w, 400, map[string]string{"error": "as_of must be an RFC3339 instant not in the future"})
			return
		}
	}
	start := time.Date(2025, 7, 1, 0, 0, 0, 0, loc)
	end := time.Date(2026, 7, 1, 0, 0, 0, 0, loc)
	if source == "session" {
		v := s.Session.Snapshot().StartedAt.In(loc)
		start = time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, loc)
		v = now.In(loc)
		end = time.Date(v.Year(), v.Month(), v.Day()+1, 0, 0, 0, 0, loc)
	}
	for key, target := range map[string]*time.Time{"start": &start, "end": &end} {
		if value := q.Get(key); value != "" {
			v, e := time.ParseInLocation("2006-01-02", value, loc)
			if e != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid " + key + " date"})
				return
			}
			*target = v
		}
	}
	result, err := metrics.Paired(s.snap().Engine.Store, s.Academics, s.Demo, s.Session.EngagementSnapshot(), source, metrics.Filter{Start: start, End: end, AsOf: asof, StaffID: q.Get("staff_id")})
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, result)
}
