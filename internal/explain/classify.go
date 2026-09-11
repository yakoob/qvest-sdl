package explain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ClassifyInput is the allowlisted classification payload. No first names,
// anecdotes, blurbs, or raw teacher/counselor prose.
type ClassifyInput struct {
	StudentID   string   `json:"student_id"`
	Cluster     string   `json:"cluster,omitempty"`
	PageComfort string   `json:"page_comfort,omitempty"`
	Themes      []string `json:"themes,omitempty"`
	Strengths   []string `json:"strengths,omitempty"`
	Allowed     []string `json:"allowed"`
}

type classifyOut struct {
	Themes []string `json:"themes"`
}

func (a AxonExplainer) Classify(ctx context.Context, in ClassifyInput) ([]string, error) {
	if strings.TrimSpace(a.Config.BaseURL) == "" {
		return nil, fmt.Errorf("empty LLM_BASE")
	}
	in = sanitizeClassify(in)
	if len(in.Allowed) == 0 {
		return nil, fmt.Errorf("empty theme allowlist")
	}
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	if len(body) > maxPromptBytes {
		return nil, fmt.Errorf("payload too large")
	}
	sys := strings.Join([]string{
		"You map labeled reading interests to allowlisted catalog themes.",
		"Return JSON only: {\"themes\":[\"<theme>\",...]}.",
		"Every theme must be one of the allowed values. At most three themes.",
		"Do not invent themes. Do not use student names or diagnoses.",
	}, " ")
	content, err := Complete(ctx, a.Config, sys, string(body), 200)
	if err != nil {
		return nil, err
	}
	return parseThemes([]byte(content), in.Allowed)
}

func sanitizeClassify(in ClassifyInput) ClassifyInput {
	allowed := map[string]struct{}{}
	cleanAllowed := make([]string, 0, len(in.Allowed))
	seen := map[string]struct{}{}
	for _, name := range in.Allowed {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		allowed[name] = struct{}{}
		cleanAllowed = append(cleanAllowed, name)
	}
	themes := make([]string, 0, len(in.Themes))
	seen = map[string]struct{}{}
	for _, theme := range in.Themes {
		theme = strings.TrimSpace(strings.ToLower(theme))
		if _, ok := allowed[theme]; !ok {
			continue
		}
		if _, ok := seen[theme]; ok {
			continue
		}
		seen[theme] = struct{}{}
		themes = append(themes, theme)
	}
	strengths := make([]string, 0, len(in.Strengths))
	for _, s := range in.Strengths {
		s = strings.TrimSpace(s)
		if s == "" || runeLen(s) > 80 {
			continue
		}
		low := strings.ToLower(s)
		if strings.Contains(low, "note") || strings.Contains(low, "secret") {
			continue
		}
		strengths = append(strengths, s)
		if len(strengths) >= 6 {
			break
		}
	}
	return ClassifyInput{
		StudentID:   strings.TrimSpace(in.StudentID),
		Cluster:     strings.TrimSpace(in.Cluster),
		PageComfort: strings.TrimSpace(in.PageComfort),
		Themes:      themes,
		Strengths:   strengths,
		Allowed:     cleanAllowed,
	}
}

func parseThemes(raw []byte, allowed []string) ([]string, error) {
	content := assistantText(raw)
	if content == "" {
		return nil, fmt.Errorf("empty model text")
	}
	var out classifyOut
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("malformed themes")
	}
	allow := map[string]struct{}{}
	for _, name := range allowed {
		allow[name] = struct{}{}
	}
	seen := map[string]struct{}{}
	clean := make([]string, 0, 3)
	for _, theme := range out.Themes {
		theme = strings.TrimSpace(strings.ToLower(theme))
		if _, ok := allow[theme]; !ok {
			continue
		}
		if _, ok := seen[theme]; ok {
			continue
		}
		seen[theme] = struct{}{}
		clean = append(clean, theme)
		if len(clean) >= 3 {
			break
		}
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("no grounded themes")
	}
	return clean, nil
}
