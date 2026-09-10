package metrics

import (
	"school_district_reading/internal/engagement"
	"testing"
	"time"
)

func instant(s string) time.Time {
	v, e := time.Parse(time.RFC3339, s)
	if e != nil {
		panic(e)
	}
	return v
}
func TestCohortMaturityAndFeedback(t *testing.T) {
	at := instant("2026-09-01T16:00:00Z")
	end := at.AddDate(0, 1, 0)
	late := at.Add(15 * 24 * time.Hour)
	early := at.Add(time.Hour)
	future := end.Add(time.Hour)
	s := engagement.State{
		Interactions: []engagement.Interaction{{ID: "i1", StudentID: "s1", Facilitator: "a", CompletedAt: &at}, {ID: "i2", StudentID: "s1", Facilitator: "b", CompletedAt: &at}, {ID: "i3", StudentID: "s2", Facilitator: "a", CompletedAt: &at}, {ID: "open", StudentID: "s3", Facilitator: "a"}},
		Offers:       []engagement.Offer{{ID: "o1", InteractionID: "i1", At: at}, {ID: "o2", InteractionID: "i2", At: at}, {ID: "o3", InteractionID: "i3", At: at}},
		Choices:      []engagement.Choice{{ID: "c1", InteractionID: "i1", BookID: "b1", At: at, LoanID: "l1", CheckoutAt: &early}, {ID: "c2", InteractionID: "i2", BookID: "b2", At: at, LoanID: "l2", CheckoutAt: &late}, {ID: "c3", InteractionID: "i3", Source: "none", At: at}},
		Feedback:     []engagement.Feedback{{InteractionID: "i1", BookID: "b1", Reading: "reading", Enjoyment: "no", Source: "student_reported", At: at}, {InteractionID: "i1", BookID: "b1", Reading: "finished", Enjoyment: "neutral", Source: "student_reported", At: early}, {InteractionID: "i1", BookID: "b1", Reading: "stopped", Enjoyment: "no", Source: "staff_observed", At: late}, {InteractionID: "i2", BookID: "b2", Reading: "finished", Enjoyment: "yes", Source: "student_reported", At: future}},
		Followups:    []engagement.Followup{{ID: "f1", InteractionID: "i1", CreatedAt: at, Due: early, CompletedAt: &late}, {ID: "f2", InteractionID: "i2", CreatedAt: at, Due: late}},
	}
	f := Filter{Start: at, End: end, AsOf: end}
	r := Aggregate(s, f)
	if r.StudentsServed != 2 || r.Completed != 3 || r.StudentsChoosing != 1 || r.NoneToday != 1 {
		t.Fatalf("cohort %+v", r)
	}
	if r.Conversion != (Ratio{1, 2}) || r.Acceptance != (Ratio{2, 3}) || r.LinkedCheckouts != 2 {
		t.Fatalf("conversion %+v", r)
	}
	if r.ReadingCompletion != (Ratio{1, 1}) || r.ResponseCoverage != (Ratio{1, 2}) || r.Neutral != 1 || r.UnknownReading != 1 || r.StaffObservations != 1 {
		t.Fatalf("feedback %+v", r)
	}
	if r.FollowupCoverage != (Ratio{1, 2}) || r.Overdue != 1 {
		t.Fatalf("followups %+v", r)
	}
	f.StaffID = "b"
	r = Aggregate(s, f)
	if r.StudentsServed != 1 || r.Completed != 1 || r.Conversion != (Ratio{0, 1}) {
		t.Fatalf("staff %+v", r)
	}
	f.StaffID = ""
	f.AsOf = early
	r = Aggregate(s, f)
	if r.PendingChoices != 2 || r.Conversion.Denominator != 0 || r.LinkedCheckouts != 1 || r.FollowupCoverage != (Ratio{0, 1}) || r.PendingFollowups != 1 {
		t.Fatalf("asof %+v", r)
	}
	for _, c := range r.Choices {
		if c.ID == "c2" && c.LoanID != "" {
			t.Fatal("future link in drilldown")
		}
	}
}
func TestFollowupReschedulingAsOf(t *testing.T) {
	created := instant("2026-09-01T16:00:00Z")
	due := created.Add(24 * time.Hour)
	moved := due.Add(24 * time.Hour)
	completed := moved.Add(time.Hour)
	s := engagement.State{Interactions: []engagement.Interaction{{ID: "i", StudentID: "s", Facilitator: "original", CompletedAt: &created}}, Followups: []engagement.Followup{{ID: "f", InteractionID: "i", CreatedAt: created, Due: moved, AppointmentID: "new", CompletedAt: &completed, ContactID: "contact", StaffID: "other", Reservations: []engagement.Reservation{{At: created, Due: due, AppointmentID: "old", Status: "scheduled"}, {At: due.Add(time.Minute), Due: moved, AppointmentID: "new", Status: "scheduled"}}}}}
	filter := Filter{Start: created, End: due.Add(time.Hour), AsOf: due.Add(2 * time.Hour), StaffID: "original"}
	r := Aggregate(s, filter)
	if r.Overdue != 1 || r.FollowupCoverage.Denominator != 1 || r.Followups[0].ContactID != "" || !r.Followups[0].Due.Equal(due) {
		t.Fatalf("late rebooking erased due cohort: %+v", r)
	}
	filter.AsOf = completed
	r = Aggregate(s, filter)
	if r.FollowupCoverage != (Ratio{1, 1}) {
		t.Fatalf("contact not counted: %+v", r)
	}
	filter.StaffID = "other"
	if Aggregate(s, filter).FollowupCoverage.Denominator != 0 {
		t.Fatal("booking staff changed cohort owner")
	}
}

func TestEmptyAndExclusiveBoundary(t *testing.T) {
	at := instant("2026-09-01T00:00:00Z")
	end := at.Add(24 * time.Hour)
	s := engagement.State{Interactions: []engagement.Interaction{{ID: "outside", StudentID: "s", CompletedAt: &end}}}
	r := Aggregate(s, Filter{Start: at, End: end, AsOf: end})
	if r.Completed != 0 || r.StudentsServed != 0 || r.Conversion.Denominator != 0 || r.ReadingCompletion.Denominator != 0 {
		t.Fatalf("boundary %+v", r)
	}
}
