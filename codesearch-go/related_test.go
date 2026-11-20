package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestToCamelToSnake(t *testing.T) {
	cases := []struct{ snake, camel string }{
		{"get_graph_parameters_flat", "getGraphParametersFlat"},
		{"foo", "foo"},
		{"foo_bar", "fooBar"},
		{"my_url_value", "myUrlValue"},
		{"get2d", "get2d"},
	}
	for _, c := range cases {
		if got := toCamel(c.snake); got != c.camel {
			t.Errorf("toCamel(%q) = %q, want %q", c.snake, got, c.camel)
		}
		if got := toSnake(c.camel); got != c.snake {
			t.Errorf("toSnake(%q) = %q, want %q", c.camel, got, c.snake)
		}
	}
	if got := toSnake("myURLValue"); got != "my_url_value" {
		t.Errorf("toSnake(myURLValue) = %q, want my_url_value", got)
	}
	if got := toCamel("myURLValue"); got != "myUrlValue" {
		t.Errorf("toCamel(myURLValue) = %q, want myUrlValue", got)
	}
}

func TestParseExportStmt(t *testing.T) {
	edge, ok, _, _ := parseExportStmt("b.mjs", "export {foo, bar as baz} from './a.mjs';", 3)
	if !ok || edge.Target != "./a.mjs" || edge.Star {
		t.Fatalf("named re-export wrong: %+v ok=%v", edge, ok)
	}
	if edge.Names["foo"] != "foo" || edge.Names["baz"] != "bar" {
		t.Errorf("specifiers wrong: %v", edge.Names)
	}
	edge, ok, _, _ = parseExportStmt("c.mjs", "export * from './b.mjs';", 1)
	if !ok || !edge.Star || edge.Target != "./b.mjs" {
		t.Errorf("star re-export wrong: %+v ok=%v", edge, ok)
	}
	edge, ok, _, _ = parseExportStmt("d.mjs", "export {local} ", 5)
	if !ok || edge.Target != "" || edge.Names["local"] != "local" {
		t.Errorf("local export list wrong: %+v ok=%v", edge, ok)
	}
	_, _, def, ok := parseExportStmt("a.mjs", "export const answer = 42;", 2)
	if !ok || def.Name != "answer" || def.Line != 2 {
		t.Errorf("export const def wrong: %+v ok=%v", def, ok)
	}
	_, ok, _, _ = parseExportStmt("a.mjs", "export function foo() {}", 1)
	if ok {
		t.Errorf("export function must not produce an edge (def query covers it)")
	}
}

func TestResolveImport(t *testing.T) {
	files := map[string]bool{
		"src/a.mjs": true, "src/lib/index.ts": true, "src/b.ts": true,
	}
	if got := resolveImport("src/c.mjs", "./a", files); got != "src/a.mjs" {
		t.Errorf("./a -> %q", got)
	}
	if got := resolveImport("src/c.mjs", "./lib", files); got != "src/lib/index.ts" {
		t.Errorf("./lib -> %q", got)
	}
	if got := resolveImport("src/c.mjs", "./missing", files); got != "" {
		t.Errorf("./missing -> %q, want empty", got)
	}
	if got := resolveImport("src/c.mjs", "react", files); got != "" {
		t.Errorf("bare specifier must not resolve, got %q", got)
	}
}

func TestResolveRelatedChain(t *testing.T) {
	defs := []relDef{
		{Path: "a.mjs", Line: 1, Name: "foo"},
	}
	edges := []relEdge{
		{From: "b.mjs", Line: 2, Target: "./a.mjs", Names: map[string]string{"foo": "foo"}},
		{From: "c.mjs", Line: 3, Target: "./b.mjs", Star: true},
	}
	files := map[string]bool{"a.mjs": true, "b.mjs": true, "c.mjs": true}
	rows := resolveRelated("foo", defs, edges, nil, files)
	kinds := map[string]int{}
	for _, r := range rows {
		kinds[r.Kind]++
	}
	if kinds["def"] != 1 || kinds["re-export"] != 2 {
		t.Errorf("chain rows = %+v, want 1 def + 2 re-exports", rows)
	}
}

func TestResolveRelatedAliasCycleUnresolved(t *testing.T) {
	defs := []relDef{{Path: "a.mjs", Line: 1, Name: "orig"}}
	edges := []relEdge{
		{From: "b.mjs", Line: 2, Target: "./a.mjs", Names: map[string]string{"alias": "orig"}},
		{From: "x.mjs", Line: 1, Target: "./y.mjs", Names: map[string]string{"loop": "loop"}},
		{From: "y.mjs", Line: 1, Target: "./x.mjs", Names: map[string]string{"loop": "loop"}},
		{From: "u.mjs", Line: 1, Target: "./gone.mjs", Names: map[string]string{"ghost": "ghost"}},
	}
	files := map[string]bool{"a.mjs": true, "b.mjs": true, "x.mjs": true, "y.mjs": true, "u.mjs": true}
	rows := resolveRelated("alias", defs, edges, nil, files)
	if len(rows) != 1 || rows[0].Kind != "re-export" || !strings.Contains(rows[0].Detail, "a.mjs:1") {
		t.Errorf("alias rows = %+v", rows)
	}
	rows = resolveRelated("loop", defs, edges, nil, files)
	if len(rows) != 2 {
		t.Errorf("cycle rows = %+v, want exactly 2 (no infinite loop)", rows)
	}
	rows = resolveRelated("ghost", defs, edges, nil, files)
	if len(rows) != 1 || !strings.Contains(rows[0].Detail, "not in index") {
		t.Errorf("unresolved rows = %+v", rows)
	}
}

// F2: a barrel of `export *` must not drown the answer in dead-end
// traces. Only the row on the chain that ends at the definition is
// signal; the rest is noise.
func TestResolveRelatedBarrelDeadEnds(t *testing.T) {
	defs := []relDef{{Path: "rag/holographic-memory.ts", Line: 1, Name: "HolographicMemoryField"}}
	edges := []relEdge{
		{From: "rag/index.ts", Line: 1, Target: "./holographic-memory", Star: true},
		{From: "rag/index.ts", Line: 2, Target: "./base-indexer", Star: true},
		{From: "rag/index.ts", Line: 3, Target: "./chunk-store", Star: true},
	}
	files := map[string]bool{
		"rag/index.ts": true, "rag/holographic-memory.ts": true,
		"rag/base-indexer.ts": true, "rag/chunk-store.ts": true,
	}
	rows := resolveRelated("HolographicMemoryField", defs, edges, nil, files)
	if len(rows) != 2 {
		t.Fatalf("barrel rows = %+v, want def + 1 live re-export", rows)
	}
	kinds := map[string]int{}
	for _, r := range rows {
		kinds[r.Kind]++
		if strings.Contains(r.Detail, "not found there") {
			t.Errorf("dead-end row kept: %+v", r)
		}
	}
	if kinds["def"] != 1 || kinds["re-export"] != 1 {
		t.Errorf("barrel kinds = %v, want 1 def + 1 re-export", kinds)
	}
	if !strings.Contains(rows[1].Detail, "defined rag/holographic-memory.ts:1") {
		t.Errorf("live re-export wrong: %+v", rows[1])
	}
}

// F2: when no definition is found anywhere, the trace is the only
// information — it is kept whole, not filtered to empty.
func TestResolveRelatedNoDefKeepsTrace(t *testing.T) {
	defs := []relDef{{Path: "other.ts", Line: 1, Name: "Other"}}
	edges := []relEdge{
		{From: "index.ts", Line: 1, Target: "./a", Star: true},
		{From: "index.ts", Line: 2, Target: "./b", Star: true},
	}
	files := map[string]bool{"index.ts": true, "a.ts": true, "b.ts": true}
	rows := resolveRelated("Missing", defs, edges, nil, files)
	if len(rows) != 2 {
		t.Fatalf("no-def rows = %+v, want the full 2-row trace", rows)
	}
	for _, r := range rows {
		if !strings.Contains(r.Detail, "not found there") {
			t.Errorf("no-def row wrong: %+v", r)
		}
	}
}

// F2 (a): a TS class name is a type_identifier in the TS grammar,
// not an identifier — the shared def query must accept both, or
// --related never sees a class definition.
func TestExtractRelatedJSTSClass(t *testing.T) {
	text := "export class HolographicMemoryField {\n  constructor() {}\n}\n\n" +
		"export interface FieldConfig {\n  dim: number;\n}\n\n" +
		"export enum Color {\n  Red = 1,\n}\n"
	defs, _, _, ok := extractRelated("rag/holographic-memory.ts", text)
	if !ok {
		t.Fatal("expected ok for valid TS")
	}
	names := map[string]bool{}
	for _, d := range defs {
		names[d.Name] = true
	}
	for _, want := range []string{"HolographicMemoryField", "FieldConfig", "Color"} {
		if !names[want] {
			t.Errorf("defs = %+v, want %s", defs, want)
		}
	}
}

func TestExtractRelatedKotlin(t *testing.T) {
	// K1-K4 (2026-11-12): class/function/interface/enum/typealias defs
	// plus the top-level val; member property "count" and enum entries
	// are NOT defs.
	text := "package a.b\n\nclass HostLauncher {\n    val count = 0\n" +
		"    fun start(): Boolean { return true }\n}\n\n" +
		"interface Shape { fun area(): Double }\n\n" +
		"enum class Color { RED, GREEN }\n\n" +
		"val appName = \"pipkin\"\n\nfun topLevel() {}\n"
	defs, _, _, ok := extractRelated("x.kt", text)
	if !ok {
		t.Fatal("expected ok for valid Kotlin")
	}
	names := map[string]bool{}
	for _, d := range defs {
		names[d.Name] = true
	}
	for _, want := range []string{"HostLauncher", "start", "Shape", "area", "Color", "appName", "topLevel"} {
		if !names[want] {
			t.Errorf("defs = %+v, want %s", defs, want)
		}
	}
	for _, no := range []string{"count", "RED", "GREEN"} {
		if names[no] {
			t.Errorf("defs = %+v, must not contain %s", defs, no)
		}
	}
}

func TestExtractRelatedKotlinAnnotationVisibility(t *testing.T) {
	// Parity (2026-11-20): annotation+visibility on ONE line defeats
	// the vendored grammar; the regex fallback recovers the def and
	// --related must agree with --defs-only (anchored line), while a
	// local val inside the recovered body must not leak as top-level.
	text := "package demo\n\n" +
		"@Composable internal fun PrimaryChip(label: String) {\n" +
		"    val localTmp = label.length\n" +
		"    renderIcon(label)\n" +
		"}\n\n" +
		"@Deprecated(\"old\") internal class LegacyBox {\n" +
		"    val flag = 1\n" +
		"}\n\n" +
		"internal object BridgeClient {\n" +
		"    val flag = 1\n" +
		"}\n\n" +
		"const val TOP_LEVEL_FLAG = true\n"
	defs, _, _, ok := extractRelated("x.kt", text)
	if !ok {
		t.Fatal("expected ok for valid Kotlin")
	}
	byName := map[string]int{}
	for _, d := range defs {
		byName[d.Name] = d.Line
	}
	// Lines: 1 pkg, 2 blank, 3 PrimaryChip, 4 localTmp, 5 renderIcon,
	// 6 }, 7 blank, 8 LegacyBox, 9 flag, 10 }, 11 blank,
	// 12 BridgeClient, 13 flag, 14 }, 15 blank, 16 TOP_LEVEL_FLAG.
	for name, line := range map[string]int{
		"PrimaryChip": 3, "LegacyBox": 8, "BridgeClient": 12, "TOP_LEVEL_FLAG": 16,
	} {
		if got, present := byName[name]; !present || got != line {
			t.Errorf("defs = %+v, want %s at line %d (got %d, present=%v)", defs, name, line, got, present)
		}
	}
	for _, no := range []string{"localTmp", "flag"} {
		if _, present := byName[no]; present {
			t.Errorf("defs = %+v, must not contain %s", defs, no)
		}
	}
	// resolveRelated returns exactly one row for the recovered def.
	rows := resolveRelated("PrimaryChip", defs, nil, nil, map[string]bool{})
	if len(rows) != 1 || rows[0].Line != 3 {
		t.Errorf("resolveRelated(PrimaryChip) = %+v, want exactly 1 row at line 3", rows)
	}

	// Annotation on its own line before `fun` also defeats the grammar
	// (probe matrix 2026-11-20); before `class` it parses.
	sep := "package demo\n\n@Composable\ninternal fun SeparateChip(label: String) {\n    renderIcon(label)\n}\n"
	sdefs, _, _, ok := extractRelated("x.kt", sep)
	if !ok {
		t.Fatal("expected ok for valid Kotlin")
	}
	found := false
	for _, d := range sdefs {
		if d.Name == "SeparateChip" && d.Line == 4 {
			found = true
		}
	}
	if !found {
		t.Errorf("separate-line defs = %+v, want SeparateChip at line 4", sdefs)
	}
}

// Regression (2026-11-20, found by the OOPBoT corpus differential):
// tree-sitter attaches a trailing `// ...` on the annotation line as
// a child of the declaration, between the modifiers node and the
// keyword. kotlinAnchorStart used to return that comment's byte, so
// the AST def landed on the annotation line (3) while the regex
// fallback (fired because the file HasError) landed on the keyword
// line (4) — and mergeKotlinRegexDefs kept both, so --related
// reported ChatTimeline twice (lines 28 and 29 in the real file).
// The anchor must skip comment nodes in both loops.
func TestExtractRelatedKotlinAnnotationTrailingComment(t *testing.T) {
	// Lines: 1 pkg, 2 blank, 3 @Suppress + trailing comment,
	// 4 class, 5 clock, 6 ) {, 7 lock, 8 }, 9 blank,
	// 10 @Composable internal fun BrokenChip (grammar-defeating,
	// recovered by the regex; also the HasError trigger that
	// makes the regex run at all in this file).
	text := "package demo\n\n" +
		"@Suppress(\"TooManyFunctions\") // Owns everything the chat renders.\n" +
		"class ChatTimeline(\n" +
		"  private val clock: () -> Long = System::currentTimeMillis,\n" +
		") {\n" +
		"  private val lock = Any()\n" +
		"}\n\n" +
		"@Composable internal fun BrokenChip(label: String) {}\n"
	defs, _, _, ok := extractRelated("x.kt", text)
	if !ok {
		t.Fatal("expected ok for valid Kotlin")
	}
	count := map[string]int{}
	byName := map[string]int{}
	for _, d := range defs {
		count[d.Name]++
		byName[d.Name] = d.Line
	}
	if count["ChatTimeline"] != 1 || byName["ChatTimeline"] != 4 {
		t.Errorf("defs = %+v, want exactly 1 ChatTimeline at line 4", defs)
	}
	if got, present := byName["BrokenChip"]; !present || got != 10 {
		t.Errorf("defs = %+v, want BrokenChip at line 10 (got %d, present=%v)", defs, got, present)
	}
	for _, no := range []string{"clock", "lock"} {
		if _, present := byName[no]; present {
			t.Errorf("defs = %+v, must not contain %s", defs, no)
		}
	}
	rows := resolveRelated("ChatTimeline", defs, nil, nil, map[string]bool{})
	if len(rows) != 1 || rows[0].Line != 4 {
		t.Errorf("resolveRelated(ChatTimeline) = %+v, want exactly 1 row at line 4", rows)
	}
}

// Regression (2026-11-20, OOPBoT differential): an extension function
// with a multi-segment receiver (`TimelineItem.ToolCall.cardText`)
// was named after its receiver (`ToolCall`) by the regex fallback,
// which skipped at most one dotted segment. That produced a phantom
// def in --related and a wrong caller in --callers. The regex now
// skips all receiver segments, so the fallback def dedups against
// the AST def (`cardText`).
func TestExtractRelatedKotlinExtensionReceiver(t *testing.T) {
	// Lines: 1 pkg, 2 blank, 3 extension fun, 4 return, 5 },
	// 6 blank, 7 BrokenChip (grammar-defeating: the HasError
	// trigger that makes the regex run at all in this file).
	text := "package demo\n\n" +
		"internal fun TimelineItem.ToolCall.cardText(): String {\n" +
		"  return editPairs(args)\n" +
		"}\n\n" +
		"@Composable internal fun BrokenChip(label: String) {}\n"
	defs, _, _, ok := extractRelated("x.kt", text)
	if !ok {
		t.Fatal("expected ok for valid Kotlin")
	}
	byName := map[string]int{}
	for _, d := range defs {
		byName[d.Name] = d.Line
	}
	if got, present := byName["cardText"]; !present || got != 3 {
		t.Errorf("defs = %+v, want cardText at line 3 (got %d, present=%v)", defs, got, present)
	}
	if _, present := byName["ToolCall"]; present {
		t.Errorf("defs = %+v, must not contain phantom receiver name ToolCall", defs)
	}
	if got, present := byName["BrokenChip"]; !present || got != 7 {
		t.Errorf("defs = %+v, want BrokenChip at line 7 (got %d, present=%v)", defs, got, present)
	}
	rows := resolveRelated("cardText", defs, nil, nil, map[string]bool{})
	if len(rows) != 1 || rows[0].Line != 3 {
		t.Errorf("resolveRelated(cardText) = %+v, want exactly 1 row at line 3", rows)
	}
}

func TestResolveRelatedFFI(t *testing.T) {
	defs := []relDef{{Path: "bind.mjs", Line: 5, Name: "getGraphParametersFlat"}}
	bridges := []relBridge{
		{RustPath: "src/lib.rs", RustLine: 42, RustName: "get_graph_parameters_flat", JSName: "getGraphParametersFlat"},
		{RustPath: "src/lib.rs", RustLine: 50, RustName: "custom_fn", JSName: "renamedFn"},
	}
	rows := resolveRelated("getGraphParametersFlat", defs, nil, bridges, map[string]bool{})
	found := false
	for _, r := range rows {
		if r.Kind == "ffi" && strings.Contains(r.Detail, "bind.mjs:5") {
			found = true
		}
	}
	if !found {
		t.Errorf("ffi rows = %+v, want paired js def", rows)
	}
	// Query by snake name finds the same bridge; js_name override kept.
	rows = resolveRelated("custom_fn", nil, nil, bridges, map[string]bool{})
	if len(rows) != 1 || rows[0].Kind != "ffi" || !strings.Contains(rows[0].Detail, "renamedFn") {
		t.Errorf("js_name rows = %+v", rows)
	}
	// Query by the JS name itself (override direction): the bridge must
	// be found even though "renamedFn" does not round-trip to custom_fn.
	rows = resolveRelated("renamedFn", nil, nil, bridges, map[string]bool{})
	if len(rows) != 1 || rows[0].Kind != "ffi" || !strings.Contains(rows[0].Detail, "custom_fn") {
		t.Errorf("js-name query rows = %+v", rows)
	}
	rows = resolveRelated("renamed_fn", nil, nil, bridges, map[string]bool{})
	if len(rows) != 1 || rows[0].Kind != "ffi" {
		t.Errorf("js-name snake query rows = %+v", rows)
	}
}

func TestExtractRelatedJS(t *testing.T) {
	text := "export function foo() {\n  return 1;\n}\n" +
		"export const bar = 2;\n" +
		"export {foo as renamed} from './a.mjs';\n" +
		"export * from './b.mjs';\n" +
		"export {bar};\n"
	defs, edges, _, ok := extractRelated("c.mjs", text)
	if !ok {
		t.Fatal("expected ok for valid JS")
	}
	names := map[string]bool{}
	for _, d := range defs {
		names[d.Name] = true
	}
	if !names["foo"] || !names["bar"] {
		t.Errorf("defs = %+v, want foo and bar", defs)
	}
	if len(edges) != 3 {
		t.Fatalf("edges = %+v, want 3", edges)
	}
	var named, star, local *relEdge
	for i := range edges {
		switch {
		case edges[i].Star:
			star = &edges[i]
		case edges[i].Target == "./a.mjs":
			named = &edges[i]
		case edges[i].Target == "":
			local = &edges[i]
		}
	}
	if named == nil || named.Names["renamed"] != "foo" {
		t.Errorf("named edge wrong: %+v", named)
	}
	if star == nil || star.Target != "./b.mjs" {
		t.Errorf("star edge wrong: %+v", star)
	}
	if local == nil || local.Names["bar"] != "bar" {
		t.Errorf("local edge wrong: %+v", local)
	}
}

func TestExtractRelatedRust(t *testing.T) {
	text := "#[wasm_bindgen]\npub fn plain_fn() {}\n\n" +
		"#[wasm_bindgen(js_name = \"renamedFn\")]\n#[allow(dead_code)]\npub fn custom_fn() {}\n\n" +
		"pub fn not_bridged() {}\n\n" +
		"#[wasm_bindgen]\nimpl Widget {\n    pub fn method_fn(&self) {}\n}\n\n" +
		"impl Plain {\n    pub fn plain_method(&self) {}\n}\n"
	_, _, bridges, ok := extractRelated("lib.rs", text)
	if !ok {
		t.Fatal("expected ok for valid Rust")
	}
	if len(bridges) != 3 {
		t.Fatalf("bridges = %+v, want plain_fn + custom_fn + Widget::method_fn", bridges)
	}
	byRust := map[string]relBridge{}
	for _, b := range bridges {
		byRust[b.RustName] = b
	}
	if b, ok := byRust["plain_fn"]; !ok || b.JSName != "plainFn" || b.RustLine != 2 {
		t.Errorf("plain_fn bridge wrong: %+v", b)
	}
	if b, ok := byRust["custom_fn"]; !ok || b.JSName != "renamedFn" {
		t.Errorf("custom_fn bridge wrong: %+v", b)
	}
	if b, ok := byRust["method_fn"]; !ok || b.Class != "Widget" || b.JSName != "methodFn" {
		t.Errorf("method_fn bridge wrong: %+v", b)
	}
	if _, ok := byRust["plain_method"]; ok {
		t.Errorf("plain impl method must not bridge")
	}
}

func TestExtractRelatedJSConsts(t *testing.T) {
	text := "const topLevel = 1;\n" +
		"export const exported = 2;\n" +
		"function f() {\n  const local = 3;\n  return local;\n}\n" +
		"const arrow = () => topLevel;\n"
	defs, _, _, ok := extractRelated("c.mjs", text)
	if !ok {
		t.Fatal("expected ok for valid JS")
	}
	names := map[string]int{}
	for _, d := range defs {
		names[d.Name]++
	}
	if names["topLevel"] != 1 {
		t.Errorf("topLevel count = %d, want 1", names["topLevel"])
	}
	if names["exported"] != 1 {
		t.Errorf("exported count = %d, want 1 (dedup export-const)", names["exported"])
	}
	if names["local"] != 0 {
		t.Errorf("local must not be tracked, got %d", names["local"])
	}
	if names["arrow"] != 1 {
		t.Errorf("arrow count = %d, want 1 (def query, not double via var query)", names["arrow"])
	}
	if names["f"] != 1 {
		t.Errorf("f count = %d, want 1", names["f"])
	}
}

func TestRelatedForSelfFilter(t *testing.T) {
	ix := newMemIndex("root")
	ix.files = map[string]FileMeta{
		"a.mjs": {}, "b.mjs": {},
	}
	ix.relDefs = []relDef{{Path: "a.mjs", Line: 1, Name: "foo"}}
	ix.relEdges = []relEdge{
		{From: "b.mjs", Line: 1, Target: "./a.mjs", Names: map[string]string{"foo": "foo"}},
	}
	rows := ix.relatedFor("foo", "a.mjs", 1)
	if len(rows) != 1 || rows[0].Kind != "re-export" || rows[0].Path != "b.mjs" {
		t.Errorf("relatedFor = %+v, want only the b.mjs re-export (self def filtered)", rows)
	}
	// Unknown name: no rows, never an error.
	if rows := ix.relatedFor("nope", "a.mjs", 1); len(rows) != 0 {
		t.Errorf("relatedFor unknown = %+v, want none", rows)
	}
}

func TestExtractRelatedSkipsGeneratedAssets(t *testing.T) {
	body := "export function foo() { return 1; }\n"
	for _, tc := range []struct {
		path string
		text string
	}{
		{"src/types.d.ts", "export declare function foo(): number;\n"},
		{"public/assets/app.mjs", body},
		{"math compesor/burn_research.js", "/* @ts-self-types=\"./burn_research.d.ts\" */\n" + body},
		{"math compesor/glue.mjs", "function m() { return __wbindgen_malloc(1); }\n" + body},
	} {
		if _, _, _, ok := extractRelated(tc.path, tc.text); ok {
			t.Errorf("extractRelated(%q) = ok, want skipped", tc.path)
		}
	}
	doc := "// Talks to wasm-bindgen bridge\n" + body
	defs, _, _, ok := extractRelated("src/bridge.mjs", doc)
	if !ok || len(defs) != 1 || defs[0].Name != "foo" {
		t.Errorf("extractRelated normal JSDoc bridge.mjs = (%+v, %v), want [foo], true", defs, ok)
	}
}

func TestRunRelatedCachedMatchesFresh(t *testing.T) {
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
	write("src/impl.ts", "export function buildGraph() { return 1; }\n", t0)
	write("src/index.ts", "export { buildGraph } from './impl';\n", t0)
	write("src/lib.rs", "#[wasm_bindgen]\npub fn build_graph() {}\n", t0)
	write("src/Host.kt", "package io.test\nclass HostRunner {\n  fun buildGraph() {}\n}\n", t0)
	write("src/impl.d.ts", "export declare function buildGraph(): number;\n", t0)

	cfg := defaultScanConfig()
	queryCachedAndFresh := func(q string) (relatedEnvelope, relatedEnvelope) {
		var envFresh, envCached relatedEnvelope
		rc, out, err := captureStdoutRun(func() int {
			return runRelated(options{related: q, relatedSet: true, jsonOut: true, cacheDir: cache}, root, cfg)
		})
		if err != nil || rc != 0 {
			t.Fatalf("fresh runRelated(%q) rc=%d err=%v", q, rc, err)
		}
		if err := json.Unmarshal([]byte(out), &envFresh); err != nil {
			t.Fatalf("unmarshal fresh: %v (%s)", err, out)
		}
		rc, out, err = captureStdoutRun(func() int {
			return runRelated(options{related: q, relatedSet: true, cached: true, jsonOut: true, cacheDir: cache}, root, cfg)
		})
		if err != nil || rc != 0 {
			t.Fatalf("cached runRelated(%q) rc=%d err=%v", q, rc, err)
		}
		if err := json.Unmarshal([]byte(out), &envCached); err != nil {
			t.Fatalf("unmarshal cached: %v (%s)", err, out)
		}
		return envFresh, envCached
	}

	f1, c1 := queryCachedAndFresh("buildGraph")
	if f1.Total != c1.Total || len(f1.Related) != len(c1.Related) || f1.Total != 4 {
		t.Fatalf("related count mismatch: fresh=%+v cached=%+v", f1, c1)
	}
	for i := range f1.Related {
		if f1.Related[i] != c1.Related[i] {
			t.Errorf("row[%d] mismatch: fresh=%+v cached=%+v", i, f1.Related[i], c1.Related[i])
		}
	}

	// Edit + delete sync: verify cached stays identical to fresh.
	t1 := t0.Add(2 * time.Second)
	write("src/impl.ts", "export function buildGraph2() { return 2; }\n", t1)
	if err := os.Remove(filepath.Join(root, "src/Host.kt")); err != nil {
		t.Fatal(err)
	}
	f2, c2 := queryCachedAndFresh("buildGraph,buildGraph2")
	if f2.Total != c2.Total || !f2.Batch || !c2.Batch || len(f2.Related) != len(c2.Related) {
		t.Fatalf("post-sync related mismatch: fresh=%+v cached=%+v", f2, c2)
	}
	for i := range f2.Related {
		if f2.Related[i] != c2.Related[i] {
			t.Errorf("post-sync row[%d] mismatch: fresh=%+v cached=%+v", i, f2.Related[i], c2.Related[i])
		}
	}
}
