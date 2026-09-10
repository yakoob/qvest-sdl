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
	kS1s, kS1e, err := TermBounds("2023-24", "S1")
	if err != nil {
		t.Fatal(err)
	}
	if FormatDateOnly(kS1s) != "2023-08-18" || FormatDateOnly(kS1e) != "2024-01-16" {
		t.Fatalf("2023-24 S1 %s %s", kS1s, kS1e)
	}
	leapS, err := ParseDateOnly("2024-02-05")
	if err != nil {
		t.Fatal(err)
	}
	leapE, err := ParseDateOnly("2024-04-28")
	if err != nil {
		t.Fatal(err)
	}
	if InclusiveDays(leapS, leapE) != MatchedWindowDays {
		t.Fatal("leap-year 84-day window")
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
	g, err = ExpectedSchoolGrade(3, "2026-27", "2023-24")
	if err != nil || g != 0 {
		t.Fatalf("kindergarten %d %v", g, err)
	}
}

func TestShuffledChronologicalRecordsSortByDate(t *testing.T) {
	st, cat := loadAll(t)
	rng := rand.New(rand.NewSource(42))
	if cat.Calendar != nil {
		rng.Shuffle(len(cat.Calendar.MatchedWindows), func(i, j int) {
			cat.Calendar.MatchedWindows[i], cat.Calendar.MatchedWindows[j] = cat.Calendar.MatchedWindows[j], cat.Calendar.MatchedWindows[i]
		})
		rng.Shuffle(len(cat.Calendar.Terms), func(i, j int) {
			cat.Calendar.Terms[i], cat.Calendar.Terms[j] = cat.Calendar.Terms[j], cat.Calendar.Terms[i]
		})
		if err := cat.Validate(st); err != nil {
			t.Fatalf("shuffled calendar: %v", err)
		}
		for i := 1; i < len(cat.Calendar.MatchedWindows); i++ {
			a, _ := ParseDateOnly(cat.Calendar.MatchedWindows[i-1].Start)
			b, _ := ParseDateOnly(cat.Calendar.MatchedWindows[i].Start)
			if b.Before(a) {
				t.Fatalf("calendar windows not chronological: %s then %s", cat.Calendar.MatchedWindows[i-1].Start, cat.Calendar.MatchedWindows[i].Start)
			}
		}
	}
	for _, id := range []string{"S-305", "S-406", "S-504"} {
		rec := cat.ByStudent[id]
		rng.Shuffle(len(rec.Semesters), func(i, j int) { rec.Semesters[i], rec.Semesters[j] = rec.Semesters[j], rec.Semesters[i] })
		rng.Shuffle(len(rec.Assessments), func(i, j int) { rec.Assessments[i], rec.Assessments[j] = rec.Assessments[j], rec.Assessments[i] })
		if rec.Scenario != nil {
			rng.Shuffle(len(rec.Scenario.Windows), func(i, j int) {
				rec.Scenario.Windows[i], rec.Scenario.Windows[j] = rec.Scenario.Windows[j], rec.Scenario.Windows[i]
			})
		}
		cat.ByStudent[id] = rec
		view := cat.View(id, st)
		for i := 1; i < len(view.Semesters); i++ {
			a, _ := ParseDateOnly(view.Semesters[i-1].Start)
			b, _ := ParseDateOnly(view.Semesters[i].Start)
			if b.Before(a) {
				t.Fatalf("%s semesters not chronological: %s then %s", id, view.Semesters[i-1].Start, view.Semesters[i].Start)
			}
		}
		for i := 1; i < len(view.Assessments); i++ {
			a, _ := ParseDateOnly(view.Assessments[i-1].Date)
			b, _ := ParseDateOnly(view.Assessments[i].Date)
			if b.Before(a) {
				t.Fatalf("%s assessments not chronological: %s then %s", id, view.Assessments[i-1].Date, view.Assessments[i].Date)
			}
		}
		if view.Scenario == nil || len(view.Scenario.Windows) != CompletedScenarioWindows {
			t.Fatalf("%s windows %d", id, len(view.Scenario.Windows))
		}
		for i := 1; i < len(view.Scenario.Windows); i++ {
			a, _ := ParseDateOnly(view.Scenario.Windows[i-1].Start)
			b, _ := ParseDateOnly(view.Scenario.Windows[i].Start)
			if b.Before(a) {
				t.Fatalf("%s windows not chronological: %s then %s", id, view.Scenario.Windows[i-1].Start, view.Scenario.Windows[i].Start)
			}
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
	st, cat = loadAll(t)
	rec = cat.ByStudent["S-305"]
	rec.Scenario.Windows[0].Start = "2026-02-29"
	cat.ByStudent["S-305"] = rec
	if err := cat.Validate(st); err == nil {
		t.Fatal("expected invalid scenario leap day")
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
	if cat.Calendar == nil || len(cat.Calendar.MatchedWindows) != CompletedScenarioWindows {
		t.Fatalf("calendar windows %d", len(cat.Calendar.MatchedWindows))
	}
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
	for _, id := range []string{"S-305", "S-406", "S-504", "S-402"} {
		n := len(cat.View(id, st).Scenario.Windows)
		if n != CompletedScenarioWindows {
			t.Fatalf("%s windows %d want %d", id, n, CompletedScenarioWindows)
		}
	}
	if len(cat.View("S-405", st).Scenario.Windows) != 0 {
		t.Fatal("newcomer invented prior years")
	}
	if len(cat.View("S-509", st).Scenario.Windows) != 0 {
		t.Fatal("zero-history invented prior years")
	}
}

func TestFutureCompletedObservationRejected(t *testing.T) {
	st, cat := loadAll(t)
	rec := cat.ByStudent["S-305"]
	rec.Scenario.Windows[len(rec.Scenario.Windows)-1].End = "2026-10-01"
	cat.ByStudent["S-305"] = rec
	if err := cat.Validate(st); err == nil {
		t.Fatal("expected future completed observation")
	}
}

func TestMismatchedWindowLengthRejected(t *testing.T) {
	st, cat := loadAll(t)
	rec := cat.ByStudent["S-305"]
	rec.Scenario.Windows[0].End = "2023-12-10"
	cat.ByStudent["S-305"] = rec
	if err := cat.Validate(st); err == nil {
		t.Fatal("expected mismatched window length")
	}
}
