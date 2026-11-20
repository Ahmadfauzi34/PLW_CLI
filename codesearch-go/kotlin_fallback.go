package main

import (
	"regexp"
	"sort"

	"github.com/odvcencio/gotreesitter"
)

// Kotlin definition recovery (PR #26 follow-up, 2026-11-20).
//
// Root cause, verified by probe against the vendored grammar
// (gotreesitter v0.55.1, and v0.57.0 — still broken): the
// tree-sitter-kotlin grammar cannot parse an annotation before a
// visibility modifier. `@Composable internal fun` degrades to ERROR
// nodes, the function_declaration disappears entirely, ktDefQuery
// matches zero definitions, and the whole file falls back to fixed
// windows (no def structure at all). The same failure hits classes
// (`@Deprecated internal class`) and any annotation+visibility pair;
// annotation alone and modifier alone both parse fine.
//
// Two repairs, both anchored on the same rule as the Python
// calibration (PR #26: anchor on the declaration itself — first
// modifier or keyword — never on a leading @annotation):
//
//  1. kotlinAnchorStart: for definitions the parser DID produce,
//     move the span start from the annotation line to the mods/kw
//     anchor by walking the def node's `modifiers` child.
//  2. kotlinRegexDefinitions: regex definitions are ALWAYS merged in
//     (P1-C, go14), not only when the parse is damaged or yields
//     nothing. A file can parse cleanly (HasError false) and still
//     miss declarations: `@Composable internal fun Target() {}` degrades
//     to a trailing-lambda call — a syntactically valid call
//     expression, so the tree carries no error and the AST yields the
//     file's other defs. The merge dedups on (name, startLine) with
//     AST defs winning, so healthy files gain nothing spurious; the
//     regex only fills gaps the AST left.
//
// Known limits of the regex path (fail-open, documented, not silent):
//   - An annotation whose arguments nest parentheses TWO levels deep
//     on the SAME line as the modifiers (`@Foo(bar(baz(qux))) internal
//     fun`) is not consumed, so that one declaration is missed when it
//     shares the line; one level (`@Suppress(names = arrayOf("UNUSED"))`)
//     is supported since go14 (P1-C). On its own line an annotation is
//     still found via the mods line either way.
//   - A class supertype on its own line (`class Foo\n : Bar {`)
//     ends the header early: the def is found with the right anchor
//     but its body is not attached.
//   - KDoc above an annotation is not attached to the def chunk: the
//     annotation line blocks the doc-comment walk-up, so the KDoc
//     lands in a gap window instead (still indexed, just not in the
//     def chunk). The reported def line is the calibrated anchor.

// kotlinAnchorStart returns the byte offset of a Kotlin definition's
// anchor — the first non-annotation modifier, else the declaration
// keyword — for a tree-sitter definition node. ok=false when the node
// carries no modifiers child (caller keeps the node start).
func kotlinAnchorStart(def *gotreesitter.Node, lang *gotreesitter.Language) (uint32, bool) {
	for i := 0; i < def.ChildCount(); i++ {
		ch := def.Child(i)
		if ch == nil || !ch.IsNamed() || ch.Type(lang) != "modifiers" {
			continue
		}
		for j := 0; j < ch.NamedChildCount(); j++ {
			mc := ch.NamedChild(j)
			if mc == nil || mc.Type(lang) == "annotation" {
				continue
			}
			// A comment can sit inside the modifiers node too
			// (e.g. `/* c */ internal fun`); it is not an anchor.
			if t := mc.Type(lang); t == "line_comment" || t == "block_comment" {
				continue
			}
			return mc.StartByte(), true
		}
		// Every modifier is an annotation: the keyword follows the
		// modifiers node (fun/class/object/typealias/enum token).
		// Comments can sit between them — tree-sitter attaches a
		// trailing `// ...` on the annotation line as a child of
		// the declaration — so skip comment nodes and take the
		// first remaining child, which is the keyword.
		for k := i + 1; k < def.ChildCount(); k++ {
			if nxt := def.Child(k); nxt != nil {
				if t := nxt.Type(lang); t == "line_comment" || t == "block_comment" {
					continue
				}
				return nxt.StartByte(), true
			}
		}
		return 0, false
	}
	return 0, false
}

// stripKotlinCode blanks comments, strings, and char literals with
// spaces, preserving newlines and byte offsets, so the declaration
// regex never matches inside a comment or a string and every byte
// offset still maps to the original text via lineOf.
func stripKotlinCode(src []byte) []byte {
	out := make([]byte, len(src))
	copy(out, src)
	blank := func(a, b int) {
		for k := a; k < b; k++ {
			if out[k] != '\n' {
				out[k] = ' '
			}
		}
	}
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			j := i + 2
			for j < len(src) && src[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			j := i + 2
			for j+1 < len(src) && !(src[j] == '*' && src[j+1] == '/') {
				j++
			}
			if j+1 < len(src) {
				j += 2
			} else {
				j = len(src)
			}
			blank(i, j)
			i = j
		case c == '"' && i+2 < len(src) && src[i+1] == '"' && src[i+2] == '"':
			j := i + 3
			for j+2 < len(src) && !(src[j] == '"' && src[j+1] == '"' && src[j+2] == '"') {
				j++
			}
			if j+2 < len(src) {
				j += 3
			} else {
				j = len(src)
			}
			blank(i, j)
			i = j
		case c == '"':
			j := i + 1
			for j < len(src) && src[j] != '\n' {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if src[j] == '"' {
					j++
					break
				}
				j++
			}
			blank(i, j)
			i = j
		case c == '\'':
			j := i + 1
			if j < len(src) && src[j] == '\\' {
				j++
				if j < len(src) && src[j] == 'u' {
					j += 5
				} else {
					j += 2
				}
			} else {
				j++
			}
			if j < len(src) && src[j] == '\'' {
				j++
			}
			blank(i, j)
			i = j
		default:
			i++
		}
	}
	return out
}

// kotlinDeclRe mirrors the Python calibration member regex
// (_KOTLIN_SAME_PKG_DECL_REGEX / kt_member_re, PR #26): leading
// annotations are skipped, the anchor is the `mods` group when
// present else the `kw` group. Groups: 1=mods, 2=kw, 3=name.
// Go RE2 has no lookahead; the pattern needs none.
// NOTE: val/var are deliberately NOT here — the engine's def model
// (K4, see ktDefQuery) excludes properties: they live in --related,
// not in defs. The fallback restores what the AST would have found,
// nothing more.
// NOTE (2026-11-20, parity): the receiver group is `*` (all dotted
// segments), NOT `?` like the Python calibration. With `?`, an
// extension function with a two-segment receiver
// (`internal fun TimelineItem.ToolCall.cardText()`) was named after
// its receiver (`ToolCall`) instead of the function (`cardText`):
// a phantom def in --related and a wrong caller in --callers
// (ToolCallSheet.kt:247, OOPBoT differential). The AST names the
// function correctly, so with `*` the regex def dedups against it.
// The Python calibration still has the one-segment form; fixing it
// there is adapter/topology scope, not this engine change.
var kotlinDeclRe = regexp.MustCompile(`(?m)^[ \t]*(?:@[\w.]+(?:\((?:[^()]|\([^()]*\))*\))?[ \t]+)*(?P<mods>(?:(?:public|internal|private|protected|open|abstract|data|sealed|final|expect|actual|inline|value|external|override|crossinline|noinline|tailrec|suspend|operator|infix|lateinit|const|companion|annotation)[ \t]+)*)(?P<kw>class|interface|object|fun|typealias|enum)(?:[ \t]+(?:class|interface))?[ \t]+(?:<[^>]*>[ \t]+)?(?:[\w]+(?:<[^>]*>)?\??\.)*(?P<name>[A-Za-z_][\w]*)`)

// kotlinDeclEnd computes (endLine, sigEnd, endByte) for a declaration whose
// regex match ends at matchEnd in cleaned text. A `{` at header depth
// 0 opens a body (sigEnd = its line, endLine = matching `}`, endByte =
// the `}` byte); an `=` at depth 0 starts a single-expression body
// (sigEnd = its line, endLine = the expression's last line, endByte =
// its last non-space byte); otherwise the declaration ends on its own
// header line (sigEnd = 0, endByte = the header's last non-space byte).
// A `{` on the line right after the header still counts as the body.
// endByte lets consumers that only have regex defs (no AST node) run
// the same byte-range filter as AST defs.
func kotlinDeclEnd(clean []byte, matchEnd int, lineOf func(uint32) int) (endLine, sigEnd int, endByte uint32) {
	n := len(clean)
	i := matchEnd
	depth := 0 // ( [ depth in the header; braces are the body signal
	for i < n {
		c := clean[i]
		switch {
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			if depth > 0 {
				depth--
			}
		case c == '{' && depth == 0:
			d := 1
			j := i + 1
			for j < n && d > 0 {
				if clean[j] == '{' {
					d++
				} else if clean[j] == '}' {
					d--
				}
				j++
			}
			if d == 0 {
				return lineOf(uint32(j - 1)), lineOf(uint32(i)), uint32(j - 1)
			}
			return lineOf(uint32(n - 1)), lineOf(uint32(i)), uint32(n - 1)
		case c == '=' && depth == 0:
			d := 0
			j := i + 1
			last := i
			for j < n {
				switch clean[j] {
				case '(', '[', '{':
					d++
				case ')', ']', '}':
					if d > 0 {
						d--
					}
				case '\n':
					if d == 0 {
						return lineOf(uint32(last)), lineOf(uint32(i)), uint32(last)
					}
				}
				if clean[j] != ' ' && clean[j] != '\t' && clean[j] != '\r' {
					last = j
				}
				j++
			}
			return lineOf(uint32(n - 1)), lineOf(uint32(i)), uint32(n - 1)
		case c == '\n' && depth == 0:
			k := i + 1
			for k < n && (clean[k] == ' ' || clean[k] == '\t') {
				k++
			}
			if k < n && clean[k] == '{' {
				i = k
				continue
			}
			// Header-only declaration: end at the header's last
			// non-space byte, so a `;`-separated declaration sharing
			// the line is not swallowed into this def's byte span.
			e := i - 1
			for e >= 0 && (clean[e] == ' ' || clean[e] == '\t' || clean[e] == '\r') {
				e--
			}
			if e < matchEnd {
				e = matchEnd
			}
			return lineOf(uint32(i)), 0, uint32(e)
		}
		i++
	}
	return lineOf(uint32(n - 1)), 0, uint32(n - 1)
}

// kotlinRegexDefinitions extracts Kotlin definitions from cleaned
// text with kotlinDeclRe. The anchor is the mods group when present,
// else the kw group — the same rule as the Python calibration, so a
// declaration preceded by annotations reports its mods/kw line.
func kotlinRegexDefinitions(norm string, lineOf func(uint32) int) []definition {
	clean := stripKotlinCode([]byte(norm))
	var defs []definition
	for _, m := range kotlinDeclRe.FindAllSubmatchIndex(clean, -1) {
		modsS, modsE := m[2], m[3]
		kwS := m[4]
		nameS, nameE := m[6], m[7]
		if nameS < 0 || kwS < 0 {
			continue
		}
		anchor := kwS
		if modsS >= 0 && modsE > modsS {
			anchor = modsS
		}
		name := string(clean[nameS:nameE])
		endLine, sigEnd, endByte := kotlinDeclEnd(clean, m[1], lineOf)
		sl := lineOf(uint32(anchor))
		if endLine < sl {
			endLine = sl
		}
		defs = append(defs, definition{startLine: sl, endLine: endLine, name: name, sigEnd: sigEnd, startByte: uint32(anchor), endByte: endByte})
	}
	return defs
}

// mergeKotlinRegexDefs adds regex-found definitions the AST did not
// already report (same name on the same start line = the same
// declaration). AST defs are never dropped or re-anchored here —
// astDefinitions already put them on the mods/kw line.
func mergeKotlinRegexDefs(astDefs, regexDefs []definition) []definition {
	out := append([]definition(nil), astDefs...)
	for _, rd := range regexDefs {
		dup := false
		for _, ad := range out {
			if ad.name == rd.name && ad.startLine == rd.startLine {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, rd)
		}
	}
	return out
}

// kotlinASTAndFallbackDefs extracts Kotlin definitions from one parsed
// tree — the single source of truth shared by --defs-only (astchunk.go),
// --related (related.go) and --callers (callers.go), so all three agree
// on names, anchored lines, and byte spans.
//
// AST definitions come from query (ktDefQuery: @def/@name/@body captures),
// anchored on the declaration itself via kotlinAnchorStart (first
// non-annotation modifier, else the keyword — the Python calibration
// rule). Regex definitions are merged in unconditionally (P1-C, go14:
// a clean parse can still miss declarations — see the header note);
// AST defs are never dropped. Returned defs are sorted by (startLine,
// endLine, name); byte spans are populated for both paths (regex defs
// via the anchor byte and kotlinDeclEnd's endByte), so byte-range
// filters work identically on either path.
func kotlinASTAndFallbackDefs(tree *gotreesitter.Tree, lang *gotreesitter.Language, query *gotreesitter.Query, src []byte, lineOf func(uint32) int) []definition {
	cursor := query.Exec(tree.RootNode(), lang, src)
	var defs []definition
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var sb, eb uint32
		var name string
		var bodyStart uint32
		var defNode *gotreesitter.Node
		hasBody := false
		found := false
		for _, c := range m.Captures {
			if c.Name == "def" {
				sb, eb = c.Node.StartByte(), c.Node.EndByte()
				defNode = c.Node
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
		startByte := sb
		if defNode != nil {
			if anchor, ok := kotlinAnchorStart(defNode, lang); ok {
				sl = lineOf(anchor)
				startByte = anchor
			}
		}
		d := definition{startLine: sl, endLine: el, name: name, startByte: startByte, endByte: eb}
		if hasBody {
			d.sigEnd = lineOf(bodyStart)
		}
		defs = append(defs, d)
	}
	if rd := kotlinRegexDefinitions(string(src), lineOf); len(rd) > 0 {
		defs = mergeKotlinRegexDefs(defs, rd)
	}
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].startLine != defs[j].startLine {
			return defs[i].startLine < defs[j].startLine
		}
		if defs[i].endLine != defs[j].endLine {
			return defs[i].endLine < defs[j].endLine
		}
		return defs[i].name < defs[j].name
	})
	return defs
}
