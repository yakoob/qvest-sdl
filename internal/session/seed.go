package session

import (
	"context"
	"fmt"
	"os"
	"time"

	"school_district_reading/internal/engagement"
)

// SeedLiveDesk preloads a small working morning so My day and Outcomes → Our work
// are not empty. It never writes historical paired evidence. Tests stay empty
// unless they call this helper; serve skips it when SHELFMATE_EMPTY_SESSION=1.
func (s *Service) SeedLiveDesk(ctx context.Context) error {
	if os.Getenv("SHELFMATE_EMPTY_SESSION") == "1" {
		return nil
	}
	if s == nil || s.calendar == nil {
		return fmt.Errorf("school calendar not loaded")
	}
	s.mu.Lock()
	if len(s.engagement.Interactions) > 0 || len(s.engagement.Appointments) > 0 {
		s.mu.Unlock()
		return nil
	}
	prev := s.clock
	s.mu.Unlock()
	defer s.SetClock(prev)

	now := prev()
	past := now.Add(-90 * time.Minute)

	visits := []struct {
		student, staff, book, reading, enjoyment string
	}{
		{"S-402", "L-002", "B-001", "finished", "yes"},
		{"S-405", "L-002", "B-005", "reading", "yes"},
		{"S-509", "L-002", "", "", ""},
		{"S-301", "L-001", "B-004", "finished", "neutral"},
	}
	for i, v := range visits {
		s.SetClock(func() time.Time { return past.Add(time.Duration(i) * 12 * time.Minute) })
		start, err := s.seedCommand(ctx, engagement.Command{Action: "start", StudentID: v.student, StaffID: v.staff})
		if err != nil {
			return fmt.Errorf("%s start: %w", v.student, err)
		}
		if v.book == "" {
			if _, err = s.seedCommand(ctx, engagement.Command{Action: "choose", InteractionID: start.ID, Source: "none"}); err != nil {
				return fmt.Errorf("%s none: %w", v.student, err)
			}
		} else {
			if _, err = s.seedCommand(ctx, engagement.Command{Action: "choose", InteractionID: start.ID, BookID: v.book, Source: "librarian"}); err != nil {
				return fmt.Errorf("%s choose: %w", v.student, err)
			}
			choice := s.choiceFor(start.ID)
			if choice == "" {
				return fmt.Errorf("%s missing choice", v.student)
			}
			if _, err = s.seedCommand(ctx, engagement.Command{Action: "checkout", ID: choice, StaffID: v.staff}); err != nil {
				return fmt.Errorf("%s checkout: %w", v.student, err)
			}
		}
		complete := engagement.Command{Action: "complete", InteractionID: start.ID}
		if i == 0 {
			slot, err := s.nextSlot(v.student, v.staff, now.Add(2*time.Hour), 10)
			if err != nil {
				return fmt.Errorf("aisha follow-up slot: %w", err)
			}
			complete.Start = slot
			complete.Duration = 10
			complete.StaffID = v.staff
			complete.Confirmed = true
			complete.Place = "Library"
		}
		if _, err = s.seedCommand(ctx, complete); err != nil {
			return fmt.Errorf("%s complete: %w", v.student, err)
		}
		if v.book != "" {
			if _, err = s.seedCommand(ctx, engagement.Command{Action: "feedback", InteractionID: start.ID, BookID: v.book, Reading: v.reading, Enjoyment: v.enjoyment, Source: "student_reported", StaffID: v.staff}); err != nil {
				return fmt.Errorf("%s feedback: %w", v.student, err)
			}
		}
	}

	s.SetClock(func() time.Time { return now.Add(-20 * time.Minute) })
	if _, err := s.seedCommand(ctx, engagement.Command{Action: "start", StudentID: "S-504", StaffID: "L-002"}); err != nil {
		return fmt.Errorf("tyler start: %w", err)
	}

	s.SetClock(prev)
	slot, err := s.nextSlot("S-401", "L-002", now.Add(time.Hour), 10)
	if err != nil {
		return fmt.Errorf("jordan slot: %w", err)
	}
	if _, err = s.seedCommand(ctx, engagement.Command{Action: "schedule", StudentID: "S-401", StaffID: "L-002", Start: slot, Duration: 10, Confirmed: true, Place: "Library"}); err != nil {
		return fmt.Errorf("jordan schedule: %w", err)
	}
	return nil
}

func (s *Service) seedCommand(ctx context.Context, c engagement.Command) (EngagementResult, error) {
	c.ExpectedRevision = s.EngagementSnapshot().Revision
	c.RequestID = fmt.Sprintf("seed-%d-%s", c.ExpectedRevision, c.Action)
	return s.EngagementCommand(ctx, c)
}

func (s *Service) choiceFor(interaction string) string {
	for _, c := range s.EngagementSnapshot().Choices {
		if c.InteractionID == interaction {
			return c.ID
		}
	}
	return ""
}

func (s *Service) nextSlot(student, staff string, after time.Time, duration int) (string, error) {
	loc := s.calendar.Location
	day := after.In(loc)
	startDay := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	for i := 0; i < 14; i++ {
		date := startDay.AddDate(0, 0, i).Format("2006-01-02")
		slots, err := s.Availability(student, staff, date, duration, "")
		if err != nil {
			continue
		}
		for _, slot := range slots {
			if !slot.Start.After(after) {
				continue
			}
			return slot.Start.In(loc).Format("2006-01-02T15:04"), nil
		}
	}
	return "", fmt.Errorf("no available slot after %s", after.Format(time.RFC3339))
}
