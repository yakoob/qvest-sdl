package retrieve

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/store"
)

func TestMateoGraphicNeighbors(t *testing.T) {
	h := loadHybrid(t)
	st, _ := h.Store.Student("S-406")
	ranked := h.Recommend(st, domain.Request{})
	ids := topIDs(ranked, 12)
	if !contains(ids, "B-007") && !contains(ids, "B-008") {
		t.Fatalf("Mateo neighbors should include Cat Kid or Bad Guys, got %v", ids[:min(5, len(ids))])
	}
}

func TestSparseHistoryDoesNotClaimCF(t *testing.T) {
	h := loadHybrid(t)
	st, _ := h.Store.Student("S-405")
	ranked := h.Recommend(st, domain.Request{})
	for _, row := range ranked[:min(8, len(ranked))] {
		for _, r := range row.Reasons {
			if r == reasonCF {
				t.Fatalf("%s claimed CF with one checkout: %v", row.Book.BookID, row.Reasons)
			}
		}
	}
}

func TestQueryEvidenceSeparatedFromHistory(t *testing.T) {
	h := loadHybrid(t)
	st, _ := h.Store.Student("S-405")
	ranked := h.Recommend(st, domain.Request{Query: "funny under 150 pages"})
	sawQuery := false
	for _, row := range ranked[:min(10, len(ranked))] {
		hasQ, hasCF := false, false
		for _, r := range row.Reasons {
			if r == reasonQuery {
				hasQ = true
				sawQuery = true
			}
			if r == reasonCF {
				hasCF = true
			}
		}
		if hasCF {
			t.Fatalf("query path claimed CF for %s: %v", row.Book.BookID, row.Reasons)
		}
		if hasQ && row.Content == 0 && row.Score == 0 {
			t.Fatalf("query reason without content score on %s", row.Book.BookID)
		}
	}
	if !sawQuery {
		t.Fatal("expected at least one query-match reason")
	}
}

func TestSeriesReasonIsSameSeriesNotNext(t *testing.T) {
	h := loadHybrid(t)
	st, _ := h.Store.Student("S-402")
	ranked := h.Recommend(st, domain.Request{})
	for _, row := range ranked {
		for _, r := range row.Reasons {
			if strings.Contains(strings.ToLower(r), "next") {
				t.Fatalf("must not claim next-in-series without sequence metadata: %s %q", row.Book.BookID, r)
			}
			if r == reasonSeries && row.Book.Series == "" {
				t.Fatalf("series reason on book with empty series %s", row.Book.BookID)
			}
		}
	}
}

func TestZeroHistoryFallbackLabeled(t *testing.T) {
	h := loadHybrid(t)
	st, _ := h.Store.Student("S-509")
	if len(h.Store.History[st.StudentID]) != 0 {
		t.Fatal("Olivia fixture drift")
	}
	ranked := h.Recommend(st, domain.Request{})
	if len(ranked) < 3 {
		t.Fatalf("fallback returned %d", len(ranked))
	}
	prevBorrowers := int(^uint(0) >> 1)
	prevID := ""
	for i, row := range ranked {
		if len(row.Reasons) != 1 || row.Reasons[0] != reasonFallback {
			t.Fatalf("row %d %s reasons %v want only fallback", i, row.Book.BookID, row.Reasons)
		}
		if row.CF != 0 || row.Content != 0 {
			t.Fatalf("fallback must not report CF/content evidence for %s", row.Book.BookID)
		}
		b := h.uniqueBorrowers(row.Book.BookID)
		if i > 0 {
			if b > prevBorrowers {
				t.Fatalf("not sorted by unique borrowers")
			}
			if b == prevBorrowers && row.Book.BookID < prevID {
				t.Fatalf("tie-break should be book_id ascending, %s then %s", prevID, row.Book.BookID)
			}
		}
		prevBorrowers = b
		prevID = row.Book.BookID
	}
}

func TestFallbackGivesWayToQuery(t *testing.T) {
	h := loadHybrid(t)
	st, _ := h.Store.Student("S-509")
	ranked := h.Recommend(st, domain.Request{Query: "sharks"})
	for _, row := range ranked[:min(5, len(ranked))] {
		for _, r := range row.Reasons {
			if r == reasonFallback {
				t.Fatalf("query match should not use fallback label on %s", row.Book.BookID)
			}
		}
	}
}

func loadHybrid(t *testing.T) *Hybrid {
	t.Helper()
	s, err := store.Load(filepath.Join(repoRoot(t), "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	return New(s)
}

func topIDs(rows []domain.ScoredBook, n int) []string {
	if n > len(rows) {
		n = len(rows)
	}
	out := make([]string, 0, n)
	for _, r := range rows[:n] {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
