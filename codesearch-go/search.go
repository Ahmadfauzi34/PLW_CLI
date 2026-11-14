package main

// Ranking: hand-rolled BM25 over two fields (body tokens, path tokens),
// mirroring codesearch.py's FTS5 setup: path BM25-weighted 10x, terms AND-ed.
// Two baked-in boosts (no-stdlib-compromise scoring):
//   - exact-literal bonus x1.5: chunk contains the query string literally
//   - definition-pattern bonus x2.0: chunk defines one of the query terms
//     (fn|struct|enum|... <term>)

import (
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	pathWeight  = 10.0
	fetchMult   = 5
	bm25K1      = 1.2
	bm25B       = 0.75
	exactBonus  = 1.5
	defBonus    = 2.0
	testPenalty = 0.2 // TF penalty for test/generated files (Zoekt-style: 1/5)
	satDecay    = 0.5 // score decay for the 2nd chunk shown from one file
)

// isTestOrGenerated reports whether path looks like a test or
// machine-generated file: *_test.*, test_*, tests/ dirs, *.pb.go,
// *.gen.go, *.generated.*.
func isTestOrGenerated(p string) bool {
	lp := strings.ToLower(p)
	if strings.HasSuffix(lp, ".pb.go") || strings.HasSuffix(lp, ".gen.go") ||
		strings.Contains(lp, ".generated.") {
		return true
	}
	base := lp
	if i := strings.LastIndex(lp, "/"); i >= 0 {
		base = lp[i+1:]
	}
	if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, "_test.rs") || strings.HasSuffix(base, "_test.js") ||
		strings.HasSuffix(base, "_test.ts") || strings.HasPrefix(base, "test_") ||
		strings.HasPrefix(base, "conftest") {
		return true
	}
	return strings.Contains(lp, "/test/") || strings.Contains(lp, "/tests/") ||
		strings.HasPrefix(lp, "test/") || strings.HasPrefix(lp, "tests/")
}

// Boost toggles (evaluation ablation): CS_NO_EXACT_BOOST=1 / CS_NO_DEF_BOOST=1.
var (
	exactBoostOn = os.Getenv("CS_NO_EXACT_BOOST") == ""
	defBoostOn   = os.Getenv("CS_NO_DEF_BOOST") == ""
)

// tokenize splits on non-letter/non-number runs (FTS5 unicode61-like) and
// lowercases. No stemming, like the Python build.
func tokenize(s string) []string {
	var toks []string
	for _, sp := range tokenizeSpans(s) {
		toks = append(toks, sp.tok)
	}
	return toks
}

func isTokenRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

type tokSpan struct {
	tok   string // lowercased token
	orig  string // original text
	start int    // byte offset in text
	end   int
}

func tokenizeSpans(text string) []tokSpan {
	var out []tokSpan
	var b strings.Builder
	start := -1
	flush := func(end int) {
		if start >= 0 {
			out = append(out, tokSpan{b.String(), text[start:end], start, end})
			b.Reset()
			start = -1
		}
	}
	for i, r := range text {
		if isTokenRune(r) {
			if start < 0 {
				start = i
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			flush(i)
		}
	}
	flush(len(text))
	return out
}

type queryPlan struct {
	runs     []qrun // AND-ed runs; a run is a phrase when len(terms)>1
	pathOnly bool
	core     string // query core for the exact-literal bonus
	likeMode bool   // literal substring path (%, \, or zero terms)
	raw      string // original query (LIKE matching)
}

// qterm is one query term; joined reports whether the gap before it was a
// non-whitespace separator (FTS5 implicit-phrase rule: separator-joined
// query tokens form a phrase, whitespace-joined tokens are AND-ed).
type qterm struct {
	term   string
	joined bool
}

// qrun is a maximal run of separator-joined terms (a phrase), or a single
// AND-ed term.
type qrun struct {
	terms  []string
	phrase bool
}

// tokenizeQuery splits s into terms, tracking separator-joined gaps.
// It mirrors FTS5's query parsing: "lorem ipsum" -> AND(lorem, ipsum);
// "deep_target" -> PHRASE(deep, target).
func tokenizeQuery(s string) []qterm {
	var out []qterm
	var b strings.Builder
	joined := false // the next term follows a non-space separator
	flush := func() {
		if b.Len() > 0 {
			out = append(out, qterm{b.String(), joined})
			b.Reset()
		}
		joined = false
	}
	for _, r := range s {
		switch {
		case isTokenRune(r):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsSpace(r):
			flush() // whitespace gap: next term is not joined
		default:
			// non-space separator: ends the token and marks the next
			// term as separator-joined (implicit phrase)
			flush()
			joined = true
		}
	}
	flush()
	return out
}

func groupRuns(qts []qterm) []qrun {
	var runs []qrun
	for _, qt := range qts {
		if qt.joined && len(runs) > 0 {
			last := &runs[len(runs)-1]
			last.terms = append(last.terms, qt.term)
			last.phrase = true
		} else {
			runs = append(runs, qrun{terms: []string{qt.term}})
		}
	}
	return runs
}

func parseQuery(q string) queryPlan {
	p := queryPlan{raw: q}
	if strings.Contains(q, "%") || strings.Contains(q, "\\") {
		p.likeMode = true
		return p
	}
	if strings.Contains(q, "::") {
		// Qualified paths (foo::Bar) are intent-exact: route to literal
		// substring instead of BM25 phrase matching.
		p.likeMode = true
		return p
	}
	rest := q
	if strings.HasPrefix(rest, "path:") {
		p.pathOnly = true
		rest = rest[len("path:"):]
	}
	if len(rest) >= 2 && rest[0] == '"' && rest[len(rest)-1] == '"' {
		// explicit phrase: the whole core is one phrase run
		rest = rest[1 : len(rest)-1]
		terms := tokenize(splitCamel(rest))
		if len(terms) == 0 {
			p.likeMode = true
			return p
		}
		p.core = rest
		p.runs = []qrun{{terms: terms, phrase: len(terms) > 1}}
		return p
	}
	p.core = rest
	qts := tokenizeQuery(splitCamel(rest))
	if len(qts) == 0 {
		p.likeMode = true
		return p
	}
	p.runs = groupRuns(qts)
	return p
}

func bm25Term(idf float64, tf int, docLen int, avgLen float64) float64 {
	if avgLen == 0 {
		return 0
	}
	tfF := float64(tf)
	den := tfF + bm25K1*(1-bm25B+bm25B*float64(docLen)/avgLen)
	return idf * tfF * (bm25K1 + 1) / den
}

func idf(nDocs int, df int) float64 {
	return math.Log(1 + (float64(nDocs)-float64(df)+0.5)/(float64(df)+0.5))
}

type scoredChunk struct {
	doc   int
	score float64
}

// defPattern matches "<keyword> <term>" definitions across languages.
// NOTE: "package" is deliberately NOT a definition keyword. A Go/Java
// package declaration ("package main") names a module, it does not
// define an identifier of that name — so it must neither earn the
// definition ranking bonus nor pass the --defs-only filter. (Regression
// 2026-10-26: --defs-only "main" returned the "package main" window
// chunk as a second "definition".)
var defKeywords = []string{"fn", "struct", "enum", "trait", "impl", "class",
	"def", "function", "const", "type", "interface", "func", "var",
	// Kotlin (K1-K4, 2026-11-12): "fun" for functions, "object" for
	// object declarations, "typealias" for type aliases, "val" for
	// value bindings (mirrors "var"/"const": a local val inside a
	// function body matches the same way a local const does).
	"fun", "object", "typealias", "val"}

// hasDefPattern is a fast hand-rolled check for definition patterns
// ("keyword term" with optional Go receiver). It gates the expensive regex:
// most candidates do not define the term, so this quickly returns false.
func hasDefPattern(lowerText string, terms []string) bool {
	for _, term := range terms {
		if hasDefForTerm(lowerText, term) {
			return true
		}
	}
	return false
}

func isWordChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

func hasDefForTerm(text, term string) bool {
	// Also try the term split into parts (for snake_case/kebab-case);
	// the gate is permissive: any part matching after a keyword triggers
	// the full regex check.
	termsToCheck := []string{term}
	// Split on _ and - to get parts
	for _, sep := range []string{"_", "-"} {
		if strings.Contains(term, sep) {
			parts := strings.Split(term, sep)
			termsToCheck = append(termsToCheck, parts...)
			// Also the concatenated form
			termsToCheck = append(termsToCheck, strings.Join(parts, ""))
		}
	}
	for _, kw := range defKeywords {
		search := kw + " "
		for i := 0; i+len(search) <= len(text); {
			idx := strings.Index(text[i:], search)
			if idx < 0 {
				break
			}
			pos := i + idx
			if pos > 0 && isWordChar(text[pos-1]) {
				i = pos + 1
				continue
			}
			j := pos + len(search)
			for j < len(text) && isSpaceByte(text[j]) {
				j++
			}
			if j < len(text) && text[j] == '(' {
				k := strings.IndexByte(text[j:], ')')
				if k < 0 {
					i = pos + 1
					continue
				}
				j += k + 1
				for j < len(text) && isSpaceByte(text[j]) {
					j++
				}
			}
			// Extract the next word and normalize (remove _/-) for comparison.
			// This handles add_numbers vs addnumbers.
			k := j
			for k < len(text) && (isWordChar(text[k]) || text[k] == '-') {
				k++
			}
			word := text[j:k]
			normWord := strings.ReplaceAll(strings.ReplaceAll(word, "_", ""), "-", "")
			for _, t := range termsToCheck {
				normT := strings.ReplaceAll(strings.ReplaceAll(t, "_", ""), "-", "")
				if strings.HasPrefix(normWord, normT) {
					return true
				}
			}
			i = pos + 1
		}
	}
	return false
}

// compileDefRes builds definition-pattern regexes ("fn <name>", "def <name>",
// ...). One regex per query term covers single-word names; an additional
// joined-identifier regex covers multi-word names in any separator style:
// the query "addNumbers" (or "add_numbers") also boosts "func add_numbers",
// "def addNumbers", etc. Without the joined form the x2.0 definition bonus
// silently never fired for camelCase/snake_case names, letting call sites
// outrank definitions.
func compileDefRes(terms []string, core string) []*regexp.Regexp {
	var out []*regexp.Regexp
	kw := strings.Join(defKeywords, "|")
	// After the keyword, an optional Go receiver "(r *T)" may sit between
	// "func" and the name: func (ix *memIndex) searchBody(...).
	recv := `(?:\([^)]*\)\s+)?`
	add := func(pat string) {
		// G1 (2026-11-14): the terminator excludes "." — a dot after
		// the name is a receiver/qualifier (fun AppGraph.folderNames),
		// not a definition of the queried name.
		re, err := regexp.Compile(`(?i)(?:^|[^a-z0-9_])(?:` + kw + `)\s+` +
			recv + pat + `(?:[^a-z0-9_.]|$)`)
		if err == nil {
			out = append(out, re)
		}
	}
	for _, t := range terms {
		add(regexp.QuoteMeta(t))
	}
	if parts := tokenize(splitCamel(core)); len(parts) > 1 {
		var sb strings.Builder
		for i, p := range parts {
			if i > 0 {
				sb.WriteString(`[_-]?`)
			}
			sb.WriteString(regexp.QuoteMeta(p))
		}
		add(sb.String())
	}
	return out
}

// excludedDocs returns the set of live docs containing all tokens of any
// exclusion term list (AND within a list, OR across lists). Tokens use the
// same tokenization as query terms (camelCase-aware) over the union of body
// and path, so --exclude-term test also drops chunks from *_test.go.
func (ix *memIndex) excludedDocs(excl [][]string) map[int]bool {
	out := map[int]bool{}
	for _, toks := range excl {
		if len(toks) == 0 {
			continue
		}
		var cur map[int]bool
		for i, t := range toks {
			docs := map[int]bool{}
			for _, po := range ix.bodyIdx[t] {
				if !ix.chunks[po.Doc].Dead {
					docs[po.Doc] = true
				}
			}
			for _, po := range ix.pathIdx[t] {
				if !ix.chunks[po.Doc].Dead {
					docs[po.Doc] = true
				}
			}
			if i == 0 {
				cur = docs
			} else {
				for d := range cur {
					if !docs[d] {
						delete(cur, d)
					}
				}
			}
			if len(cur) == 0 {
				break
			}
		}
		for d := range cur {
			out[d] = true
		}
	}
	return out
}

func (ix *memIndex) searchBody(p queryPlan, excl [][]string, limit int) (int, []scoredChunk) {
	n := ix.live
	if n == 0 || len(p.runs) == 0 {
		return 0, nil
	}
	// flatten terms for candidacy/scoring; phrase runs verified separately
	var allTerms []string
	seen := map[string]bool{}
	for _, r := range p.runs {
		for _, t := range r.terms {
			if !seen[t] {
				seen[t] = true
				allTerms = append(allTerms, t)
			}
		}
	}
	// candidate docs: AND over all terms (union of body+path per term)
	type termDocs struct {
		term string
		docs map[int]bool // union of body+path
		body map[int]int  // doc -> tf in body
		path map[int]int  // doc -> tf in path
		df   int          // union document frequency
	}
	var tds []termDocs
	for _, t := range allTerms {
		td := termDocs{term: t, docs: map[int]bool{}, body: map[int]int{}, path: map[int]int{}}
		if !p.pathOnly {
			for _, po := range ix.bodyIdx[t] {
				if ix.chunks[po.Doc].Dead {
					continue
				}
				td.docs[po.Doc] = true
				td.body[po.Doc] = po.TF
			}
		}
		for _, po := range ix.pathIdx[t] {
			if ix.chunks[po.Doc].Dead {
				continue
			}
			td.docs[po.Doc] = true
			td.path[po.Doc] = po.TF
		}
		if len(td.docs) == 0 {
			return 0, nil // AND semantics: one missing term kills it
		}
		td.df = len(td.docs)
		tds = append(tds, td)
	}
	sort.Slice(tds, func(i, j int) bool { return len(tds[i].docs) < len(tds[j].docs) })
	byTerm := map[string]*termDocs{}
	for i := range tds {
		byTerm[tds[i].term] = &tds[i]
	}

	avgB, avgP := ix.avgBody(), ix.avgPath()
	defRes := compileDefRes(allTerms, p.core)
	coreLower := strings.ToLower(p.core)
	// lowercased terms for the hasDefPattern gate; include the full core
	// so camelCase/snake_case definitions match (the joined-identifier case)
	lowerTerms := make([]string, 0, len(allTerms)+1)
	for _, t := range allTerms {
		lowerTerms = append(lowerTerms, strings.ToLower(t))
	}
	if coreLower != "" {
		lowerTerms = append(lowerTerms, coreLower)
	}

	var scored []scoredChunk
	var exclDocs map[int]bool
	if len(excl) > 0 {
		exclDocs = ix.excludedDocs(excl)
	}
	for doc := range tds[0].docs {
		if exclDocs[doc] {
			continue
		}
		ok := true
		for _, td := range tds[1:] {
			if !td.docs[doc] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		// phrase runs must occur consecutively (body or path token stream)
		if !ix.phrasesMatch(doc, p.runs) {
			continue
		}
		bodyScore, pathScore := 0.0, 0.0
		for _, td := range tds {
			termIDF := idf(n, td.df)
			if tf, found := td.body[doc]; found {
				bodyScore += bm25Term(termIDF, tf, ix.bodyLen[doc], avgB)
			}
			if tf, found := td.path[doc]; found {
				pathScore += pathWeight * bm25Term(termIDF, tf, ix.pathLen[doc], avgP)
			}
		}
		score := bodyScore + pathScore
		if isTestOrGenerated(ix.chunks[doc].Path) {
			// Penalize body matches for test/generated files so they
			// don't outrank real source. Path matches are NOT penalized,
			// so explicit "foo_test" queries still find them.
			score = bodyScore*testPenalty + pathScore
		}
		// baked-in boosts (toggleable via env for ablation studies)
		text := ix.chunks[doc].Text
		lowered := strings.ToLower(text)
		if exactBoostOn && coreLower != "" && strings.Contains(lowered, coreLower) {
			score *= exactBonus
		}
		if defBoostOn {
			// Fast gate: hand-rolled matcher quickly rejects non-definitions.
			// Only run the expensive regex if the gate says "maybe".
			if hasDefPattern(lowered, lowerTerms) {
				for _, re := range defRes {
					if re.MatchString(text) {
						score *= defBonus
						break
					}
				}
			}
		}
		scored = append(scored, scoredChunk{doc, score})
	}
	// sort: score desc; ties broken deterministically by (path, line) so
	// that sync and full-build produce identical output regardless of
	// internal chunk indexing order (Go map iteration is random).
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		pi, pj := ix.chunks[scored[i].doc].Path, ix.chunks[scored[j].doc].Path
		if pi != pj {
			return pi < pj
		}
		return ix.chunks[scored[i].doc].Line < ix.chunks[scored[j].doc].Line
	})
	return len(scored), scored
}

// phrasesMatch verifies every phrase run occurs consecutively in the
// chunk's body or path token stream. Single-term runs always match here
// (their presence was already established by the AND candidacy).
func (ix *memIndex) phrasesMatch(doc int, runs []qrun) bool {
	d := ix.chunks[doc]
	for _, r := range runs {
		if !r.phrase {
			continue
		}
		if !hasPhrase(tokenize(splitCamel(d.Text)), r.terms) &&
			!hasPhrase(tokenize(d.Path), r.terms) {
			return false
		}
	}
	return true
}

func hasPhrase(toks, terms []string) bool {
	if len(terms) == 0 || len(toks) < len(terms) {
		return false
	}
outer:
	for i := 0; i+len(terms) <= len(toks); i++ {
		for j, t := range terms {
			if toks[i+j] != t {
				continue outer
			}
		}
		return true
	}
	return false
}

// likeSearch: literal case-insensitive substring, unranked, insertion order.
func (ix *memIndex) likeSearch(raw string, excl [][]string, limit int) (int, []scoredChunk) {
	needle := strings.ToLower(raw)
	var exclDocs map[int]bool
	if len(excl) > 0 {
		exclDocs = ix.excludedDocs(excl)
	}
	var out []scoredChunk
	for i, d := range ix.chunks {
		if d.Dead || exclDocs[i] {
			continue
		}
		if strings.Contains(strings.ToLower(d.Text), needle) {
			out = append(out, scoredChunk{i, 1})
		}
	}
	return len(out), out
}

// lastNameSegment extracts the trailing identifier from a literal query:
// "util::helper" -> "helper", `C:\temp\x` -> "x".
func lastNameSegment(raw string) string {
	segs := tokenize(raw) // splits on non-letter/number runs, lowercases
	if len(segs) == 0 {
		return ""
	}
	return segs[len(segs)-1]
}

// defIdentifierTerms returns the per-identifier token parts for a query,
// shared by defIdentifierPatterns (regex) and filterDefChunks (AST-name
// check). Single identifier (possibly camelCase/snake_case) -> one entry;
// several whitespace-separated identifiers -> one entry each.
func defIdentifierTerms(plan queryPlan) [][]string {
	var identTerms [][]string
	if plan.likeMode {
		name := lastNameSegment(plan.raw)
		if name == "" {
			return nil
		}
		identTerms = [][]string{{name}}
	} else if len(plan.runs) == 1 {
		// single term, implicit phrase ("add_numbers"), or explicit phrase:
		// one identifier (camelCase queries split into several AND runs,
		// but the pre-split core is a single field — handled below).
		identTerms = [][]string{plan.runs[0].terms}
	} else {
		// several whitespace-separated identifiers ("foo bar"): each is
		// its own identifier (split here, not via runs, because camelCase
		// splitting already turned one identifier into several runs).
		for _, field := range strings.Fields(plan.core) {
			if parts := tokenize(splitCamel(field)); len(parts) > 0 {
				identTerms = append(identTerms, parts)
			}
		}
	}
	return identTerms
}

// defIdentifierPatterns builds one definition-pattern regex per identifier
// in the query (see defIdentifierTerms). A chunk must match every
// identifier's pattern (mirroring the query's AND semantics). Each
// identifier matches in any separator style: the identifier "addNumbers"
// matches "func add_numbers", "def addNumbers", etc. Crucially, only the
// FULL identifier is required — a sub-term alone never suffices (so
// "getGraphParametersFlat" does not match "const flat = ..."). This
// strictness is what makes --defs-only a precision mode; the ranking bonus
// intentionally stays looser.
func defIdentifierPatterns(plan queryPlan) []*regexp.Regexp {
	identTerms := defIdentifierTerms(plan)
	kw := strings.Join(defKeywords, "|")
	// Optional Go receiver between "func" and the name (see compileDefRes).
	recv := `(?:\([^)]*\)\s+)?`
	var out []*regexp.Regexp
	for _, parts := range identTerms {
		var sb strings.Builder
		for i, p := range parts {
			if i > 0 {
				sb.WriteString(`[_-]?`)
			}
			sb.WriteString(regexp.QuoteMeta(p))
		}
		re, err := regexp.Compile(`(?i)(?:^|[^a-z0-9_])(?:` + kw + `)\s+` +
			// G1 (2026-11-14): the terminator excludes "." — see
			// compileDefRes: a dot after the name is a receiver.
			recv + sb.String() + `(?:[^a-z0-9_.]|$)`)
		if err == nil {
			out = append(out, re)
		}
	}
	return out
}

// defNameMatches reports whether the AST-recorded defined name of a chunk
// is the queried identifier, separator-style agnostic (addNumbers ==
// add_numbers == addnumbers) and case-insensitive. Full equality only:
// a prefix match would let "flat" claim the definition of
// "getGraphParametersFlat" (K5 lesson).
func defNameMatches(chunkName string, parts []string) bool {
	if chunkName == "" || len(parts) == 0 {
		return false
	}
	norm := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "_", "")
		s = strings.ReplaceAll(s, "-", "")
		return s
	}
	joined := ""
	for _, p := range parts {
		joined += norm(p)
	}
	return norm(chunkName) == joined
}

// filterDefChunks keeps only chunks that define every identifier in the
// query (see defIdentifierPatterns). A chunk passes an identifier when
// the definition regex matches its text OR when the AST recorded that
// identifier as the chunk's defined name (opsi 1, 2026-11-13: trust the
// AST — a signature chunk carries its defined name). The Name check is
// what makes the option sound: it ties the chunk to the identifier, so
// "func foo(bar string)" cannot leak into a query for "bar", keeping the
// AND semantics across identifiers; and it is what completes the option
// for one-line and body-less members, which never get a separate
// signature chunk (astchunk.go: sigEnd == endLine -> no sig chunk) but
// whose def body chunk carries the same recorded name. Windows and
// fallback chunks have Name == "" and are unaffected.
func filterDefChunks(sc []scoredChunk, ix *memIndex, plan queryPlan) []scoredChunk {
	res := defIdentifierPatterns(plan)
	terms := defIdentifierTerms(plan)
	if len(res) == 0 || len(terms) != len(res) {
		return nil
	}
	var out []scoredChunk
outer:
	for _, s := range sc {
		d := ix.chunks[s.doc]
		for i, re := range res {
			if re.MatchString(d.Text) {
				continue
			}
			if defNameMatches(d.Name, terms[i]) {
				continue
			}
			continue outer
		}
		out = append(out, s)
	}
	return out
}

// chunkSpan is a 1-based inclusive line range of one matched chunk.
type chunkSpan struct{ start, end int }

// contains reports whether a strictly contains b. An identical range
// is not containment: identical ranges are kept (exact duplicates are
// already removed at chunk time).
func (a chunkSpan) contains(b chunkSpan) bool {
	return a.start <= b.start && a.end >= b.end &&
		(a.start != b.start || a.end != b.end)
}

// chunkSpansAndGroups measures the line span of every matched chunk and
// groups their indexes by file path. Shared by dedupContained and
// dropContainedSigs: the two differ only in which nested chunk they
// drop, not in how spans are measured.
func chunkSpansAndGroups(sc []scoredChunk, ix *memIndex) ([]chunkSpan, map[string][]int) {
	spans := make([]chunkSpan, len(sc))
	for i, s := range sc {
		d := ix.chunks[s.doc]
		n := 1
		if d.Text != "" {
			n = strings.Count(d.Text, "\n") + 1
		}
		spans[i] = chunkSpan{d.Line, d.Line + n - 1}
	}
	byPath := map[string][]int{}
	for i, s := range sc {
		byPath[ix.chunks[s.doc].Path] = append(byPath[ix.chunks[s.doc].Path], i)
	}
	return spans, byPath
}

// dedupContained resolves nested matches within one file: among chunks
// where one strictly contains another, only the best survives — the
// higher BM25 score wins; on a tie the smaller (more specific) chunk
// wins. That is why a name-only query shows a signature chunk (dense,
// short) while a name+body-term query shows the body chunk (more
// matching terms). Overlapping but non-containing chunks (fixed
// windows, split definitions) are kept; identical ranges are kept
// (exact duplicates are already removed at chunk time).
// O(k^2) per file over its matched chunks only.
func dedupContained(sc []scoredChunk, ix *memIndex) []scoredChunk {
	spans, byPath := chunkSpansAndGroups(sc, ix)
	drop := make([]bool, len(sc))
	for _, idxs := range byPath {
		for _, i := range idxs {
			for _, j := range idxs {
				if i == j {
					continue
				}
				a, b := spans[i], spans[j]
				if !a.contains(b) && !b.contains(a) {
					continue
				}
				// i is nested with j: drop i when j scores higher,
				// or ties with a strictly smaller span.
				if sc[j].score > sc[i].score ||
					(sc[j].score == sc[i].score && a.contains(b)) {
					drop[i] = true
					break
				}
			}
		}
	}
	out := make([]scoredChunk, 0, len(sc))
	for i, s := range sc {
		if !drop[i] {
			out = append(out, s)
		}
	}
	return out
}

// dropContainedSigs implements --full: a signature chunk is dropped
// when its body chunk is also among the matches (the body is always a
// valid substitute — it contains the signature text). A signature
// whose body is absent (excluded, or a one-liner with no separate
// signature) is kept, so --full never loses a hit.
func dropContainedSigs(sc []scoredChunk, ix *memIndex) []scoredChunk {
	spans, byPath := chunkSpansAndGroups(sc, ix)
	drop := make([]bool, len(sc))
	for _, idxs := range byPath {
		for _, i := range idxs {
			if ix.chunks[sc[i].doc].Kind != "sig" {
				continue
			}
			for _, j := range idxs {
				if i == j {
					continue
				}
				if spans[j].contains(spans[i]) {
					drop[i] = true
					break
				}
			}
		}
	}
	out := make([]scoredChunk, 0, len(sc))
	for i, s := range sc {
		if !drop[i] {
			out = append(out, s)
		}
	}
	return out
}

func diversify(sc []scoredChunk, ix *memIndex, limit, perFileCap int, full bool) []scoredChunk {
	// Nested-chunk policy (capped regime only): --full drops signature
	// chunks whose body is present; otherwise the score-based nested
	// rule picks the best chunk per nest. Unlimited recall (--all /
	// --per-file-cap 0) bypasses this and keeps every matching chunk.
	if perFileCap > 0 {
		if full {
			sc = dropContainedSigs(sc, ix)
		}
		sc = dedupContained(sc, ix)
	}
	// limit 0 means show nothing. Check before appending: the old
	// append-then-break loop always showed 1 hit for limit 0, even
	// though the documented semantics were show-none.
	if limit == 0 {
		return nil
	}
	seen := map[string]int{}
	var out []scoredChunk
	for _, s := range sc {
		p := ix.chunks[s.doc].Path
		if perFileCap > 0 && seen[p] >= perFileCap {
			continue
		}
		if perFileCap > 0 && seen[p] == 1 {
			s.score *= satDecay
		}
		seen[p]++
		out = append(out, s)
		if limit >= 0 && len(out) >= limit {
			break
		}
	}
	// Decay may have reordered; restore score-desc with the same
	// deterministic (path, line) tie-break used in searchBody.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		pi, pj := ix.chunks[out[i].doc].Path, ix.chunks[out[j].doc].Path
		if pi != pj {
			return pi < pj
		}
		return ix.chunks[out[i].doc].Line < ix.chunks[out[j].doc].Line
	})
	return out
}
