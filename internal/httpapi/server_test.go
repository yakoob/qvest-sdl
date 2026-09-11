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
	type want struct {
		id, opGrade string
		counts      []float64
		grades      []string
		shortcut    bool
	}
	cases := []want{
		{id: "S-305", opGrade: "B+", counts: []float64{2, 3, 5, 6, 8, 10}, grades: []string{"D", "D+", "C-", "C", "B-", "B+"}, shortcut: true},
		{id: "S-406", opGrade: "A-", counts: []float64{2, 4, 5, 7, 8, 11}, grades: []string{"C-", "C", "C+", "B-", "B", "A-"}, shortcut: true},
		{id: "S-504", opGrade: "A", counts: []float64{1, 3, 4, 6, 7, 9}, grades: []string{"D+", "C", "C+", "B-", "B", "A"}},
	}
	if len(cases) < 3 {
		t.Fatal("need at least three improving scenarios")
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students/"+tc.id+"/academics", nil))
		if rr.Code != 200 {
			t.Fatal(rr.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		acad := payload["academics"].(map[string]any)
		if acad["demo_case"] != "improving_engagement_illustrative" {
			t.Fatalf("%s demo_case %v", tc.id, acad["demo_case"])
		}
		sc := acad["scenario"].(map[string]any)
		if sc["id"] != "improving_engagement_illustrative" || sc["isolated_from_operations"] != true {
			t.Fatalf("%s scenario %+v", tc.id, sc)
		}
		windows := sc["windows"].([]any)
		if len(windows) != academics.CompletedScenarioWindows {
			t.Fatalf("%s windows %d want %d (not just non-nil)", tc.id, len(windows), academics.CompletedScenarioWindows)
		}
		years := map[string]bool{}
		for i, raw := range windows {
			row := raw.(map[string]any)
			years[fmt.Sprint(row["academic_year"])] = true
			if row["inclusive_days"].(float64) != float64(academics.MatchedWindowDays) {
				t.Fatalf("%s window %d days %v", tc.id, i, row["inclusive_days"])
			}
			if row["checkout_count"].(float64) != tc.counts[i] {
				t.Fatalf("%s window %d checkouts %v want %v", tc.id, i, row["checkout_count"], tc.counts[i])
			}
			if row["english_grade"] != tc.grades[i] {
				t.Fatalf("%s window %d grade %v want %s", tc.id, i, row["english_grade"], tc.grades[i])
			}
		}
		if len(years) != academics.CompletedScenarioYears {
			t.Fatalf("%s years %d want %d", tc.id, len(years), academics.CompletedScenarioYears)
		}
		var s2 map[string]any
		for _, raw := range acad["semesters"].([]any) {
			row := raw.(map[string]any)
			if row["id"] == "SY25-S2" {
				s2 = row
			}
		}
		if s2["english_grade"] != tc.opGrade {
			t.Fatalf("%s operational latest grade %v want %s", tc.id, s2["english_grade"], tc.opGrade)
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/students/S-405/academics", nil))
	var priya map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &priya); err != nil {
		t.Fatal(err)
	}
	priyaSc := priya["academics"].(map[string]any)["scenario"].(map[string]any)
	if wins, _ := priyaSc["windows"].([]any); len(wins) != 0 {
		t.Fatalf("Priya invented prior years: %d windows", len(wins))
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
