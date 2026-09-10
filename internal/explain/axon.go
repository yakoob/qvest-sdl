package explain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"school_district_reading/internal/domain"
)

const (
	defaultTimeout   = 4 * time.Second
	maxResponseBytes = 64 << 10
	maxPromptBytes   = 12 << 10
)

// Config is the optional OpenAI-compatible (Axon) hook.
// LLM_BASE should be the origin plus any router prefix, without
// /v1/chat/completions — this client appends that path.
// Example: http://127.0.0.1:8791/router
// Credentials come from LLM_API_KEY (optional) and are never logged.
type Config struct {
	BaseURL    string
	Model      string
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

func ConfigFromEnv() Config {
	timeout := defaultTimeout
	if v := envLookup("SHELFMATE_LLM_TIMEOUT", ""); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			timeout = d
		}
	}
	return Config{
		BaseURL: envLookup("LLM_BASE", ""),
		Model:   envLookup("LLM_MODEL", "local"),
		APIKey:  envLookup("LLM_API_KEY", envLookup("OPENAI_API_KEY", "")),
		Timeout: timeout,
	}
}

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
	points, err := a.call(ctx, in)
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
	return Output{Mode: mode, Note: note, Points: filled}, nil
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

type chatRequest struct {
	Model       string        `json:"model"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Messages    []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type modelOut struct {
	TalkingPoints map[string]string `json:"talking_points"`
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
}

type modelPayload struct {
	StudentID  string        `json:"student_id"`
	Stretch    bool          `json:"stretch"`
	Under150   bool          `json:"under_150"`
	Short      bool          `json:"short"`
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
		Candidates: cands,
	}
}

func (a AxonExplainer) call(ctx context.Context, in Input) (map[string]string, error) {
	payload := BuildPayload(in)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(body) > maxPromptBytes {
		return nil, fmt.Errorf("payload too large")
	}
	sys := strings.Join([]string{
		"You write 20-second talking points for a school librarian.",
		"Use only the candidate books in the JSON. Never invent a title or book_id.",
		"Return JSON only: {\"talking_points\": {\"<book_id>\": \"<one or two sentences>\"}}.",
		"Do not use student names. Do not mention reading level, lexile, or diagnosis.",
		"Talking points are drafts; the librarian speaks, not you.",
	}, " ")
	reqBody, err := json.Marshal(chatRequest{
		Model:       a.Config.Model,
		Temperature: 0,
		MaxTokens:   400,
		Messages: []chatMessage{
			{Role: "system", Content: sys},
			{Role: "user", Content: string(body)},
		},
	})
	if err != nil {
		return nil, err
	}
	endpoint, err := chatCompletionsURL(a.Config.BaseURL)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, a.Config.Timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.Config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.Config.APIKey)
	}
	client := a.Config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: a.Config.Timeout}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxResponseBytes {
		return nil, fmt.Errorf("response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llm http %d", resp.StatusCode)
	}
	return parseTalkingPoints(raw, in)
}

func parseTalkingPoints(raw []byte, in Input) (map[string]string, error) {
	allowed := map[string]struct{}{}
	for _, r := range in.Items {
		allowed[r.Book.BookID] = struct{}{}
	}
	var cr chatResponse
	content := strings.TrimSpace(string(raw))
	if err := json.Unmarshal(raw, &cr); err == nil && len(cr.Choices) > 0 {
		content = strings.TrimSpace(cr.Choices[0].Message.Content)
	}
	content = stripFence(content)
	if !utf8.ValidString(content) {
		return nil, fmt.Errorf("invalid utf-8")
	}
	var out modelOut
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("malformed talking points")
	}
	if out.TalkingPoints == nil {
		return nil, fmt.Errorf("missing talking_points")
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
		return nil, fmt.Errorf("no grounded talking points")
	}
	return clean, nil
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func chatCompletionsURL(base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", errors.New("empty LLM_BASE")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid LLM_BASE")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid LLM_BASE scheme")
	}
	p := strings.TrimSuffix(u.Path, "/")
	switch {
	case strings.HasSuffix(p, "/v1/chat/completions"):
		// already complete
	case strings.HasSuffix(p, "/chat/completions"):
		// already complete
	case strings.HasSuffix(p, "/v1"):
		p += "/chat/completions"
	default:
		p += "/v1/chat/completions"
	}
	u.Path = p
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
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
