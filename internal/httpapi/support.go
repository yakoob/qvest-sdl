package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/engagement"
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
		StudentID        string    `json:"student_id"`
		Grade            int       `json:"grade"`
		Band             string    `json:"band"`
		Label            string    `json:"label"`
		GradeStatus      string    `json:"grade_status"`
		GradeStatusLabel string    `json:"grade_status_label"`
		English          string    `json:"english,omitempty"`
		Reading          string    `json:"reading,omitempty"`
		Waiting          bool      `json:"waiting"`
		Coverage         string    `json:"coverage"`
		Reasons          []string  `json:"reason_codes"`
		Trend            []float64 `json:"trend"`
	}
	busy := map[string]bool{}
	if s.Session != nil {
		state := s.Session.EngagementSnapshot()
		for _, in := range state.Interactions {
			if in.CompletedAt == nil {
				busy[in.StudentID] = true
			}
		}
		for _, a := range state.Appointments {
			if a.Status == "scheduled" || a.Status == "in_progress" {
				busy[a.StudentID] = true
			}
		}
		checkout := map[string]bool{}
		for _, c := range state.Choices {
			if c.BookID != "" && c.LoanID != "" {
				checkout[c.InteractionID] = true
			}
		}
		for _, f := range state.Followups {
			if f.CompletedAt != nil {
				continue
			}
			if checkout[f.InteractionID] {
				busy[interactionStudent(state.Interactions, f.InteractionID)] = true
			}
		}
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
		var trend []float64
		if s.Academics != nil && !s.Academics.Missing {
			if rec, ok := s.Academics.ByStudent[st.StudentID]; ok {
				sem := append([]academics.SemesterIn(nil), rec.Semesters...)
				sort.Slice(sem, func(i, j int) bool { return sem[i].End < sem[j].End })
				trend = make([]float64, 0, len(sem))
				for i := range sem {
					if p := gradePoint(sem[i].Grade); p != nil {
						trend = append(trend, *p)
					} else {
						trend = append(trend, -1) // missing: rendered as a gap, not zero
					}
				}
			}
		}
		waiting := !busy[st.StudentID] && (result.GradeStatus == support.BelowGrade || result.GradeStatus == support.UnknownGrade)
		rows = append(rows, row{st.StudentID, st.Grade, result.Band, result.Label, result.GradeStatus, result.GradeStatusLabel, result.English, result.Reading, waiting, result.Coverage, codes, trend})
	}
	order := map[string]int{support.BelowGrade: 0, support.UnknownGrade: 1, support.OnGrade: 2, support.AboveGrade: 3}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Waiting != rows[j].Waiting {
			return rows[i].Waiting
		}
		if rows[i].GradeStatus != rows[j].GradeStatus {
			return order[rows[i].GradeStatus] < order[rows[j].GradeStatus]
		}
		if rows[i].Grade != rows[j].Grade {
			return rows[i].Grade < rows[j].Grade
		}
		return rows[i].StudentID < rows[j].StudentID
	})
	writeJSON(w, 200, map[string]any{"students": rows, "config": support.DefaultConfig(), "synthetic": true})
}

func interactionStudent(rows []engagement.Interaction, id string) string {
	for _, in := range rows {
		if in.ID == id {
			return in.StudentID
		}
	}
	return ""
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
	st, _ := snap.Engine.Store.Student(id)
	rec := academics.Record{StudentID: id}
	if s.Academics != nil {
		if v, ok := s.Academics.ByStudent[id]; ok {
			rec = v
			rec.StudentID = id
		}
	}
	_, themes, below := support.ClassifiedInterests(st, rec, record)
	writeJSON(w, 200, map[string]any{"student_id": id, "support": s.supportResult(id), "reading_guidance": record, "classified_themes": themes, "below_grade": below, "followups": follows, "followup_revision": rev, "revision": snap.Revision, "synthetic": true, "access_note": "Fictional localhost demo. Staff selection is not authentication or role-based access control. Classified themes are allowlisted catalog terms, not raw notes."})
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
