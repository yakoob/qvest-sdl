package retrieve

import (
	"math"
	"sort"
	"strings"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/store"
)

type Hybrid struct {
	Store *store.Store
	idf   map[string]float64
	docs  map[string]map[string]float64
	item  map[string]map[string]float64 // book -> student set as 1.0
}

func New(s *store.Store) *Hybrid {
	h := &Hybrid{Store: s}
	h.buildContent()
	h.buildCF()
	return h
}

func (h *Hybrid) buildContent() {
	df := map[string]int{}
	tf := map[string]map[string]int{}
	for _, b := range h.Store.Books {
		toks := tokenize(bookDoc(b.Title, b.Author, b.Blurb, b.Genre, b.Cluster, b.Series, b.Subjects))
		seen := map[string]struct{}{}
		tf[b.BookID] = map[string]int{}
		for _, t := range toks {
			tf[b.BookID][t]++
			if _, ok := seen[t]; !ok {
				df[t]++
				seen[t] = struct{}{}
			}
		}
	}
	n := float64(len(h.Store.Books))
	h.idf = map[string]float64{}
	for t, d := range df {
		h.idf[t] = math.Log((n+1)/float64(d+1)) + 1
	}
	h.docs = map[string]map[string]float64{}
	for id, counts := range tf {
		vec := map[string]float64{}
		for t, c := range counts {
			vec[t] = float64(c) * h.idf[t]
		}
		h.docs[id] = vec
	}
}

func (h *Hybrid) buildCF() {
	h.item = map[string]map[string]float64{}
	for sid, events := range h.Store.History {
		seen := map[string]struct{}{}
		for _, ev := range events {
			if _, ok := seen[ev.BookID]; ok {
				continue
			}
			seen[ev.BookID] = struct{}{}
			if h.item[ev.BookID] == nil {
				h.item[ev.BookID] = map[string]float64{}
			}
			h.item[ev.BookID][sid] = 1
		}
	}
}

func (h *Hybrid) itemSim(a, b string) float64 {
	ua, ok := h.item[a]
	if !ok {
		return 0
	}
	ub, ok := h.item[b]
	if !ok {
		return 0
	}
	var inter float64
	for sid := range ua {
		if _, ok := ub[sid]; ok {
			inter++
		}
	}
	if inter == 0 {
		return 0
	}
	return inter / math.Sqrt(float64(len(ua)*len(ub)))
}

func (h *Hybrid) queryVec(student domain.Student, history []domain.CirculationEvent, nl string) map[string]float64 {
	parts := []string{student.Cluster, student.PageComfort, nl}
	seen := map[string]struct{}{}
	for _, ev := range history {
		if _, dup := seen[ev.BookID]; dup {
			continue
		}
		seen[ev.BookID] = struct{}{}
		if b, ok := h.Store.Book(ev.BookID); ok {
			parts = append(parts, bookDoc(b.Title, b.Author, b.Blurb, b.Genre, b.Cluster, b.Series, b.Subjects))
		}
	}
	tf := map[string]int{}
	for _, t := range tokenize(strings.Join(parts, " ")) {
		tf[t]++
	}
	vec := map[string]float64{}
	for t, c := range tf {
		vec[t] = float64(c) * h.idf[t]
	}
	return vec
}

func (h *Hybrid) Recommend(student domain.Student, req domain.Request) []domain.ScoredBook {
	history := h.Store.History[student.StudentID]
	histSet := map[string]int{}
	for i := len(history) - 1; i >= 0; i-- {
		histSet[history[i].BookID]++
	}

	q := h.queryVec(student, history, req.Query)
	type raw struct {
		id      string
		cf      float64
		content float64
		bonus   float64
		reasons []string
	}
	var rows []raw
	for _, b := range h.Store.Books {
		cf := 0.0
		for hid := range histSet {
			cf += h.itemSim(b.BookID, hid)
		}
		content := cosine(q, h.docs[b.BookID])
		bonus := 0.0
		var reasons []string
		if student.Cluster != "" && student.Cluster != "unknown" && b.Cluster == student.Cluster {
			bonus += 0.15
			reasons = append(reasons, "same cluster as checkout history")
		}
		for hid := range histSet {
			hb, ok := h.Store.Book(hid)
			if ok && hb.Series != "" && hb.Series == b.Series && hid != b.BookID {
				bonus += 0.2
				reasons = append(reasons, "next in "+b.Series)
				break
			}
		}
		if req.Query != "" && content > 0 {
			reasons = append(reasons, "matches librarian query")
		}
		if cf > 0 {
			reasons = append(reasons, "kids with similar checkouts also took this")
		}
		rows = append(rows, raw{id: b.BookID, cf: cf, content: content, bonus: bonus, reasons: reasons})
	}

	cfN := make([]float64, len(rows))
	coN := make([]float64, len(rows))
	for i, r := range rows {
		cfN[i] = r.cf
		coN[i] = r.content
	}
	cfN = minMax(cfN)
	coN = minMax(coN)

	cfWeight := 0.55
	coWeight := 0.45
	if len(histSet) < 2 {
		cfWeight = 0
		coWeight = 1
	}
	if req.Query != "" {
		cfWeight *= 0.7
		coWeight = 1 - cfWeight
	}

	out := make([]domain.ScoredBook, 0, len(rows))
	for i, r := range rows {
		b := h.Store.BookByID[r.id]
		score := cfWeight*cfN[i] + coWeight*coN[i] + r.bonus
		out = append(out, domain.ScoredBook{
			Book:    b,
			Score:   score,
			CF:      cfN[i],
			Content: coN[i],
			Bonus:   r.bonus,
			Reasons: r.reasons,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Book.Title < out[j].Book.Title
		}
		return out[i].Score > out[j].Score
	})
	return out
}
