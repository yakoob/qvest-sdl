package academics

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

var dateOnlyRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ParseDateOnly accepts a canonical ISO-8601 date-only string and returns
// that civil date at UTC midnight. It rejects times, timezones, and
// overflowing calendar dates (e.g. 2026-02-29).
func ParseDateOnly(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if !dateOnlyRe.MatchString(s) {
		return time.Time{}, fmt.Errorf("not ISO date-only: %q", s)
	}
	t, err := time.ParseInLocation(dateLayout, s, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	if t.Format(dateLayout) != s {
		return time.Time{}, fmt.Errorf("non-canonical date %q", s)
	}
	if t.Location() != time.UTC {
		return time.Time{}, fmt.Errorf("date %q is not UTC", s)
	}
	return t, nil
}

func FormatDateOnly(t time.Time) string {
	return t.In(time.UTC).Format(dateLayout)
}

func InclusiveDays(start, end time.Time) int {
	s := start.In(time.UTC).Truncate(24 * time.Hour)
	e := end.In(time.UTC).Truncate(24 * time.Hour)
	return int(e.Sub(s)/(24*time.Hour)) + 1
}

func InClosedRange(d, start, end time.Time) bool {
	ds := d.In(time.UTC).Truncate(24 * time.Hour)
	ss := start.In(time.UTC).Truncate(24 * time.Hour)
	ee := end.In(time.UTC).Truncate(24 * time.Hour)
	return !ds.Before(ss) && !ds.After(ee)
}

func RangesOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	return !aEnd.Before(bStart) && !bEnd.Before(aStart)
}

// TermBounds returns canonical Willow Bend semester bounds for an academic
// year like "2024-25". S1: 18 Aug Y .. 16 Jan Y+1. S2: 20 Jan Y+1 .. 5 Jun Y+1.
func TermBounds(academicYear, semester string) (time.Time, time.Time, error) {
	y, y2, err := parseAcademicYear(academicYear)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	switch strings.ToUpper(strings.TrimSpace(semester)) {
	case "S1":
		start := time.Date(y, time.August, 18, 0, 0, 0, 0, time.UTC)
		end := time.Date(y2, time.January, 16, 0, 0, 0, 0, time.UTC)
		return start, end, nil
	case "S2":
		start := time.Date(y2, time.January, 20, 0, 0, 0, 0, time.UTC)
		end := time.Date(y2, time.June, 5, 0, 0, 0, 0, time.UTC)
		return start, end, nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unknown semester %q", semester)
	}
}

func parseAcademicYear(s string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("academic year %q must look like 2024-25", s)
	}
	y, err := strconv.Atoi(parts[0])
	if err != nil || y < 2000 || y > 2100 {
		return 0, 0, fmt.Errorf("academic year start %q", parts[0])
	}
	tail := parts[1]
	if len(tail) == 2 {
		tail = parts[0][:2] + tail
	}
	y2, err := strconv.Atoi(tail)
	if err != nil || y2 != y+1 {
		return 0, 0, fmt.Errorf("academic year %q must span consecutive years", s)
	}
	return y, y2, nil
}

func ExpectedSchoolGrade(currentGrade int, currentAcademicYear, windowAcademicYear string) (int, error) {
	cy, _, err := parseAcademicYear(currentAcademicYear)
	if err != nil {
		return 0, err
	}
	wy, _, err := parseAcademicYear(windowAcademicYear)
	if err != nil {
		return 0, err
	}
	g := currentGrade - (cy - wy)
	if g < 0 || g > 12 {
		return 0, fmt.Errorf("implausible school grade %d for %s vs current %s grade %d", g, windowAcademicYear, currentAcademicYear, currentGrade)
	}
	return g, nil
}
