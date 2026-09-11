package session

import (
	"context"
	"testing"
	"time"

	"school_district_reading/internal/metrics"
)

func TestLiveDeskSeedPopulatesAgendaAndOurWork(t *testing.T) {
	s := engagementService(t)
	if n := len(s.EngagementSnapshot().Interactions); n != 0 {
		t.Fatalf("tests must start empty, got %d interactions", n)
	}
	if err := s.SeedLiveDesk(context.Background()); err != nil {
		t.Fatal(err)
	}
	snap := s.EngagementSnapshot()
	if len(snap.Interactions) < 5 {
		t.Fatalf("interactions %d", len(snap.Interactions))
	}
	open, scheduled := 0, 0
	for _, in := range snap.Interactions {
		if in.CompletedAt == nil {
			open++
		}
	}
	for _, a := range snap.Appointments {
		if a.Status == "scheduled" {
			scheduled++
		}
	}
	if open != 1 || scheduled < 2 {
		t.Fatalf("open=%d scheduled=%d", open, scheduled)
	}
	now := s.Now()
	loc, err := time.LoadLocation(s.EngagementTimezone())
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)
	report := metrics.Aggregate(snap, metrics.Filter{Start: start, End: end, AsOf: now})
	if report.StudentsServed < 4 || report.Completed < 4 || report.LinkedCheckouts < 3 || report.NoneToday != 1 {
		t.Fatalf("our work %+v", report)
	}
	if report.ReadingCompletion.Numerator < 2 || report.Enjoyment.Numerator < 2 {
		t.Fatalf("feedback %+v", report)
	}
}

func TestLiveDeskSeedDisabledForEmptySession(t *testing.T) {
	t.Setenv("SHELFMATE_EMPTY_SESSION", "1")
	s := engagementService(t)
	if err := s.SeedLiveDesk(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(s.EngagementSnapshot().Interactions); n != 0 {
		t.Fatalf("empty session seed wrote %d interactions", n)
	}
}

func TestLiveDeskSeedDoesNotTouchHistoricalDemo(t *testing.T) {
	s := engagementService(t)
	if err := s.SeedLiveDesk(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, in := range s.EngagementSnapshot().Interactions {
		if in.StartedAt.Year() < 2026 {
			t.Fatalf("historical-looking contact leaked into session: %+v", in)
		}
	}
}
