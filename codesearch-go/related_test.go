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
	rows := resolveRelated("foo", defs, edges, nil, nil, files)
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
	rows := resolveRelated("alias", defs, edges, nil, nil, files)
	if len(rows) != 1 || rows[0].Kind != "re-export" || !strings.Contains(rows[0].Detail, "a.mjs:1") {
		t.Errorf("alias rows = %+v", rows)
	}
	rows = resolveRelated("loop", defs, edges, nil, nil, files)
	if len(rows) != 2 {
		t.Errorf("cycle rows = %+v, want exactly 2 (no infinite loop)", rows)
	}
	rows = resolveRelated("ghost", defs, edges, nil, nil, files)
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
	rows := resolveRelated("HolographicMemoryField", defs, edges, nil, nil, files)
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
	rows := resolveRelated("Missing", defs, edges, nil, nil, files)
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
	defs, _, _, _, ok := extractRelated("rag/holographic-memory.ts", text)
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
	defs, _, _, _, ok := extractRelated("x.kt", text)
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
	defs, _, _, _, ok := extractRelated("x.kt", text)
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
	rows := resolveRelated("PrimaryChip", defs, nil, nil, nil, map[string]bool{})
	if len(rows) != 1 || rows[0].Line != 3 {
		t.Errorf("resolveRelated(PrimaryChip) = %+v, want exactly 1 row at line 3", rows)
	}

	// Annotation on its own line before `fun` also defeats the grammar
	// (probe matrix 2026-11-20); before `class` it parses.
	sep := "package demo\n\n@Composable\ninternal fun SeparateChip(label: String) {\n    renderIcon(label)\n}\n"
	sdefs, _, _, _, ok := extractRelated("x.kt", sep)
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
	defs, _, _, _, ok := extractRelated("x.kt", text)
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
	rows := resolveRelated("ChatTimeline", defs, nil, nil, nil, map[string]bool{})
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
	defs, _, _, _, ok := extractRelated("x.kt", text)
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
	rows := resolveRelated("cardText", defs, nil, nil, nil, map[string]bool{})
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
	rows := resolveRelated("getGraphParametersFlat", defs, nil, bridges, nil, map[string]bool{})
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
	rows = resolveRelated("custom_fn", nil, nil, bridges, nil, map[string]bool{})
	if len(rows) != 1 || rows[0].Kind != "ffi" || !strings.Contains(rows[0].Detail, "renamedFn") {
		t.Errorf("js_name rows = %+v", rows)
	}
	// Query by the JS name itself (override direction): the bridge must
	// be found even though "renamedFn" does not round-trip to custom_fn.
	rows = resolveRelated("renamedFn", nil, nil, bridges, nil, map[string]bool{})
	if len(rows) != 1 || rows[0].Kind != "ffi" || !strings.Contains(rows[0].Detail, "custom_fn") {
		t.Errorf("js-name query rows = %+v", rows)
	}
	rows = resolveRelated("renamed_fn", nil, nil, bridges, nil, map[string]bool{})
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
	defs, edges, _, _, ok := extractRelated("c.mjs", text)
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
		"#[wasm_bindgen]\nimpl Widget {\n    pub fn method_fn(&self) {}\n    pub const KIND: u32 = 1;\n}\n\n" +
		"impl Plain {\n    pub fn plain_method(&self) {}\n}\n\n" +
		"pub const LIMIT: u32 = 10;\npub static MODE: &str = \"x\";\n" +
		"fn with_local() {\n    const INNER: u32 = 1;\n}\n"
	defs, _, bridges, _, ok := extractRelated("lib.rs", text)
	if !ok {
		t.Fatal("expected ok for valid Rust")
	}
	// P1-B (go14): non-bindgen symbols are defs now, not nil.
	names := map[string]int{}
	for _, d := range defs {
		names[d.Name]++
	}
	for _, want := range []string{"plain_fn", "custom_fn", "not_bridged", "method_fn", "with_local", "LIMIT", "MODE"} {
		if names[want] != 1 {
			t.Errorf("def %q count = %d, want 1 (defs %v)", want, names[want], defs)
		}
	}
	// No struct-name duplication from impl_item; no member/local consts.
	for _, banned := range []string{"Plain", "Widget", "KIND", "INNER"} {
		if names[banned] != 0 {
			t.Errorf("def %q count = %d, want 0", banned, names[banned])
		}
	}
	if len(bridges) != 3 {
		t.Fatalf("bridges = %+v, want plain_fn + custom_fn + Widget::method_fn", bridges)
	}
	byRust := map[string]relBridge{}
	for _, b := range bridges {
		byRust[b.RustName] = b
	}
	if b, ok := byRust["plain_fn"]; !ok || b.JSName != "plainFn" || b.RustLine != 2 || b.Kind != "wasm" {
		t.Errorf("plain_fn bridge wrong: %+v", b)
	}
	if b, ok := byRust["custom_fn"]; !ok || b.JSName != "renamedFn" || b.Kind != "wasm" {
		t.Errorf("custom_fn bridge wrong: %+v", b)
	}
	if b, ok := byRust["method_fn"]; !ok || b.Class != "Widget" || b.JSName != "methodFn" || b.Kind != "wasm" {
		t.Errorf("method_fn bridge wrong: %+v", b)
	}
	if _, ok := byRust["plain_method"]; ok {
		t.Errorf("plain impl method must not bridge")
	}
}

// TestExtractRelatedRustReexport checks the `pub use` edges from real
// text: groups, aliases, globs, visibility forms — and that a private
// `use` produces no edge.
func TestExtractRelatedRustReexport(t *testing.T) {
	text := "pub use crate::engine::{Engine, run};\n" +
		"pub use crate::engine::run as go;\n" +
		"pub use self::inner::*;\n" +
		"use crate::secret::hidden;\n" +
		"pub(crate) use super::sib::Thing;\n"
	_, edges, _, _, ok := extractRelated("src/lib.rs", text)
	if !ok {
		t.Fatal("expected ok for valid Rust")
	}
	// 2 (group) + 1 (alias) + 1 (glob) + 0 (private) + 1 (pub(crate)).
	if len(edges) != 5 {
		t.Fatalf("edges = %+v, want 5", edges)
	}
	byTarget := map[string][]relEdge{}
	for _, e := range edges {
		byTarget[e.Target] = append(byTarget[e.Target], e)
	}
	if len(byTarget["crate::engine"]) != 3 {
		t.Errorf("crate::engine edges = %d, want 3 (group x2 + alias)", len(byTarget["crate::engine"]))
	}
	star := false
	for _, e := range edges {
		if e.Target == "self::inner" && e.Star {
			star = true
		}
	}
	if !star {
		t.Errorf("missing self::inner star edge: %+v", edges)
	}
	found := false
	for _, e := range edges {
		if e.Target == "super::sib" && e.Names["Thing"] == "Thing" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing super::sib Thing edge: %+v", edges)
	}
}

func TestParseRustUse(t *testing.T) {
	type wantEdge struct {
		target string
		star   bool
		names  map[string]string
	}
	cases := []struct {
		in   string
		want []wantEdge
	}{
		{"pub use crate::foo::Bar;", []wantEdge{{"crate::foo", false, map[string]string{"Bar": "Bar"}}}},
		{"pub use crate::foo::{Bar, Baz};", []wantEdge{
			{"crate::foo", false, map[string]string{"Bar": "Bar"}},
			{"crate::foo", false, map[string]string{"Baz": "Baz"}},
		}},
		{"pub use crate::foo::{Bar as Qux};", []wantEdge{{"crate::foo", false, map[string]string{"Qux": "Bar"}}}},
		{"pub use crate::foo::*;", []wantEdge{{"crate::foo", true, nil}}},
		{"pub use self::x;", []wantEdge{{"self", false, map[string]string{"x": "x"}}}},
		{"pub use super::y::Z;", []wantEdge{{"super::y", false, map[string]string{"Z": "Z"}}}},
		{"pub use a::{b::c, d};", []wantEdge{
			{"a::b", false, map[string]string{"c": "c"}},
			{"a", false, map[string]string{"d": "d"}},
		}},
		{"pub use a::{b::{c, d}, e};", []wantEdge{
			{"a::b", false, map[string]string{"c": "c"}},
			{"a::b", false, map[string]string{"d": "d"}},
			{"a", false, map[string]string{"e": "e"}},
		}},
		{"pub use a::self;", []wantEdge{{"", false, map[string]string{"a": "a"}}}},
		{"pub use foo;", []wantEdge{{"", false, map[string]string{"foo": "foo"}}}},
		{"pub use crate::a::b::{self};", []wantEdge{{"crate::a", false, map[string]string{"b": "b"}}}},
		{"pub(crate) use crate::x::Y;", []wantEdge{{"crate::x", false, map[string]string{"Y": "Y"}}}},
		{"use crate::x::Y;", nil},
		{"pub fn not_a_use() {}", nil},
		{"pub use crate::x::Y", []wantEdge{{"crate::x", false, map[string]string{"Y": "Y"}}}},
		// go15: comments inside multiline use, nested block comments,
		// whitespace/newline around `as`, and `self as alias`.
		{"pub use crate::foo::{\n    // main type\n    Bar,\n    /* nested /* block */ comment */ Baz,\n};", []wantEdge{
			{"crate::foo", false, map[string]string{"Bar": "Bar"}},
			{"crate::foo", false, map[string]string{"Baz": "Baz"}},
		}},
		{"pub use crate::foo::Bar\n\tas\tQux;", []wantEdge{{"crate::foo", false, map[string]string{"Qux": "Bar"}}}},
		{"pub use crate::a::b::{self as my_b, Item};", []wantEdge{
			{"crate::a", false, map[string]string{"my_b": "b"}},
			{"crate::a::b", false, map[string]string{"Item": "Item"}},
		}},
		{"pub use a::self as alias_a;", []wantEdge{{"", false, map[string]string{"alias_a": "a"}}}},
	}
	for _, c := range cases {
		got := parseRustUse("f.rs", c.in, 1)
		if len(got) != len(c.want) {
			t.Errorf("parseRustUse(%q) = %+v, want %d edge(s)", c.in, got, len(c.want))
			continue
		}
		for i, w := range c.want {
			g := got[i]
			if g.Target != w.target || g.Star != w.star || len(g.Names) != len(w.names) {
				t.Errorf("parseRustUse(%q)[%d] = %+v, want target=%q star=%v names=%v", c.in, i, g, w.target, w.star, w.names)
				continue
			}
			for k, v := range w.names {
				if g.Names[k] != v {
					t.Errorf("parseRustUse(%q)[%d] names = %v, want %v", c.in, i, g.Names, w.names)
				}
			}
		}
	}
}

func TestResolveRustImport(t *testing.T) {
	files := map[string]bool{
		"src/lib.rs": true, "src/engine.rs": true, "src/engine/inner.rs": true,
		"src/engine/inner/deep.rs": true, "src/other/mod.rs": true, "src/other/util.rs": true,
		"flat.rs": true, "flat2.rs": true,
	}
	cases := []struct{ from, spec, want string }{
		{"src/lib.rs", "crate::engine", "src/engine.rs"},
		{"src/lib.rs", "crate::engine::inner", "src/engine/inner.rs"},
		{"src/engine.rs", "self::inner", "src/engine/inner.rs"},
		{"src/engine.rs", "self::inner::deep", "src/engine/inner/deep.rs"},
		{"src/engine/inner.rs", "super::inner", "src/engine/inner.rs"},
		{"src/engine/inner.rs", "super::super::other", "src/other/mod.rs"},
		{"src/lib.rs", "engine", "src/engine.rs"},
		{"src/other/mod.rs", "self::util", "src/other/util.rs"},
		{"src/lib.rs", "::engine", "src/engine.rs"},
		{"flat.rs", "crate::flat2", "flat2.rs"},
		{"src/lib.rs", "crate::missing", ""},
		{"src/lib.rs", "crate::engine::gone", ""},
		// go15: bare super / super::super / crate / self targets.
		{"src/engine/inner.rs", "super", "src/engine.rs"},
		{"src/engine/inner.rs", "super::super", "src/lib.rs"},
		{"src/other/util.rs", "super", "src/other/mod.rs"},
		{"src/engine.rs", "crate", "src/lib.rs"},
		{"src/lib.rs", "self", "src/lib.rs"},
		{"src/lib.rs", "super::super::super", ""},
	}
	for _, c := range cases {
		if got := resolveRustImport(c.from, c.spec, files); got != c.want {
			t.Errorf("resolveRustImport(%q, %q) = %q, want %q", c.from, c.spec, got, c.want)
		}
	}
}

func TestResolveRelatedRustChain(t *testing.T) {
	defs := []relDef{
		{Path: "src/engine.rs", Line: 1, Name: "Engine"},
		{Path: "src/engine.rs", Line: 2, Name: "run"},
		{Path: "src/engine/inner.rs", Line: 1, Name: "helper"},
	}
	edges := []relEdge{
		{From: "src/lib.rs", Line: 1, Target: "crate::engine", Names: map[string]string{"Engine": "Engine", "run": "run"}},
		{From: "src/engine.rs", Line: 5, Target: "self::inner", Names: map[string]string{"helper": "helper"}},
		{From: "src/lib.rs", Line: 2, Target: "crate::engine", Names: map[string]string{"go": "run"}},
		{From: "src/a.rs", Line: 1, Target: "crate::b", Names: map[string]string{"loop": "loop"}},
		{From: "src/b.rs", Line: 1, Target: "crate::a", Names: map[string]string{"loop": "loop"}},
	}
	files := map[string]bool{"src/lib.rs": true, "src/engine.rs": true, "src/engine/inner.rs": true, "src/a.rs": true, "src/b.rs": true}
	kinds := func(rows []relEntry) map[string]int {
		m := map[string]int{}
		for _, r := range rows {
			m[r.Kind]++
		}
		return m
	}
	if k := kinds(resolveRelated("Engine", defs, edges, nil, nil, files)); k["def"] != 1 || k["re-export"] != 1 {
		t.Errorf("Engine rows kinds = %v, want 1 def + 1 re-export", k)
	}
	if k := kinds(resolveRelated("helper", defs, edges, nil, nil, files)); k["def"] != 1 || k["re-export"] != 1 {
		t.Errorf("helper rows kinds = %v, want 1 def + 1 re-export", k)
	}
	rows := resolveRelated("go", defs, edges, nil, nil, files)
	if len(rows) != 1 || rows[0].Kind != "re-export" || !strings.Contains(rows[0].Detail, "src/engine.rs:2") {
		t.Errorf("go (alias) rows = %+v, want 1 re-export pointing at run's def", rows)
	}
	if rows := resolveRelated("loop", defs, edges, nil, nil, files); len(rows) != 2 {
		t.Errorf("loop (cycle) rows = %+v, want exactly 2 (no infinite loop)", rows)
	}
}

func TestExtractRelatedJSConsts(t *testing.T) {
	text := "const topLevel = 1;\n" +
		"export const exported = 2;\n" +
		"function f() {\n  const local = 3;\n  return local;\n}\n" +
		"const arrow = () => topLevel;\n"
	defs, _, _, _, ok := extractRelated("c.mjs", text)
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
		if _, _, _, _, ok := extractRelated(tc.path, tc.text); ok {
			t.Errorf("extractRelated(%q) = ok, want skipped", tc.path)
		}
	}
	doc := "// Talks to wasm-bindgen bridge\n" + body
	defs, _, _, _, ok := extractRelated("src/bridge.mjs", doc)
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

// --- Batch 4 (go14, P2: JNI bridge, Option 3) ---

func TestParseJNISig(t *testing.T) {
	cases := []struct {
		sig    string
		want   []string
		wantOK bool
	}{
		{"", nil, true},            // zero params
		{"I", []string{"I"}, true}, // one int
		{"__I", nil, false},        // __ is not part of a signature
		{"IJ", []string{"I", "J"}, true},
		{"_3BI", []string{"_3B", "I"}, true},
		{"_3B", []string{"_3B"}, true},
		{"_3_3Ljava_lang_String_2", []string{"_3_3Ljava_lang_String_2"}, true},
		{"ZBCS", []string{"Z", "B", "C", "S"}, true},
		{"_3", nil, false},   // dangling array prefix
		{"Lfoo", nil, false}, // unterminated object
		{"X", nil, false},    // unknown letter
		{"_3X", nil, false},
	}
	for _, c := range cases {
		got, ok := parseJNISig(c.sig)
		if ok != c.wantOK {
			t.Errorf("parseJNISig(%q) ok = %v, want %v", c.sig, ok, c.wantOK)
			continue
		}
		if !c.wantOK {
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("parseJNISig(%q) = %v, want %v", c.sig, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("parseJNISig(%q)[%d] = %q, want %q", c.sig, i, got[i], c.want[i])
			}
		}
	}
}

func TestParseJNIRustName(t *testing.T) {
	cases := []struct {
		name       string
		wantBase   string
		wantTokens []string
		wantOver   bool
		wantOK     bool
	}{
		{"Java_com_example_Bridge_send", "Java_com_example_Bridge_send", nil, false, true},
		{"Java_com_example_Bridge_send__I", "Java_com_example_Bridge_send", []string{"I"}, true, true},
		{"Java_com_example_Bridge_send___3BI", "Java_com_example_Bridge_send", []string{"_3B", "I"}, true, true},
		{"Java_com_example_Bridge_send___3B", "Java_com_example_Bridge_send", []string{"_3B"}, true, true},
		{"Java_com_example_Bridge_send__ZBCS", "Java_com_example_Bridge_send", []string{"Z", "B", "C", "S"}, true, true},
		{"Java_com_example_Bridge_send___3_3Ljava_lang_String_2", "Java_com_example_Bridge_send", []string{"_3_3Ljava_lang_String_2"}, true, true},
		{"Java_com_example_My_1Class_send", "Java_com_example_My_1Class_send", nil, false, true},
		{"Java_com_example_Bridge_send__", "Java_com_example_Bridge_send", nil, true, true}, // empty sig = 0 params
		{"Java_com_example_Bridge_send__X", "", nil, false, false},                          // malformed sig
		{"Java_com_example_Bridge_send___3", "", nil, false, false},                         // dangling _3
		{"plain_fn", "", nil, false, false}, // not a JNI name
		{"", "", nil, false, false},
	}
	for _, c := range cases {
		base, toks, over, ok := parseJNIRustName(c.name)
		if ok != c.wantOK || base != c.wantBase || over != c.wantOver {
			t.Errorf("parseJNIRustName(%q) = (%q, %v, %v, %v), want (%q, %v, %v, %v)",
				c.name, base, toks, over, ok, c.wantBase, c.wantTokens, c.wantOver, c.wantOK)
			continue
		}
		if len(toks) != len(c.wantTokens) {
			t.Errorf("parseJNIRustName(%q) tokens = %v, want %v", c.name, toks, c.wantTokens)
			continue
		}
		for i := range toks {
			if toks[i] != c.wantTokens[i] {
				t.Errorf("parseJNIRustName(%q) tokens[%d] = %q, want %q", c.name, i, toks[i], c.wantTokens[i])
			}
		}
	}
}

func TestJNIEscapeAndKotlinBase(t *testing.T) {
	if got := jniEscape("send_data"); got != "send_1data" {
		t.Errorf("jniEscape(send_data) = %q, want send_1data", got)
	}
	if got := jniEscape("My$Class"); got != "My_00024Class" {
		t.Errorf("jniEscape(My$Class) = %q, want My_00024Class", got)
	}
	if got := jniKotlinBase("com.example", []string{"Bridge"}, "send"); got != "Java_com_example_Bridge_send" {
		t.Errorf("jniKotlinBase class = %q, want Java_com_example_Bridge_send", got)
	}
	if got := jniKotlinBase("com.example", []string{"Outer", "Inner"}, "go"); got != "Java_com_example_Outer_00024Inner_go" {
		t.Errorf("jniKotlinBase nested = %q, want Java_com_example_Outer_00024Inner_go", got)
	}
	if got := jniKotlinBase("", []string{"BridgeKt"}, "main"); got != "Java_BridgeKt_main" {
		t.Errorf("jniKotlinBase no-pkg = %q, want Java_BridgeKt_main", got)
	}
	if got := jniKotlinBase("com.example", []string{"Bridge"}, "send_data"); got != "Java_com_example_Bridge_send_1data" {
		t.Errorf("jniKotlinBase escaped method = %q, want Java_com_example_Bridge_send_1data", got)
	}
}

func TestJniCoarseToken(t *testing.T) {
	cases := []struct {
		typ    string
		vararg bool
		want   string
	}{
		{"Int", false, "I"},
		{"Long", false, "J"},
		{"Boolean", false, "Z"},
		{"Float", false, "F"},
		{"Double", false, "D"},
		{"Byte", false, "B"},
		{"Short", false, "S"},
		{"Char", false, "C"},
		{"String", false, "Ljava_lang_String_2"},
		{"ByteArray", false, "_3B"},
		{"IntArray", false, "_3I"},
		{"Int", true, "_3I"}, // vararg Int = [I
		{"String", true, "_3Ljava_lang_String_2"},
		{"Int?", false, ""}, // nullable: boxed, wildcard
		{"String?", false, ""},
		{"Map<String, Int>", false, ""}, // generic: wildcard
		{"Widget", false, ""},           // custom class: wildcard
		{"(Int) -> Unit", false, ""},    // function type: wildcard
		{"Unit", false, ""},
	}
	for _, c := range cases {
		if got := jniCoarseToken(c.typ, c.vararg); got != c.want {
			t.Errorf("jniCoarseToken(%q, vararg=%v) = %q, want %q", c.typ, c.vararg, got, c.want)
		}
	}
}

// TestExtractRelatedKotlinJNI covers the Kotlin side of the JNI bridge:
// class, named object, companion ($Companion), nested class, and the
// top-level facade <FileStem>Kt — with arity and coarse tokens.
func TestExtractRelatedKotlinJNI(t *testing.T) {
	text := "package com.example\n\n" +
		"class Bridge {\n" +
		"    external fun send(data: ByteArray, count: Int): Boolean\n" +
		"    external fun ping(): Unit\n" +
		"    fun plain() {}\n" +
		"}\n\n" +
		"object Holder {\n" +
		"    external fun store(key: String, value: Long)\n" +
		"}\n\n" +
		"class Outer {\n" +
		"    class Inner {\n" +
		"        external fun go(x: Int)\n" +
		"    }\n" +
		"    companion object {\n" +
		"        external fun create(name: String?): Widget\n" +
		"    }\n" +
		"}\n\n" +
		"external fun topLevel(flag: Boolean)\n"
	_, _, _, jni, ok := extractRelated("Bridge.kt", text)
	if !ok {
		t.Fatal("expected ok for valid Kotlin")
	}
	byName := map[string]relJNIDecl{}
	for _, d := range jni {
		if _, dup := byName[d.Name]; dup {
			t.Errorf("duplicate JNI decl for %q", d.Name)
		}
		byName[d.Name] = d
	}
	want := map[string]relJNIDecl{
		"send":     {Base: "Java_com_example_Bridge_send", Arity: 2, Coarse: []string{"_3B", "I"}},
		"ping":     {Base: "Java_com_example_Bridge_ping", Arity: 0, Coarse: nil},
		"store":    {Base: "Java_com_example_Holder_store", Arity: 2, Coarse: []string{"Ljava_lang_String_2", "J"}},
		"go":       {Base: "Java_com_example_Outer_00024Inner_go", Arity: 1, Coarse: []string{"I"}},
		"create":   {Base: "Java_com_example_Outer_00024Companion_create", Arity: 1, Coarse: []string{""}},
		"topLevel": {Base: "Java_com_example_BridgeKt_topLevel", Arity: 1, Coarse: []string{"Z"}},
	}
	if len(jni) != len(want) {
		t.Fatalf("jni decls = %+v, want %d", jni, len(want))
	}
	for name, w := range want {
		d, found := byName[name]
		if !found {
			t.Errorf("missing JNI decl %q", name)
			continue
		}
		if d.Base != w.Base {
			t.Errorf("decl %q base = %q, want %q", name, d.Base, w.Base)
		}
		if d.Arity != w.Arity {
			t.Errorf("decl %q arity = %d, want %d", name, d.Arity, w.Arity)
		}
		if len(d.Coarse) != len(w.Coarse) {
			t.Errorf("decl %q coarse = %v, want %v", name, d.Coarse, w.Coarse)
			continue
		}
		for i := range w.Coarse {
			if d.Coarse[i] != w.Coarse[i] {
				t.Errorf("decl %q coarse[%d] = %q, want %q", name, i, d.Coarse[i], w.Coarse[i])
			}
		}
	}
	if d := byName["send"]; d.Line != 4 {
		t.Errorf("send decl line = %d, want 4", d.Line)
	}
	if d := byName["create"]; d.Line != 18 {
		t.Errorf("create decl line = %d, want 18", d.Line)
	}
	// A non-external fun must not become a JNI decl.
	if _, found := byName["plain"]; found {
		t.Errorf("plain fun must not be a JNI decl")
	}
}

// TestExtractRelatedRustJNI covers the Rust side: a top-level Java_*
// fn is a JNI bridge (no attribute needed); a Java_* method inside an
// impl is not; a normal fn is not.
func TestExtractRelatedRustJNI(t *testing.T) {
	text := "pub fn Java_com_example_Bridge_send(data: &[u8], count: i32) -> bool { true }\n" +
		"pub fn Java_com_example_Bridge_ping() {}\n" +
		"pub fn plain() {}\n" +
		"impl Widget {\n    pub fn Java_com_example_Widget_draw(&self) {}\n}\n"
	_, _, bridges, _, ok := extractRelated("native.rs", text)
	if !ok {
		t.Fatal("expected ok for valid Rust")
	}
	jni := map[string]relBridge{}
	for _, b := range bridges {
		if b.Kind == "jni" {
			jni[b.RustName] = b
		}
	}
	if len(jni) != 2 {
		t.Fatalf("jni bridges = %+v, want send + ping", bridges)
	}
	if b := jni["Java_com_example_Bridge_send"]; b.RustLine != 1 || b.RustPath != "native.rs" {
		t.Errorf("send bridge wrong: %+v", b)
	}
	if b := jni["Java_com_example_Bridge_ping"]; b.RustLine != 2 {
		t.Errorf("ping bridge wrong: %+v", b)
	}
	for _, b := range bridges {
		if b.Kind == "jni" && b.JSName != "" {
			t.Errorf("jni bridge must not carry a JSName: %+v", b)
		}
	}
}

// TestResolveRelatedJNI checks the two directions of the JNI bridge:
// Kotlin method -> Rust overloads, and Rust JNI name -> Kotlin decls,
// with arity + coarse-token narrowing and fail-open candidates.
func TestResolveRelatedJNI(t *testing.T) {
	defs := []relDef{
		{Path: "Bridge.kt", Line: 4, Name: "send"},
		{Path: "Bridge.kt", Line: 5, Name: "ping"},
		{Path: "native.rs", Line: 1, Name: "Java_com_example_Bridge_send__I"},
		{Path: "native.rs", Line: 2, Name: "Java_com_example_Bridge_send__Ljava_lang_String_2"},
		{Path: "native.rs", Line: 3, Name: "Java_com_example_Bridge_ping"},
	}
	jni := []relJNIDecl{
		{Path: "Bridge.kt", Line: 4, Name: "send", Base: "Java_com_example_Bridge_send", Arity: 1, Coarse: []string{"I"}},
		{Path: "Bridge.kt", Line: 5, Name: "ping", Base: "Java_com_example_Bridge_ping", Arity: 0, Coarse: nil},
	}
	bridges := []relBridge{
		{RustPath: "native.rs", RustLine: 1, RustName: "Java_com_example_Bridge_send__I", Kind: "jni"},
		{RustPath: "native.rs", RustLine: 2, RustName: "Java_com_example_Bridge_send__Ljava_lang_String_2", Kind: "jni"},
		{RustPath: "native.rs", RustLine: 3, RustName: "Java_com_example_Bridge_ping", Kind: "jni"},
	}
	files := map[string]bool{"Bridge.kt": true, "native.rs": true}

	// Kotlin direction: one row per Rust overload for the base.
	rows := resolveRelated("send", defs, nil, bridges, jni, files)
	ffi := filterKind(rows, "ffi")
	if len(ffi) != 2 {
		t.Fatalf("related(send) ffi = %+v, want 2 overload rows", ffi)
	}
	if ffi[0].Path != "native.rs" || ffi[0].Line != 1 || ffi[1].Line != 2 {
		t.Errorf("related(send) ffi rows wrong: %+v", ffi)
	}
	// Rust direction, overloaded query: arity + coarse narrow to one decl.
	rows = resolveRelated("Java_com_example_Bridge_send__I", defs, nil, bridges, jni, files)
	ffi = filterKind(rows, "ffi")
	if len(ffi) != 1 || ffi[0].Path != "Bridge.kt" || ffi[0].Line != 4 {
		t.Errorf("related(send__I) ffi = %+v, want exactly the send decl at Bridge.kt:4", ffi)
	}
	// Rust direction, String overload: the same decl (its coarse is I,
	// not the String token) must NOT match — arity is 1 either way, so
	// the coarse token is what separates them.
	rows = resolveRelated("Java_com_example_Bridge_send__Ljava_lang_String_2", defs, nil, bridges, jni, files)
	ffi = filterKind(rows, "ffi")
	if len(ffi) != 0 {
		t.Errorf("related(send__String) ffi = %+v, want none (no String decl)", ffi)
	}
	// Rust direction, short name: no signature to narrow with, so the
	// decl appears (fail-open).
	rows = resolveRelated("Java_com_example_Bridge_send", defs, nil, bridges, jni, files)
	ffi = filterKind(rows, "ffi")
	if len(ffi) != 1 || ffi[0].Path != "Bridge.kt" || ffi[0].Line != 4 {
		t.Errorf("related(send short) ffi = %+v, want the send decl", ffi)
	}
	// Zero-arity: ping both ways.
	rows = resolveRelated("ping", defs, nil, bridges, jni, files)
	if ffi := filterKind(rows, "ffi"); len(ffi) != 1 || ffi[0].Line != 3 {
		t.Errorf("related(ping) ffi = %+v, want the ping bridge at native.rs:3", ffi)
	}
	rows = resolveRelated("Java_com_example_Bridge_ping", defs, nil, bridges, jni, files)
	if ffi := filterKind(rows, "ffi"); len(ffi) != 1 || ffi[0].Path != "Bridge.kt" || ffi[0].Line != 5 {
		t.Errorf("related(ping rust) ffi = %+v, want the ping decl at Bridge.kt:5", ffi)
	}
	// A wasm bridge must not leak into JNI rows, and a JNI bridge must
	// not leak into wasm rows.
	wasmBridges := append(append([]relBridge{}, bridges...),
		relBridge{RustPath: "lib.rs", RustLine: 9, RustName: "get_data", JSName: "getData", Kind: "wasm"})
	rows = resolveRelated("getData", defs, nil, wasmBridges, jni, files)
	if ffi := filterKind(rows, "ffi"); len(ffi) != 1 || ffi[0].Path != "lib.rs" {
		t.Errorf("related(getData) ffi = %+v, want only the wasm bridge", ffi)
	}
	rows = resolveRelated("send", defs, nil, wasmBridges, jni, files)
	if ffi := filterKind(rows, "ffi"); len(ffi) != 2 {
		t.Errorf("related(send) with wasm bridge present = %+v, want still 2 jni rows", ffi)
	}
}

func filterKind(rows []relEntry, kind string) []relEntry {
	var out []relEntry
	for _, r := range rows {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}

// TestExtractRelatedKotlinJNIVararg locks the harness-found defect
// (2026-11-21): `vararg` lives in a `parameter_modifiers` SIBLING of
// the parameter node, not inside it — a vararg Int param must map to
// the array token _3I, not I.
func TestExtractRelatedKotlinJNIVararg(t *testing.T) {
	text := "package com.example\n\n" +
		"class Bridge {\n" +
		"    external fun sum(vararg xs: Int): Int\n" +
		"    external fun both(a: Int, vararg rest: String)\n" +
		"}\n"
	_, _, _, jni, ok := extractRelated("Bridge.kt", text)
	if !ok {
		t.Fatal("expected ok for valid Kotlin")
	}
	byName := map[string]relJNIDecl{}
	for _, d := range jni {
		byName[d.Name] = d
	}
	if d := byName["sum"]; d.Arity != 1 || len(d.Coarse) != 1 || d.Coarse[0] != "_3I" {
		t.Errorf("sum decl = %+v, want arity 1 coarse [_3I]", d)
	}
	if d := byName["both"]; d.Arity != 2 || len(d.Coarse) != 2 || d.Coarse[0] != "I" || d.Coarse[1] != "_3Ljava_lang_String_2" {
		t.Errorf("both decl = %+v, want arity 2 coarse [I _3Ljava_lang_String_2]", d)
	}
}

// TestExtractRelatedKotlinJNICompanionAndComments (go15) checks:
//   - @JvmStatic inside a companion object binds to the enclosing outer
//     class (Java_com_example_Outer_staticCreate, no $Companion), while a
//     plain companion external fun stays on Outer$Companion.
//   - A named companion object (`companion object Native`) uses its
//     declared name (Outer$Native), and @JvmStatic inside it still binds
//     to the enclosing outer class.
//   - A `package ...` line inside a leading block comment is ignored in
//     favor of the real package declaration.
func TestExtractRelatedKotlinJNICompanionAndComments(t *testing.T) {
	text := "/*\npackage fake.commented\n*/\n" +
		"package com.example\n\n" +
		"class Outer {\n" +
		"    companion object {\n" +
		"        @JvmStatic\n" +
		"        external fun staticCreate(n: Int): Long\n" +
		"        @kotlin.jvm.JvmStatic()\n" +
		"        external fun fqStatic(): Unit\n" +
		"        external fun plainCompanion(): Unit\n" +
		"    }\n" +
		"}\n\n" +
		"class NamedHost {\n" +
		"    companion object Native {\n" +
		"        external fun namedCreate(): Unit\n" +
		"        @JvmStatic\n" +
		"        external fun namedStatic(): Unit\n" +
		"    }\n" +
		"}\n"
	_, _, _, jni, ok := extractRelated("Outer.kt", text)
	if !ok {
		t.Fatal("expected ok for valid Kotlin")
	}
	byName := map[string]relJNIDecl{}
	for _, d := range jni {
		byName[d.Name] = d
	}
	want := map[string]string{
		"staticCreate":   "Java_com_example_Outer_staticCreate",
		"fqStatic":       "Java_com_example_Outer_fqStatic",
		"plainCompanion": "Java_com_example_Outer_00024Companion_plainCompanion",
		"namedCreate":    "Java_com_example_NamedHost_00024Native_namedCreate",
		"namedStatic":    "Java_com_example_NamedHost_namedStatic",
	}
	for name, wantBase := range want {
		d, ok := byName[name]
		if !ok {
			t.Errorf("missing JNI decl %q (got %+v)", name, jni)
			continue
		}
		if d.Base != wantBase {
			t.Errorf("decl %q base = %q, want %q", name, d.Base, wantBase)
		}
	}
}

// TestResolveRelatedJNIPolish (go15) locks the three resolve-side JNI
// fixes: qualified Rust query (`ffi::Java_...`), overloaded rustName
// preserved in Detail when same-arity bridges coexist (and coarse-filtered
// on short queries), and two #[cfg] bridges in the same file on different
// lines both emitted for the Kotlin-side query.
func TestResolveRelatedJNIPolish(t *testing.T) {
	defs := []relDef{
		{Path: "Bridge.kt", Line: 4, Name: "send"},
		{Path: "native.rs", Line: 10, Name: "Java_com_example_Bridge_send__I"},
		{Path: "native.rs", Line: 20, Name: "Java_com_example_Bridge_send__Ljava_lang_String_2"},
		{Path: "native.rs", Line: 30, Name: "Java_com_example_Bridge_ping"},
		{Path: "native.rs", Line: 40, Name: "Java_com_example_Bridge_ping"},
	}
	jni := []relJNIDecl{
		{Path: "Bridge.kt", Line: 4, Name: "send", Base: "Java_com_example_Bridge_send", Arity: 1, Coarse: []string{"I"}},
		{Path: "Bridge.kt", Line: 5, Name: "ping", Base: "Java_com_example_Bridge_ping", Arity: 0, Coarse: nil},
	}
	bridges := []relBridge{
		{RustPath: "native.rs", RustLine: 10, RustName: "Java_com_example_Bridge_send__I", Kind: "jni"},
		{RustPath: "native.rs", RustLine: 20, RustName: "Java_com_example_Bridge_send__Ljava_lang_String_2", Kind: "jni"},
		{RustPath: "native.rs", RustLine: 30, RustName: "Java_com_example_Bridge_ping", Kind: "jni"},
		{RustPath: "native.rs", RustLine: 40, RustName: "Java_com_example_Bridge_ping", Kind: "jni"},
	}
	files := map[string]bool{"Bridge.kt": true, "native.rs": true}

	// Overloaded query keeps its full Rust JNI name in Detail even though
	// another 1-arg overload exists for the same base.
	rows := resolveRelated("Java_com_example_Bridge_send__I", defs, nil, bridges, jni, files)
	ffi := filterKind(rows, "ffi")
	if len(ffi) != 1 || !strings.Contains(ffi[0].Detail, "bound to Java_com_example_Bridge_send__I") {
		t.Errorf("related(send__I) detail = %+v, want bound to Java_com_example_Bridge_send__I", ffi)
	}

	// Short query also narrows bridge by coarse token: only send__I matches I.
	rows = resolveRelated("Java_com_example_Bridge_send", defs, nil, bridges, jni, files)
	ffi = filterKind(rows, "ffi")
	if len(ffi) != 1 || !strings.Contains(ffi[0].Detail, "bound to Java_com_example_Bridge_send__I") {
		t.Errorf("related(send short) detail = %+v, want bound to Java_com_example_Bridge_send__I", ffi)
	}

	// Qualified Rust query (`ffi::Java_...`) still resolves the Kotlin decl.
	rows = resolveRelated("ffi::Java_com_example_Bridge_send__I", defs, nil, bridges, jni, files)
	ffi = filterKind(rows, "ffi")
	if len(ffi) != 1 || ffi[0].Path != "Bridge.kt" || ffi[0].Line != 4 {
		t.Errorf("related(ffi::send__I) ffi = %+v, want Bridge.kt:4", ffi)
	}

	// Two #[cfg] Rust bridges with the same name in one file on different
	// lines must both appear on Kotlin->Rust lookup.
	rows = resolveRelated("ping", defs, nil, bridges, jni, files)
	ffi = filterKind(rows, "ffi")
	if len(ffi) != 2 || ffi[0].Line != 30 || ffi[1].Line != 40 {
		t.Errorf("related(ping) with #[cfg] variants = %+v, want lines 30 and 40", ffi)
	}
}
