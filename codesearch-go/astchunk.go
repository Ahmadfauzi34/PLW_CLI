package main

// Function-aware chunking via tree-sitter (gotreesitter: pure Go, no cgo).
//
// For supported languages (Go, Rust, Python, JavaScript, TypeScript,
// Kotlin), a file is chunked into one chunk per function/method/class/type
// definition instead of fixed 30-line windows, so a snippet never splits
// a definition in half (cAST-style). A definition's doc comment
// (contiguous doc-syntax lines directly above it, capped) is part of its
// chunk. Function-like definitions additionally yield a signature
// chunk (doc comment + declaration header) before the body chunk, so a
// name query shows the signature while a body-term query shows the
// body; --full forces bodies. Lines not covered by any definition are covered by fixed
// windows ("module chunks"), so every line stays searchable.
// Definitions longer than maxDefLines are split with overlap.
// Generated/minified files, unknown languages, oversized files, or
// parse/query failures fall back to chunkLines (fail-open): the tool
// never returns less than the old behavior.

import (
	"sort"
	"strings"
	"time"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const (
	maxDefLines    = 80      // definitions longer than this are split with overlap
	defSplitStride = 40      // stride when splitting a long definition
	maxASTBytes    = 256_000 // files larger than this use fixed windows
	// astTimeout bounds one file's AST attempt. Pure-Go tree-sitter error
	// recovery can degrade superlinearly on long runs of syntax errors
	// (measured 12s parse for 20k invalid lines vs 0.1s for valid code of
	// the same size), so a slow attempt fails open to fixed windows instead
	// of stalling the search. Generous: legit files (3k defs) parse in
	// <0.3s on a VM, with headroom for slow devices.
	astTimeout = 5 * time.Second
)

// defQuery captures definition nodes as @def (their names as @name, unused
// here but kept for debugging). Patterns verified by spike 2026-10-09:
// Go (func, method, type), Rust (fn incl. methods in impl blocks;
// struct, enum, trait, impl, type alias), Python (def, class).
// JS/TS reuse the callers def query (function, method, class,
// arrow/function-expression declarators) plus TS-only type declarations;
// TS uses tsDefQuery (callers.go), whose class pattern also accepts
// type_identifier. The query strings concatenate as two pattern lists.
// Kotlin uses ktDefQuery (class/function/object/type-alias; positional
// names — the grammar has no "name"/"body" fields).
type astLang struct {
	ext   string
	lang  func() *gotreesitter.Language
	query string
}

// tsChunkExtra adds TypeScript-only declaration units to the shared JS
// def query. Node names verified against tree-sitter-typescript.
const tsChunkExtra = `[
	(interface_declaration name: (type_identifier) @name) @def
	(type_alias_declaration name: (type_identifier) @name) @def
	(enum_declaration name: (identifier) @name) @def
]`

// ktDefQuery captures Kotlin definition units. Node names verified
// against tree-sitter-kotlin by probe (2026-11-12, S-expression of a
// real .kt file): class/interface/enum/data/sealed/annotation/
// fun-interface are ALL class_declaration; typealias is type_alias.
// Unlike Go/Rust/JS, Kotlin declarations carry NO "name" or "body"
// field (ChildByFieldName returns nil), so names and bodies are
// captured positionally: the first type_identifier/simple_identifier
// child is the name (supertypes live in delegation_specifier, never
// as a direct type_identifier child), and function_body is the
// optional body child (single-expression functions wrap their
// expression in a function_body too). Properties are deliberately
// NOT here: a local val inside a function would become its own def
// chunk, the way JS plain vars are not defs either — top-level vals
// are found by --related instead (K4).
const ktDefQuery = `[
	(class_declaration (type_identifier) @name) @def
	(function_declaration (simple_identifier) @name (function_body)? @body) @def
	(object_declaration (type_identifier) @name) @def
	(type_alias (type_identifier) @name) @def
]`

var astLangs = []astLang{
	{".go", grammars.GoLanguage, `[
		(function_declaration name: (identifier) @name body: (block)? @body) @def
		(method_declaration name: (field_identifier) @name body: (block)? @body) @def
		(type_spec name: (type_identifier) @name) @def
	]`},
	{".rs", grammars.RustLanguage, `[
		(function_item name: (identifier) @name body: (block)? @body) @def
		(function_signature_item name: (identifier) @name) @def
		(struct_item name: (type_identifier) @name) @def
		(enum_item name: (type_identifier) @name) @def
		(trait_item name: (type_identifier) @name) @def
		(impl_item type: (_) @name) @def
		(type_item name: (type_identifier) @name) @def
	]`},
	{".py", grammars.PythonLanguage, `[
		(function_definition name: (identifier) @name body: (block) @body) @def
		(class_definition name: (identifier) @name) @def
	]`},
	{".js", grammars.JavascriptLanguage, jsDefQuery},
	{".mjs", grammars.JavascriptLanguage, jsDefQuery},
	{".cjs", grammars.JavascriptLanguage, jsDefQuery},
	{".ts", grammars.TypescriptLanguage, tsDefQuery + tsChunkExtra},
	{".mts", grammars.TypescriptLanguage, tsDefQuery + tsChunkExtra},
	{".cts", grammars.TypescriptLanguage, tsDefQuery + tsChunkExtra},
	{".tsx", grammars.TsxLanguage, tsDefQuery + tsChunkExtra},
	{".jsx", grammars.TsxLanguage, tsDefQuery + tsChunkExtra},
	{".kt", grammars.KotlinLanguage, ktDefQuery},
}

type compiledSpec struct {
	lang  *gotreesitter.Language
	query *gotreesitter.Query
}

var astSpecStore = newSpecStore[compiledSpec]()

// getSpec returns the compiled language+query for ext, building it once.
// Thread-safe: buildFull chunks files in parallel (see specStore).
func getSpec(ext string) (*compiledSpec, bool) {
	return astSpecStore.get(ext, func() (*compiledSpec, bool) {
		var al *astLang
		for i := range astLangs {
			if astLangs[i].ext == ext {
				al = &astLangs[i]
				break
			}
		}
		if al == nil {
			return nil, false
		}
		lang := al.lang()
		if lang == nil {
			return nil, false
		}
		q, err := gotreesitter.NewQuery(al.query, lang)
		if err != nil {
			return nil, false
		}
		return &compiledSpec{lang: lang, query: q}, true
	})
}

// definition is a 1-based inclusive line range of one def node, plus its
// declared name ("" when the query did not capture one) and the last
// line of its signature chunk (0 = no separate signature chunk: the
// def has no body, or its body starts on the def's first line).
type definition struct {
	startLine int
	endLine   int
	name      string
	sigEnd    int
}

// astDefinitions parses text and returns definition line ranges.
// ok=false means: unsupported extension, oversized file, or a parse/query
// failure — the caller falls back to chunkLines.
func astDefinitions(relPath, text string) (defs []definition, ok bool) {
	ext := extOf(relPath)
	spec, ok := getSpec(ext)
	if !ok || len(text) > maxASTBytes {
		return nil, false
	}
	// Normalize line endings exactly like chunkLines, so byte offsets
	// from the parser and our line numbers agree.
	norm := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	src := []byte(norm)
	// Shared byte-offset -> line mapping (related.go): one scan builds
	// it, the max-line scan below is separate (kept for the
	// generated/minified check, which needs line lengths, not offsets).
	li := newLineIndex(src)
	lineOf := li.lineOf
	maxLineLen := maxLineLenOf(norm)
	if isGeneratedOrMinified(relPath, norm, maxLineLen) {
		return nil, false
	}
	parser := gotreesitter.NewParser(spec.lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, false
	}
	cursor := spec.query.Exec(tree.RootNode(), spec.lang, src)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var sb, eb uint32
		var name string
		var bodyStart uint32
		hasBody := false
		found := false
		for _, c := range m.Captures {
			if c.Name == "def" {
				sb, eb = c.Node.StartByte(), c.Node.EndByte()
				found = true
			}
			if c.Name == "name" {
				name = string(src[c.Node.StartByte():c.Node.EndByte()])
			}
			if c.Name == "body" {
				bodyStart = c.Node.StartByte()
				hasBody = true
			}
		}
		if !found {
			continue
		}
		sl, el := lineOf(sb), lineOf(eb)
		if el < sl {
			continue
		}
		d := definition{sl, el, name, 0}
		if hasBody {
			// Signature end: the body's first line for brace languages
			// (the block starts on the `{` line); the line before the
			// body for Python (the block starts after the `:` line).
			bl := lineOf(bodyStart)
			if ext == ".py" {
				d.sigEnd = bl - 1
			} else {
				d.sigEnd = bl
			}
		}
		defs = append(defs, d)
	}
	if len(defs) == 0 {
		return nil, false
	}
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].startLine != defs[j].startLine {
			return defs[i].startLine < defs[j].startLine
		}
		return defs[i].endLine < defs[j].endLine
	})
	// dedup identical ranges, preferring the entry that carries a
	// signature (a def can match twice when its body is optional).
	out := defs[:0]
	for _, d := range defs {
		dup := -1
		for i, o := range out {
			if o.startLine == d.startLine && o.endLine == d.endLine && o.name == d.name {
				dup = i
				break
			}
		}
		if dup < 0 {
			out = append(out, d)
		} else if out[dup].sigEnd == 0 && d.sigEnd > 0 {
			out[dup] = d
		}
	}
	return attachDocComments(out, strings.Split(norm, "\n"), docMarkers(ext)), true
}

// maxLineLenOf returns the longest byte line length in normalized text.
func maxLineLenOf(norm string) int {
	maxLen, cur := 0, 0
	for i := 0; i < len(norm); i++ {
		if norm[i] == '\n' {
			if cur > maxLen {
				maxLen = cur
			}
			cur = 0
		} else {
			cur++
		}
	}
	if cur > maxLen {
		maxLen = cur
	}
	return maxLen
}

// isGeneratedOrMinified reports whether a file should skip the AST
// attempt: machine-written, declaration-only, or minified files parse
// slowly and their chunks are not read for comprehension anyway. The
// consequence is fixed windows (fail-open), never exclusion from BM25 search.
func isGeneratedOrMinified(relPath, text string, maxLineLen int) bool {
	lp := strings.ToLower(strings.TrimPrefix(strings.ReplaceAll(relPath, "\\", "/"), "./"))
	if lp == "public/assets" || strings.HasPrefix(lp, "public/assets/") || strings.Contains(lp, "/public/assets/") {
		return true
	}
	base := lp
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if strings.Contains(base, "generated") {
		return true
	}
	for _, suf := range []string{".pb.go", ".min.js", ".min.mjs", ".min.cjs", ".bundle.js", ".d.ts", ".d.mts", ".d.cts"} {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	if maxLineLen > 8192 {
		return true
	}
	head := text
	if len(head) > 2048 {
		head = head[:2048]
	}
	return strings.Contains(head, "Code generated") ||
		strings.Contains(head, "@generated") ||
		strings.Contains(head, "DO NOT EDIT") ||
		strings.Contains(head, "/* @ts-self-types=") ||
		strings.Contains(head, "this.__wbg_ptr") ||
		strings.Contains(head, "__wbindgen_") ||
		strings.Contains(head, "wasm.__wbg_") ||
		strings.Contains(head, "Generated by ") ||
		strings.Contains(head, "auto-generated") ||
		strings.Contains(head, "Auto-generated")
}

// maxDocLines bounds doc-comment attachment: a doc comment is usually
// a few lines; without a cap a file header would be swallowed whole by
// the first definition.
const maxDocLines = 20

// docMarkers returns the doc-comment markers whose contiguous block
// directly above a definition counts as its doc comment. Only real doc
// syntax attaches: Rust /// and block comments (plain // above a fn is
// an ordinary comment, not documentation); JS/TS JSDoc blocks; Go //
// (Go has no other doc syntax); Python attaches nothing (its
// docstrings live inside the definition range already). A wall of
// ordinary // lines above a function must not be swallowed as "docs".
func docMarkers(ext string) []string {
	switch ext {
	case ".py":
		return nil
	case ".rs":
		return []string{"///", "/**", "/*", "*"}
	case ".go":
		return []string{"//", "/*", "*"}
	case ".kt":
		// Kotlin KDoc is /** ... */ like JSDoc.
		return []string{"/**", "/*", "*"}
	default: // JS/TS family
		return []string{"/**", "/*", "*"}
	}
}

// attachDocComments extends each definition upward over directly
// adjacent doc-comment lines: the doc comment belongs to the definition
// below it. A blank line or non-doc line stops the extension. Extension
// can only claim doc lines, and a definition's own code lines act
// as barriers, so no doc line is ever claimed twice and def order
// is preserved.
func attachDocComments(defs []definition, lines []string, markers []string) []definition {
	isComment := func(t string) bool {
		for _, m := range markers {
			if strings.HasPrefix(t, m) {
				return true
			}
		}
		return false
	}
	for i := range defs {
		s := defs[i].startLine
		for n := 0; n < maxDocLines && s >= 2; n++ {
			if !isComment(strings.TrimSpace(lines[s-2])) {
				break
			}
			s--
		}
		defs[i].startLine = s
	}
	return defs
}

// chunkFile is the single chunking entry point: AST definitions where
// supported, fixed windows everywhere else (fail-open). The AST attempt is
// time-boxed: a pathological file never stalls the search.
// Returns the chunks and whether the AST path was taken (for K7 visibility).
func chunkFile(relPath, text string) ([]chunk, bool) {
	if defs, ok := astDefinitionsTimed(relPath, text); ok {
		return defChunks(text, defs), true
	}
	return chunkLines(text), false
}

// runTimed runs fn with a time budget, reporting whether it finished in
// time. On expiry it returns false immediately; fn's goroutine keeps
// running to completion in the background (goroutines can't be killed),
// so fn must not touch shared mutable state — it only costs background
// CPU, never user-visible latency. Shared by chunking, callers, and
// related scans so a pathological file stalls at most one budget.
func runTimed(timeout time.Duration, fn func()) bool {
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// astDefinitionsTimed runs astDefinitions with a time budget, failing open
// (ok=false, caller falls back to chunkLines) when the budget expires.
func astDefinitionsTimed(relPath, text string) (defs []definition, ok bool) {
	var out struct {
		defs []definition
		ok   bool
	}
	if !runTimed(astTimeout, func() {
		out.defs, out.ok = astDefinitions(relPath, text)
	}) {
		return nil, false
	}
	return out.defs, out.ok
}

// defChunks builds one chunk per definition (splitting long ones with
// overlap) plus fixed windows over lines no definition covers.
// Function-like definitions additionally yield a signature chunk (doc
// comment + declaration header) emitted before the body chunk, so the
// two share a start line and stable sort keeps the signature first.
func defChunks(text string, defs []definition) []chunk {
	norm := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(norm, "\n")
	n := len(lines)
	covered := make([]bool, n+1) // 1-based
	var out []chunk
	emit := func(s, e int, k chunkKind, name string) {
		if s < 1 {
			s = 1
		}
		if e > n {
			e = n
		}
		if s > e {
			return
		}
		out = append(out, chunk{startLine: s, lines: lines[s-1 : e], kind: k, name: name})
		for l := s; l <= e; l++ {
			covered[l] = true
		}
	}
	for _, d := range defs {
		if d.sigEnd > 0 && d.startLine <= d.sigEnd && d.sigEnd < d.endLine {
			emit(d.startLine, d.sigEnd, kindSig, d.name)
		}
		if d.endLine-d.startLine+1 > maxDefLines {
			for ws := d.startLine; ws <= d.endLine; {
				we := ws + maxDefLines - 1
				if we > d.endLine {
					we = d.endLine
				}
				emit(ws, we, kindBody, d.name)
				if we == d.endLine {
					break
				}
				ws += defSplitStride
			}
		} else {
			emit(d.startLine, d.endLine, kindBody, d.name)
		}
	}
	// gap fill: fixed windows over uncovered line ranges
	for i := 1; i <= n; {
		if covered[i] {
			i++
			continue
		}
		j := i
		for j <= n && !covered[j] {
			j++
		}
		for _, c := range chunkLines(strings.Join(lines[i-1:j-1], "\n")) {
			emit(i+c.startLine-1, i+c.startLine-1+len(c.lines)-1, kindWindow, "")
		}
		i = j
	}
	// Deterministic line order (matters for unranked literal search).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].startLine < out[j].startLine
	})
	return out
}
