// Rust symbol extraction, pub use re-export parsing, #[wasm_bindgen]
// bridge discovery, and crate/module import resolution for --related
// (split from related.go).

package main

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

var relRustQuery = `[
	(attribute_item) @attr
	(function_item name: (identifier) @name) @fn
	(impl_item type: (_) @implType) @impl
	(use_declaration) @use
]`

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
