package policy

import (
	"school_district_reading/internal/domain"
	"school_district_reading/internal/store"
)

type Filter struct {
	Store *store.Store
}

func (f Filter) Apply(student domain.Student, req domain.Request, ranked []domain.ScoredBook) (keep []domain.ScoredBook, dropped []domain.Dropped) {
	already := seenIDs(f.Store.History[student.StudentID])
	for _, row := range ranked {
		b := row.Book
		if b.CopiesAvailable <= 0 {
			dropped = append(dropped, domain.Dropped{BookID: b.BookID, Title: b.Title, Why: "copies_available=0"})
			continue
		}
		if !gradeOK(student.Grade, b, req.Stretch) {
			dropped = append(dropped, domain.Dropped{BookID: b.BookID, Title: b.Title, Why: "outside grade band"})
			continue
		}
		if already[b.BookID] {
			dropped = append(dropped, domain.Dropped{BookID: b.BookID, Title: b.Title, Why: "already checked out"})
			continue
		}
		if req.Query != "" && wantsShort(req.Query) && b.Pages > 160 {
			dropped = append(dropped, domain.Dropped{BookID: b.BookID, Title: b.Title, Why: "query asked for short / under 150 pages"})
			continue
		}
		keep = append(keep, row)
	}
	return keep, dropped
}

func gradeOK(grade int, b domain.Book, stretch bool) bool {
	if stretch {
		return b.GradeMin <= grade+1 && b.GradeMax >= grade
	}
	return b.GradeMin <= grade && b.GradeMax >= grade
}

func seenIDs(hist []domain.CirculationEvent) map[string]bool {
	out := map[string]bool{}
	for _, ev := range hist {
		out[ev.BookID] = true
	}
	return out
}

func wantsShort(q string) bool {
	ql := toLower(q)
	return contains(ql, "under 150") || contains(ql, "short") || contains(ql, "under 150 pages")
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
