package engagement

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"
)

type Window struct {
	StaffID    string `json:"staff_id"`
	HomeroomID string `json:"homeroom_id"`
	Weekday    string `json:"weekday"`
	Start      string `json:"start_local"`
	End        string `json:"end_local"`
	Active     bool   `json:"active"`
}
type Calendar struct {
	Location   *time.Location
	YearStart  string
	YearEnd    string
	Shifts     []Window
	Blocks     []Window
	Hours      map[string][2]string
	Exceptions map[string]string
}

func LoadCalendar(dir string) (*Calendar, error) {
	read := func(name string, v any) error {
		b, e := os.ReadFile(filepath.Join(dir, name+".json"))
		if e != nil {
			return e
		}
		return json.Unmarshal(b, v)
	}
	var district struct {
		YearStart string `json:"year_start"`
		YearEnd   string `json:"year_end"`
		ILS       struct {
			Timezone string `json:"timezone"`
		} `json:"ils"`
	}
	if e := read("district", &district); e != nil {
		return nil, e
	}
	loc, e := time.LoadLocation(district.ILS.Timezone)
	if e != nil || district.ILS.Timezone == "" {
		return nil, fmt.Errorf("invalid school timezone")
	}
	c := &Calendar{Location: loc, YearStart: district.YearStart, YearEnd: district.YearEnd, Hours: map[string][2]string{}, Exceptions: map[string]string{}}
	if e = read("desk_shifts", &c.Shifts); e != nil {
		return nil, e
	}
	for _, name := range []string{"class_visits", "open_circulation"} {
		var rows []Window
		if e = read(name, &rows); e != nil {
			return nil, e
		}
		c.Blocks = append(c.Blocks, rows...)
	}
	var clubs []Window
	if e = read("book_clubs", &clubs); e != nil {
		return nil, e
	}
	for _, w := range clubs {
		if w.Active {
			c.Blocks = append(c.Blocks, w)
		}
	}
	var hours []struct {
		Weekday string `json:"weekday"`
		Open    string `json:"open_local"`
		Close   string `json:"close_local"`
	}
	if e = read("library_hours", &hours); e != nil {
		return nil, e
	}
	for _, h := range hours {
		c.Hours[h.Weekday] = [2]string{h.Open, h.Close}
	}
	var exceptions []struct {
		Date   string `json:"date"`
		Status string `json:"status"`
	}
	if e = read("calendar_exceptions", &exceptions); e != nil {
		return nil, e
	}
	for _, x := range exceptions {
		c.Exceptions[x.Date] = x.Status
	}
	return c, nil
}

// ParseLocal rejects gaps and folds instead of silently choosing a DST offset.
// RFC3339 permits explicit disambiguation; its offset must match the school zone.
func (c *Calendar) ParseLocal(value string) (time.Time, error) {
	if t, e := time.Parse(time.RFC3339, value); e == nil {
		if t.Second() != 0 || t.Nanosecond() != 0 {
			return time.Time{}, fmt.Errorf("school-local time must use whole minutes")
		}
		_, a := t.Zone()
		_, b := t.In(c.Location).Zone()
		if a != b {
			return time.Time{}, fmt.Errorf("offset does not match school timezone")
		}
		return t.UTC(), nil
	}
	const layout = "2006-01-02T15:04"
	t, e := time.ParseInLocation(layout, value, c.Location)
	if e != nil || t.Format(layout) != value {
		return time.Time{}, fmt.Errorf("invalid or nonexistent school-local time")
	}
	for _, d := range []time.Duration{-time.Hour, time.Hour} {
		if t.Add(d).Format(layout) == value {
			return time.Time{}, fmt.Errorf("ambiguous local time: provide RFC3339 with explicit offset")
		}
	}
	return t.UTC(), nil
}
func Overlaps(a, b, c, d time.Time) bool { return a.Before(d) && c.Before(b) }
func (c *Calendar) Validate(staff, homeroom string, start, end time.Time) error {
	if c == nil {
		return fmt.Errorf("school calendar not loaded")
	}
	a, b := start.In(c.Location), end.In(c.Location)
	date := a.Format("2006-01-02")
	day := a.Weekday().String()
	if date < c.YearStart || date > c.YearEnd || date != b.Format("2006-01-02") || !start.Before(end) {
		return fmt.Errorf("outside school calendar")
	}
	h := c.Hours[day]
	if h[0] == "" || h[0] == "closed" || c.Exceptions[date] == "closed" {
		return fmt.Errorf("library closed")
	}
	close := h[1]
	if strings.HasPrefix(c.Exceptions[date], "early_close_") {
		close = strings.TrimPrefix(c.Exceptions[date], "early_close_")
	}
	clockA, clockB := a.Format("15:04"), b.Format("15:04")
	if clockA < h[0] || clockB > close {
		return fmt.Errorf("outside library hours")
	}
	onDuty := false
	for _, w := range c.Shifts {
		if w.StaffID == staff && w.Weekday == day && clockA >= w.Start && clockB <= w.End {
			onDuty = true
		}
	}
	if !onDuty {
		return fmt.Errorf("outside declared staff shift")
	}
	for _, w := range c.Blocks {
		if w.Weekday == day && (w.StaffID == staff || (homeroom != "" && w.HomeroomID == homeroom)) && clockA < w.End && w.Start < clockB {
			return fmt.Errorf("conflicts with class visit, circulation duty or club")
		}
	}
	return nil
}
