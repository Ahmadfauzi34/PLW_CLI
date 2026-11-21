// Kotlin symbol extraction and JNI external-fun discovery for --related
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

var relKtVarSpecStore = newSpecStore[callSpec]()

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
