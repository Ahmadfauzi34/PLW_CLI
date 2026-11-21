// --related: cross-file symbol units. A fresh AST scan (like --callers,
// no BM25 index) that answers "where does NAME really live": local
// definitions, JS/TS re-export chains (export ... from, export * from,
// local export {..} lists), Rust re-export chains (pub use, incl.
// groups, aliases, globs, and crate::/self::/super:: module paths),
// and the Rust<->JS wasm_bindgen bridge.
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
//     (external fun <-> Java_... is a later stage, go14 P2).
//   - Rust defs (go14, P1-B): the chunking def query (functions,
//     trait signatures, structs, enums, traits, type aliases) plus
//     top-level const/static (outside every def range, mirroring the
//     JS top-level-var rule). impl_item defs are skipped: the def
//     query names an impl by its type, which would duplicate the
//     struct def. Rust re-exports are `pub use` declarations, parsed
//     from text like JS export statements; resolveRustImport maps
//     crate::/self::/super:: paths to files (mod.rs / <mod>.rs).

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
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
// as JS class methods). Kind is "wasm" (default) or "jni" (go14, P2):
// a JNI bridge is a Rust fn named Java_<pkg>_<Class>_<method> and its
// JSName/Class are empty — the Kotlin side lives in relJNIDecl and is
// matched at resolve time by base key, not stored here.
type relBridge struct {
	RustPath string
	RustLine int
	RustName string
	JSName   string
	Class    string
	Kind     string // "wasm" or "jni"
}

// relJNIDecl is one Kotlin `external fun` declaration: the JNI base key
// it maps to (Java_<pkg>_<Class>_<method>), its arity, and coarse type
// tokens per parameter ("" = unknown/wildcard: a custom class, a
// nullable, or anything the static indexer cannot know without a
// classpath). The Rust side is a relBridge with Kind "jni"; resolve
// matches decls to bridges by Base, then narrows by arity and coarse
// tokens (Option 3, go14 P2).
type relJNIDecl struct {
	Path   string
	Line   int
	Name   string // the Kotlin method name (send)
	Base   string // Java_com_example_Bridge_send
	Arity  int
	Coarse []string
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

// lineStart returns the byte offset at which line (1-based) starts,
// or -1 when the line is out of range.
func (li lineIndex) lineStart(line int) int {
	if line < 1 || line > len(li.starts) {
		return -1
	}
	return li.starts[line-1]
}

// extractRelated parses one file and returns its defs, re-export edges
// and bindgen bridges. ok=false when the extension is unsupported, the
// file is too large, or parsing fails.
func extractRelated(relPath, text string) ([]relDef, []relEdge, []relBridge, []relJNIDecl, bool) {
	ext := extOf(relPath)
	if len(text) > maxASTBytes {
		return nil, nil, nil, nil, false
	}
	norm := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if isGeneratedOrMinified(relPath, norm, maxLineLenOf(norm)) {
		return nil, nil, nil, nil, false
	}
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
	return nil, nil, nil, nil, false
}

// isIdentByte reports whether b can appear inside an identifier
// (ASCII letter, digit, _, $ — the boundary set for hasWord).
func isIdentByte(b byte) bool {
	return b == '_' || b == '$' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// hasWord reports whether text contains word as a whole word (bounded
// by non-identifier characters on both sides).
func hasWord(text, word string) bool {
	for i := 0; i+len(word) <= len(text); i++ {
		if text[i:i+len(word)] != word {
			continue
		}
		beforeOK := i == 0 || !isIdentByte(text[i-1])
		afterOK := i+len(word) == len(text) || !isIdentByte(text[i+len(word)])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
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

// --- resolution ---

// relIndex pre-indexes definitions, re-export edges, FFI bridges, and live
// nodes once so repeated relatedFor / batch --related queries run in O(1)
// when no re-export edges or bridges match the queried symbol.
type relIndex struct {
	defs       []relDef
	edges      []relEdge
	bridges    []relBridge
	jni        []relJNIDecl
	files      map[string]bool
	defByName  map[string][]relDef
	defByFile  map[string][]relDef
	starEdges  []relEdge
	edgesByExp map[string][]relEdge
	edgesBySrc map[string][]relEdge
	liveBase   map[string]bool
	byRustName map[string][]relBridge
	byJSName   map[string][]relBridge
	// JNI (go14, P2): decls by base key and by method name; bridges
	// (Kind "jni") by base key. The base is the join key between the
	// Kotlin decl and the Rust fn.
	jniByBase       map[string][]relJNIDecl
	jniByName       map[string][]relJNIDecl
	jniBridgeByBase map[string][]relBridge
}

func newRelIndex(defs []relDef, edges []relEdge, bridges []relBridge, jni []relJNIDecl, files map[string]bool) *relIndex {
	ri := &relIndex{
		defs:            defs,
		edges:           edges,
		bridges:         bridges,
		jni:             jni,
		files:           files,
		defByName:       make(map[string][]relDef, len(defs)),
		defByFile:       make(map[string][]relDef),
		edgesByExp:      make(map[string][]relEdge),
		edgesBySrc:      make(map[string][]relEdge),
		liveBase:        make(map[string]bool, len(defs)),
		byRustName:      make(map[string][]relBridge, len(bridges)),
		byJSName:        make(map[string][]relBridge, len(bridges)),
		jniByBase:       make(map[string][]relJNIDecl, len(jni)),
		jniByName:       make(map[string][]relJNIDecl, len(jni)),
		jniBridgeByBase: make(map[string][]relBridge),
	}
	for _, d := range defs {
		ri.defByName[d.Name] = append(ri.defByName[d.Name], d)
		ri.defByFile[d.Path] = append(ri.defByFile[d.Path], d)
		ri.liveBase[d.Path+"\x00"+d.Name] = true
	}
	for _, e := range edges {
		ri.edgesBySrc[e.From] = append(ri.edgesBySrc[e.From], e)
		if e.Star {
			ri.starEdges = append(ri.starEdges, e)
		} else {
			for expName := range e.Names {
				ri.edgesByExp[expName] = append(ri.edgesByExp[expName], e)
			}
		}
	}
	for _, b := range bridges {
		ri.byRustName[b.RustName] = append(ri.byRustName[b.RustName], b)
		if b.JSName != "" {
			ri.byJSName[b.JSName] = append(ri.byJSName[b.JSName], b)
		}
		if b.Kind == "jni" {
			if base, _, _, ok := parseJNIRustName(b.RustName); ok {
				ri.jniBridgeByBase[base] = append(ri.jniBridgeByBase[base], b)
			}
		}
	}
	for _, d := range jni {
		ri.jniByBase[d.Base] = append(ri.jniByBase[d.Base], d)
		ri.jniByName[d.Name] = append(ri.jniByName[d.Name], d)
	}
	return ri
}

func (ri *relIndex) findDef(file, name string) (relDef, bool) {
	for _, d := range ri.defByFile[file] {
		if d.Name == name {
			return d, true
		}
	}
	return relDef{}, false
}

// resolveRelated computes every row for query: local defs, the full
// re-export chain (cycle-safe), and the wasm_bindgen bridge if any.
func resolveRelated(query string, defs []relDef, edges []relEdge, bridges []relBridge, jni []relJNIDecl, files map[string]bool) []relEntry {
	return newRelIndex(defs, edges, bridges, jni, files).resolve(query)
}

func (ri *relIndex) resolve(query string) []relEntry {
	want := calleeBase(query)
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
	wantDefs := ri.defByName[want]
	for _, d := range wantDefs {
		add(relEntry{Path: d.Path, Line: d.Line, Kind: "def",
			Detail: fmt.Sprintf("%s defined here", d.Name)})
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
	if len(ri.starEdges) > 0 || len(ri.edgesByExp[want]) > 0 {
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
				// Rust edges carry module paths (crate::/self::/super::),
				// JS edges carry relative specifiers: different resolvers.
				if extOf(e.From) == ".rs" {
					target = resolveRustImport(e.From, e.Target, ri.files)
				} else {
					target = resolveImport(e.From, e.Target, ri.files)
				}
			}
			if target == "" {
				traceAdd(relEntry{Path: e.From, Line: e.Line, Kind: "re-export",
					Detail: fmt.Sprintf("re-exports %s from %q (target not in index)", exportedName, e.Target)}, "")
				return
			}
			node := nkey(target, local)
			succ[nkey(e.From, exportedName)] = append(succ[nkey(e.From, exportedName)], node)
			if d, ok := ri.findDef(target, local); ok {
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
		for _, e := range ri.edges {
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
			for _, e := range ri.edgesBySrc[t.file] {
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
		if len(trace) > 0 {
			live := map[string]bool{}
			isLive := func(node string) bool {
				return ri.liveBase[node] || live[node]
			}
			for changed := true; changed; {
				changed = false
				for src, dsts := range succ {
					if isLive(src) {
						continue
					}
					for _, dst := range dsts {
						if isLive(dst) {
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
			anyLive := false
			for _, t := range trace {
				if t.node != "" && isLive(t.node) {
					anyLive = true
					break
				}
			}
			hasDef := len(wantDefs) > 0
			if !anyLive && !hasDef {
				for _, t := range trace {
					add(t.entry)
				}
			} else {
				for _, t := range trace {
					if t.node != "" && isLive(t.node) {
						add(t.entry)
					}
				}
			}
		}
	}
	// FFI bridge: the Rust side decides; the JS side may or may not be
	// in the repo (generated bindings usually are not). A bridge is
	// found from either side: by Rust name (default conversion or the
	// query's snake form) or by its JS name (covers js_name overrides,
	// whose JS name does not round-trip to the Rust name).
	if len(ri.bridges) > 0 {
		rustName := toSnake(query)
		camelName := toCamel(query)
		if len(ri.byRustName[rustName]) > 0 || len(ri.byJSName[query]) > 0 || len(ri.byJSName[camelName]) > 0 {
			for _, b := range ri.bridges {
				if b.Kind == "jni" {
					continue // JNI bridges are matched below, by base key
				}
				if b.RustName != rustName && b.JSName != query && b.JSName != camelName {
					continue
				}
				jsDef := ""
				if jsDefs := ri.defByName[b.JSName]; len(jsDefs) > 0 {
					jsDef = fmt.Sprintf("%s:%d", jsDefs[0].Path, jsDefs[0].Line)
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
		}
	}
	// JNI bridge (go14, P2 — Option 3, Base-Key Grouping): a Rust
	// Java_<pkg>_<Class>_<method> fn implements a Kotlin external fun
	// of the same base key. Two directions:
	//   - Rust query (a full JNI name): the Kotlin decls for the base,
	//     narrowed by the signature's arity and coarse tokens when the
	//     query carries one; survivors that still collide are shown as
	//     overload candidates, never dropped (fail-open).
	//   - Kotlin query (a method name): every Rust overload for each
	//     decl's base, one row per Rust fn. A bridge shared by several
	//     overload decls is emitted once.
	if len(ri.jniByBase) > 0 || len(ri.jniBridgeByBase) > 0 {
		if qBase, qTokens, qOverloaded, qIsJNI := parseJNIRustName(want); qIsJNI {
			decls := ri.jniByBase[qBase]
			if qOverloaded {
				var f []relJNIDecl
				for _, d := range decls {
					if d.Arity == len(qTokens) && jniCoarseCompatible(d.Coarse, qTokens) {
						f = append(f, d)
					}
				}
				decls = f
			}
			ambiguous := len(decls) > 1
			for _, d := range decls {
				// The Rust fn this decl is bound to: an overloaded query
				// keeps its queried Rust name; a short query narrows the
				// base's bridges by arity + coarse tokens (a short-name
				// bridge carries no signature, so it matches any decl).
				rustName := qBase
				if qOverloaded {
					rustName = want
				} else {
					var matches []relBridge
					for _, b := range ri.jniBridgeByBase[qBase] {
						if _, toks, over, ok := parseJNIRustName(b.RustName); ok {
							if !over || (len(toks) == d.Arity && jniCoarseCompatible(d.Coarse, toks)) {
								matches = append(matches, b)
							}
						}
					}
					if len(matches) == 1 {
						rustName = matches[0].RustName
					}
				}
				detail := fmt.Sprintf("jni external fun %s (%d params) bound to %s", d.Name, d.Arity, rustName)
				if ambiguous {
					detail += " (overload candidate)"
				}
				add(relEntry{Path: d.Path, Line: d.Line, Kind: "ffi", Detail: detail})
			}
		} else if decls := ri.jniByName[want]; len(decls) > 0 {
			seenBridge := map[string]bool{}
			for _, d := range decls {
				bridges := ri.jniBridgeByBase[d.Base]
				overloads := len(ri.jniByBase[d.Base])
				for _, b := range bridges {
					k := fmt.Sprintf("%s:%d\x00%s", b.RustPath, b.RustLine, b.RustName)
					if seenBridge[k] {
						continue
					}
					seenBridge[k] = true
					detail := fmt.Sprintf("jni %s implements external fun %s", b.RustName, d.Name)
					if overloads == 1 {
						detail += fmt.Sprintf(" (%s:%d)", d.Path, d.Line)
					} else {
						detail += fmt.Sprintf(" (%d overloads)", overloads)
					}
					add(relEntry{Path: b.RustPath, Line: b.RustLine, Kind: "ffi", Detail: detail})
				}
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		if entries[i].Line != entries[j].Line {
			return entries[i].Line < entries[j].Line
		}
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].Detail < entries[j].Detail
	})
	return entries
}

// runRelated scans root (fresh by default, or cached index with --cached)
// and prints every row for the queried symbol: local defs, re-export
// chains, and the wasm_bindgen bridge. Returns a process exit code.
func runRelated(o options, root string, cfg *ScanConfig) int {
	names := splitBatchNames(o.related)
	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "codesearch: --related requires a non-empty name")
		return 2
	}
	var fileSet map[string]bool
	var defs []relDef
	var edges []relEdge
	var bridges []relBridge
	var jni []relJNIDecl
	if o.cached {
		ix, err := loadCachedIndex(o, root, cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "codesearch:", err)
			return 1
		}
		fileSet = make(map[string]bool, len(ix.files))
		for rel := range ix.files {
			fileSet[rel] = true
		}
		defs = ix.relDefs
		edges = ix.relEdges
		bridges = ix.relBridges
		jni = ix.relJNI
	} else {
		files := iterFiles(root, cfg)
		fileSet = map[string]bool{}
		for _, rel := range files {
			fileSet[rel] = true
		}
		// Skip taxonomy, mirroring extractRelated's own order: readSource
		// failure -> readSource skip -> AST oversize -> unsupported language
		// -> generated/minified skip -> parse/spec failure or parse timeout.
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
			norm := strings.ReplaceAll(strings.ReplaceAll(fr.text, "\r\n", "\n"), "\r", "\n")
			if isGeneratedOrMinified(rel, norm, maxLineLenOf(norm)) {
				sourceSkip++
				continue
			}
			var d []relDef
			var e []relEdge
			var b []relBridge
			var j []relJNIDecl
			var ok bool
			if !runTimed(astTimeout, func() {
				d, e, b, j, ok = extractRelated(rel, fr.text)
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
			jni = append(jni, j...)
		}
		skipped := unsupported + astOversize + parseFail + timedOut + sourceSkip + unreadable
		fmt.Fprintf(os.Stderr, "(related: parsed %d files, skipped %d (unsupported %d, ast-oversize %d, parse-fail %d, timeout %d, source-skip %d, unreadable %d) in %s)\n",
			parsed, skipped, unsupported, astOversize, parseFail, timedOut, sourceSkip, unreadable, time.Since(t0).Round(time.Millisecond))
	}
	ri := newRelIndex(defs, edges, bridges, jni, fileSet)
	entries := []relEntry{}
	for _, name := range names {
		entries = append(entries, ri.resolve(name)...)
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
