// --related: cross-file symbol units. A fresh AST scan (like --callers,
// no BM25 index) that answers "where does NAME really live": local
// definitions, JS/TS re-export chains (export ... from, export * from,
// local export {..} lists), and the Rust<->JS wasm_bindgen bridge.
//
// Design notes (K14, 2026-10-26):
//   - Export statements are captured whole ((export_statement) @stmt) and
//     classified by text parsing. This avoids depending on the exact
//     tree-sitter child shapes for export_clause/export_specifier, which
//     vary between grammar versions.
//   - Rust bindgen detection is positional: a function is bridged when a
//     #[wasm_bindgen] attribute sits in the contiguous attribute block
//     immediately preceding it (whitespace/qualifiers between attrs and
//     the fn only). Methods (functions inside impl_item ranges) are
//     excluded in v1: their JS names collide with plain functions.
//   - The bridge name mapping follows wasm-bindgen's default (heck case
//     conversion); an explicit js_name attribute overrides it. The
//     generated .d.ts is NOT parsed (often gitignored/absent); the Rust
//     source plus the naming convention is the source of truth.
//   - v1 limits: JS/TS defs are the jsDefQuery set (functions, methods,
//     classes, arrows, function expressions) plus exported const/let/var
//     declarations and top-level const/let/var (declarators outside any
//     def range; a heuristic — top-level blocks/loops may leak in).
//     Only top-level Rust fns and methods of #[wasm_bindgen] impl
//     blocks are bridged.
//   - Kotlin (K4, 2026-11-12): local definitions from ktDefQuery
//     (classes, functions, objects, type aliases) plus top-level
//     properties (vals outside any def range). No re-export edges
//     (Kotlin has no re-export syntax) and no JNI bridge yet
//     (external fun <-> Java_... is a later stage).

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// relDef is one local definition: file, 1-based line, plain name.
type relDef struct {
	Path string
	Line int
	Name string
}

// relEdge is one re-export statement. Target is the module specifier as
// written ("" for a local export {..} list with no from-clause). Names
// maps exported name -> local name in the target; empty Names with
// Star=false means a local list (resolved against the same file).
// Star=true means export * from (every name passes through).
type relEdge struct {
	From   string
	Line   int
	Target string
	Star   bool
	Names  map[string]string
}

// relBridge is one #[wasm_bindgen] Rust function and the JS name it
// generates. Class is "" for top-level functions, or the impl type for
// methods (v2: an impl block with #[wasm_bindgen] bridges its methods
// as JS class methods).
type relBridge struct {
	RustPath string
	RustLine int
	RustName string
	JSName   string
	Class    string
}

// relEntry is one resolved output row.
type relEntry struct {
	Path   string
	Line   int
	Kind   string // "def", "re-export", "ffi"
	Detail string
	Symbol string // the queried name this row answers (batch attribution)
}

type relEntryJSON struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	Symbol string `json:"symbol"`
}

type relatedEnvelope struct {
	Query   string         `json:"query"`
	Batch   bool           `json:"batch,omitempty"`
	Total   int            `json:"total"`
	Related []relEntryJSON `json:"related"`
}

var relJSExts = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true,
	".ts": true, ".mts": true, ".cts": true,
	".tsx": true, ".jsx": true,
}

var relJSLangs = []callLang{
	{".js", grammars.JavascriptLanguage, `[(export_statement) @stmt]`},
	{".mjs", grammars.JavascriptLanguage, `[(export_statement) @stmt]`},
	{".cjs", grammars.JavascriptLanguage, `[(export_statement) @stmt]`},
	{".ts", grammars.TypescriptLanguage, `[(export_statement) @stmt]`},
	{".mts", grammars.TypescriptLanguage, `[(export_statement) @stmt]`},
	{".cts", grammars.TypescriptLanguage, `[(export_statement) @stmt]`},
	{".tsx", grammars.TsxLanguage, `[(export_statement) @stmt]`},
	{".jsx", grammars.TsxLanguage, `[(export_statement) @stmt]`},
}

// relVarLangs captures every variable declarator; extractRelatedJS
// keeps only those outside any def range (top-level const/let/var).
// Locals inside functions are skipped by the range filter, not by the
// query (tree-sitter queries cannot express "top level" directly).
var relVarLangs = []callLang{
	{".js", grammars.JavascriptLanguage, `[(variable_declarator name: (identifier) @name) @decl]`},
	{".mjs", grammars.JavascriptLanguage, `[(variable_declarator name: (identifier) @name) @decl]`},
	{".cjs", grammars.JavascriptLanguage, `[(variable_declarator name: (identifier) @name) @decl]`},
	{".ts", grammars.TypescriptLanguage, `[(variable_declarator name: (identifier) @name) @decl]`},
	{".mts", grammars.TypescriptLanguage, `[(variable_declarator name: (identifier) @name) @decl]`},
	{".cts", grammars.TypescriptLanguage, `[(variable_declarator name: (identifier) @name) @decl]`},
	{".tsx", grammars.TsxLanguage, `[(variable_declarator name: (identifier) @name) @decl]`},
	{".jsx", grammars.TsxLanguage, `[(variable_declarator name: (identifier) @name) @decl]`},
}

var (
	relExportSpecStore = newSpecStore[callSpec]()
	relVarSpecStore    = newSpecStore[callSpec]()
	relKtVarSpecStore  = newSpecStore[callSpec]()
)

// relKtVarLangs captures every Kotlin property declaration; the
// extractRelatedKotlin range filter keeps only those outside any def
// range (top-level vals), mirroring relVarLangs for JS/TS. Member
// properties sit inside their class def range and are skipped, the
// same way JS class fields are not reported.
var relKtVarLangs = []callLang{
	{".kt", grammars.KotlinLanguage, `[(property_declaration (variable_declaration (simple_identifier) @name)) @decl]`},
}

func getRelKtVarSpec(ext string) (*callSpec, bool) {
	return relKtVarSpecStore.get(ext, func() (*callSpec, bool) {
		return compileCallSpec(lookupCallLang(relKtVarLangs, ext))
	})
}

var relRustQuery = `[
	(attribute_item) @attr
	(function_item name: (identifier) @name) @fn
	(impl_item type: (_) @implType) @impl
]`

func getRelExportSpec(ext string) (*callSpec, bool) {
	return relExportSpecStore.get(ext, func() (*callSpec, bool) {
		return compileCallSpec(lookupCallLang(relJSLangs, ext))
	})
}

func getRelVarSpec(ext string) (*callSpec, bool) {
	return relVarSpecStore.get(ext, func() (*callSpec, bool) {
		return compileCallSpec(lookupCallLang(relVarLangs, ext))
	})
}

// The Rust spec has no extension key (a single query), so it uses the
// same store with a fixed key — not a seventh hand-rolled cache.
var relRustSpecStore = newSpecStore[callSpec]()

func getRelRustSpec() (*callSpec, bool) {
	return relRustSpecStore.get("", func() (*callSpec, bool) {
		lang := grammars.RustLanguage()
		if lang == nil {
			return nil, false
		}
		q, err := gotreesitter.NewQuery(relRustQuery, lang)
		if err != nil {
			return nil, false
		}
		return &callSpec{lang: lang, query: q}, true
	})
}

// lineIndex maps byte offsets to 1-based line numbers for norm text.
type lineIndex struct {
	starts []int
}

func newLineIndex(src []byte) lineIndex {
	li := lineIndex{starts: []int{0}}
	for i, b := range src {
		if b == '\n' {
			li.starts = append(li.starts, i+1)
		}
	}
	return li
}

func (li lineIndex) lineOf(byteOff uint32) int {
	i := sort.Search(len(li.starts), func(i int) bool {
		return li.starts[i] > int(byteOff)
	})
	return i
}

var (
	reFromClause = regexp.MustCompile(`from\s+['"]([^'"]+)['"]`)
	reExportFn   = regexp.MustCompile(`^export\s+(?:default\s+)?(?:async\s+)?function\s*\*?\s*([A-Za-z_$][A-Za-z0-9_$]*)`)
	reExportCls  = regexp.MustCompile(`^export\s+(?:default\s+)?class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	reExportVar  = regexp.MustCompile(`^export\s+(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
)

// parseBraceNames parses the {..} list of an export statement into
// exported -> local names ("a as b" maps b -> a; "type " prefixes are
// stripped). Shared by the named re-export and local export-list forms
// of parseExportStmt — they differed only in the edge Target, never in
// list parsing. ok=false when the braces are missing/misordered or the
// list is empty.
func parseBraceNames(t string) (map[string]string, bool) {
	open := strings.Index(t, "{")
	close := strings.LastIndex(t, "}")
	if open < 0 || close < 0 || close < open {
		return nil, false
	}
	names := map[string]string{}
	for _, item := range strings.Split(t[open+1:close], ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		item = strings.TrimPrefix(item, "type ")
		parts := strings.Split(item, " as ")
		if len(parts) == 2 {
			names[strings.TrimSpace(parts[1])] = strings.TrimSpace(parts[0])
		} else {
			names[item] = item
		}
	}
	if len(names) == 0 {
		return nil, false
	}
	return names, true
}

// parseExportStmt classifies one export statement's text into an edge
// and/or a local const def. Returns (edge, edgeOK, def, defOK).
func parseExportStmt(from, text string, line int) (relEdge, bool, relDef, bool) {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "export *") {
		m := reFromClause.FindStringSubmatch(t)
		if m == nil {
			return relEdge{}, false, relDef{}, false
		}
		return relEdge{From: from, Line: line, Target: m[1], Star: true}, true, relDef{}, false
	}
	if m := reFromClause.FindStringSubmatch(t); m != nil {
		// Named re-export: parse the {..} list.
		names, ok := parseBraceNames(t)
		if !ok {
			return relEdge{}, false, relDef{}, false
		}
		return relEdge{From: from, Line: line, Target: m[1], Names: names}, true, relDef{}, false
	}
	if strings.HasPrefix(t, "export {") {
		// Local export list: names defined in this same file.
		names, ok := parseBraceNames(t)
		if !ok {
			return relEdge{}, false, relDef{}, false
		}
		return relEdge{From: from, Line: line, Target: "", Names: names}, true, relDef{}, false
	}
	// Declaration form: function/class defs are already captured by the
	// def query; only const/let/var needs recording here.
	if m := reExportVar.FindStringSubmatch(t); m != nil {
		return relEdge{}, false, relDef{Path: from, Line: line, Name: m[1]}, true
	}
	return relEdge{}, false, relDef{}, false
}

// extractRelated parses one file and returns its defs, re-export edges
// and bindgen bridges. ok=false when the extension is unsupported, the
// file is too large, or parsing fails.
func extractRelated(relPath, text string) ([]relDef, []relEdge, []relBridge, bool) {
	ext := extOf(relPath)
	if len(text) > maxASTBytes {
		return nil, nil, nil, false
	}
	norm := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	src := []byte(norm)
	li := newLineIndex(src)
	if relJSExts[ext] {
		return extractRelatedJS(relPath, src, li)
	}
	if ext == ".rs" {
		return extractRelatedRust(relPath, src, li)
	}
	if ext == ".kt" {
		return extractRelatedKotlin(relPath, src, li)
	}
	return nil, nil, nil, false
}

// extractRelatedKotlin returns Kotlin definitions (classes, functions,
// objects, type aliases — the shared ktDefQuery) plus top-level
// properties (vals outside every def range). Kotlin has no re-export
// syntax, so there are no edges; the Kotlin<->Rust JNI bridge
// (external fun <-> Java_...) is a later stage, not v1.
func extractRelatedKotlin(relPath string, src []byte, li lineIndex) ([]relDef, []relEdge, []relBridge, bool) {
	ext := extOf(relPath)
	spec, ok := getSpec(ext)
	if !ok {
		return nil, nil, nil, false
	}
	dq := spec.query
	parser := gotreesitter.NewParser(spec.lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, nil, nil, false
	}
	var defs []relDef
	var defRanges [][2]uint32
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
		if !found || name == "" {
			continue
		}
		defs = append(defs, relDef{Path: relPath, Line: li.lineOf(sb), Name: name})
		defRanges = append(defRanges, [2]uint32{sb, eb})
	}
	// Top-level properties: declarators outside every def range.
	// (Member properties sit inside their class def range and are
	// skipped, like JS class fields.)
	if vs, ok := getRelKtVarSpec(ext); ok {
		vcursor := vs.query.Exec(tree.RootNode(), vs.lang, src)
		for {
			m, ok := vcursor.NextMatch()
			if !ok {
				break
			}
			var sb uint32
			var name string
			found := false
			for _, c := range m.Captures {
				if c.Name == "decl" {
					sb = c.Node.StartByte()
					found = true
				}
				if c.Name == "name" {
					name = string(src[c.Node.StartByte():c.Node.EndByte()])
				}
			}
			if !found || name == "" {
				continue
			}
			inside := false
			for _, r := range defRanges {
				if sb >= r[0] && sb < r[1] {
					inside = true
					break
				}
			}
			if inside {
				continue
			}
			defs = append(defs, relDef{Path: relPath, Line: li.lineOf(sb), Name: name})
		}
	}
	seen := map[string]bool{}
	uniq := defs[:0]
	for _, d := range defs {
		k := fmt.Sprintf("%s:%d:%s", d.Path, d.Line, d.Name)
		if seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, d)
	}
	return uniq, nil, nil, true
}

func extractRelatedJS(relPath string, src []byte, li lineIndex) ([]relDef, []relEdge, []relBridge, bool) {
	ext := extOf(relPath)
	// Defs from the shared def query (functions, methods, classes,
	// arrows, function expressions).
	dq, ok := defQueryFor(ext)
	if !ok {
		return nil, nil, nil, false
	}
	eq, ok := getRelExportSpec(ext)
	if !ok {
		return nil, nil, nil, false
	}
	parser := gotreesitter.NewParser(eq.lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, nil, nil, false
	}
	var defs []relDef
	var defRanges [][2]uint32
	dcursor := dq.Exec(tree.RootNode(), eq.lang, src)
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
		if !found || name == "" {
			continue
		}
		defs = append(defs, relDef{Path: relPath, Line: li.lineOf(sb), Name: name})
		defRanges = append(defRanges, [2]uint32{sb, eb})
	}
	var edges []relEdge
	ecursor := eq.query.Exec(tree.RootNode(), eq.lang, src)
	for {
		m, ok := ecursor.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			if c.Name != "stmt" {
				continue
			}
			st := string(src[c.Node.StartByte():c.Node.EndByte()])
			line := li.lineOf(c.Node.StartByte())
			edge, edgeOK, def, defOK := parseExportStmt(relPath, st, line)
			if edgeOK {
				edges = append(edges, edge)
			}
			if defOK {
				defs = append(defs, def)
			}
		}
	}
	// Top-level const/let/var: declarators outside every def range.
	// (Arrow-function declarators are defs themselves: their start
	// equals a def range start, so the inclusive-start check skips
	// them here; they are already in defs via the def query.)
	if vs, ok := getRelVarSpec(ext); ok {
		vcursor := vs.query.Exec(tree.RootNode(), vs.lang, src)
		for {
			m, ok := vcursor.NextMatch()
			if !ok {
				break
			}
			var sb uint32
			var name string
			found := false
			for _, c := range m.Captures {
				if c.Name == "decl" {
					sb = c.Node.StartByte()
					found = true
				}
				if c.Name == "name" {
					name = string(src[c.Node.StartByte():c.Node.EndByte()])
				}
			}
			if !found || name == "" {
				continue
			}
			inside := false
			for _, r := range defRanges {
				if sb >= r[0] && sb < r[1] {
					inside = true
					break
				}
			}
			if inside {
				continue
			}
			defs = append(defs, relDef{Path: relPath, Line: li.lineOf(sb), Name: name})
		}
	}
	// Dedup: export const appears both via parseExportStmt and via the
	// declarator query (same path, line, name).
	seen := map[string]bool{}
	uniq := defs[:0]
	for _, d := range defs {
		k := fmt.Sprintf("%s:%d:%s", d.Path, d.Line, d.Name)
		if seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, d)
	}
	return uniq, edges, nil, true
}

var reJSName = regexp.MustCompile(`js_name\s*=\s*"([^"]+)"`)

// attrApplies reports whether the attribute block ending at attrEnd
// (with the given following attributes up to fnStart) belongs to the
// function starting at fnStart. The gap right before the fn must be
// exactly a qualifier sequence (whitespace + pub/async/unsafe/extern);
// anything else (another item's text, e.g. a whole preceding function)
// breaks the chain. Gaps between stacked attributes must be whitespace
// only. Comments between an attribute and its item break the chain
// (rare in real code, fail-closed).
func attrChainBindgen(src []byte, attrs [][2]uint32, fnStart uint32) (bool, string) {
	// attrs sorted by start. Walk backwards from fnStart through the
	// contiguous attribute block.
	next := fnStart
	first := true
	found := false
	jsName := ""
	for i := len(attrs) - 1; i >= 0; i-- {
		aStart, aEnd := attrs[i][0], attrs[i][1]
		if aEnd > next {
			continue
		}
		gap := string(src[aEnd:next])
		if first {
			// Compact all whitespace, then require an exact qualifier
			// sequence. "pub fn foo(){}..." compacts to something
			// never in this set, so an intervening item breaks the
			// chain instead of passing as a qualifier.
			compact := strings.Map(func(r rune) rune {
				if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
					return -1
				}
				return r
			}, gap)
			switch compact {
			case "", "pub", "pub(crate)", "async", "unsafe",
				"pubasync", "pubunsafe", "asyncunsafe", "pubasyncunsafe",
				`extern"C"`, `pubextern"C"`:
			default:
				return found, jsName
			}
			first = false
		} else {
			if strings.TrimSpace(gap) != "" {
				break
			}
		}
		attrText := string(src[aStart:aEnd])
		if strings.Contains(attrText, "wasm_bindgen") {
			found = true
			if m := reJSName.FindStringSubmatch(attrText); m != nil {
				jsName = m[1]
			}
		}
		next = aStart
	}
	return found, jsName
}

// baseTypeName reduces an impl type to its bare name: Foo<T> -> Foo,
// a::Foo -> Foo.
func baseTypeName(t string) string {
	if i := strings.Index(t, "<"); i >= 0 {
		t = t[:i]
	}
	if i := strings.LastIndex(t, "::"); i >= 0 {
		t = t[i+2:]
	}
	return strings.TrimSpace(t)
}

func extractRelatedRust(relPath string, src []byte, li lineIndex) ([]relDef, []relEdge, []relBridge, bool) {
	spec, ok := getRelRustSpec()
	if !ok {
		return nil, nil, nil, false
	}
	parser := gotreesitter.NewParser(spec.lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, nil, nil, false
	}
	type fnRec struct {
		start, end uint32
		name       string
	}
	type implRec struct {
		start, end uint32
		typeName   string
	}
	var attrs [][2]uint32
	var fns []fnRec
	var impls []implRec
	cursor := spec.query.Exec(tree.RootNode(), spec.lang, src)
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var fnStart, fnEnd uint32
		var fnName string
		var implStart, implEnd uint32
		var implType string
		var attrStart, attrEnd uint32
		kind := ""
		for _, c := range m.Captures {
			switch c.Name {
			case "fn":
				fnStart, fnEnd = c.Node.StartByte(), c.Node.EndByte()
				kind = "fn"
			case "name":
				fnName = string(src[c.Node.StartByte():c.Node.EndByte()])
			case "impl":
				implStart, implEnd = c.Node.StartByte(), c.Node.EndByte()
				kind = "impl"
			case "implType":
				implType = string(src[c.Node.StartByte():c.Node.EndByte()])
			case "attr":
				attrStart, attrEnd = c.Node.StartByte(), c.Node.EndByte()
				kind = "attr"
			}
		}
		switch kind {
		case "fn":
			if fnName != "" {
				fns = append(fns, fnRec{start: fnStart, end: fnEnd, name: fnName})
			}
		case "impl":
			impls = append(impls, implRec{start: implStart, end: implEnd, typeName: baseTypeName(implType)})
		case "attr":
			attrs = append(attrs, [2]uint32{attrStart, attrEnd})
		}
	}
	sort.Slice(attrs, func(i, j int) bool { return attrs[i][0] < attrs[j][0] })
	var bridges []relBridge
	for _, fn := range fns {
		// Inside an impl range: a method. Bridged only when the impl
		// block itself carries #[wasm_bindgen] (the documented bindgen
		// pattern; a struct-level attribute alone does not expose
		// methods). The JS name is Class.method.
		var enclosing *implRec
		for i := range impls {
			if fn.start >= impls[i].start && fn.end <= impls[i].end {
				enclosing = &impls[i]
				break
			}
		}
		if enclosing != nil {
			isBindgen, _ := attrChainBindgen(src, attrs, enclosing.start)
			if !isBindgen || enclosing.typeName == "" {
				continue
			}
			bridges = append(bridges, relBridge{
				RustPath: relPath, RustLine: li.lineOf(fn.start),
				RustName: fn.name, JSName: toCamel(fn.name), Class: enclosing.typeName,
			})
			continue
		}
		isBindgen, jsName := attrChainBindgen(src, attrs, fn.start)
		if !isBindgen {
			continue
		}
		if jsName == "" {
			jsName = toCamel(fn.name)
		}
		bridges = append(bridges, relBridge{
			RustPath: relPath, RustLine: li.lineOf(fn.start),
			RustName: fn.name, JSName: jsName,
		})
	}
	return nil, nil, bridges, true
}

// --- case conversion (heck-compatible for common names) ---

// splitWords splits one separator-free segment into words: boundaries
// on lower/digit -> upper transitions and on acronym runs
// (URLValue -> URL, Value).
func splitWords(s string) []string {
	var words []string
	rs := []rune(s)
	start := 0
	isUpper := func(r rune) bool { return r >= 'A' && r <= 'Z' }
	isLower := func(r rune) bool { return r >= 'a' && r <= 'z' }
	isDigit := func(r rune) bool { return r >= '0' && r <= '9' }
	for i := 1; i < len(rs); i++ {
		prev, cur := rs[i-1], rs[i]
		boundary := false
		if (isLower(prev) || isDigit(prev)) && isUpper(cur) {
			boundary = true
		} else if isUpper(prev) && isUpper(cur) && i+1 < len(rs) && isLower(rs[i+1]) {
			// Acronym run ends: the current upper starts a new word.
			boundary = true
		}
		if boundary {
			words = append(words, string(rs[start:i]))
			start = i
		}
	}
	words = append(words, string(rs[start:]))
	var out []string
	for _, w := range words {
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

// splitIdent splits an identifier on _/- separators, then on case
// boundaries within each segment.
func splitIdent(s string) []string {
	var words []string
	for _, seg := range strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' }) {
		words = append(words, splitWords(seg)...)
	}
	return words
}

func toCamel(s string) string {
	words := splitIdent(s)
	for i, w := range words {
		lw := strings.ToLower(w)
		if i == 0 {
			words[i] = lw
		} else {
			words[i] = strings.ToUpper(lw[:1]) + lw[1:]
		}
	}
	return strings.Join(words, "")
}

func toSnake(s string) string {
	words := splitIdent(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}

// --- import resolution ---

var relImportExts = []string{".js", ".mjs", ".cjs", ".ts", ".mts", ".cts", ".tsx", ".jsx"}

// resolveImport maps a module specifier written in fromFile to an
// indexed file path, or "" when it cannot be resolved. Only relative
// specifiers are resolved (bare package names are out of scope).
func resolveImport(fromFile, spec string, files map[string]bool) string {
	if !strings.HasPrefix(spec, ".") {
		return ""
	}
	base := path.Clean(path.Join(path.Dir(fromFile), spec))
	if files[base] {
		return base
	}
	for _, ext := range relImportExts {
		if files[base+ext] {
			return base + ext
		}
	}
	for _, ext := range relImportExts {
		if files[base+"/index"+ext] {
			return base + "/index" + ext
		}
	}
	return ""
}

// --- resolution ---

// resolveRelated computes every row for query: local defs, the full
// re-export chain (cycle-safe), and the wasm_bindgen bridge if any.
func resolveRelated(query string, defs []relDef, edges []relEdge, bridges []relBridge, files map[string]bool) []relEntry {
	want := calleeBase(query)
	defByFile := map[string][]relDef{}
	for _, d := range defs {
		defByFile[d.Path] = append(defByFile[d.Path], d)
	}
	findDef := func(file, name string) (relDef, bool) {
		for _, d := range defByFile[file] {
			if d.Name == name {
				return d, true
			}
		}
		return relDef{}, false
	}
	var entries []relEntry
	seen := map[string]bool{}
	add := func(e relEntry) {
		e.Symbol = want
		k := fmt.Sprintf("%s:%d:%s:%s", e.Path, e.Line, e.Kind, e.Detail)
		if !seen[k] {
			seen[k] = true
			entries = append(entries, e)
		}
	}
	for _, d := range defs {
		if d.Name == want {
			add(relEntry{Path: d.Path, Line: d.Line, Kind: "def",
				Detail: fmt.Sprintf("%s defined here", d.Name)})
		}
	}
	// F2 phase 1/2: re-export rows are collected, not emitted, until
	// liveness is known. A row is signal only when its chain ends at
	// a found definition; rows on dead-end chains are noise — with a
	// barrel of 20 `export *`, every star edge "exports" the name, so
	// emitting eagerly produces 20 "(definition not found there)"
	// rows even though the def sits in exactly one target.
	// traceRow.node is the "file\x00name" the row points at ("" when
	// the target is unknown); succ maps a node to the nodes its
	// re-export edges point at, for the liveness fixpoint below.
	type traceRow struct {
		entry relEntry
		node  string
	}
	var trace []traceRow
	succ := map[string][]string{}
	nkey := func(file, name string) string { return file + "\x00" + name }
	// traceSeen is separate from `seen`: a trace row is deduped at
	// collection time, but must NOT count as already-emitted — phase 2
	// emits through `add`, which skips keys in `seen`. Sharing the map
	// would silently drop every trace row at emission.
	traceSeen := map[string]bool{}
	traceAdd := func(e relEntry, node string) {
		e.Symbol = want
		k := fmt.Sprintf("%s:%d:%s:%s", e.Path, e.Line, e.Kind, e.Detail)
		if !traceSeen[k] {
			traceSeen[k] = true
			trace = append(trace, traceRow{e, node})
		}
	}
	// Re-export walk: queue of (file, exportedName) to explain.
	type task struct {
		file, name string
	}
	visited := map[string]bool{}
	queue := []task{}
	// processEdge explains one edge that exports exportedName: it
	// records the row and, when the target does not define the name
	// itself, queues the target for further chain following.
	processEdge := func(e relEdge, exportedName string) {
		local := exportedName
		if !e.Star {
			ln, ok := e.Names[exportedName]
			if !ok {
				return
			}
			local = ln
		}
		target := e.From
		if e.Target != "" {
			target = resolveImport(e.From, e.Target, files)
		}
		if target == "" {
			traceAdd(relEntry{Path: e.From, Line: e.Line, Kind: "re-export",
				Detail: fmt.Sprintf("re-exports %s from %q (target not in index)", exportedName, e.Target)}, "")
			return
		}
		node := nkey(target, local)
		succ[nkey(e.From, exportedName)] = append(succ[nkey(e.From, exportedName)], node)
		if d, ok := findDef(target, local); ok {
			traceAdd(relEntry{Path: e.From, Line: e.Line, Kind: "re-export",
				Detail: fmt.Sprintf("re-exports %s from %s (defined %s:%d)", exportedName, target, target, d.Line)}, node)
			return
		}
		traceAdd(relEntry{Path: e.From, Line: e.Line, Kind: "re-export",
			Detail: fmt.Sprintf("re-exports %s via %s (definition not found there)", exportedName, target)}, node)
		if !visited[node] {
			visited[node] = true
			queue = append(queue, task{target, local})
		}
	}
	// Seed: every edge that exports `want` (as its exported name).
	for _, e := range edges {
		if e.Star {
			processEdge(e, want)
		} else if _, ok := e.Names[want]; ok {
			processEdge(e, want)
		}
	}
	// Continue chains: an intermediate file that re-exports the name
	// onward gets its own row, then we follow it.
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		for _, e := range edges {
			if e.From != t.file {
				continue
			}
			if e.Star {
				processEdge(e, t.name)
			} else if _, ok := e.Names[t.name]; ok {
				processEdge(e, t.name)
			}
		}
	}
	// F2 phase 2: emit only the trace rows on live chains. A node
	// (file, name) is live when the file defines the name as it
	// arrives there — the local name, which may differ from the
	// queried name through an alias (`export {orig as alias}`) — or
	// re-exports it onward to a live node; the fixpoint converges
	// even with re-export cycles (each pass only adds nodes).
	live := map[string]bool{}
	for _, d := range defs {
		live[nkey(d.Path, d.Name)] = true
	}
	for changed := true; changed; {
		changed = false
		for src, dsts := range succ {
			if live[src] {
				continue
			}
			for _, dst := range dsts {
				if live[dst] {
					live[src] = true
					changed = true
					break
				}
			}
		}
	}
	// When no chain reaches a definition of `want` and there is no
	// def row either, the trace is the only information ("looked
	// here, here, not found"), so it is kept whole instead of
	// emitting an empty answer.
	anyLive, hasDef := false, false
	for _, t := range trace {
		if t.node != "" && live[t.node] {
			anyLive = true
			break
		}
	}
	for _, d := range defs {
		if d.Name == want {
			hasDef = true
			break
		}
	}
	if !anyLive && !hasDef {
		for _, t := range trace {
			add(t.entry)
		}
	} else {
		for _, t := range trace {
			if t.node != "" && live[t.node] {
				add(t.entry)
			}
		}
	}
	// FFI bridge: the Rust side decides; the JS side may or may not be
	// in the repo (generated bindings usually are not). A bridge is
	// found from either side: by Rust name (default conversion or the
	// query's snake form) or by its JS name (covers js_name overrides,
	// whose JS name does not round-trip to the Rust name).
	rustName := toSnake(query)
	for _, b := range bridges {
		if b.RustName != rustName && b.JSName != query && b.JSName != toCamel(query) {
			continue
		}
		jsDef := ""
		for _, d := range defs {
			if d.Name == b.JSName {
				jsDef = fmt.Sprintf("%s:%d", d.Path, d.Line)
				break
			}
		}
		detail := fmt.Sprintf("rust %s -> js %s", b.RustName, b.JSName)
		if b.Class != "" {
			detail = fmt.Sprintf("rust %s::%s -> js %s.%s", b.Class, b.RustName, b.Class, b.JSName)
		}
		if jsDef != "" {
			detail += fmt.Sprintf(" (js def in repo: %s)", jsDef)
		} else {
			detail += " (js def not in repo: generated binding)"
		}
		add(relEntry{Path: b.RustPath, Line: b.RustLine, Kind: "ffi", Detail: detail})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		if entries[i].Line != entries[j].Line {
			return entries[i].Line < entries[j].Line
		}
		return entries[i].Kind < entries[j].Kind
	})
	return entries
}

// runRelated scans root fresh and prints every row for the queried
// symbol: local defs, re-export chains, and the wasm_bindgen bridge.
// Returns a process exit code.
func runRelated(o options, root string, cfg *ScanConfig) int {
	names := splitBatchNames(o.related)
	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "codesearch: --related requires a non-empty name")
		return 2
	}
	files := iterFiles(root, cfg)
	fileSet := map[string]bool{}
	for _, rel := range files {
		fileSet[rel] = true
	}
	var defs []relDef
	var edges []relEdge
	var bridges []relBridge
	// Skip taxonomy, mirroring extractRelated's own order: readSource
	// failure -> readSource skip -> AST oversize -> unsupported language
	// -> parse/spec failure or parse timeout. Buckets are disjoint;
	// parsed + skipped == len(files). Spec-build failures inside
	// extractRelatedJS/Rust land in parse-fail. The timeout bounds one
	// file, not the run (see the callers note on leaked goroutines).
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
		if len(fr.text) > maxASTBytes {
			astOversize++
			continue
		}
		if ext := extOf(rel); !relJSExts[ext] && ext != ".rs" && ext != ".kt" {
			unsupported++
			continue
		}
		var d []relDef
		var e []relEdge
		var b []relBridge
		var ok bool
		if !runTimed(astTimeout, func() {
			d, e, b, ok = extractRelated(rel, fr.text)
		}) {
			timedOut++
			continue
		}
		if !ok {
			parseFail++
			continue
		}
		parsed++
		defs = append(defs, d...)
		edges = append(edges, e...)
		bridges = append(bridges, b...)
	}
	skipped := unsupported + astOversize + parseFail + timedOut + sourceSkip + unreadable
	fmt.Fprintf(os.Stderr, "(related: parsed %d files, skipped %d (unsupported %d, ast-oversize %d, parse-fail %d, timeout %d, source-skip %d, unreadable %d) in %s)\n",
		parsed, skipped, unsupported, astOversize, parseFail, timedOut, sourceSkip, unreadable, time.Since(t0).Round(time.Millisecond))
	entries := []relEntry{}
	for _, name := range names {
		entries = append(entries, resolveRelated(name, defs, edges, bridges, fileSet)...)
	}
	if o.jsonOut {
		env := relatedEnvelope{Query: o.related, Batch: len(names) > 1, Total: len(entries), Related: []relEntryJSON{}}
		for _, en := range entries {
			env.Related = append(env.Related, relEntryJSON{
				Path: en.Path, Line: en.Line, Kind: en.Kind, Detail: en.Detail, Symbol: en.Symbol,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(env); err != nil {
			fmt.Fprintf(os.Stderr, "codesearch: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "%d related row(s)\n", len(entries))
		return 0
	}
	for _, en := range entries {
		fmt.Printf("%s:%d: %s: %s\n", en.Path, en.Line, en.Kind, en.Detail)
	}
	fmt.Fprintf(os.Stderr, "%d related row(s)\n", len(entries))
	return 0
}
