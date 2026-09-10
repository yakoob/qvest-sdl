package engagement

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/store"
)

func TestDemoValidationAndOptionalFile(t *testing.T) {
	st, err := store.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	acad, err := academics.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	demo, err := LoadDemo("../../data/json", st, acad)
	if err != nil || demo == nil {
		t.Fatalf("load: %v", err)
	}
	clone := func() *Demo {
		b, _ := json.Marshal(demo)
		var d Demo
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		return &d
	}
	for name, mutate := range map[string]func(*Demo){
		"duplicate contact":  func(d *Demo) { d.Contacts = append(d.Contacts, d.Contacts[0]) },
		"unknown student":    func(d *Demo) { d.Contacts[0].StudentID = "missing" },
		"unknown staff":      func(d *Demo) { d.Contacts[0].StaffID = "missing" },
		"invalid status":     func(d *Demo) { d.Contacts[0].Status = "invented" },
		"unknown window":     func(d *Demo) { d.Coverage[0].WindowID = "missing" },
		"invalid coverage":   func(d *Demo) { d.Coverage[0].EnrollmentEnd = d.Coverage[0].EnrollmentStart },
		"invalid provenance": func(d *Demo) { d.Source = "session" },
	} {
		t.Run(name, func(t *testing.T) {
			d := clone()
			mutate(d)
			if d.Validate(st, acad) == nil {
				t.Fatal("invalid fixture accepted")
			}
		})
	}
	dir := t.TempDir()
	if d, err := LoadDemo(dir, st, acad); err != nil || d != nil {
		t.Fatal("missing optional fixture")
	}
	if err := os.WriteFile(filepath.Join(dir, "engagement_demo.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if d, err := LoadDemo(dir, st, acad); err == nil || d != nil {
		t.Fatal("malformed fixture accepted")
	}
	if err := demo.Validate(st, nil); err != nil {
		t.Fatalf("optional academics: %v", err)
	}
}
