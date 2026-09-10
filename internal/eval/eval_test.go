package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/engine"
	"school_district_reading/internal/policy"
	"school_district_reading/internal/store"
)

type goldenFile struct {
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	ID             string   `json:"id"`
	StudentID      string   `json:"student_id"`
	Stretch        bool     `json:"stretch"`
	Query          string   `json:"query"`
	MustIncludeAny []string `json:"must_include_any"`
	MustExclude    []string `json:"must_exclude"`
	ClusterAny     []string `json:"cluster_any"`
	PreferAny      []string `json:"prefer_any"`
	MinItems       int      `json:"min_items"`
	MaxPages       int      `json:"max_pages"`
}

func TestGoldens(t *testing.T) {
	root := repoRoot(t)
	st, err := store.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	eng := engine.New(st)
	raw, err := os.ReadFile(filepath.Join(root, "testdata", "golden", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var gf goldenFile
	if err := json.Unmarshal(raw, &gf); err != nil {
		t.Fatal(err)
	}
	for _, c := range gf.Cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			rec, err := eng.Recommend(domain.Request{
				StudentID: c.StudentID,
				StaffID:   "L-001",
				Query:     c.Query,
				Stretch:   c.Stretch,
				Limit:     5,
			})
			if err != nil {
				t.Fatal(err)
			}
			ids := map[string]domain.RecItem{}
			for _, it := range rec.Items {
				ids[it.BookID] = it
				if _, ok := st.BookByID[it.BookID]; !ok {
					t.Fatalf("invented title %s", it.BookID)
				}
				if it.CopiesAvailable <= 0 {
					t.Fatalf("recommended zero-copy title %s", it.BookID)
				}
			}
			if c.MinItems > 0 && len(rec.Items) < c.MinItems {
				t.Fatalf("want >= %d recs, got %d", c.MinItems, len(rec.Items))
			}
			if len(c.MustIncludeAny) > 0 && !anyHit(ids, c.MustIncludeAny) {
				t.Fatalf("want one of %v in %v", c.MustIncludeAny, keys(ids))
			}
			for _, bad := range c.MustExclude {
				if _, ok := ids[bad]; ok {
					t.Fatalf("must not recommend %s", bad)
				}
			}
			if len(c.ClusterAny) > 0 {
				ok := false
				for _, it := range rec.Items {
					for _, cl := range c.ClusterAny {
						if it.Cluster == cl {
							ok = true
						}
					}
				}
				if !ok {
					t.Fatalf("want cluster %v in recs", c.ClusterAny)
				}
			}
			if c.MaxPages > 0 {
				for _, it := range rec.Items {
					if it.Pages > c.MaxPages {
						t.Fatalf("%s has %d pages, query asked short", it.BookID, it.Pages)
					}
				}
			}
			if policy.ParseConstraints(c.Query).Under150 {
				for _, it := range rec.Items {
					if it.Pages >= 150 {
						t.Fatalf("%s has %d pages; under 150 means fewer than 150", it.BookID, it.Pages)
					}
				}
			}
			if len(c.PreferAny) > 0 && !anyHit(ids, c.PreferAny) {
				t.Fatalf("stretch prefer %v not in %v", c.PreferAny, keys(ids))
			}
		})
	}
}

func TestBadGuysNeverSpoken(t *testing.T) {
	root := repoRoot(t)
	st, err := store.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.BookByID["B-008"].CopiesAvailable != 0 {
		t.Fatal("fixture drift: B-008 should be 0 copies")
	}
	eng := engine.New(st)
	for _, sid := range []string{"S-406", "S-301", "S-504"} {
		rec, err := eng.Recommend(domain.Request{StudentID: sid, Limit: 8})
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range rec.Items {
			if it.BookID == "B-008" {
				t.Fatalf("%s received Bad Guys", sid)
			}
			if it.TalkingPoint != "" && containsID(it.TalkingPoint, "B-008") {
				t.Fatalf("talking point leaked B-008")
			}
		}
		for _, tp := range rec.TalkingPoints {
			if containsID(tp, "The Bad Guys") && rec.Items != nil {
				// title may be mentioned only if recommended; it must not be
				t.Fatalf("talking points mentioned Bad Guys for %s: %s", sid, tp)
			}
		}
	}
}

func TestAishaStretchHonesty(t *testing.T) {
	root := repoRoot(t)
	st, err := store.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	aisha := st.StudentByID["S-402"]
	for _, id := range []string{"B-013", "B-016", "B-017"} {
		b := st.BookByID[id]
		if !policy.GradeOK(aisha.Grade, b, false) {
			t.Fatalf("%s is not default-eligible; stretch-unlock claim would be real", id)
		}
	}
	eng := engine.New(st)
	def, err := eng.Recommend(domain.Request{StudentID: "S-402", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	str, err := eng.Recommend(domain.Request{StudentID: "S-402", Stretch: true, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range []domain.Recommendation{def, str} {
		for _, it := range rec.Items {
			if it.BookID == "B-051" {
				t.Fatal("Westing Game must not be recommended on lexile/stretch")
			}
		}
	}
}

func anyHit(ids map[string]domain.RecItem, want []string) bool {
	for _, w := range want {
		if _, ok := ids[w]; ok {
			return true
		}
	}
	return false
}

func keys(m map[string]domain.RecItem) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func containsID(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found")
	return ""
}
