package store

import (
	"path/filepath"
	"runtime"
	"testing"
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
