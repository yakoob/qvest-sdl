package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"school_district_reading/internal/domain"
)

type Event struct {
	TS          string   `json:"ts"`
	StudentID   string   `json:"student_id"`
	StaffID     string   `json:"staff_id"`
	Query       string   `json:"query,omitempty"`
	Stretch     bool     `json:"stretch"`
	LLM         bool     `json:"llm"`
	BookIDs     []string `json:"book_ids"`
	Dropped     []string `json:"dropped,omitempty"`
	ModelFields []string `json:"model_payload_fields"`
}

func Append(path string, rec domain.Recommendation) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	ids := make([]string, 0, len(rec.Items))
	for _, it := range rec.Items {
		ids = append(ids, it.BookID)
	}
	dropped := make([]string, 0, len(rec.Dropped))
	for _, d := range rec.Dropped {
		dropped = append(dropped, d.BookID+":"+d.Why)
	}
	ev := Event{
		TS:          time.Now().UTC().Format(time.RFC3339),
		StudentID:   rec.StudentID,
		StaffID:     rec.StaffID,
		Query:       rec.Query,
		Stretch:     rec.Stretch,
		LLM:         rec.LLM,
		BookIDs:     ids,
		Dropped:     dropped,
		ModelFields: []string{"student_id", "book_ids"},
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}
