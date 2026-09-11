package explain

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/policy"
)

func sampleInput() Input {
	return Input{
		StudentID:   "S-406",
		Stretch:     false,
		Constraints: policy.Constraints{Under150: true},
		Items: []domain.ScoredBook{
			{
				Book: domain.Book{
					BookID:          "B-007",
					Title:           "Cat Kid Comic Club",
					Author:          "Dav Pilkey",
					Cluster:         "graphic",
					Series:          "Cat Kid",
					Pages:           176,
					CopiesAvailable: 3,
					Blurb:           "Priya would like this invented blurb",
				},
				Score:   1.2,
				Reasons: []string{"same cluster as a previous checkout"},
			},
			{
				Book: domain.Book{
					BookID:          "B-006",
					Title:           "Investigators",
					Author:          "John Patrick Green",
					Cluster:         "graphic",
					Pages:           208,
					CopiesAvailable: 1,
				},
				Score:   1.1,
				Reasons: []string{"matches librarian query"},
			},
		},
	}
}

func TestTemplateHasNoFunnyEnoughClaim(t *testing.T) {
	out, err := TemplateExplainer{}.Explain(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Mode != domain.ExplainTemplate {
		t.Fatalf("mode %s", out.Mode)
	}
	for id, p := range out.Points {
		low := strings.ToLower(p)
		if strings.Contains(low, "funny enough to finish") {
			t.Fatalf("%s still claims funny enough: %s", id, p)
		}
		if strings.Contains(low, "next in") {
			t.Fatalf("%s claims next in series: %s", id, p)
		}
	}
}

func TestPayloadPrivacy(t *testing.T) {
	in := sampleInput()
	raw, err := json.Marshal(BuildPayload(in))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, banned := range []string{"Mateo", "Priya", "anecdote", "first_name", "blurb", "funny, reluctant", "Dog Man"} {
		if strings.Contains(s, banned) {
			t.Fatalf("payload contains %q: %s", banned, s)
		}
	}
	if !strings.Contains(s, `"student_id":"S-406"`) {
		t.Fatalf("missing student_id: %s", s)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"query", "anecdote", "first_name", "blurb"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("payload key %q not allowed: %s", key, s)
		}
	}
}

func TestAxonSuccess(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer secret") {
			t.Errorf("missing auth")
		}
		gotBody, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"talking_points":{"B-007":"Cat Kid is a graphic next step.","B-006":"Alligator detectives, on the shelf."}}`}},
			},
		})
	}))
	defer srv.Close()
	ex := NewAxon(TemplateExplainer{}, Config{BaseURL: srv.URL, Model: "test", APIKey: "secret", Timeout: time.Second, HTTPClient: srv.Client()})
	out, err := ex.Explain(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Mode != domain.ExplainLive {
		t.Fatalf("mode %s note %s", out.Mode, out.Note)
	}
	if out.Points["B-007"] != "Cat Kid is a graphic next step." {
		t.Fatalf("points %v", out.Points)
	}
	if strings.Contains(string(gotBody), "secret") && strings.Count(string(gotBody), "secret") > 0 {
		// API key must not be in JSON body
		var req chatRequest
		if err := json.Unmarshal(gotBody, &req); err == nil {
			enc, _ := json.Marshal(req)
			if strings.Contains(string(enc), "secret") {
				t.Fatal("api key leaked into body")
			}
		}
	}
}

func TestAxonInventedIDsDropped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"talking_points":{"B-999":"Invented","B-007":"Real one."}}`}},
			},
		})
	}))
	defer srv.Close()
	ex := NewAxon(TemplateExplainer{}, Config{BaseURL: srv.URL, Model: "test", Timeout: time.Second, HTTPClient: srv.Client()})
	out, err := ex.Explain(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Points["B-999"]; ok {
		t.Fatal("invented id survived")
	}
	if out.Points["B-007"] != "Real one." {
		t.Fatalf("B-007 %q", out.Points["B-007"])
	}
	if out.Points["B-006"] == "" || out.Points["B-006"] == "Invented" {
		t.Fatal("missing item should fall back to template")
	}
	if out.Mode != domain.ExplainFallback {
		t.Fatalf("partial fill should be fallback, got %s", out.Mode)
	}
}

func TestAxonMalformedFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not-json"}}]}`))
	}))
	defer srv.Close()
	ex := NewAxon(TemplateExplainer{}, Config{BaseURL: srv.URL, Timeout: time.Second, HTTPClient: srv.Client()})
	out, err := ex.Explain(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Mode != domain.ExplainFallback {
		t.Fatalf("mode %s", out.Mode)
	}
	if !strings.Contains(out.Points["B-007"], "Cat Kid Comic Club") {
		t.Fatalf("expected template, got %s", out.Points["B-007"])
	}
}

func TestAxonHTTPFailureFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()
	ex := NewAxon(TemplateExplainer{}, Config{BaseURL: srv.URL, Timeout: time.Second, HTTPClient: srv.Client()})
	out, err := ex.Explain(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Mode != domain.ExplainFallback {
		t.Fatalf("mode %s", out.Mode)
	}
}

func TestAxonTimeoutFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()
	client := srv.Client()
	client.Timeout = 30 * time.Millisecond
	ex := NewAxon(TemplateExplainer{}, Config{BaseURL: srv.URL, Timeout: 30 * time.Millisecond, HTTPClient: client})
	out, err := ex.Explain(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Mode != domain.ExplainFallback {
		t.Fatalf("mode %s", out.Mode)
	}
}

func TestChatCompletionsURL(t *testing.T) {
	u, err := chatCompletionsURL("http://127.0.0.1:8791/router")
	if err != nil {
		t.Fatal(err)
	}
	if u != "http://127.0.0.1:8791/router/v1/chat/completions" {
		t.Fatal(u)
	}
	u, err = chatCompletionsURL("http://127.0.0.1:8791/router/v1")
	if err != nil {
		t.Fatal(err)
	}
	if u != "http://127.0.0.1:8791/router/v1/chat/completions" {
		t.Fatal(u)
	}
}

func TestAxonMessagesPathAndEnjoyGrounding(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": `{"talking_points":{"B-007":"Graphic next step.","B-006":"On the shelf."},"enjoy":["B-007","B-999","B-006"]}`},
			},
		})
	}))
	defer srv.Close()
	ex := NewAxon(TemplateExplainer{}, Config{
		BaseURL:    srv.URL + "/control-plane/proxy",
		Model:      "auto:medium",
		Timeout:    time.Second,
		HTTPClient: srv.Client(),
	})
	out, err := ex.Explain(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/control-plane/proxy/v1/messages" {
		t.Fatalf("path %s", gotPath)
	}
	if out.Mode != domain.ExplainLive {
		t.Fatalf("mode %s note %s", out.Mode, out.Note)
	}
	if len(out.Enjoy) != 2 || out.Enjoy[0] != "B-007" || out.Enjoy[1] != "B-006" {
		t.Fatalf("enjoy %v", out.Enjoy)
	}
}

func TestClassifyAllowlistDropsUnknownThemes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"themes":["sports","secret-note","underdogs"]}`}},
			},
		})
	}))
	defer srv.Close()
	ex := NewAxon(TemplateExplainer{}, Config{BaseURL: srv.URL, Timeout: time.Second, HTTPClient: srv.Client()})
	got, err := ex.Classify(context.Background(), ClassifyInput{
		StudentID: "S-504",
		Allowed:   []string{"sports", "underdogs", "creativity"},
		Themes:    []string{"sports"},
		Strengths: []string{"Enjoys sports stories"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "sports" || got[1] != "underdogs" {
		t.Fatalf("themes %v", got)
	}
}

func TestClassifyPayloadOmitsProse(t *testing.T) {
	in := sanitizeClassify(ClassifyInput{
		StudentID: "S-504",
		Allowed:   []string{"sports"},
		Themes:    []string{"sports", "secret"},
		Strengths: []string{"secret classroom note", "teamwork"},
	})
	raw, _ := json.Marshal(in)
	s := string(raw)
	if strings.Contains(s, "secret") {
		t.Fatalf("prose leaked: %s", s)
	}
	if !strings.Contains(s, `"student_id":"S-504"`) {
		t.Fatalf("missing student_id: %s", s)
	}
}
