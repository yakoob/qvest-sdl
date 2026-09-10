package support

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"school_district_reading/internal/store"
)

type TeacherNote struct {
	ID       string `json:"id"`
	Date     string `json:"date"`
	SourceID string `json:"source_id"`
	Request  string `json:"request"`
	Text     string `json:"text"`
}

type Guidance struct {
	ID         string   `json:"id"`
	SourceID   string   `json:"source_id"`
	SourceRole string   `json:"source_role"`
	ApprovedAt string   `json:"approved_at"`
	ReviewBy   string   `json:"review_by"`
	Summary    string   `json:"summary"`
	Themes     []string `json:"themes"`
}

type Record struct {
	StudentID    string        `json:"student_id"`
	Strengths    []string      `json:"strengths"`
	TeacherNotes []TeacherNote `json:"teacher_notes"`
	Guidance     []Guidance    `json:"guidance"`
}

type File struct {
	Synthetic bool              `json:"synthetic"`
	Note      string            `json:"note"`
	Sources   map[string]string `json:"sources"`
	Students  []Record          `json:"students"`
}

type Catalog struct {
	ByStudent map[string]Record
	Missing   bool
}

// ThemeTerms is an allowlist of neutral local catalog search terms, not student context.
func ThemeTerms(theme string) (string, bool) {
	terms := map[string]string{
		"perseverance": "determination courage survival",
		"belonging":    "friendship belonging community",
		"sports":       "sports basketball soccer baseball",
		"underdogs":    "teamwork courage competition",
		"creativity":   "drawing art comics",
		"animals":      "animals wildlife nature",
	}
	v, ok := terms[theme]
	return v, ok
}

func Load(dir string, st *store.Store) (*Catalog, error) {
	c := &Catalog{ByStudent: map[string]Record{}}
	raw, err := os.ReadFile(filepath.Join(dir, "support_demo.json"))
	if os.IsNotExist(err) {
		c.Missing = true
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	var f File
	if err = json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("support fixture: %w", err)
	}
	if !f.Synthetic {
		return nil, fmt.Errorf("support fixture must be labeled synthetic")
	}
	ids := map[string]bool{}
	validDate := func(s string) bool { _, err := time.Parse("2006-01-02", s); return err == nil }
	for _, r := range f.Students {
		if _, ok := st.Student(r.StudentID); !ok {
			return nil, fmt.Errorf("unknown support student %s", r.StudentID)
		}
		if _, ok := c.ByStudent[r.StudentID]; ok {
			return nil, fmt.Errorf("duplicate support student %s", r.StudentID)
		}
		for _, n := range r.TeacherNotes {
			if n.ID == "" || ids[n.ID] || !validDate(n.Date) || f.Sources[n.SourceID] != "teacher" {
				return nil, fmt.Errorf("invalid teacher note %s", n.ID)
			}
			if n.Request != "none" && n.Request != "routine" && n.Request != "prompt" {
				return nil, fmt.Errorf("invalid teacher request %s", n.ID)
			}
			if len(n.Text) > 2000 {
				return nil, fmt.Errorf("teacher note too long")
			}
			ids[n.ID] = true
		}
		for _, g := range r.Guidance {
			if g.ID == "" || ids[g.ID] || !validDate(g.ApprovedAt) || !validDate(g.ReviewBy) || g.ReviewBy < g.ApprovedAt || g.SourceRole != "counselor" || f.Sources[g.SourceID] != g.SourceRole {
				return nil, fmt.Errorf("invalid shared guidance %s", g.ID)
			}
			if len(g.Summary) > 2000 {
				return nil, fmt.Errorf("guidance too long")
			}
			for _, theme := range g.Themes {
				if _, ok := ThemeTerms(theme); !ok {
					return nil, fmt.Errorf("unknown theme %s", theme)
				}
			}
			ids[g.ID] = true
		}
		c.ByStudent[r.StudentID] = r
	}
	return c, nil
}
