package retrieve

import (
	"math"
	"sort"
	"strings"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/store"
)

const (
	reasonQuery    = "matches librarian query"
	reasonHistory  = "similar to recent checkouts"
	reasonCF       = "borrowers with overlapping checkouts also took this"
	reasonCluster  = "same cluster as a previous checkout"
	reasonSeries   = "same series as a previous checkout"
	reasonFallback = "grade-band popularity fallback (no checkout history or query match)"
	reasonContent  = "catalog text similar to checkout history"
)

// Hybrid combines item-item cosine (circulation) with catalog TF-IDF.
// Ranking is not specialized per student_id.
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

func (h *Hybrid) tfidf(parts ...string) map[string]float64 {
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

func (h *Hybrid) uniqueBorrowers(bookID string) int {
	return len(h.item[bookID])
}

func (h *Hybrid) Recommend(student domain.Student, req domain.Request) []domain.ScoredBook {
	history := h.Store.History[student.StudentID]
	histIDs := uniqueHistoryIDs(history)
	histDocs := make([]string, 0, len(histIDs))
	histClusters := map[string]struct{}{}
	histSeries := map[string]struct{}{}
	for _, hid := range histIDs {
		b, ok := h.Store.Book(hid)
		if !ok {
			continue
		}
		histDocs = append(histDocs, bookDoc(b.Title, b.Author, b.Blurb, b.Genre, b.Cluster, b.Series, b.Subjects))
		if b.Cluster != "" && b.Cluster != "unknown" {
			histClusters[b.Cluster] = struct{}{}
		}
		if b.Series != "" {
			histSeries[b.Series] = struct{}{}
		}
	}

	queryVec := h.tfidf(req.Query)
	histVec := h.tfidf(histDocs...)
	hasQueryTokens := len(queryVec) > 0
	hasHistory := len(histIDs) > 0

	type raw struct {
		id        string
		cf        float64
		query     float64
		hist      float64
		bonus     float64
		borrowers int
		reasons   []string
	}

	rows := make([]raw, 0, len(h.Store.Books))
	anyQueryHit := false
	for _, b := range h.Store.Books {
		qScore := cosine(queryVec, h.docs[b.BookID])
		hScore := cosine(histVec, h.docs[b.BookID])
		if qScore > 0 {
			anyQueryHit = true
		}
		cf := 0.0
		for _, hid := range histIDs {
			cf += h.itemSim(b.BookID, hid)
		}
		bonus := 0.0
		var reasons []string
		if _, ok := histClusters[b.Cluster]; ok && b.Cluster != "" {
			bonus += 0.15
			reasons = append(reasons, reasonCluster)
		}
		if _, ok := histSeries[b.Series]; ok && b.Series != "" && !containsID(histIDs, b.BookID) {
			bonus += 0.2
			reasons = append(reasons, reasonSeries)
		}
		rows = append(rows, raw{
			id:        b.BookID,
			cf:        cf,
			query:     qScore,
			hist:      hScore,
			bonus:     bonus,
			borrowers: h.uniqueBorrowers(b.BookID),
			reasons:   reasons,
		})
	}

	useFallback := !hasHistory && !anyQueryHit

	cfWeight := 0.55
	coWeight := 0.45
	if len(histIDs) < 2 {
		cfWeight = 0
		coWeight = 1
	}
	if hasQueryTokens && !useFallback {
		cfWeight *= 0.7
		coWeight = 1 - cfWeight
	}

	contentScores := make([]float64, len(rows))
	cfScores := make([]float64, len(rows))
	for i, r := range rows {
		cfScores[i] = r.cf
		switch {
		case hasQueryTokens && hasHistory:
			contentScores[i] = 0.65*r.query + 0.35*r.hist
		case hasQueryTokens:
			contentScores[i] = r.query
		default:
			contentScores[i] = r.hist
		}
	}
	cfN := minMax(cfScores)
	coN := minMax(contentScores)

	out := make([]domain.ScoredBook, 0, len(rows))
	for i, r := range rows {
		b := h.Store.BookByID[r.id]
		reasons := append([]string(nil), r.reasons...)
		var score float64
		var cfOut, coOut float64
		if useFallback {
			score = float64(r.borrowers)
			cfOut = 0
			coOut = 0
			reasons = []string{reasonFallback}
		} else {
			cfOut = cfN[i]
			coOut = coN[i]
			score = cfWeight*cfOut + coWeight*coOut + r.bonus
			if r.query > 0 && hasQueryTokens {
				reasons = append(reasons, reasonQuery)
			}
			if r.hist > 0 && hasHistory {
				reasons = append(reasons, reasonHistory)
			}
			if cfWeight > 0 && r.cf > 0 {
				reasons = append(reasons, reasonCF)
			} else if r.hist > 0 && hasHistory && cfWeight == 0 {
				reasons = appendIfMissing(reasons, reasonContent)
			}
		}
		out = append(out, domain.ScoredBook{
			Book:    b,
			Score:   score,
			CF:      cfOut,
			Content: coOut,
			Bonus:   r.bonus,
			Reasons: uniqReasons(reasons),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			bi, bj := out[i].Book.BookID, out[j].Book.BookID
			if useFallback {
				pi, pj := h.uniqueBorrowers(bi), h.uniqueBorrowers(bj)
				if pi != pj {
					return pi > pj
				}
			}
			return bi < bj
		}
		return out[i].Score > out[j].Score
	})
	return out
}

// Search ranks catalog TF-IDF neighbors for a query. This is the in-process
// catalog index used for demo ranking/classification — not a separate database.
func (h *Hybrid) Search(query string, limit int) []domain.ScoredBook {
	if h == nil || h.Store == nil {
		return nil
	}
	if limit <= 0 {
		limit = 8
	}
	vec := h.tfidf(query)
	type hit struct {
		id    string
		score float64
	}
	hits := make([]hit, 0, len(h.Store.Books))
	for _, b := range h.Store.Books {
		s := cosine(vec, h.docs[b.BookID])
		if s <= 0 {
			continue
		}
		hits = append(hits, hit{b.BookID, s})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score == hits[j].score {
			return hits[i].id < hits[j].id
		}
		return hits[i].score > hits[j].score
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]domain.ScoredBook, 0, len(hits))
	for _, hit := range hits {
		b := h.Store.BookByID[hit.id]
		out = append(out, domain.ScoredBook{
			Book:    b,
			Score:   hit.score,
			Content: hit.score,
			Reasons: []string{reasonQuery},
		})
	}
	return out
}

func uniqueHistoryIDs(history []domain.CirculationEvent) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, ev := range history {
		if ev.BookID == "" {
			continue
		}
		if _, ok := seen[ev.BookID]; ok {
			continue
		}
		seen[ev.BookID] = struct{}{}
		ids = append(ids, ev.BookID)
	}
	return ids
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func appendIfMissing(in []string, s string) []string {
	for _, x := range in {
		if x == s {
			return in
		}
	}
	return append(in, s)
}

func uniqReasons(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, r := range in {
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	return out
}
