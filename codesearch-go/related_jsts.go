// JS/TS/JSX/TSX symbol extraction, export parsing, and relative import
// resolution for --related (split from related.go).

package main

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

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
)

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

var (
	reFromClause    = regexp.MustCompile(`from\s+['"]([^'"]+)['"]`)
	reExportStarAs  = regexp.MustCompile(`^export\s+(?:type\s+)?\*\s+as\s+([A-Za-z_$][A-Za-z0-9_$]*)\s+from\s+['"]([^'"]+)['"]`)
	reExportStar    = regexp.MustCompile(`^export\s+(?:type\s+)?\*\s+from\s+['"]([^'"]+)['"]`)
	reLocalExport   = regexp.MustCompile(`^export\s+(?:type\s+)?\{`)
	reExportDefault = regexp.MustCompile(`^export\s+default\b`)
	reExportVar     = regexp.MustCompile(`^export\s+(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
)

// stripJSComments removes // line comments and /* ... */ block comments from
// a JS/TS export statement while preserving single- and double-quoted string
// literals (such as module specifiers in `from "./mod"`). Each comment is
// replaced with a single space so adjacent tokens never merge.
func stripJSComments(s string) string {
	if !strings.Contains(s, "//") && !strings.Contains(s, "/*") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '"' || s[i] == '\'' {
			q := s[i]
			b.WriteByte(q)
			i++
			for i < len(s) {
				b.WriteByte(s[i])
				if s[i] == '\\' && i+1 < len(s) {
					i++
					b.WriteByte(s[i])
					i++
					continue
				}
				if s[i] == q {
					i++
					break
				}
				i++
			}
			continue
		}
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
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < len(s) {
				i += 2
			} else {
				i = len(s)
			}
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// parseBraceNames parses the {..} list of an export statement into
// exported -> local names ("a as b" maps b -> a; "type " prefixes and
// JS/TS comments are stripped; tabs/newlines around `as` or `type` are
// supported). ok=false when braces are missing/misordered or empty.
func parseBraceNames(t string) (map[string]string, bool) {
	clean := stripJSComments(t)
	open := strings.Index(clean, "{")
	close := strings.LastIndex(clean, "}")
	if open < 0 || close < 0 || close < open {
		return nil, false
	}
	names := map[string]string{}
	for _, item := range strings.Split(clean[open+1:close], ",") {
		toks := strings.Fields(item)
		if len(toks) == 0 {
			continue
		}
		if len(toks) > 1 && toks[0] == "type" {
			toks = toks[1:]
		}
		if len(toks) == 3 && toks[1] == "as" {
			names[toks[2]] = toks[0]
		} else if len(toks) == 1 {
			names[toks[0]] = toks[0]
		} else {
			joined := strings.Join(toks, " ")
			names[joined] = joined
		}
	}
	if len(names) == 0 {
		return nil, false
	}
	return names, true
}

// parseExportStmt classifies one export statement's text into an edge
// and/or a local def (const/let/var or `default`). Returns (edge, edgeOK, def, defOK).
func parseExportStmt(from, text string, line int) (relEdge, bool, relDef, bool) {
	t := strings.TrimSpace(stripJSComments(text))
	if m := reExportStarAs.FindStringSubmatch(t); m != nil {
		// Namespace re-export (`export * as NS from "./mod"`): binds NS
		// to the target module without re-exporting its members at top level.
		return relEdge{From: from, Line: line, Target: m[2], Names: map[string]string{m[1]: "*"}}, true, relDef{}, false
	}
	if m := reExportStar.FindStringSubmatch(t); m != nil {
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
	if reLocalExport.MatchString(t) {
		// Local export list (including `export type { ... }`): names defined in this same file.
		names, ok := parseBraceNames(t)
		if !ok {
			return relEdge{}, false, relDef{}, false
		}
		return relEdge{From: from, Line: line, Target: "", Names: names}, true, relDef{}, false
	}
	if reExportDefault.MatchString(t) {
		// Record `"default"` at the export-default line so `export { default as X } from ...`
		// resolves cleanly to this module without interfering with named defs.
		return relEdge{}, false, relDef{Path: from, Line: line, Name: "default"}, true
	}
	// Declaration form: function/class defs are already captured by the
	// def query; only const/let/var needs recording here.
	if m := reExportVar.FindStringSubmatch(t); m != nil {
		return relEdge{}, false, relDef{Path: from, Line: line, Name: m[1]}, true
	}
	return relEdge{}, false, relDef{}, false
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
