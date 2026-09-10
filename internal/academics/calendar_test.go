package academics

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestParseDateOnlyUTC(t *testing.T) {
	ok, err := ParseDateOnly("2026-04-26")
	if err != nil {
		t.Fatal(err)
	}
	if ok.Location() != time.UTC || ok.Hour() != 0 || FormatDateOnly(ok) != "2026-04-26" {
		t.Fatalf("%v", ok)
	}
	for _, bad := range []string{"2026-02-29", "2026/04/26", "2026-4-26", "2026-04-26T00:00:00Z", "04-26-2026", ""} {
		if _, err := ParseDateOnly(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestTermBoundsAndOverlap(t *testing.T) {
	s1s, s1e, err := TermBounds("2025-26", "S1")
	if err != nil {
		t.Fatal(err)
	}
	if FormatDateOnly(s1s) != "2025-08-18" || FormatDateOnly(s1e) != "2026-01-16" {
		t.Fatalf("%s %s", s1s, s1e)
	}
	s2s, s2e, err := TermBounds("2025-26", "S2")
	if err != nil {
		t.Fatal(err)
	}
	if RangesOverlap(s1s, s1e, s2s, s2e) {
		t.Fatal("S1 and S2 must not overlap")
	}
	w1s, _ := ParseDateOnly("2026-02-02")
	w1e, _ := ParseDateOnly("2026-04-26")
	if InclusiveDays(w1s, w1e) != MatchedWindowDays {
		t.Fatal("matched window length")
	}
	if !InClosedRange(w1s, s2s, s2e) || !InClosedRange(w1e, s2s, s2e) {
		t.Fatal("window must sit inside S2")
	}
}

func TestExpectedSchoolGradeProgression(t *testing.T) {
	g, err := ExpectedSchoolGrade(3, "2026-27", "2024-25")
	if err != nil || g != 1 {
		t.Fatalf("%d %v", g, err)
	}
	g, err = ExpectedSchoolGrade(4, "2026-27", "2025-26")
	if err != nil || g != 3 {
		t.Fatalf("%d %v", g, err)
	}
}

func TestShuffledChronologicalRecordsSortByDate(t *testing.T) {
	st, cat := loadAll(t)
	rec := cat.ByStudent["S-305"]
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(rec.Semesters), func(i, j int) { rec.Semesters[i], rec.Semesters[j] = rec.Semesters[j], rec.Semesters[i] })
	rng.Shuffle(len(rec.Assessments), func(i, j int) { rec.Assessments[i], rec.Assessments[j] = rec.Assessments[j], rec.Assessments[i] })
	if rec.Scenario != nil {
		rng.Shuffle(len(rec.Scenario.Windows), func(i, j int) {
			rec.Scenario.Windows[i], rec.Scenario.Windows[j] = rec.Scenario.Windows[j], rec.Scenario.Windows[i]
		})
	}
	cat.ByStudent["S-305"] = rec
	view := cat.View("S-305", st)
	for i := 1; i < len(view.Semesters); i++ {
		a, _ := ParseDateOnly(view.Semesters[i-1].Start)
		b, _ := ParseDateOnly(view.Semesters[i].Start)
		if b.Before(a) {
			t.Fatalf("semesters not chronological: %s then %s", view.Semesters[i-1].Start, view.Semesters[i].Start)
		}
	}
	for i := 1; i < len(view.Assessments); i++ {
		a, _ := ParseDateOnly(view.Assessments[i-1].Date)
		b, _ := ParseDateOnly(view.Assessments[i].Date)
		if b.Before(a) {
			t.Fatalf("assessments not chronological: %s then %s", view.Assessments[i-1].Date, view.Assessments[i].Date)
		}
	}
	for i := 1; i < len(view.Scenario.Windows); i++ {
		a, _ := ParseDateOnly(view.Scenario.Windows[i-1].Start)
		b, _ := ParseDateOnly(view.Scenario.Windows[i].Start)
		if b.Before(a) {
			t.Fatalf("windows not chronological: %s then %s", view.Scenario.Windows[i-1].Start, view.Scenario.Windows[i].Start)
		}
	}
}

func TestInvalidDatesRejected(t *testing.T) {
	st, cat := loadAll(t)
	rec := cat.ByStudent["S-406"]
	rec.Semesters[0].Start = "2026-02-29"
	cat.ByStudent["S-406"] = rec
	if err := cat.Validate(st); err == nil {
		t.Fatal("expected invalid leap day")
	}
}

func TestOverlappingScenarioWindowsRejected(t *testing.T) {
	st, cat := loadAll(t)
	rec := cat.ByStudent["S-305"]
	if rec.Scenario == nil || len(rec.Scenario.Windows) < 2 {
		t.Fatal("need scenario windows")
	}
	rec.Scenario.Windows[1].Start = rec.Scenario.Windows[0].Start
	rec.Scenario.Windows[1].End = rec.Scenario.Windows[0].End
	cat.ByStudent["S-305"] = rec
	if err := cat.Validate(st); err == nil {
		t.Fatal("expected overlap error")
	}
}

func TestScenarioIsolationFromOperationalLoans(t *testing.T) {
	st, cat := loadAll(t)
	view := cat.View("S-305", st)
	raw, _ := json.Marshal(st.Circulation)
	if strings.Contains(string(raw), "2024-09-16") {
		t.Fatal("synthetic 2024 loan appeared in operational circulation")
	}
	for _, sem := range view.Semesters {
		for _, b := range sem.Borrowed {
			if strings.HasPrefix(b.CheckoutDate, "2024-") {
				t.Fatalf("scenario loan leaked into extract join %s %s", b.BookID, b.CheckoutDate)
			}
		}
	}
}

func TestDemoStudentsHaveScenarios(t *testing.T) {
	st, cat := loadAll(t)
	ids := []string{"S-406", "S-402", "S-405", "S-509", "S-504", "S-305"}
	for _, id := range ids {
		view := cat.View(id, st)
		if view.Scenario == nil {
			t.Fatalf("%s missing scenario", id)
		}
		if !view.Scenario.IsolatedFromOperations {
			t.Fatalf("%s not isolated", id)
		}
	}
	if len(cat.View("S-405", st).Scenario.Windows) != 0 {
		t.Fatal("newcomer invented prior years")
	}
	if len(cat.View("S-509", st).Scenario.Windows) != 0 {
		t.Fatal("zero-history invented prior years")
	}
}
