package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/support"
)

func (s Server) supportResult(id string) support.Result {
	rec := academics.Record{StudentID: id}
	if s.Academics != nil {
		rec = s.Academics.ByStudent[id]
		rec.StudentID = id
	}
	guidance := support.Record{}
	if s.Support != nil {
		guidance = s.Support.ByStudent[id]
	}
	return support.Evaluate(rec, guidance, support.DefaultConfig())
}

func (s Server) supportQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	type row struct {
		StudentID string   `json:"student_id"`
		Band      string   `json:"band"`
		Label     string   `json:"label"`
		Coverage  string   `json:"coverage"`
		Reasons   []string `json:"reason_codes"`
	}
	rows := []row{}
	for _, st := range s.snap().Engine.Store.Students {
		result := s.supportResult(st.StudentID)
		codes := []string{}
		for _, e := range result.Evidence {
			if e.Rule != "" {
				codes = append(codes, e.Rule)
			}
		}
		rows = append(rows, row{st.StudentID, result.Band, result.Label, result.Coverage, codes})
	}
	order := map[string]int{support.First: 0, support.Soon: 1, support.Insufficient: 2, support.None: 3}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Band != rows[j].Band {
			return order[rows[i].Band] < order[rows[j].Band]
		}
		return rows[i].StudentID < rows[j].StudentID
	})
	writeJSON(w, 200, map[string]any{"students": rows, "config": support.DefaultConfig(), "synthetic": true})
}

func (s Server) supportDetail(w http.ResponseWriter, r *http.Request, id string) {
	snap := s.snap()
	if _, ok := snap.Engine.Store.Student(id); !ok {
		writeJSON(w, 404, map[string]string{"error": "unknown student"})
		return
	}
	record := support.Record{StudentID: id, Strengths: []string{}, TeacherNotes: []support.TeacherNote{}, Guidance: []support.Guidance{}}
	if s.Support != nil {
		if v, ok := s.Support.ByStudent[id]; ok {
			record = v
		}
	}
	follows, rev := s.Followups.ForStudent(id)
	writeJSON(w, 200, map[string]any{"student_id": id, "support": s.supportResult(id), "reading_guidance": record, "followups": follows, "followup_revision": rev, "revision": snap.Revision, "synthetic": true, "access_note": "Fictional localhost demo. Staff selection is not authentication or role-based access control."})
}

func (s Server) followup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	raw, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var body struct {
		StudentID string `json:"student_id"`
		StaffID   string `json:"staff_id"`
		Action    string `json:"action"`
		RetryID   string `json:"retry_id"`
	}
	if json.Unmarshal(raw, &body) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	st := s.snap().Engine.Store
	if _, ok := st.Student(body.StudentID); !ok {
		writeJSON(w, 404, map[string]string{"error": "unknown student"})
		return
	}
	if _, ok := st.LibrarianByID[body.StaffID]; !ok {
		writeJSON(w, 404, map[string]string{"error": "unknown staff"})
		return
	}
	out, err := s.Followups.Add(support.Followup{StudentID: body.StudentID, StaffID: body.StaffID, Action: body.Action, RetryID: body.RetryID})
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"followup": out, "note": "Recorded in memory. Academic evidence and support band unchanged; restart clears follow-ups."})
}
