package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"school_district_reading/internal/domain"
)

func sampleRec() domain.Recommendation {
	return domain.Recommendation{
		StudentID: "S-406",
		StaffID:   "L-001",
		Query:     "funny, reluctant 4th, under 150 pages",
		QueryParsed: domain.QueryInterpretation{
			Under150: true,
			Short:    false,
			Raw:      "funny, reluctant 4th, under 150 pages",
		},
		Stretch:     false,
		LLMEnabled:  false,
		ExplainMode: domain.ExplainTemplate,
		Version:     "test",
		Items: []domain.RecItem{
			{BookID: "B-007", Title: "Cat Kid Comic Club", TalkingPoint: "say this"},
		},
		Dropped: []domain.Dropped{
			{BookID: "B-008", Title: "The Bad Guys", Why: "copies_available=0"},
		},
	}
}

func TestJSONLOmitsSensitiveFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	l := &Log{Path: path}
	if err := l.Append(FromRecommendation(sampleRec(), time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, banned := range []string{"Mateo", "funny, reluctant", "say this", "Cat Kid", "Bearer", "anecdote"} {
		if strings.Contains(s, banned) {
			t.Fatalf("audit leaked %q: %s", banned, s)
		}
	}
	var ev Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.StudentID != "S-406" || ev.BookIDs[0] != "B-007" {
		t.Fatalf("%+v", ev)
	}
	if !ev.QueryFlags.Under150 || ev.QueryFlags.Present != true {
		t.Fatalf("flags %+v", ev.QueryFlags)
	}
	if ev.Dropped[0].BookID != "B-008" {
		t.Fatal("dropped ids")
	}
}

func TestConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	l := &Log{Path: path}
	var wg sync.WaitGroup
	n := 20
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if err := l.Append(FromRecommendation(sampleRec(), time.Now().UTC())); err != nil {
				t.Errorf("%v", err)
			}
		}()
	}
	wg.Wait()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	lines := 0
	for sc.Scan() {
		lines++
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("line %d: %v", lines, err)
		}
	}
	if lines != n {
		t.Fatalf("lines %d want %d", lines, n)
	}
}

func TestAppendFailure(t *testing.T) {
	l := &Log{Path: filepath.Join(t.TempDir(), "missing-dir-as-file")}
	// create a file where the directory should be
	parent := l.Path
	if err := os.WriteFile(parent, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	l.Path = filepath.Join(parent, "audit.jsonl")
	if err := l.Append(FromRecommendation(sampleRec(), time.Now().UTC())); err == nil {
		t.Fatal("expected failure")
	}
}
