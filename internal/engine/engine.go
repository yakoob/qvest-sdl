package engine

import (
	"context"
	"fmt"
	"time"

	"school_district_reading/internal/audit"
	"school_district_reading/internal/domain"
	"school_district_reading/internal/explain"
	"school_district_reading/internal/policy"
	"school_district_reading/internal/retrieve"
	"school_district_reading/internal/store"
	"school_district_reading/internal/version"
)

type Engine struct {
	Store    *store.Store
	Retrieve *retrieve.Hybrid
	Policy   policy.Filter
	Explain  explain.Explainer
	Audit    *audit.Log
	LLMOn    bool
	Version  string
}

func New(s *store.Store) *Engine {
	return &Engine{
		Store:    s,
		Retrieve: retrieve.New(s),
		Policy:   policy.Filter{Store: s},
		Explain:  explain.New(),
		Audit:    audit.FromEnv(),
		LLMOn:    explain.LLMEnabled(),
		Version:  version.Version,
	}
}

func (e *Engine) Recommend(req domain.Request) (domain.Recommendation, error) {
	return e.RecommendContext(context.Background(), req)
}

func (e *Engine) RecommendContext(ctx context.Context, req domain.Request) (domain.Recommendation, error) {
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.Limit > 20 {
		req.Limit = 20
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
	constraints := policy.ParseConstraints(req.Query)
	exp := e.Explain
	if exp == nil {
		exp = explain.TemplateExplainer{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	out, err := exp.Explain(ctx, explain.Input{
		StudentID:   st.StudentID,
		Stretch:     req.Stretch,
		Constraints: constraints,
		Themes:      append([]string(nil), req.Themes...),
		Items:       keep,
	})
	if err != nil {
		fb, _ := explain.TemplateExplainer{}.Explain(ctx, explain.Input{
			StudentID:   st.StudentID,
			Stretch:     req.Stretch,
			Constraints: constraints,
			Themes:      append([]string(nil), req.Themes...),
			Items:       keep,
		})
		out = fb
		out.Mode = domain.ExplainFallback
		out.Note = "Librarian-reviewed draft. Explainer error; template used. Ranking unchanged."
	}
	if out.Points == nil {
		out.Points = map[string]string{}
	}
	enjoySet := map[string]struct{}{}
	for _, id := range out.Enjoy {
		enjoySet[id] = struct{}{}
	}

	rec := domain.Recommendation{
		StudentID: st.StudentID,
		StaffID:   req.StaffID,
		Query:     req.Query,
		QueryParsed: domain.QueryInterpretation{
			Under150: constraints.Under150,
			Short:    constraints.Short,
			Raw:      req.Query,
		},
		Stretch:     req.Stretch,
		LLMEnabled:  e.LLMOn,
		ExplainMode: out.Mode,
		ExplainNote: out.Note,
		Version:     e.Version,
		Dropped:     firstDropped(dropped, 8),
		Enjoy:       append([]string(nil), out.Enjoy...),
	}
	points := make([]string, 0, len(keep))
	for _, row := range keep {
		tp := out.Points[row.Book.BookID]
		_, enjoy := enjoySet[row.Book.BookID]
		item := domain.RecItem{
			BookID:          row.Book.BookID,
			Title:           row.Book.Title,
			Author:          row.Book.Author,
			Cluster:         row.Book.Cluster,
			Series:          row.Book.Series,
			Pages:           row.Book.Pages,
			CopiesAvailable: row.Book.CopiesAvailable,
			Score:           row.Score,
			Reasons:         append([]string(nil), row.Reasons...),
			TalkingPoint:    tp,
			Enjoy:           enjoy,
		}
		rec.Items = append(rec.Items, item)
		if tp != "" {
			points = append(points, tp)
		}
	}
	rec.TalkingPoints = points
	if e.Audit != nil {
		if err := e.Audit.Append(audit.FromRecommendation(rec, time.Now().UTC())); err != nil {
			rec.AuditError = "audit write failed"
		}
	}
	return rec, nil
}

func firstDropped(in []domain.Dropped, n int) []domain.Dropped {
	if len(in) <= n {
		return in
	}
	return in[:n]
}
