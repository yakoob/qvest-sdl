package session

import (
	"context"
	"reflect"
	"testing"
	"time"

	"school_district_reading/internal/engagement"
)

func TestFollowupReservationLifecycle(t *testing.T) {
	s := engagementService(t)
	in := command(t, s, engagement.Command{Action: "start", StudentID: "S-406", StaffID: "L-001"})
	before := s.EngagementSnapshot()
	for _, c := range []engagement.Command{
		{Action: "complete", InteractionID: in.ID, Due: "2026-09-11T09:00"},
		{Action: "complete", InteractionID: in.ID, Start: "2026-09-11T09:01", Duration: 10, StaffID: "L-002", Confirmed: true},
		{Action: "complete", InteractionID: in.ID, Start: "2026-09-11T09:00", Duration: 10, StaffID: "L-002"},
	} {
		c.RequestID = "invalid"
		c.ExpectedRevision = before.Revision
		if _, err := s.EngagementCommand(context.Background(), c); err == nil {
			t.Fatal("invalid booking accepted")
		}
		if !reflect.DeepEqual(before, s.EngagementSnapshot()) {
			t.Fatal("failed booking changed state")
		}
	}
	command(t, s, engagement.Command{Action: "complete", InteractionID: in.ID, Start: "2026-09-11T09:00", Duration: 10, StaffID: "L-002", Confirmed: true})
	snap := s.EngagementSnapshot()
	f := snap.Followups[0]
	a := snap.Appointments[0]
	if f.AppointmentID != a.ID || f.CompletedAt != nil || snap.Interactions[0].CompletedAt == nil {
		t.Fatal("bad reservation linkage")
	}
	slots, err := s.Availability("S-405", "L-002", "2026-09-11", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range slots {
		if engagement.Overlaps(slot.Start, slot.End, a.Start, a.End) {
			t.Fatal("reserved slot offered")
		}
	}
	command(t, s, engagement.Command{Action: "cancel", ID: a.ID})
	if s.EngagementSnapshot().Followups[0].CompletedAt != nil {
		t.Fatal("cancellation counted as contact")
	}
	command(t, s, engagement.Command{Action: "book_followup", ID: f.ID, Start: "2026-09-11T09:10", Duration: 10, StaffID: "L-002", Confirmed: true})
	f = s.EngagementSnapshot().Followups[0]
	s.SetClock(func() time.Time { return time.Date(2026, 9, 11, 16, 10, 0, 0, time.UTC) })
	contact := command(t, s, engagement.Command{Action: "start", AppointmentID: f.AppointmentID, StaffID: "L-002"})
	command(t, s, engagement.Command{Action: "complete", InteractionID: contact.ID})
	f = s.EngagementSnapshot().Followups[0]
	if f.CompletedAt == nil || f.ContactID != contact.ID || f.StaffID != "L-002" || f.InteractionID != in.ID {
		t.Fatal("contact attribution/link lost")
	}
}
