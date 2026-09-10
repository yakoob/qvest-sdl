package policy

import (
	"strings"
	"unicode"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/store"
)

// ShortPageDefault is the looser "short" heuristic used when a librarian types
// "short" without an explicit page cap. It is not the same as "under 150 pages",
// which means strictly fewer than 150 pages.
const ShortPageDefault = 180

type Filter struct {
	Store *store.Store
}

// Apply enforces catalog membership and live availability from the store, not
// from candidate copies. Previously borrowed titles are excluded conservatively
// (any historical checkout, not only the last three).
func (f Filter) Apply(student domain.Student, req domain.Request, ranked []domain.ScoredBook) (keep []domain.ScoredBook, dropped []domain.Dropped) {
	already := seenIDs(f.Store.History[student.StudentID])
	c := ParseConstraints(req.Query)
	for _, row := range ranked {
		canon, ok := f.Store.Book(row.Book.BookID)
		if !ok {
			dropped = append(dropped, domain.Dropped{BookID: row.Book.BookID, Title: row.Book.Title, Why: "not in catalog"})
			continue
		}
		row.Book = canon
		if canon.CopiesAvailable <= 0 {
			dropped = append(dropped, domain.Dropped{BookID: canon.BookID, Title: canon.Title, Why: "copies_available=0"})
			continue
		}
		if !GradeOK(student.Grade, canon, req.Stretch) {
			dropped = append(dropped, domain.Dropped{BookID: canon.BookID, Title: canon.Title, Why: "outside grade band"})
			continue
		}
		if already[canon.BookID] {
			dropped = append(dropped, domain.Dropped{BookID: canon.BookID, Title: canon.Title, Why: "already checked out"})
			continue
		}
		if c.Under150 && canon.Pages >= 150 {
			dropped = append(dropped, domain.Dropped{BookID: canon.BookID, Title: canon.Title, Why: "query asked for under 150 pages"})
			continue
		}
		if c.Short && !c.Under150 && canon.Pages > ShortPageDefault {
			dropped = append(dropped, domain.Dropped{BookID: canon.BookID, Title: canon.Title, Why: "query asked for a short book"})
			continue
		}
		keep = append(keep, row)
	}
	return keep, dropped
}

// GradeOK is librarian-controlled eligibility. Stretch relaxes the lower bound
// by one grade (grade_min <= grade+1) and still requires grade_max >= grade.
// It is not an ability diagnosis and does not change ranking weights.
func GradeOK(grade int, b domain.Book, stretch bool) bool {
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

// Constraints is a local interpretation of the librarian query. It is never
// forwarded as raw text to a model.
type Constraints struct {
	Under150 bool
	Short    bool
}

func ParseConstraints(q string) Constraints {
	ql := normalizeQuery(q)
	c := Constraints{}
	if strings.Contains(ql, "under 150") || strings.Contains(ql, "under150") || strings.Contains(ql, "< 150") || strings.Contains(ql, "<150") {
		c.Under150 = true
	}
	if hasWord(ql, "short") {
		c.Short = true
	}
	return c
}

func normalizeQuery(q string) string {
	var b strings.Builder
	b.Grow(len(q))
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteByte(' ')
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func hasWord(q, word string) bool {
	for _, w := range strings.Fields(q) {
		if w == word {
			return true
		}
	}
	return false
}
