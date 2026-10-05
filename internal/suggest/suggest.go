// Package suggest finds likely intended names for a mistyped one.
package suggest

import (
	"cmp"
	"slices"
	"strings"
)

// Max is the most suggestions Of returns.
const Max = 3

// Of returns the candidates that are close to s, closest first: those within an
// edit distance of 2 (but not more than half of s, so very short inputs do not
// match everything) and those s is a prefix of. Duplicates are removed.
func Of(s string, candidates []string) []string {
	if s == "" {
		return nil
	}

	type hit struct {
		name string
		dist int
	}
	hits := []hit{}
	for _, c := range candidates {
		if c == s || slices.ContainsFunc(hits, func(h hit) bool { return h.name == c }) {
			continue
		}

		d := distance(s, c)
		if d <= 2 && d*2 <= len(s) || strings.HasPrefix(c, s) {
			hits = append(hits, hit{c, d})
		}
	}

	slices.SortStableFunc(hits, func(a, b hit) int {
		return cmp.Compare(a.dist, b.dist)
	})
	vs := []string{}
	for _, h := range hits[:min(len(hits), Max)] {
		vs = append(vs, h.name)
	}
	return vs
}

// distance is the Levenshtein distance between a and b, counted in runes.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

// Hint renders suggestions as a parenthesized hint, e.g.
// ` (did you mean "deploy"?)`, or "" when there are none.
func Hint(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	qs := make([]string, len(vs))
	for i, v := range vs {
		qs[i] = `"` + v + `"`
	}
	return " (did you mean " + strings.Join(qs, " or ") + "?)"
}
