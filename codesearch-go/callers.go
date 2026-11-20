package main

// --callers: structural "who calls X?" query on top of the gotreesitter AST.
//
// Unlike BM25/literal search (which finds a string wherever it appears),
// this answers a role question: which call expressions invoke a function
// with this trailing name, in which enclosing function, at which line.
// Matching is name-based, not type-resolved: f() where f is a local
// variable is still reported as a call to f, and pkg.Foo() matches a query
// for Foo (trailing-segment match, mirroring --defs-only's likeMode rule).
//
// Fail-open per file: unsupported language, oversized file, or a
// parse/query failure skips that file instead of failing the search.
// The scan is always fresh (no BM25 index involved): it reuses iterFiles
// and readSource, so --ext, --exclude and --no-gitignore apply, and
// --json selects machine-readable output.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// callLang pairs a file extension with its tree-sitter language and the
// query capturing call expressions: the whole call as @call, its callee
// expression as @callee. Field name "function" holds across Go, Rust,
// Python and JS/TS call nodes. Kotlin has no "function" field (its
// grammar carries no field names on call expressions at all), so the
// Kotlin query captures the callee positionally instead: a bare
// simple_identifier for plain calls, the whole navigation_expression
// for qualified calls (client.connect(), super.foo(), this.bar()).
type callLang struct {
	ext   string
	lang  func() *gotreesitter.Language
	query string
}

// ktCallQuery captures Kotlin call expressions. Node names verified
// against tree-sitter-kotlin by probe (2026-11-12): the callee is the
// direct simple_identifier child for plain calls, or the whole
// navigation_expression for qualified calls (client.connect(),
// super.foo(), this.bar()) — the full expression, like Go's
// "x.Method" and Rust's "foo::bar", so calleeBase reduces it to the
// trailing name. Constructor calls (Foo()) are call_expressions too
// and match the plain pattern.
// jsCallQuery captures both standard calls and JS/TS constructor invocations (`new Foo(...)`).
const jsCallQuery = `[
	(call_expression function: (_) @callee) @call
	(new_expression constructor: (_) @callee) @call
]`

const ktCallQuery = `[
	(call_expression (simple_identifier) @callee) @call
	(call_expression (navigation_expression) @callee) @call
]`

var callLangs = []callLang{
	{".go", grammars.GoLanguage, `[
		(call_expression function: (_) @callee) @call
	]`},
	{".rs", grammars.RustLanguage, `[
		(call_expression function: (_) @callee) @call
	]`},
	{".py", grammars.PythonLanguage, `[
		(call function: (_) @callee) @call
	]`},
	{".js", grammars.JavascriptLanguage, jsCallQuery},
	{".mjs", grammars.JavascriptLanguage, jsCallQuery},
	{".cjs", grammars.JavascriptLanguage, jsCallQuery},
	{".ts", grammars.TypescriptLanguage, jsCallQuery},
	{".mts", grammars.TypescriptLanguage, jsCallQuery},
	{".cts", grammars.TypescriptLanguage, jsCallQuery},
	{".tsx", grammars.TsxLanguage, jsCallQuery},
	{".jsx", grammars.TsxLanguage, jsCallQuery},
	{".kt", grammars.KotlinLanguage, ktCallQuery},
}

type callSpec struct {
	lang  *gotreesitter.Language
	query *gotreesitter.Query
}

// lookupCallLang finds ext's row in a callLang table, or nil.
func lookupCallLang(table []callLang, ext string) *callLang {
	for i := range table {
		if table[i].ext == ext {
			return &table[i]
		}
	}
	return nil
}

// compileCallSpec builds a callSpec from one table row (no caching;
// callers go through their specStore, which owns the lock and cache).
func compileCallSpec(cl *callLang) (*callSpec, bool) {
	if cl == nil {
		return nil, false
	}
	lang := cl.lang()
	if lang == nil {
		return nil, false
	}
	q, err := gotreesitter.NewQuery(cl.query, lang)
	if err != nil {
		return nil, false
	}
	return &callSpec{lang: lang, query: q}, true
}

var callSpecStore = newSpecStore[callSpec]()

// getCallSpec returns the compiled language+call-query for ext.
func getCallSpec(ext string) (*callSpec, bool) {
	return callSpecStore.get(ext, func() (*callSpec, bool) {
		return compileCallSpec(lookupCallLang(callLangs, ext))
	})
}

// callerDefLangs supplies enclosing-function ranges for languages that
// astDefinitions does not cover (JS/TS). Chunking stays untouched
// (fail-open fallback); this only feeds the caller column of --callers.
// Arrow functions and function expressions take the name of their
// variable declarator.
// jsDefQuery is shared with chunking (astchunk.go): classes count as
// chunk units too, and class-level calls report the class as caller.
// Function-like patterns also capture @body (ignored by callers) so
// chunking can split a signature chunk from the body chunk.
const jsDefQuery = `[
	(function_declaration name: (identifier) @name body: (statement_block)? @body) @def
	(method_definition name: (property_identifier) @name body: (statement_block)? @body) @def
	(class_declaration name: (identifier) @name) @def
	(variable_declarator name: (identifier) @name value: (arrow_function body: (_) @body) @def)
	(variable_declarator name: (identifier) @name value: (function_expression body: (statement_block) @body) @def)
]`

// tsDefQuery is jsDefQuery with two TypeScript-only widenings: the
// class-name pattern accepts type_identifier (in the TypeScript
// grammar a class name is a type_identifier, not an identifier — F2:
// --related never saw a TS class definition), and interface members
// count as definitions via method_signature (2026-11-13: --defs-only
// never saw an interface member, because it is a method_signature,
// not a method_definition). Neither can live in jsDefQuery itself:
// tree-sitter-javascript has neither node, so the shared query would
// fail to compile for .js/.mjs/.cjs (caught by TestExtractRelatedJS
// for type_identifier and by TestTSDefQueryMethodSignature for
// method_signature). The append uses TrimSuffix, not Replace on "]":
// the widened class pattern already contains "]", so a plain Replace
// would splice the new pattern into the class alternation.
var tsDefQuery = strings.TrimSuffix(strings.Replace(jsDefQuery,
	"(class_declaration name: (identifier) @name)",
	"(class_declaration name: [(identifier) (type_identifier)] @name)", 1), "]") +
	"\n\t(method_signature name: (property_identifier) @name) @def\n]"

var callerDefLangs = []callLang{
	{".js", grammars.JavascriptLanguage, jsDefQuery},
	{".mjs", grammars.JavascriptLanguage, jsDefQuery},
	{".cjs", grammars.JavascriptLanguage, jsDefQuery},
	{".ts", grammars.TypescriptLanguage, tsDefQuery},
	{".mts", grammars.TypescriptLanguage, tsDefQuery},
	{".cts", grammars.TypescriptLanguage, tsDefQuery},
	{".tsx", grammars.TsxLanguage, tsDefQuery},
	{".jsx", grammars.TsxLanguage, tsDefQuery},
}

var callerDefSpecStore = newSpecStore[callSpec]()

// getCallerDefSpec returns the compiled language+def-query for JS/TS exts.
func getCallerDefSpec(ext string) (*callSpec, bool) {
	return callerDefSpecStore.get(ext, func() (*callSpec, bool) {
		return compileCallSpec(lookupCallLang(callerDefLangs, ext))
	})
}

// defQueryFor returns the definition query for ext: the chunking def
// query where available (Go/Rust/Python), else the JS/TS def query.
func defQueryFor(ext string) (*gotreesitter.Query, bool) {
	if cs, ok := getSpec(ext); ok {
		return cs.query, true
	}
	if ds, ok := getCallerDefSpec(ext); ok {
		return ds.query, true
	}
	return nil, false
}

// callSite is one call expression: the file, the 1-based line the call
// starts on, the enclosing definition's name ("" at top level), the callee
// expression as written, and the full source line.
type callSite struct {
	Path   string
	Line   int
	Caller string
	Callee string
	Text   string
}

// isKotlinDeclHeaderCall reports whether a call match is really the
// header of a Kotlin declaration the grammar failed to parse: when
// `@Annotation internal fun Name(...)` degrades to an ERROR node,
// tree-sitter treats the `Name(...)` fragment as a call_expression,
// so the declaration line itself shows up as a call site with an
// empty caller. A match is a header call when its callee is the name
// of a def starting on the same line and no body delimiter (`=` or
// `{` at paren depth 0) precedes the call on that line — a real call
// in a single-expression body (`fun wrap(x) = Name(x)`) or a
// single-line body (`fun f() { Name() }`) always has such a delimiter
// before it. Known residual: a recursive call in a default argument
// (`fun f(x: Int = f(0))`) has no delimiter before it and is filtered
// too; a def the regex fallback missed keeps its false header.
func isKotlinDeclHeaderCall(calStart uint32, calText string, line int, lineText string, li lineIndex, defs []definition) bool {
	callee := calleeBase(calText)
	if callee == "" {
		return false
	}
	found := false
	for i := range defs {
		if defs[i].name == callee && defs[i].startLine == line {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	// Column of the call within its line.
	lineStart := li.lineStart(line)
	if lineStart < 0 || int(calStart) < lineStart {
		return false
	}
	col := int(calStart) - lineStart
	if col > len(lineText) {
		return false
	}
	prefix := lineText[:col]
	// A body delimiter at paren depth 0 before the call means the call
	// is inside the body (or the single-expression body), not the header.
	depth := 0
	for _, r := range prefix {
		switch r {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case '=', '{':
			if depth == 0 {
				return false
			}
		}
	}
	return true
}

// calleeBase reduces a callee expression to its trailing identifier,
// processing right-to-left: a balanced generic/index suffix at the very
// end (<...> or [...]) is peeled first — repeatedly, so foo::<T>[0]
// becomes foo — then trailing separators (. :: ?.) are skipped and the
// trailing identifier run wins. Indexed receivers (items[0].execute)
// and Rust turbofish (Boxed::<u32>::build) keep their trailing name,
// while a call result used as the callee (getFn(x)()) yields "".
// "" when nothing identifier-like remains.
//
// The old implementation cut at the FIRST < or [ anywhere in the
// expression, so items[0].execute reduced to "items" (a false positive
// for --callers items) and execute itself matched nothing (P1-A, go14).
func calleeBase(expr string) string {
	expr = strings.TrimSpace(expr)
	for {
		stripped, ok := stripBalancedSuffix(expr)
		if !ok {
			break
		}
		expr = strings.TrimSpace(stripped)
	}
	// Skip trailing separators (foo::, a., x?.) then take the run.
	i := len(expr)
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(expr[:i])
		if r == '.' || r == ':' || r == '?' || unicode.IsSpace(r) {
			i -= size
			continue
		}
		break
	}
	j := i
	for j > 0 {
		r, size := utf8.DecodeLastRuneInString(expr[:j])
		if r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			j -= size
			continue
		}
		break
	}
	return expr[j:i]
}

// stripBalancedSuffix peels one balanced <...> or [...] suffix at the end
// of expr, returning the remainder. ok=false when expr does not end with
// a bracket or the brackets are unbalanced (malformed input: the caller
// then falls through to the identifier run, which yields "" for a
// bracket-terminated expression).
func stripBalancedSuffix(expr string) (string, bool) {
	if expr == "" {
		return expr, false
	}
	var open, close byte
	switch expr[len(expr)-1] {
	case '>':
		open, close = '<', '>'
	case ']':
		open, close = '[', ']'
	default:
		return expr, false
	}
	depth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		switch expr[i] {
		case close:
			depth++
		case open:
			depth--
			if depth == 0 {
				return expr[:i], true
			}
		}
	}
	return expr, false
}

// extractCalls parses text and returns every call site in it.
// ok=false means: unsupported extension, oversized file, or a parse/query
// failure — the caller skips the file (fail-open).
func extractCalls(relPath, text string) ([]callSite, bool) {
	ext := extOf(relPath)
	spec, ok := getCallSpec(ext)
	if !ok || len(text) > maxASTBytes {
		return nil, false
	}
	// Normalize line endings exactly like chunkLines, so byte offsets
	// from the parser and our line numbers agree.
	norm := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if isGeneratedOrMinified(relPath, norm, maxLineLenOf(norm)) {
		return nil, false
	}
	src := []byte(norm)
	lines := strings.Split(norm, "\n")
	// Shared byte-offset -> line mapping (related.go).
	li := newLineIndex(src)
	lineOf := li.lineOf
	parser := gotreesitter.NewParser(spec.lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, false
	}
	// Enclosing definitions (with names) for the caller column, from the
	// same parsed tree: the chunking def query where available, else the
	// JS/TS def query. One parse per file serves both queries. Kotlin
	// defs come from the shared helper (kotlin_fallback.go), so calls
	// inside a function the grammar failed to parse are still
	// attributed to that function instead of landing at top level.
	var defs []definition
	if ext == ".kt" {
		if cs, ok := getSpec(ext); ok {
			defs = kotlinASTAndFallbackDefs(tree, cs.lang, cs.query, src, lineOf)
		}
	} else if dq, ok := defQueryFor(ext); ok {
		dcursor := dq.Exec(tree.RootNode(), spec.lang, src)
		for {
			m, ok := dcursor.NextMatch()
			if !ok {
				break
			}
			var sb, eb uint32
			var name string
			found := false
			for _, c := range m.Captures {
				if c.Name == "def" {
					sb, eb = c.Node.StartByte(), c.Node.EndByte()
					found = true
				}
				if c.Name == "name" {
					name = string(src[c.Node.StartByte():c.Node.EndByte()])
				}
			}
			if !found {
				continue
			}
			sl, el := lineOf(sb), lineOf(eb)
			if el < sl {
				continue
			}
			defs = append(defs, definition{startLine: sl, endLine: el, name: name})
		}
	}
	callerOf := func(line int) string {
		best := ""
		bestSpan := 0
		for _, d := range defs {
			if d.name == "" {
				continue
			}
			if d.startLine <= line && line <= d.endLine {
				if span := d.endLine - d.startLine; best == "" || span < bestSpan {
					best, bestSpan = d.name, span
				}
			}
		}
		return best
	}
	var out []callSite
	cursor := spec.query.Exec(tree.RootNode(), spec.lang, src)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var callStart, calStart, calEnd uint32
		var calText string
		found := false
		for _, c := range m.Captures {
			switch c.Name {
			case "call":
				callStart = c.Node.StartByte()
			case "callee":
				calStart, calEnd = c.Node.StartByte(), c.Node.EndByte()
				calText = string(src[calStart:calEnd])
				found = true
			}
		}
		if !found {
			continue
		}
		line := lineOf(callStart)
		text := ""
		if line >= 1 && line <= len(lines) {
			text = lines[line-1]
		}
		// Kotlin: a declaration header the grammar failed to parse
		// degrades to an ERROR node whose `Name(...)` fragment reads
		// as a call — filter that false call site (see
		// isKotlinDeclHeaderCall).
		if ext == ".kt" && isKotlinDeclHeaderCall(calStart, calText, line, text, li, defs) {
			continue
		}
		out = append(out, callSite{
			Path:   relPath,
			Line:   line,
			Caller: callerOf(line),
			Callee: calText,
			Text:   text,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out, true
}

// callersEnvelope is the --json output for --callers. Stdout stays pure
// JSON; the human summary goes to stderr like the BM25 envelope.
type callersEnvelope struct {
	Query string         `json:"query"`
	Batch bool           `json:"batch,omitempty"`
	Total int            `json:"total"`
	Calls []callSiteJSON `json:"calls"`
}

type callSiteJSON struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Caller string `json:"caller"`
	Callee string `json:"callee"`
	Text   string `json:"text"`
}

// runCallers scans root fresh and prints every call site whose callee's
// trailing name matches the query's trailing name. Returns a process
// exit code.
// extOf returns the lowercased extension of path ("" when none).
func extOf(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return strings.ToLower(path[i:])
	}
	return ""
}

// splitBatchNames splits a --callers/--related flag value on commas:
// "a, b,,c" -> ["a" "b" "c"]. Empty segments are dropped and duplicates
// removed, so the batch is deterministic for a given flag value. Names
// cannot contain commas, so the split is unambiguous.
func splitBatchNames(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(s, ",") {
		name := strings.TrimSpace(part)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func runCallers(o options, root string, cfg *ScanConfig) int {
	names := splitBatchNames(o.callers)
	wants := map[string]bool{}
	for _, name := range names {
		if base := calleeBase(name); base != "" {
			wants[base] = true
		}
	}
	if len(wants) == 0 {
		fmt.Fprintln(os.Stderr, "codesearch: --callers requires a non-empty name")
		return 2
	}
	var sites []callSite
	if o.cached {
		ix, err := loadCachedIndex(o, root, cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "codesearch:", err)
			return 1
		}
		for _, c := range ix.callSites {
			if wants[calleeBase(c.Callee)] {
				sites = append(sites, c)
			}
		}
		sort.SliceStable(sites, func(i, j int) bool {
			if sites[i].Path != sites[j].Path {
				return sites[i].Path < sites[j].Path
			}
			return sites[i].Line < sites[j].Line
		})
	} else {
		files := iterFiles(root, cfg)
		// Skip taxonomy, in the order the scan itself encounters them:
		// readSource failure -> readSource skip -> unsupported language ->
		// AST oversize -> generated/minified skip -> parse failure or
		// parse timeout. Buckets are disjoint; parsed + skipped == len(files).
		var parsed, unsupported, astOversize, parseFail, timedOut, sourceSkip, unreadable int
		t0 := time.Now()
		for _, rel := range files {
			fr := readSource(root, rel)
			if !fr.statOK {
				unreadable++
				continue
			}
			if fr.skip != "" {
				sourceSkip++
				continue
			}
			if _, ok := getCallSpec(extOf(rel)); !ok {
				unsupported++
				continue
			}
			if len(fr.text) > maxASTBytes {
				astOversize++
				continue
			}
			norm := strings.ReplaceAll(strings.ReplaceAll(fr.text, "\r\n", "\n"), "\r", "\n")
			if isGeneratedOrMinified(rel, norm, maxLineLenOf(norm)) {
				sourceSkip++
				continue
			}
			var calls []callSite
			var ok bool
			if !runTimed(astTimeout, func() {
				calls, ok = extractCalls(rel, fr.text)
			}) {
				timedOut++
				continue
			}
			if !ok {
				parseFail++
				continue
			}
			parsed++
			for _, c := range calls {
				if wants[calleeBase(c.Callee)] {
					sites = append(sites, c)
				}
			}
		}
		skipped := unsupported + astOversize + parseFail + timedOut + sourceSkip + unreadable
		fmt.Fprintf(os.Stderr, "(callers: parsed %d files, skipped %d (unsupported %d, ast-oversize %d, parse-fail %d, timeout %d, source-skip %d, unreadable %d) in %s)\n",
			parsed, skipped, unsupported, astOversize, parseFail, timedOut, sourceSkip, unreadable, time.Since(t0).Round(time.Millisecond))
	}
	if o.jsonOut {
		env := callersEnvelope{Query: o.callers, Batch: len(names) > 1, Total: len(sites), Calls: []callSiteJSON{}}
		for _, s := range sites {
			env.Calls = append(env.Calls, callSiteJSON{
				Path: s.Path, Line: s.Line, Caller: s.Caller, Callee: s.Callee, Text: s.Text,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(env); err != nil {
			fmt.Fprintf(os.Stderr, "codesearch: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "%d call site(s)\n", len(sites))
		return 0
	}
	for _, s := range sites {
		caller := s.Caller
		if caller == "" {
			caller = "-"
		}
		fmt.Printf("%s:%d: %s: %s\n", s.Path, s.Line, caller, s.Text)
	}
	fmt.Fprintf(os.Stderr, "%d call site(s)\n", len(sites))
	return 0
}
