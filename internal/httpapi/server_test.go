package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"school_district_reading/internal/engine"
	"school_district_reading/internal/store"
)

func TestMethodAndUnknownStudent(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/recommend", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET recommend %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	body := `{"student_id":"S-NOPE"}`
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/recommend", strings.NewReader(body)))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown student %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students/S-NOPE", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown get %d", rr.Code)
	}
}

func TestRecommendDTODoesNotExposeLastNameOrDOB(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/recommend", strings.NewReader(`{"student_id":"S-406","staff_id":"L-002"}`))
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	s := rr.Body.String()
	for _, banned := range []string{"last_name", "dob", "date_of_birth", "anecdote"} {
		if strings.Contains(strings.ToLower(s), banned) {
			t.Fatalf("leaked %s", banned)
		}
	}
	var rec recResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.StudentID != "S-406" || len(rec.Items) == 0 {
		t.Fatalf("%+v", rec)
	}
}

func TestStudentDetailIsAllowlisted(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students/S-406", nil))
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	st := payload["student"].(map[string]any)
	if _, ok := st["lexile_approx"]; ok {
		t.Fatal("lexile should not be in console DTO")
	}
	if st["first_name"] != "Mateo" {
		t.Fatalf("%v", st["first_name"])
	}
}

func TestBodyLimit(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	rr := httptest.NewRecorder()
	big := bytes.Repeat([]byte("a"), maxBodyBytes+10)
	req := httptest.NewRequest(http.MethodPost, "/api/recommend", bytes.NewReader(append([]byte(`{"student_id":"`), append(big, []byte(`"}`)...)...)))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge && rr.Code != http.StatusBadRequest {
		t.Fatalf("code %d", rr.Code)
	}
}

func newTestServer(t *testing.T) Server {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	st, err := store.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	return Server{Engine: engine.New(st)}
}
