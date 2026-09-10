package httpapi

import (
	"net/http"
	"school_district_reading/internal/metrics"
)

func (s Server) studentProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Query().Get("staff_id") != "" {
		writeJSON(w, 400, map[string]string{"error": "portfolio progress is not attributed to a librarian"})
		return
	}
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "scenario"
	}
	report, err := metrics.Progress(s.snap().Engine.Store, s.Academics, source)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, report)
}
