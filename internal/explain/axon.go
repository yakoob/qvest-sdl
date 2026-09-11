package explain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"school_district_reading/internal/domain"
)

const (
	defaultTimeout   = 4 * time.Second
	maxResponseBytes = 64 << 10
	maxPromptBytes   = 12 << 10
	maxEnjoyPicks    = 3
)

type AxonExplainer struct {
	Fallback TemplateExplainer
	Config   Config
}

func NewAxon(fallback TemplateExplainer, cfg Config) AxonExplainer {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	return AxonExplainer{Fallback: fallback, Config: cfg}
}

func (a AxonExplainer) Explain(ctx context.Context, in Input) (Output, error) {
	fb, _ := a.Fallback.Explain(ctx, in)
	if strings.TrimSpace(a.Config.BaseURL) == "" {
		fb.Mode = domain.ExplainFallback
		fb.Note = draftLabel + ". LLM enabled but LLM_BASE is empty; using template."
		return fb, nil
	}
	points, enjoy, err := a.call(ctx, in)
	if err != nil {
		fb.Mode = domain.ExplainFallback
		fb.Note = draftLabel + ". Model unavailable (" + sanitizeErr(err) + "); template used. Ranking unchanged."
		return fb, nil
	}
	filled := fillMissing(in, points)
	mode := domain.ExplainLive
	note := draftLabel + ". Live model text; IDs checked against retrieve, not a factuality proof."
	if !sameKeys(filled, points, in) {
		mode = domain.ExplainFallback
		note = draftLabel + ". Partial model output; template filled missing or invalid items."
	}
	return Output{Mode: mode, Note: note, Points: filled, Enjoy: enjoy}, nil
}

func sameKeys(filled, raw map[string]string, in Input) bool {
	if len(raw) != len(in.Items) {
		return false
	}
	for _, r := range in.Items {
		p, ok := raw[r.Book.BookID]
		if !ok || strings.TrimSpace(p) == "" {
			return false
		}
		if filled[r.Book.BookID] != strings.TrimSpace(p) {
			return false
		}
	}
	return true
}

type modelOut struct {
	TalkingPoints map[string]string `json:"talking_points"`
	Enjoy         []string          `json:"enjoy"`
}

type payloadBook struct {
	BookID          string   `json:"book_id"`
	Title           string   `json:"title"`
	Author          string   `json:"author"`
	Cluster         string   `json:"cluster,omitempty"`
	Series          string   `json:"series,omitempty"`
	Pages           int      `json:"pages"`
	CopiesAvailable int      `json:"copies_available"`
	Reasons         []string `json:"reasons"`
	TalkingPoint    string   `json:"talking_point,omitempty"`
}

type modelPayload struct {
	StudentID  string        `json:"student_id"`
	Stretch    bool          `json:"stretch"`
	Under150   bool          `json:"under_150"`
	Short      bool          `json:"short"`
	Themes     []string      `json:"themes,omitempty"`
	Candidates []payloadBook `json:"candidates"`
}

func BuildPayload(in Input) modelPayload {
	cands := make([]payloadBook, 0, len(in.Items))
	for _, r := range in.Items {
		cands = append(cands, payloadBook{
			BookID:          r.Book.BookID,
			Title:           r.Book.Title,
			Author:          r.Book.Author,
			Cluster:         r.Book.Cluster,
			Series:          r.Book.Series,
			Pages:           r.Book.Pages,
			CopiesAvailable: r.Book.CopiesAvailable,
			Reasons:         append([]string(nil), r.Reasons...),
		})
	}
	return modelPayload{
		StudentID:  in.StudentID,
		Stretch:    in.Stretch,
		Under150:   in.Constraints.Under150,
		Short:      in.Constraints.Short,
		Themes:     append([]string(nil), in.Themes...),
		Candidates: cands,
	}
}

func (a AxonExplainer) call(ctx context.Context, in Input) (map[string]string, []string, error) {
	payload := BuildPayload(in)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	if len(body) > maxPromptBytes {
		return nil, nil, fmt.Errorf("payload too large")
	}
	sys := strings.Join([]string{
		"You write 20-second talking points for a school librarian.",
		"Use only the candidate books in the JSON. Never invent a title or book_id.",
		"Return JSON only: {\"talking_points\": {\"<book_id>\": \"<one or two sentences>\"}, \"enjoy\": [\"<book_id>\", ...]}",
		"enjoy is at most three candidate book_ids the student might enjoy. Omit any id not in candidates.",
		"Do not use student names. Do not mention reading level, lexile, or diagnosis.",
		"Talking points are drafts; the librarian speaks, not you.",
	}, " ")
	content, err := Complete(ctx, a.Config, sys, string(body), 400)
	if err != nil {
		return nil, nil, err
	}
	return parseTalkingPoints([]byte(content), in)
}

func parseTalkingPoints(raw []byte, in Input) (map[string]string, []string, error) {
	allowed := map[string]struct{}{}
	order := make([]string, 0, len(in.Items))
	for _, r := range in.Items {
		allowed[r.Book.BookID] = struct{}{}
		order = append(order, r.Book.BookID)
	}
	content := assistantText(raw)
	if content == "" {
		return nil, nil, fmt.Errorf("empty model text")
	}
	var out modelOut
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, nil, fmt.Errorf("malformed talking points")
	}
	if out.TalkingPoints == nil {
		return nil, nil, fmt.Errorf("missing talking_points")
	}
	clean := map[string]string{}
	for id, text := range out.TalkingPoints {
		if _, ok := allowed[id]; !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" || runeLen(text) > maxTalkingPointRunes {
			continue
		}
		clean[id] = text
	}
	if len(clean) == 0 {
		return nil, nil, fmt.Errorf("no grounded talking points")
	}
	return clean, groundEnjoy(out.Enjoy, allowed, order), nil
}

func groundEnjoy(ids []string, allowed map[string]struct{}, order []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, maxEnjoyPicks)
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if _, ok := allowed[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
		if len(out) >= maxEnjoyPicks {
			break
		}
	}
	_ = order
	return out
}

func sanitizeErr(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := err.Error()
	msg = strings.ReplaceAll(msg, "\n", " ")
	if len(msg) > 80 {
		msg = msg[:80]
	}
	return msg
}
