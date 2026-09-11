package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/engagement"
)

type EngagementResult struct {
	ID                string                 `json:"id"`
	Revision          int64                  `json:"revision"`
	Idempotent        bool                   `json:"idempotent"`
	InventoryRevision int64                  `json:"inventory_revision,omitempty"`
	Recommendation    *domain.Recommendation `json:"recommendation,omitempty"`
	Checkout          *CheckoutResult        `json:"checkout,omitempty"`
}
type engagementReceipt struct {
	Command engagement.Command
	Result  EngagementResult
}

func (s *Service) SetCalendar(c *engagement.Calendar) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calendar = c
}
func (s *Service) EngagementSnapshot() engagement.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	// JSON clone severs all slice and optional-time aliases exposed to readers.
	b, _ := json.Marshal(s.engagement)
	var out engagement.State
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Service) EngagementTimezone() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calendar == nil {
		return "UTC"
	}
	return s.calendar.Location.String()
}
func (s *Service) engagementRetryLocked(c engagement.Command) (EngagementResult, bool, error) {
	if c.RequestID == "" || len(c.RequestID) > 128 {
		return EngagementResult{}, false, fmt.Errorf("request_id required (max 128 characters)")
	}
	if v, ok := s.engagementRetry[c.RequestID]; ok {
		if v.Command != c {
			return EngagementResult{}, false, fmt.Errorf("request_id reused with different payload")
		}
		r := cloneEngagementResult(v.Result)
		r.Idempotent = true
		return r, true, nil
	}
	if c.ExpectedRevision != s.engagement.Revision {
		return EngagementResult{}, false, fmt.Errorf("revision conflict: refresh and try again")
	}
	return EngagementResult{}, false, nil
}
func (s *Service) interactionLocked(id string) *engagement.Interaction {
	for i := range s.engagement.Interactions {
		if s.engagement.Interactions[i].ID == id {
			return &s.engagement.Interactions[i]
		}
	}
	return nil
}
func (s *Service) appointmentLocked(id string) *engagement.Appointment {
	for i := range s.engagement.Appointments {
		if s.engagement.Appointments[i].ID == id {
			return &s.engagement.Appointments[i]
		}
	}
	return nil
}
func (s *Service) slotLocked(staff, student, start string, duration int, exclude string) (time.Time, time.Time, error) {
	if s.calendar == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("school calendar not loaded")
	}
	if duration < 5 || duration > 60 {
		return time.Time{}, time.Time{}, fmt.Errorf("duration must be 5–60 minutes")
	}
	a, e := s.calendar.ParseLocal(start)
	if e != nil {
		return a, a, e
	}
	b := a.Add(time.Duration(duration) * time.Minute)
	local := a.In(s.calendar.Location)
	if local.Minute()%5 != 0 || local.Hour() < 7 || local.Hour() >= 16 {
		return a, b, fmt.Errorf("choose a suggested five-minute slot")
	}
	st, ok := s.snap.Engine.Store.Student(student)
	if !ok {
		return a, b, ErrUnknownStudent
	}
	if _, ok := s.snap.Engine.Store.LibrarianByID[staff]; !ok {
		return a, b, ErrUnknownStaff
	}
	if a.Before(s.clock()) {
		return a, b, fmt.Errorf("appointment must be in the future")
	}
	if e = s.calendar.Validate(staff, st.HomeroomID, a, b); e != nil {
		return a, b, e
	}
	for _, v := range s.engagement.Appointments {
		if v.ID != exclude && (v.Status == "scheduled" || v.Status == "in_progress") && (v.StaffID == staff || v.StudentID == student) && engagement.Overlaps(a, b, v.Start, v.End) {
			return a, b, fmt.Errorf("appointment conflict for student or staff")
		}
	}
	return a, b, nil
}

// Availability suggests slots from shifts minus explicit blocks. Staff must confirm
// availability; duty does not establish free time and prose is not parsed as policy.
func (s *Service) Availability(student, staff, date string, duration int, exclude string) ([]engagement.Appointment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []engagement.Appointment{}
	if _, e := time.Parse("2006-01-02", date); e != nil {
		return nil, fmt.Errorf("invalid date")
	}
	if duration < 5 || duration > 60 {
		return nil, fmt.Errorf("invalid duration")
	}
	if s.calendar == nil {
		return nil, fmt.Errorf("school calendar not loaded")
	}
	if _, ok := s.snap.Engine.Store.Student(student); !ok {
		return nil, ErrUnknownStudent
	}
	if _, ok := s.snap.Engine.Store.LibrarianByID[staff]; !ok {
		return nil, ErrUnknownStaff
	}
	for m := 7 * 60; m < 16*60; m += 5 {
		value := fmt.Sprintf("%sT%02d:%02d", date, m/60, m%60)
		a, b, e := s.slotLocked(staff, student, value, duration, exclude)
		if e == nil {
			out = append(out, engagement.Appointment{StudentID: student, StaffID: staff, Start: a, End: b})
		}
	}
	return out, nil
}
func (s *Service) EngagementCommand(ctx context.Context, c engagement.Command) (EngagementResult, error) {
	s.mu.Lock()
	if r, ok, e := s.engagementRetryLocked(c); ok || e != nil {
		s.mu.Unlock()
		return r, e
	}
	var rec *domain.Recommendation
	var inventoryRevision int64
	if c.Action == "offer" {
		in := s.interactionLocked(c.InteractionID)
		if in == nil || in.CompletedAt != nil {
			s.mu.Unlock()
			return EngagementResult{}, fmt.Errorf("open interaction required")
		}
		if len(c.Query) > 500 {
			s.mu.Unlock()
			return EngagementResult{}, fmt.Errorf("query too long")
		}
		studentID, staffID, q, stretch := in.StudentID, in.Facilitator, c.Query, c.Stretch
		snap := s.snap
		s.mu.Unlock()
		q = s.classifiedQuery(studentID, q)
		req := domain.Request{StudentID: studentID, StaffID: staffID, Query: q, Stretch: stretch, Limit: 5}
		r, e := snap.Engine.RecommendContext(ctx, req)
		if e != nil {
			return EngagementResult{}, e
		}
		// No model call holds the writer lock. A stale response must not mutate state.
		s.mu.Lock()
		if out, ok, e := s.engagementRetryLocked(c); ok || e != nil {
			s.mu.Unlock()
			return out, e
		}
		if s.snap.Revision != snap.Revision {
			s.mu.Unlock()
			return EngagementResult{}, fmt.Errorf("inventory revision conflict: refresh recommendations")
		}
		r.Query = ""
		r.QueryParsed.Raw = ""
		rec = &r
		inventoryRevision = snap.Revision
	}
	defer s.mu.Unlock()
	now := s.clock().UTC()
	id := fmt.Sprintf("ENG-%06d", s.engagement.Revision+1)
	out := EngagementResult{ID: id}
	fail := func(msg string) (EngagementResult, error) { return EngagementResult{}, fmt.Errorf("%s", msg) }
	if c.Due != "" {
		return fail("choose an available follow-up slot with start, duration, staff_id and confirmation")
	}
	st := s.snap.Engine.Store
	switch c.Action {
	case "schedule", "reschedule":
		if !c.Confirmed {
			return fail("staff must confirm availability")
		}
		if len(c.Place) > 100 {
			return fail("place too long")
		}
		student, staff := c.StudentID, c.StaffID
		var existing *engagement.Appointment
		if c.Action == "reschedule" {
			existing = s.appointmentLocked(c.ID)
			if existing == nil || existing.Status != "scheduled" {
				return fail("scheduled appointment required")
			}
			student = existing.StudentID
			staff = existing.StaffID
		}
		exclude := ""
		if existing != nil {
			exclude = existing.ID
		}
		a, b, e := s.slotLocked(staff, student, c.Start, c.Duration, exclude)
		if e != nil {
			return EngagementResult{}, e
		}
		if existing != nil {
			existing.Start = a
			existing.End = b
			existing.Place = c.Place
			id = existing.ID
		} else {
			s.engagement.Appointments = append(s.engagement.Appointments, engagement.Appointment{ID: id, StudentID: student, StaffID: staff, Start: a, End: b, Place: c.Place, Status: "scheduled", CreatedAt: now})
		}
	case "cancel", "no_show":
		a := s.appointmentLocked(c.ID)
		if a == nil || a.Status != "scheduled" {
			return fail("scheduled appointment required")
		}
		if c.Action == "no_show" && now.Before(a.Start) {
			return fail("cannot mark future appointment no-show")
		}
		a.Status = "cancelled"
		if c.Action == "no_show" {
			a.Status = "no_show"
		}
		id = a.ID
	case "start":
		if _, ok := st.LibrarianByID[c.StaffID]; !ok {
			return EngagementResult{}, ErrUnknownStaff
		}
		student := c.StudentID
		var a *engagement.Appointment
		if c.AppointmentID != "" {
			a = s.appointmentLocked(c.AppointmentID)
			if a == nil || a.Status != "scheduled" {
				return fail("scheduled appointment required")
			}
			student = a.StudentID
			if now.Before(a.Start) {
				return fail("appointment has not started; reschedule or record a walk-in")
			}
		}
		if _, ok := st.Student(student); !ok {
			return EngagementResult{}, ErrUnknownStudent
		}
		for _, v := range s.engagement.Interactions {
			if v.CompletedAt == nil && (v.StudentID == student || v.Facilitator == c.StaffID) {
				return fail("student or facilitator already has an open conversation")
			}
		}
		if a != nil {
			a.Status = "in_progress"
		}
		s.engagement.Interactions = append(s.engagement.Interactions, engagement.Interaction{ID: id, StudentID: student, Facilitator: c.StaffID, AppointmentID: c.AppointmentID, StartedAt: now})
	case "complete":
		in := s.interactionLocked(c.InteractionID)
		if in == nil || in.CompletedAt != nil {
			return fail("open interaction required")
		}
		var booking *engagement.Appointment
		if c.Start != "" {
			if !c.Confirmed || len(c.Place) > 100 {
				return fail("confirm availability and use a short place label")
			}
			a, b, err := s.slotLocked(c.StaffID, in.StudentID, c.Start, c.Duration, "")
			if err != nil {
				return EngagementResult{}, err
			}
			booking = &engagement.Appointment{ID: "AP-" + id, StudentID: in.StudentID, StaffID: c.StaffID, Start: a, End: b, Place: c.Place, Status: "scheduled", CreatedAt: now}
		}
		in.CompletedAt = &now
		id = in.ID
		if a := s.appointmentLocked(in.AppointmentID); a != nil {
			a.Status = "completed"
		}
		for j := range s.engagement.Followups {
			f := &s.engagement.Followups[j]
			if f.AppointmentID != "" && f.AppointmentID == in.AppointmentID && f.CompletedAt == nil {
				f.CompletedAt = &now
				f.ContactID = in.ID
				f.StaffID = in.Facilitator
			}
		}
		if booking != nil {
			s.engagement.Appointments = append(s.engagement.Appointments, *booking)
			f := engagement.Followup{ID: "F-" + booking.ID, InteractionID: id, Due: booking.Start, CreatedAt: now, AppointmentID: booking.ID}
			f.Reservations = []engagement.Reservation{{At: now, AppointmentID: booking.ID, Due: booking.Start, Status: "scheduled"}}
			s.engagement.Followups = append(s.engagement.Followups, f)
		}
	case "offer":
		books := []string{}
		for _, b := range rec.Items {
			books = append(books, b.BookID)
		}
		s.engagement.Offers = append(s.engagement.Offers, engagement.Offer{ID: id, InteractionID: c.InteractionID, BookIDs: books, At: now, InventoryRevision: inventoryRevision, EvidenceVersion: rec.Version, Stretch: rec.Stretch, Under150: rec.QueryParsed.Under150, Short: rec.QueryParsed.Short})
		out.Recommendation = rec
		out.InventoryRevision = inventoryRevision
	case "choose":
		in := s.interactionLocked(c.InteractionID)
		if in == nil || in.CompletedAt != nil {
			return fail("open interaction required")
		}
		for _, v := range s.engagement.Choices {
			if v.InteractionID == in.ID {
				return fail("choice already recorded for this conversation")
			}
		}
		if c.Source == "none" {
			if c.BookID != "" {
				return fail("none today cannot include a book")
			}
		} else {
			b, ok := st.Book(c.BookID)
			if !ok {
				return EngagementResult{}, ErrUnknownBook
			}
			if b.CopiesAvailable <= 0 {
				return EngagementResult{}, ErrZeroCopies
			}
			if c.Source == "offered" {
				found := false
				for _, o := range s.engagement.Offers {
					if o.ID == c.OfferID && o.InteractionID == in.ID {
						for _, bid := range o.BookIDs {
							if bid == c.BookID {
								found = true
							}
						}
					}
				}
				if !found {
					return fail("book was not in the offered candidate set")
				}
			} else if c.Source != "librarian" {
				return fail("choice source must be offered, librarian or none")
			}
		}
		s.engagement.Choices = append(s.engagement.Choices, engagement.Choice{ID: id, InteractionID: in.ID, OfferID: c.OfferID, BookID: c.BookID, Source: c.Source, At: now})
	case "checkout":
		var choice *engagement.Choice
		for i := range s.engagement.Choices {
			if s.engagement.Choices[i].ID == c.ID {
				choice = &s.engagement.Choices[i]
			}
		}
		if choice == nil || choice.BookID == "" {
			return fail("accepted book choice required")
		}
		if choice.LoanID != "" {
			return fail("choice already checked out")
		}
		in := s.interactionLocked(choice.InteractionID)
		r, e := s.checkoutLocked(CheckoutRequest{StudentID: in.StudentID, BookID: choice.BookID, StaffID: c.StaffID})
		if e != nil {
			return EngagementResult{}, e
		}
		// No fallible work between successful checkout and link publication under mu.
		choice.LoanID = r.Loan.LoanID
		choice.CheckoutAt = &now
		choice.CirculationStaff = c.StaffID
		out.Checkout = &r
		id = choice.ID
	case "feedback":
		in := s.interactionLocked(c.InteractionID)
		if in == nil || in.CompletedAt == nil {
			return fail("completed conversation required")
		}
		if _, ok := st.LibrarianByID[c.StaffID]; !ok {
			return EngagementResult{}, ErrUnknownStaff
		}
		if c.Source != "student_reported" && c.Source != "staff_observed" {
			return fail("invalid feedback source")
		}
		if !oneOf(c.Reading, "finished", "reading", "not_started", "stopped", "unknown") || !oneOf(c.Enjoyment, "yes", "no", "neutral", "unknown") {
			return fail("invalid structured feedback")
		}
		loan := ""
		found := false
		for _, v := range s.engagement.Choices {
			if v.InteractionID == in.ID && v.BookID == c.BookID && v.BookID != "" {
				found = true
				loan = v.LoanID
			}
		}
		if !found {
			return fail("feedback book must be chosen in this conversation")
		}
		s.engagement.Feedback = append(s.engagement.Feedback, engagement.Feedback{ID: id, InteractionID: in.ID, BookID: c.BookID, LoanID: loan, Reading: c.Reading, Enjoyment: c.Enjoyment, Source: c.Source, StaffID: c.StaffID, At: now})
	case "book_followup":
		var f *engagement.Followup
		for j := range s.engagement.Followups {
			if s.engagement.Followups[j].ID == c.ID {
				f = &s.engagement.Followups[j]
				break
			}
		}
		if f == nil || f.CompletedAt != nil {
			return fail("open follow-up required")
		}
		if previous := s.appointmentLocked(f.AppointmentID); previous != nil && (previous.Status == "scheduled" || previous.Status == "in_progress") {
			return fail("reschedule the existing reservation")
		}
		if !c.Confirmed || len(c.Place) > 100 {
			return fail("confirm availability and use a short place label")
		}
		in := s.interactionLocked(f.InteractionID)
		a, b, err := s.slotLocked(c.StaffID, in.StudentID, c.Start, c.Duration, "")
		if err != nil {
			return EngagementResult{}, err
		}
		booking := engagement.Appointment{ID: "AP-" + id, StudentID: in.StudentID, StaffID: c.StaffID, Start: a, End: b, Place: c.Place, Status: "scheduled", CreatedAt: now}
		s.engagement.Appointments = append(s.engagement.Appointments, booking)
		f.AppointmentID = booking.ID
		f.Due = a
		f.Reservations = append(f.Reservations, engagement.Reservation{At: now, AppointmentID: booking.ID, Due: a, Status: "scheduled"})
		id = f.ID
	case "followup":
		return fail("book and complete a follow-up conversation to record contact")
	default:
		return fail("unknown engagement action")
	}
	if c.Action == "reschedule" || c.Action == "cancel" || c.Action == "no_show" {
		appointment := s.appointmentLocked(id)
		for j := range s.engagement.Followups {
			f := &s.engagement.Followups[j]
			if f.AppointmentID == id {
				f.Due = appointment.Start
				f.Reservations = append(f.Reservations, engagement.Reservation{At: now, AppointmentID: id, Due: appointment.Start, Status: appointment.Status})
			}
		}
	}
	s.engagement.Revision++
	out.ID = id
	out.Revision = s.engagement.Revision
	s.engagement.Events = append(s.engagement.Events, engagement.Event{ID: fmt.Sprintf("EV-%06d", out.Revision), Action: c.Action, EntityID: id, At: now, Revision: out.Revision})
	if s.engagementRetry == nil {
		s.engagementRetry = map[string]engagementReceipt{}
	}
	s.engagementRetry[c.RequestID] = engagementReceipt{Command: c, Result: cloneEngagementResult(out)}
	return out, nil
}
func cloneEngagementResult(in EngagementResult) EngagementResult {
	b, _ := json.Marshal(in)
	var out EngagementResult
	_ = json.Unmarshal(b, &out)
	return out
}
func oneOf(value string, values ...string) bool {
	for _, v := range values {
		if value == v {
			return true
		}
	}
	return false
}
