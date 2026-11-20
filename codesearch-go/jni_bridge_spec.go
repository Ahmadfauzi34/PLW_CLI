//go:build verify

// ===========================================================================
// Penempatan skill Go verification-harness di codesearch, untuk JNI bridge
// --related dua arah (go14, P2 — Option 3, Base-Key Grouping)
// ===========================================================================
// - harness.go di direktori ini adalah SALINAN VERBATIM dari skill
//   local-verification-harness-go (core TIDAK diubah; yang dicustom
//   hanya file spec ini, sesuai aturan skill).
// - File ini memakai build tag "verify": build/test/vet normal
//   (tanpa tag) mengabaikannya, sehingga tidak ada main() ganda dengan
//   main.go. Jalankan HANYA lewat ./verify-jni-bridge.sh, yang memakai
//   `go run` dengan daftar file eksplisit (mode file mengabaikan tag)
//   dan TIDAK mencantumkan main.go.
// - Subjek: jawaban --related (baris def + ffi) untuk satu pasang file
//   (Bridge.kt + native.rs) lewat extractRelated + resolveRelated.
//   Perilaku yang dikunci: satu baris ffi per overload Rust untuk query
//   Kotlin; query Rust (nama JNI penuh) menyaring decl Kotlin lewat
//   arity + coarse token bila membawa signature, dan fail-open bila
//   tidak; wildcard (nullable/generik/custom) tidak pernah mem-veto;
//   nama JNI malformed tidak menghasilkan baris ffi.
// - Oracle: pemindai baris independen (tanpa tree-sitter) yang
//   menghitung decl Kotlin (package + rantai class dari kedalaman
//   brace + external fun satu baris), bridge Rust (fn top-level
//   berawalan Java_), dan aturan resolusi Option 3 dari teks mentah.
//   Ekuivalensi gerbang 3 membandingkan (path, line, kind) — Detail
//   tidak dibandingkan (string detail adalah presentasi, dikunci oleh
//   unit test related_test.go).
// - Batas oracle (terdokumentasi, bukan diam): signature multi-baris
//   tidak dimodelkan; blok impl Rust tidak dimodelkan (method di impl
//   bukan JNI entry point — dikunci unit test TestExtractRelatedRustJNI);
//   parameter bertipe fungsi `(Int) -> Unit` tidak dimodelkan (regex
//   param berhenti di `)` pertama); nilai default berisi koma tidak
//   dimodelkan; fungsi generik `fun <T> name` tidak dimodelkan;
//   string berisi brace tidak dimodelkan (pelacak brace naif).
//   Matematika base-key (escaping, pemisah `__`, parse signature,
//   tabel coarse) ditulis ulang di oracle dari teks spesifikasi JNI —
//   implementasi yang sama dengan subjek, tetapi dikunci terpisah oleh
//   unit test (TestJNIEscapeAndKotlinBase, TestParseJNISig,
//   TestParseJNIRustName, TestJniCoarseToken di related_test.go),
//   bukan oleh harness ini.
// - MaxDurationMs dinaikkan dari default 50ms ke 500ms: subjek
//   mem-parse DUA file (tree-sitter per file) + resolve; 50ms adalah
//   ambang untuk satu file, bukan dua.

package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// --- Input / output ---------------------------------------------------------

type JNIProbe struct {
	Kt    string `json:"kt"`
	Rs    string `json:"rs"`
	Query string `json:"query"`
}

type JNIRow struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
}

type JNIRows struct {
	Rows []JNIRow `json:"rows"`
}

func jniRowsOf(entries []relEntry) JNIRows {
	rows := []JNIRow{}
	for _, e := range entries {
		rows = append(rows, JNIRow{Path: e.Path, Line: e.Line, Kind: e.Kind})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Path != rows[j].Path {
			return rows[i].Path < rows[j].Path
		}
		if rows[i].Line != rows[j].Line {
			return rows[i].Line < rows[j].Line
		}
		return rows[i].Kind < rows[j].Kind
	})
	return JNIRows{Rows: rows}
}

// JNIBridgeSubject is the harness Subject: the production --related
// answer (def + ffi rows) for one Kotlin + Rust fixture pair.
func JNIBridgeSubject(in JNIProbe) (JNIRows, error) {
	var defs []relDef
	var edges []relEdge
	var bridges []relBridge
	var jni []relJNIDecl
	if in.Kt != "" {
		d, e, b, j, ok := extractRelated("Bridge.kt", in.Kt)
		if !ok {
			return JNIRows{}, fmt.Errorf("extractRelated(Bridge.kt) failed")
		}
		defs, edges, bridges, jni = append(defs, d...), append(edges, e...), append(bridges, b...), append(jni, j...)
	}
	if in.Rs != "" {
		d, e, b, j, ok := extractRelated("native.rs", in.Rs)
		if !ok {
			return JNIRows{}, fmt.Errorf("extractRelated(native.rs) failed")
		}
		defs, edges, bridges, jni = append(defs, d...), append(edges, e...), append(bridges, b...), append(jni, j...)
	}
	files := map[string]bool{"Bridge.kt": true, "native.rs": true}
	return jniRowsOf(resolveRelated(in.Query, defs, edges, bridges, jni, files)), nil
}

// --- Oracle (independent line scanner) --------------------------------------

// jniOracleEscape re-implements the JNI name escaping from the spec
// text (not by calling the production jniEscape).
func jniOracleEscape(s string) string {
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

func jniOracleBase(pkg string, chain []string, method string) string {
	var parts []string
	if pkg != "" {
		for _, seg := range strings.Split(pkg, ".") {
			parts = append(parts, jniOracleEscape(seg))
		}
	}
	parts = append(parts, jniOracleEscape(strings.Join(chain, "$")))
	parts = append(parts, jniOracleEscape(method))
	return "Java_" + strings.Join(parts, "_")
}

// jniOracleParseSig re-implements the mangled-signature parser from
// the spec text.
func jniOracleParseSig(sig string) ([]string, bool) {
	var tokens []string
	for len(sig) > 0 {
		rest := sig
		for strings.HasPrefix(sig, "_3") {
			sig = sig[2:]
		}
		if len(sig) == 0 {
			return nil, false
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

func jniOracleParseName(name string) (base string, tokens []string, overloaded, ok bool) {
	if !strings.HasPrefix(name, "Java_") {
		return "", nil, false, false
	}
	base, raw, cut := strings.Cut(name, "__")
	if !cut {
		return name, nil, false, true
	}
	tokens, ok = jniOracleParseSig(raw)
	if !ok {
		return "", nil, false, false
	}
	return base, tokens, true, true
}

var jniOracleCoarseTable = map[string]string{
	"Byte": "B", "Char": "C", "Double": "D", "Float": "F",
	"Int": "I", "Long": "J", "Short": "S", "Boolean": "Z",
	"String":    "Ljava_lang_String_2",
	"ByteArray": "_3B", "CharArray": "_3C", "DoubleArray": "_3D",
	"FloatArray": "_3F", "IntArray": "_3I", "LongArray": "_3J",
	"ShortArray": "_3S", "BooleanArray": "_3Z",
}

func jniOracleCoarse(typ string, vararg bool) string {
	t := strings.TrimSpace(typ)
	if strings.HasSuffix(t, "?") {
		return ""
	}
	tok, ok := jniOracleCoarseTable[t]
	if !ok {
		return ""
	}
	if vararg {
		return "_3" + tok
	}
	return tok
}

// splitTopLevel splits s on sep at depth 0 of <>, (), [].
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<', '(', '[':
			depth++
		case '>', ')', ']':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

type jniOracleDef struct {
	line int
	name string
	path string
}

type jniOracleDecl struct {
	line   int
	name   string
	base   string
	arity  int
	coarse []string
}

type jniOracleBridge struct {
	line int
	name string
}

var (
	reOraclePackage = regexp.MustCompile(`(?m)^package\s+([\w.]+)`)
	reOracleClass   = regexp.MustCompile(`(?:^|\s)(?:class|object)\s+(\w+)`)
	reOracleFun     = regexp.MustCompile(`(?:^|\s)(external\s+)?fun\s+(\w+)\s*\(([^)]*)\)`)
	reOracleRustFn  = regexp.MustCompile(`^\s*(?:pub\s+)?fn\s+(\w+)\s*\(`)
)

// jniOracleScan parses the Kotlin fixture into named defs + JNI decls
// and the Rust fixture into named defs + JNI bridges, using only line
// patterns. Frames are pushed only for declarations whose line opens
// a brace (a bodyless `data class` opens no frame).
func jniOracleScan(kt, rs string) (defs []jniOracleDef, decls []jniOracleDecl, bridges []jniOracleBridge) {
	pkg := ""
	if m := reOraclePackage.FindStringSubmatch(kt); m != nil {
		pkg = m[1]
	}
	type frame struct {
		name  string
		depth int
	}
	var stack []frame
	depth := 0
	for i, line := range strings.Split(strings.ReplaceAll(kt, "\r\n", "\n"), "\n") {
		lineno := i + 1
		delta := strings.Count(line, "{") - strings.Count(line, "}")
		if strings.Contains(line, "{") {
			if strings.Contains(line, "companion object") {
				stack = append(stack, frame{name: "", depth: depth + delta})
			} else if m := reOracleClass.FindStringSubmatch(line); m != nil {
				stack = append(stack, frame{name: m[1], depth: depth + delta})
			}
		}
		if m := reOracleFun.FindStringSubmatch(line); m != nil {
			isExternal := strings.TrimSpace(m[1]) == "external"
			name := m[2]
			params := strings.TrimSpace(m[3])
			defs = append(defs, jniOracleDef{line: lineno, name: name, path: "Bridge.kt"})
			if isExternal {
				var coarse []string
				arity := 0
				if params != "" {
					for _, p := range splitTopLevel(params, ',') {
						p = strings.TrimSpace(p)
						if p == "" {
							continue
						}
						arity++
						vararg := false
						typ := ""
						if idx := strings.Index(p, ":"); idx >= 0 {
							before := strings.TrimSpace(p[:idx])
							vararg = strings.Contains(before, "vararg")
							typ = strings.TrimSpace(p[idx+1:])
							if eq := strings.Index(typ, "="); eq >= 0 {
								typ = strings.TrimSpace(typ[:eq])
							}
						}
						coarse = append(coarse, jniOracleCoarse(typ, vararg))
					}
				}
				var chain []string
				for _, f := range stack {
					if f.name != "" {
						chain = append(chain, f.name)
					} else if len(chain) > 0 {
						chain[len(chain)-1] += "$Companion"
					}
				}
				if len(chain) == 0 {
					chain = []string{"BridgeKt"}
				}
				decls = append(decls, jniOracleDecl{
					line: lineno, name: name,
					base:  jniOracleBase(pkg, chain, name),
					arity: arity, coarse: coarse,
				})
			}
		}
		depth += delta
		for len(stack) > 0 && depth < stack[len(stack)-1].depth {
			stack = stack[:len(stack)-1]
		}
	}
	for i, line := range strings.Split(strings.ReplaceAll(rs, "\r\n", "\n"), "\n") {
		lineno := i + 1
		if m := reOracleRustFn.FindStringSubmatch(line); m != nil {
			name := m[1]
			defs = append(defs, jniOracleDef{line: lineno, name: name, path: "native.rs"})
			if strings.HasPrefix(name, "Java_") {
				bridges = append(bridges, jniOracleBridge{line: lineno, name: name})
			}
		}
	}
	return defs, decls, bridges
}

// JNIBridgeOracle answers the same query as the subject, from the
// line-scanned defs/decls/bridges, implementing Option 3 from the design.
func JNIBridgeOracle(in JNIProbe) (JNIRows, error) {
	defs, decls, bridges := jniOracleScan(in.Kt, in.Rs)
	// want = trailing identifier of the query (calleeBase, minimal).
	want := in.Query
	for _, sep := range []string{".", "::", ":"} {
		if idx := strings.LastIndex(want, sep); idx >= 0 {
			want = want[idx+len(sep):]
		}
	}
	rows := []JNIRow{}
	add := func(r JNIRow) {
		for _, e := range rows {
			if e == r {
				return
			}
		}
		rows = append(rows, r)
	}
	for _, d := range defs {
		if d.name == want || d.name == in.Query {
			add(JNIRow{Path: d.path, Line: d.line, Kind: "def"})
		}
	}
	if qBase, qTokens, qOverloaded, qIsJNI := jniOracleParseName(in.Query); qIsJNI {
		for _, d := range decls {
			if d.base != qBase {
				continue
			}
			if qOverloaded {
				if d.arity != len(qTokens) || len(d.coarse) != len(qTokens) {
					continue
				}
				compat := true
				for i := range d.coarse {
					if d.coarse[i] != "" && d.coarse[i] != qTokens[i] {
						compat = false
						break
					}
				}
				if !compat {
					continue
				}
			}
			add(JNIRow{Path: "Bridge.kt", Line: d.line, Kind: "ffi"})
		}
	} else {
		for _, d := range decls {
			if d.name != want {
				continue
			}
			for _, b := range bridges {
				if base, _, _, ok := jniOracleParseName(b.name); ok && base == d.base {
					add(JNIRow{Path: "native.rs", Line: b.line, Kind: "ffi"})
				}
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Path != rows[j].Path {
			return rows[i].Path < rows[j].Path
		}
		if rows[i].Line != rows[j].Line {
			return rows[i].Line < rows[j].Line
		}
		return rows[i].Kind < rows[j].Kind
	})
	return JNIRows{Rows: rows}, nil
}

// --- Fixtures -----------------------------------------------------------------

const jniKtE2E = "package com.example\n\n" +
	"class Bridge {\n" +
	"    external fun send(data: ByteArray, count: Int): Boolean\n" +
	"    external fun send(text: String): Boolean\n" +
	"    external fun ping(): Unit\n" +
	"    fun plain() {}\n" +
	"}\n\n" +
	"external fun topLevel(flag: Boolean)\n"

const jniRsE2E = "pub fn Java_com_example_Bridge_send___3BI(data: &[u8], count: i32) -> bool { true }\n" +
	"pub fn Java_com_example_Bridge_send__Ljava_lang_String_2(text: &str) -> bool { true }\n" +
	"pub fn Java_com_example_Bridge_ping() {}\n" +
	"pub fn Java_com_example_BridgeKt_topLevel__Z(flag: bool) {}\n" +
	"pub fn plain() {}\n"

const jniKtCompanion = "package com.example\n\n" +
	"class Outer {\n" +
	"    companion object {\n" +
	"        external fun create(name: String?): Widget\n" +
	"    }\n" +
	"}\n"

const jniRsCompanion = "pub fn Java_com_example_Outer_00024Companion_create__Ljava_lang_String_2(name: &str) {}\n"

const jniKtCoarse = "package com.example\n\n" +
	"class Bridge {\n" +
	"    external fun send(a: Int): Unit\n" +
	"    external fun send(a: String): Unit\n" +
	"}\n"

const jniRsCoarse = "pub fn Java_com_example_Bridge_send__I(a: i32) {}\n" +
	"pub fn Java_com_example_Bridge_send__Ljava_lang_String_2(a: &str) {}\n"

const jniKtNoPkg = "class Bridge {\n" +
	"    external fun send(x: Int)\n" +
	"}\n"

const jniRsNoPkg = "pub fn Java_Bridge_send__I(x: i32) {}\n"

const jniKtVararg = "package com.example\n\n" +
	"class Bridge {\n" +
	"    external fun sum(vararg xs: Int): Int\n" +
	"}\n"

const jniRsVararg = "pub fn Java_com_example_Bridge_sum___3I(xs: &[i32]) -> i32 { 0 }\n"

// --- SuiteSpec 4 gerbang ------------------------------------------------------

func BuildSuiteSpec() SuiteSpec[JNIProbe, JNIRows] {
	opts := DefaultSuiteOptions()
	opts.MaxDurationMs = 500.0

	return SuiteSpec[JNIProbe, JNIRows]{
		Name:            "JNIBridge",
		Domain:          "low-level",
		Subject:         JNIBridgeSubject,
		ReferenceOracle: JNIBridgeOracle,
		Options:         &opts,

		PrimaryCases: []PrimaryCase[JNIProbe, JNIRows]{
			{
				Name:  "Kotlin query send: 2 def + 2 ffi overload",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "send"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 4, Kind: "def"},
					{Path: "Bridge.kt", Line: 5, Kind: "def"},
					{Path: "native.rs", Line: 1, Kind: "ffi"},
					{Path: "native.rs", Line: 2, Kind: "ffi"},
				}},
			},
			{
				Name:  "Rust query send___3BI: def + tepat decl ByteArray",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "Java_com_example_Bridge_send___3BI"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 4, Kind: "ffi"},
					{Path: "native.rs", Line: 1, Kind: "def"},
				}},
			},
			{
				Name:  "Rust query send__String: def + tepat decl String",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "Java_com_example_Bridge_send__Ljava_lang_String_2"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 5, Kind: "ffi"},
					{Path: "native.rs", Line: 2, Kind: "def"},
				}},
			},
			{
				Name:  "Rust query short name: fail-open, kedua decl tampil",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "Java_com_example_Bridge_send"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 4, Kind: "ffi"},
					{Path: "Bridge.kt", Line: 5, Kind: "ffi"},
				}},
			},
			{
				Name:  "ping dua arah (Kotlin)",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "ping"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 6, Kind: "def"},
					{Path: "native.rs", Line: 3, Kind: "ffi"},
				}},
			},
			{
				Name:  "ping dua arah (Rust)",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "Java_com_example_Bridge_ping"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 6, Kind: "ffi"},
					{Path: "native.rs", Line: 3, Kind: "def"},
				}},
			},
			{
				Name:  "topLevel facade <FileStem>Kt dua arah",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "topLevel"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 10, Kind: "def"},
					{Path: "native.rs", Line: 4, Kind: "ffi"},
				}},
			},
			{
				Name:  "plain tanpa JNI: def saja, tanpa ffi",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "plain"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 7, Kind: "def"},
					{Path: "native.rs", Line: 5, Kind: "def"},
				}},
			},
			{
				Name:  "companion $Companion + nullable wildcard tidak mem-veto",
				Input: JNIProbe{Kt: jniKtCompanion, Rs: jniRsCompanion, Query: "Java_com_example_Outer_00024Companion_create__Ljava_lang_String_2"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 5, Kind: "ffi"},
					{Path: "native.rs", Line: 1, Kind: "def"},
				}},
			},
			{
				Name:  "coarse memisah overload arity-sama (Int vs String)",
				Input: JNIProbe{Kt: jniKtCoarse, Rs: jniRsCoarse, Query: "Java_com_example_Bridge_send__I"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 4, Kind: "ffi"},
					{Path: "native.rs", Line: 1, Kind: "def"},
				}},
			},
		},

		PoisonInputs: []PoisonCase[JNIProbe, JNIRows]{
			{
				Name:        "fixture kosong: tanpa baris, tanpa error",
				Input:       JNIProbe{Kt: "", Rs: "", Query: "send"},
				ExpectError: false,
				InvariantCheck: func(actual JNIRows, _ JNIProbe) bool {
					return actual.Rows != nil && len(actual.Rows) == 0
				},
				FailureHint: "Fixture kosong harus memberi Rows non-nil kosong, bukan nil atau error.",
			},
			{
				Name:        "query kosong: tanpa baris, tanpa error",
				Input:       JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: ""},
				ExpectError: false,
				InvariantCheck: func(actual JNIRows, _ JNIProbe) bool {
					return len(actual.Rows) == 0
				},
				FailureHint: "Query kosong bukan error — tidak ada nama yang dicari.",
			},
			{
				Name:        "nama tidak dikenal: tanpa baris, tanpa error",
				Input:       JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "ghost"},
				ExpectError: false,
				InvariantCheck: func(actual JNIRows, _ JNIProbe) bool {
					return len(actual.Rows) == 0
				},
				FailureHint: "Nama yang tidak ada di index harus memberi jawaban kosong.",
			},
			{
				Name:        "signature malformed (__X): def ada, ffi tidak ada",
				Input:       JNIProbe{Kt: jniKtE2E, Rs: "pub fn Java_com_example_Bridge_send__X() {}\n", Query: "send"},
				ExpectError: false,
				InvariantCheck: func(actual JNIRows, _ JNIProbe) bool {
					if len(actual.Rows) != 2 {
						return false
					}
					for _, r := range actual.Rows {
						if r.Kind == "ffi" {
							return false
						}
					}
					return true
				},
				FailureHint: "Bridge malformed harus fail-closed untuk matching (tanpa ffi), tetapi def Kotlin tetap ada.",
			},
			{
				Name:        "signature dangling _3: def ada, ffi tidak ada",
				Input:       JNIProbe{Kt: jniKtE2E, Rs: "pub fn Java_com_example_Bridge_send___3() {}\n", Query: "send"},
				ExpectError: false,
				InvariantCheck: func(actual JNIRows, _ JNIProbe) bool {
					for _, r := range actual.Rows {
						if r.Kind == "ffi" {
							return false
						}
					}
					return len(actual.Rows) == 2
				},
				FailureHint: "Array prefix menggantung bukan signature yang valid.",
			},
			{
				Name:        "tanpa package: base tanpa pkg tetap ketemu",
				Input:       JNIProbe{Kt: jniKtNoPkg, Rs: jniRsNoPkg, Query: "send"},
				ExpectError: false,
				InvariantCheck: func(actual JNIRows, _ JNIProbe) bool {
					return len(actual.Rows) == 2 &&
						actual.Rows[0].Path == "Bridge.kt" && actual.Rows[0].Kind == "def" &&
						actual.Rows[1].Path == "native.rs" && actual.Rows[1].Kind == "ffi"
				},
				FailureHint: "File tanpa package memakai base Java_<Class>_<method>.",
			},
			{
				Name:        "CRLF sama dengan LF",
				Input:       JNIProbe{Kt: strings.ReplaceAll(jniKtE2E, "\n", "\r\n"), Rs: strings.ReplaceAll(jniRsE2E, "\n", "\r\n"), Query: "send"},
				ExpectError: false,
				InvariantCheck: func(actual JNIRows, _ JNIProbe) bool {
					return len(actual.Rows) == 4 &&
						actual.Rows[0].Line == 4 && actual.Rows[1].Line == 5 &&
						actual.Rows[2].Line == 1 && actual.Rows[3].Line == 2
				},
				FailureHint: "Line ending CRLF tidak boleh menggeser nomor baris.",
			},
		},

		GeneratedSamples: []JNIProbe{
			{Kt: "package com.example\n\nclass Bridge {\n    external fun ping(): Unit\n}\n", Rs: "pub fn Java_com_example_Bridge_ping() {}\n", Query: "ping"},
			{Kt: "package com.example\n\nclass Bridge {\n    external fun send(a: Int, b: Long): Unit\n}\n", Rs: "pub fn Java_com_example_Bridge_send__IJ(a: i32, b: i64) {}\n", Query: "Java_com_example_Bridge_send__IJ"},
			{Kt: jniKtCoarse, Rs: jniRsCoarse, Query: "Java_com_example_Bridge_send__Ljava_lang_String_2"},
			{Kt: jniKtCoarse, Rs: jniRsCoarse, Query: "send"},
			{Kt: jniKtVararg, Rs: jniRsVararg, Query: "sum"},
			{Kt: jniKtVararg, Rs: jniRsVararg, Query: "Java_com_example_Bridge_sum___3I"},
			{Kt: jniKtCompanion, Rs: jniRsCompanion, Query: "create"},
			{Kt: jniKtNoPkg, Rs: jniRsNoPkg, Query: "Java_Bridge_send__I"},
			{Kt: "package com.example\n\nobject Holder {\n    external fun store(key: String, value: Long)\n}\n", Rs: "pub fn Java_com_example_Holder_store__Ljava_lang_String_2J(k: &str, v: i64) {}\n", Query: "store"},
			{Kt: "package com.example\n\nclass Outer {\n    class Inner {\n        external fun go(x: Int)\n    }\n}\n", Rs: "pub fn Java_com_example_Outer_00024Inner_go__I(x: i32) {}\n", Query: "go"},
			{Kt: "package com.example\n\nclass Bridge {\n    fun plain() {}\n}\n", Rs: "pub fn plain() {}\n", Query: "plain"},
			{Kt: "", Rs: "pub fn Java_com_example_Bridge_send__I(x: i32) {}\n", Query: "Java_com_example_Bridge_send__I"},
			{Kt: "package com.example\n\nclass Bridge {\n    external fun send(text: String)\n}\n", Rs: "pub fn Java_com_example_Bridge_send__I(x: i32) {}\n", Query: "Java_com_example_Bridge_send__I"},
			{Kt: "package com.example\n\nclass Bridge {\n    external fun send(a: Int, b: Int)\n}\n", Rs: "pub fn Java_com_example_Bridge_send__I(x: i32) {}\n", Query: "Java_com_example_Bridge_send__I"},
			{Kt: jniKtE2E, Rs: jniRsE2E, Query: "Bridge.send"},
		},

		RegressionSuite: []RegressionCase[JNIProbe, JNIRows]{
			{
				ID:    "REG-JNI-PROBE-01",
				Name:  "Probe E2E 2026-11-21: send dua overload, dua arah",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "send"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 4, Kind: "def"},
					{Path: "Bridge.kt", Line: 5, Kind: "def"},
					{Path: "native.rs", Line: 1, Kind: "ffi"},
					{Path: "native.rs", Line: 2, Kind: "ffi"},
				}},
			},
			{
				ID:    "REG-JNI-SHORT-01",
				Name:  "Short-name query tidak boleh menghilangkan decl (fail-open)",
				Input: JNIProbe{Kt: jniKtE2E, Rs: jniRsE2E, Query: "Java_com_example_Bridge_send"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 4, Kind: "ffi"},
					{Path: "Bridge.kt", Line: 5, Kind: "ffi"},
				}},
			},
			{
				ID:    "REG-JNI-WILDCARD-01",
				Name:  "Nullable wildcard tidak mem-veto token String",
				Input: JNIProbe{Kt: jniKtCompanion, Rs: jniRsCompanion, Query: "Java_com_example_Outer_00024Companion_create__Ljava_lang_String_2"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 5, Kind: "ffi"},
					{Path: "native.rs", Line: 1, Kind: "def"},
				}},
			},
			{
				ID:    "REG-JNI-MALFORMED-01",
				Name:  "Bridge malformed tidak menghasilkan ffi",
				Input: JNIProbe{Kt: jniKtE2E, Rs: "pub fn Java_com_example_Bridge_send__X() {}\n", Query: "send"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 4, Kind: "def"},
					{Path: "Bridge.kt", Line: 5, Kind: "def"},
				}},
			},
			{
				ID:    "REG-JNI-VARARG-01",
				Name:  "Harness-found 2026-11-21: vararg Int -> _3I, bukan I (parameter_modifiers adalah sibling, bukan isi parameter)",
				Input: JNIProbe{Kt: jniKtVararg, Rs: jniRsVararg, Query: "Java_com_example_Bridge_sum___3I"},
				Expected: JNIRows{Rows: []JNIRow{
					{Path: "Bridge.kt", Line: 4, Kind: "ffi"},
					{Path: "native.rs", Line: 1, Kind: "def"},
				}},
			},
		},
	}
}

func main() {
	spec := BuildSuiteSpec()
	report := RunAgentVerificationSuite(spec)
	exitCode := PrintVerificationReport(report)
	os.Exit(exitCode)
}
