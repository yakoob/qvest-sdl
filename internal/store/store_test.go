package store

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"school_district_reading/internal/domain"
)

func TestLoadJSON(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	s, err := Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Books) < 40 {
		t.Fatalf("catalog too small: %d", len(s.Books))
	}
	if _, ok := s.Student("S-406"); !ok {
		t.Fatal("missing Mateo")
	}
	if s.BookByID["B-008"].CopiesAvailable != 0 {
		t.Fatal("Bad Guys should have 0 copies")
	}
	if len(s.History["S-405"]) != 1 {
		t.Fatalf("Priya should have 1 checkout, got %d", len(s.History["S-405"]))
	}
	if len(s.History["S-509"]) != 0 {
		t.Fatal("Olivia should have zero checkouts")
	}
}

func TestCloneCirculationDoesNotShareMutableSlices(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	s, err := Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	origCopies := s.BookByID["B-007"].CopiesAvailable
	origCirc := len(s.Circulation)
	origHist := len(s.History["S-406"])
	clone := s.CloneCirculation()
	if clone == s {
		t.Fatal("clone must be a new Store")
	}

	clone.Books[0].CopiesAvailable = clone.Books[0].CopiesAvailable + 9
	clone.Circulation = append(clone.Circulation, domain.CirculationEvent{
		EventID:   "SESS-TEST",
		StudentID: "S-406",
		BookID:    "B-007",
	})
	clone.reindexCirculation()

	if s.BookByID["B-007"].CopiesAvailable != origCopies {
		t.Fatal("clone mutation leaked into original copies")
	}
	if len(s.Circulation) != origCirc {
		t.Fatal("clone mutation leaked into original circulation")
	}
	if len(s.History["S-406"]) != origHist {
		t.Fatal("clone mutation leaked into original history")
	}
	if len(clone.Circulation) != origCirc+1 {
		t.Fatalf("clone circ %d", len(clone.Circulation))
	}
	if len(clone.History["S-406"]) != origHist+1 {
		t.Fatalf("clone history %d", len(clone.History["S-406"]))
	}
	if &s.Students[0] != &clone.Students[0] {
		t.Fatal("students should remain shared (read-only)")
	}
}

func TestFrozenExtractHashesUnchanged(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	want := map[string]string{
		"catalog.json":     "fbba212fef781673b776223c943558617663d942c81328b81a5e12e10ca693a9",
		"circulation.json": "6af4abf7b59c2aea2bb11fb72f2a975d5ce870c70756775ffb5a2c82f9d0bcc1",
		"students.json":    "603d86509f7c7397c27a5c2b680ed1d42464b23773c8629c1bd650f618254dcd",
	}
	for name, sum := range want {
		raw, err := os.ReadFile(filepath.Join(root, "data", "json", name))
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprintf("%x", sha256.Sum256(raw))
		if got != sum {
			t.Fatalf("%s hash %s want %s — do not edit source extracts", name, got, sum)
		}
	}
}
