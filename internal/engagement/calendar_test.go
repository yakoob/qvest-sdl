package engagement

import (
	"testing"
	"time"
)

func TestCalendarDSTAndBlocks(t *testing.T) {
	c, e := LoadCalendar("../../data/json")
	if e != nil {
		t.Fatal(e)
	}
	for _, value := range []string{"2026-11-01T01:30", "2027-03-14T02:30", "2026-09-11T09:00:00Z", "2026-09-11T14:10:30-07:00", "2026-09-11T14:10:00.001-07:00"} {
		if _, e := c.ParseLocal(value); e == nil {
			t.Errorf("accepted %s", value)
		}
	}
	for _, value := range []string{"2026-11-01T01:30:00-07:00", "2026-11-01T01:30:00-08:00"} {
		if _, e := c.ParseLocal(value); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []struct {
		start, staff, room string
		valid              bool
	}{{"2026-09-11T09:00", "L-001", "H-4B", true}, {"2026-09-07T09:00", "L-001", "H-4B", false}, {"2026-09-18T13:00", "L-001", "H-4B", false}, {"2026-09-15T10:10", "L-002", "H-4B", false}, {"2026-09-11T14:25", "L-001", "H-4B", false}} {
		a, e := c.ParseLocal(v.start)
		if e != nil {
			t.Fatal(e)
		}
		e = c.Validate(v.staff, v.room, a, a.Add(10*time.Minute))
		if (e == nil) != v.valid {
			t.Errorf("%s: %v", v.start, e)
		}
	}
}
