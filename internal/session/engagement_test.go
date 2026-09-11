package session

import (
	"context"
	"fmt"
	"school_district_reading/internal/engagement"
	"strings"
	"sync"
	"testing"
	"time"
)

func engagementService(t *testing.T) *Service {
	t.Helper()
	s := New(loadEngine(t))
	c, e := engagement.LoadCalendar("../../data/json")
	if e != nil {
		t.Fatal(e)
	}
	s.SetCalendar(c)
	s.SetClock(func() time.Time { return time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC) })
	return s
}
func command(t *testing.T, s *Service, c engagement.Command) EngagementResult {
	t.Helper()
	c.ExpectedRevision = s.EngagementSnapshot().Revision
	c.RequestID = fmt.Sprint(c.ExpectedRevision, "-", c.Action)
	r, e := s.EngagementCommand(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestEngagementAtomicCheckoutAndRetry(t *testing.T) {
	s := engagementService(t)
	before := s.Snapshot().Engine.Store.BookByID["B-007"].CopiesAvailable
	in := command(t, s, engagement.Command{Action: "start", StudentID: "S-406", StaffID: "L-001"})
	offer := command(t, s, engagement.Command{Action: "offer", InteractionID: in.ID})
	if offer.InventoryRevision != s.Snapshot().Revision {
		t.Fatal("offer missing inventory revision")
	}
	choice := command(t, s, engagement.Command{Action: "choose", InteractionID: in.ID, OfferID: offer.ID, Source: "offered", BookID: "B-007"})
	if s.Snapshot().Engine.Store.BookByID["B-007"].CopiesAvailable != before {
		t.Fatal("choice consumed inventory")
	}
	c := engagement.Command{Action: "checkout", ID: choice.ID, StaffID: "L-002", RequestID: "atomic", ExpectedRevision: s.EngagementSnapshot().Revision}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.EngagementCommand(context.Background(), c)
			if e != nil || r.Checkout == nil {
				t.Errorf("checkout %v %v", r, e)
			}
		}()
	}
	wg.Wait()
	snap := s.EngagementSnapshot()
	if len(snap.Choices) != 1 || snap.Choices[0].LoanID == "" || snap.Choices[0].CirculationStaff != "L-002" {
		t.Fatal("missing link")
	}
	if s.Snapshot().Engine.Store.BookByID["B-007"].CopiesAvailable != before-1 {
		t.Fatal("duplicate checkout")
	}
	if snap.Interactions[0].CompletedAt != nil {
		t.Fatal("checkout completed conversation")
	}
	c.StaffID = "L-003"
	if _, e := s.EngagementCommand(context.Background(), c); e == nil {
		t.Fatal("payload mismatch accepted")
	}
	command(t, s, engagement.Command{Action: "complete", InteractionID: in.ID, Start: "2026-09-17T09:00", Duration: 10, StaffID: "L-002", Confirmed: true})
	command(t, s, engagement.Command{Action: "feedback", InteractionID: in.ID, BookID: "B-007", StaffID: "L-002", Source: "student_reported", Reading: "finished", Enjoyment: "yes"})
	if s.EngagementSnapshot().Feedback[0].LoanID != snap.Choices[0].LoanID {
		t.Fatal("feedback unlinked")
	}
	// Returned snapshot must not allow caller mutation of service state.
	snap.Choices[0].BookID = "bad"
	if s.EngagementSnapshot().Choices[0].BookID != "B-007" {
		t.Fatal("snapshot alias")
	}
}
func TestEngagementSchedulingTransitions(t *testing.T) {
	s := engagementService(t)
	a := command(t, s, engagement.Command{Action: "schedule", StudentID: "S-406", StaffID: "L-001", Start: "2026-09-11T09:00", Duration: 10, Confirmed: true})
	c := engagement.Command{Action: "schedule", StudentID: "S-405", StaffID: "L-001", Start: "2026-09-11T09:05", Duration: 10, Confirmed: true, RequestID: "collision", ExpectedRevision: s.EngagementSnapshot().Revision}
	if _, e := s.EngagementCommand(context.Background(), c); e == nil {
		t.Fatal("collision accepted")
	}
	c.Start = "2026-09-11T09:10"
	command(t, s, c)
	command(t, s, engagement.Command{Action: "reschedule", ID: a.ID, Start: "2026-09-11T09:00", Duration: 10, Confirmed: true})
	command(t, s, engagement.Command{Action: "cancel", ID: a.ID})
	if _, e := s.EngagementCommand(context.Background(), engagement.Command{Action: "start", AppointmentID: a.ID, StaffID: "L-001", RequestID: "cancelled", ExpectedRevision: s.EngagementSnapshot().Revision}); e == nil {
		t.Fatal("started cancelled appointment")
	}
}
func TestConcurrentSlotBookingLeavesOneWinner(t *testing.T) {
	s := engagementService(t)
	c1 := engagement.Command{Action: "schedule", StudentID: "S-406", StaffID: "L-002", Start: "2026-09-11T09:00", Duration: 10, Confirmed: true, RequestID: "a", ExpectedRevision: 0}
	c2 := engagement.Command{Action: "schedule", StudentID: "S-405", StaffID: "L-002", Start: "2026-09-11T09:00", Duration: 10, Confirmed: true, RequestID: "b", ExpectedRevision: 0}
	var wg sync.WaitGroup
	errc := make(chan error, 2)
	for _, c := range []engagement.Command{c1, c2} {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.EngagementCommand(context.Background(), c)
			errc <- err
		}()
	}
	wg.Wait()
	close(errc)
	var ok, conflict int
	for err := range errc {
		if err == nil {
			ok++
			continue
		}
		if !strings.Contains(err.Error(), "conflict") {
			t.Fatalf("unexpected error %v", err)
		}
		conflict++
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("ok=%d conflict=%d", ok, conflict)
	}
	if n := len(s.EngagementSnapshot().Appointments); n != 1 {
		t.Fatalf("appointments %d", n)
	}
}
func TestEngagementInvalidChoiceDoesNotConsumeInventory(t *testing.T) {
	s := engagementService(t)
	in := command(t, s, engagement.Command{Action: "start", StudentID: "S-406", StaffID: "L-001"})
	for _, book := range []string{"B-008", "invented"} {
		_, e := s.EngagementCommand(context.Background(), engagement.Command{Action: "choose", InteractionID: in.ID, BookID: book, Source: "librarian", RequestID: book, ExpectedRevision: s.EngagementSnapshot().Revision})
		if e == nil {
			t.Fatal("accepted unavailable/unknown")
		}
	}
	if len(s.EngagementSnapshot().Choices) != 0 {
		t.Fatal("failed choice mutated state")
	}
}
func TestStartNowJumpsIntoBookedConversation(t *testing.T) {
	s := engagementService(t)
	open := command(t, s, engagement.Command{Action: "start", StudentID: "S-504", StaffID: "L-002"})
	if open.ID == "" {
		t.Fatal("tyler start")
	}
	booked := command(t, s, engagement.Command{Action: "schedule", StudentID: "S-401", StaffID: "L-002", Start: "2026-09-11T09:00", Duration: 10, Confirmed: true})
	jump := command(t, s, engagement.Command{Action: "start", AppointmentID: booked.ID, StaffID: "L-002"})
	if jump.ID == "" {
		t.Fatal("jordan early start")
	}
	snap := s.EngagementSnapshot()
	openCount := 0
	for _, in := range snap.Interactions {
		if in.CompletedAt == nil {
			openCount++
		}
	}
	if openCount != 2 {
		t.Fatalf("open conversations %d", openCount)
	}
	_, err := s.EngagementCommand(context.Background(), engagement.Command{Action: "start", StudentID: "S-401", StaffID: "L-001", RequestID: "dup", ExpectedRevision: snap.Revision})
	if err == nil || !strings.Contains(err.Error(), "already has an open conversation") {
		t.Fatalf("same student should stay locked, got %v", err)
	}
}
