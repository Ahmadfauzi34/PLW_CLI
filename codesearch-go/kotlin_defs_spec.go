//go:build verify

// ============================================================================
// Penempatan skill Go verification-harness di codesearch, untuk def Kotlin
// ============================================================================
// - harness.go di direktori ini adalah SALINAN VERBATIM dari skill
//   local-verification-harness-go (core TIDAK diubah; yang dicustom
//   hanya file spec ini, sesuai aturan skill).
// - File ini memakai build tag "verify": build/test/vet normal
//   (tanpa tag) mengabaikannya, sehingga tidak ada main() ganda dengan
//   main.go. Jalankan HANYA lewat ./verify-kotlin-defs.sh, yang memakai
//   `go run` dengan daftar file eksplisit (mode file mengabaikan tag)
//   dan TIDAK mencantumkan main.go.
// - Subjek: span definisi Kotlin (name -> startLine) lewat
//   astDefinitions — termasuk penyesuaian anchor anotasi dan fallback
//   regex (kotlin_fallback.go). Perilaku yang dikunci: anchor selalu
//   di mods/kw, bukan di anotasi; file yang gagal di-parse grammar
//   tetap menghasilkan def.
// - Oracle: pemindai baris independen (tanpa tree-sitter, tanpa regex
//   subjek) yang berjalan mundur dari keyword deklarasi ke anchor dan
//   meniru aturan doc-attachment. Ekuivalensi gerbang 3 hanya
//   membandingkan (name, startLine); endLine/sigEnd dikunci oleh unit
//   test (astchunk_test.go), bukan oleh harness ini.

package main

import (
	"os"
	"sort"
	"strings"
)

type KtDefSpan struct {
	Name      string `json:"name"`
	StartLine int    `json:"startLine"`
}

// STEP 1: Subjek — span def Kotlin lewat astDefinitions.
func KotlinDefSpansSubject(in string) ([]KtDefSpan, error) {
	defs, ok := astDefinitions("x.kt", in)
	out := []KtDefSpan{}
	if !ok {
		return out, nil
	}
	for _, d := range defs {
		if d.name == "" {
			continue
		}
		out = append(out, KtDefSpan{Name: d.name, StartLine: d.startLine})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartLine != out[j].StartLine {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// STEP 2: Reference oracle — pemindai baris independen.
//
// Untuk tiap baris kode (komentar/string diblank dengan state machine
// sendiri), cari keyword deklarasi pertama sebagai kata utuh; nama
// adalah identifier pertama sesudahnya (lewati generics, `class` /
// `interface` sesudah `fun`, dan receiver `Type.`); anchor adalah
// baris itu sendiri — karena anotasi di baris sebelumnya tidak pernah
// menggeser anchor, dan modifier/keyword selalu sebaris dengan nama
// pada sampel yang diuji. Lalu tiru attachDocComments: naik selama
// baris di atasnya adalah baris doc (`/**`, `/*`, `*`).
func KotlinDefSpansOracle(in string) ([]KtDefSpan, error) {
	norm := strings.ReplaceAll(strings.ReplaceAll(in, "\r\n", "\n"), "\r", "\n")
	rawLines := strings.Split(norm, "\n")
	codeLines := oracleCodeLines(rawLines)

	keywords := map[string]bool{
		"class": true, "interface": true, "object": true, "fun": true,
		"val": true, "var": true, "typealias": true, "enum": true,
	}
	modifiers := map[string]bool{
		"public": true, "internal": true, "private": true, "protected": true,
		"open": true, "abstract": true, "data": true, "sealed": true,
		"final": true, "expect": true, "actual": true, "inline": true,
		"value": true, "external": true, "override": true, "crossinline": true,
		"noinline": true, "tailrec": true, "suspend": true, "operator": true,
		"infix": true, "lateinit": true, "const": true, "companion": true,
		"annotation": true,
	}

	isIdentStart := func(r byte) bool {
		return r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
	}
	isIdent := func(r byte) bool {
		return isIdentStart(r) || (r >= '0' && r <= '9')
	}
	var out []KtDefSpan = []KtDefSpan{}
	for idx, line := range codeLines {
		ln := idx + 1
		// Cari keyword deklarasi pertama sebagai kata utuh; awalan
		// sebelum keyword hanya boleh berisi anotasi/modifier.
		fields := strings.Fields(line)
		kwAt := -1
		kw := ""
		for fi, f := range fields {
			word := f
			// Kupas `(` / `<` / `.` yang menempel pada token.
			for len(word) > 0 && !isIdent(word[len(word)-1]) {
				word = word[:len(word)-1]
			}
			if keywords[word] {
				// Validasi awalan: semua token sebelumnya harus
				// anotasi (@...) atau modifier.
				ok := true
				for _, pre := range fields[:fi] {
					p := pre
					for len(p) > 0 && !isIdent(p[len(p)-1]) && p[len(p)-1] != ')' {
						p = p[:len(p)-1]
					}
					if strings.HasPrefix(p, "@") || modifiers[p] {
						continue
					}
					ok = false
					break
				}
				if ok {
					kwAt = fi
					kw = word
				}
				break
			}
			// Token pertama yang bukan anotasi/modifier menghentikan
			// pencarian keyword pada baris ini.
			if !strings.HasPrefix(f, "@") && !modifiers[f] {
				break
			}
		}
		// Nama: identifier pertama sesudah keyword, diambil dari
		// field-field sesudahnya satu per satu (lewati generics,
		// `class`/`interface` sesudah `fun`, dan receiver `Type.`).
		// Hanya field pertama yang diperiksa, jadi titik di badan
		// `= bar.baz()` tidak pernah terbaca sebagai receiver.
		name := ""
		if kwAt >= 0 {
			fi := kwAt + 1
			// `fun interface` / `fun class`: lewati kata kedua.
			if kw == "fun" && fi < len(fields) && (fields[fi] == "class" || fields[fi] == "interface") {
				fi++
			}
			// Lewati generics `<...>` (tanpa nesting pada sampel).
			if fi < len(fields) && strings.HasPrefix(fields[fi], "<") {
				for fi < len(fields) && !strings.Contains(fields[fi], ">") {
					fi++
				}
				if fi < len(fields) {
					fi++
				}
			}
			if fi < len(fields) {
				tok := fields[fi]
				// Receiver `Type.` / `Type<T>.`: ambil segmen terakhir.
				if dot := strings.LastIndex(tok, "."); dot >= 0 {
					tok = tok[dot+1:]
				}
				if len(tok) > 0 && isIdentStart(tok[0]) {
					j := 0
					for j < len(tok) && isIdent(tok[j]) {
						j++
					}
					name = tok[:j]
				}
			}
		}
		if name == "" {
			continue
		}
		// val/var BUKAN def di model engine (K4: properti hidup di
		// --related, bukan di defs) — oracle tidak mengeluarkannya.
		if kw == "val" || kw == "var" {
			continue
		}
		// Doc-attachment: naik selama baris di atasnya doc.
		start := ln
		for start >= 2 {
			up := strings.TrimSpace(codeLines[start-2])
			if strings.HasPrefix(up, "/**") || strings.HasPrefix(up, "/*") || strings.HasPrefix(up, "*") {
				start--
			} else {
				break
			}
			if ln-start > 20 {
				break
			}
		}
		out = append(out, KtDefSpan{Name: name, StartLine: start})
	}
	// Dedup + urut deterministik.
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartLine != out[j].StartLine {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].Name < out[j].Name
	})
	dedup := out[:0]
	for _, s := range out {
		if len(dedup) > 0 && dedup[len(dedup)-1] == s {
			continue
		}
		dedup = append(dedup, s)
	}
	return dedup, nil
}

// oracleCodeLines blanks comments and strings per line with its own
// carried state (block comments, regular + raw strings), independent
// of stripKotlinCode.
func oracleCodeLines(raw []string) []string {
	out := make([]string, len(raw))
	inBlock := false
	inRaw := false
	inStr := false
	for i, ln := range raw {
		var b strings.Builder
		for j := 0; j < len(ln); j++ {
			c := ln[j]
			if inBlock {
				if c == '*' && j+1 < len(ln) && ln[j+1] == '/' {
					inBlock = false
					b.WriteString("  ")
					j++
				} else {
					b.WriteByte(' ')
				}
				continue
			}
			if inRaw {
				if c == '"' && j+2 < len(ln) && ln[j+1] == '"' && ln[j+2] == '"' {
					inRaw = false
					b.WriteString("   ")
					j += 2
				} else {
					b.WriteByte(' ')
				}
				continue
			}
			if inStr {
				if c == '\\' {
					b.WriteString("  ")
					j++
					continue
				}
				if c == '"' {
					inStr = false
					b.WriteByte(' ')
				} else {
					b.WriteByte(' ')
				}
				continue
			}
			if c == '/' && j+1 < len(ln) && ln[j+1] == '/' {
				break
			}
			if c == '/' && j+1 < len(ln) && ln[j+1] == '*' {
				inBlock = true
				b.WriteString("  ")
				j++
				continue
			}
			if c == '"' && j+2 < len(ln) && ln[j+1] == '"' && ln[j+2] == '"' {
				inRaw = true
				b.WriteString("   ")
				j += 2
				continue
			}
			if c == '"' {
				inStr = true
				b.WriteByte(' ')
				continue
			}
			b.WriteByte(c)
		}
		out[i] = b.String()
	}
	return out
}

// STEP 3: SuiteSpec 4 gerbang.
func BuildSuiteSpec() SuiteSpec[string, []KtDefSpan] {
	opts := DefaultSuiteOptions()

	return SuiteSpec[string, []KtDefSpan]{
		Name:            "KotlinDefSpans",
		Domain:          "low-level",
		Subject:         KotlinDefSpansSubject,
		ReferenceOracle: KotlinDefSpansOracle,
		Options:         &opts,

		PrimaryCases: []PrimaryCase[string, []KtDefSpan]{
			{
				Name:  "LoadingPill: KDoc + @Composable + internal fun anchor di baris fun",
				Input: "package io.jeph.pipkin.ui.components\n\n/** A pot and [text] in a pill. */\n@Composable\ninternal fun LoadingPill(text: String) {\n  Row {\n  }\n}\n",
				Expected: []KtDefSpan{
					{Name: "LoadingPill", StartLine: 5},
				},
				// Kasus pertama dalam proses: parse Kotlin pertama
				// membayar lazy-load grammar gotreesitter sekali
				// (~150ms terukur; parse hangat ~1ms). Anggaran ini
				// hanya untuk cold-start; semua kasus lain tetap
				// pada default 50ms dan membuktikan biaya hangat.
				MaxDurationMs: 500,
			},
			{
				Name:  "@Composable fun tanpa modifier anchor di baris fun",
				Input: "package p\n\n@Composable\nfun Plain() {\n}\n",
				Expected: []KtDefSpan{
					{Name: "Plain", StartLine: 4},
				},
			},
			{
				Name:  "internal fun tanpa anotasi tidak bergerak",
				Input: "package p\n\ninternal fun Foo(x: Int): String = x.toString()\n",
				Expected: []KtDefSpan{
					{Name: "Foo", StartLine: 3},
				},
			},
			{
				Name:  "Object + @JvmStatic external fun anchor di baris external",
				Input: "package p\n\ninternal object Bridge {\n  /** Version. */\n  @JvmStatic\n  external fun version(): String\n}\n",
				Expected: []KtDefSpan{
					{Name: "Bridge", StartLine: 3},
					{Name: "version", StartLine: 6},
				},
			},
			{
				Name:  "KDoc tanpa anotasi tetap menempel (attachDocComments dipertahankan)",
				Input: "package p\n\n/** Starts the host. */\nclass HostLauncher {\n}\n",
				Expected: []KtDefSpan{
					{Name: "HostLauncher", StartLine: 3},
				},
			},
		},

		PoisonInputs: []PoisonCase[string, []KtDefSpan]{
			{
				Name:        "String kosong mengembalikan slice kosong non-nil",
				Input:       "",
				ExpectError: false,
				InvariantCheck: func(actual []KtDefSpan, _ string) bool {
					return actual != nil && len(actual) == 0
				},
				FailureHint: "Input kosong harus []KtDefSpan{} (non-nil), bukan error.",
			},
			{
				Name:        "Hanya komentar mengembalikan slice kosong",
				Input:       "package p\n\n// fun Fake() {}\n/* class AlsoFake {} */\n",
				ExpectError: false,
				InvariantCheck: func(actual []KtDefSpan, _ string) bool {
					return len(actual) == 0
				},
				FailureHint: "Deklarasi di dalam komentar tidak boleh jadi def.",
			},
			{
				Name:        "Block comment tidak tertutup tidak boleh panic",
				Input:       "package p\n\n/* terbuka\nfun Fake() {}\n",
				ExpectError: false,
				InvariantCheck: func(actual []KtDefSpan, _ string) bool {
					return len(actual) == 0
				},
				FailureHint: "Komentar tak tertutup harus fail-open (nol def), bukan panic.",
			},
			{
				Name:        "String tidak tertutup tidak boleh panic",
				Input:       "package p\n\nval s = \"abc\nfun Foo() {}\n",
				ExpectError: false,
				InvariantCheck: func(actual []KtDefSpan, _ string) bool {
					return len(actual) >= 1
				},
				FailureHint: "String tak tertutup harus fail-open, bukan panic.",
			},
			{
				Name:        "Deklarasi di dalam string tidak boleh jadi def",
				Input:       "package p\n\nval s = \"fun Fake() {}\"\nfun Real() {}\n",
				ExpectError: false,
				InvariantCheck: func(actual []KtDefSpan, _ string) bool {
					// NOTE: top-level val bukan def (K4) — hanya Real.
					return len(actual) == 1 && actual[0].Name == "Real"
				},
				FailureHint: "Hanya Real yang boleh jadi def; Fake di dalam string tidak.",
			},
			{
				Name:        "CRLF sama dengan LF",
				Input:       "package p\r\n\r\ninternal fun Foo() {}\r\n",
				ExpectError: false,
				InvariantCheck: func(actual []KtDefSpan, _ string) bool {
					return len(actual) == 1 && actual[0].Name == "Foo" && actual[0].StartLine == 3
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
		},

		RegressionSuite: []RegressionCase[string, []KtDefSpan]{
			{
				ID:    "REG-KT-ANCHOR-01",
				Name:  "Matriks probe 2026-11-20: anotasi saja dan modifier saja tidak bergerak",
				Input: "package p\n\n@Composable\nfun A() {}\n\ninternal fun B() {}\n\n@Deprecated(\"x\")\nfun C() {}\n",
				Expected: []KtDefSpan{
					{Name: "A", StartLine: 4},
					{Name: "B", StartLine: 6},
					{Name: "C", StartLine: 9},
				},
			},
			{
				ID:    "REG-KT-FALLBACK-02",
				Name:  "Dua overload nama sama di baris berbeda tetap dua def (merge tidak menelan)",
				Input: "package p\n\n@Composable\ninternal fun foo() {}\n\n@Composable\ninternal fun foo(x: Int) {}\n",
				Expected: []KtDefSpan{
					{Name: "foo", StartLine: 4},
					{Name: "foo", StartLine: 7},
				},
			},
			{
				ID:    "REG-KT-HOSTLAUNCHER-03",
				Name:  "Fixture TestASTChunkKotlin lama: def tanpa anotasi tidak bergerak",
				Input: "package io.jeph.pipkin.host\n\n/** Starts the host. */\nclass HostLauncher(private val client: RpcClient) {\n    fun start(): Boolean {\n        val ok = client.connect()\n        return ok\n    }\n}\n\nobject HostRegistry {\n    val count = 0\n}\n\ntypealias Handler = (String) -> Unit\n\nfun topLevel(x: Int): String = x.toString()\n",
				Expected: []KtDefSpan{
					{Name: "HostLauncher", StartLine: 3},
					{Name: "start", StartLine: 5},
					{Name: "HostRegistry", StartLine: 11},
					{Name: "Handler", StartLine: 15},
					{Name: "topLevel", StartLine: 17},
				},
			},
			{
				ID:    "REG-KT-OOPBOTBRIDGE-04",
				Name:  "Member OopbotBridge: external fun di dalam object",
				Input: "package p\n\ninternal object OopbotBridge {\n  val isAvailable: Boolean = true\n\n  /** Version. */\n  @JvmStatic\n  external fun version(): String\n\n  /** Catalog. */\n  @JvmStatic\n  external fun referenceCatalog(): String\n}\n",
				Expected: []KtDefSpan{
					{Name: "OopbotBridge", StartLine: 3},
					{Name: "version", StartLine: 8},
					{Name: "referenceCatalog", StartLine: 12},
				},
			},
			{
				ID:       "AUTO-REG-PRIMARY-TARGET-1",
				Name:     "LoadingPill: KDoc + @Composable + internal fun anchor di baris fun",
				Input:    "package io.jeph.pipkin.ui.components\n\n/** A pot and [text] in a pill. */\n@Composable\ninternal fun LoadingPill(text: String) {\n  Row {\n  }\n}\n",
				Expected: []KtDefSpan{{Name: "LoadingPill", StartLine: 5}},
			},
			{
				ID:       "AUTO-REG-DIFFERENTIAL-EQUIVALENCE-2",
				Name:     "Top-level val bukan def: nil vs empty slice harus [] non-nil (jebakan nil-vs-empty)",
				Input:    "package p\n\ninternal val count = 0\n",
				Expected: []KtDefSpan{},
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
