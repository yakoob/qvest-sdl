package metrics

import (
	"reflect"
	"school_district_reading/internal/academics"
	"school_district_reading/internal/engagement"
	"school_district_reading/internal/store"
	"testing"
)

func TestHistoricalPairs(t *testing.T) {
	st, err := store.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	acad, err := academics.Load("../../data/json")
	if err != nil {
		t.Fatal(err)
	}
	demo, err := engagement.LoadDemo("../../data/json", st, acad)
	if err != nil {
		t.Fatal(err)
	}
	f := Filter{Start: instant("2025-07-01T00:00:00Z"), End: instant("2026-07-01T00:00:00Z"), AsOf: instant("2026-09-04T00:00:00Z")}
	r, err := Paired(st, acad, demo, engagement.State{}, "historical", f)
	if err != nil {
		t.Fatal(err)
	}
	if r.Students != 8 || r.Borrowing.Eligible != 6 || r.Borrowing.Excluded != 2 {
		t.Fatalf("unexpected cohort: %+v", r)
	}
	for _, counts := range []DirectionCounts{r.Borrowing, r.English, r.Reading} {
		if counts.Eligible+counts.Excluded != len(r.Rows) || counts.Increased+counts.Unchanged+counts.Decreased != counts.Eligible {
			t.Fatal("summary does not reconcile")
		}
	}
	for _, row := range r.Rows {
		if row.StudentID == "S-305" && (row.Facilitator != "L-001" || len(row.LaterContacts) != 1 || row.Borrowing.Before != "8" || row.Borrowing.After != "10" || row.English.Direction != "increased") {
			t.Fatalf("Sofia %+v", row)
		}
		if row.StudentID == "S-402" && row.English.Direction != "decreased" {
			t.Fatal("decline hidden")
		}
	}
	f.StaffID = "L-002"
	filtered, err := Paired(st, acad, demo, engagement.State{}, "historical", f)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range filtered.Rows {
		if row.StudentID == "S-305" {
			t.Fatal("staff filter changed index contact")
		}
	}
	f.StaffID = ""
	f.AsOf = instant("2026-02-01T00:00:00Z")
	early, err := Paired(st, acad, demo, engagement.State{}, "historical", f)
	if err != nil || early.Borrowing.Eligible != 0 || early.English.Eligible != 0 || early.Reading.Eligible != 0 {
		t.Fatal("future evidence admitted")
	}
	f.AsOf = instant("2026-04-15T08:00:00Z")
	midWindow, err := Paired(st, acad, demo, engagement.State{}, "historical", f)
	if err != nil || midWindow.Reading.Eligible != 6 || midWindow.Borrowing.Eligible != 0 || midWindow.English.Eligible != 0 {
		t.Fatal("posted reading evidence must not wait for borrowing-window completion")
	}
	f.AsOf = instant("2026-09-04T00:00:00Z")
	at := instant("2026-01-15T17:00:00Z")
	live := engagement.State{Interactions: []engagement.Interaction{{ID: "live", StudentID: "S-305", Facilitator: "L-001", CompletedAt: &at}}}
	session, err := Paired(st, acad, demo, live, "session", f)
	if err != nil || session.Students != 1 || session.Borrowing.Eligible != 0 {
		t.Fatal("session used historical scenario")
	}
	absent, err := Paired(st, nil, demo, live, "historical", f)
	if err != nil || absent.English.Eligible != 0 {
		t.Fatal("optional academics")
	}
	again, err := Paired(st, acad, demo, live, "historical", f)
	if err != nil || !reflect.DeepEqual(r, again) {
		t.Fatal("live state changed historical report")
	}
}
func TestPairBoundariesAndCompatibility(t *testing.T) {
	observations := []pairedObservation{{id: "old", date: "2025-12-01", key: "form1", numeric: 1}, {id: "same-day", date: "2026-01-15", key: "form1", numeric: 2}, {id: "wrong-form", date: "2026-02-01", key: "form2", numeric: 4}}
	r := choosePair(observations, "2026-01-15", "2026-09-01", 180)
	if r.Direction != "excluded" {
		t.Fatal("incompatible or same-day pair")
	}
	observations = append(observations, pairedObservation{id: "compatible", date: "2026-03-01", key: "form1", numeric: 2})
	r = choosePair(observations, "2026-01-15", "2026-09-01", 180)
	if r.AfterID != "compatible" || r.Direction != "increased" {
		t.Fatal("compatible pair not found")
	}
}
