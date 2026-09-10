package policy

import (
	"path/filepath"
	"runtime"
	"testing"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/store"
)

func TestGradeOK(t *testing.T) {
	book := domain.Book{GradeMin: 4, GradeMax: 8}
	if !GradeOK(4, book, false) {
		t.Fatal("grade 4 in 4-8")
	}
	if GradeOK(3, book, false) {
		t.Fatal("grade 3 should fail without stretch")
	}
	if !GradeOK(3, book, true) {
		t.Fatal("stretch allows grade+1 on min")
	}
}

func TestStretchBoundaryFixture(t *testing.T) {
	// grade 4, book listed 5-8: ineligible until librarian stretch.
	book := domain.Book{BookID: "X-STRETCH", GradeMin: 5, GradeMax: 8, CopiesAvailable: 2, Title: "Boundary"}
	if GradeOK(4, book, false) {
		t.Fatal("grade_min 5 must fail for grade 4 without stretch")
	}
	if !GradeOK(4, book, true) {
		t.Fatal("stretch should allow grade_min <= grade+1")
	}
	tooHigh := domain.Book{GradeMin: 6, GradeMax: 8}
	if GradeOK(4, tooHigh, true) {
		t.Fatal("stretch is one grade only")
	}
	tooLowMax := domain.Book{GradeMin: 2, GradeMax: 3}
	if GradeOK(4, tooLowMax, true) {
		t.Fatal("stretch must still require grade_max >= grade")
	}
}

func TestCatalogMembershipAndLiveAvailability(t *testing.T) {
	f := loadFilter(t)
	st, _ := f.Store.Student("S-406")
	ranked := []domain.ScoredBook{
		{Book: domain.Book{BookID: "B-FAKE", Title: "Invented", CopiesAvailable: 9, GradeMin: 1, GradeMax: 8}},
		{Book: domain.Book{BookID: "B-008", Title: "The Bad Guys", CopiesAvailable: 99, GradeMin: 2, GradeMax: 5}},
		{Book: f.Store.BookByID["B-007"]},
	}
	keep, dropped := f.Apply(st, domain.Request{}, ranked)
	if len(keep) != 1 || keep[0].Book.BookID != "B-007" {
		t.Fatalf("keep=%v", ids(keep))
	}
	if keep[0].Book.CopiesAvailable != f.Store.BookByID["B-007"].CopiesAvailable {
		t.Fatal("policy must copy canonical catalog fields")
	}
	why := map[string]string{}
	for _, d := range dropped {
		why[d.BookID] = d.Why
	}
	if why["B-FAKE"] != "not in catalog" {
		t.Fatalf("fake book: %q", why["B-FAKE"])
	}
	if why["B-008"] != "copies_available=0" {
		t.Fatalf("B-008: %q (must use store copies, not candidate)", why["B-008"])
	}
}

func TestPreviouslyBorrowedExcluded(t *testing.T) {
	f := loadFilter(t)
	st, _ := f.Store.Student("S-406")
	// Investigators is in Mateo's history; conservative exclusion of any prior checkout.
	b := f.Store.BookByID["B-006"]
	keep, dropped := f.Apply(st, domain.Request{}, []domain.ScoredBook{{Book: b}})
	if len(keep) != 0 {
		t.Fatal("already-borrowed title must be excluded")
	}
	if len(dropped) != 1 || dropped[0].Why != "already checked out" {
		t.Fatalf("dropped=%v", dropped)
	}
}

func TestUnder150IsStrictlyFewerThan150(t *testing.T) {
	f := loadFilter(t)
	st := domain.Student{StudentID: "S-TEST", Grade: 4}
	// Use canonical rows.
	under := f.Store.BookByID["B-043"] // 139
	bound := f.Store.BookByID["B-003"] // 144
	if bound.Pages >= 150 {
		t.Fatalf("fixture unexpected pages %d", bound.Pages)
	}
	keep, dropped := f.Apply(st, domain.Request{Query: "under 150 pages"}, []domain.ScoredBook{
		{Book: under},
		{Book: bound},
		{Book: f.Store.BookByID["B-002"]}, // 224
	})
	got := ids(keep)
	if !contains(got, "B-043") || !contains(got, "B-003") {
		t.Fatalf("books under 150 should keep, got %v dropped %v", got, dropped)
	}
	if contains(got, "B-002") {
		t.Fatal("224 pages must drop for under 150")
	}

	// Exact 150 would drop. Synthesize via a real book if none: Lunch Lady is 96.
	// Copy a catalog row and... no, membership requires catalog id. Use B-023 Bridge 163.
	keep2, _ := f.Apply(st, domain.Request{Query: "under 150 pages"}, []domain.ScoredBook{
		{Book: f.Store.BookByID["B-023"]},
	})
	if len(keep2) != 0 {
		t.Fatalf("163 pages must be excluded by under 150, kept %v", ids(keep2))
	}
}

func TestShortDefaultIsNotUnder150(t *testing.T) {
	if ParseConstraints("short").Short == false {
		t.Fatal("short flag")
	}
	if ParseConstraints("short").Under150 {
		t.Fatal("short must not imply under 150")
	}
	if !ParseConstraints("funny, reluctant 4th, under 150 pages").Under150 {
		t.Fatal("under 150 pages")
	}
	f := loadFilter(t)
	st := domain.Student{StudentID: "S-TEST", Grade: 4}
	mid := f.Store.BookByID["B-023"] // 163 pages: short-ok, under-150-not
	if mid.Pages <= 150 || mid.Pages > ShortPageDefault {
		t.Fatalf("need a 151-180 page fixture, got %d", mid.Pages)
	}
	keepShort, _ := f.Apply(st, domain.Request{Query: "short"}, []domain.ScoredBook{{Book: mid}})
	if len(keepShort) != 1 {
		t.Fatalf("163 pages should pass default short cap %d", ShortPageDefault)
	}
	keepUnder, _ := f.Apply(st, domain.Request{Query: "under 150 pages"}, []domain.ScoredBook{{Book: mid}})
	if len(keepUnder) != 0 {
		t.Fatal("163 pages must fail under 150")
	}
	long := f.Store.BookByID["B-011"] // 309
	keepLong, dropped := f.Apply(st, domain.Request{Query: "a short book please"}, []domain.ScoredBook{{Book: long}})
	if len(keepLong) != 0 {
		t.Fatalf("long book kept: %v", keepLong)
	}
	if dropped[0].Why != "query asked for a short book" {
		t.Fatalf("why=%q", dropped[0].Why)
	}
}

func TestAishaPreferTitlesAlreadyGradeEligible(t *testing.T) {
	f := loadFilter(t)
	aisha, _ := f.Store.Student("S-402")
	if aisha.Grade != 4 {
		t.Fatalf("grade %d", aisha.Grade)
	}
	for _, id := range []string{"B-013", "B-016", "B-017"} {
		b := f.Store.BookByID[id]
		if !GradeOK(aisha.Grade, b, false) {
			t.Fatalf("%s g%d-%d should already be eligible for grade 4; stretch is not required to unlock it", id, b.GradeMin, b.GradeMax)
		}
	}
	westing := f.Store.BookByID["B-051"]
	if GradeOK(aisha.Grade, westing, false) {
		t.Fatal("Westing Game should be outside default grade band")
	}
	if !GradeOK(aisha.Grade, westing, true) {
		t.Fatal("stretch unlocks Westing Game eligibility; ranking/policy still must not prefer it")
	}
}

func loadFilter(t *testing.T) Filter {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	s, err := store.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	return Filter{Store: s}
}

func ids(rows []domain.ScoredBook) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Book.BookID)
	}
	return out
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
