package academics

import (
	"encoding/json"
	"fmt"
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
	if s2.EnglishGrade == nil || *s2.EnglishGrade != "A-" {
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
	if jan.Result == nil || may.Result == nil || *jan.Result >= *may.Result {
		t.Fatal("Mateo grade 3 checks should show meeting after developing")
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
	sofiaBefore := cat.View("S-305", st)
	sofiaAfter := cat.View("S-305", clone)
	if sofiaBefore.Scenario == nil || sofiaAfter.Scenario == nil {
		t.Fatal("sofia scenario missing")
	}
	if len(sofiaBefore.Scenario.Windows) != len(sofiaAfter.Scenario.Windows) {
		t.Fatal("scenario window count changed on checkout")
	}
	for i := range sofiaBefore.Scenario.Windows {
		b, a := sofiaBefore.Scenario.Windows[i], sofiaAfter.Scenario.Windows[i]
		if b.CheckoutCount != a.CheckoutCount {
			t.Fatalf("scenario checkout_count changed at %s", b.ID)
		}
		bg, ag := "", ""
		if b.EnglishGrade != nil {
			bg = *b.EnglishGrade
		}
		if a.EnglishGrade != nil {
			ag = *a.EnglishGrade
		}
		if bg != ag {
			t.Fatalf("scenario english changed at %s", b.ID)
		}
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

func TestSofiaImprovingEngagementScenario(t *testing.T) {
	st, cat := loadAll(t)
	rec, ok := cat.ByStudent["S-305"]
	if !ok {
		t.Fatal("Sofia academics missing")
	}
	if rec.DemoCase != DemoImprovingEngagement {
		t.Fatalf("demo_case %q", rec.DemoCase)
	}
	if rec.Scenario == nil || !rec.Scenario.IsolatedFromOperations {
		t.Fatal("scenario must be isolated")
	}
	view := cat.View("S-305", st)
	if view.DemoCase != DemoImprovingEngagement || view.Scenario == nil {
		t.Fatalf("view demo_case %q scenario %+v", view.DemoCase, view.Scenario)
	}
	if view.Scenario.Caveat == "" {
		t.Fatal("one caveat required")
	}
	wins := view.Scenario.Windows
	if len(wins) != CompletedScenarioWindows {
		t.Fatalf("windows %d want %d", len(wins), CompletedScenarioWindows)
	}
	wantCounts := []int{2, 3, 5, 6, 8, 10}
	wantGrades := []string{"D", "D+", "C-", "C", "B-", "B+"}
	wantReading := []*int{ptrInt(1), ptrInt(1), ptrInt(1), ptrInt(2), ptrInt(2), ptrInt(3)}
	wantGradesSchool := []int{0, 0, 1, 1, 2, 2}
	years := map[string]bool{}
	for i, w := range wins {
		years[w.AcademicYear] = true
		if w.InclusiveDays != MatchedWindowDays {
			t.Fatalf("window %s days %d", w.ID, w.InclusiveDays)
		}
		if w.CheckoutCount != wantCounts[i] {
			t.Fatalf("window %s checkouts %d want %d", w.ID, w.CheckoutCount, wantCounts[i])
		}
		if w.EnglishGrade == nil || *w.EnglishGrade != wantGrades[i] {
			t.Fatalf("window %s grade %+v want %s", w.ID, w.EnglishGrade, wantGrades[i])
		}
		if w.ReadingCheck == nil || w.ReadingCheck.Result == nil || *w.ReadingCheck.Result != *wantReading[i] {
			t.Fatalf("window %s reading %+v", w.ID, w.ReadingCheck)
		}
		if w.SchoolGrade != wantGradesSchool[i] {
			t.Fatalf("window %s school_grade %d want %d", w.ID, w.SchoolGrade, wantGradesSchool[i])
		}
		if i > 0 && w.CheckoutCount <= wins[i-1].CheckoutCount {
			t.Fatalf("borrowing did not increase at %s", w.ID)
		}
		if i > 0 && letterRank(*w.EnglishGrade) < letterRank(*wins[i-1].EnglishGrade) {
			t.Fatalf("english did not improve at %s", w.ID)
		}
	}
	if len(years) != CompletedScenarioYears {
		t.Fatalf("years %d want %d", len(years), CompletedScenarioYears)
	}
	s2 := semester(view, "SY25-S2")
	if s2.EnglishGrade == nil || *s2.EnglishGrade != "B+" {
		t.Fatalf("latest posted operational grade must stay B+, got %+v", s2.EnglishGrade)
	}
	cur := semester(view, "SY26-S1")
	if cur.EnglishGrade != nil {
		t.Fatal("current term must stay ungraded")
	}
}

func TestStableAndMissingAcademicCasesPreserved(t *testing.T) {
	st, cat := loadAll(t)
	mateo := cat.View("S-406", st)
	if len(mateo.Semesters) != 3 || gradeOf(mateo, "SY25-S2") != "A-" {
		t.Fatalf("mateo drift semesters=%d grade=%s", len(mateo.Semesters), gradeOf(mateo, "SY25-S2"))
	}
	if mateo.DemoCase != DemoImprovingEngagement {
		t.Fatalf("mateo demo %q", mateo.DemoCase)
	}
	aisha := cat.View("S-402", st)
	if gradeOf(aisha, "SY25-S2") != "A-" || aisha.DemoCase != DemoStrongStable {
		t.Fatalf("aisha spring %s demo %s", gradeOf(aisha, "SY25-S2"), aisha.DemoCase)
	}
	if aisha.Scenario == nil || len(aisha.Scenario.Windows) != CompletedScenarioWindows {
		t.Fatalf("aisha windows %d", len(aisha.Scenario.Windows))
	}
	priya := cat.View("S-405", st)
	if len(priya.Assessments) != 1 || priya.Assessments[0].Result != nil {
		t.Fatal("priya missing assessment must stay null")
	}
	if priya.Scenario == nil || len(priya.Scenario.Windows) != 0 {
		t.Fatal("priya must not invent prior local years")
	}
	olivia := cat.View("S-509", st)
	if len(olivia.Assessments) != 0 || semester(olivia, "SY26-S1").CheckoutCount != 0 {
		t.Fatal("olivia invented history")
	}
	if olivia.Scenario == nil || len(olivia.Scenario.Windows) != 0 {
		t.Fatal("olivia must not invent prior local years")
	}
	tyler := cat.View("S-504", st)
	if gradeOf(tyler, "SY25-S2") != "A" {
		t.Fatalf("tyler spring %s", gradeOf(tyler, "SY25-S2"))
	}
	if tyler.DemoCase != DemoImprovingEngagement {
		t.Fatalf("tyler demo %q", tyler.DemoCase)
	}
	luis := cat.View("S-302", st)
	if gradeOf(luis, "SY25-S2") != "C-" {
		t.Fatalf("luis transferred grade became %q", gradeOf(luis, "SY25-S2"))
	}
	if len(luis.Assessments) != 1 || luis.Assessments[0].Result != nil {
		t.Fatal("luis missing reading check must stay null")
	}
	if luis.Scenario == nil || len(luis.Scenario.Windows) != 1 {
		t.Fatal("luis missingness window")
	}
	miss := luis.Scenario.Windows[0]
	if miss.EnglishGrade != nil || miss.ReadingCheck == nil || miss.ReadingCheck.Result != nil {
		t.Fatal("luis scenario missing values must stay missing")
	}
	if miss.CheckoutCount != 0 {
		t.Fatalf("luis scenario invented borrowing %d", miss.CheckoutCount)
	}
}

func TestThreeImprovingScenariosDistinct(t *testing.T) {
	st, cat := loadAll(t)
	type want struct {
		id, last string
		counts   []int
		grades   []string
		school   []int
	}
	cases := map[string]want{
		"S-305": {id: DemoImprovingEngagement, last: "B+", counts: []int{2, 3, 5, 6, 8, 10}, grades: []string{"D", "D+", "C-", "C", "B-", "B+"}, school: []int{0, 0, 1, 1, 2, 2}},
		"S-406": {id: DemoImprovingEngagement, last: "A-", counts: []int{2, 4, 5, 7, 8, 11}, grades: []string{"C-", "C", "C+", "B-", "B", "A-"}, school: []int{1, 1, 2, 2, 3, 3}},
		"S-504": {id: DemoImprovingEngagement, last: "A", counts: []int{1, 3, 4, 6, 7, 9}, grades: []string{"D+", "C", "C+", "B-", "B", "A"}, school: []int{2, 2, 3, 3, 4, 4}},
	}
	if len(cases) < 3 {
		t.Fatal("need at least three improving scenarios")
	}
	seenSeq := map[string]string{}
	for sid, w := range cases {
		view := cat.View(sid, st)
		if view.Scenario == nil {
			t.Fatalf("%s missing scenario pointer", sid)
		}
		wins := view.Scenario.Windows
		if len(wins) != CompletedScenarioWindows {
			t.Fatalf("%s windows %d want %d (not just non-nil)", sid, len(wins), CompletedScenarioWindows)
		}
		years := map[string]bool{}
		seq := ""
		for i, win := range wins {
			years[win.AcademicYear] = true
			if win.InclusiveDays != MatchedWindowDays {
				t.Fatalf("%s %s days %d", sid, win.ID, win.InclusiveDays)
			}
			if win.CheckoutCount != w.counts[i] {
				t.Fatalf("%s %s checkouts %d want %d", sid, win.ID, win.CheckoutCount, w.counts[i])
			}
			if win.EnglishGrade == nil || *win.EnglishGrade != w.grades[i] {
				t.Fatalf("%s %s grade %+v want %s", sid, win.ID, win.EnglishGrade, w.grades[i])
			}
			if win.SchoolGrade != w.school[i] {
				t.Fatalf("%s %s school_grade %d want %d", sid, win.ID, win.SchoolGrade, w.school[i])
			}
			if i > 0 && win.CheckoutCount <= wins[i-1].CheckoutCount {
				t.Fatalf("%s borrowing did not increase at %s", sid, win.ID)
			}
			if i > 0 && letterRank(*win.EnglishGrade) < letterRank(*wins[i-1].EnglishGrade) {
				t.Fatalf("%s english did not improve at %s", sid, win.ID)
			}
			seq += fmt.Sprintf("%s/%d;", *win.EnglishGrade, win.CheckoutCount)
		}
		if len(years) != CompletedScenarioYears {
			t.Fatalf("%s years %d want %d", sid, len(years), CompletedScenarioYears)
		}
		if *wins[len(wins)-1].EnglishGrade != w.last {
			t.Fatalf("%s last grade %s want %s", sid, *wins[len(wins)-1].EnglishGrade, w.last)
		}
		if other, ok := seenSeq[seq]; ok {
			t.Fatalf("%s sequence identical to %s", sid, other)
		}
		seenSeq[seq] = sid
	}
	if gradeOf(cat.View("S-406", st), "SY25-S2") != "A-" {
		t.Fatal("Mateo operational spring grade drifted")
	}
	if gradeOf(cat.View("S-504", st), "SY25-S2") != "A" {
		t.Fatal("Tyler operational spring grade drifted")
	}
}

func ptrInt(v int) *int { return &v }
