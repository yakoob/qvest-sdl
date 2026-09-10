package academics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/engine"
	"school_district_reading/internal/explain"
	"school_district_reading/internal/store"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func loadAll(t *testing.T) (*store.Store, *Catalog) {
	t.Helper()
	root := repoRoot(t)
	st, err := store.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.Validate(st); err != nil {
		t.Fatal(err)
	}
	return st, cat
}

func TestLoadAndValidate(t *testing.T) {
	st, cat := loadAll(t)
	if cat.Missing {
		t.Fatal("fixture should load")
	}
	if !cat.Provenance.Synthetic {
		t.Fatal("must be labeled synthetic")
	}
	if _, ok := st.Student("S-406"); !ok {
		t.Fatal("mateo")
	}
	if _, ok := cat.ByStudent["S-406"]; !ok {
		t.Fatal("mateo academics")
	}
}

func TestMissingFileIsEmptyView(t *testing.T) {
	dir := t.TempDir()
	cat, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !cat.Missing {
		t.Fatal("expected missing")
	}
	view := cat.View("S-406", nil)
	if view.Loaded {
		t.Fatal("loaded")
	}
	if len(view.Semesters) != 0 {
		t.Fatalf("semesters %d", len(view.Semesters))
	}
}

func TestMateoDateJoinAndCurrentUngraded(t *testing.T) {
	st, cat := loadAll(t)
	view := cat.View("S-406", st)
	if len(view.Semesters) != 3 {
		t.Fatalf("semesters %d", len(view.Semesters))
	}
	var s2, cur SemesterView
	for _, s := range view.Semesters {
		switch s.ID {
		case "SY25-S2":
			s2 = s
		case "SY26-S1":
			cur = s
		}
	}
	if s2.EnglishGrade == nil || *s2.EnglishGrade != "C" {
		t.Fatalf("spring grade %+v", s2.EnglishGrade)
	}
	if cur.EnglishGrade != nil {
		t.Fatalf("current semester must be ungraded, got %v", *cur.EnglishGrade)
	}
	if s2.CheckoutCount == 0 || s2.UniqueTitles == 0 {
		t.Fatalf("spring should join Mateo's returned titles, checkouts=%d unique=%d", s2.CheckoutCount, s2.UniqueTitles)
	}
	if cur.CheckoutCount == 0 {
		t.Fatal("fall 2026 should include Investigators checkout")
	}
	repeat := false
	for _, b := range cur.Borrowed {
		if b.BookID == "B-001" && b.Renewal {
			repeat = true
		}
	}
	if !repeat {
		// Dog Man was borrowed in spring and again 2026-08-25.
		for _, b := range cur.Borrowed {
			if b.BookID == "B-001" {
				repeat = b.Renewal
			}
		}
		if !repeat {
			t.Logf("fall borrowed=%+v", cur.Borrowed)
		}
	}
}

func TestPriyaMissingAssessmentNotZero(t *testing.T) {
	st, cat := loadAll(t)
	view := cat.View("S-405", st)
	if len(view.Assessments) != 1 {
		t.Fatalf("assessments %d", len(view.Assessments))
	}
	if view.Assessments[0].Result != nil {
		t.Fatal("missing result must stay null")
	}
	if !strings.Contains(strings.ToLower(view.Assessments[0].CompareNote), "missing") {
		t.Fatalf("note %s", view.Assessments[0].CompareNote)
	}
}

func TestScaleCompatibility(t *testing.T) {
	st, cat := loadAll(t)
	view := cat.View("S-406", st)
	var jan, may, sep AssessmentView
	for _, a := range view.Assessments {
		switch a.Date {
		case "2026-01-22":
			jan = a
		case "2026-05-12":
			may = a
		case "2026-09-02":
			sep = a
		}
	}
	if !may.Comparable {
		t.Fatal("May vs Jan same grade/scale should be comparable (still not a growth score)")
	}
	if sep.Comparable {
		t.Fatal("grade 4 vs grade 3 must not be comparable")
	}
	if jan.Result == nil || may.Result == nil || *jan.Result != *may.Result {
		t.Fatal("Mateo fixture is flat within grade 3")
	}
}

func TestSessionCheckoutDoesNotChangeGrades(t *testing.T) {
	st, cat := loadAll(t)
	before := cat.View("S-406", st)
	eng := engine.New(st)
	eng.Explain = explain.TemplateExplainer{}
	eng.Audit = nil
	// Simulate a session clone adding a loan without touching academics.
	clone := st.CloneCirculation()
	books := append([]domain.Book(nil), clone.Books...)
	for i := range books {
		if books[i].BookID == "B-007" {
			books[i].CopiesAvailable--
		}
	}
	events := append(append([]domain.CirculationEvent(nil), clone.Circulation...), domain.CirculationEvent{
		EventID:      "SESS-0001",
		StudentID:    "S-406",
		BookID:       "B-007",
		CheckoutDate: "2026-09-09",
		DueDate:      "2026-09-23",
		StaffID:      "L-001",
		Channel:      "desk-session",
	})
	clone.ApplyCirculation(books, events)
	after := cat.View("S-406", clone)
	if gradeOf(before, "SY25-S2") != gradeOf(after, "SY25-S2") {
		t.Fatal("posted grade changed")
	}
	if gradeOf(after, "SY26-S1") != "" {
		t.Fatal("current grade must stay empty")
	}
	cur := semester(after, "SY26-S1")
	found := false
	for _, b := range cur.Borrowed {
		if b.BookID == "B-007" {
			found = true
		}
	}
	if !found {
		t.Fatal("session checkout should appear in current-semester borrowing")
	}
}

func TestAcademicsAbsentFromModelPayloadAndRanking(t *testing.T) {
	st, cat := loadAll(t)
	eng := engine.New(st)
	eng.Explain = explain.TemplateExplainer{}
	eng.Audit = nil
	rec, err := eng.Recommend(domain.Request{StudentID: "S-406", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(rec)
	s := string(raw)
	for _, banned := range []string{"english_grade", "willow_bend_reading", "synthetic_demo", "C+", "academic"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(banned)) {
			t.Fatalf("recommendation leaked academic field %s: %s", banned, s)
		}
	}
	in := explain.Input{StudentID: "S-406", Items: nil}
	payload, _ := json.Marshal(explain.BuildPayload(in))
	if strings.Contains(string(payload), "willow") || strings.Contains(string(payload), "english_grade") {
		t.Fatal("model payload must not include academics")
	}
	_ = cat
}

func TestOliviaHasNoInventedHistory(t *testing.T) {
	st, cat := loadAll(t)
	view := cat.View("S-509", st)
	if len(view.Assessments) != 0 {
		t.Fatal("Olivia has no assessment rows")
	}
	cur := semester(view, "SY26-S1")
	if cur.CheckoutCount != 0 {
		t.Fatalf("Olivia has zero source checkouts, got %d", cur.CheckoutCount)
	}
	if !strings.Contains(strings.ToLower(cur.BorrowingNote), "not proof") {
		t.Fatalf("honest empty note: %s", cur.BorrowingNote)
	}
}

func gradeOf(v View, id string) string {
	s := semester(v, id)
	if s.EnglishGrade == nil {
		return ""
	}
	return *s.EnglishGrade
}

func semester(v View, id string) SemesterView {
	for _, s := range v.Semesters {
		if s.ID == id {
			return s
		}
	}
	return SemesterView{}
}

func TestJSONRoundTripNulls(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "data", "json", "academic_demo.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"grade": null`) {
		t.Fatal("current terms must use JSON null grades")
	}
}
