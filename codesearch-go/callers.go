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

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// callLang pairs a file extension with its tree-sitter language and the
// query capturing call expressions: the whole call as @call, its callee
// expression as @callee. Field name "function" holds across Go, Rust and
// Python call nodes.
type callLang struct {
	ext   string
	lang  func() *gotreesitter.Language
	query string
}

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
	{".js", grammars.JavascriptLanguage, `[
		(call_expression function: (_) @callee) @call
	]`},
	{".mjs", grammars.JavascriptLanguage, `[
		(call_expression function: (_) @callee) @call
	]`},
	{".cjs", grammars.JavascriptLanguage, `[
		(call_expression function: (_) @callee) @call
	]`},
	{".ts", grammars.TypescriptLanguage, `[
		(call_expression function: (_) @callee) @call
	]`},
	{".mts", grammars.TypescriptLanguage, `[
		(call_expression function: (_) @callee) @call
	]`},
	{".cts", grammars.TypescriptLanguage, `[
		(call_expression function: (_) @callee) @call
	]`},
	{".tsx", grammars.TsxLanguage, `[
		(call_expression function: (_) @callee) @call
	]`},
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

var callerDefLangs = []callLang{
	{".js", grammars.JavascriptLanguage, jsDefQuery},
	{".mjs", grammars.JavascriptLanguage, jsDefQuery},
	{".cjs", grammars.JavascriptLanguage, jsDefQuery},
	{".ts", grammars.TypescriptLanguage, jsDefQuery},
	{".mts", grammars.TypescriptLanguage, jsDefQuery},
	{".cts", grammars.TypescriptLanguage, jsDefQuery},
	{".tsx", grammars.TsxLanguage, jsDefQuery},
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

// calleeBase reduces a callee expression to its trailing identifier:
// generics and index arguments are stripped (foo::<T> -> foo,
// pkg.Fn[int] -> Fn), then the last segment after "." or "::" wins.
// "" when nothing identifier-like remains.
func calleeBase(expr string) string {
	if i := strings.IndexAny(expr, "<["); i >= 0 {
		expr = expr[:i]
	}
	expr = strings.ReplaceAll(expr, "::", ".")
	parts := strings.Split(expr, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return ""
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
	// JS/TS def query. One parse per file serves both queries.
	var defs []definition
	if dq, ok := defQueryFor(ext); ok {
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
	files := iterFiles(root, cfg)
	var sites []callSite
	// Skip taxonomy, in the order the scan itself encounters them:
	// readSource failure -> readSource skip -> unsupported language ->
	// AST oversize -> parse failure or parse timeout. Buckets are
	// disjoint; parsed + skipped == len(files). A file with an
	// unsupported extension AND a readSource skip counts as source-skip
	// (the haveText gate runs before extractCalls), never double-counted.
	// The timeout bounds one file, not the run: a repo full of
	// pathological files still sums (worst case N x 5s). Leaked timeout
	// goroutines hold at most maxASTBytes (oversize is filtered first).
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
