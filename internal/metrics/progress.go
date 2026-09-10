package metrics

import (
	"fmt"
	"sort"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/store"
)

// Progress rows reuse the student View; they are portfolio evidence, not contact outcomes.
type ProgressRow struct {
	StudentID    string               `json:"student_id"`
	Period       string               `json:"period"`
	Start        string               `json:"start"`
	End          string               `json:"end"`
	Checkouts    int                  `json:"checkouts"`
	UniqueTitles int                  `json:"unique_titles"`
	Coverage     string               `json:"coverage"`
	English      *string              `json:"english"`
	Borrowed     []academics.Borrowed `json:"borrowed"`
}
type ProgressPeriod struct {
	ID             string         `json:"id"`
	Start          string         `json:"start"`
	End            string         `json:"end"`
	Students       int            `json:"students"`
	Checkouts      int            `json:"checkouts"`
	StudentTitles  int            `json:"student_titles"`
	DistinctTitles int            `json:"distinct_titles"`
	Covered        int            `json:"covered"`
	Partial        int            `json:"partial"`
	Missing        int            `json:"missing"`
	Grades         map[string]int `json:"grades"`
	MissingGrades  int            `json:"missing_grades"`
}
type ReadingObservation struct {
	StudentID  string `json:"student_id"`
	Date       string `json:"date"`
	Instrument string `json:"instrument"`
	Scale      string `json:"scale"`
	Form       int    `json:"form"`
	Result     *int   `json:"result"`
	PriorDate  string `json:"prior_date,omitempty"`
	Prior      *int   `json:"prior,omitempty"`
	Delta      *int   `json:"delta,omitempty"`
}
type ProgressReport struct {
	Source                 string               `json:"source"`
	Note                   string               `json:"note"`
	Rows                   []ProgressRow        `json:"rows"`
	Periods                []ProgressPeriod     `json:"periods"`
	Reading                []ReadingObservation `json:"reading"`
	StudentsWithoutRecords []string             `json:"students_without_records"`
}

func Progress(st *store.Store, acad *academics.Catalog, source string) (ProgressReport, error) {
	r := ProgressReport{Source: source, Rows: []ProgressRow{}, Periods: []ProgressPeriod{}, Reading: []ReadingObservation{}, StudentsWithoutRecords: []string{}}
	if source != "extract" && source != "scenario" {
		return r, fmt.Errorf("source must be extract or scenario")
	}
	r.Note = "All-student portfolio, not attributed to a librarian. Extract counts include observed events only; partial coverage is not a full observation window. Grades are distributions, not average scores. Reading deltas only compare the same instrument, scale and grade form."
	if source == "scenario" {
		r.Note = "Illustrative historical portfolio: isolated 84-day windows, not outcomes caused by librarian contact. Missing windows are unknown. Student-title pairs and district distinct titles are different units. Grade distributions across years do not establish comparable course growth."
	}
	for _, student := range st.Students {
		view := acad.View(student.StudentID, st)
		before := len(r.Rows)
		if source == "scenario" {
			if view.Scenario != nil {
				for _, w := range view.Scenario.Windows {
					r.Rows = append(r.Rows, ProgressRow{StudentID: student.StudentID, Period: w.ID, Start: w.Start, End: w.End, Checkouts: w.CheckoutCount, UniqueTitles: w.UniqueTitles, Coverage: "full", English: w.EnglishGrade, Borrowed: w.Borrowed})
					if w.ReadingCheck != nil {
						a := w.ReadingCheck
						r.Reading = append(r.Reading, ReadingObservation{StudentID: student.StudentID, Date: a.Date, Instrument: "Willow Bend Reading Check", Scale: academics.ScaleWillow, Form: a.GradeForm, Result: a.Result})
					}
				}
			}
		} else {
			for _, w := range view.Semesters {
				coverage := "missing"
				// The frozen export is contiguous only within these bounds. A session loan
				// after the export does not establish coverage of the intervening dates.
				if w.Start <= "2026-09-03" && w.End >= "2026-01-13" {
					coverage = "partial"
				}
				if w.Start >= "2026-01-13" && w.End <= "2026-09-03" {
					coverage = "full"
				}
				r.Rows = append(r.Rows, ProgressRow{StudentID: student.StudentID, Period: w.Start + " / " + w.End, Start: w.Start, End: w.End, Checkouts: w.CheckoutCount, UniqueTitles: w.UniqueTitles, Coverage: coverage, English: w.EnglishGrade, Borrowed: w.Borrowed})
			}
			for _, a := range view.Assessments {
				r.Reading = append(r.Reading, ReadingObservation{StudentID: student.StudentID, Date: a.Date, Instrument: a.Name, Scale: a.Scale, Form: a.Grade, Result: a.Result})
			}
		}
		if len(r.Rows) == before {
			r.StudentsWithoutRecords = append(r.StudentsWithoutRecords, student.StudentID)
		}
	}
	periods := map[string]*ProgressPeriod{}
	titles := map[string]map[string]bool{}
	// Scenario calendar defines missing rows too, so every student is accounted for
	// even when their academic fixture or one historical window is absent.
	if source == "scenario" && acad != nil && acad.Calendar != nil {
		for _, w := range acad.Calendar.MatchedWindows {
			periods[w.ID] = &ProgressPeriod{ID: w.ID, Start: w.Start, End: w.End, Grades: map[string]int{}}
			titles[w.ID] = map[string]bool{}
		}
	}
	for _, row := range r.Rows {
		p := periods[row.Period]
		if p == nil {
			p = &ProgressPeriod{ID: row.Period, Start: row.Start, End: row.End, Grades: map[string]int{}}
			periods[row.Period] = p
			titles[row.Period] = map[string]bool{}
		}
		p.Students++
		p.Checkouts += row.Checkouts
		p.StudentTitles += row.UniqueTitles
		switch row.Coverage {
		case "full":
			p.Covered++
		case "partial":
			p.Partial++
		default:
			p.Missing++
		}
		if row.English != nil {
			p.Grades[*row.English]++
		} else {
			p.MissingGrades++
		}
		for _, b := range row.Borrowed {
			titles[row.Period][b.BookID] = true
		}
	}
	for _, p := range periods {
		present := map[string]bool{}
		for _, row := range r.Rows {
			if row.Period == p.ID {
				present[row.StudentID] = true
			}
		}
		for _, student := range st.Students {
			if !present[student.StudentID] {
				r.Rows = append(r.Rows, ProgressRow{StudentID: student.StudentID, Period: p.ID, Start: p.Start, End: p.End, Coverage: "missing", Borrowed: []academics.Borrowed{}})
				p.Missing++
				p.MissingGrades++
			}
		}
		p.DistinctTitles = len(titles[p.ID])
		r.Periods = append(r.Periods, *p)
	}
	sort.Slice(r.Periods, func(i, j int) bool { return r.Periods[i].Start < r.Periods[j].Start })
	sort.Slice(r.Rows, func(i, j int) bool {
		a, b := r.Rows[i], r.Rows[j]
		if a.Start == b.Start {
			return a.StudentID < b.StudentID
		}
		return a.Start < b.Start
	})
	sort.Slice(r.Reading, func(i, j int) bool {
		a, b := r.Reading[i], r.Reading[j]
		if a.StudentID == b.StudentID {
			return a.Date < b.Date
		}
		return a.StudentID < b.StudentID
	})
	prior := map[string]ReadingObservation{}
	for i := range r.Reading {
		a := &r.Reading[i]
		key := fmt.Sprintf("%s/%s/%s/%d", a.StudentID, a.Instrument, a.Scale, a.Form)
		if b, ok := prior[key]; ok && a.Result != nil && b.Result != nil && b.Date < a.Date {
			delta := *a.Result - *b.Result
			a.Prior = b.Result
			a.PriorDate = b.Date
			a.Delta = &delta
		}
		prior[key] = *a
	}
	return r, nil
}
