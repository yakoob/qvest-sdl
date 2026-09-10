package metrics

import (
	"reflect"
	"testing"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/store"
)

func TestProgressReconcilesStudentViews(t *testing.T) {
	st, err := store.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	acad, err := academics.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"scenario", "extract"} {
		report, err := Progress(st, acad, source)
		if err != nil {
			t.Fatal(err)
		}
		if source == "scenario" && len(report.Periods) != 6 {
			t.Fatal("lost six windows")
		}
		for _, p := range report.Periods {
			checkouts, unique, full, partial, missing, missingGrades, count := 0, 0, 0, 0, 0, 0, 0
			grades := map[string]int{}
			titles := map[string]bool{}
			for _, row := range report.Rows {
				if row.Period != p.ID {
					continue
				}
				count++
				checkouts += row.Checkouts
				unique += row.UniqueTitles
				switch row.Coverage {
				case "full":
					full++
				case "partial":
					partial++
				default:
					missing++
				}
				if row.English == nil {
					missingGrades++
				} else {
					grades[*row.English]++
				}
				for _, book := range row.Borrowed {
					titles[book.BookID] = true
				}
				view := acad.View(row.StudentID, st)
				if source == "scenario" && view.Scenario != nil {
					for _, w := range view.Scenario.Windows {
						if w.ID == row.Period && (w.CheckoutCount != row.Checkouts || w.UniqueTitles != row.UniqueTitles || !reflect.DeepEqual(w.EnglishGrade, row.English)) {
							t.Fatal("student scenario mismatch")
						}
					}
				}
			}
			if count != len(st.Students) || checkouts != p.Checkouts || unique != p.StudentTitles || len(titles) != p.DistinctTitles || full != p.Covered || partial != p.Partial || missing != p.Missing || missingGrades != p.MissingGrades || !reflect.DeepEqual(grades, p.Grades) {
				t.Fatalf("%s does not reconcile: %+v", source, p)
			}
		}
		for _, r := range report.Reading {
			if r.Delta != nil {
				found := false
				for _, prior := range report.Reading {
					if prior.StudentID == r.StudentID && prior.Date == r.PriorDate && prior.Instrument == r.Instrument && prior.Scale == r.Scale && prior.Form == r.Form && prior.Result != nil {
						found = *r.Delta == *r.Result-*prior.Result
					}
				}
				if !found {
					t.Fatalf("incompatible reading pair: %+v", r)
				}
			}
		}
	}
	empty, err := Progress(st, nil, "scenario")
	if err != nil || len(empty.Rows) != 0 || len(empty.StudentsWithoutRecords) != len(st.Students) {
		t.Fatal("optional academics unavailable handling")
	}
	if _, err := Progress(st, acad, "session"); err == nil {
		t.Fatal("unsupported source accepted")
	}
}
