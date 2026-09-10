package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"school_district_reading/internal/support"
)

func TestSupportWorkflow(t *testing.T) {
	s := newTestServer(t)
	var err error
	s.Support, err = support.Load("../../data/json", s.snap().Engine.Store)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	for id, band := range map[string]string{"S-504": "first", "S-406": "soon", "S-405": "insufficient", "S-402": "none"} {
		w := request("GET", "/api/students/"+id+"/support", "")
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		var v struct {
			Support support.Result `json:"support"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if v.Support.Band != band {
			t.Fatalf("%s: %+v", id, v.Support)
		}
	}
	queue := request("GET", "/api/support/queue", "").Body.String()
	for _, secret := range []string{"teacher_notes", "guidance", "Approved reading", "strengths"} {
		if strings.Contains(queue, secret) {
			t.Fatal("queue leaked detail")
		}
	}
	before := s.supportResult("S-504")
	body := `{"student_id":"S-504","staff_id":"L-001","action":"check_in","retry_id":"test-1"}`
	a := request("POST", "/api/support/followups", body)
	b := request("POST", "/api/support/followups", body)
	if a.Code != 200 || a.Body.String() != b.Body.String() {
		t.Fatal("retry not idempotent")
	}
	if s.supportResult("S-504").Band != before.Band {
		t.Fatal("followup changed band")
	}
	if request("POST", "/api/support/followups", strings.Replace(body, "check_in", "unknown", 1)).Code != 400 {
		t.Fatal("invalid action accepted")
	}
	if request("POST", "/api/recommend", `{"student_id":"S-504","theme":"private"}`).Code != 400 {
		t.Fatal("invalid theme accepted")
	}
	cross := httptest.NewRequest(http.MethodPost, "/api/support/followups", strings.NewReader(body))
	cross.Header.Set("Origin", "https://other.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, cross)
	if w.Code != 403 {
		t.Fatal("cross origin accepted")
	}
	fresh := s.Handler()
	w = httptest.NewRecorder()
	fresh.ServeHTTP(w, httptest.NewRequest("GET", "/api/students/S-504/support", nil))
	if !strings.Contains(w.Body.String(), `"followups":[]`) {
		t.Fatal("followups persisted to new handler")
	}
}
