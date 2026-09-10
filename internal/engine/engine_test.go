package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
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
