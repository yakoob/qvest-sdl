package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"school_district_reading/internal/engagement"
)

func TestClockDriverAbsentWithoutEnv(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/test/clock", strings.NewReader(`{"now":"2026-09-11T16:00:00Z"}`)))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("production must not expose the test clock, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestInjectedClockAgendaAndFutureStart(t *testing.T) {
	t.Setenv("SHELFMATE_TEST_DRIVER", "1")
	s := newTestServer(t)
	cal, err := engagement.LoadCalendar("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	s.Session.SetCalendar(cal)
	s.Session.SetClock(func() time.Time { return time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC) })
	h := s.Handler()
	post := func(path, body string, code int) map[string]any {
		t.Helper()
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		if rr.Code != code {
			t.Fatalf("%s got %d want %d: %s", path, rr.Code, code, rr.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}
	get := func(path string) map[string]any {
		t.Helper()
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != 200 {
			t.Fatalf("%s %d %s", path, rr.Code, rr.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}
	booked := post("/api/engagement", `{"action":"schedule","student_id":"S-406","staff_id":"L-002","start":"2026-09-11T09:00","duration":10,"confirmed":true,"request_id":"book","expected_revision":0}`, 200)
	id, _ := booked["id"].(string)
	if id == "" {
		t.Fatalf("missing appointment id: %v", booked)
	}
	early := post("/api/engagement", `{"action":"start","appointment_id":"`+id+`","staff_id":"L-002","request_id":"early","expected_revision":1}`, 400)
	if !strings.Contains(early["error"].(string), "has not started") {
		t.Fatalf("future start: %v", early)
	}
	post("/api/test/clock", `{"now":"2026-09-11T16:05:00Z"}`, 200)
	agenda := get("/api/agenda")
	if agenda["now"] != "2026-09-11T16:05:00Z" {
		t.Fatalf("agenda now %v", agenda["now"])
	}
	started := post("/api/engagement", `{"action":"start","appointment_id":"`+id+`","staff_id":"L-002","request_id":"start","expected_revision":1}`, 200)
	if started["id"] == nil {
		t.Fatalf("start after clock: %v", started)
	}
}
