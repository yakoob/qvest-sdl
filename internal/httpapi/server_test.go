package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/engine"
	"school_district_reading/internal/explain"
	"school_district_reading/internal/session"
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
	if rec.Revision < 1 {
		t.Fatal("revision")
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
	if _, ok := payload["loans"]; !ok {
		t.Fatal("loans required")
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

func TestCheckoutAndReturnHTTP(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/checkouts", strings.NewReader(`{"student_id":"S-509","book_id":"B-007","staff_id":"L-002","retry_id":"t1"}`)))
	if rr.Code != 200 {
		t.Fatalf("checkout %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	loan := out["loan"].(map[string]any)
	loanID := loan["loan_id"].(string)
	if loanID == "" {
		t.Fatal("loan id")
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students/S-509", nil))
	var detail map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &detail)
	loans := detail["loans"].([]any)
	if len(loans) == 0 {
		t.Fatal("expected open loan")
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/checkouts", strings.NewReader(`{"student_id":"S-509","book_id":"B-007","staff_id":"L-002","retry_id":"t1"}`)))
	if rr.Code != 200 {
		t.Fatalf("retry %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/returns", strings.NewReader(`{"student_id":"S-509","loan_id":"`+loanID+`","staff_id":"L-002","retry_id":"r1"}`)))
	if rr.Code != 200 {
		t.Fatalf("return %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/activity", nil))
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var act map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &act)
	items := act["items"].([]any)
	if len(items) < 2 {
		t.Fatalf("activity %v", items)
	}
}

func TestCheckoutZeroCopiesConflict(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/checkouts", strings.NewReader(`{"student_id":"S-406","book_id":"B-008","staff_id":"L-001"}`)))
	if rr.Code != http.StatusConflict {
		t.Fatalf("code %d %s", rr.Code, rr.Body.String())
	}
}

func TestCrossOriginPostRejected(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/checkouts", strings.NewReader(`{"student_id":"S-406","book_id":"B-007","staff_id":"L-001"}`))
	req.Host = "127.0.0.1:8088"
	req.Header.Set("Origin", "http://evil.example")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code %d", rr.Code)
	}
}

func TestAcademicsEndpoint(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students/S-406/academics", nil))
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	acad := payload["academics"].(map[string]any)
	if acad["synthetic"] != true {
		t.Fatal("synthetic")
	}
	if acad["source_label"] != "synthetic_demo" {
		t.Fatalf("label %v", acad["source_label"])
	}
}

func TestSofiaImprovingAcademicsHTTP(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students/S-305/academics", nil))
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	acad := payload["academics"].(map[string]any)
	if acad["demo_case"] != "improving_engagement_illustrative" {
		t.Fatalf("demo_case %v", acad["demo_case"])
	}
	note := strings.ToLower(fmt.Sprint(acad["demo_note"]))
	if !strings.Contains(note, "illustrative") {
		t.Fatalf("demo_note %v", acad["demo_note"])
	}
	sc := acad["scenario"].(map[string]any)
	if sc["id"] != "improving_engagement_illustrative" || sc["isolated_from_operations"] != true {
		t.Fatalf("scenario %+v", sc)
	}
	windows := sc["windows"].([]any)
	if len(windows) != 4 {
		t.Fatalf("windows %d", len(windows))
	}
	wantCounts := []float64{2, 4, 6, 9}
	wantGrades := []string{"D+", "C-", "C", "B+"}
	for i, raw := range windows {
		row := raw.(map[string]any)
		if row["inclusive_days"].(float64) != 84 {
			t.Fatalf("window %d days %v", i, row["inclusive_days"])
		}
		if row["checkout_count"].(float64) != wantCounts[i] {
			t.Fatalf("window %d checkouts %v want %v", i, row["checkout_count"], wantCounts[i])
		}
		if row["english_grade"] != wantGrades[i] {
			t.Fatalf("window %d grade %v want %s", i, row["english_grade"], wantGrades[i])
		}
	}
	var s2 map[string]any
	for _, raw := range acad["semesters"].([]any) {
		row := raw.(map[string]any)
		if row["id"] == "SY25-S2" {
			s2 = row
		}
	}
	if s2["english_grade"] != "B+" {
		t.Fatalf("operational latest grade %v", s2["english_grade"])
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students", nil))
	var list map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	found := false
	for _, raw := range list["students"].([]any) {
		row := raw.(map[string]any)
		if row["student_id"] == "S-305" {
			found = true
			if row["shortcut"] != true {
				t.Fatal("Sofia should be a demo shortcut")
			}
		}
	}
	if !found {
		t.Fatal("S-305 missing from list")
	}
}

func TestCheckoutDoesNotChangeAcademicGradesOverHTTP(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	grade := func() any {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students/S-406/academics", nil))
		var payload map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &payload)
		acad := payload["academics"].(map[string]any)
		for _, raw := range acad["semesters"].([]any) {
			row := raw.(map[string]any)
			if row["id"] == "SY25-S2" {
				return row["english_grade"]
			}
		}
		return nil
	}
	before := grade()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/checkouts", strings.NewReader(`{"student_id":"S-406","book_id":"B-007","staff_id":"L-001"}`)))
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	after := grade()
	if before != after {
		t.Fatalf("%v vs %v", before, after)
	}
}

func TestRecommendAfterCheckoutDropsTitle(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/checkouts", strings.NewReader(`{"student_id":"S-406","book_id":"B-007","staff_id":"L-001"}`)))
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/recommend", strings.NewReader(`{"student_id":"S-406","staff_id":"L-001","limit":8}`)))
	var rec recResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &rec)
	for _, it := range rec.Items {
		if it.BookID == "B-007" {
			t.Fatal("checked out title still recommended")
		}
	}
}

func TestConcurrentLastCopyHTTP(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	bodies := []string{
		`{"student_id":"S-509","book_id":"B-003","staff_id":"L-001","retry_id":"a"}`,
		`{"student_id":"S-405","book_id":"B-003","staff_id":"L-001","retry_id":"b"}`,
	}
	for _, b := range bodies {
		b := b
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/checkouts", strings.NewReader(b)))
			codes <- rr.Code
		}()
	}
	wg.Wait()
	close(codes)
	var ok, conflict int
	for c := range codes {
		switch c {
		case 200:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Fatalf("code %d", c)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("ok=%d conflict=%d", ok, conflict)
	}
}

func TestStudentsListHasRevision(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students", nil))
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["revision"]; !ok {
		t.Fatal("revision")
	}
	if _, ok := payload["students"]; !ok {
		t.Fatal("students")
	}
}

func newTestServer(t *testing.T) Server {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	dir := filepath.Join(root, "data", "json")
	st, err := store.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	eng := engine.New(st)
	eng.Explain = explain.TemplateExplainer{}
	eng.Audit = nil
	cat, err := academics.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.Validate(st); err != nil {
		t.Fatal(err)
	}
	return Server{Session: session.New(eng), Academics: cat}
}
