//go:build verify

// ===========================================================================
// Penempatan skill Go verification-harness di codesearch, untuk parity
// Kotlin --related/--callers (PR #27 follow-up)
// ===========================================================================
// - harness.go di direktori ini adalah SALINAN VERBATIM dari skill
//   local-verification-harness-go (core TIDAK diubah; yang dicustom
//   hanya file spec ini, sesuai aturan skill).
// - File ini memakai build tag "verify": build/test/vet normal
//   (tanpa tag) mengabaikannya, sehingga tidak ada main() ganda dengan
//   main.go. Jalankan HANYA lewat ./verify-kotlin-parity.sh, yang memakai
//   `go run` dengan daftar file eksplisit (mode file mengabaikan tag)
//   dan TIDAK mencantumkan main.go.
// - Subjek: jawaban --related (defs + top-level vals) dan --callers
//   (call sites + caller) untuk satu file .kt, lewat extractRelated
//   dan extractCalls — termasuk def hasil fallback regex
//   (anotasi+visibility satu baris), filter header palsu, dan
//   atribusi caller ke dalam body def yang rusak AST-nya. Perilaku
//   yang dikunci: related/defs/callers sepakat soal nama + baris
//   ter-anchor; val lokal di dalam body def hasil regex tidak bocor
//   sebagai top-level; header deklarasi bukan call site; body
//   satu-baris dan single-expression tetap terhitung.
// - Oracle: pemindai baris independen (tanpa tree-sitter, tanpa regex
//   subjek) yang menghitung span def dari kedalaman kurung/braces,
//   properti top-level dari luar span, dan call site dari pola
//   `IDENT(` dengan aturan header/delegasi yang diturunkan ulang.
//   Ekuivalensi gerbang 3 membandingkan (name, startLine) related dan
//   (line, caller, callee-base) calls; endLine/sigEnd dikunci oleh unit
//   test (related_test.go, callers_test.go, index_test.go), bukan oleh
//   harness ini.
// - Batas oracle (terdokumentasi, bukan diam): modifier di baris
//   sendiri sebelum keyword tidak dimodelkan; generic call bersarang
//   (`foo<Map<String, Int>>()`) tidak tertangkap pola `IDENT(`;
//   sampel GeneratedSamples dijaga di dalam batas ini — kasus tepi
//   tersebut dikunci oleh Primary/Regression dengan ekspektasi
//   tulisan-tangan, bukan oleh oracle.

package main

import (
	"os"
	"regexp"
	"sort"
	"strings"
)

type ParityDefSpan struct {
	Name      string `json:"name"`
	StartLine int    `json:"startLine"`
}

type ParityCall struct {
	Line   int    `json:"line"`
	Caller string `json:"caller"`
	Callee string `json:"callee"`
}

type KotlinParityOut struct {
	Related []ParityDefSpan `json:"related"`
	Calls   []ParityCall    `json:"calls"`
}

// STEP 1: Subjek — related + calls untuk satu file .kt.
func KotlinParitySubject(in string) (KotlinParityOut, error) {
	rel := []ParityDefSpan{}
	if defs, _, _, _, ok := extractRelated("x.kt", in); ok {
		for _, d := range defs {
			rel = append(rel, ParityDefSpan{Name: d.Name, StartLine: d.Line})
		}
	}
	sort.Slice(rel, func(i, j int) bool {
		if rel[i].StartLine != rel[j].StartLine {
			return rel[i].StartLine < rel[j].StartLine
		}
		return rel[i].Name < rel[j].Name
	})
	calls := []ParityCall{}
	if sites, ok := extractCalls("x.kt", in); ok {
		for _, s := range sites {
			calls = append(calls, ParityCall{Line: s.Line, Caller: s.Caller, Callee: calleeBase(s.Callee)})
		}
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].Line != calls[j].Line {
			return calls[i].Line < calls[j].Line
		}
		if calls[i].Callee != calls[j].Callee {
			return calls[i].Callee < calls[j].Callee
		}
		return calls[i].Caller < calls[j].Caller
	})
	return KotlinParityOut{Related: rel, Calls: calls}, nil
}

// STEP 2: Reference oracle — pemindai baris independen.
//
// Aturan yang ditiru ulang dari spesifikasi (bukan dari kode subjek):
// - Def = keyword deklarasi (fun/class/interface/object/typealias/
//   enum) yang semua token sebelumnya pada baris itu adalah anotasi
//   atau modifier; anchor = baris keyword (kolom modifier pertama
//   pada baris itu, else kolom keyword); nama = identifier pertama
//   sesudah keyword (lewati generics, `class`/`interface` sesudah
//   `fun`, receiver `Type.`).
// - Span def = dari baris anchor sampai baris kedalaman kurung/braces
//   kembali 0 (dengan aturan `{` di baris berikut header masih jadi
//   body, seperti kotlinDeclEnd); def tanpa body berakhir di baris
//   header.
// - Properti top-level = deklarasi val/var yang baris+kolomnya di
//   luar semua span def.
// - Call = pola `IDENT(` di luar string/komentar, kecuali keyword,
//   nama anotasi, konstruktor supertype (`: Bar()` tanpa `=`),
//   dan nama def pada baris start-nya sendiri tanpa delimiter body
//   sebelumnya (fragmen header). Caller = def tersempit yang
//   rentang barisnya memuat baris call.

type parityDef struct {
	name      string
	startLine int
	startCol  int
	endLine   int
	endCol    int
}

type parityProp struct {
	name string
	line int
	col  int
}

var parityKeywords = map[string]bool{
	"class": true, "interface": true, "object": true, "fun": true,
	"typealias": true, "enum": true,
}

var parityModifiers = map[string]bool{
	"public": true, "internal": true, "private": true, "protected": true,
	"open": true, "abstract": true, "data": true, "sealed": true,
	"final": true, "expect": true, "actual": true, "inline": true,
	"value": true, "external": true, "override": true, "crossinline": true,
	"noinline": true, "tailrec": true, "suspend": true, "operator": true,
	"infix": true, "lateinit": true, "const": true, "companion": true,
	"annotation": true,
}

var parityCallKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "when": true, "return": true,
	"fun": true, "class": true, "object": true, "interface": true,
	"enum": true, "val": true, "var": true, "const": true, "import": true,
	"package": true, "try": true, "catch": true, "finally": true,
	"do": true, "else": true, "is": true, "as": true, "in": true,
	"this": true, "super": true, "throw": true,
}

func parityIsIdentStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func parityIsIdent(c byte) bool {
	return parityIsIdentStart(c) || (c >= '0' && c <= '9')
}

// parityCodeLines blanks comments and strings per line with its own
// carried state (block comments, regular + raw strings, char
// literals), independent of stripKotlinCode.
func parityCodeLines(raw []string) []string {
	out := make([]string, len(raw))
	inBlock := false
	inRaw := false
	inStr := false
	for i, ln := range raw {
		var b strings.Builder
		for j := 0; j < len(ln); j++ {
			c := ln[j]
			switch {
			case inBlock:
				if c == '*' && j+1 < len(ln) && ln[j+1] == '/' {
					inBlock = false
					b.WriteString("  ")
					j++
				} else {
					b.WriteByte(' ')
				}
			case inRaw:
				if c == '"' && j+2 < len(ln) && ln[j+1] == '"' && ln[j+2] == '"' {
					inRaw = false
					b.WriteString("   ")
					j += 2
				} else {
					b.WriteByte(' ')
				}
			case inStr:
				if c == '\\' && j+1 < len(ln) {
					b.WriteString("  ")
					j++
				} else if c == '"' {
					inStr = false
					b.WriteByte(' ')
				} else {
					b.WriteByte(' ')
				}
			case c == '/' && j+1 < len(ln) && ln[j+1] == '/':
				for k := j; k < len(ln); k++ {
					b.WriteByte(' ')
				}
				j = len(ln)
			case c == '/' && j+1 < len(ln) && ln[j+1] == '*':
				inBlock = true
				b.WriteString("  ")
				j++
			case c == '"' && j+2 < len(ln) && ln[j+1] == '"' && ln[j+2] == '"':
				inRaw = true
				b.WriteString("   ")
				j += 2
			case c == '"':
				inStr = true
				b.WriteByte(' ')
			case c == '\'':
				// Char literal: blank to the closing quote on this line.
				b.WriteByte(' ')
				j++
				for j < len(ln) && ln[j] != '\'' {
					if ln[j] == '\\' {
						j++
					}
					b.WriteByte(' ')
					j++
				}
				if j < len(ln) {
					b.WriteByte(' ')
				}
			default:
				b.WriteByte(c)
			}
		}
		out[i] = b.String()
	}
	return out
}

// parityTokens splits a blanked line into (token, byte column) pairs.
func parityTokens(line string) [][2]interface{} {
	var out [][2]interface{}
	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '\r') {
			i++
		}
		if i >= len(line) {
			break
		}
		start := i
		for i < len(line) && line[i] != ' ' && line[i] != '\t' && line[i] != '\r' {
			i++
		}
		out = append(out, [2]interface{}{line[start:i], start})
	}
	return out
}

func parityWord(tok string) string {
	w := tok
	for len(w) > 0 && !parityIsIdent(w[len(w)-1]) && w[len(w)-1] != ')' {
		w = w[:len(w)-1]
	}
	return w
}

func parityLastNonSpaceCol(line string) int {
	for i := len(line) - 1; i >= 0; i-- {
		if line[i] != ' ' && line[i] != '\t' && line[i] != '\r' {
			return i
		}
	}
	return 0
}

// parityDefEnd computes (endLine, endCol) for a def starting at
// startLine: scan forward tracking paren/brace depth on blanked
// lines; the def ends where both depths return to 0. A `{` on the
// line right after the header still counts as the body.
func parityDefEnd(codeLines []string, startLine int) (int, int) {
	paren, brace := 0, 0
	for ln := startLine; ln <= len(codeLines); ln++ {
		line := codeLines[ln-1]
		for col := 0; col < len(line); col++ {
			switch line[col] {
			case '(', '[':
				paren++
			case ')', ']':
				if paren > 0 {
					paren--
				}
			case '{':
				if paren == 0 {
					brace++
				}
			case '}':
				if paren == 0 && brace > 0 {
					brace--
					if brace == 0 && paren == 0 {
						return ln, col
					}
				}
			}
		}
		if paren == 0 && brace == 0 {
			if ln == startLine && ln < len(codeLines) &&
				strings.HasPrefix(strings.TrimLeft(codeLines[ln], " \t"), "{") {
				continue
			}
			return ln, parityLastNonSpaceCol(line)
		}
	}
	last := len(codeLines)
	return last, parityLastNonSpaceCol(codeLines[last-1])
}

func parityHasIdent(tok string) bool {
	for i := 0; i < len(tok); i++ {
		if parityIsIdentStart(tok[i]) {
			return true
		}
	}
	return false
}

// parityScanDefs finds defs and top-level property candidates.
func parityScanDefs(codeLines []string) ([]parityDef, []parityProp) {
	var defs []parityDef
	var props []parityProp
	for li, line := range codeLines {
		ln := li + 1
		toks := parityTokens(line)
		kwAt := -1
		kw := ""
		anchorCol := -1
		for fi, t := range toks {
			tok := t[0].(string)
			col := t[1].(int)
			// Pure punctuation left by blanked strings (`)` from
			// `@Deprecated(" ")`) carries no meaning: skip it instead
			// of letting it stop the keyword search.
			if !parityHasIdent(tok) {
				continue
			}
			word := parityWord(tok)
			if parityKeywords[word] || word == "val" || word == "var" {
				ok := true
				for _, pre := range toks[:fi] {
					ptok := pre[0].(string)
					if !parityHasIdent(ptok) {
						continue
					}
					p := parityWord(ptok)
					if strings.HasPrefix(p, "@") || parityModifiers[p] || p == "const" {
						continue
					}
					ok = false
					break
				}
				if ok {
					kwAt = fi
					kw = word
					anchorCol = col
					for _, pre := range toks[:fi] {
						ptok := pre[0].(string)
						if !parityHasIdent(ptok) {
							continue
						}
						p := parityWord(ptok)
						if parityModifiers[p] {
							anchorCol = pre[1].(int)
							break
						}
					}
				}
				break
			}
			if !strings.HasPrefix(tok, "@") && !parityModifiers[tok] && tok != "const" {
				break
			}
		}
		if kwAt < 0 {
			continue
		}
		// Name: first identifier after the keyword.
		fi := kwAt + 1
		if kw == "fun" && fi < len(toks) && (toks[fi][0].(string) == "class" || toks[fi][0].(string) == "interface") {
			fi++
		}
		if fi < len(toks) && strings.HasPrefix(toks[fi][0].(string), "<") {
			for fi < len(toks) && !strings.Contains(toks[fi][0].(string), ">") {
				fi++
			}
			if fi < len(toks) {
				fi++
			}
		}
		name := ""
		nameCol := -1
		if fi < len(toks) {
			tok := toks[fi][0].(string)
			if dot := strings.LastIndex(tok, "."); dot >= 0 {
				tok = tok[dot+1:]
			}
			if len(tok) > 0 && parityIsIdentStart(tok[0]) {
				j := 0
				for j < len(tok) && parityIsIdent(tok[j]) {
					j++
				}
				name = tok[:j]
				nameCol = toks[fi][1].(int)
			}
		}
		if name == "" {
			continue
		}
		if kw == "val" || kw == "var" {
			props = append(props, parityProp{name: name, line: ln, col: nameCol})
			continue
		}
		endLine, endCol := parityDefEnd(codeLines, ln)
		defs = append(defs, parityDef{name: name, startLine: ln, startCol: anchorCol, endLine: endLine, endCol: endCol})
	}
	return defs, props
}

func parityPropInside(p parityProp, d parityDef) bool {
	if p.line > d.startLine && p.line < d.endLine {
		return true
	}
	if p.line == d.startLine && p.line == d.endLine {
		return p.col > d.startCol && p.col < d.endCol
	}
	if p.line == d.startLine {
		return p.col > d.startCol
	}
	if p.line == d.endLine {
		return p.col < d.endCol
	}
	return false
}

var parityCallRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*(?:<[^<>\n]*>)?\s*\(`)

// parityFindCalls scans blanked lines for IDENT( patterns.
func parityFindCalls(codeLines []string, defs []parityDef) []ParityCall {
	var out []ParityCall
	for li, line := range codeLines {
		ln := li + 1
		for _, m := range parityCallRe.FindAllStringSubmatchIndex(line, -1) {
			ident := line[m[2]:m[3]]
			col := m[2]
			if parityCallKeywords[ident] {
				continue
			}
			// Annotation: `@Foo(`.
			if col > 0 && line[col-1] == '@' {
				continue
			}
			// Supertype constructor: `:` at depth 0 before the call
			// and no `=` at depth 0 before it (`: Bar()` vs
			// `val x: Foo = Bar()`).
			prefix := line[:col]
			depth := 0
			sawColon, sawEq := false, false
			for _, r := range prefix {
				switch r {
				case '(', '[':
					depth++
				case ')', ']':
					if depth > 0 {
						depth--
					}
				case ':':
					if depth == 0 {
						sawColon = true
					}
				case '=':
					if depth == 0 {
						sawEq = true
					}
				}
			}
			if sawColon && !sawEq {
				continue
			}
			// Declaration header: the def's own name on its start
			// line with no body delimiter before it.
			isHeader := false
			for _, d := range defs {
				if d.name != ident || d.startLine != ln {
					continue
				}
				depth = 0
				delim := false
				for _, r := range prefix {
					switch r {
					case '(', '[':
						depth++
					case ')', ']':
						if depth > 0 {
							depth--
						}
					case '=', '{':
						if depth == 0 {
							delim = true
						}
					}
				}
				if !delim {
					isHeader = true
				}
				break
			}
			if isHeader {
				continue
			}
			// Caller: smallest enclosing def span (line ranges).
			caller := ""
			bestSpan := 0
			for _, d := range defs {
				if d.startLine <= ln && ln <= d.endLine {
					if span := d.endLine - d.startLine; caller == "" || span < bestSpan {
						caller, bestSpan = d.name, span
					}
				}
			}
			out = append(out, ParityCall{Line: ln, Caller: caller, Callee: ident})
		}
	}
	return out
}

func KotlinParityOracle(in string) (KotlinParityOut, error) {
	norm := strings.ReplaceAll(strings.ReplaceAll(in, "\r\n", "\n"), "\r", "\n")
	codeLines := parityCodeLines(strings.Split(norm, "\n"))
	defs, props := parityScanDefs(codeLines)
	rel := []ParityDefSpan{}
	for _, d := range defs {
		rel = append(rel, ParityDefSpan{Name: d.name, StartLine: d.startLine})
	}
	for _, p := range props {
		inside := false
		for _, d := range defs {
			if parityPropInside(p, d) {
				inside = true
				break
			}
		}
		if !inside {
			rel = append(rel, ParityDefSpan{Name: p.name, StartLine: p.line})
		}
	}
	sort.Slice(rel, func(i, j int) bool {
		if rel[i].StartLine != rel[j].StartLine {
			return rel[i].StartLine < rel[j].StartLine
		}
		return rel[i].Name < rel[j].Name
	})
	calls := parityFindCalls(codeLines, defs)
	if calls == nil {
		calls = []ParityCall{}
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].Line != calls[j].Line {
			return calls[i].Line < calls[j].Line
		}
		if calls[i].Callee != calls[j].Callee {
			return calls[i].Callee < calls[j].Callee
		}
		return calls[i].Caller < calls[j].Caller
	})
	return KotlinParityOut{Related: rel, Calls: calls}, nil
}

// STEP 3: SuiteSpec 4 gerbang.
func BuildSuiteSpec() SuiteSpec[string, KotlinParityOut] {
	opts := DefaultSuiteOptions()

	return SuiteSpec[string, KotlinParityOut]{
		Name:            "KotlinParity",
		Domain:          "low-level",
		Subject:         KotlinParitySubject,
		ReferenceOracle: KotlinParityOracle,
		Options:         &opts,

		PrimaryCases: []PrimaryCase[string, KotlinParityOut]{
			{
				Name:  "Probe bedah: related + calls fixture parity",
				Input: "package demo\n\n" + "@Composable internal fun PrimaryChip(label: String) {\n" + "    val localTmp = label.length\n" + "    renderIcon(label)\n" + "}\n\n" + "@Composable fun ScreenView() {\n" + "    PrimaryChip(\"ok\")\n" + "}\n\n" + "@Composable internal fun ChipAlias(x: String) = PrimaryChip(x)\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "PrimaryChip", StartLine: 3},
						{Name: "ScreenView", StartLine: 8},
						{Name: "ChipAlias", StartLine: 12},
					},
					Calls: []ParityCall{
						{Line: 5, Caller: "PrimaryChip", Callee: "renderIcon"},
						{Line: 9, Caller: "ScreenView", Callee: "PrimaryChip"},
						{Line: 12, Caller: "ChipAlias", Callee: "PrimaryChip"},
					},
				},
				// Kasus pertama: parse Kotlin pertama membayar
				// lazy-load grammar gotreesitter sekali (~150ms);
				// anggaran ini hanya untuk cold-start.
				MaxDurationMs: 500,
			},
			{
				Name:  "Anotasi baris sendiri sebelum fun + class satu baris",
				Input: "package demo\n\n@Composable\ninternal fun SeparateChip(label: String) {\n    renderIcon(label)\n}\n\n@Deprecated(\"old\") internal class LegacyBox {\n    val flag = 1\n}\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "SeparateChip", StartLine: 4},
						{Name: "LegacyBox", StartLine: 8},
					},
					Calls: []ParityCall{
						{Line: 5, Caller: "SeparateChip", Callee: "renderIcon"},
					},
				},
			},
			{
				Name:  "Komentar trailing di baris anotasi bukan anchor",
				Input: "package demo\n\n@Suppress(\"TooManyFunctions\") // Owns everything the chat renders.\nclass ChatTimeline(\n  private val clock: () -> Long = System::currentTimeMillis,\n) {\n  private val lock = Any()\n}\n\n@Composable internal fun BrokenChip(label: String) {}\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "ChatTimeline", StartLine: 4},
						{Name: "BrokenChip", StartLine: 10},
					},
					Calls: []ParityCall{
						{Line: 7, Caller: "ChatTimeline", Callee: "Any"},
					},
				},
			},
			{
				Name:  "Extension fun ber-receiver dua segmen dinamai fungsinya",
				Input: "package demo\n\ninternal fun TimelineItem.ToolCall.cardText(): String {\n  return editPairs(args)\n}\n\n@Composable internal fun BrokenChip(label: String) {}\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "cardText", StartLine: 3},
						{Name: "BrokenChip", StartLine: 7},
					},
					Calls: []ParityCall{
						{Line: 4, Caller: "cardText", Callee: "editPairs"},
					},
				},
			},
			{
				Name:  "Body satu baris dan single-expression tetap terhitung",
				Input: "package p\n\nfun wrap(x: Int) = helper(x)\n\nfun single() { helper() }\n\nfun helper(x: Int = 0) {}\n\nval top = 1\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "wrap", StartLine: 3},
						{Name: "single", StartLine: 5},
						{Name: "helper", StartLine: 7},
						{Name: "top", StartLine: 9},
					},
					Calls: []ParityCall{
						{Line: 3, Caller: "wrap", Callee: "helper"},
						{Line: 5, Caller: "single", Callee: "helper"},
					},
				},
			},
			{
				Name:  "Delegasi supertype bukan call",
				Input: "package p\n\nopen class Base\n\nclass Child : Base() {\n    fun run() {\n        helper()\n    }\n}\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "Base", StartLine: 3},
						{Name: "Child", StartLine: 5},
						{Name: "run", StartLine: 6},
					},
					Calls: []ParityCall{
						{Line: 7, Caller: "run", Callee: "helper"},
					},
				},
			},
			{
				Name:  "Kotlin polos tidak berubah: member val bukan top-level",
				Input: "package p\n\nclass Host {\n    val count = 0\n    fun start(): Boolean {\n        val ok = true\n        return ok\n    }\n}\n\nval appName = \"pipkin\"\n\nfun topLevel() {}\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "Host", StartLine: 3},
						{Name: "start", StartLine: 5},
						{Name: "appName", StartLine: 11},
						{Name: "topLevel", StartLine: 13},
					},
					Calls: []ParityCall{},
				},
			},
		},

		PoisonInputs: []PoisonCase[string, KotlinParityOut]{
			{
				Name:        "String kosong mengembalikan slice kosong non-nil",
				Input:       "",
				ExpectError: false,
				InvariantCheck: func(actual KotlinParityOut, _ string) bool {
					return actual.Related != nil && actual.Calls != nil &&
						len(actual.Related) == 0 && len(actual.Calls) == 0
				},
				FailureHint: "Input kosong harus out kosong non-nil, bukan error.",
			},
			{
				Name:        "Hanya komentar mengembalikan kosong",
				Input:       "package p\n\n// fun Fake() {}\n/* class AlsoFake {} */\n",
				ExpectError: false,
				InvariantCheck: func(actual KotlinParityOut, _ string) bool {
					return len(actual.Related) == 0 && len(actual.Calls) == 0
				},
				FailureHint: "Deklarasi di dalam komentar tidak boleh jadi def/call.",
			},
			{
				Name:        "Block comment tidak tertutup tidak boleh panic",
				Input:       "package p\n\n/* terbuka\nfun Fake() {}\n",
				ExpectError: false,
				InvariantCheck: func(actual KotlinParityOut, _ string) bool {
					return len(actual.Related) == 0
				},
				FailureHint: "Komentar tak tertutup harus fail-open (nol def), bukan panic.",
			},
			{
				Name:        "String tidak tertutup tidak boleh panic",
				Input:       "package p\n\nval s = \"abc\nfun Foo() {}\n",
				ExpectError: false,
				InvariantCheck: func(actual KotlinParityOut, _ string) bool {
					return len(actual.Related) >= 1
				},
				FailureHint: "String tak tertutup harus fail-open, bukan panic.",
			},
			{
				Name:        "Deklarasi di dalam string tidak boleh jadi def",
				Input:       "package p\n\nval s = \"fun Fake() {}\"\nfun Real() {}\n",
				ExpectError: false,
				InvariantCheck: func(actual KotlinParityOut, _ string) bool {
					// `s` itself is a genuine top-level val; Fake
					// inside the string must not appear.
					names := map[string]bool{}
					for _, r := range actual.Related {
						names[r.Name] = true
					}
					return len(actual.Related) == 2 && names["s"] && names["Real"] &&
						len(actual.Calls) == 0
				},
				FailureHint: "Hanya s (top-level val) dan Real yang boleh jadi related; Fake di dalam string tidak.",
			},
			{
				Name:        "CRLF sama dengan LF",
				Input:       "package p\r\n\r\ninternal fun Foo() {}\r\n",
				ExpectError: false,
				InvariantCheck: func(actual KotlinParityOut, _ string) bool {
					return len(actual.Related) == 1 && actual.Related[0].Name == "Foo" &&
						actual.Related[0].StartLine == 3
				},
				FailureHint: "Line ending CRLF tidak boleh menggeser nomor baris.",
			},
		},

		GeneratedSamples: []string{
			"package p\n\nfun Plain() {}\n",
			"package p\n\ninternal fun Foo() {}\n",
			"package p\n\n@Composable\nfun Plain() {}\n",
			"package p\n\n@Composable\ninternal fun Foo() {}\n",
			"package p\n\n/** Doc. */\n@Composable\ninternal fun Foo() {}\n",
			"package p\n\n@Deprecated(\"x\")\nprivate fun Foo() {}\n",
			"package p\n\ninternal @Composable fun Foo() {}\n",
			"package p\n\n@JvmStatic\nexternal fun Foo(): String\n",
			"package p\n\ninternal object Bridge {\n  @JvmStatic\n  external fun version(): String\n}\n",
			"package p\n\ndata class Point(val x: Int, val y: Int)\n",
			"package p\n\nsealed interface Shape\n",
			"package p\n\nfun interface Handler {\n  fun handle(): Unit\n}\n",
			"package p\n\nfun String.shout(): String = uppercase()\n",
			"package p\n\nfun <T> first(items: List<T>): T = items[0]\n",
			"package p\n\nclass Host {\n  fun start() {\n    val ok = true\n  }\n}\n",
			"package p\n\nclass Host {\n  companion object {\n    fun create(): Host = Host()\n  }\n}\n",
			"package p\n\ntypealias Handler = (String) -> Unit\n",
			"package p\n\ninternal val count = 0\n",
			"package p\n\ninternal fun multi(\n  a: Int,\n  b: String,\n): String = baz(\n  a,\n  b,\n)\n",
			"package p\n\n@Composable\ninternal fun A() {}\n\n@Composable\ninternal fun B() {}\n",
			"package demo\n\n@Composable internal fun PrimaryChip(label: String) {\n    val localTmp = label.length\n    renderIcon(label)\n}\n\n@Composable fun ScreenView() {\n    PrimaryChip(\"ok\")\n}\n\n@Composable internal fun ChipAlias(x: String) = PrimaryChip(x)\n",
			"package p\n\nopen class Base\n\nclass Child : Base() {\n  fun run() {\n    helper()\n  }\n}\n\nfun helper() {}\n",
			"package p\n\nfun single() { helper() }\n\nfun helper() {}\n",
			"package p\n\n@Deprecated(\"x\") internal object Solo { val y = 1 }\n",
			"package p\n\nval top = 1\n\nfun f() {\n  val local = 2\n  use(top)\n}\n",
		},

		RegressionSuite: []RegressionCase[string, KotlinParityOut]{
			{
				ID:    "REG-PARITY-PROBE-01",
				Name:  "Probe bedah 2026-11-20: tiga mode gagal sekaligus",
				Input: "package demo\n\n" + "@Composable internal fun PrimaryChip(label: String) {\n" + "    val localTmp = label.length\n" + "    renderIcon(label)\n" + "}\n\n" + "@Composable fun ScreenView() {\n" + "    PrimaryChip(\"ok\")\n" + "}\n\n" + "@Composable internal fun ChipAlias(x: String) = PrimaryChip(x)\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "PrimaryChip", StartLine: 3},
						{Name: "ScreenView", StartLine: 8},
						{Name: "ChipAlias", StartLine: 12},
					},
					Calls: []ParityCall{
						{Line: 5, Caller: "PrimaryChip", Callee: "renderIcon"},
						{Line: 9, Caller: "ScreenView", Callee: "PrimaryChip"},
						{Line: 12, Caller: "ChipAlias", Callee: "PrimaryChip"},
					},
				},
			},
			{
				ID:    "REG-PARITY-DEFAULT-ARG-02",
				Name:  "Trade-off terdokumentasi: panggilan rekursif di argumen default ikut terfilter (filter posisi/bentuk tidak bisa membedakannya dari fragmen header)",
				Input: "package p\n\nfun foo(x: Int = foo(0)) {}\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "foo", StartLine: 3},
					},
					Calls: []ParityCall{},
				},
			},
			{
				ID:    "REG-PARITY-ONE-LINE-OBJECT-03",
				Name:  "Object satu baris: member val di dalam byte span, bukan top-level",
				Input: "package p\n\n@Deprecated(\"x\") internal object Solo { val y = 1 }\n",
				Expected: KotlinParityOut{
					Related: []ParityDefSpan{
						{Name: "Solo", StartLine: 3},
					},
					Calls: []ParityCall{},
				},
			},
		},
	}
}

// STEP 4: Runner (sama seperti template).
func main() {
	spec := BuildSuiteSpec()
	report := RunAgentVerificationSuite(spec)
	exitCode := PrintVerificationReport(report)
	os.Exit(exitCode)
}
