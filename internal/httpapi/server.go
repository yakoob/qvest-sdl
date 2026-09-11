package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/domain"
	"school_district_reading/internal/engagement"
	"school_district_reading/internal/explain"
	"school_district_reading/internal/session"
	"school_district_reading/internal/support"
	"school_district_reading/internal/version"
)

const maxBodyBytes = 8 << 10

type Server struct {
	Demo      *engagement.Demo
	Session   *session.Service
	Academics *academics.Catalog
	Support   *support.Catalog
	Followups *support.Followups
	Web       http.Handler
}

func (s Server) Handler() http.Handler {
	if s.Followups == nil {
		s.Followups = &support.Followups{}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agenda", s.agenda)
	mux.HandleFunc("/api/availability", s.availability)
	mux.HandleFunc("/api/engagement", s.engagementCommand)
	mux.HandleFunc("/api/metrics/paired", s.pairedOutcomes)
	mux.HandleFunc("/api/metrics/progress", s.studentProgress)
	mux.HandleFunc("/api/metrics", s.engagementMetrics)
	mux.HandleFunc("/api/support/queue", s.supportQueue)
	mux.HandleFunc("/api/support/followups", s.followup)
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/students", s.students)
	mux.HandleFunc("/api/students/", s.studentRoutes)
	mux.HandleFunc("/api/recommend", s.recommend)
	mux.HandleFunc("/api/checkouts", s.checkouts)
	mux.HandleFunc("/api/returns", s.returns)
	mux.HandleFunc("/api/activity", s.activity)
	if s.Web != nil {
		mux.Handle("/", s.Web)
	}
	s.withTestDriver(mux)
	return withSameOrigin(mux)
}

func withSameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
			if !sameOrigin(r) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin requests are not accepted"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	host := r.Host
	if host == "" {
		return false
	}
	// Browsers send Origin like http://127.0.0.1:8088. Compare host:port only.
	trimmed := strings.TrimPrefix(origin, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	return strings.EqualFold(trimmed, host)
}

func (s Server) snap() session.Snapshot {
	if s.Session == nil {
		return session.Snapshot{}
	}
	return s.Session.Snapshot()
}

func (s Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	snap := s.snap()
	st := snap.Engine.Store
	acadLoaded := s.Academics != nil && !s.Academics.Missing
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"students":         len(st.Students),
		"books":            len(st.Books),
		"circ":             len(st.Circulation),
		"llm":              explain.LLMEnabled(),
		"version":          version.Version,
		"bind":             "localhost",
		"revision":         snap.Revision,
		"session_started":  snap.StartedAt.UTC().Format(time.RFC3339),
		"session_note":     "Demo checkout lives in this process only. Restart reloads the frozen extract.",
		"academics_loaded": acadLoaded,
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
	OpenLoans   int    `json:"open_loans"`
}

func (s Server) students(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	snap := s.snap()
	st := snap.Engine.Store
	out := make([]studentListRow, 0, len(st.Students))
	for _, stu := range st.Students {
		open := 0
		for _, ev := range st.History[stu.StudentID] {
			if strings.TrimSpace(ev.ReturnDate) == "" {
				open++
			}
		}
		out = append(out, studentListRow{
			StudentID:   stu.StudentID,
			Grade:       stu.Grade,
			HomeroomID:  stu.HomeroomID,
			FirstName:   stu.FirstName,
			LastInitial: stu.LastInitial,
			Cluster:     stu.Cluster,
			DemoRole:    stu.DemoRole,
			Shortcut:    isShortcut(stu.StudentID),
			OpenLoans:   open,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Shortcut != out[j].Shortcut {
			return out[i].Shortcut
		}
		return out[i].StudentID < out[j].StudentID
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"revision":        snap.Revision,
		"session_started": snap.StartedAt.UTC().Format(time.RFC3339),
		"session_note":    "Restart the server to reset demo checkouts.",
		"students":        out,
	})
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

func (s Server) studentRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/students/")
	rest = strings.Trim(rest, "/")
	if rest == "" {
		s.students(w, r)
		return
	}
	parts := strings.Split(rest, "/")
	id := parts[0]
	if len(parts) == 2 && parts[1] == "support" {
		s.supportDetail(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "academics" {
		s.academics(w, r, id)
		return
	}
	if len(parts) > 1 && parts[len(parts)-1] != "history" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown path"})
		return
	}
	s.studentDetail(w, r, id)
}

func (s Server) studentDetail(w http.ResponseWriter, r *http.Request, id string) {
	snap := s.snap()
	st := snap.Engine.Store
	stu, ok := st.Student(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown student"})
		return
	}
	open, hist := s.Session.LoansFor(id)
	if open == nil {
		open = []session.LoanView{}
	}
	if hist == nil {
		hist = []session.LoanView{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"student": studentDetail{
			StudentID:   stu.StudentID,
			Grade:       stu.Grade,
			HomeroomID:  stu.HomeroomID,
			FirstName:   stu.FirstName,
			LastInitial: stu.LastInitial,
			Cluster:     stu.Cluster,
			ReadingBand: stu.ReadingBand,
			PageComfort: stu.PageComfort,
			DemoRole:    stu.DemoRole,
			Anecdote:    stu.Anecdote,
		},
		"loans":           open,
		"history":         hist,
		"revision":        snap.Revision,
		"session_started": snap.StartedAt.UTC().Format(time.RFC3339),
		"session_note":    "Demo loans in this process. Restart restores the frozen extract.",
	})
}

func (s Server) academics(w http.ResponseWriter, r *http.Request, id string) {
	snap := s.snap()
	st := snap.Engine.Store
	if _, ok := st.Student(id); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown student"})
		return
	}
	cat := s.Academics
	if cat == nil {
		cat = &academics.Catalog{Missing: true, ByStudent: map[string]academics.Record{}}
	}
	view := cat.View(id, st)
	writeJSON(w, http.StatusOK, map[string]any{
		"revision":        snap.Revision,
		"session_started": snap.StartedAt.UTC().Format(time.RFC3339),
		"academics":       view,
	})
}

type recBody struct {
	Theme     string `json:"theme"`
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
	Enjoy         []string                   `json:"enjoy,omitempty"`
	DraftLabel    string                     `json:"draft_label"`
	Revision      int64                      `json:"revision"`
}

func (s Server) recommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	raw, ok := readJSONBody(w, r)
	if !ok {
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
	snap := s.snap()
	if _, ok := snap.Engine.Store.Student(body.StudentID); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown student"})
		return
	}
	if body.Theme != "" {
		terms, ok := support.ThemeTerms(body.Theme)
		if !ok {
			writeJSON(w, 400, map[string]string{"error": "unknown reading theme"})
			return
		}
		body.Query = strings.TrimSpace(body.Query + " " + terms)
	}
	rec, rev, err := s.Session.Recommend(r.Context(), domain.Request{
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
		Enjoy:         rec.Enjoy,
		DraftLabel:    "Librarian-reviewed draft",
		Revision:      rev,
	})
}

type checkoutBody struct {
	StudentID string `json:"student_id"`
	BookID    string `json:"book_id"`
	StaffID   string `json:"staff_id"`
	RetryID   string `json:"retry_id"`
}

type returnBody struct {
	StudentID string `json:"student_id"`
	LoanID    string `json:"loan_id"`
	StaffID   string `json:"staff_id"`
	RetryID   string `json:"retry_id"`
}

func (s Server) checkouts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	raw, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var body checkoutBody
	if err := json.Unmarshal(raw, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	out, err := s.Session.Checkout(session.CheckoutRequest{
		StudentID: body.StudentID,
		BookID:    body.BookID,
		StaffID:   body.StaffID,
		RetryID:   body.RetryID,
	})
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"loan":             out.Loan,
		"copies_available": out.CopiesAfter,
		"copies_total":     out.CopiesTotal,
		"revision":         out.Revision,
		"idempotent":       out.Idempotent,
		"activity":         out.Activity,
		"note":             "Checked out in this demo session. Restart restores the frozen extract. Borrowed is not finished.",
	})
}

func (s Server) returns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	raw, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var body returnBody
	if err := json.Unmarshal(raw, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	out, err := s.Session.Return(session.ReturnRequest{
		StudentID: body.StudentID,
		LoanID:    body.LoanID,
		StaffID:   body.StaffID,
		RetryID:   body.RetryID,
	})
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"loan":             out.Loan,
		"copies_available": out.CopiesAfter,
		"copies_total":     out.CopiesTotal,
		"revision":         out.Revision,
		"idempotent":       out.Idempotent,
		"activity":         out.Activity,
		"note":             "Returned in this demo session. The title stays in borrowing history (not a finished-reading claim).",
	})
}

func (s Server) activity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	snap := s.snap()
	acts := s.Session.Activity()
	if acts == nil {
		acts = []session.Activity{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"revision":        snap.Revision,
		"session_started": snap.StartedAt.UTC().Format(time.RFC3339),
		"session_note":    "Activity is this process only. Restart clears it.",
		"items":           acts,
	})
}

func writeSessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, session.ErrUnknownStudent), errors.Is(err, session.ErrUnknownBook), errors.Is(err, session.ErrUnknownStaff), errors.Is(err, session.ErrUnknownLoan):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, session.ErrDuplicateLoan), errors.Is(err, session.ErrZeroCopies), errors.Is(err, session.ErrAlreadyReturned), errors.Is(err, session.ErrInventoryConflict), errors.Is(err, session.ErrLoanStudentMismatch):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error(), "hint": "Refresh the student. Inventory or loan state changed in this session."})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
}

func readJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request too large"})
		return nil, false
	}
	if !utf8.Valid(raw) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid utf-8"})
		return nil, false
	}
	return raw, true
}

func isShortcut(id string) bool {
	switch id {
	case "S-406", "S-402", "S-405", "S-509", "S-305":
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
