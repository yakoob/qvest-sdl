package support

import (
	"strings"
	"testing"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/domain"
	"school_district_reading/internal/store"
)

func TestClassifiedInterestsUsesAllowlistedThemesNotProse(t *testing.T) {
	st := domain.Student{StudentID: "S-504", ReadingBand: "below", Cluster: "graphic", PageComfort: "short"}
	grade := "C"
	reading := 2
	rec := academics.Record{Semesters: []academics.SemesterIn{{Course: "English Language Arts", End: "2026-06-05", Status: "final", Scale: academics.ScaleLetter, Grade: &grade}}, Assessments: []academics.Assessment{{Name: "Willow Bend Reading Check", Scale: academics.ScaleWillow, Grade: 5, Date: "2026-09-02", Result: &reading}}}
	guidance := Record{Guidance: []Guidance{{Themes: []string{"sports", "underdogs"}}}, TeacherNotes: []TeacherNote{{Text: "secret classroom note"}}}
	query, themes, below := ClassifiedInterests(st, rec, guidance)
	if !below {
		t.Fatal("expected below-grade classification")
	}
	if !containsAll(themes, "sports", "underdogs") {
		t.Fatalf("themes %v", themes)
	}
	if !strings.Contains(query, "sports") || !strings.Contains(query, "short") {
		t.Fatalf("query %q", query)
	}
	if strings.Contains(query, "secret") || strings.Contains(strings.Join(themes, " "), "secret") {
		t.Fatal("raw note leaked into classified query")
	}
	on := domain.Student{StudentID: "S-402", ReadingBand: "above", Cluster: "fantasy"}
	q, th, onBelow := ClassifiedInterests(on, academics.Record{}, Record{Guidance: []Guidance{{Themes: []string{"sports"}}}})
	if onBelow || q != "" || th != nil {
		t.Fatalf("on-grade should not auto-classify, got %v %q %v", onBelow, q, th)
	}
}

func TestRosterBelowGradeStudentsHaveRichSupport(t *testing.T) {
	st, err := store.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := academics.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.Validate(st); err != nil {
		t.Fatal(err)
	}
	sup, err := Load("../../data/json", st)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.ByStudent) != 28 || len(sup.ByStudent) != 28 {
		t.Fatalf("coverage academics=%d support=%d", len(cat.ByStudent), len(sup.ByStudent))
	}
	for _, id := range []string{"S-301", "S-308", "S-406", "S-410", "S-504"} {
		r := cat.ByStudent[id]
		r.StudentID = id
		got := Evaluate(r, sup.ByStudent[id], DefaultConfig())
		if got.GradeStatus != BelowGrade {
			t.Fatalf("%s status %s", id, got.GradeStatus)
		}
		g := sup.ByStudent[id]
		if len(g.Strengths) < 2 || len(g.TeacherNotes) == 0 || len(g.Guidance) == 0 || len(g.Guidance[0].Themes) == 0 {
			t.Fatalf("%s missing rich support %+v", id, g)
		}
		_, themes, below := ClassifiedInterests(st.StudentByID[id], r, g)
		if !below || len(themes) == 0 {
			t.Fatalf("%s classified %+v", id, themes)
		}
	}
	for _, id := range []string{"S-405", "S-509", "S-302"} {
		if Evaluate(cat.ByStudent[id], sup.ByStudent[id], DefaultConfig()).GradeStatus != UnknownGrade {
			t.Fatalf("%s should stay unknown", id)
		}
	}
}

func containsAll(in []string, need ...string) bool {
	have := map[string]bool{}
	for _, v := range in {
		have[v] = true
	}
	for _, n := range need {
		if !have[n] {
			return false
		}
	}
	return true
}
