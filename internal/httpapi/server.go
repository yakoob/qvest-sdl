package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/engine"
)

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
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"students": len(s.Engine.Store.Students),
		"books":    len(s.Engine.Store.Books),
		"circ":     len(s.Engine.Store.Circulation),
		"llm":      false,
	})
}

func (s Server) students(w http.ResponseWriter, r *http.Request) {
	type row struct {
		StudentID   string `json:"student_id"`
		Grade       int    `json:"grade"`
		HomeroomID  string `json:"homeroom_id"`
		FirstName   string `json:"first_name"`
		LastInitial string `json:"last_initial"`
		Cluster     string `json:"cluster"`
		DemoRole    string `json:"demo_role"`
	}
	out := make([]row, 0, len(s.Engine.Store.Students))
	for _, st := range s.Engine.Store.Students {
		out = append(out, row{
			StudentID:   st.StudentID,
			Grade:       st.Grade,
			HomeroomID:  st.HomeroomID,
			FirstName:   st.FirstName,
			LastInitial: st.LastInitial,
			Cluster:     st.Cluster,
			DemoRole:    st.DemoRole,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s Server) student(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/students/")
	id = strings.TrimSuffix(id, "/history")
	st, ok := s.Engine.Store.Student(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	hist := s.Engine.Store.History[id]
	type ev struct {
		EventID      string `json:"event_id"`
		BookID       string `json:"book_id"`
		Title        string `json:"title"`
		CheckoutDate string `json:"checkout_date"`
		ReturnDate   string `json:"return_date"`
	}
	events := make([]ev, 0, len(hist))
	for _, e := range hist {
		title := e.BookID
		if b, ok := s.Engine.Store.Book(e.BookID); ok {
			title = b.Title
		}
		events = append(events, ev{
			EventID:      e.EventID,
			BookID:       e.BookID,
			Title:        title,
			CheckoutDate: e.CheckoutDate,
			ReturnDate:   e.ReturnDate,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"student": st,
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

func (s Server) recommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body recBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rec, err := s.Engine.Recommend(domain.Request{
		StudentID: body.StudentID,
		StaffID:   body.StaffID,
		Query:     body.Query,
		Stretch:   body.Stretch,
		Limit:     body.Limit,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
