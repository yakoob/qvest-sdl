package retrieve

import (
	"regexp"
	"strings"
	"unicode"
)

var splitter = regexp.MustCompile(`[^a-z0-9]+`)

var stop = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "of": {}, "to": {}, "in": {},
	"on": {}, "for": {}, "with": {}, "is": {}, "his": {}, "her": {}, "who": {},
	"that": {}, "this": {}, "from": {}, "into": {}, "not": {}, "all": {},
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	parts := splitter.Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) < 2 {
			continue
		}
		if _, skip := stop[p]; skip {
			continue
		}
		if isAllDigits(p) && len(p) > 4 {
			continue
		}
		out = append(out, p)
	}
	return out
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return len(s) > 0
}

func bookDoc(title, author, blurb, genre, cluster, series string, subjects []string) string {
	parts := []string{title, author, blurb, genre, cluster, series}
	parts = append(parts, subjects...)
	return strings.Join(parts, " ")
}

func cosine(a, b map[string]float64) float64 {
	var dot, na, nb float64
	for k, va := range a {
		na += va * va
		if vb, ok := b[k]; ok {
			dot += va * vb
		}
	}
	for _, vb := range b {
		nb += vb * vb
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (sqrt(na) * sqrt(nb))
}

func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 12; i++ {
		x = 0.5 * (x + v/x)
	}
	return x
}

func minMax(vals []float64) []float64 {
	out := make([]float64, len(vals))
	if len(vals) == 0 {
		return out
	}
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	span := hi - lo
	for i, v := range vals {
		if span == 0 {
			out[i] = 0
			continue
		}
		out[i] = (v - lo) / span
	}
	return out
}
