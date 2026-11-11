package main

import (
	"reflect"
	"strconv"
	"testing"
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
