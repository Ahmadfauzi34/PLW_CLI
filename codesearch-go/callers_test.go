package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestCalleeBase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo", "foo"},
		{"m.getGraphParametersFlat", "getGraphParametersFlat"},
		{"crate::registry::foo", "foo"},
		{"foo::<T>", "foo"},
		{"pkg.Fn[int]", "Fn"},
		{"a.b.c", "c"},
		{"foo::", "foo"},
		{"", ""},
	}
	for _, c := range cases {
		if got := calleeBase(c.in); got != c.want {
			t.Errorf("calleeBase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// callSummary reduces sites to path-independent triples for comparison.
func callSummary(sites []callSite) [][3]string {
	var out [][3]string
	for _, s := range sites {
		out = append(out, [3]string{s.Callee, s.Caller, strconv.Itoa(s.Line)})
	}
	return out
}

func TestExtractCallsGo(t *testing.T) {
	text := "package p\n\nfunc a() {\n\tb()\n\tx.Method()\n}\n\nfunc b() {}\n"
	sites, ok := extractCalls("x.go", text)
	if !ok {
		t.Fatal("expected ok for valid Go")
	}
	// lines: 1 pkg, 2 blank, 3 func a, 4 b(), 5 x.Method(), 6 }, 7 blank,
	// 8 func b.
	want := [][3]string{
		{"b", "a", "4"},
		{"x.Method", "a", "5"},
	}
	if got := callSummary(sites); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestExtractCallsRust(t *testing.T) {
	text := "fn a() {\n    foo::bar();\n    obj.method();\n    generic::<u8>();\n}\n"
	sites, ok := extractCalls("x.rs", text)
	if !ok {
		t.Fatal("expected ok for valid Rust")
	}
	want := [][3]string{
		{"foo::bar", "a", "2"},
		{"obj.method", "a", "3"},
		{"generic::<u8>", "a", "4"},
	}
	if got := callSummary(sites); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestExtractCallsPython(t *testing.T) {
	text := "def a():\n    b()\n    x.y()\n\ndef b():\n    pass\n"
	sites, ok := extractCalls("x.py", text)
	if !ok {
		t.Fatal("expected ok for valid Python")
	}
	want := [][3]string{
		{"b", "a", "2"},
		{"x.y", "a", "3"},
	}
	if got := callSummary(sites); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestExtractCallsTopLevelAndReference(t *testing.T) {
	// A bare reference is not a call; a top-level call has no caller.
	text := "b()\n\ndef a():\n    f = b\n"
	sites, ok := extractCalls("x.py", text)
	if !ok {
		t.Fatal("expected ok for valid Python")
	}
	want := [][3]string{
		{"b", "", "1"},
	}
	if got := callSummary(sites); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestExtractCallsUnsupported(t *testing.T) {
	if _, ok := extractCalls("x.md", "# hi\n"); ok {
		t.Error("expected ok=false for unsupported extension")
	}
}

func TestExtractCallsJS(t *testing.T) {
	text := "function outer() {\n\tfoo();\n\tobj.method();\n}\n\nconst arrow = () => {\n\tbar();\n};\n\nclass C {\n\tm() {\n\t\tbaz();\n\t}\n}\n"
	sites, ok := extractCalls("x.js", text)
	if !ok {
		t.Fatal("expected ok for valid JS")
	}
	// lines: 1 func outer, 2 foo(), 3 obj.method(), 4 }, 5 blank,
	// 6 const arrow, 7 bar(), 8 };, 9 blank, 10 class C, 11 m(), 12 baz(),
	// 13 }, 14 }.
	want := [][3]string{
		{"foo", "outer", "2"},
		{"obj.method", "outer", "3"},
		{"bar", "arrow", "7"},
		{"baz", "m", "12"},
	}
	if got := callSummary(sites); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestExtractCallsMjsTopLevel(t *testing.T) {
	// The demo pattern: member call at top level, no enclosing function.
	text := "import * as m from './x.mjs';\nconst y = m.getGraphParametersFlat(g, r);\n"
	sites, ok := extractCalls("x.mjs", text)
	if !ok {
		t.Fatal("expected ok for valid .mjs")
	}
	want := [][3]string{
		{"m.getGraphParametersFlat", "", "2"},
	}
	if got := callSummary(sites); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestExtractCallsTS(t *testing.T) {
	text := "function f(x: number): void {\n\tgreet(x);\n}\n"
	sites, ok := extractCalls("x.ts", text)
	if !ok {
		t.Fatal("expected ok for valid TS")
	}
	want := [][3]string{
		{"greet", "f", "2"},
	}
	if got := callSummary(sites); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestExtractCallsKotlin(t *testing.T) {
	// K1-K4 (2026-11-12): plain, qualified (client.connect),
	// super/this calls, and a top-level constructor call.
	text := "package a.b\n\nclass Foo {\n    fun run() {\n" +
		"        helper()\n        client.connect()\n        super.toString()\n" +
		"    }\n}\n\nfun helper() {}\n\nval made = Foo()\n"
	sites, ok := extractCalls("x.kt", text)
	if !ok {
		t.Fatal("expected ok for valid Kotlin")
	}
	// lines: 1 pkg, 2 blank, 3 class, 4 fun run, 5 helper(), 6 connect,
	// 7 super, 8 }, 9 }, 10 blank, 11 fun helper, 12 blank, 13 Foo().
	want := [][3]string{
		{"helper", "run", "5"},
		{"client.connect", "run", "6"},
		{"super.toString", "run", "7"},
		{"Foo", "", "13"},
	}
	if got := callSummary(sites); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestExtractCallsJSReferenceNotCall(t *testing.T) {
	// Bare references and member access without () are not calls.
	text := "const h = foo;\nfoo.bar;\n"
	sites, ok := extractCalls("x.js", text)
	if !ok {
		t.Fatal("expected ok for valid JS")
	}
	if len(sites) != 0 {
		t.Errorf("calls = %v, want none", callSummary(sites))
	}
}

func TestSplitBatchNames(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Greet", []string{"Greet"}},
		{"Greet,main", []string{"Greet", "main"}},
		{"Greet, main,,Run", []string{"Greet", "main", "Run"}},
		{"Greet,Greet,main", []string{"Greet", "main"}},
		{"", nil},
		{",,,", nil},
		{"  ", nil},
	}
	for _, c := range cases {
		got := splitBatchNames(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitBatchNames(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitBatchNames(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestExtractCallsJSX(t *testing.T) {
	text := "export function Card(props) {\n\treturn renderItem(props.id);\n}\n"
	sites, ok := extractCalls("src/Card.jsx", text)
	if !ok {
		t.Fatal("expected ok for valid JSX")
	}
	want := [][3]string{{"renderItem", "Card", "2"}}
	if got := callSummary(sites); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestExtractCallsSkipsGeneratedAssets(t *testing.T) {
	body := "export function wrap() { return helper(); }\n"
	for _, tc := range []struct {
		path string
		text string
	}{
		{"src/types.d.ts", "export declare function wrap(): void;\n"},
		{"public/assets/app.js", body},
		{"math compesor/burn_research.js", "/* @ts-self-types=\"./burn_research.d.ts\" */\n" + body},
		{"math compesor/node.mjs", "function f() { return this.__wbg_ptr + helper(); }\n"},
	} {
		if _, ok := extractCalls(tc.path, tc.text); ok {
			t.Errorf("extractCalls(%q) = ok, want skipped", tc.path)
		}
	}
	// Normal JS mentioning wasm-bindgen in a comment must still be extracted.
	doc := "// Talks to wasm-bindgen export helper\nexport function wrap() { return helper(); }\n"
	sites, ok := extractCalls("src/bridge.js", doc)
	if !ok || len(sites) != 1 || sites[0].Callee != "helper" {
		t.Errorf("extractCalls normal JSDoc bridge.js = (%+v, %v), want [helper], true", sites, ok)
	}
}

func captureStdoutRun(fn func() int) (int, string, error) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return 1, "", err
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	rc := fn()
	_ = w.Close()
	os.Stdout = old
	out := <-done
	_ = r.Close()
	return rc, out, nil
}

func TestRunCallersCachedMatchesFresh(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	write := func(rel, body string, mtime time.Time) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(full, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	t0 := time.Now().Add(-10 * time.Second).Truncate(time.Second)
	write("src/a.ts", "export function run() { helper(); }\nexport function helper() {}\n", t0)
	write("src/b.jsx", "export function View() { return helper() + other(); }\n", t0)
	write("public/assets/gen.js", "function ignored() { helper(); other(); }\n", t0)
	write("src/d.d.ts", "declare function helper(): void;\n", t0)

	cfg := defaultScanConfig()
	queryCachedAndFresh := func(q string) (callersEnvelope, callersEnvelope) {
		var envFresh, envCached callersEnvelope
		rc, out, err := captureStdoutRun(func() int {
			return runCallers(options{callers: q, callersSet: true, jsonOut: true, cacheDir: cache}, root, cfg)
		})
		if err != nil || rc != 0 {
			t.Fatalf("fresh runCallers(%q) rc=%d err=%v", q, rc, err)
		}
		if err := json.Unmarshal([]byte(out), &envFresh); err != nil {
			t.Fatalf("unmarshal fresh: %v (%s)", err, out)
		}
		rc, out, err = captureStdoutRun(func() int {
			return runCallers(options{callers: q, callersSet: true, cached: true, jsonOut: true, cacheDir: cache}, root, cfg)
		})
		if err != nil || rc != 0 {
			t.Fatalf("cached runCallers(%q) rc=%d err=%v", q, rc, err)
		}
		if err := json.Unmarshal([]byte(out), &envCached); err != nil {
			t.Fatalf("unmarshal cached: %v (%s)", err, out)
		}
		return envFresh, envCached
	}

	// Cold cached run builds gob index; second run hits warm cache.
	f1, c1 := queryCachedAndFresh("helper,other")
	if !reflect.DeepEqual(f1, c1) || f1.Total != 3 || !f1.Batch {
		t.Errorf("batch mismatch: fresh=%+v cached=%+v", f1, c1)
	}
	f2, c2 := queryCachedAndFresh("helper")
	if !reflect.DeepEqual(f2, c2) || f2.Total != 2 || f2.Batch {
		t.Errorf("warm single mismatch: fresh=%+v cached=%+v", f2, c2)
	}

	// Edit a file and remove another: --cached syncs incrementally and matches fresh.
	t1 := t0.Add(2 * time.Second)
	write("src/a.ts", "export function run() { other(); }\nexport function helper() {}\n", t1)
	if err := os.Remove(filepath.Join(root, "src/b.jsx")); err != nil {
		t.Fatal(err)
	}
	f3, c3 := queryCachedAndFresh("helper,other")
	if !reflect.DeepEqual(f3, c3) || f3.Total != 1 || f3.Calls[0].Callee != "other" {
		t.Errorf("post-sync mismatch: fresh=%+v cached=%+v", f3, c3)
	}
}
