package engagement

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/store"
)

// Demo is read-only fictional evidence. It is never installed in session state.
type Demo struct {
	Source     string         `json:"source"`
	Timezone   string         `json:"timezone"`
	Provenance string         `json:"provenance"`
	Contacts   []DemoContact  `json:"contacts"`
	Coverage   []DemoCoverage `json:"coverage"`
}
type DemoContact struct {
	ID            string     `json:"id"`
	StudentID     string     `json:"student_id"`
	StaffID       string     `json:"staff_id"`
	At            time.Time  `json:"at"`
	Status        string     `json:"status"`
	PreviousStart *time.Time `json:"previous_start,omitempty"`
}

// Window IDs explicitly link contact evidence to the separate academic scenario.
// EnglishDate is the fictional posting date, not inferred from a checkout.
type DemoCoverage struct {
	StudentID       string `json:"student_id"`
	WindowID        string `json:"window_id"`
	EnrollmentStart string `json:"enrollment_start"`
	EnrollmentEnd   string `json:"enrollment_end"`
	EnglishDate     string `json:"english_date"`
	Course          string `json:"course"`
}

func LoadDemo(dir string, st *store.Store, acad *academics.Catalog) (*Demo, error) {
	b, err := os.ReadFile(filepath.Join(dir, "engagement_demo.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var d Demo
	if err = json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if err = d.Validate(st, acad); err != nil {
		return nil, err
	}
	return &d, nil
}
func (d *Demo) Validate(st *store.Store, acad *academics.Catalog) error {
	if d.Source != "illustrative_historical" || d.Provenance == "" || d.Timezone != "America/Los_Angeles" {
		return fmt.Errorf("invalid historical provenance or timezone")
	}
	ids := map[string]bool{}
	for _, c := range d.Contacts {
		if c.ID == "" || ids[c.ID] || c.At.IsZero() {
			return fmt.Errorf("invalid historical contact %q", c.ID)
		}
		ids[c.ID] = true
		if _, ok := st.Student(c.StudentID); !ok {
			return fmt.Errorf("unknown historical student %s", c.StudentID)
		}
		if _, ok := st.LibrarianByID[c.StaffID]; !ok {
			return fmt.Errorf("unknown historical staff %s", c.StaffID)
		}
		if c.Status != "completed" && c.Status != "cancelled" && c.Status != "no_show" {
			return fmt.Errorf("invalid historical status")
		}
		if c.PreviousStart != nil && (c.PreviousStart.IsZero() || !c.PreviousStart.Before(c.At)) {
			return fmt.Errorf("invalid reschedule chronology")
		}
	}
	seen := map[string]bool{}
	for _, v := range d.Coverage {
		key := v.StudentID + "/" + v.WindowID
		if seen[key] || v.WindowID == "" || v.Course == "" {
			return fmt.Errorf("invalid historical coverage %s", key)
		}
		seen[key] = true
		if _, ok := st.Student(v.StudentID); !ok {
			return fmt.Errorf("unknown coverage student")
		}
		a, e := academics.ParseDateOnly(v.EnrollmentStart)
		if e != nil {
			return e
		}
		b, e := academics.ParseDateOnly(v.EnrollmentEnd)
		if e != nil || a.After(b) {
			return fmt.Errorf("invalid enrollment bounds")
		}
		if _, e = academics.ParseDateOnly(v.EnglishDate); e != nil {
			return e
		}
		// Optional academics can be absent; no pairs will be admitted without them.
		if acad == nil || acad.Missing {
			continue
		}
		view := acad.View(v.StudentID, st)
		found := false
		if view.Scenario != nil {
			for _, w := range view.Scenario.Windows {
				if w.ID == v.WindowID {
					found = true
					if v.EnrollmentStart > w.Start || v.EnrollmentEnd < w.End || v.EnglishDate < w.End || v.EnglishDate > v.EnrollmentEnd {
						return fmt.Errorf("coverage does not contain window %s", key)
					}
				}
			}
		}
		if !found {
			return fmt.Errorf("unknown scenario window %s", key)
		}
	}
	return nil
}
