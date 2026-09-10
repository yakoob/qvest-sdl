package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/version"
)

// Event is the durable rec record. It must not contain raw query text, names,
// anecdotes, credentials, or complete model request/response bodies.
type Event struct {
	TS          string           `json:"ts"`
	StudentID   string           `json:"student_id"`
	StaffID     string           `json:"staff_id"`
	Stretch     bool             `json:"stretch"`
	LLMEnabled  bool             `json:"llm_enabled"`
	ExplainMode string           `json:"explain_mode"`
	Version     string           `json:"version"`
	BookIDs     []string         `json:"book_ids"`
	Dropped     []DroppedOutcome `json:"dropped,omitempty"`
	QueryFlags  QueryFlags       `json:"query_flags"`
	ModelFields []string         `json:"model_payload_fields"`
}

type DroppedOutcome struct {
	BookID string `json:"book_id"`
	Why    string `json:"why"`
}

type QueryFlags struct {
	Present  bool `json:"present"`
	Under150 bool `json:"under_150"`
	Short    bool `json:"short"`
}

type Log struct {
	Path string
	mu   sync.Mutex
}

func FromEnv() *Log {
	path := os.Getenv("SHELFMATE_AUDIT")
	if path == "" {
		return nil
	}
	return &Log{Path: path}
}

func FromRecommendation(rec domain.Recommendation, now time.Time) Event {
	ids := make([]string, 0, len(rec.Items))
	for _, it := range rec.Items {
		ids = append(ids, it.BookID)
	}
	dropped := make([]DroppedOutcome, 0, len(rec.Dropped))
	for _, d := range rec.Dropped {
		dropped = append(dropped, DroppedOutcome{BookID: d.BookID, Why: d.Why})
	}
	ver := rec.Version
	if ver == "" {
		ver = version.Version
	}
	fields := []string{"student_id", "book_ids", "stretch", "under_150", "short"}
	return Event{
		TS:          now.UTC().Format(time.RFC3339),
		StudentID:   rec.StudentID,
		StaffID:     rec.StaffID,
		Stretch:     rec.Stretch,
		LLMEnabled:  rec.LLMEnabled,
		ExplainMode: rec.ExplainMode,
		Version:     ver,
		BookIDs:     ids,
		Dropped:     dropped,
		QueryFlags: QueryFlags{
			Present:  rec.Query != "",
			Under150: rec.QueryParsed.Under150,
			Short:    rec.QueryParsed.Short,
		},
		ModelFields: fields,
	}
}

func (l *Log) Append(ev Event) error {
	if l == nil || l.Path == "" {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("audit write: %w", err)
	}
	return nil
}

// Append is kept for older call sites that pass a recommendation and a path.
func Append(path string, rec domain.Recommendation) error {
	l := &Log{Path: path}
	return l.Append(FromRecommendation(rec, time.Now().UTC()))
}
