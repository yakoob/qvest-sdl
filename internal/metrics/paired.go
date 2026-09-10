package metrics

import (
	"fmt"
	"sort"
	"time"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/engagement"
	"school_district_reading/internal/store"
)

type PairedEvidence struct {
	BeforeID    string `json:"before_id,omitempty"`
	AfterID     string `json:"after_id,omitempty"`
	BeforeDate  string `json:"before_date,omitempty"`
	AfterDate   string `json:"after_date,omitempty"`
	Before      string `json:"before,omitempty"`
	After       string `json:"after,omitempty"`
	BeforeStart string `json:"before_start,omitempty"`
	AfterStart  string `json:"after_start,omitempty"`
	Direction   string `json:"direction"`
	Exclusion   string `json:"exclusion,omitempty"`
}
type PairedStudent struct {
	StudentID     string                   `json:"student_id"`
	ContactID     string                   `json:"contact_id"`
	ContactAt     time.Time                `json:"contact_at"`
	Facilitator   string                   `json:"facilitator"`
	LaterContacts []engagement.DemoContact `json:"later_contacts"`
	Borrowing     PairedEvidence           `json:"borrowing"`
	English       PairedEvidence           `json:"english"`
	Reading       PairedEvidence           `json:"reading"`
}
type DirectionCounts struct {
	Eligible  int            `json:"eligible"`
	Increased int            `json:"increased"`
	Unchanged int            `json:"unchanged"`
	Decreased int            `json:"decreased"`
	Excluded  int            `json:"excluded"`
	Reasons   map[string]int `json:"reasons"`
}
type PairedReport struct {
	Source    string          `json:"source"`
	Filter    Filter          `json:"filter"`
	Note      string          `json:"note"`
	Available bool            `json:"available"`
	Students  int             `json:"students"`
	Borrowing DirectionCounts `json:"borrowing"`
	English   DirectionCounts `json:"english"`
	Reading   DirectionCounts `json:"reading"`
	Rows      []PairedStudent `json:"rows"`
}

// Paired indexes contacts before filtering staff, so shared students cannot change
// ownership with a dropdown. Historical evidence never comes from the live Store.
func Paired(st *store.Store, acad *academics.Catalog, demo *engagement.Demo, live engagement.State, source string, f Filter) (PairedReport, error) {
	r := PairedReport{Source: source, Filter: f, Rows: []PairedStudent{}, Available: true}
	r.Note = "Descriptive paired observations, not causal librarian effectiveness. First completed contact per student in the period determines attribution; later contacts remain visible. Borrowing compares observed events in fully covered 84-day windows (renewals/repeats included). English uses ordinal direction only; reading compares the same instrument, scale and grade form."
	if source != "session" && source != "historical" {
		return r, fmt.Errorf("source must be session or historical")
	}
	if !f.Start.Before(f.End) || f.AsOf.IsZero() {
		return r, fmt.Errorf("invalid reporting range or as-of")
	}
	if f.StaffID != "" {
		if _, ok := st.LibrarianByID[f.StaffID]; !ok {
			return r, fmt.Errorf("unknown staff")
		}
	}
	contacts := []engagement.DemoContact{}
	if source == "historical" {
		if demo == nil {
			r.Available = false
			r.Note = "Historical fixture unavailable. Session workflow remains available."
		} else {
			if err := demo.Validate(st, acad); err != nil {
				return r, err
			}
			contacts = append(contacts, demo.Contacts...)
		}
	} else {
		for _, in := range live.Interactions {
			if in.CompletedAt != nil {
				contacts = append(contacts, engagement.DemoContact{ID: in.ID, StudentID: in.StudentID, StaffID: in.Facilitator, At: *in.CompletedAt, Status: "completed"})
			}
		}
	}
	sort.Slice(contacts, func(i, j int) bool {
		if contacts[i].At.Equal(contacts[j].At) {
			return contacts[i].ID < contacts[j].ID
		}
		return contacts[i].At.Before(contacts[j].At)
	})
	rows := map[string]*PairedStudent{}
	for _, c := range contacts {
		if c.Status != "completed" || c.At.Before(f.Start) || !c.At.Before(f.End) || c.At.After(f.AsOf) {
			continue
		}
		if row := rows[c.StudentID]; row != nil {
			row.LaterContacts = append(row.LaterContacts, c)
			continue
		}
		rows[c.StudentID] = &PairedStudent{StudentID: c.StudentID, ContactID: c.ID, ContactAt: c.At, Facilitator: c.StaffID, LaterContacts: []engagement.DemoContact{}}
	}
	for _, row := range rows {
		if f.StaffID != "" && row.Facilitator != f.StaffID {
			continue
		}
		reason := ""
		if source == "session" {
			reason = "No declared full enrollment/export coverage for session pairing; historical scenarios are not session evidence"
		} else if acad == nil || acad.Missing {
			reason = "Optional academic scenario unavailable"
		}
		if reason != "" {
			row.Borrowing = excluded(reason)
			row.English = excluded(reason)
			row.Reading = excluded(reason)
		} else {
			pairHistorical(row, st, acad, demo, f.AsOf)
		}
		r.Rows = append(r.Rows, *row)
	}
	sort.Slice(r.Rows, func(i, j int) bool { return r.Rows[i].StudentID < r.Rows[j].StudentID })
	r.Students = len(r.Rows)
	for _, row := range r.Rows {
		countDirection(&r.Borrowing, row.Borrowing)
		countDirection(&r.English, row.English)
		countDirection(&r.Reading, row.Reading)
	}
	return r, nil
}
func excluded(reason string) PairedEvidence {
	return PairedEvidence{Direction: "excluded", Exclusion: reason}
}
func countDirection(c *DirectionCounts, p PairedEvidence) {
	if c.Reasons == nil {
		c.Reasons = map[string]int{}
	}
	switch p.Direction {
	case "increased":
		c.Eligible++
		c.Increased++
	case "unchanged":
		c.Eligible++
		c.Unchanged++
	case "decreased":
		c.Eligible++
		c.Decreased++
	default:
		c.Excluded++
		c.Reasons[p.Exclusion]++
	}
}
func direction(before, after int) string {
	if after > before {
		return "increased"
	}
	if after < before {
		return "decreased"
	}
	return "unchanged"
}

type pairedObservation struct {
	id, date, start, key, value string
	numeric                     int
}

func choosePair(observations []pairedObservation, contact, asof string, days int) PairedEvidence {
	t, _ := academics.ParseDateOnly(contact)
	lower, upper := t.AddDate(0, 0, -days).Format("2006-01-02"), t.AddDate(0, 0, days).Format("2006-01-02")
	sort.Slice(observations, func(i, j int) bool {
		if observations[i].date == observations[j].date {
			return observations[i].id < observations[j].id
		}
		return observations[i].date < observations[j].date
	})
	var before *pairedObservation
	for i := range observations {
		o := &observations[i]
		if o.date < contact && o.date >= lower && o.date <= asof && (o.start == "" || o.start >= lower) {
			before = o
		}
	}
	if before == nil {
		return excluded("No valid pre-contact observation within the comparison window")
	}
	for _, after := range observations {
		if after.date <= contact || after.date > upper || after.date > asof || after.key != before.key {
			continue
		}
		return PairedEvidence{BeforeID: before.id, AfterID: after.id, BeforeDate: before.date, AfterDate: after.date, BeforeStart: before.start, AfterStart: after.start, Before: before.value, After: after.value, Direction: direction(before.numeric, after.numeric)}
	}
	return excluded("No compatible post-contact observation available within the comparison window")
}
func pairHistorical(row *PairedStudent, st *store.Store, acad *academics.Catalog, demo *engagement.Demo, asof time.Time) {
	loc, _ := time.LoadLocation(demo.Timezone)
	contact := row.ContactAt.In(loc).Format("2006-01-02")
	// Date-only evidence has no within-day availability time: include completed days only.
	knownThrough := asof.In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	coverage := map[string]engagement.DemoCoverage{}
	for _, v := range demo.Coverage {
		if v.StudentID == row.StudentID {
			coverage[v.WindowID] = v
		}
	}
	view := acad.View(row.StudentID, st)
	row.Borrowing = excluded("No fully covered 84-day windows before and after contact")
	row.English = excluded("No comparable English observations")
	row.Reading = excluded("No comparable reading observations")
	if view.Scenario == nil {
		return
	}
	var borrow, english, reading []pairedObservation
	for _, w := range view.Scenario.Windows {
		c, ok := coverage[w.ID]
		if !ok || c.EnrollmentStart > w.Start || c.EnrollmentEnd < w.End {
			continue
		}
		if w.End <= knownThrough && w.InclusiveDays == 84 && w.BorrowingKnown && (w.End < contact || w.Start > contact) {
			borrow = append(borrow, pairedObservation{id: w.ID, date: w.End, start: w.Start, key: "84-day-events", value: fmt.Sprint(w.CheckoutCount), numeric: w.CheckoutCount})
		}
		if w.EnglishGrade != nil && c.EnglishDate <= knownThrough {
			if rank, ok := gradeRank(*w.EnglishGrade); ok {
				english = append(english, pairedObservation{id: w.ID + "/english", date: c.EnglishDate, key: fmt.Sprintf("%s/%d/%s", c.Course, w.SchoolGrade, academics.ScaleLetter), value: *w.EnglishGrade, numeric: rank})
			}
		}
		if a := w.ReadingCheck; a != nil && a.Result != nil && a.Date <= knownThrough {
			reading = append(reading, pairedObservation{id: w.ID + "/reading", date: a.Date, key: fmt.Sprintf("Willow/%s/%d", academics.ScaleWillow, a.GradeForm), value: fmt.Sprintf("%d (grade form %d)", *a.Result, a.GradeForm), numeric: *a.Result})
		}
	}
	row.Borrowing = choosePair(borrow, contact, knownThrough, 365)
	row.English = choosePair(english, contact, knownThrough, 180)
	row.Reading = choosePair(reading, contact, knownThrough, 180)
}
func gradeRank(grade string) (int, bool) {
	for i, g := range []string{"F", "D-", "D", "D+", "C-", "C", "C+", "B-", "B", "B+", "A-", "A", "A+"} {
		if grade == g {
			return i, true
		}
	}
	return 0, false
}
