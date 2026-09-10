package academics_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/domain"
	"school_district_reading/internal/engine"
	"school_district_reading/internal/store"
	"school_district_reading/internal/support"
)

func TestSupportRankingAndBaseFixtureInvariance(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	st, err := store.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := academics.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.Validate(st); err != nil {
		t.Fatal(err)
	}
	wantHash := map[string]string{
		"catalog.json":     "fbba212fef781673b776223c943558617663d942c81328b81a5e12e10ca693a9",
		"circulation.json": "6af4abf7b59c2aea2bb11fb72f2a975d5ce870c70756775ffb5a2c82f9d0bcc1",
		"students.json":    "603d86509f7c7397c27a5c2b680ed1d42464b23773c8629c1bd650f618254dcd",
	}
	for name, sum := range wantHash {
		raw, err := os.ReadFile(filepath.Join(root, "data", "json", name))
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprintf("%x", sha256.Sum256(raw))
		if got != sum {
			t.Fatalf("%s hash %s want %s", name, got, sum)
		}
	}
	eng := engine.New(st)
	wantRank := map[string][]string{
		"S-406": {"B-007", "B-063", "B-061", "B-025", "B-043"},
		"S-504": {"B-003", "B-005", "B-004", "B-006", "B-021"},
		"S-305": {"B-003", "B-034", "B-001", "B-043", "B-004"},
	}
	for id, ids := range wantRank {
		rec, err := eng.Recommend(domain.Request{StudentID: id, Limit: 5})
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		for i, bookID := range ids {
			if rec.Items[i].BookID != bookID {
				t.Fatalf("%s[%d]=%s want %s", id, i, rec.Items[i].BookID, bookID)
			}
		}
	}
	sup, err := support.Load(filepath.Join(root, "data", "json"), st)
	if err != nil {
		t.Fatal(err)
	}
	wantBand := map[string]string{"S-406": support.Soon, "S-504": support.First, "S-305": support.None, "S-405": support.Insufficient, "S-509": support.Insufficient}
	for id, band := range wantBand {
		r := cat.ByStudent[id]
		r.StudentID = id
		got := support.Evaluate(r, sup.ByStudent[id], support.DefaultConfig())
		if got.Band != band {
			t.Fatalf("%s support %s want %s", id, got.Band, band)
		}
	}
}
