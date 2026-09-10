package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"school_district_reading/internal/engagement"
	"strings"
	"testing"
)

func TestEngagementHTTPJourneyAndGuards(t *testing.T) {
	s := newTestServer(t)
	cal, err := engagement.LoadCalendar("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	s.Session.SetCalendar(cal)
	h := s.Handler()
	post := func(body string, code int) map[string]any {
		t.Helper()
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/engagement", strings.NewReader(body)))
		if rr.Code != code {
			t.Fatalf("got %d want %d: %s", rr.Code, code, rr.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}
	post(`{"action":"start","student_id":"S-406","staff_id":"L-001","request_id":"start","expected_revision":0}`, 200)
	post(`{"action":"start","student_id":"S-406","staff_id":"L-001","request_id":"start","expected_revision":0}`, 200)
	post(`{"action":"start","student_id":"S-405","staff_id":"L-001","request_id":"start","expected_revision":0}`, 409)
	post(`{"action":"complete","interaction_id":"ENG-000001","request_id":"stale","expected_revision":0}`, 409)
	post(`{"action":"complete","interaction_id":"ENG-000001","request_id":"complete","expected_revision":1}`, 200)
	post(`{"action":"start","unknown":true}`, 400)
	post(strings.Repeat("x", maxBodyBytes+1), 413)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/engagement", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://elsewhere.example")
	h.ServeHTTP(rr, req)
	if rr.Code != 403 {
		t.Fatal(rr.Code)
	}
	for _, path := range []string{"/api/agenda", "/api/metrics"} {
		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if rr.Code != 200 {
			t.Fatalf("%s: %d", path, rr.Code)
		}
	}
	for _, path := range []string{"/api/metrics?source=historical", "/api/metrics?start=bad", "/api/metrics?start=2026-10-01&end=2026-09-01", "/api/metrics?staff_id=unknown"} {
		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if rr.Code != 400 {
			t.Fatalf("%s: %d", path, rr.Code)
		}
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/engagement", nil))
	if rr.Code != 405 {
		t.Fatal(rr.Code)
	}
}
