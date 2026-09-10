package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/explain"
	"school_district_reading/internal/store"
)

func TestLLMDoesNotChangeRanking(t *testing.T) {
	st := loadStore(t)
	off := New(st)
	off.LLMOn = false
	off.Explain = explain.TemplateExplainer{}
	offRec, err := off.Recommend(domain.Request{StudentID: "S-406", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	fail := New(st)
	fail.LLMOn = true
	fail.Explain = explain.NewAxon(explain.TemplateExplainer{}, explain.Config{
		BaseURL:    srv.URL,
		Model:      "x",
		Timeout:    time.Second,
		HTTPClient: srv.Client(),
	})
	failRec, err := fail.Recommend(domain.Request{StudentID: "S-406", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	assertSameRank(t, offRec, failRec)
	if failRec.ExplainMode != domain.ExplainFallback {
		t.Fatalf("expected fallback, got %s", failRec.ExplainMode)
	}

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"talking_points":{"B-007":"live A","B-025":"live B","B-004":"live C","B-063":"live D","B-005":"live E","B-001":"live F","B-002":"live G","B-006":"live H"}}`}},
			},
		})
	}))
	t.Cleanup(ok.Close)
	live := New(st)
	live.LLMOn = true
	live.Explain = explain.NewAxon(explain.TemplateExplainer{}, explain.Config{
		BaseURL:    ok.URL,
		Model:      "x",
		Timeout:    time.Second,
		HTTPClient: ok.Client(),
	})
	liveRec, err := live.RecommendContext(context.Background(), domain.Request{StudentID: "S-406", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	assertSameRank(t, offRec, liveRec)
}

func TestUnknownStudent(t *testing.T) {
	eng := New(loadStore(t))
	_, err := eng.Recommend(domain.Request{StudentID: "S-NOPE"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClosedCatalogAndAvailability(t *testing.T) {
	eng := New(loadStore(t))
	rec, err := eng.Recommend(domain.Request{StudentID: "S-406", Limit: 8})
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Items) == 0 {
		t.Fatal("empty recs")
	}
	for _, it := range rec.Items {
		if _, ok := eng.Store.BookByID[it.BookID]; !ok {
			t.Fatalf("invented %s", it.BookID)
		}
		if it.CopiesAvailable <= 0 {
			t.Fatalf("zero copies %s", it.BookID)
		}
		if it.BookID == "B-008" {
			t.Fatal("B-008 spoken")
		}
	}
	foundDrop := false
	for _, d := range rec.Dropped {
		if d.BookID == "B-008" && d.Why == "copies_available=0" {
			foundDrop = true
		}
	}
	if !foundDrop {
		t.Fatalf("B-008 should appear as unavailable exclusion, dropped=%v", rec.Dropped)
	}
}

func TestOliviaFallbackPositiveCount(t *testing.T) {
	eng := New(loadStore(t))
	rec, err := eng.Recommend(domain.Request{StudentID: "S-509", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Items) < 3 {
		t.Fatalf("got %d", len(rec.Items))
	}
	for _, it := range rec.Items {
		ok := false
		for _, r := range it.Reasons {
			if r == "grade-band popularity fallback (no checkout history or query match)" {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("%s reasons %v", it.BookID, it.Reasons)
		}
	}
}

func assertSameRank(t *testing.T, a, b domain.Recommendation) {
	t.Helper()
	if len(a.Items) != len(b.Items) {
		t.Fatalf("len %d vs %d", len(a.Items), len(b.Items))
	}
	for i := range a.Items {
		if a.Items[i].BookID != b.Items[i].BookID {
			t.Fatalf("id %d %s vs %s", i, a.Items[i].BookID, b.Items[i].BookID)
		}
		da := a.Items[i].Score - b.Items[i].Score
		if da < 0 {
			da = -da
		}
		if da > 1e-9 {
			t.Fatalf("score %d %v vs %v", i, a.Items[i].Score, b.Items[i].Score)
		}
	}
}

func TestRankedIDsUnchangedByAcademicFixture(t *testing.T) {
	eng := New(loadStore(t))
	want := map[string][]string{
		"S-406": {"B-007", "B-063", "B-061", "B-025", "B-043"},
		"S-402": {"B-011", "B-017", "B-013", "B-016", "B-054"},
		"S-405": {"B-060", "B-063", "B-061", "B-042", "B-040"},
		"S-509": {"B-001", "B-015", "B-043", "B-007", "B-063"},
		"S-504": {"B-003", "B-005", "B-004", "B-006", "B-021"},
		"S-305": {"B-003", "B-034", "B-001", "B-043", "B-004"},
		"S-301": {"B-005", "B-006", "B-004", "B-063", "B-061"},
		"S-510": {"B-022", "B-050", "B-027", "B-020", "B-043"},
		"S-401": {"B-014", "B-012", "B-013", "B-017", "B-016"},
		"S-302": {"B-006", "B-004", "B-062", "B-060", "B-043"},
	}
	for id, ids := range want {
		rec, err := eng.Recommend(domain.Request{StudentID: id, Limit: 5})
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if len(rec.Items) != len(ids) {
			t.Fatalf("%s len %d want %d", id, len(rec.Items), len(ids))
		}
		for i, bookID := range ids {
			if rec.Items[i].BookID != bookID {
				t.Fatalf("%s[%d]=%s want %s", id, i, rec.Items[i].BookID, bookID)
			}
		}
		raw, _ := json.Marshal(rec)
		s := strings.ToLower(string(raw))
		for _, banned := range []string{"improving_engagement", "scenario_calendar", "w-23-s1", "synthetic:"} {
			if strings.Contains(s, banned) {
				t.Fatalf("%s ranking leaked scenario field %s", id, banned)
			}
		}
	}
}

func loadStore(t *testing.T) *store.Store {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	st, err := store.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}
