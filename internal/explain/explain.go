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

// whyText translates retrieval evidence into a plain why-this-story line.
// It restates only what retrieve already claimed for this book.
func whyText(r domain.ScoredBook) string {
	var parts []string
	for _, reason := range r.Reasons {
		switch reason {
		case "matches librarian query":
			parts = append(parts, "matches what you asked for")
		case "similar to recent checkouts":
			parts = append(parts, "reads like the books they already pick")
		case "borrowers with overlapping checkouts also took this":
			parts = append(parts, "students with similar checkouts took this one too")
		case "same cluster as a previous checkout":
			parts = append(parts, "same kind of story as ones they finished")
		case "same series as a previous checkout":
			parts = append(parts, "same series as one they already know")
		case "catalog text similar to checkout history":
			parts = append(parts, "catalog text echoes their history")
		case "grade-band popularity fallback (no checkout history or query match)":
			parts = append(parts, "popular at their grade level (no history to match yet)")
		default:
			parts = append(parts, reason)
		}
	}
	if len(parts) == 0 {
		return "catalog match"
	}
	seen := map[string]bool{}
	out := parts[:0]
	for _, p := range parts {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return strings.Join(out, "; ")
}

func templateLine(r domain.ScoredBook) string {
	series := ""
	if r.Book.Series != "" {
		series = " Series: " + r.Book.Series + " (same series is not a numbered next volume)."
	}
	blurb := strings.TrimSpace(r.Book.Blurb)
	if blurb != "" {
		return fmt.Sprintf("%s — %s. Why this story: %s. %d pages, %d copies on the shelf.%s",
			r.Book.Title, blurb, whyText(r), r.Book.Pages, r.Book.CopiesAvailable, series)
	}
	return fmt.Sprintf("%s — %s. %d pages, %d copies on the shelf.%s",
		r.Book.Title, whyText(r), r.Book.Pages, r.Book.CopiesAvailable, series)
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
