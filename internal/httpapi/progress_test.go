package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"school_district_reading/internal/metrics"
)

func TestStudentProgressAPI(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()
	for _, source := range []string{"scenario", "extract"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/metrics/progress?source="+source, nil))
		if rr.Code != 200 {
			t.Fatalf("%s: %d %s", source, rr.Code, rr.Body.String())
		}
		var report metrics.ProgressReport
		if err := json.Unmarshal(rr.Body.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.Source != source {
			t.Fatalf("wrong source %q", report.Source)
		}
	}
	for _, path := range []string{"/api/metrics/progress?source=invalid", "/api/metrics/progress?staff_id=L-001"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if rr.Code != 400 {
			t.Fatalf("%s: %d", path, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/metrics/progress", nil))
	if rr.Code != 405 {
		t.Fatal(rr.Code)
	}
}
