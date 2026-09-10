package support

import (
	"fmt"
	"sync"
	"time"
)

type Followup struct {
	StudentID string `json:"student_id"`
	StaffID   string `json:"staff_id"`
	Action    string `json:"action"`
	RetryID   string `json:"retry_id"`
	At        string `json:"at"`
	Revision  int64  `json:"revision"`
}

type Followups struct {
	mu       sync.Mutex
	items    []Followup
	revision int64
}

// Add retains retry keys for this process lifetime; a full demo log rejects writes.
func (f *Followups) Add(in Followup) (Followup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if in.RetryID == "" || len(in.RetryID) > 128 {
		return Followup{}, fmt.Errorf("retry_id required (max 128 characters)")
	}
	if in.Action != "check_in" && in.Action != "enjoyed" && in.Action != "try_another" {
		return Followup{}, fmt.Errorf("unknown follow-up action")
	}
	for _, old := range f.items {
		if old.RetryID == in.RetryID {
			if old.StudentID != in.StudentID || old.StaffID != in.StaffID || old.Action != in.Action {
				return Followup{}, fmt.Errorf("retry_id reused for a different action")
			}
			return old, nil
		}
	}
	if len(f.items) >= 1000 {
		return Followup{}, fmt.Errorf("demo follow-up log full; restart to reset")
	}
	f.revision++
	in.Revision = f.revision
	in.At = time.Now().UTC().Format(time.RFC3339)
	f.items = append(f.items, in)
	return in, nil
}

func (f *Followups) ForStudent(id string) ([]Followup, int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Followup{}
	for i := len(f.items) - 1; i >= 0; i-- {
		if f.items[i].StudentID == id {
			out = append(out, f.items[i])
		}
	}
	return out, f.revision
}
