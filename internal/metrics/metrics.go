package metrics

import (
	"school_district_reading/internal/engagement"
	"sort"
	"time"
)

type Filter struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	AsOf    time.Time `json:"as_of"`
	StaffID string    `json:"staff_id"`
}
type Ratio struct {
	Numerator   int `json:"numerator"`
	Denominator int `json:"denominator"`
}
type Pair struct {
	InteractionID string `json:"interaction_id"`
	StudentID     string `json:"student_id"`
	Facilitator   string `json:"facilitator"`
	BookID        string `json:"book_id"`
	LoanID        string `json:"loan_id,omitempty"`
	Source        string `json:"source"`
	Reading       string `json:"reading"`
	Enjoyment     string `json:"enjoyment"`
	Reported      bool   `json:"reported"`
}
type Report struct {
	Source            string                   `json:"source"`
	Filter            Filter                   `json:"filter"`
	StudentsServed    int                      `json:"students_served"`
	Completed         int                      `json:"completed"`
	StudentsChoosing  int                      `json:"students_choosing"`
	NoneToday         int                      `json:"none_today"`
	LinkedCheckouts   int                      `json:"linked_checkouts"`
	Acceptance        Ratio                    `json:"acceptance"`
	Conversion        Ratio                    `json:"conversion"`
	PendingChoices    int                      `json:"pending_choices"`
	ReadingCompletion Ratio                    `json:"reading_completion"`
	Enjoyment         Ratio                    `json:"enjoyment"`
	ResponseCoverage  Ratio                    `json:"response_coverage"`
	Neutral           int                      `json:"neutral"`
	UnknownReading    int                      `json:"unknown_reading"`
	UnknownEnjoyment  int                      `json:"unknown_enjoyment"`
	StaffObservations int                      `json:"staff_observations"`
	FollowupCoverage  Ratio                    `json:"followup_coverage"`
	Overdue           int                      `json:"overdue"`
	PendingFollowups  int                      `json:"pending_followups"`
	Interactions      []engagement.Interaction `json:"interactions"`
	Choices           []engagement.Choice      `json:"choices"`
	Pairs             []Pair                   `json:"pairs"`
	Followups         []engagement.Followup    `json:"followups"`
}

// Aggregate never joins circulation heuristically or treats legacy check-ins as book feedback.
// Completion cohort uses completed_at; follow-up coverage independently uses due_at in range.
func Aggregate(s engagement.State, f Filter) Report {
	r := Report{Source: "This server session", Filter: f, Interactions: []engagement.Interaction{}, Choices: []engagement.Choice{}, Pairs: []Pair{}, Followups: []engagement.Followup{}}
	inRange := func(t time.Time) bool { return !t.Before(f.Start) && t.Before(f.End) && !t.After(f.AsOf) }
	observed := func(t time.Time) bool { return !t.After(f.AsOf) }
	cohort := map[string]engagement.Interaction{}
	all := map[string]engagement.Interaction{}
	students := map[string]bool{}
	choosing := map[string]bool{}
	for _, in := range s.Interactions {
		all[in.ID] = in
		if in.CompletedAt != nil && inRange(*in.CompletedAt) && (f.StaffID == "" || in.Facilitator == f.StaffID) {
			cohort[in.ID] = in
			students[in.StudentID] = true
			r.Interactions = append(r.Interactions, in)
		}
	}
	r.StudentsServed = len(students)
	r.Completed = len(cohort)
	offered := map[string]bool{}
	accepted := map[string]bool{}
	for _, o := range s.Offers {
		if _, ok := cohort[o.InteractionID]; ok && observed(o.At) {
			offered[o.InteractionID] = true
		}
	}
	pairs := map[string]Pair{}
	for _, c := range s.Choices {
		in, ok := cohort[c.InteractionID]
		if !ok || !observed(c.At) {
			continue
		}
		visible := c
		if c.CheckoutAt != nil && !observed(*c.CheckoutAt) {
			visible.LoanID = ""
			visible.CheckoutAt = nil
			visible.CirculationStaff = ""
		}
		r.Choices = append(r.Choices, visible)
		if c.BookID == "" {
			r.NoneToday++
			continue
		}
		choosing[in.StudentID] = true
		accepted[in.ID] = true
		linked := c.LoanID != "" && c.CheckoutAt != nil && observed(*c.CheckoutAt)
		if linked {
			r.LinkedCheckouts++
		}
		mature := c.At.Add(14 * 24 * time.Hour)
		if mature.After(f.AsOf) {
			r.PendingChoices++
		} else {
			r.Conversion.Denominator++
			if linked && !c.CheckoutAt.Before(c.At) && !c.CheckoutAt.After(mature) {
				r.Conversion.Numerator++
			}
		}
		pairs[c.InteractionID+"/"+c.BookID] = Pair{InteractionID: in.ID, StudentID: in.StudentID, Facilitator: in.Facilitator, BookID: c.BookID, LoanID: visible.LoanID, Source: "no report", Reading: "unknown", Enjoyment: "unknown"}
	}
	r.StudentsChoosing = len(choosing)
	for id := range offered {
		r.Acceptance.Denominator++
		if accepted[id] {
			r.Acceptance.Numerator++
		}
	}
	// Student reports and staff observations are separate series. Later staff observations
	// do not erase a student's report. Within each source, latest per pair wins.
	latest := map[string]engagement.Feedback{}
	staff := map[string]bool{}
	for _, fb := range s.Feedback {
		key := fb.InteractionID + "/" + fb.BookID
		if _, ok := pairs[key]; !ok || !observed(fb.At) {
			continue
		}
		if fb.Source == "staff_observed" {
			staff[key] = true
			continue
		}
		if fb.Source != "student_reported" {
			continue
		}
		old, ok := latest[key]
		if !ok || !fb.At.Before(old.At) {
			latest[key] = fb
		}
	}
	r.StaffObservations = len(staff)
	for key, p := range pairs {
		r.ResponseCoverage.Denominator++
		if fb, ok := latest[key]; ok {
			p.Source = fb.Source
			p.Reading = fb.Reading
			p.Enjoyment = fb.Enjoyment
			p.Reported = true
			r.ResponseCoverage.Numerator++
		}
		if p.Reading != "unknown" {
			r.ReadingCompletion.Denominator++
			if p.Reading == "finished" {
				r.ReadingCompletion.Numerator++
			}
		} else {
			r.UnknownReading++
		}
		switch p.Enjoyment {
		case "yes":
			r.Enjoyment.Numerator++
			r.Enjoyment.Denominator++
		case "no":
			r.Enjoyment.Denominator++
		case "neutral":
			r.Neutral++
		default:
			r.UnknownEnjoyment++
		}
		r.Pairs = append(r.Pairs, p)
	}
	sort.Slice(r.Pairs, func(i, j int) bool {
		return r.Pairs[i].InteractionID+r.Pairs[i].BookID < r.Pairs[j].InteractionID+r.Pairs[j].BookID
	})
	for _, fu := range s.Followups {
		// Reconstruct scheduling evidence known at as-of. Once an obligation is
		// overdue, later rebooking must not hide it from its original due cohort.
		visible := fu
		visible.Reservations = nil
		if len(fu.Reservations) > 0 {
			visible.AppointmentID = ""
			visible.Due = fu.Reservations[0].Due
			for _, change := range fu.Reservations {
				if !observed(change.At) {
					continue
				}
				if change.At.Before(visible.Due) {
					visible.Due = change.Due
				}
				visible.AppointmentID = change.AppointmentID
				visible.Reservations = append(visible.Reservations, change)
			}
		}
		in, ok := all[fu.InteractionID]
		if !ok || !observed(fu.CreatedAt) || (f.StaffID != "" && in.Facilitator != f.StaffID) || visible.Due.Before(f.Start) || !visible.Due.Before(f.End) {
			continue
		}
		if fu.CompletedAt != nil && !observed(*fu.CompletedAt) {
			visible.CompletedAt = nil
			visible.StaffID = ""
			visible.ContactID = ""
		}
		r.Followups = append(r.Followups, visible)
		if visible.Due.After(f.AsOf) {
			r.PendingFollowups++
			continue
		}
		r.FollowupCoverage.Denominator++
		if visible.CompletedAt != nil {
			r.FollowupCoverage.Numerator++
		} else {
			r.Overdue++
		}
	}
	return r
}
