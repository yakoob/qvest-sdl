package explain

import (
	"fmt"
	"os"
	"strings"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/store"
)

// Explainer writes talking points. It must not introduce book IDs.
type Explainer interface {
	Explain(student domain.Student, recs []domain.ScoredBook) []string
}

func New(s *store.Store) Explainer {
	if os.Getenv("SHELFMATE_LLM") == "on" {
		return LLMExplainer{Store: s}
	}
	return TemplateExplainer{Store: s}
}

type TemplateExplainer struct {
	Store *store.Store
}

func (t TemplateExplainer) Explain(student domain.Student, recs []domain.ScoredBook) []string {
	var out []string
	hist := t.Store.History[student.StudentID]
	var titles []string
	seen := map[string]struct{}{}
	for i := len(hist) - 1; i >= 0 && len(titles) < 3; i-- {
		if _, ok := seen[hist[i].BookID]; ok {
			continue
		}
		seen[hist[i].BookID] = struct{}{}
		if b, ok := t.Store.Book(hist[i].BookID); ok {
			titles = append(titles, b.Title)
		}
	}
	histBit := "their checkout history"
	if len(titles) > 0 {
		histBit = strings.Join(titles, ", ")
	}
	for _, r := range recs {
		reason := "same kind of book"
		if len(r.Reasons) > 0 {
			reason = r.Reasons[0]
		}
		line := fmt.Sprintf("%s — funny enough to finish, %s. On the shelf (%d copies).", r.Book.Title, reason, r.Book.CopiesAvailable)
		if histBit != "their checkout history" {
			line = fmt.Sprintf("%s — because of %s. %s. %d copies on the shelf.", r.Book.Title, histBit, reason, r.Book.CopiesAvailable)
		}
		if student.PageComfort == "short" && r.Book.Pages <= 180 {
			line += " Short enough for the weekend."
		}
		out = append(out, line)
	}
	return out
}

// LLMExplainer is the hook for a grounded model. Skeleton refuses to call out
// until SHELFMATE_LLM=on and LLM_BASE is set. Even then it must only talk about
// retrieved IDs. Finish this in Claude Code.
type LLMExplainer struct {
	Store *store.Store
}

func (l LLMExplainer) Explain(student domain.Student, recs []domain.ScoredBook) []string {
	// TODO(claude-code): POST $LLM_BASE/v1/chat/completions with student_id only.
	// If the model emits a title not in recs, drop it and log ungrounded_title.
	fallback := TemplateExplainer{Store: l.Store}
	return fallback.Explain(student, recs)
}
