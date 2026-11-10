package main

// Snippet generation: best token window around query-term matches, terms
// wrapped in >>> <<<. Mirrors the spirit of codesearch.py's FTS snippet()
// (markers, ' ... ' separator, snip-token window) and its LIKE fallback
// (400-char window around the first literal occurrence).

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// snippetWindow computes the best window [winStart, winEnd) and the match
// segments within it, without rendering. Shared by the highlighted and
// plain (machine-output) snippet renderers.
func snippetWindow(text string, runs []qrun, snip int) (winStart, winEnd int, segs [][2]int) {
	spans := fineSpans(text)
	if len(spans) == 0 {
		return 0, len(text), nil
	}
	if snip <= 0 {
		snip = 40
	}
	regions := highlightRegions(spans, runs)
	bestI, bestJ := bestRegionWindow(spans, regions, snip)
	winStart, winEnd = spans[bestI].start, spans[bestJ-1].end
	// clip regions to the window
	for _, rg := range regions {
		s, e := rg[0], rg[1]
		if s < winStart {
			s = winStart
		}
		if e > winEnd {
			e = winEnd
		}
		if s < e {
			segs = append(segs, [2]int{s, e})
		}
	}
	return winStart, winEnd, segs
}

func renderSnippet(text string, winStart, winEnd int, segs [][2]int, mark bool) string {
	var b strings.Builder
	pos := winStart
	for _, sg := range segs {
		b.WriteString(text[pos:sg[0]])
		if mark {
			b.WriteString(">>>")
		}
		b.WriteString(text[sg[0]:sg[1]])
		if mark {
			b.WriteString("<<<")
		}
		pos = sg[1]
	}
	b.WriteString(text[pos:winEnd])
	return b.String()
}

// ftsSnippet renders the best window from the original content text.
// Phrase runs are wrapped as one merged span (>>>deep_target<<<);
// single AND terms are wrapped individually (>>>lorem<<< >>>ipsum<<<),
// mirroring FTS5's snippet().
func ftsSnippet(text string, runs []qrun, snip int) string {
	ws, we, segs := snippetWindow(text, runs, snip)
	return renderSnippet(text, ws, we, segs, true)
}

// plainSnippet is ftsSnippet without highlight markers, for machine output
// (--json). Same window selection, no >>> <<< pollution.
func plainSnippet(text string, runs []qrun, snip int) string {
	ws, we, segs := snippetWindow(text, runs, snip)
	return renderSnippet(text, ws, we, segs, false)
}

// camelBoundary reports whether a camelCase boundary sits before curr,
// given the previous rune and the next rune (if any).
func camelBoundary(prev, curr, next rune, hasNext bool) bool {
	if !unicode.IsUpper(curr) {
		return false
	}
	nextIsLower := hasNext && unicode.IsLower(next)
	return unicode.IsLower(prev) || unicode.IsDigit(prev) ||
		(unicode.IsUpper(prev) && nextIsLower)
}

// camelSubSpans splits a raw alphanumeric span at camelCase boundaries,
// returning sub-spans with absolute byte offsets.
func camelSubSpans(sp tokSpan) []tokSpan {
	rs := []rune(sp.orig)
	offs := make([]int, 0, len(rs)+1)
	o := sp.start
	for _, r := range rs {
		offs = append(offs, o)
		o += len(string(r))
	}
	offs = append(offs, o)

	var out []tokSpan
	var b strings.Builder
	start := 0
	flush := func(end int) {
		if b.Len() > 0 {
			out = append(out, tokSpan{
				tok:   b.String(),
				orig:  string(rs[start:end]),
				start: offs[start],
				end:   offs[end],
			})
			b.Reset()
		}
	}
	for i, r := range rs {
		if i > 0 {
			var next rune
			hasNext := i+1 < len(rs)
			if hasNext {
				next = rs[i+1]
			}
			if camelBoundary(rs[i-1], r, next, hasNext) {
				flush(i)
				start = i
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	flush(len(rs))
	return out
}

// fineSpans tokenizes text into camel-aware sub-spans: the same token
// sequence the search index uses (tokenize(splitCamel(text))), but with
// original byte offsets for snippet rendering.
func fineSpans(text string) []tokSpan {
	var out []tokSpan
	for _, sp := range tokenizeSpans(text) {
		out = append(out, camelSubSpans(sp)...)
	}
	return out
}

func highlightRegions(spans []tokSpan, runs []qrun) [][2]int {
	var regions [][2]int
	for _, r := range runs {
		if r.phrase {
			// consecutive occurrences -> one merged region each
			for i := 0; i+len(r.terms) <= len(spans); i++ {
				match := true
				for j, t := range r.terms {
					if spans[i+j].tok != t {
						match = false
						break
					}
				}
				if match {
					regions = append(regions, [2]int{spans[i].start, spans[i+len(r.terms)-1].end})
				}
			}
			continue
		}
		t := r.terms[0]
		for _, sp := range spans {
			if sp.tok == t {
				regions = append(regions, [2]int{sp.start, sp.end})
			}
		}
	}
	if len(regions) == 0 {
		return regions
	}
	sort.Slice(regions, func(i, j int) bool { return regions[i][0] < regions[j][0] })
	merged := [][2]int{regions[0]}
	for _, rg := range regions[1:] {
		last := &merged[len(merged)-1]
		if rg[0] <= last[1] {
			if rg[1] > last[1] {
				last[1] = rg[1]
			}
		} else {
			merged = append(merged, rg)
		}
	}
	return merged
}

// bestRegionWindow picks the snip-token window intersecting the most regions.
func bestRegionWindow(spans []tokSpan, regions [][2]int, snip int) (int, int) {
	bestI, bestJ, bestHits := 0, 0, -1
	for i := 0; i < len(spans); i++ {
		hits := 0
		j := i
		for ; j < len(spans) && j-i < snip; j++ {
			for _, rg := range regions {
				if rg[0] < spans[j].end && rg[1] > spans[j].start {
					hits++
					break
				}
			}
		}
		if hits > bestHits {
			bestHits, bestI, bestJ = hits, i, j
		}
	}
	return bestI, bestJ
}

// likeWindowRaw: 400-char window around the first literal occurrence,
// no affixes, no highlight markers.
func likeWindowRaw(text, rawQuery string) string {
	lower := strings.ToLower(text)
	anchor := strings.Index(lower, strings.ToLower(rawQuery))
	if anchor < 0 {
		anchor = 0
	}
	start := anchor - 200
	if start < 0 {
		start = 0
	}
	end := start + 400
	if end > len(text) {
		end = len(text)
	}
	// avoid splitting mid-rune
	for start > 0 && start < len(text) && !isUTF8Start(text[start]) {
		start--
	}
	for end > start && end < len(text) && !isUTF8Start(text[end]) {
		end--
	}
	return text[start:end]
}

// likeWindow is likeSnippet without highlight markers, for machine output
// (--json): "..." affixes around the raw window.
func likeWindow(text, rawQuery string) string {
	return "..." + likeWindowRaw(text, rawQuery) + "..."
}

// likeSnippet: 400-char window around the first literal occurrence,
// "..." affixes, query wrapped in >>> <<< (case-insensitive).
func likeSnippet(text, rawQuery string) string {
	win := likeWindowRaw(text, rawQuery)
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(rawQuery))
	win = re.ReplaceAllString(win, `>>>$0<<<`)
	return "..." + win + "..."
}

func isUTF8Start(b byte) bool { return b < 0x80 || b >= 0xC0 }
