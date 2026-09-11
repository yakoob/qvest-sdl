package support

import (
	"fmt"
	"strings"
	"time"

	"school_district_reading/internal/academics"
)

const (
	First        = "first"
	Soon         = "soon"
	None         = "none"
	Insufficient = "insufficient"

	BelowGrade   = "below"
	OnGrade      = "on"
	AboveGrade   = "above"
	UnknownGrade = "unknown"
)

type Config struct {
	ID             string `json:"id"`
	AsOf           string `json:"as_of"`
	EnglishDays    int    `json:"english_days"`
	AssessmentDays int    `json:"assessment_days"`
	TeacherDays    int    `json:"teacher_days"`
}

func DefaultConfig() Config {
	return Config{"demo-v1", "2026-09-04", 270, 365, 90}
}

type Evidence struct {
	Kind     string `json:"kind"`
	Date     string `json:"date,omitempty"`
	Included bool   `json:"included"`
	Value    string `json:"value,omitempty"`
	Reason   string `json:"reason"`
	Rule     string `json:"rule,omitempty"`
}

type Result struct {
	StudentID        string     `json:"student_id"`
	Band             string     `json:"band"`
	Label            string     `json:"label"`
	GradeStatus      string     `json:"grade_status"`
	GradeStatusLabel string     `json:"grade_status_label"`
	English          string     `json:"english,omitempty"`
	Reading          string     `json:"reading,omitempty"`
	Coverage         string     `json:"coverage"`
	Evidence         []Evidence `json:"evidence"`
	Config           Config     `json:"config"`
	Disclaimer       string     `json:"disclaimer"`
}

// Evaluate is independent of circulation mutations, counseling themes and note text.
func Evaluate(rec academics.Record, guidance Record, cfg Config) Result {
	out := Result{StudentID: rec.StudentID, Band: Insufficient, Config: cfg,
		Disclaimer: "Unvalidated synthetic demo rules for librarian attention, not a diagnosis or prediction."}
	english := Evidence{Kind: "english", Reason: "No completed English grade in the supplied records"}
	var latest *academics.SemesterIn
	for i := range rec.Semesters {
		s := &rec.Semesters[i]
		if s.Status != "final" || s.End > cfg.AsOf || !strings.Contains(strings.ToLower(s.Course), "english") {
			continue
		}
		if latest == nil || s.End > latest.End {
			latest = s
		}
	}
	severity := 0
	if latest != nil {
		english.Date = latest.End
		switch {
		case !fresh(latest.End, cfg.AsOf, cfg.EnglishDays):
			english.Reason = "Historical or invalid date; excluded"
		case latest.Grade == nil:
			english.Reason = "Latest completed grade is missing; not treated as zero"
		case latest.Scale != academics.ScaleLetter:
			english.Reason = "Unknown grading scale; excluded"
		default:
			grade := *latest.Grade
			switch grade {
			case "A", "A+", "A-", "B", "B+", "B-", "C", "C+", "C-", "D", "D+", "D-", "F":
				english.Included = true
				english.Value = grade
				english.Reason = "Latest completed English grade (letter scale)"
				if grade[0] == 'F' {
					severity = 2
					english.Rule = "english_f"
				} else if grade[0] == 'C' || grade[0] == 'D' {
					severity = 1
					english.Rule = "english_c_d"
				}
			default:
				english.Reason = "Unrecognized letter grade; excluded"
			}
		}
	}
	assessment := Evidence{Kind: "assessment", Reason: "No reading assessment in the supplied records"}
	var last *academics.Assessment
	for i := range rec.Assessments {
		a := &rec.Assessments[i]
		if a.Date > cfg.AsOf {
			continue
		}
		if last == nil || a.Date > last.Date {
			last = a
		}
	}
	if last != nil {
		assessment.Date = last.Date
		switch {
		case !fresh(last.Date, cfg.AsOf, cfg.AssessmentDays):
			assessment.Reason = "Historical or invalid date; excluded"
		case last.Name != "Willow Bend Reading Check" || last.Scale != academics.ScaleWillow || last.Grade <= 0:
			assessment.Reason = "Unrecognized instrument, scale or grade form; excluded"
		case last.Result == nil:
			assessment.Reason = "Latest result missing; not treated as zero"
		case *last.Result < 1 || *last.Result > 4:
			assessment.Reason = "Result outside the fictional 1–4 scale; excluded"
		default:
			assessment.Included = true
			assessment.Value = fmt.Sprintf("%d / 4 · grade %d form", *last.Result, last.Grade)
			assessment.Reason = "Latest fictional reading check; no growth inferred across grade forms"
			if *last.Result == 1 {
				severity = 2
				assessment.Rule = "reading_1"
			} else if *last.Result == 2 {
				if severity < 1 {
					severity = 1
				}
				assessment.Rule = "reading_2"
			}
		}
	}
	teacher := Evidence{Kind: "teacher", Reason: "No structured teacher input in this demo file"}
	var request *TeacherNote
	for i := range guidance.TeacherNotes {
		n := &guidance.TeacherNotes[i]
		if n.Date > cfg.AsOf {
			continue
		}
		if request == nil || n.Date > request.Date {
			request = n
		}
	}
	if request != nil {
		teacher.Date = request.Date
		if !fresh(request.Date, cfg.AsOf, cfg.TeacherDays) {
			teacher.Reason = "Historical teacher input; excluded"
		} else {
			switch request.Request {
			case "none", "routine", "prompt":
				teacher.Included = true
				teacher.Value = request.Request
				teacher.Reason = "Explicit staff request, not an interpretation of note text"
				if request.Request == "prompt" {
					severity = 2
					teacher.Rule = "teacher_prompt"
				} else if request.Request == "routine" {
					if severity < 1 {
						severity = 1
					}
					teacher.Rule = "teacher_routine"
				}
			default:
				teacher.Reason = "Unknown request type; excluded"
			}
		}
	}
	out.Evidence = []Evidence{english, assessment, teacher, {Kind: "borrowing", Reason: "Observed borrowing is context only: complete enrollment and open-day export coverage are not established"}}
	count := 0
	for _, e := range out.Evidence {
		if e.Included {
			count++
		}
	}
	out.Coverage = fmt.Sprintf("%d of 3 decision inputs available; borrowing is contextual", count)
	switch {
	case severity == 2:
		out.Band = First
		out.Label = "Check in first"
	case severity == 1:
		out.Band = Soon
		out.Label = "Check in soon"
	case english.Included && assessment.Included:
		out.Band = None
		out.Label = "No current flags"
	default:
		out.Label = "Not enough information"
	}
	out.GradeStatus, out.GradeStatusLabel, out.English, out.Reading = gradeStatus(english, assessment)
	return out
}

func gradeStatus(english, assessment Evidence) (status, label, letter, reading string) {
	if english.Included {
		letter = english.Value
	}
	if assessment.Included {
		reading = assessment.Value
	}
	below, on, above := false, false, false
	if english.Included && letter != "" {
		switch letter[0] {
		case 'C', 'D', 'F':
			below = true
		case 'B':
			on = true
		case 'A':
			above = true
		}
	}
	if assessment.Included && assessment.Value != "" {
		switch assessment.Value[0] {
		case '1', '2':
			below = true
		case '3':
			on = true
		case '4':
			above = true
		}
	}
	switch {
	case below:
		return BelowGrade, "Below grade", letter, reading
	case !english.Included && !assessment.Included:
		return UnknownGrade, "Not enough information", letter, reading
	case above && !on:
		return AboveGrade, "Above grade", letter, reading
	case on:
		return OnGrade, "On grade", letter, reading
	default:
		return UnknownGrade, "Not enough information", letter, reading
	}
}

func fresh(date, asOf string, days int) bool {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false
	}
	a, err := time.Parse("2006-01-02", asOf)
	if err != nil {
		return false
	}
	age := a.Sub(d)
	return age >= 0 && age <= time.Duration(days)*24*time.Hour
}
