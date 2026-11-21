package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// Batch 2-6 (2026-10-26): runTimed is the shared time budget for chunking,
// callers, and related scans. A slow function must be abandoned on time
// (false) while a fast one completes (true) — without a real parser, so
// the test is deterministic and fast.
func TestRunTimed(t *testing.T) {
	done := false
	if !runTimed(5*time.Second, func() { done = true }) || !done {
		t.Error("runTimed fast function: want true + done")
	}
	start := time.Now()
	if runTimed(50*time.Millisecond, func() { time.Sleep(2 * time.Second) }) {
		t.Error("runTimed slow function: want false")
	}
	if el := time.Since(start); el > 1500*time.Millisecond {
		t.Errorf("runTimed slow function took %v, want ~50ms (leaked goroutine must not block)", el)
	}
}

// coveredLines returns the set of 1-based lines covered by chunks.
func coveredLines(chunks []chunk) map[int]bool {
	m := map[int]bool{}
	for _, c := range chunks {
		for l := c.startLine; l < c.startLine+len(c.lines); l++ {
			m[l] = true
		}
	}
	return m
}

func assertFullCoverage(t *testing.T, chunks []chunk, nlines int) {
	t.Helper()
	cov := coveredLines(chunks)
	for l := 1; l <= nlines; l++ {
		if !cov[l] {
			t.Errorf("line %d not covered by any chunk", l)
		}
	}
}

func chunkStarts(chunks []chunk) []int {
	var out []int
	for _, c := range chunks {
		out = append(out, c.startLine)
	}
	return out
}

func TestASTChunkGoDefs(t *testing.T) {
	text := "package sample\n\n// Add adds two ints.\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\ntype Counter struct{ n int }\n\nfunc (c *Counter) Inc() { c.n++ }\n"
	chunks, astOK := chunkFile("x.go", text)
	if !astOK {
		t.Error("expected AST path for valid Go")
	}
	// lines: 1 pkg, 2 blank, 3 comment, 4-6 Add, 7 blank, 8 type, 9 blank,
	// 10 Inc, 11 trailing blank. K12: the doc comment (3) attaches to
	// Add (now 3-6) and the type declaration (8) is its own chunk.
	// K13: Add also yields a signature chunk (3-4) sharing start 3.
	wantStarts := []int{1, 3, 3, 7, 8, 9, 10, 11}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	// The Add definition yields a signature chunk (doc + header) and a
	// body chunk (doc + full function).
	var sawSig, sawBody bool
	for _, c := range chunks {
		if c.startLine == 3 && c.kind == kindSig {
			sawSig = len(c.lines) == 2 && strings.Contains(c.lines[0], "Add adds") &&
				strings.Contains(c.lines[1], "func Add")
		}
		if c.startLine == 3 && c.kind == kindBody {
			sawBody = len(c.lines) == 4 && strings.Contains(c.lines[0], "Add adds") &&
				strings.Contains(c.lines[1], "func Add")
		}
	}
	if !sawSig || !sawBody {
		t.Errorf("Add sig/body chunks missing")
	}
	assertFullCoverage(t, chunks, 11)
}

func TestASTChunkPythonNested(t *testing.T) {
	text := "\"\"\"Doc.\"\"\"\n\ndef add(a, b):\n    return a + b\n\nclass Counter:\n    def inc(self):\n        pass\n"
	chunks, astOK := chunkFile("x.py", text)
	if !astOK {
		t.Error("expected AST path for valid Python")
	}
	// defs: add(3-4), Counter(6-8), inc(7-8); gaps: 1-2, 5-5, 9-9.
	// K13: add and inc also yield signature chunks (3-3, 7-7).
	wantStarts := []int{1, 3, 3, 5, 6, 7, 7, 9}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 9)
}

func TestASTChunkLongDefSplit(t *testing.T) {
	var b strings.Builder
	b.WriteString("package p\n\nfunc big() {\n") // lines 1-3
	for i := 0; i < 196; i++ {
		b.WriteString("\tx := 1\n") // lines 4-199
	}
	b.WriteString("}\n") // line 200 (+ trailing blank 201)
	chunks, astOK := chunkFile("big.go", b.String())
	if !astOK {
		t.Error("expected AST path for valid Go (long-def split)")
	}
	for _, c := range chunks {
		if len(c.lines) > maxDefLines {
			t.Errorf("chunk at line %d has %d lines, max %d",
				c.startLine, len(c.lines), maxDefLines)
		}
	}
	// The long def must be split into overlapping windows covering 3-200.
	cov := coveredLines(chunks)
	for l := 3; l <= 200; l++ {
		if !cov[l] {
			t.Errorf("def line %d not covered", l)
		}
	}
	// Overlap: consecutive def chunks must share lines.
	var defChunks []chunk
	for _, c := range chunks {
		if c.startLine >= 3 && c.startLine <= 200 {
			defChunks = append(defChunks, c)
		}
	}
	for i := 1; i < len(defChunks); i++ {
		prevEnd := defChunks[i-1].startLine + len(defChunks[i-1].lines) - 1
		if defChunks[i].startLine > prevEnd {
			t.Errorf("gap between def chunks at %d and %d",
				defChunks[i-1].startLine, defChunks[i].startLine)
		}
	}
	assertFullCoverage(t, chunks, 201)
}

func TestASTChunkFallback(t *testing.T) {
	text := "package sample\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	// Unknown extension -> identical to fixed windows.
	if got, ok := chunkFile("x.txt", text); !reflect.DeepEqual(got, chunkLines(text)) || ok {
		t.Errorf("txt fallback differs from chunkLines (ok=%v)", ok)
	}
	// Unparseable Go -> no defs found -> fixed windows.
	garbage := "{{{not go code[[[\n@@@\n"
	if got, ok := chunkFile("x.go", garbage); !reflect.DeepEqual(got, chunkLines(garbage)) || ok {
		t.Errorf("garbage-go fallback differs from chunkLines (ok=%v)", ok)
	}
	// Oversized file -> fixed windows (no AST parse attempted).
	big := strings.Repeat("x := 1\n", maxASTBytes)
	if got, ok := chunkFile("x.go", big); !reflect.DeepEqual(got, chunkLines(big)) || ok {
		t.Errorf("oversized fallback differs from chunkLines (ok=%v)", ok)
	}
}

func TestASTChunkRustFn(t *testing.T) {
	text := "/// Adds.\npub fn add(a: i32, b: i32) -> i32 {\n    a + b\n}\n\nfn main() {}\n"
	chunks, astOK := chunkFile("x.rs", text)
	if !astOK {
		t.Error("expected AST path for valid Rust")
	}
	// defs: add(1-4, doc comment attached), main(6-6); gaps: 5-5, 7-7(trailing).
	// K13: add also yields a signature chunk (1-2).
	wantStarts := []int{1, 1, 5, 6, 7}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 7)
}

func TestASTChunkJS(t *testing.T) {
	text := "// helper\nfunction add(a, b) {\n  return a + b;\n}\n\nclass Counter {\n  inc() {\n    this.n++;\n  }\n}\n\nconst double = (x) => {\n  return x * 2;\n};\n"
	chunks, astOK := chunkFile("x.js", text)
	if !astOK {
		t.Error("expected AST path for valid JS")
	}
	// defs: add(2-4), Counter(6-10), inc(7-9), double-arrow(12-14);
	// gaps: 1-1, 5-5, 11-11, 15-15(trailing). The plain // line above
	// add is NOT a JSDoc block, so it stays a gap chunk.
	// K13: add, inc, double also yield signature chunks (2-2, 7-7, 12-12).
	wantStarts := []int{1, 2, 2, 5, 6, 7, 7, 11, 12, 12, 15}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 15)
}

func TestASTChunkTS(t *testing.T) {
	text := "interface Shape {\n  area(): number;\n}\n\ntype ID = string;\n\nenum Color {\n  Red,\n  Green,\n}\n\nfunction greet(name: string): string {\n  return \"hi \" + name;\n}\n"
	chunks, astOK := chunkFile("x.ts", text)
	if !astOK {
		t.Error("expected AST path for valid TS")
	}
	// defs: Shape(1-3), area(2-2, interface member), ID(5-5),
	// Color(7-10), greet(12-14); gaps: 4-4, 6-6, 11-11, 15-15(trailing).
	// K13: greet also yields a signature chunk (12-12). The member
	// chunk overlaps its interface chunk; both are kept (the member
	// is the specific hit, the interface the container) — the same
	// overlap as class methods in TestASTChunkJS.
	wantStarts := []int{1, 2, 4, 5, 6, 7, 11, 12, 12, 15}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 15)
}

func TestASTChunkMjs(t *testing.T) {
	text := "export function foo() {\n  return 1;\n}\n\nexport const bar = () => {\n  return 2;\n};\n"
	chunks, astOK := chunkFile("x.mjs", text)
	if !astOK {
		t.Error("expected AST path for valid .mjs")
	}
	// defs: foo(1-3), bar-arrow(5-7); gaps: 4-4, 8-8(trailing).
	// K13: foo and bar also yield signature chunks (1-1, 5-5).
	wantStarts := []int{1, 1, 4, 5, 5, 8}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 8)
}

func TestASTChunkTSX(t *testing.T) {
	text := "export function App() {\n  return <div>hi</div>;\n}\n"
	chunks, astOK := chunkFile("x.tsx", text)
	if !astOK {
		t.Error("expected AST path for valid TSX")
	}
	// defs: App(1-3); gap: 4-4(trailing).
	// K13: App also yields a signature chunk (1-1).
	wantStarts := []int{1, 1, 4}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 4)
}

func TestASTChunkKotlin(t *testing.T) {
	// K1-K4 (2026-11-12): Kotlin chunking. Lines: 1 package, 2 blank,
	// 3 KDoc, 4-9 class HostLauncher, 5-8 fun start, 10 blank,
	// 11-13 object HostRegistry, 14 blank, 15 typealias Handler,
	// 16 blank, 17 fun topLevel (single-expression).
	text := "package io.jeph.pipkin.host\n\n/** Starts the host. */\n" +
		"class HostLauncher(private val client: RpcClient) {\n" +
		"    fun start(): Boolean {\n" +
		"        val ok = client.connect()\n" +
		"        return ok\n" +
		"    }\n" +
		"}\n\nobject HostRegistry {\n    val count = 0\n}\n\n" +
		"typealias Handler = (String) -> Unit\n\n" +
		"fun topLevel(x: Int): String = x.toString()\n"
	chunks, astOK := chunkFile("x.kt", text)
	if !astOK {
		t.Error("expected AST path for valid Kotlin")
	}
	// defs: HostLauncher(3-9, KDoc attached), start sig(5-5) + body(5-8),
	// HostRegistry(11-13), Handler(15), topLevel(17); gaps: 1-2, 10, 14,
	// 16, 18 (trailing).
	wantStarts := []int{1, 3, 5, 5, 10, 11, 14, 15, 16, 17, 18}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	// The class body chunk carries its KDoc; start has a sig chunk.
	var sawKDoc, sawSig bool
	names := map[string]bool{}
	for _, c := range chunks {
		if c.name != "" {
			names[c.name] = true
		}
		if c.startLine == 3 && c.kind == kindBody {
			sawKDoc = len(c.lines) == 7 && strings.Contains(c.lines[0], "Starts the host")
		}
		if c.startLine == 5 && c.kind == kindSig {
			sawSig = len(c.lines) == 1 && strings.Contains(c.lines[0], "fun start")
		}
	}
	if !sawKDoc || !sawSig {
		t.Errorf("Kotlin KDoc/sig chunks missing (kdoc=%v sig=%v)", sawKDoc, sawSig)
	}
	// Local val "ok" and member property "count" are NOT definitions
	// (properties live in --related, not in chunk defs).
	for _, want := range []string{"HostLauncher", "start", "HostRegistry", "Handler", "topLevel"} {
		if !names[want] {
			t.Errorf("def names = %v, want %s", names, want)
		}
	}
	for _, no := range []string{"ok", "count"} {
		if names[no] {
			t.Errorf("def names = %v, must not contain %s", names, no)
		}
	}
	assertFullCoverage(t, chunks, 18)
}

func TestASTChunkRustTypes(t *testing.T) {
	text := "struct Point { x: i32, y: i32 }\n\nenum Shape { Circle, Square }\n\ntrait Drawable {\n    fn draw(&self);\n}\n\ntype Alias = Point;\n\nimpl Point {\n    fn new() -> Self { Point { x: 0, y: 0 } }\n}\n"
	chunks, astOK := chunkFile("x.rs", text)
	if !astOK {
		t.Error("expected AST path for valid Rust types")
	}
	// defs: Point(1), Shape(3), Drawable(5-7), draw(6), Alias(9),
	// impl(11-13), new(12); gaps: 2, 4, 8, 10, 14(trailing).
	wantStarts := []int{1, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12, 14}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 14)
}

func TestASTChunkGoTypeBlock(t *testing.T) {
	text := "package p\n\ntype (\n\tA struct{ x int }\n\tB int\n)\n\nfunc f() {}\n"
	chunks, astOK := chunkFile("x.go", text)
	if !astOK {
		t.Error("expected AST path for valid Go type block")
	}
	// defs: A(4), B(5), f(8); gaps: 1-3, 6-7, 9(trailing).
	wantStarts := []int{1, 4, 5, 6, 8, 9}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 9)
}

func TestASTChunkDocBlock(t *testing.T) {
	text := "/**\n * Adds two numbers.\n */\nfunction add(a, b) {\n  return a + b;\n}\n"
	chunks, astOK := chunkFile("x.js", text)
	if !astOK {
		t.Error("expected AST path for valid JS with block doc")
	}
	// def: add(1-6, block doc attached); gap: 7(trailing).
	// K13: add also yields a signature chunk (1-4).
	wantStarts := []int{1, 1, 7}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 7)
}

func TestASTChunkDocCap(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 25; i++ {
		b.WriteString("// note\n")
	}
	b.WriteString("func f() {\n}\n")
	chunks, astOK := chunkFile("x.go", b.String())
	if !astOK {
		t.Error("expected AST path")
	}
	// def f(26-27) attaches at most maxDocLines (20) comment lines:
	// 6-27; gaps: 1-5, 28(trailing).
	// K13: f also yields a signature chunk (6-26).
	wantStarts := []int{1, 6, 6, 28}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	assertFullCoverage(t, chunks, 28)
}

func TestASTChunkGeneratedSkip(t *testing.T) {
	// .pb.go suffix -> fixed windows, not AST.
	pb := "package p\n\nfunc F() {}\n"
	if got, ok := chunkFile("x.pb.go", pb); !reflect.DeepEqual(got, chunkLines(pb)) || ok {
		t.Errorf("pb.go should fall back (ok=%v)", ok)
	}
	// Generated header marker -> fixed windows.
	gen := "// Code generated by tool. DO NOT EDIT.\npackage p\n\nfunc F() {}\n"
	if got, ok := chunkFile("x.go", gen); !reflect.DeepEqual(got, chunkLines(gen)) || ok {
		t.Errorf("generated header should fall back (ok=%v)", ok)
	}
	// "generated" in basename -> fixed windows.
	if got, ok := chunkFile("x_generated.go", pb); !reflect.DeepEqual(got, chunkLines(pb)) || ok {
		t.Errorf("generated basename should fall back (ok=%v)", ok)
	}
	// Minified: one line over the limit -> fixed windows.
	min := "function f() { return " + strings.Repeat("1+", 9000) + "1; }\n"
	if got, ok := chunkFile("x.js", min); !reflect.DeepEqual(got, chunkLines(min)) || ok {
		t.Errorf("minified line should fall back (ok=%v)", ok)
	}
}

func TestASTChunkSigPython(t *testing.T) {
	text := "def add(a, b):\n    return a + b\n"
	chunks, astOK := chunkFile("x.py", text)
	if !astOK {
		t.Error("expected AST path for valid Python")
	}
	// def add(1-2); body block starts line 2, so sigEnd = 1:
	// sig(1-1) + body(1-2); gap 3(trailing).
	wantStarts := []int{1, 1, 3}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	if chunks[0].kind != kindSig || chunks[1].kind != kindBody {
		t.Errorf("kinds = %v, %v, want sig then body", chunks[0].kind, chunks[1].kind)
	}
	assertFullCoverage(t, chunks, 3)
}

func TestASTChunkSigOneLiner(t *testing.T) {
	text := "func f() {}\n"
	chunks, astOK := chunkFile("x.go", text)
	if !astOK {
		t.Error("expected AST path")
	}
	// One-line def: signature == body, so no separate sig chunk.
	wantStarts := []int{1, 2}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	if chunks[0].kind != kindBody {
		t.Errorf("kind = %v, want body", chunks[0].kind)
	}
}

func TestASTChunkSigRustMultiline(t *testing.T) {
	text := "pub fn add(\n    a: i32,\n    b: i32,\n) -> i32 {\n    a + b\n}\n"
	chunks, astOK := chunkFile("x.rs", text)
	if !astOK {
		t.Error("expected AST path for valid Rust")
	}
	// Body block starts line 4 (the `) -> i32 {` line): sig(1-4) +
	// body(1-6); gap 7(trailing).
	wantStarts := []int{1, 1, 7}
	if got := chunkStarts(chunks); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("chunk starts = %v, want %v", got, wantStarts)
	}
	if chunks[0].kind != kindSig || len(chunks[0].lines) != 4 {
		t.Errorf("sig chunk wrong: kind=%v lines=%v", chunks[0].kind, chunks[0].lines)
	}
	assertFullCoverage(t, chunks, 7)
}

func TestASTChunkNames(t *testing.T) {
	text := "package p\n\n// Add adds.\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	chunks, astOK := chunkFile("x.go", text)
	if !astOK {
		t.Error("expected AST path")
	}
	// chunks: gap(1-2), sig(3-4), body(3-6), gap(7).
	if len(chunks) != 4 {
		t.Fatalf("chunks = %d, want 4", len(chunks))
	}
	if chunks[0].name != "" || chunks[3].name != "" {
		t.Errorf("gap names = %q, %q, want empty", chunks[0].name, chunks[3].name)
	}
	if chunks[1].name != "Add" || chunks[1].kind != kindSig {
		t.Errorf("sig chunk = %+v, want name Add kind sig", chunks[1])
	}
	if chunks[2].name != "Add" || chunks[2].kind != kindBody {
		t.Errorf("body chunk = %+v, want name Add kind body", chunks[2])
	}
}

func TestIsGeneratedOrMinifiedAssetsAndHeaders(t *testing.T) {
	yesCases := []struct {
		name string
		path string
		text string
	}{
		{"d.ts", "src/types.d.ts", "export declare function foo(): void;\n"},
		{"d.mts", "src/types.d.mts", "export declare function foo(): void;\n"},
		{"d.cts", "src/types.d.cts", "export declare function foo(): void;\n"},
		{"public assets root", "public/assets/app.js", "export function run() {}\n"},
		{"public assets nested", "./sub/public/assets/harness.mjs", "export function run() {}\n"},
		{"ts-self-types", "math compesor/burn_research.js", "/* @ts-self-types=\"./burn_research.d.ts\" */\nexport function foo() {}\n"},
		{"wbg ptr", "math compesor/node.mjs", "class X {\n  get __wbg_ptr() { return this.__wbg_ptr; }\n}\n"},
		{"wbindgen symbol", "pkg/glue.js", "function passStringToWasm0() { return __wbindgen_malloc(1); }\n"},
		{"wasm wbg call", "pkg/glue.js", "export function f() { return wasm.__wbg_foo(); }\n"},
		{"Generated by header", "proto/out.js", "// Generated by protoc-gen-es\nexport function f() {}\n"},
		{"auto-generated header", "pkg/meta.ts", "// auto-generated file\nexport const X = 1;\n"},
		{"Auto-generated header", "pkg/meta2.ts", "// Auto-generated file\nexport const X = 1;\n"},
	}
	for _, c := range yesCases {
		if !isGeneratedOrMinified(c.path, c.text, maxLineLenOf(c.text)) {
			t.Errorf("%s (%q): expected isGeneratedOrMinified=true", c.name, c.path)
		}
	}

	noCases := []struct {
		name string
		path string
		text string
	}{
		{"lowercase generated by prose", "src/litert.txt", "Model was generated by conversion script.\n"},
		{"plain wasm-bindgen word in JSDoc", "src/bridge.js", "// Talks to wasm-bindgen export helper\nexport function bridgeCall() {}\n"},
		{"similar dir name", "src/public_assets_helper.js", "export function ok() { return 1; }\n"},
	}
	for _, c := range noCases {
		if isGeneratedOrMinified(c.path, c.text, maxLineLenOf(c.text)) {
			t.Errorf("%s (%q): expected isGeneratedOrMinified=false", c.name, c.path)
		}
	}
}

// TestASTChunkKotlinAnnotations (PR #26 follow-up, 2026-11-20): a
// declaration's span anchors on its modifiers/keyword, never on a
// leading @annotation — and a file the Kotlin grammar cannot parse
// (@Composable internal fun) still yields def structure via the
// regex fallback instead of degrading to fixed windows.
func TestASTChunkKotlinAnnotations(t *testing.T) {
	// Lines: 1 package, 2 blank, 3 KDoc, 4 @Composable,
	// 5 internal fun LoadingPill, 6-7 Row{...}, 8 }, 9 blank,
	// 10 @Composable, 11 fun PlainComposable, 12 }, 13 blank,
	// 14 internal object OopbotBridge, 15 KDoc, 16 @JvmStatic,
	// 17 external fun version, 18 blank, 19 @JvmStatic,
	// 20 external fun engineNew, 21 }.
	text := "package io.jeph.pipkin.ui.components\n\n" +
		"/** A pot and [text] in a pill. */\n" +
		"@Composable\n" +
		"internal fun LoadingPill(text: String) {\n" +
		"  Row {\n" +
		"  }\n" +
		"}\n\n" +
		"@Composable\n" +
		"fun PlainComposable() {\n" +
		"}\n\n" +
		"internal object OopbotBridge {\n" +
		"  /** Version. */\n" +
		"  @JvmStatic\n" +
		"  external fun version(): String\n\n" +
		"  @JvmStatic\n" +
		"  external fun engineNew(apiKey: String): Long\n" +
		"}\n"
	chunks, astOK := chunkFile("x.kt", text)
	if !astOK {
		t.Fatal("expected AST path for annotated Kotlin (regex fallback must keep defs)")
	}
	want := map[string]int{
		"LoadingPill":     5,
		"PlainComposable": 11,
		"OopbotBridge":    14,
		"version":         17,
		"engineNew":       20,
	}
	got := map[string]int{}
	for _, c := range chunks {
		if c.name == "" {
			continue
		}
		if _, seen := got[c.name]; !seen {
			got[c.name] = c.startLine
		}
	}
	for name, line := range want {
		if got[name] != line {
			t.Errorf("def %s starts at line %d, want %d (got all: %v)", name, got[name], line, got)
		}
	}
	// LoadingPill's signature chunk is exactly its fun line: the
	// KDoc and the annotation are NOT the def start.
	var sawSig bool
	for _, c := range chunks {
		if c.name == "LoadingPill" && c.kind == kindSig {
			sawSig = len(c.lines) == 1 && strings.Contains(c.lines[0], "internal fun LoadingPill")
		}
	}
	if !sawSig {
		t.Errorf("LoadingPill sig chunk missing or not anchored on its fun line")
	}
	assertFullCoverage(t, chunks, 21)
}

// TestKotlinRegexFallbackDefs exercises the regex recovery path
// directly: broken-only files, mixed files (one parsable def + one
// grammar-broken def), and declarations that must NOT match (inside
// strings and doc comments).
func TestKotlinRegexFallbackDefs(t *testing.T) {
	lineOf := func(src []byte) func(uint32) int {
		li := newLineIndex(src)
		return li.lineOf
	}

	// Broken-only: the single def is annotation+modifier.
	broken := "package p\n\n/** Doc. */\n@Composable\ninternal fun Foo(x: Int): String = x.toString()\n"
	defs := kotlinRegexDefinitions(broken, lineOf([]byte(broken)))
	if len(defs) != 1 || defs[0].name != "Foo" || defs[0].startLine != 5 {
		t.Errorf("broken-only fallback defs = %+v, want [{Foo 5}]", defs)
	}
	if defs[0].sigEnd != 5 {
		t.Errorf("broken-only sigEnd = %d, want 5 (the `=` line)", defs[0].sigEnd)
	}

	// Multi-line header + multi-line single-expression body.
	multi := "package p\n\n@Composable\ninternal fun Bar(\n  a: Int,\n  b: String,\n): String = baz(\n  a,\n  b,\n)\n"
	defs = kotlinRegexDefinitions(multi, lineOf([]byte(multi)))
	if len(defs) != 1 || defs[0].name != "Bar" || defs[0].startLine != 4 {
		t.Errorf("multi-line fallback defs = %+v, want [{Bar 4}]", defs)
	}
	if len(defs) == 1 && (defs[0].endLine != 10 || defs[0].sigEnd != 7) {
		t.Errorf("multi-line end/sig = %d/%d, want 10/7", defs[0].endLine, defs[0].sigEnd)
	}

	// Declarations inside strings and doc comments must not match.
	hidden := "package p\n\n/** fun FakeDoc() {} */\nval s = \"fun FakeStr() {}\"\nfun Real() {}\n"
	defs = kotlinRegexDefinitions(hidden, lineOf([]byte(hidden)))
	names := map[string]bool{}
	for _, d := range defs {
		names[d.name] = true
	}
	if names["FakeDoc"] || names["FakeStr"] {
		t.Errorf("regex matched declarations inside comments/strings: %+v", defs)
	}
	// NOTE: top-level val is NOT a def in the engine model (K4) —
	// only Real is expected here; `s` is just the string carrier.
	if !names["Real"] || len(defs) != 1 {
		t.Errorf("regex defs = %+v, want only [Real]", defs)
	}

	// One level of nested parens in the annotation args (P1-C, go14):
	// the annotation shares the line with the modifiers.
	nested := "package p\n\n@Suppress(names = arrayOf(\"UNUSED\")) internal fun Guarded() {}\nfun ok() {}\n"
	defs = kotlinRegexDefinitions(nested, lineOf([]byte(nested)))
	names = map[string]bool{}
	for _, d := range defs {
		names[d.name] = true
	}
	if !names["Guarded"] || !names["ok"] || len(defs) != 2 {
		t.Errorf("nested-paren annotation defs = %+v, want [Guarded ok]", defs)
	}

	// Merge: AST defs are never dropped, regex defs fill the gap.
	ast := []definition{{startLine: 3, endLine: 3, name: "Ok", sigEnd: 0}}
	regex := []definition{
		{startLine: 3, endLine: 3, name: "Ok", sigEnd: 0},
		{startLine: 6, endLine: 9, name: "Broken", sigEnd: 6},
	}
	merged := mergeKotlinRegexDefs(ast, regex)
	if len(merged) != 2 {
		t.Errorf("merge = %+v, want 2 defs (Ok + Broken, no duplicate Ok)", merged)
	}
}

// TestKotlinASTAndFallbackDefsSilentGap is the P1-C end-to-end case:
// a file that parses cleanly (HasError false) and yields other defs,
// but silently misses zero-arg annotation+visibility functions —
// `@Composable internal fun Target() {}` degrades to a trailing-lambda
// call, a valid call expression. The always-on regex merge must
// recover Target and PreviewCard without duplicating peer, and a
// healthy file must gain no phantom def from a same-named call.
func TestKotlinASTAndFallbackDefsSilentGap(t *testing.T) {
	text := "package demo\n\n" +
		"import androidx.compose.runtime.Composable\n" +
		"import androidx.compose.ui.tooling.preview.Preview\n\n" +
		"@Composable\n" +
		"internal fun Target() {}\n\n" +
		"@Preview\n" +
		"private fun PreviewCard() {}\n\n" +
		"fun peer() {}\n"
	chunks, astOK := chunkFile("x.kt", text)
	if !astOK {
		t.Fatal("expected AST path for the silent-gap file")
	}
	got := map[string]int{}
	for _, c := range chunks {
		if c.name == "" {
			continue
		}
		if _, seen := got[c.name]; !seen {
			got[c.name] = c.startLine
		}
	}
	for name, line := range map[string]int{"Target": 7, "PreviewCard": 10, "peer": 12} {
		if got[name] != line {
			t.Errorf("def %s starts at line %d, want %d (got all: %v)", name, got[name], line, got)
		}
	}

	// Healthy file: a call to Target() inside peer's body must not
	// become a def; the only defs are peer and the Target declaration.
	healthy := "package demo\n\nfun peer() {\n    Target()\n}\n\nfun Target() {}\n"
	chunks, astOK = chunkFile("x.kt", healthy)
	if !astOK {
		t.Fatal("expected AST path for the healthy file")
	}
	got = map[string]int{}
	for _, c := range chunks {
		if c.name == "" {
			continue
		}
		if _, seen := got[c.name]; !seen {
			got[c.name] = c.startLine
		}
	}
	if len(got) != 2 || got["peer"] != 3 || got["Target"] != 7 {
		t.Errorf("healthy file defs = %v, want {peer:3 Target:7}", got)
	}
}

func TestGo16DefsGoAliasInterfaceTSAbstractRustMacro(t *testing.T) {
	// Go type_alias and interface method_elem
	goSrc := "package p\n\ntype UserID = string\n\ntype Runner interface {\n\tDoWork(x int) error\n\tStop()\n}\n"
	chunks, ok := chunkFile("runner.go", goSrc)
	if !ok {
		t.Fatal("expected AST chunking for runner.go")
	}
	gotGo := map[string]int{}
	for _, c := range chunks {
		if c.name != "" {
			gotGo[c.name] = c.startLine
		}
	}
	for name, line := range map[string]int{"UserID": 3, "Runner": 5, "DoWork": 6, "Stop": 7} {
		if gotGo[name] != line {
			t.Errorf("Go def %s line = %d, want %d (all=%v)", name, gotGo[name], line, gotGo)
		}
	}

	// TypeScript abstract class and abstract method signature
	tsSrc := "export abstract class BaseWorker {\n  abstract runTask(id: number): Promise<void>;\n  stop(): void {}\n}\n"
	for _, ext := range []string{".ts", ".tsx"} {
		chunks, ok := chunkFile("worker"+ext, tsSrc)
		if !ok {
			t.Fatalf("expected AST chunking for worker%s", ext)
		}
		gotTS := map[string]int{}
		for _, c := range chunks {
			if c.name != "" {
				gotTS[c.name] = c.startLine
			}
		}
		for name, line := range map[string]int{"BaseWorker": 1, "runTask": 2, "stop": 3} {
			if gotTS[name] != line {
				t.Errorf("TS%s def %s line = %d, want %d (all=%v)", ext, name, gotTS[name], line, gotTS)
			}
		}
	}

	// Rust macro_rules!
	rsSrc := "macro_rules! log_event {\n    ($msg:expr) => { println!(\"{}\", $msg); };\n}\n\npub fn helper() {}\n"
	chunks, ok = chunkFile("macros.rs", rsSrc)
	if !ok {
		t.Fatal("expected AST chunking for macros.rs")
	}
	gotRS := map[string]int{}
	for _, c := range chunks {
		if c.name != "" {
			gotRS[c.name] = c.startLine
		}
	}
	for name, line := range map[string]int{"log_event": 1, "helper": 5} {
		if gotRS[name] != line {
			t.Errorf("Rust def %s line = %d, want %d (all=%v)", name, gotRS[name], line, gotRS)
		}
	}
}
