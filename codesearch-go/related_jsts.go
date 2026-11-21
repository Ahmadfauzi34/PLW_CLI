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
