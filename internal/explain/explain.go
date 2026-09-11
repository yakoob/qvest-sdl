package explain

import (
	"context"
	"fmt"
	"strings"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/policy"
)

const (
	maxTalkingPointRunes = 400
	draftLabel           = "Librarian-reviewed draft"
)

// Explainer writes talking points for already-ranked titles. It must not
// introduce book IDs, change ranking, or receive a domain.Student.
type Explainer interface {
	Explain(ctx context.Context, in Input) (Output, error)
}

// Input is the allowlisted explainer payload. No first names, anecdotes,
// blurbs, or raw free-text query.
type Input struct {
	StudentID   string
	Stretch     bool
	Constraints policy.Constraints
	Themes      []string
	Items       []domain.ScoredBook
}

type Output struct {
	Mode   string
	Note   string
	Points map[string]string
	Enjoy  []string
}

func LLMEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(envOr("SHELFMATE_LLM", "off")))
	return v == "on" || v == "true" || v == "1"
}

func New() Explainer {
	tmpl := TemplateExplainer{}
	if !LLMEnabled() {
		return tmpl
	}
	return NewAxon(tmpl, ConfigFromEnv())
}

// TemplateExplainer is the default. It restates retrieve evidence and shelf
// facts. It does not invent humor, ability, or "next in series" claims.
type TemplateExplainer struct{}

func (TemplateExplainer) Explain(_ context.Context, in Input) (Output, error) {
	points := make(map[string]string, len(in.Items))
	for _, r := range in.Items {
		points[r.Book.BookID] = templateLine(r)
	}
	return Output{
		Mode:   domain.ExplainTemplate,
		Note:   draftLabel + ". Template text; no model call.",
		Points: points,
	}, nil
}

func templateLine(r domain.ScoredBook) string {
	reason := "catalog match"
	if len(r.Reasons) > 0 {
		reason = strings.Join(r.Reasons, "; ")
	}
	series := ""
	if r.Book.Series != "" {
		series = " Series: " + r.Book.Series + " (same series is not a numbered next volume)."
	}
	return fmt.Sprintf("%s — %s. %d pages, %d copies on the shelf.%s",
		r.Book.Title, reason, r.Book.Pages, r.Book.CopiesAvailable, series)
}

func fillMissing(in Input, points map[string]string) map[string]string {
	out := make(map[string]string, len(in.Items))
	for _, r := range in.Items {
		id := r.Book.BookID
		if p, ok := points[id]; ok {
			p = strings.TrimSpace(p)
			if p != "" && runeLen(p) <= maxTalkingPointRunes {
				out[id] = p
				continue
			}
		}
		out[id] = templateLine(r)
	}
	return out
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func envOr(key, fallback string) string {
	// thin wrapper so tests can stay in this package without os in every file
	return envLookup(key, fallback)
}
