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
	(use_declaration) @use
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

// relRustConstQuery captures top-level const/static items. The chunking
// def query has no const/static patterns, so this runs as a second
// cursor; extractRelatedRust keeps only items outside every def range
// (a const inside a function or impl is a member/local, not a
// top-level definition — the same range rule as the JS/Kotlin
// top-level-var filters).
var relRustConstQuery = `[
	(const_item name: (identifier) @name) @def
	(static_item name: (identifier) @name) @def
]`

var relRustConstSpecStore = newSpecStore[callSpec]()

func getRelRustConstSpec() (*callSpec, bool) {
	return relRustConstSpecStore.get("", func() (*callSpec, bool) {
		lang := grammars.RustLanguage()
		if lang == nil {
			return nil, false
		}
		q, err := gotreesitter.NewQuery(relRustConstQuery, lang)
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

// lineStart returns the byte offset at which line (1-based) starts,
// or -1 when the line is out of range.
func (li lineIndex) lineStart(line int) int {
	if line < 1 || line > len(li.starts) {
		return -1
	}
	return li.starts[line-1]
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

// stripRustComments removes // line comments and (possibly nested)
// /* ... */ block comments from a Rust use statement, replacing each
// comment with a single space so adjacent tokens never merge.
func stripRustComments(s string) string {
	if !strings.Contains(s, "//") && !strings.Contains(s, "/*") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '/' && i+1 < len(s) && s[i+1] == '/' {
			i += 2
			for i < len(s) && s[i] != '\n' {
				i++
			}
			b.WriteByte(' ')
			continue
		}
		if s[i] == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			depth := 1
			for i < len(s) && depth > 0 {
				if s[i] == '/' && i+1 < len(s) && s[i+1] == '*' {
					depth++
					i += 2
				} else if s[i] == '*' && i+1 < len(s) && s[i+1] == '/' {
					depth--
					i += 2
				} else {
					i++
				}
			}
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// splitRustAs splits a terminal use segment ("Name", "Name as Alias",
// including tabs or newlines around `as`) into its name and alias.
func splitRustAs(seg string) (string, string) {
	f := strings.Fields(seg)
	if len(f) == 3 && f[1] == "as" {
		return f[0], f[2]
	}
	return strings.TrimSpace(seg), ""
}

// parseRustUse parses one `use ...;` declaration's text into its
// re-export edges. Private `use` (no pub) is an import, not a
// re-export, and returns no edges. Text parsing (like parseExportStmt
// for JS): the tree-sitter child shapes for use trees vary, but the
// text form is stable. One statement can yield several edges (nested
// groups re-export from different modules: `use a::{b::c, d};`).
func parseRustUse(from, text string, line int) []relEdge {
	t := strings.TrimSpace(stripRustComments(text))
	if !strings.HasPrefix(t, "pub") {
		return nil
	}
	rest := strings.TrimSpace(t[len("pub"):])
	if strings.HasPrefix(rest, "(") {
		end := strings.Index(rest, ")")
		if end < 0 {
			return nil
		}
		rest = strings.TrimSpace(rest[end+1:])
	}
	if !strings.HasPrefix(rest, "use") {
		return nil
	}
	rest = strings.TrimSpace(rest[len("use"):])
	rest = strings.TrimSpace(strings.TrimSuffix(rest, ";"))
	if rest == "" {
		return nil
	}
	var edges []relEdge
	rustUseWalk(from, line, nil, splitUsePath(rest), &edges)
	return edges
}

// splitUsePath splits a use path on :: at brace depth 0, so a group
// {A, B} stays one segment. A leading :: (2015-style absolute path)
// yields a leading empty segment, treated as crate:: by the resolver.
func splitUsePath(p string) []string {
	var segs []string
	depth := 0
	start := 0
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 && i+1 < len(p) && p[i+1] == ':' {
				segs = append(segs, strings.TrimSpace(p[start:i]))
				i++
				start = i + 1
			}
		}
	}
	segs = append(segs, strings.TrimSpace(p[start:]))
	return segs
}

// splitUseGroup splits a group's comma list at brace depth 0, so a
// nested group {b::{c, d}, e} splits into its two items, not four.
func splitUseGroup(g string) []string {
	var items []string
	depth := 0
	start := 0
	for i := 0; i < len(g); i++ {
		switch g[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				items = append(items, strings.TrimSpace(g[start:i]))
				start = i + 1
			}
		}
	}
	if tail := strings.TrimSpace(g[start:]); tail != "" {
		items = append(items, tail)
	}
	return items
}

// rustUseWalk expands one use path (already split into segments) into
// edges, with prefix being the module path accumulated so far. A group
// recurses per item; a glob yields a star edge; the final segment is a
// Name, `Name as Alias`, or `self` (which re-exports the module
// itself: `use a::self;` exports `a` from its parent).
func rustUseWalk(from string, line int, prefix, segs []string, edges *[]relEdge) {
	if len(segs) == 0 {
		return
	}
	last := segs[len(segs)-1]
	head := segs[:len(segs)-1]
	if strings.HasPrefix(last, "{") && strings.HasSuffix(last, "}") {
		np := append(append([]string{}, prefix...), head...)
		for _, item := range splitUseGroup(last[1 : len(last)-1]) {
			if item == "" {
				continue
			}
			rustUseWalk(from, line, np, splitUsePath(item), edges)
		}
		return
	}
	if last == "*" {
		np := append(append([]string{}, prefix...), head...)
		*edges = append(*edges, relEdge{From: from, Line: line,
			Target: strings.Join(np, "::"), Star: true})
		return
	}
	name, alias := splitRustAs(last)
	if name == "self" {
		np := append(append([]string{}, prefix...), head...)
		if len(np) == 0 {
			return
		}
		mod := np[len(np)-1]
		exported := alias
		if exported == "" {
			exported = mod
		}
		*edges = append(*edges, relEdge{From: from, Line: line,
			Target: strings.Join(np[:len(np)-1], "::"),
			Names:  map[string]string{exported: mod}})
		return
	}
	exported := alias
	if exported == "" {
		exported = name
	}
	np := append(append([]string{}, prefix...), head...)
	*edges = append(*edges, relEdge{From: from, Line: line,
		Target: strings.Join(np, "::"),
		Names:  map[string]string{exported: name}})
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

// extractRelatedKotlin returns Kotlin definitions (classes, functions,
// objects, type aliases — the shared ktDefQuery) plus top-level
// properties (vals outside every def range). Kotlin has no re-export
// syntax, so there are no edges; the Kotlin<->Rust JNI bridge
// (external fun <-> Java_...) is a later stage, not v1.
// --- Kotlin JNI declarations (go14, P2) ---
//
// An `external fun` inside a class/object/companion — or at top level
// (whose class is the file facade <FileStem>Kt) — is implemented by a
// Rust fn named Java_<pkg>_<Class>_<method>. The AST gives the package
// (via a text regex — the package_header node has no capture-friendly
// shape), the enclosing class chain, the arity, and the coarse type
// tokens per parameter.

// reKotlinPackage extracts the declared package from Kotlin source
// text. A file without a package line has the default (empty) package.
var reKotlinPackage = regexp.MustCompile(`(?m)^package\s+([\w.]+)`)

// relKtJNIQuery captures every function declaration with its parameter
// list; the external-modifier check is done on the node (modifiers are
// not a capture target of the def query).
var relKtJNIQuery = `[
	(function_declaration (simple_identifier) @name (function_value_parameters) @params) @fn
]`

// relKtClassQuery captures enclosing declarations: a class or named
// object (with @name) or a companion_object (no name — it inherits
// <enclosing>$Companion as its JVM name).
var relKtClassQuery = `[
	(class_declaration (type_identifier) @name) @decl
	(object_declaration (type_identifier) @name) @decl
	(companion_object) @decl
]`

var (
	relKtJNISpecStore   = newSpecStore[callSpec]()
	relKtClassSpecStore = newSpecStore[callSpec]()
)

func getRelKtJNISpec() (*callSpec, bool) {
	return relKtJNISpecStore.get(".kt", func() (*callSpec, bool) {
		lang := grammars.KotlinLanguage()
		if lang == nil {
			return nil, false
		}
		q, err := gotreesitter.NewQuery(relKtJNIQuery, lang)
		if err != nil {
			return nil, false
		}
		return &callSpec{lang: lang, query: q}, true
	})
}

func getRelKtClassSpec() (*callSpec, bool) {
	return relKtClassSpecStore.get(".kt", func() (*callSpec, bool) {
		lang := grammars.KotlinLanguage()
		if lang == nil {
			return nil, false
		}
		q, err := gotreesitter.NewQuery(relKtClassQuery, lang)
		if err != nil {
			return nil, false
		}
		return &callSpec{lang: lang, query: q}, true
	})
}

// kotlinFnIsExternal reports whether a function_declaration node
// carries the `external` modifier (the JNI marker). The modifiers
// child holds function_modifier nodes; annotations live there too, so
// the check is on the modifier text, not on a substring of the whole
// node.
func kotlinFnIsExternal(fn *gotreesitter.Node, lang *gotreesitter.Language, src []byte) bool {
	for i := 0; i < fn.ChildCount(); i++ {
		ch := fn.Child(i)
		if ch == nil || ch.Type(lang) != "modifiers" {
			continue
		}
		for j := 0; j < ch.NamedChildCount(); j++ {
			mc := ch.NamedChild(j)
			if mc != nil && mc.Type(lang) == "function_modifier" && string(src[mc.StartByte():mc.EndByte()]) == "external" {
				return true
			}
		}
	}
	return false
}

// kotlinFnHasAnnotation reports whether a function_declaration node
// carries the named annotation (e.g. "JvmStatic", matching @JvmStatic,
// @kotlin.jvm.JvmStatic, @JvmStatic(), or @[JvmStatic]).
func kotlinFnHasAnnotation(fn *gotreesitter.Node, lang *gotreesitter.Language, src []byte, annoName string) bool {
	for i := 0; i < fn.ChildCount(); i++ {
		ch := fn.Child(i)
		if ch == nil || ch.Type(lang) != "modifiers" {
			continue
		}
		for j := 0; j < ch.NamedChildCount(); j++ {
			mc := ch.NamedChild(j)
			if mc == nil || mc.Type(lang) != "annotation" {
				continue
			}
			raw := strings.TrimSpace(string(src[mc.StartByte():mc.EndByte()]))
			raw = strings.TrimPrefix(raw, "@")
			// Strip optional use-site target (e.g. @jvm:JvmStatic).
			if _, after, ok := strings.Cut(raw, ":"); ok {
				raw = strings.TrimSpace(after)
			}
			var items []string
			if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
				inner := strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")
				items = strings.Fields(inner)
			} else {
				items = []string{raw}
			}
			for _, item := range items {
				if before, _, ok := strings.Cut(item, "("); ok {
					item = before
				}
				item = strings.TrimSpace(item)
				if dot := strings.LastIndexByte(item, '.'); dot >= 0 {
					item = item[dot+1:]
				}
				if item == annoName {
					return true
				}
			}
		}
	}
	return false
}

// ktClassRange is one enclosing declaration for the JNI chain: a named
// class/object, or a companion object (unnamed defaults to "Companion";
// a named companion object uses its declared name, e.g. Outer$Native).
type ktClassRange struct {
	start, end uint32
	name       string
	companion  bool
}

// ktEnclosingChain returns the enclosing class/object names for a byte
// offset, outermost-first. A companion object contributes its name or
// "Companion" (JVM Outer$Companion / Outer$Name), EXCEPT when jvmStatic
// is true and the companion is the innermost enclosing scope — Kotlin
// compiles @JvmStatic companion methods as static methods directly on
// the enclosing outer class (Outer.method). An orphan companion with no
// enclosing class returns nil (fail-closed).
func ktEnclosingChain(classes []ktClassRange, at uint32, jvmStatic bool) []string {
	var enc []ktClassRange
	for _, c := range classes {
		if at >= c.start && at < c.end {
			enc = append(enc, c)
		}
	}
	if len(enc) == 0 {
		return nil
	}
	// Innermost has the largest start; iterate outermost-first.
	sort.Slice(enc, func(i, j int) bool { return enc[i].start > enc[j].start })
	var chain []string
	for i := len(enc) - 1; i >= 0; i-- {
		c := enc[i]
		if c.companion {
			if len(chain) == 0 {
				return nil
			}
			if jvmStatic && i == 0 {
				continue
			}
			compName := c.name
			if compName == "" {
				compName = "Companion"
			}
			chain = append(chain, compName)
			continue
		}
		if c.name == "" {
			return nil
		}
		chain = append(chain, c.name)
	}
	return chain
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

// kotlinParamCoarse maps one parameter node to its JNI coarse token:
// the type's text through jniCoarseToken, "" when unknown. vararg is
// passed in by the caller: in the grammar, `vararg` lives in a
// `parameter_modifiers` node that is a SIBLING of the `parameter`
// node inside `function_value_parameters`, not inside the parameter
// itself — so the parameter's own text (`xs: Int`) can never show it.
// A vararg parameter is an array parameter in the JVM signature.
func kotlinParamCoarse(p *gotreesitter.Node, lang *gotreesitter.Language, src []byte, vararg bool) string {
	var typeText string
	for i := 0; i < p.NamedChildCount(); i++ {
		ch := p.NamedChild(i)
		if ch == nil {
			continue
		}
		if strings.HasSuffix(ch.Type(lang), "_type") && typeText == "" {
			typeText = string(src[ch.StartByte():ch.EndByte()])
		}
	}
	if typeText == "" {
		return ""
	}
	return jniCoarseToken(typeText, vararg)
}

func extractRelatedKotlin(relPath string, src []byte, li lineIndex) ([]relDef, []relEdge, []relBridge, []relJNIDecl, bool) {
	ext := extOf(relPath)
	spec, ok := getSpec(ext)
	if !ok {
		return nil, nil, nil, nil, false
	}
	parser := gotreesitter.NewParser(spec.lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, nil, nil, nil, false
	}
	// Definitions come from the shared helper (kotlin_fallback.go), so
	// --related agrees with --defs-only on names, anchored lines, and
	// byte spans — including declarations the grammar failed to parse
	// (annotation before a visibility modifier), which the regex
	// fallback recovers with byte spans of their own. The property
	// filter below keeps working on byte ranges unchanged: a local
	// val inside a recovered def's body is inside its byte span, so
	// it no longer leaks as a top-level property.
	ktDefs := kotlinASTAndFallbackDefs(tree, spec.lang, spec.query, src, li.lineOf)
	var defs []relDef
	var defRanges [][2]uint32
	for _, d := range ktDefs {
		if d.name == "" {
			continue
		}
		defs = append(defs, relDef{Path: relPath, Line: d.startLine, Name: d.name})
		defRanges = append(defRanges, [2]uint32{d.startByte, d.endByte})
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
	// JNI declarations (go14, P2): external funs and their base keys.
	// Runs on the same parsed tree as the defs above — no second parse.
	var jniDecls []relJNIDecl
	pkg := ""
	if m := reKotlinPackage.FindStringSubmatch(string(stripKotlinCode(src))); m != nil {
		pkg = m[1]
	}
	var classes []ktClassRange
	if cs, ok := getRelKtClassSpec(); ok {
		ccursor := cs.query.Exec(tree.RootNode(), cs.lang, src)
		for {
			m, ok := ccursor.NextMatch()
			if !ok {
				break
			}
			var sb, eb uint32
			var name string
			var isCompanion bool
			found := false
			for _, c := range m.Captures {
				if c.Name == "decl" {
					sb, eb = c.Node.StartByte(), c.Node.EndByte()
					found = true
					if c.Node.Type(cs.lang) == "companion_object" {
						isCompanion = true
						for k := 0; k < c.Node.NamedChildCount(); k++ {
							ch := c.Node.NamedChild(k)
							if ch == nil {
								continue
							}
							if t := ch.Type(cs.lang); t == "type_identifier" || t == "simple_identifier" {
								name = string(src[ch.StartByte():ch.EndByte()])
								break
							}
						}
					}
				}
				if c.Name == "name" {
					name = string(src[c.Node.StartByte():c.Node.EndByte()])
				}
			}
			if !found {
				continue
			}
			classes = append(classes, ktClassRange{sb, eb, name, isCompanion})
		}
	}
	if js, ok := getRelKtJNISpec(); ok {
		jcursor := js.query.Exec(tree.RootNode(), js.lang, src)
		for {
			m, ok := jcursor.NextMatch()
			if !ok {
				break
			}
			var fnNode *gotreesitter.Node
			var name string
			var paramsNode *gotreesitter.Node
			for _, c := range m.Captures {
				switch c.Name {
				case "fn":
					fnNode = c.Node
				case "name":
					name = string(src[c.Node.StartByte():c.Node.EndByte()])
				case "params":
					paramsNode = c.Node
				}
			}
			if fnNode == nil || name == "" || paramsNode == nil {
				continue
			}
			if !kotlinFnIsExternal(fnNode, js.lang, src) {
				continue
			}
			// Arity + coarse tokens from the parameter nodes. A
			// `parameter_modifiers` sibling (e.g. `vararg`) applies to
			// the NEXT parameter — it is not inside the parameter node.
			var coarse []string
			arity := 0
			varargNext := false
			for i := 0; i < paramsNode.NamedChildCount(); i++ {
				ch := paramsNode.NamedChild(i)
				if ch == nil {
					continue
				}
				if ch.Type(js.lang) == "parameter_modifiers" {
					varargNext = hasWord(string(src[ch.StartByte():ch.EndByte()]), "vararg")
					continue
				}
				if ch.Type(js.lang) != "parameter" {
					continue
				}
				arity++
				coarse = append(coarse, kotlinParamCoarse(ch, js.lang, src, varargNext))
				varargNext = false
			}
			// Enclosing class chain (outermost-first); a top-level
			// external fun belongs to the file facade <FileStem>Kt.
			jvmStatic := kotlinFnHasAnnotation(fnNode, js.lang, src, "JvmStatic")
			chain := ktEnclosingChain(classes, fnNode.StartByte(), jvmStatic)
			if chain == nil {
				stem := strings.TrimSuffix(path.Base(relPath), ".kt")
				chain = []string{stem + "Kt"}
			}
			anchor := fnNode.StartByte()
			if a, ok := kotlinAnchorStart(fnNode, js.lang); ok {
				anchor = a
			}
			jniDecls = append(jniDecls, relJNIDecl{
				Path: relPath, Line: li.lineOf(anchor), Name: name,
				Base: jniKotlinBase(pkg, chain, name), Arity: arity, Coarse: coarse,
			})
		}
	}
	return uniq, nil, nil, jniDecls, true
}

func extractRelatedJS(relPath string, src []byte, li lineIndex) ([]relDef, []relEdge, []relBridge, []relJNIDecl, bool) {
	ext := extOf(relPath)
	// Defs from the shared def query (functions, methods, classes,
	// arrows, function expressions).
	dq, ok := defQueryFor(ext)
	if !ok {
		return nil, nil, nil, nil, false
	}
	eq, ok := getRelExportSpec(ext)
	if !ok {
		return nil, nil, nil, nil, false
	}
	parser := gotreesitter.NewParser(eq.lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, nil, nil, nil, false
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
	return uniq, edges, nil, nil, true
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

func extractRelatedRust(relPath string, src []byte, li lineIndex) ([]relDef, []relEdge, []relBridge, []relJNIDecl, bool) {
	spec, ok := getRelRustSpec()
	if !ok {
		return nil, nil, nil, nil, false
	}
	parser := gotreesitter.NewParser(spec.lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, nil, nil, nil, false
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
	var useTexts [][2]uint32 // (start, end) of each use_declaration
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
		var useStart, useEnd uint32
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
			case "use":
				useStart, useEnd = c.Node.StartByte(), c.Node.EndByte()
				kind = "use"
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
		case "use":
			useTexts = append(useTexts, [2]uint32{useStart, useEnd})
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
				Kind: "wasm",
			})
			continue
		}
		isBindgen, jsName := attrChainBindgen(src, attrs, fn.start)
		// JNI (go14, P2): a top-level fn named Java_<pkg>_<Class>_<method>
		// is a native implementation — the name IS the convention, no
		// attribute needed. (A fn can in principle be both bindgen and
		// JNI-named; both bridge rows are emitted, they never collide:
		// the wasm section keys on JSName, the JNI section on Base.)
		if strings.HasPrefix(fn.name, "Java_") {
			bridges = append(bridges, relBridge{
				RustPath: relPath, RustLine: li.lineOf(fn.start),
				RustName: fn.name, Kind: "jni",
			})
		}
		if !isBindgen {
			continue
		}
		if jsName == "" {
			jsName = toCamel(fn.name)
		}
		bridges = append(bridges, relBridge{
			RustPath: relPath, RustLine: li.lineOf(fn.start),
			RustName: fn.name, JSName: jsName, Kind: "wasm",
		})
	}
	// Definitions: the chunking def query (functions, trait
	// signatures, structs, enums, traits, type aliases), minus
	// impl_item defs — the query names an impl by its type, which
	// would duplicate the struct def at a different line. The ranges
	// of ALL def nodes (impl ranges included) feed the const/static
	// filter below, so an associated const inside an impl still
	// counts as a member, not a top-level definition.
	var defs []relDef
	var defRanges [][2]uint32
	if dq, ok := defQueryFor(".rs"); ok {
		dcursor := dq.Exec(tree.RootNode(), spec.lang, src)
		for {
			m, ok := dcursor.NextMatch()
			if !ok {
				break
			}
			var sb, eb uint32
			var name string
			var isImpl bool
			found := false
			for _, c := range m.Captures {
				if c.Name == "def" {
					sb, eb = c.Node.StartByte(), c.Node.EndByte()
					found = true
					if c.Node.Type(spec.lang) == "impl_item" {
						isImpl = true
					}
				}
				if c.Name == "name" {
					name = string(src[c.Node.StartByte():c.Node.EndByte()])
				}
			}
			if !found {
				continue
			}
			defRanges = append(defRanges, [2]uint32{sb, eb})
			if isImpl || name == "" {
				continue
			}
			defs = append(defs, relDef{Path: relPath, Line: li.lineOf(sb), Name: name})
		}
	}
	// Top-level const/static: items outside every def range.
	if cs, ok := getRelRustConstSpec(); ok {
		ccursor := cs.query.Exec(tree.RootNode(), spec.lang, src)
		for {
			m, ok := ccursor.NextMatch()
			if !ok {
				break
			}
			var sb uint32
			var name string
			found := false
			for _, c := range m.Captures {
				if c.Name == "def" {
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
	// Re-export edges: `pub use` declarations, parsed from text.
	var edges []relEdge
	for _, u := range useTexts {
		edges = append(edges, parseRustUse(relPath, string(src[u[0]:u[1]]), li.lineOf(u[0]))...)
	}
	// Dedup: a def can arrive twice (e.g. a function_item matched by
	// both the def query and the fn capture is only emitted once —
	// defs come from one cursor — but a const named like a fn would
	// still collide on path:line:name).
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
	return uniq, edges, bridges, nil, true
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

// --- JNI bridge (go14, P2: Option 3, Base-Key Grouping) ---
//
// The JNI name mangling is per the JNI spec (docs.oracle.com, "Design
// Overview"): a native function is named
// Java_<pkg>_<Class>_<method>[__<mangled signature>], with _ -> _1,
// / -> _, ; -> _2, [ -> _3, and any other non-alphanumeric rune ->
// _0wxyz (lowercase hex). Because _ escapes to _1, the literal sequence
// __ never occurs inside the Java_<pkg>_<Class>_<method> prefix —
// splitting on the FIRST __ is exact, and the long (overloaded) form
// carries the full argument signature after it.

// jniEscape escapes one JNI name segment (a package segment, a class
// name, or a method name) per the spec. Alphanumerics are unchanged;
// _ -> _1, $ -> _00024 (inner-class separator); any other rune ->
// _0xxxx. The / -> _ rule never applies here: package separators are
// handled by joining escaped segments with _.
func jniEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '_':
			b.WriteString("_1")
		case r == '$':
			b.WriteString("_00024")
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "_0%04x", r)
		}
	}
	return b.String()
}

// jniKotlinBase builds the JNI base key for a Kotlin external fun:
// Java_<escaped pkg segments joined by _>_<escaped class chain>_<escaped
// method>. classChain is outermost-first (["Bridge"] or
// ["Outer", "Inner"]); the chain is joined with $ (the JVM binary
// name) before escaping, so Outer$Inner becomes Outer_00024Inner. An
// empty chain means top-level — the caller supplies the file facade
// <FileStem>Kt as the single element.
func jniKotlinBase(pkg string, classChain []string, method string) string {
	var parts []string
	if pkg != "" {
		for _, seg := range strings.Split(pkg, ".") {
			parts = append(parts, jniEscape(seg))
		}
	}
	parts = append(parts, jniEscape(strings.Join(classChain, "$")))
	parts = append(parts, jniEscape(method))
	return "Java_" + strings.Join(parts, "_")
}

// parseJNIRustName splits a Rust JNI function name on the FIRST __
// (the overload separator). Returns the base, the parsed signature
// tokens (nil for a short name), and whether the name is an overloaded
// (long) name. ok=false when the name is not a JNI name or its
// signature is malformed — the caller treats that as "not a JNI
// bridge" and skips it (fail-closed for matching, silent for indexing).
func parseJNIRustName(name string) (base string, tokens []string, overloaded, ok bool) {
	if !strings.HasPrefix(name, "Java_") {
		return "", nil, false, false
	}
	base, raw, cut := strings.Cut(name, "__")
	if !cut {
		return name, nil, false, true
	}
	tokens, ok = parseJNISig(raw)
	if !ok {
		return "", nil, false, false
	}
	return base, tokens, true, true
}

// parseJNISig parses a mangled JNI argument signature (after __) into
// one token per parameter. Exact, no classpath: _3 prefixes
// (repeatable) are array dimensions, primitives (B C D F I J S Z) are
// one token each, objects run L..._2. Returns ok=false on malformed
// input (dangling array prefix, unterminated object, unknown letter).
// An empty signature parses to zero tokens (a zero-arity function).
func parseJNISig(sig string) ([]string, bool) {
	var tokens []string
	for len(sig) > 0 {
		rest := sig
		for strings.HasPrefix(sig, "_3") {
			sig = sig[2:]
		}
		if len(sig) == 0 {
			return nil, false // dangling array prefix
		}
		switch sig[0] {
		case 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z':
			sig = sig[1:]
		case 'L':
			end := strings.Index(sig, "_2")
			if end < 0 {
				return nil, false
			}
			sig = sig[end+2:]
		default:
			return nil, false
		}
		tokens = append(tokens, rest[:len(rest)-len(sig)])
	}
	return tokens, true
}

// jniCoarseToken maps a Kotlin parameter type to its JNI signature
// token when that mapping is certain without a classpath, else "".
// Primitives map 1:1; String and primitive arrays have fixed JVM
// forms; nullable types, generics, and custom classes are wildcards
// ("" — the JVM type of Int? is Ljava/lang/Integer;, of Map<..> is
// Ljava/util/Map;, neither knowable from the source text alone). A
// vararg parameter is an array parameter in the JVM signature.
func jniCoarseToken(kotlinType string, vararg bool) string {
	t := strings.TrimSpace(kotlinType)
	if strings.HasSuffix(t, "?") {
		return "" // nullable: boxed — classpath knowledge
	}
	var tok string
	switch t {
	case "Byte":
		tok = "B"
	case "Char":
		tok = "C"
	case "Double":
		tok = "D"
	case "Float":
		tok = "F"
	case "Int":
		tok = "I"
	case "Long":
		tok = "J"
	case "Short":
		tok = "S"
	case "Boolean":
		tok = "Z"
	case "String":
		tok = "Ljava_lang_String_2"
	case "ByteArray":
		tok = "_3B"
	case "CharArray":
		tok = "_3C"
	case "DoubleArray":
		tok = "_3D"
	case "FloatArray":
		tok = "_3F"
	case "IntArray":
		tok = "_3I"
	case "LongArray":
		tok = "_3J"
	case "ShortArray":
		tok = "_3S"
	case "BooleanArray":
		tok = "_3Z"
	default:
		return ""
	}
	if vararg {
		return "_3" + tok
	}
	return tok
}

// jniCoarseCompatible reports whether a Kotlin decl's coarse tokens
// are compatible with a Rust JNI signature's tokens: same length, and
// every non-wildcard ("") Kotlin token equals the Rust token at the
// same position. A wildcard never vetoes — the source text cannot
// prove that parameter's JVM type.
func jniCoarseCompatible(coarse, rustTokens []string) bool {
	if len(coarse) != len(rustTokens) {
		return false
	}
	for i := range coarse {
		if coarse[i] != "" && coarse[i] != rustTokens[i] {
			return false
		}
	}
	return true
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

// resolveRustImport maps a Rust module path written in fromFile to an
// indexed file path, or "" when it cannot be resolved. Segments:
// crate:: (crate root), self:: (current module), super:: (parent
// module, repeatable), a leading :: (crate root, 2015 style), or a
// plain path (relative to the current module, like self::). The file
// is <dir>/<segs>.rs or <dir>/<segs>/mod.rs.
func resolveRustImport(fromFile, spec string, files map[string]bool) string {
	if strings.TrimSpace(spec) == "" {
		return ""
	}
	segs := splitUsePath(spec)
	if len(segs) == 0 {
		return ""
	}
	var base string
	rest := segs
	switch segs[0] {
	case "", "crate":
		base = rustCrateRootDir(fromFile, files)
		rest = segs[1:]
	case "self":
		base = rustModuleDir(fromFile)
		rest = segs[1:]
	case "super":
		base = rustModuleDir(fromFile)
		for len(rest) > 0 && rest[0] == "super" {
			parent := path.Dir(base)
			if parent == base {
				return ""
			}
			base = parent
			rest = rest[1:]
		}
	default:
		base = rustModuleDir(fromFile)
	}
	if len(rest) == 0 {
		switch segs[0] {
		case "self":
			if files[fromFile] {
				return fromFile
			}
		case "crate":
			for _, cand := range []string{"lib.rs", "main.rs"} {
				if f := path.Join(base, cand); files[f] {
					return f
				}
			}
		case "super":
			if base != "." && base != "" && base != "/" {
				if f := base + ".rs"; files[f] {
					return f
				}
			}
			for _, cand := range []string{"mod.rs", "lib.rs", "main.rs"} {
				if f := path.Join(base, cand); files[f] {
					return f
				}
			}
		}
		return ""
	}
	if f := path.Join(base, strings.Join(rest, "/")+".rs"); files[f] {
		return f
	}
	if f := path.Join(base, strings.Join(rest, "/"), "mod.rs"); files[f] {
		return f
	}
	return ""
}

// rustModuleDir is the directory holding fromFile's child modules: the
// file's own directory for mod.rs/lib.rs/main.rs (the module IS that
// directory), else dir/<stem> (foo.rs -> foo/).
func rustModuleDir(file string) string {
	base := path.Base(file)
	if base == "mod.rs" || base == "lib.rs" || base == "main.rs" {
		return path.Dir(file)
	}
	return path.Join(path.Dir(file), strings.TrimSuffix(base, ".rs"))
}

// rustCrateRootDir finds the crate root directory for fromFile: the
// nearest ancestor directory containing lib.rs or main.rs in the
// index, else fromFile's own directory (a flat single-file crate).
func rustCrateRootDir(fromFile string, files map[string]bool) string {
	dir := path.Dir(fromFile)
	for {
		if files[path.Join(dir, "lib.rs")] || files[path.Join(dir, "main.rs")] {
			return dir
		}
		parent := path.Dir(dir)
		if parent == dir {
			return path.Dir(fromFile)
		}
		dir = parent
	}
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
