package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"school_district_reading/internal/engagement"
	"school_district_reading/internal/metrics"
)

func TestPairedOutcomesAPI(t *testing.T) {
	s := newTestServer(t)
	var err error
	s.Demo, err = engagement.LoadDemo("../../data/json", s.snap().Engine.Store, s.Academics)
	if err != nil {
		t.Fatal(err)
	}
	cal, err := engagement.LoadCalendar("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	s.Session.SetCalendar(cal)
	h := s.Handler()
	for _, source := range []string{"historical", "session"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/metrics/paired?source="+source, nil))
		if rr.Code != 200 {
			t.Fatalf("%s: %d %s", source, rr.Code, rr.Body.String())
		}
		var report metrics.PairedReport
		if err := json.Unmarshal(rr.Body.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.Source != source {
			t.Fatal("source mismatch")
		}
	}
	for _, q := range []string{"source=invalid", "staff_id=missing", "start=bad", "start=2026-07-01&end=2025-07-01", "as_of=bad", "as_of=9999-01-01T00:00:00Z"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/metrics/paired?"+q, nil))
		if rr.Code != 400 {
			t.Fatalf("%s: %d", q, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/metrics/paired", nil))
	if rr.Code != 405 {
		t.Fatal(rr.Code)
	}
	s.Demo = nil
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/metrics/paired", nil))
	var unavailable metrics.PairedReport
	if err := json.Unmarshal(rr.Body.Bytes(), &unavailable); err != nil {
		t.Fatal(err)
	}
	if rr.Code != 200 || unavailable.Available {
		t.Fatal("missing fixture did not degrade gracefully")
	}
}
