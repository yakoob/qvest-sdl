package engine

import (
	"fmt"
	"os"

	"school_district_reading/internal/audit"
	"school_district_reading/internal/domain"
	"school_district_reading/internal/explain"
	"school_district_reading/internal/policy"
	"school_district_reading/internal/retrieve"
	"school_district_reading/internal/store"
)

type Engine struct {
	Store     *store.Store
	Retrieve  *retrieve.Hybrid
	Policy    policy.Filter
	Explain   explain.Explainer
	AuditPath string
}

func New(s *store.Store) *Engine {
	return &Engine{
		Store:     s,
		Retrieve:  retrieve.New(s),
		Policy:    policy.Filter{Store: s},
		Explain:   explain.New(s),
		AuditPath: os.Getenv("SHELFMATE_AUDIT"),
	}
}

func (e *Engine) Recommend(req domain.Request) (domain.Recommendation, error) {
	if req.Limit <= 0 {
		req.Limit = 5
	}
	st, ok := e.Store.Student(req.StudentID)
	if !ok {
		return domain.Recommendation{}, fmt.Errorf("unknown student_id %s", req.StudentID)
	}
	ranked := e.Retrieve.Recommend(st, req)
	keep, dropped := e.Policy.Apply(st, req, ranked)
	if len(keep) > req.Limit {
		keep = keep[:req.Limit]
	}
	points := e.Explain.Explain(st, keep)
	rec := domain.Recommendation{
		StudentID:     st.StudentID,
		StaffID:       req.StaffID,
		Query:         req.Query,
		Stretch:       req.Stretch,
		LLM:           os.Getenv("SHELFMATE_LLM") == "on",
		Dropped:       firstDropped(dropped, 8),
		TalkingPoints: points,
	}
	for i, row := range keep {
		item := domain.RecItem{
			BookID:          row.Book.BookID,
			Title:           row.Book.Title,
			Author:          row.Book.Author,
			Cluster:         row.Book.Cluster,
			Series:          row.Book.Series,
			Pages:           row.Book.Pages,
			CopiesAvailable: row.Book.CopiesAvailable,
			Score:           row.Score,
			Reasons:         row.Reasons,
		}
		if i < len(points) {
			item.TalkingPoint = points[i]
		}
		rec.Items = append(rec.Items, item)
	}
	if e.AuditPath != "" {
		_ = audit.Append(e.AuditPath, rec)
	}
	return rec, nil
}

func firstDropped(in []domain.Dropped, n int) []domain.Dropped {
	if len(in) <= n {
		return in
	}
	return in[:n]
}
