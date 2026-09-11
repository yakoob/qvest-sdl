package support

import (
	"reflect"
	"testing"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/store"
)

func TestBands(t *testing.T) {
	for _, tc := range []struct {
		name, grade   string
		reading       int
		request, want string
	}{
		{"good", "A-", 3, "none", None}, {"grade concern", "F", 4, "none", First},
		{"reading concern", "B", 1, "none", First}, {"grade watch", "C+", 3, "none", Soon},
		{"reading watch", "B", 2, "none", Soon}, {"teacher prompt", "A", 4, "prompt", First},
		{"teacher routine", "A", 4, "routine", Soon},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := academics.Record{StudentID: "S", Semesters: []academics.SemesterIn{{Course: "English Language Arts", End: "2026-06-05", Status: "final", Scale: academics.ScaleLetter, Grade: &tc.grade}}, Assessments: []academics.Assessment{{Name: "Willow Bend Reading Check", Scale: academics.ScaleWillow, Grade: 4, Date: "2026-09-02", Result: &tc.reading}}}
			g := Record{TeacherNotes: []TeacherNote{{Date: "2026-09-02", Request: tc.request}}}
			got := Evaluate(r, g, DefaultConfig())
			if got.Band != tc.want {
				t.Fatalf("got %s want %s: %+v", got.Band, tc.want, got)
			}
		})
	}
}

func TestGradeStatusFromEnglishAndReading(t *testing.T) {
	for _, tc := range []struct {
		name, grade, want string
		reading           int
	}{
		{"below english", "C", BelowGrade, 3},
		{"below reading", "B", BelowGrade, 2},
		{"on", "B+", OnGrade, 3},
		{"above", "A-", AboveGrade, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := academics.Record{Semesters: []academics.SemesterIn{{Course: "English Language Arts", End: "2026-06-05", Status: "final", Scale: academics.ScaleLetter, Grade: &tc.grade}}, Assessments: []academics.Assessment{{Name: "Willow Bend Reading Check", Scale: academics.ScaleWillow, Grade: 4, Date: "2026-09-02", Result: &tc.reading}}}
			got := Evaluate(r, Record{}, DefaultConfig())
			if got.GradeStatus != tc.want {
				t.Fatalf("got %s want %s (%s / %s)", got.GradeStatus, tc.want, got.English, got.Reading)
			}
		})
	}
	if got := Evaluate(academics.Record{}, Record{}, DefaultConfig()); got.GradeStatus != UnknownGrade {
		t.Fatalf("missing evidence %s", got.GradeStatus)
	}
}

func TestMissingAndInvalidEvidence(t *testing.T) {
	grade := "F"
	value := 1
	cases := []academics.Record{
		{},
		{Semesters: []academics.SemesterIn{{Course: "English", End: "2027-01-01", Status: "final", Scale: academics.ScaleLetter, Grade: &grade}}},
		{Semesters: []academics.SemesterIn{{Course: "English", End: "2026-06-05", Status: "in_progress", Scale: academics.ScaleLetter, Grade: &grade}}},
		{Semesters: []academics.SemesterIn{{Course: "English", End: "2026-06-05", Status: "final", Scale: "unknown", Grade: &grade}}},
		{Semesters: []academics.SemesterIn{{Course: "English", End: "2024-06-05", Status: "final", Scale: academics.ScaleLetter, Grade: &grade}}},
		{Assessments: []academics.Assessment{{Name: "Other test", Scale: academics.ScaleWillow, Grade: 4, Date: "2026-09-02", Result: &value}}},
		{Assessments: []academics.Assessment{{Name: "Willow Bend Reading Check", Scale: academics.ScaleWillow, Grade: 4, Date: "2026-09-02", Result: nil}}},
	}
	for i, r := range cases {
		if got := Evaluate(r, Record{}, DefaultConfig()); got.Band != Insufficient {
			t.Fatalf("case %d: %+v", i, got)
		}
	}
}

func TestLatestMissingDoesNotResurrectOldResult(t *testing.T) {
	value := 1
	r := academics.Record{Assessments: []academics.Assessment{
		{Name: "Willow Bend Reading Check", Scale: academics.ScaleWillow, Grade: 3, Date: "2026-05-01", Result: &value},
		{Name: "Willow Bend Reading Check", Scale: academics.ScaleWillow, Grade: 4, Date: "2026-09-02"},
	}}
	if got := Evaluate(r, Record{}, DefaultConfig()); got.Band != Insufficient {
		t.Fatal(got)
	}
}

func TestGuidanceAndNotesDoNotInferPriority(t *testing.T) {
	r := academics.Record{StudentID: "S"}
	a := Evaluate(r, Record{}, DefaultConfig())
	b := Evaluate(r, Record{Strengths: []string{"sports"}, Guidance: []Guidance{{Summary: "private", Themes: []string{"perseverance"}}}}, DefaultConfig())
	if !reflect.DeepEqual(a, b) {
		t.Fatal("counselor guidance changed priority")
	}
	stale := Record{TeacherNotes: []TeacherNote{{Date: "2026-02-01", Request: "prompt"}}}
	if got := Evaluate(r, stale, DefaultConfig()); got.Band != Insufficient {
		t.Fatal("stale teacher request used")
	}
	future := Record{TeacherNotes: []TeacherNote{{Date: "2027-01-01", Request: "prompt"}}}
	if got := Evaluate(r, future, DefaultConfig()); got.Band != Insufficient {
		t.Fatal("future teacher request used")
	}
}

func TestFreshBoundary(t *testing.T) {
	if !fresh("2026-09-01", "2026-09-04", 3) || fresh("2026-08-31", "2026-09-04", 3) || fresh("2026-09-05", "2026-09-04", 3) {
		t.Fatal("freshness boundary")
	}
}

func TestDemoFixtures(t *testing.T) {
	st, err := store.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	ac, err := academics.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Load("../../data/json", st)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"S-406": None, "S-402": None, "S-405": Insufficient, "S-509": Insufficient, "S-504": First, "S-510": None, "S-305": None} {
		r := ac.ByStudent[id]
		r.StudentID = id
		got := Evaluate(r, c.ByStudent[id], DefaultConfig())
		if got.Band != want {
			t.Fatalf("%s: %+v", id, got)
		}
	}
	mateo := ac.ByStudent["S-406"]
	mateo.StudentID = "S-406"
	if mateo.Scenario == nil || len(mateo.Scenario.Windows) == 0 || mateo.Scenario.Windows[len(mateo.Scenario.Windows)-1].EnglishGrade == nil || *mateo.Scenario.Windows[len(mateo.Scenario.Windows)-1].EnglishGrade != "A-" {
		t.Fatal("mateo scenario last grade")
	}
	mg := Evaluate(mateo, c.ByStudent["S-406"], DefaultConfig())
	if evidenceValue(mg, "english") != "A-" {
		t.Fatalf("mateo support english %q; scenario must not feed support", evidenceValue(mg, "english"))
	}
	tyler := ac.ByStudent["S-504"]
	tyler.StudentID = "S-504"
	if tyler.Scenario == nil || len(tyler.Scenario.Windows) == 0 || tyler.Scenario.Windows[len(tyler.Scenario.Windows)-1].EnglishGrade == nil || *tyler.Scenario.Windows[len(tyler.Scenario.Windows)-1].EnglishGrade != "A" {
		t.Fatal("tyler scenario last grade")
	}
	tg := Evaluate(tyler, c.ByStudent["S-504"], DefaultConfig())
	if evidenceValue(tg, "english") != "C" {
		t.Fatalf("tyler support english %q; scenario must not feed support", evidenceValue(tg, "english"))
	}
	sofia := ac.ByStudent["S-305"]
	sofia.StudentID = "S-305"
	sg := Evaluate(sofia, c.ByStudent["S-305"], DefaultConfig())
	if evidenceValue(sg, "english") != "B+" {
		t.Fatalf("sofia support english %q", evidenceValue(sg, "english"))
	}
	missing, err := Load(t.TempDir(), st)
	if err != nil || !missing.Missing {
		t.Fatalf("optional missing fixture: %v", err)
	}
}

func evidenceValue(r Result, kind string) string {
	for _, e := range r.Evidence {
		if e.Kind == kind && e.Included {
			return e.Value
		}
	}
	return ""
}
