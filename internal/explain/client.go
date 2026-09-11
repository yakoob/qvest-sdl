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
)

// Config is the optional Axon hook. Claude Code uses the Anthropic Messages
// API at ANTHROPIC_BASE_URL (/control-plane/proxy). A router origin still
// uses OpenAI-compatible /v1/chat/completions via LLM_BASE.
type Config struct {
	BaseURL    string
	Model      string
	APIKey     string
	Protocol   string
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
	base := envLookup("LLM_BASE", envLookup("ANTHROPIC_BASE_URL", ""))
	model := envLookup("LLM_MODEL", envLookup("ANTHROPIC_DEFAULT_HAIKU_MODEL", "auto:medium"))
	if model == "" {
		model = "auto:medium"
	}
	key := envLookup("LLM_API_KEY", envLookup("OPENAI_API_KEY", envLookup("ANTHROPIC_AUTH_TOKEN", envLookup("ANTHROPIC_API_KEY", ""))))
	return Config{
		BaseURL:  base,
		Model:    model,
		APIKey:   key,
		Protocol: envLookup("LLM_PROTOCOL", ""),
		Timeout:  timeout,
	}
}

func (c Config) anthropic() bool {
	p := strings.ToLower(strings.TrimSpace(c.Protocol))
	if p == "anthropic" || p == "messages" {
		return true
	}
	if p == "openai" || p == "chat" {
		return false
	}
	b := strings.ToLower(c.BaseURL)
	return strings.Contains(b, "control-plane/proxy") || strings.Contains(b, "/v1/messages")
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

type messagesRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	System      string        `json:"system,omitempty"`
	Messages    []chatMessage `json:"messages"`
}

type messagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func Complete(ctx context.Context, cfg Config, system, user string, maxTokens int) (string, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return "", errors.New("empty LLM_BASE")
	}
	if maxTokens <= 0 {
		maxTokens = 400
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	var (
		endpoint string
		payload  []byte
		err      error
	)
	if cfg.anthropic() {
		endpoint, err = messagesURL(cfg.BaseURL)
		if err != nil {
			return "", err
		}
		payload, err = json.Marshal(messagesRequest{
			Model:       cfg.Model,
			MaxTokens:   maxTokens,
			Temperature: 0,
			System:      system,
			Messages:    []chatMessage{{Role: "user", Content: user}},
		})
	} else {
		endpoint, err = chatCompletionsURL(cfg.BaseURL)
		if err != nil {
			return "", err
		}
		payload, err = json.Marshal(chatRequest{
			Model:       cfg.Model,
			Temperature: 0,
			MaxTokens:   maxTokens,
			Messages: []chatMessage{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
		})
	}
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.anthropic() {
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	}
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		if cfg.anthropic() {
			httpReq.Header.Set("x-api-key", cfg.APIKey)
		}
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return "", err
	}
	if len(raw) > maxResponseBytes {
		return "", fmt.Errorf("response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm http %d", resp.StatusCode)
	}
	return assistantText(raw), nil
}

func assistantText(raw []byte) string {
	content := strings.TrimSpace(string(raw))
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err == nil && len(cr.Choices) > 0 {
		content = strings.TrimSpace(cr.Choices[0].Message.Content)
	}
	var mr messagesResponse
	if err := json.Unmarshal(raw, &mr); err == nil {
		var b strings.Builder
		for _, block := range mr.Content {
			if block.Type == "text" || block.Type == "" {
				b.WriteString(block.Text)
			}
		}
		if t := strings.TrimSpace(b.String()); t != "" {
			content = t
		}
	}
	content = stripFence(content)
	if !utf8.ValidString(content) {
		return ""
	}
	return content
}

func messagesURL(base string) (string, error) {
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
	case strings.HasSuffix(p, "/v1/messages"):
	case strings.HasSuffix(p, "/messages"):
	case strings.HasSuffix(p, "/v1"):
		p += "/messages"
	default:
		p += "/v1/messages"
	}
	u.Path = p
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
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
	case strings.HasSuffix(p, "/chat/completions"):
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
