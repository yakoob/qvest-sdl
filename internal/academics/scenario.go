package academics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"school_district_reading/internal/store"
)

const (
	DemoImprovingEngagement = "improving_engagement_illustrative"
	DemoStableComparator    = "stable_comparator_illustrative"
	DemoStrongStable        = "strong_stable_illustrative"
	DemoNewcomerMissing     = "newcomer_missing_illustrative"
	DemoZeroHistory         = "zero_history_illustrative"
	DemoSupportFlat         = "support_flat_illustrative"
	MatchedWindowDays       = 84
	CurrentAcademicYear     = "2026-27"
	// ScenarioAsOf is the extract date. Completed scenario observations
	// must not end after it.
	ScenarioAsOf = "2026-09-04"
	// CompletedScenarioYears / CompletedScenarioWindows describe the
	// isolated three-year, six-semester matched-window coverage.
	CompletedScenarioYears   = 3
	CompletedScenarioWindows = 6
)

type ScenarioCalendar struct {
	Timezone            string       `json:"timezone"`
	DateFormat          string       `json:"date_format"`
	CurrentAcademicYear string       `json:"current_academic_year"`
	Terms               []TermSpec   `json:"terms"`
	MatchedWindows      []WindowSpec `json:"matched_windows"`
}

type TermSpec struct {
	ID           string `json:"id"`
	AcademicYear string `json:"academic_year"`
	Semester     string `json:"semester"`
	Label        string `json:"label"`
	Start        string `json:"start"`
	End          string `json:"end"`
}

type WindowSpec struct {
	ID            string `json:"id"`
	AcademicYear  string `json:"academic_year"`
	Semester      string `json:"semester"`
	Label         string `json:"label"`
	Start         string `json:"start"`
	End           string `json:"end"`
	InclusiveDays int    `json:"inclusive_days"`
}

type ScenarioIn struct {
	ID                     string             `json:"id"`
	Label                  string             `json:"label"`
	Caveat                 string             `json:"caveat"`
	IsolatedFromOperations bool               `json:"isolated_from_operations"`
	Windows                []ScenarioWindowIn `json:"windows"`
}

type ScenarioWindowIn struct {
	ID            string           `json:"id"`
	AcademicYear  string           `json:"academic_year"`
	Semester      string           `json:"semester"`
	SchoolGrade   int              `json:"school_grade"`
	Start         string           `json:"start"`
	End           string           `json:"end"`
	EnglishGrade  *string          `json:"english_grade"`
	EnglishStatus string           `json:"english_status"`
	ReadingCheck  *ScenarioReading `json:"reading_check,omitempty"`
	Borrowed      []ScenarioLoanIn `json:"borrowed"`
}

type ScenarioReading struct {
	Date      string `json:"date"`
	GradeForm int    `json:"grade_form"`
	Result    *int   `json:"result"`
	Band      string `json:"band,omitempty"`
}

type ScenarioLoanIn struct {
	BookID       string `json:"book_id"`
	CheckoutDate string `json:"checkout_date"`
	ReturnDate   string `json:"return_date,omitempty"`
}

type ScenarioView struct {
	ID                     string               `json:"id,omitempty"`
	Label                  string               `json:"label,omitempty"`
	Caveat                 string               `json:"caveat,omitempty"`
	IsolatedFromOperations bool                 `json:"isolated_from_operations,omitempty"`
	CurrentAcademicYear    string               `json:"current_academic_year,omitempty"`
	MatchedWindowDays      int                  `json:"matched_window_days,omitempty"`
	Windows                []ScenarioWindowView `json:"windows,omitempty"`
}

type ScenarioWindowView struct {
	ID             string           `json:"id"`
	AcademicYear   string           `json:"academic_year"`
	Semester       string           `json:"semester"`
	SchoolGrade    int              `json:"school_grade"`
	Start          string           `json:"start"`
	End            string           `json:"end"`
	InclusiveDays  int              `json:"inclusive_days"`
	EnglishGrade   *string          `json:"english_grade"`
	EnglishStatus  string           `json:"english_status"`
	ReadingCheck   *ScenarioReading `json:"reading_check,omitempty"`
	Borrowed       []Borrowed       `json:"borrowed"`
	CheckoutCount  int              `json:"checkout_count"`
	UniqueTitles   int              `json:"unique_titles"`
	BorrowingKnown bool             `json:"borrowing_known"`
}

func parseClosedRange(start, end string) (time.Time, time.Time, error) {
	s, err := ParseDateOnly(start)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start: %w", err)
	}
	e, err := ParseDateOnly(end)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end: %w", err)
	}
	if e.Before(s) {
		return time.Time{}, time.Time{}, fmt.Errorf("end %s before start %s", end, start)
	}
	return s, e, nil
}

func validLetter(g string) bool {
	switch g {
	case "A+", "A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D+", "D", "D-", "F":
		return true
	default:
		return false
	}
}

func letterRank(g string) int {
	order := []string{"F", "D-", "D", "D+", "C-", "C", "C+", "B-", "B", "B+", "A-", "A", "A+"}
	for i, v := range order {
		if v == g {
			return i
		}
	}
	return -1
}

func (c *Catalog) validateCalendar() error {
	if c == nil || c.Calendar == nil {
		return nil
	}
	cal := c.Calendar
	if cal.Timezone != "" && !strings.EqualFold(cal.Timezone, "UTC") {
		return fmt.Errorf("scenario calendar timezone must be UTC")
	}
	if cal.CurrentAcademicYear == "" {
		cal.CurrentAcademicYear = CurrentAcademicYear
	}
	if _, _, err := parseAcademicYear(cal.CurrentAcademicYear); err != nil {
		return fmt.Errorf("scenario calendar current_academic_year: %w", err)
	}
	byTerm := map[string]TermSpec{}
	for i, term := range cal.Terms {
		start, end, err := parseClosedRange(term.Start, term.End)
		if err != nil {
			return fmt.Errorf("term %s: %w", term.ID, err)
		}
		wantStart, wantEnd, err := TermBounds(term.AcademicYear, term.Semester)
		if err != nil {
			return fmt.Errorf("term %s bounds: %w", term.ID, err)
		}
		if !start.Equal(wantStart) || !end.Equal(wantEnd) {
			return fmt.Errorf("term %s does not match canonical %s %s bounds", term.ID, term.AcademicYear, term.Semester)
		}
		cal.Terms[i].Start = FormatDateOnly(start)
		cal.Terms[i].End = FormatDateOnly(end)
		byTerm[term.AcademicYear+"|"+strings.ToUpper(term.Semester)] = cal.Terms[i]
	}
	sort.SliceStable(cal.Terms, func(i, j int) bool {
		a, _ := ParseDateOnly(cal.Terms[i].Start)
		b, _ := ParseDateOnly(cal.Terms[j].Start)
		if a.Equal(b) {
			return cal.Terms[i].ID < cal.Terms[j].ID
		}
		return a.Before(b)
	})
	asOf, err := ParseDateOnly(ScenarioAsOf)
	if err != nil {
		return fmt.Errorf("scenario as_of: %w", err)
	}
	type calSpan struct {
		idx        int
		id         string
		start, end time.Time
		days       int
	}
	spans := make([]calSpan, 0, len(cal.MatchedWindows))
	var windowDays int
	for i, win := range cal.MatchedWindows {
		start, end, err := parseClosedRange(win.Start, win.End)
		if err != nil {
			return fmt.Errorf("matched window %s: %w", win.ID, err)
		}
		days := InclusiveDays(start, end)
		if win.InclusiveDays != 0 && win.InclusiveDays != days {
			return fmt.Errorf("matched window %s inclusive_days %d != %d", win.ID, win.InclusiveDays, days)
		}
		if days != MatchedWindowDays {
			return fmt.Errorf("matched window %s is %d days, want %d", win.ID, days, MatchedWindowDays)
		}
		if windowDays == 0 {
			windowDays = days
		} else if days != windowDays {
			return fmt.Errorf("matched window %s length %d != %d", win.ID, days, windowDays)
		}
		if end.After(asOf) {
			return fmt.Errorf("matched window %s completed observation ends after %s", win.ID, ScenarioAsOf)
		}
		term, ok := byTerm[win.AcademicYear+"|"+strings.ToUpper(win.Semester)]
		if !ok {
			return fmt.Errorf("matched window %s has no term %s %s", win.ID, win.AcademicYear, win.Semester)
		}
		termStart, termEnd, err := parseClosedRange(term.Start, term.End)
		if err != nil {
			return err
		}
		if start.Before(termStart) || end.After(termEnd) {
			return fmt.Errorf("matched window %s outside %s %s", win.ID, win.AcademicYear, win.Semester)
		}
		cal.MatchedWindows[i].Start = FormatDateOnly(start)
		cal.MatchedWindows[i].End = FormatDateOnly(end)
		cal.MatchedWindows[i].InclusiveDays = days
		spans = append(spans, calSpan{idx: i, id: win.ID, start: start, end: end, days: days})
	}
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].start.Equal(spans[j].start) {
			return spans[i].id < spans[j].id
		}
		return spans[i].start.Before(spans[j].start)
	})
	ordered := make([]WindowSpec, 0, len(spans))
	for i, sp := range spans {
		if i > 0 && RangesOverlap(spans[i-1].start, spans[i-1].end, sp.start, sp.end) {
			return fmt.Errorf("matched windows %s and %s overlap", spans[i-1].id, sp.id)
		}
		ordered = append(ordered, cal.MatchedWindows[sp.idx])
	}
	cal.MatchedWindows = ordered
	if n := len(cal.MatchedWindows); n != 0 && n != CompletedScenarioWindows {
		return fmt.Errorf("scenario calendar has %d matched windows, want %d", n, CompletedScenarioWindows)
	}
	return nil
}

func (c *Catalog) validateScenario(sid string, rec Record, st *store.Store) error {
	sc := rec.Scenario
	if sc == nil {
		return nil
	}
	if !sc.IsolatedFromOperations {
		return fmt.Errorf("%s scenario must be isolated_from_operations", sid)
	}
	if strings.TrimSpace(sc.Label) == "" || strings.TrimSpace(sc.Caveat) == "" {
		return fmt.Errorf("%s scenario needs a label and one caveat", sid)
	}
	stu, ok := st.Student(sid)
	if !ok {
		return fmt.Errorf("%s unknown student", sid)
	}
	currentYear := CurrentAcademicYear
	if c.Calendar != nil && c.Calendar.CurrentAcademicYear != "" {
		currentYear = c.Calendar.CurrentAcademicYear
	}
	asOf, err := ParseDateOnly(ScenarioAsOf)
	if err != nil {
		return fmt.Errorf("scenario as_of: %w", err)
	}
	seen := map[string]bool{}
	type span struct {
		id         string
		start, end time.Time
	}
	spans := make([]span, 0, len(sc.Windows))
	var windowDays int
	for i := range sc.Windows {
		w := &sc.Windows[i]
		if seen[w.ID] {
			return fmt.Errorf("%s duplicate scenario window %s", sid, w.ID)
		}
		seen[w.ID] = true
		start, end, err := parseClosedRange(w.Start, w.End)
		if err != nil {
			return fmt.Errorf("%s window %s: %w", sid, w.ID, err)
		}
		w.Start = FormatDateOnly(start)
		w.End = FormatDateOnly(end)
		days := InclusiveDays(start, end)
		if days != MatchedWindowDays {
			return fmt.Errorf("%s window %s is %d days, want %d", sid, w.ID, days, MatchedWindowDays)
		}
		if windowDays == 0 {
			windowDays = days
		} else if days != windowDays {
			return fmt.Errorf("%s window %s length %d != %d", sid, w.ID, days, windowDays)
		}
		if strings.EqualFold(strings.TrimSpace(w.EnglishStatus), "final") && end.After(asOf) {
			return fmt.Errorf("%s window %s completed observation ends after %s", sid, w.ID, ScenarioAsOf)
		}
		termStart, termEnd, err := TermBounds(w.AcademicYear, w.Semester)
		if err != nil {
			return fmt.Errorf("%s window %s year: %w", sid, w.ID, err)
		}
		if start.Before(termStart) || end.After(termEnd) {
			return fmt.Errorf("%s window %s outside %s %s", sid, w.ID, w.AcademicYear, w.Semester)
		}
		wantGrade, err := ExpectedSchoolGrade(stu.Grade, currentYear, w.AcademicYear)
		if err != nil {
			return fmt.Errorf("%s window %s grade: %w", sid, w.ID, err)
		}
		if w.SchoolGrade != wantGrade {
			return fmt.Errorf("%s window %s school_grade %d want %d", sid, w.ID, w.SchoolGrade, wantGrade)
		}
		if stu.NewThisYear {
			wy, _, _ := parseAcademicYear(w.AcademicYear)
			cy, _, _ := parseAcademicYear(currentYear)
			if wy < cy {
				return fmt.Errorf("%s is new this year; scenario must not invent prior local years (%s)", sid, w.AcademicYear)
			}
		}
		if w.EnglishGrade != nil && !validLetter(*w.EnglishGrade) {
			return fmt.Errorf("%s window %s bad english grade %q", sid, w.ID, *w.EnglishGrade)
		}
		if w.ReadingCheck != nil {
			rd, err := ParseDateOnly(w.ReadingCheck.Date)
			if err != nil {
				return fmt.Errorf("%s window %s reading date: %w", sid, w.ID, err)
			}
			if !InClosedRange(rd, start, end) {
				return fmt.Errorf("%s window %s reading date outside window", sid, w.ID)
			}
			w.ReadingCheck.Date = FormatDateOnly(rd)
			if w.ReadingCheck.GradeForm != w.SchoolGrade {
				return fmt.Errorf("%s window %s reading form %d != school grade %d", sid, w.ID, w.ReadingCheck.GradeForm, w.SchoolGrade)
			}
			if w.ReadingCheck.Result != nil && (*w.ReadingCheck.Result < 1 || *w.ReadingCheck.Result > 4) {
				return fmt.Errorf("%s window %s reading result", sid, w.ID)
			}
		}
		spans = append(spans, span{id: w.ID, start: start, end: end})
		for j, loan := range w.Borrowed {
			ld, err := ParseDateOnly(loan.CheckoutDate)
			if err != nil {
				return fmt.Errorf("%s window %s loan %d: %w", sid, w.ID, j, err)
			}
			if !InClosedRange(ld, start, end) {
				return fmt.Errorf("%s window %s loan %s outside window", sid, w.ID, loan.BookID)
			}
			if _, ok := st.Book(loan.BookID); !ok {
				return fmt.Errorf("%s window %s unknown book %s", sid, w.ID, loan.BookID)
			}
			if loan.ReturnDate != "" {
				rd, err := ParseDateOnly(loan.ReturnDate)
				if err != nil {
					return fmt.Errorf("%s window %s return %s: %w", sid, w.ID, loan.BookID, err)
				}
				w.Borrowed[j].ReturnDate = FormatDateOnly(rd)
			}
			w.Borrowed[j].CheckoutDate = FormatDateOnly(ld)
		}
	}
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].start.Equal(spans[j].start) {
			return spans[i].id < spans[j].id
		}
		return spans[i].start.Before(spans[j].start)
	})
	for i := 1; i < len(spans); i++ {
		if RangesOverlap(spans[i-1].start, spans[i-1].end, spans[i].start, spans[i].end) {
			return fmt.Errorf("%s windows %s and %s overlap", sid, spans[i-1].id, spans[i].id)
		}
	}
	return nil
}

func (c *Catalog) scenarioView(studentID string, rec Record, st *store.Store) *ScenarioView {
	if rec.Scenario == nil {
		return nil
	}
	sc := rec.Scenario
	currentYear := CurrentAcademicYear
	if c != nil && c.Calendar != nil && c.Calendar.CurrentAcademicYear != "" {
		currentYear = c.Calendar.CurrentAcademicYear
	}
	out := &ScenarioView{
		ID:                     sc.ID,
		Label:                  sc.Label,
		Caveat:                 sc.Caveat,
		IsolatedFromOperations: true,
		CurrentAcademicYear:    currentYear,
		MatchedWindowDays:      MatchedWindowDays,
		Windows:                []ScenarioWindowView{},
	}
	windows := append([]ScenarioWindowIn(nil), sc.Windows...)
	sort.SliceStable(windows, func(i, j int) bool {
		a, _ := ParseDateOnly(windows[i].Start)
		b, _ := ParseDateOnly(windows[j].Start)
		if a.Equal(b) {
			return windows[i].ID < windows[j].ID
		}
		return a.Before(b)
	})
	for _, w := range windows {
		start, end, err := parseClosedRange(w.Start, w.End)
		if err != nil {
			continue
		}
		sv := ScenarioWindowView{
			ID:             w.ID,
			AcademicYear:   w.AcademicYear,
			Semester:       w.Semester,
			SchoolGrade:    w.SchoolGrade,
			Start:          FormatDateOnly(start),
			End:            FormatDateOnly(end),
			InclusiveDays:  InclusiveDays(start, end),
			EnglishGrade:   w.EnglishGrade,
			EnglishStatus:  w.EnglishStatus,
			ReadingCheck:   w.ReadingCheck,
			Borrowed:       []Borrowed{},
			BorrowingKnown: true,
		}
		seen := map[string]int{}
		loans := append([]ScenarioLoanIn(nil), w.Borrowed...)
		sort.SliceStable(loans, func(i, j int) bool {
			if loans[i].CheckoutDate == loans[j].CheckoutDate {
				return loans[i].BookID < loans[j].BookID
			}
			ai, _ := ParseDateOnly(loans[i].CheckoutDate)
			aj, _ := ParseDateOnly(loans[j].CheckoutDate)
			return ai.Before(aj)
		})
		for _, loan := range loans {
			title := loan.BookID
			if st != nil {
				if b, ok := st.Book(loan.BookID); ok {
					title = b.Title
				}
			}
			seen[loan.BookID]++
			sv.Borrowed = append(sv.Borrowed, Borrowed{
				BookID:       loan.BookID,
				Title:        title,
				CheckoutDate: loan.CheckoutDate,
				ReturnDate:   loan.ReturnDate,
				Renewal:      seen[loan.BookID] > 1,
			})
		}
		sv.CheckoutCount = len(sv.Borrowed)
		sv.UniqueTitles = len(seen)
		out.Windows = append(out.Windows, sv)
	}
	return out
}
