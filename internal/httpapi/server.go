package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/engine"
	"school_district_reading/internal/explain"
	"school_district_reading/internal/version"
)

const maxBodyBytes = 8 << 10

type Server struct {
	Engine *engine.Engine
	Web    http.Handler
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/students", s.students)
	mux.HandleFunc("/api/students/", s.student)
	mux.HandleFunc("/api/recommend", s.recommend)
	if s.Web != nil {
		mux.Handle("/", s.Web)
	}
	return mux
}

func (s Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"students": len(s.Engine.Store.Students),
		"books":    len(s.Engine.Store.Books),
		"circ":     len(s.Engine.Store.Circulation),
		"llm":      explain.LLMEnabled(),
		"version":  version.Version,
		"bind":     "localhost",
	})
}

type studentListRow struct {
	StudentID   string `json:"student_id"`
	Grade       int    `json:"grade"`
	HomeroomID  string `json:"homeroom_id"`
	FirstName   string `json:"first_name"`
	LastInitial string `json:"last_initial"`
	Cluster     string `json:"cluster"`
	DemoRole    string `json:"demo_role,omitempty"`
	Shortcut    bool   `json:"shortcut"`
}

func (s Server) students(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	out := make([]studentListRow, 0, len(s.Engine.Store.Students))
	for _, st := range s.Engine.Store.Students {
		out = append(out, studentListRow{
			StudentID:   st.StudentID,
			Grade:       st.Grade,
			HomeroomID:  st.HomeroomID,
			FirstName:   st.FirstName,
			LastInitial: st.LastInitial,
			Cluster:     st.Cluster,
			DemoRole:    st.DemoRole,
			Shortcut:    isShortcut(st.StudentID),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Shortcut != out[j].Shortcut {
			return out[i].Shortcut
		}
		return out[i].StudentID < out[j].StudentID
	})
	writeJSON(w, http.StatusOK, out)
}

type studentDetail struct {
	StudentID   string `json:"student_id"`
	Grade       int    `json:"grade"`
	HomeroomID  string `json:"homeroom_id"`
	FirstName   string `json:"first_name"`
	LastInitial string `json:"last_initial"`
	Cluster     string `json:"cluster"`
	ReadingBand string `json:"reading_band"`
	PageComfort string `json:"page_comfort"`
	DemoRole    string `json:"demo_role,omitempty"`
	Anecdote    string `json:"anecdote,omitempty"`
}

type historyEvent struct {
	EventID      string `json:"event_id"`
	BookID       string `json:"book_id"`
	Title        string `json:"title"`
	CheckoutDate string `json:"checkout_date"`
	ReturnDate   string `json:"return_date"`
}

func (s Server) student(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/students/")
	id = strings.TrimSuffix(id, "/history")
	id = strings.TrimSpace(id)
	st, ok := s.Engine.Store.Student(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown student"})
		return
	}
	hist := s.Engine.Store.History[id]
	events := make([]historyEvent, 0, len(hist))
	for i := len(hist) - 1; i >= 0; i-- {
		e := hist[i]
		title := e.BookID
		if b, ok := s.Engine.Store.Book(e.BookID); ok {
			title = b.Title
		}
		events = append(events, historyEvent{
			EventID:      e.EventID,
			BookID:       e.BookID,
			Title:        title,
			CheckoutDate: e.CheckoutDate,
			ReturnDate:   e.ReturnDate,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"student": studentDetail{
			StudentID:   st.StudentID,
			Grade:       st.Grade,
			HomeroomID:  st.HomeroomID,
			FirstName:   st.FirstName,
			LastInitial: st.LastInitial,
			Cluster:     st.Cluster,
			ReadingBand: st.ReadingBand,
			PageComfort: st.PageComfort,
			DemoRole:    st.DemoRole,
			Anecdote:    st.Anecdote,
		},
		"history": events,
	})
}

type recBody struct {
	StudentID string `json:"student_id"`
	StaffID   string `json:"staff_id"`
	Query     string `json:"query"`
	Stretch   bool   `json:"stretch"`
	Limit     int    `json:"limit"`
}

type recResponse struct {
	StudentID     string                     `json:"student_id"`
	StaffID       string                     `json:"staff_id"`
	Query         string                     `json:"query,omitempty"`
	QueryParsed   domain.QueryInterpretation `json:"query_parsed"`
	Stretch       bool                       `json:"stretch"`
	LLMEnabled    bool                       `json:"llm_enabled"`
	ExplainMode   string                     `json:"explain_mode"`
	ExplainNote   string                     `json:"explain_note,omitempty"`
	Version       string                     `json:"version,omitempty"`
	Items         []domain.RecItem           `json:"items"`
	Dropped       []domain.Dropped           `json:"dropped,omitempty"`
	TalkingPoints []string                   `json:"talking_points"`
	DraftLabel    string                     `json:"draft_label"`
}

func (s Server) recommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request too large"})
		return
	}
	if !utf8.Valid(raw) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid utf-8"})
		return
	}
	var body recBody
	if err := json.Unmarshal(raw, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	body.StudentID = strings.TrimSpace(body.StudentID)
	body.StaffID = strings.TrimSpace(body.StaffID)
	if body.StudentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "student_id required"})
		return
	}
	if _, ok := s.Engine.Store.Student(body.StudentID); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown student"})
		return
	}
	rec, err := s.Engine.RecommendContext(r.Context(), domain.Request{
		StudentID: body.StudentID,
		StaffID:   body.StaffID,
		Query:     body.Query,
		Stretch:   body.Stretch,
		Limit:     body.Limit,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, recResponse{
		StudentID:     rec.StudentID,
		StaffID:       rec.StaffID,
		Query:         rec.Query,
		QueryParsed:   rec.QueryParsed,
		Stretch:       rec.Stretch,
		LLMEnabled:    rec.LLMEnabled,
		ExplainMode:   rec.ExplainMode,
		ExplainNote:   rec.ExplainNote,
		Version:       rec.Version,
		Items:         rec.Items,
		Dropped:       rec.Dropped,
		TalkingPoints: rec.TalkingPoints,
		DraftLabel:    "Librarian-reviewed draft",
	})
}

func isShortcut(id string) bool {
	switch id {
	case "S-406", "S-402", "S-405", "S-509":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
