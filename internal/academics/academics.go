package academics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/store"
)

const (
	SourceLabel     = "synthetic_demo"
	CoverageNote    = "Circulation export covers 2026-01-13 through 2026-09-03 plus this server session. Earlier borrowing is not in the extract and is not evidence of no borrowing."
	MissingFileNote = "No academic_demo.json loaded. Reading & learning is empty; recommendations still run."
	ScaleLetter     = "letter_A_F"
	ScaleWillow     = "willow_bend_reading_1_4"
)

type File struct {
	Provenance Provenance `json:"provenance"`
	Students   []Record   `json:"students"`
}

type Provenance struct {
	Synthetic           bool   `json:"synthetic"`
	Label               string `json:"label"`
	Note                string `json:"note"`
	CirculationCoverage string `json:"circulation_coverage"`
}

type Record struct {
	StudentID   string       `json:"student_id"`
	Semesters   []SemesterIn `json:"semesters"`
	Assessments []Assessment `json:"assessments"`
}

type SemesterIn struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Start       string  `json:"start"`
	End         string  `json:"end"`
	Course      string  `json:"course"`
	Grade       *string `json:"grade"`
	Scale       string  `json:"scale"`
	Status      string  `json:"status"`
	MissingNote string  `json:"missing_note,omitempty"`
}

type Assessment struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Date   string  `json:"date"`
	Grade  int     `json:"grade"`
	Scale  string  `json:"scale"`
	Result *int    `json:"result"`
	Band   *string `json:"band,omitempty"`
	Note   string  `json:"note,omitempty"`
}

type Catalog struct {
	Path       string
	Provenance Provenance
	ByStudent  map[string]Record
	Missing    bool
}

type View struct {
	StudentID           string           `json:"student_id"`
	Synthetic           bool             `json:"synthetic"`
	SourceLabel         string           `json:"source_label"`
	Note                string           `json:"note"`
	CirculationCoverage string           `json:"circulation_coverage"`
	Loaded              bool             `json:"loaded"`
	Semesters           []SemesterView   `json:"semesters"`
	Assessments         []AssessmentView `json:"assessments"`
	Disclaimer          string           `json:"disclaimer"`
}

type SemesterView struct {
	ID             string     `json:"id"`
	Label          string     `json:"label"`
	Start          string     `json:"start"`
	End            string     `json:"end"`
	Course         string     `json:"course"`
	EnglishGrade   *string    `json:"english_grade"`
	Scale          string     `json:"scale"`
	Status         string     `json:"status"`
	MissingNote    string     `json:"missing_note,omitempty"`
	CheckoutCount  int        `json:"checkout_count"`
	UniqueTitles   int        `json:"unique_titles"`
	BorrowingKnown bool       `json:"borrowing_known"`
	BorrowingNote  string     `json:"borrowing_note"`
	Borrowed       []Borrowed `json:"borrowed"`
}

type AssessmentView struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Date        string  `json:"date"`
	Grade       int     `json:"grade"`
	Scale       string  `json:"scale"`
	Result      *int    `json:"result"`
	Band        *string `json:"band,omitempty"`
	Note        string  `json:"note,omitempty"`
	Comparable  bool    `json:"comparable_to_prior"`
	CompareNote string  `json:"compare_note"`
}

type Borrowed struct {
	BookID       string `json:"book_id"`
	Title        string `json:"title"`
	CheckoutDate string `json:"checkout_date"`
	ReturnDate   string `json:"return_date,omitempty"`
	Renewal      bool   `json:"renewal_or_repeat"`
}

func Load(dir string) (*Catalog, error) {
	path := filepath.Join(dir, "academic_demo.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Catalog{
				Path:      path,
				Missing:   true,
				ByStudent: map[string]Record{},
				Provenance: Provenance{
					Synthetic:           true,
					Label:               SourceLabel,
					Note:                MissingFileNote,
					CirculationCoverage: CoverageNote,
				},
			}, nil
		}
		return nil, fmt.Errorf("read academics: %w", err)
	}
	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("json academics: %w", err)
	}
	c := &Catalog{
		Path:       path,
		Provenance: file.Provenance,
		ByStudent:  map[string]Record{},
	}
	if c.Provenance.Label == "" {
		c.Provenance.Label = SourceLabel
	}
	if c.Provenance.CirculationCoverage == "" {
		c.Provenance.CirculationCoverage = CoverageNote
	}
	c.Provenance.Synthetic = true
	for _, rec := range file.Students {
		c.ByStudent[rec.StudentID] = rec
	}
	return c, nil
}

func (c *Catalog) Validate(st *store.Store) error {
	if c == nil || c.Missing {
		return nil
	}
	for sid, rec := range c.ByStudent {
		if _, ok := st.Student(sid); !ok {
			return fmt.Errorf("academic record for unknown student %s", sid)
		}
		for _, sem := range rec.Semesters {
			if _, err := time.Parse("2006-01-02", sem.Start); err != nil {
				return fmt.Errorf("%s semester %s start: %w", sid, sem.ID, err)
			}
			if _, err := time.Parse("2006-01-02", sem.End); err != nil {
				return fmt.Errorf("%s semester %s end: %w", sid, sem.ID, err)
			}
			if sem.Grade != nil && strings.TrimSpace(*sem.Grade) == "" {
				return fmt.Errorf("%s semester %s empty grade string; use null", sid, sem.ID)
			}
			if sem.Scale != "" && sem.Scale != ScaleLetter {
				return fmt.Errorf("%s semester %s unexpected scale %s", sid, sem.ID, sem.Scale)
			}
		}
		for _, a := range rec.Assessments {
			if _, err := time.Parse("2006-01-02", a.Date); err != nil {
				return fmt.Errorf("%s assessment %s date: %w", sid, a.ID, err)
			}
			if a.Scale != ScaleWillow {
				return fmt.Errorf("%s assessment %s unexpected scale %s", sid, a.ID, a.Scale)
			}
			if a.Result != nil && (*a.Result < 1 || *a.Result > 4) {
				return fmt.Errorf("%s assessment %s result %d out of 1-4", sid, a.ID, *a.Result)
			}
		}
	}
	return nil
}

func (c *Catalog) View(studentID string, st *store.Store) View {
	note := ""
	label := SourceLabel
	coverage := CoverageNote
	if c != nil {
		note = c.Provenance.Note
		if c.Provenance.Label != "" {
			label = c.Provenance.Label
		}
		if c.Provenance.CirculationCoverage != "" {
			coverage = c.Provenance.CirculationCoverage
		}
	}
	if note == "" {
		note = "Synthetic desk fixture. Not a district SIS extract. Grades do not change when a book is checked out."
	}
	view := View{
		StudentID:           studentID,
		Synthetic:           true,
		SourceLabel:         label,
		Note:                note,
		CirculationCoverage: coverage,
		Loaded:              c != nil && !c.Missing,
		Disclaimer:          "Borrowed is not finished. These figures are not evidence that a title caused a grade change. Do not compare scores across different grade-year tests.",
		Semesters:           []SemesterView{},
		Assessments:         []AssessmentView{},
	}
	if c == nil || c.Missing {
		view.Loaded = false
		view.Note = MissingFileNote
		return view
	}
	rec, ok := c.ByStudent[studentID]
	if !ok {
		view.Note = "No synthetic academic rows for this student. Missing is not a zero."
		return view
	}
	var hist []domain.CirculationEvent
	if st != nil {
		hist = st.History[studentID]
	}
	for _, sem := range rec.Semesters {
		sv := SemesterView{
			ID:           sem.ID,
			Label:        sem.Label,
			Start:        sem.Start,
			End:          sem.End,
			Course:       sem.Course,
			EnglishGrade: sem.Grade,
			Scale:        sem.Scale,
			Status:       sem.Status,
			MissingNote:  sem.MissingNote,
		}
		if sv.Scale == "" {
			sv.Scale = ScaleLetter
		}
		sv.Borrowed, sv.CheckoutCount, sv.UniqueTitles = joinLoans(hist, st, sem.Start, sem.End)
		covered := overlapsCoverage(sem.Start, sem.End)
		sv.BorrowingKnown = covered
		switch {
		case !covered:
			sv.BorrowingNote = "This window is mostly outside the circulation export. Title lists are only the overlapping dates; absence is not zero borrowing."
		case len(sv.Borrowed) == 0:
			sv.BorrowingNote = "No checkouts in this extract for these dates. That is not proof the student borrowed nothing."
		default:
			sv.BorrowingNote = "Titles borrowed (or renewed) whose checkout date falls in this semester. Not a claim they were finished or caused the grade."
		}
		view.Semesters = append(view.Semesters, sv)
	}
	sort.SliceStable(view.Semesters, func(i, j int) bool {
		return view.Semesters[i].Start < view.Semesters[j].Start
	})
	prevSame := map[string]Assessment{}
	prevName := map[string]Assessment{}
	assess := append([]Assessment(nil), rec.Assessments...)
	sort.SliceStable(assess, func(i, j int) bool { return assess[i].Date < assess[j].Date })
	for _, a := range assess {
		av := AssessmentView{
			ID:     a.ID,
			Name:   a.Name,
			Date:   a.Date,
			Grade:  a.Grade,
			Scale:  a.Scale,
			Result: a.Result,
			Band:   a.Band,
			Note:   a.Note,
		}
		key := fmt.Sprintf("%s|%s|%d", a.Name, a.Scale, a.Grade)
		nameKey := a.Name + "|" + a.Scale
		if prior, ok := prevSame[key]; ok && a.Result != nil && prior.Result != nil {
			av.Comparable = true
			av.CompareNote = fmt.Sprintf("Same assessment, scale, and grade as %s. Shown as two dated results — not a growth score.", prior.Date)
		} else if prior, ok := prevName[nameKey]; ok {
			av.Comparable = false
			av.CompareNote = fmt.Sprintf("Same instrument name as %s but grade %d vs %d. Do not subtract; grade-year tests are not a delta.", prior.Date, prior.Grade, a.Grade)
		} else if a.Result == nil {
			av.CompareNote = "Result missing. Displayed as missing, not zero."
		} else {
			av.CompareNote = "No prior result on the same grade, scale, and instrument."
		}
		prevSame[key] = a
		prevName[nameKey] = a
		view.Assessments = append(view.Assessments, av)
	}
	return view
}

func joinLoans(hist []domain.CirculationEvent, st *store.Store, start, end string) ([]Borrowed, int, int) {
	seen := map[string]int{}
	out := []Borrowed{}
	for _, ev := range hist {
		if ev.CheckoutDate < start || ev.CheckoutDate > end {
			continue
		}
		title := ev.BookID
		if st != nil {
			if b, ok := st.Book(ev.BookID); ok {
				title = b.Title
			}
		}
		seen[ev.BookID]++
		out = append(out, Borrowed{
			BookID:       ev.BookID,
			Title:        title,
			CheckoutDate: ev.CheckoutDate,
			ReturnDate:   ev.ReturnDate,
			Renewal:      seen[ev.BookID] > 1,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CheckoutDate == out[j].CheckoutDate {
			return out[i].BookID < out[j].BookID
		}
		return out[i].CheckoutDate < out[j].CheckoutDate
	})
	return out, len(out), len(seen)
}

func overlapsCoverage(start, end string) bool {
	const covStart, covEnd = "2026-01-13", "2026-09-03"
	return end >= covStart && start <= covEnd
}
