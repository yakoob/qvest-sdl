package support

import (
	"strings"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/domain"
)

// ClassifiedInterests turns labeled grade status and approved themes into
// allowlisted catalog terms. Raw teacher/counselor prose is never returned.
func ClassifiedInterests(st domain.Student, rec academics.Record, guidance Record) (query string, themes []string, below bool) {
	result := Evaluate(rec, guidance, DefaultConfig())
	below = result.GradeStatus == BelowGrade || st.ReadingBand == "below"
	if !below {
		return "", nil, false
	}
	seen := map[string]bool{}
	for _, g := range guidance.Guidance {
		for _, theme := range g.Themes {
			if _, ok := ThemeTerms(theme); ok && !seen[theme] {
				seen[theme] = true
				themes = append(themes, theme)
			}
		}
	}
	if clusterTheme, ok := clusterTheme(st.Cluster); ok && !seen[clusterTheme] {
		seen[clusterTheme] = true
		themes = append(themes, clusterTheme)
	}
	return QueryFromThemes(st, themes), themes, true
}

func QueryFromThemes(st domain.Student, themes []string) string {
	parts := make([]string, 0, len(themes)+1)
	seen := map[string]bool{}
	for _, theme := range themes {
		theme = strings.TrimSpace(theme)
		if seen[theme] {
			continue
		}
		terms, ok := ThemeTerms(theme)
		if !ok {
			continue
		}
		seen[theme] = true
		parts = append(parts, terms)
	}
	if st.PageComfort == "short" {
		parts = append(parts, "short")
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func clusterTheme(cluster string) (string, bool) {
	switch cluster {
	case "graphic":
		return "creativity", true
	case "sports":
		return "sports", true
	case "animals":
		return "animals", true
	case "fantasy", "mystery", "realistic":
		return "belonging", true
	default:
		return "", false
	}
}
