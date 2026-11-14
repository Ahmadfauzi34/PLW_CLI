package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestParseQuery(t *testing.T) {
	p := parseQuery("100%")
	if !p.likeMode {
		t.Error("100% should route to LIKE")
	}
	p = parseQuery(`C:\temp`)
	if !p.likeMode {
		t.Error(`C:\temp should route to LIKE`)
	}
	p = parseQuery("hello_world")
	if p.likeMode {
		t.Error("hello_world should stay on BM25 path")
	}
	p = parseQuery("getGraphParametersFlat")
	want := []string{"get", "graph", "parameters", "flat"}
	var gotTerms []string
	for _, r := range p.runs {
		gotTerms = append(gotTerms, r.terms...)
	}
	if strings.Join(gotTerms, ",") != strings.Join(want, ",") {
		t.Errorf("terms = %v, want %v", gotTerms, want)
	}
	if len(p.runs) != 4 { // camel-split inserts spaces -> AND, not phrase
		t.Errorf("camel query should be 4 AND runs, got %+v", p.runs)
	}
	p = parseQuery(`"exact phrase"`)
	if len(p.runs) != 1 || !p.runs[0].phrase ||
		strings.Join(p.runs[0].terms, ",") != "exact,phrase" {
		t.Errorf("phrase parse = %+v", p)
	}
	p = parseQuery("path:foo")
	if !p.pathOnly || len(p.runs) != 1 || p.runs[0].terms[0] != "foo" {
		t.Errorf("path: parse = %+v", p)
	}
	p = parseQuery("deep_target")
	if len(p.runs) != 1 || !p.runs[0].phrase ||
		strings.Join(p.runs[0].terms, ",") != "deep,target" {
		t.Errorf("implicit phrase parse = %+v", p)
	}
	p = parseQuery("lorem ipsum")
	if len(p.runs) != 2 || p.runs[0].phrase || p.runs[1].phrase {
		t.Errorf("whitespace query should be AND runs, got %+v", p)
	}
	p = parseQuery("***")
	if !p.likeMode {
		t.Error("all-separator query should fall back to LIKE")
	}
}

func TestBM25Monotonic(t *testing.T) {
	// higher TF -> higher score; longer doc -> lower score, all else equal
	s1 := bm25Term(2.0, 5, 100, 100)
	s2 := bm25Term(2.0, 1, 100, 100)
	if !(s1 > s2) {
		t.Errorf("TF monotonicity violated: %v <= %v", s1, s2)
	}
	s3 := bm25Term(2.0, 3, 500, 100)
	s4 := bm25Term(2.0, 3, 50, 100)
	if !(s4 > s3) {
		t.Errorf("length norm violated: %v <= %v", s4, s3)
	}
	if idf(100, 90) <= 0 || idf(100, 1) <= idf(100, 90) {
		t.Error("IDF should be positive and decreasing in df")
	}
}

func TestSearchRanking(t *testing.T) {
	ix := newMemIndex("/r")
	add := func(path, text string) {
		for _, c := range chunkLines(text) {
			d := ChunkDoc{Path: path, Line: c.startLine, Text: joinLines(c.lines)}
			ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
		}
	}
	// noise: 5x "facade" in body; real: path match
	add("src/noise.rs", "facade facade facade facade facade\nline2\nline3\n")
	add("src/facade/mod.rs", "// the facade module\npub fn x() {}\n")
	total, scored := ix.searchBody(parseQuery("facade"), nil, 10)
	if total == 0 {
		t.Fatal("no hits")
	}
	top := ix.chunks[scored[0].doc].Path
	if top != "src/facade/mod.rs" {
		t.Errorf("path weight should win: top = %s", top)
	}

	// definition boost: single-mention def should beat many-mention caller
	ix2 := newMemIndex("/r")
	add2 := func(path, text string) {
		for _, c := range chunkLines(text) {
			d := ChunkDoc{Path: path, Line: c.startLine, Text: joinLines(c.lines)}
			ix2.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
		}
	}
	caller := "use thing;\n"
	for i := 0; i < 20; i++ {
		caller += "do_thing(x);\n"
	}
	add2("src/caller.rs", caller)
	add2("src/thing.rs", "pub fn do_thing(x: i32) {}\n")
	_, scored2 := ix2.searchBody(parseQuery("do_thing"), nil, 10)
	if top := ix2.chunks[scored2[0].doc].Path; top != "src/thing.rs" {
		t.Errorf("definition boost should win: top = %s", top)
	}
}

func TestDiversify(t *testing.T) {
	ix := newMemIndex("/r")
	for i := 0; i < 5; i++ {
		d := ChunkDoc{Path: "a.rs", Line: i*30 + 1, Text: "filler filler"}
		ix.addChunkDoc(d, tokenize(d.Text), tokenize(d.Path))
	}
	d := ChunkDoc{Path: "b.rs", Line: 1, Text: "filler"}
	ix.addChunkDoc(d, tokenize(d.Text), tokenize(d.Path))
	_, scored := ix.searchBody(parseQuery("filler"), nil, 50)
	got := diversify(scored, ix, 10, 2, false)
	countA := 0
	for _, s := range got {
		if ix.chunks[s.doc].Path == "a.rs" {
			countA++
		}
	}
	if countA > 2 {
		t.Errorf("per-file cap violated: %d chunks of a.rs", countA)
	}
	// unlimited recall: every matching chunk, pure BM25 scores, still sorted
	gotAll := diversify(scored, ix, -1, 0, false)
	if len(gotAll) != len(scored) {
		t.Errorf("unlimited: got %d chunks, want all %d", len(gotAll), len(scored))
	}
	pre := map[int]float64{}
	for _, s := range scored {
		pre[s.doc] = s.score
	}
	for i, s := range gotAll {
		if s.score != pre[s.doc] {
			t.Errorf("unlimited chunk %d: score %v mutated (want pure BM25 %v)",
				i, s.score, pre[s.doc])
		}
		if i > 0 && gotAll[i-1].score < s.score {
			t.Errorf("unlimited not score-sorted at index %d", i)
		}
	}
}

func TestQueryShapeRouting(t *testing.T) {
	p := parseQuery("foo::Bar")
	if !p.likeMode {
		t.Error("foo::Bar should route to LIKE (exact literal)")
	}
	p = parseQuery("crate::module::Item")
	if !p.likeMode {
		t.Error("crate::module::Item should route to LIKE")
	}
	// single colon is a plain separator -> BM25, not LIKE
	p = parseQuery("foo:bar")
	if p.likeMode {
		t.Error("foo:bar (single colon) should stay on BM25 path")
	}
}

func TestTestFilePenalty(t *testing.T) {
	ix := newMemIndex("/r")
	add := func(path, text string) {
		for _, c := range chunkLines(text) {
			d := ChunkDoc{Path: path, Line: c.startLine, Text: joinLines(c.lines)}
			ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
		}
	}
	// Test file mentions the target 20x; source mentions it once.
	// Without the penalty the test file would win on raw TF.
	body := ""
	for i := 0; i < 20; i++ {
		body += "targetword\n"
	}
	add("src/widget_test.go", body)
	add("src/widget.go", "targetword is here\n")
	_, scored := ix.searchBody(parseQuery("targetword"), nil, 10)
	if len(scored) < 2 {
		t.Fatal("expected hits in both files")
	}
	if top := ix.chunks[scored[0].doc].Path; top != "src/widget.go" {
		t.Errorf("test-file penalty should rank source first: top = %s", top)
	}
	// Explicit path query still finds the test file: path not penalized.
	_, scored2 := ix.searchBody(parseQuery("path:widget_test"), nil, 10)
	if len(scored2) == 0 || ix.chunks[scored2[0].doc].Path != "src/widget_test.go" {
		t.Errorf("path query should still find the test file")
	}
	// isTestOrGenerated unit checks
	for _, p := range []string{"src/foo_test.go", "a/b_test.py", "x/y_test.rs",
		"gen/foo.pb.go", "gen/bar.generated.go", "tests/e2e/x.go"} {
		if !isTestOrGenerated(p) {
			t.Errorf("isTestOrGenerated(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"src/foo.go", "src/widget_test.go.bak", "contest/x.go"} {
		if isTestOrGenerated(p) {
			t.Errorf("isTestOrGenerated(%q) = true, want false", p)
		}
	}
}

func TestSaturationDecay(t *testing.T) {
	ix := newMemIndex("/r")
	for i := 0; i < 3; i++ {
		d := ChunkDoc{Path: "a.rs", Line: i*30 + 1, Text: "filler filler"}
		ix.addChunkDoc(d, tokenize(d.Text), tokenize(d.Path))
	}
	_, scored := ix.searchBody(parseQuery("filler"), nil, 50)
	pre := map[int]float64{}
	for _, s := range scored {
		pre[s.doc] = s.score
	}
	got := diversify(scored, ix, 10, 2, false)
	seen := map[string]int{}
	for _, s := range got {
		p := ix.chunks[s.doc].Path
		seen[p]++
		if seen[p] == 2 {
			if want := pre[s.doc] * satDecay; s.score != want {
				t.Errorf("2nd chunk of %s: score = %v, want decayed %v", p, s.score, want)
			}
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i].score > got[i-1].score {
			t.Errorf("diversify output not score-sorted at index %d", i)
		}
	}
}

func TestCompileDefResMultiWord(t *testing.T) {
	// The x2.0 definition bonus must fire for multi-word identifiers in any
	// separator style (regression: the per-term-only regexes never matched
	// camelCase/snake_case definitions, letting call sites outrank them).
	matchAny := func(res []*regexp.Regexp, text string) bool {
		for _, re := range res {
			if re.MatchString(text) {
				return true
			}
		}
		return false
	}
	cases := []struct {
		query string
		terms []string
		def   string // chunk text that defines the queried name
		call  string // chunk text that merely calls it
	}{
		{"addNumbers", []string{"add", "numbers"},
			"func addNumbers(a int, b int) int {",
			"fmt.Println(addNumbers(1, 2))"},
		{"add_numbers", []string{"add", "numbers"},
			"def add_numbers(a, b):",
			"print(add_numbers(1, 2))"},
		{"getGraphParametersFlat", []string{"get", "graph", "parameters", "flat"},
			"pub fn get_graph_parameters_flat() {",
			"let x = get_graph_parameters_flat();"},
		{"FooBar", []string{"foo", "bar"},
			"class FooBar:",
			"x = FooBar()"},
		{"foo", []string{"foo"},
			"def foo():",
			"foo()"},
	}
	for _, c := range cases {
		res := compileDefRes(c.terms, c.query)
		if !matchAny(res, c.def) {
			t.Errorf("query %q: no def regex matched definition %q", c.query, c.def)
		}
		if matchAny(res, c.call) {
			t.Errorf("query %q: def regex wrongly matched call site %q", c.query, c.call)
		}
	}
}

func TestDefIdentifierPatterns(t *testing.T) {
	// one identifier, any separator style
	res := defIdentifierPatterns(parseQuery("addNumbers"))
	if len(res) != 1 {
		t.Fatalf("addNumbers: got %d patterns, want 1", len(res))
	}
	for _, yes := range []string{
		"func addNumbers(a int) int {",
		"def add_numbers(self, a):",
		"function addNumbers(a) {",
	} {
		if !res[0].MatchString(yes) {
			t.Errorf("pattern should match %q", yes)
		}
	}
	for _, no := range []string{
		"const flat = Array.from(m.getGraphParametersFlat(g));", // sub-term only
		"println(addNumbers(1, 2))",                             // call site
		"// addNumbers is used below",                           // comment
		"func addNumber(a int) int {",                           // different name
	} {
		if res[0].MatchString(no) {
			t.Errorf("pattern should NOT match %q", no)
		}
	}
	// getGraphParametersFlat must not match a chunk defining only "flat"
	res = defIdentifierPatterns(parseQuery("getGraphParametersFlat"))
	if len(res) != 1 {
		t.Fatalf("getGraphParametersFlat: got %d patterns, want 1", len(res))
	}
	if res[0].MatchString("const flat = load();") {
		t.Error("full identifier required: wrongly matched 'const flat'")
	}
	if !res[0].MatchString("pub fn get_graph_parameters_flat(") {
		t.Error("should match snake_case definition")
	}
	// likeMode: trailing name only
	res = defIdentifierPatterns(parseQuery("util::helper"))
	if len(res) != 1 {
		t.Fatalf("util::helper: got %d patterns, want 1", len(res))
	}
	if !res[0].MatchString("pub fn helper() -> i32 {") {
		t.Error("likeMode should look for definitions of the trailing name")
	}
	// multiple identifiers: one pattern each (AND semantics in the filter)
	res = defIdentifierPatterns(parseQuery("foo bar"))
	if len(res) != 2 {
		t.Fatalf("foo bar: got %d patterns, want 2", len(res))
	}
}

// TestDefIdentifierPatternsDotTerminator (G1, 2026-11-14): a dot after
// the queried name is a receiver/qualifier, not a definition terminator.
// `fun AppGraph.folderNames()` defines folderNames, not AppGraph — the
// old terminator [^a-z0-9_] let the dot through, so the extension
// outranked `class AppGraph` in --defs-only. Generics, paths and the
// opening brace must still terminate the name.
func TestDefIdentifierPatternsDotTerminator(t *testing.T) {
	res := defIdentifierPatterns(parseQuery("AppGraph"))
	if len(res) != 1 {
		t.Fatalf("AppGraph: got %d patterns, want 1", len(res))
	}
	for _, no := range []string{
		"internal fun AppGraph.folderNames(): FolderNames = FolderNames(1)",
		"fun AppGraph.folderNames() {",
		"fun AppGraph.create(): AppGraph = AppGraph()",
	} {
		if res[0].MatchString(no) {
			t.Errorf("pattern should NOT match receiver/qualifier %q", no)
		}
	}
	for _, yes := range []string{
		"internal class AppGraph(private val application: Application) {",
		"class AppGraph<T> {",
		"class AppGraph : Base() {",
		"struct AppGraph {",
		"impl AppGraph {",
		"fun AppGraph() {",
	} {
		if !res[0].MatchString(yes) {
			t.Errorf("pattern should match definition %q", yes)
		}
	}
	// The x2.0 definition bonus (compileDefRes) shares the terminator
	// and must reject the receiver form too, without losing the class.
	matchAny := func(res []*regexp.Regexp, text string) bool {
		for _, re := range res {
			if re.MatchString(text) {
				return true
			}
		}
		return false
	}
	bonus := compileDefRes([]string{"app", "graph"}, "AppGraph")
	if matchAny(bonus, "internal fun AppGraph.folderNames(): FolderNames = FolderNames(1)") {
		t.Error("bonus regex should NOT match the receiver form")
	}
	if !matchAny(bonus, "internal class AppGraph(private val application: Application) {") {
		t.Error("bonus regex should match the class definition")
	}
}

func TestFilterDefChunksDotReceiver(t *testing.T) {
	// G1 end to end: the extension chunk must leave --defs-only AppGraph
	// (regex rejects the dot, Name folderNames != AppGraph), while
	// --defs-only folderNames still finds it through the AST Name.
	ix := newMemIndex("/r")
	add := func(path, text, kind, name string, line int) {
		d := ChunkDoc{Path: path, Line: line, Text: text, Kind: kind, Name: name}
		ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
	}
	add("FolderNames.kt", "internal fun AppGraph.folderNames(): FolderNames = FolderNames(1)", "sig", "folderNames", 105)
	add("AppGraph.kt", "internal class AppGraph(private val application: Application) {", "body", "AppGraph", 54)
	_, scored := ix.searchBody(parseQuery("AppGraph"), nil, 50)
	got := filterDefChunks(scored, ix, parseQuery("AppGraph"))
	if len(got) != 1 {
		t.Fatalf("query AppGraph: got %d chunks, want 1 (the class only)", len(got))
	}
	if p := ix.chunks[got[0].doc].Path; p != "AppGraph.kt" {
		t.Errorf("query AppGraph: kept %q, want the class chunk", p)
	}
	_, scored = ix.searchBody(parseQuery("folderNames"), nil, 50)
	if got := filterDefChunks(scored, ix, parseQuery("folderNames")); len(got) != 1 {
		t.Errorf("query folderNames: got %d chunks, want 1 (the extension via Name)", len(got))
	}
}

func TestFilterDefChunks(t *testing.T) {
	ix := newMemIndex("/r")
	add := func(path, text string, line int) {
		d := ChunkDoc{Path: path, Line: line, Text: text}
		ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
	}
	add("a.go", "func addNumbers(a int) int {\n\treturn a\n}", 1)
	add("a.go", "func main() {\n\tprintln(addNumbers(1))\n}", 10)
	add("b.py", "def add_numbers(a):\n\treturn a", 1)
	_, scored := ix.searchBody(parseQuery("addNumbers"), nil, 50)
	got := filterDefChunks(scored, ix, parseQuery("addNumbers"))
	if len(got) != 2 {
		t.Fatalf("filterDefChunks: got %d chunks, want 2 defs", len(got))
	}
	for _, s := range got {
		if p := ix.chunks[s.doc].Path; p == "a.go" && ix.chunks[s.doc].Line == 10 {
			t.Error("call-site chunk leaked through --defs-only filter")
		}
	}
}

func TestFilterDefChunksTrustsASTName(t *testing.T) {
	// Regression 2026-11-13 (blind spot defs-only): TS/JS methods carry
	// no definition keyword, so the regex alone drops them even when the
	// AST classified the chunk as a definition. The filter trusts the
	// AST-recorded name (sig chunk or def body chunk) as an OR.
	ix := newMemIndex("/r")
	add := func(path, text, kind, name string, line int) {
		d := ChunkDoc{Path: path, Line: line, Text: text, Kind: kind, Name: name}
		ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
	}
	// signature chunk of a TS method (no keyword before the name)
	add("a.ts", "rebuildCSR(): void {", "sig", "rebuildCSR", 438)
	// one-line method: no sig chunk exists, only a def body chunk
	add("a.ts", "methodOne(): void {}", "body", "methodOne", 6)
	// interface member: no body at all, only a def body chunk
	add("a.ts", "ifaceMethod(): void;", "body", "ifaceMethod", 2)
	// trap: sig chunk of foo whose signature mentions bar as a parameter
	add("a.go", "func foo(bar string) {", "sig", "foo", 1)
	// call site in a window chunk: no recorded name
	add("a.ts", "this.rebuildCSR();", "window", "", 10)
	// separator-insensitive name match via Name only (no keyword in text)
	add("c.ts", "add_numbers(): void {", "sig", "add_numbers", 3)

	// TS method via sig chunk; the call-site window must be dropped.
	_, scored := ix.searchBody(parseQuery("rebuildCSR"), nil, 50)
	if got := filterDefChunks(scored, ix, parseQuery("rebuildCSR")); len(got) != 1 {
		t.Errorf("rebuildCSR via sig chunk: got %d chunks, want 1", len(got))
	}
	// one-line method via def body chunk
	_, scored = ix.searchBody(parseQuery("methodOne"), nil, 50)
	if got := filterDefChunks(scored, ix, parseQuery("methodOne")); len(got) != 1 {
		t.Errorf("methodOne via body chunk: got %d chunks, want 1", len(got))
	}
	// interface member via def body chunk
	_, scored = ix.searchBody(parseQuery("ifaceMethod"), nil, 50)
	if got := filterDefChunks(scored, ix, parseQuery("ifaceMethod")); len(got) != 1 {
		t.Errorf("ifaceMethod via body chunk: got %d chunks, want 1", len(got))
	}
	// separator-insensitive Name match (addNumbers == add_numbers)
	_, scored = ix.searchBody(parseQuery("addNumbers"), nil, 50)
	if got := filterDefChunks(scored, ix, parseQuery("addNumbers")); len(got) != 1 {
		t.Errorf("addNumbers via Name: got %d chunks, want 1", len(got))
	}
	// parameter name must NOT claim the enclosing definition
	_, scored = ix.searchBody(parseQuery("bar"), nil, 50)
	if got := filterDefChunks(scored, ix, parseQuery("bar")); len(got) != 0 {
		t.Errorf("query bar: got %d chunks, want 0 (foo's sig must not leak)", len(got))
	}
	// AND semantics across identifiers survive the OR: foo passes, bar fails.
	_, scored = ix.searchBody(parseQuery("foo bar"), nil, 50)
	if got := filterDefChunks(scored, ix, parseQuery("foo bar")); len(got) != 0 {
		t.Errorf("query foo bar: got %d chunks, want 0", len(got))
	}
}

func TestChunkingStats(t *testing.T) {
	ix := newMemIndex("/r")
	add := func(path, text string, ast bool) {
		d := ChunkDoc{Path: path, Line: 1, Text: text}
		ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
		ix.files[path] = FileMeta{Mtime: 1, Size: int64(len(text)), AST: ast}
	}
	add("a.go", "func foo() {}", true)   // AST
	add("b.py", "def bar(): pass", true) // AST
	add("c.js", "function baz() {}", false)
	// binary/skipped file: in manifest, no chunks -> excluded from both
	ix.files["d.bin"] = FileMeta{Mtime: 1, Size: 10, AST: false}
	// dead chunk: file removed logically -> not counted
	dd := ChunkDoc{Path: "e.go", Line: 1, Text: "func old() {}", Dead: true}
	ix.chunks = append(ix.chunks, dd)
	ix.files["e.go"] = FileMeta{Mtime: 1, Size: 10, AST: true}
	astF, fbF := ix.chunkingStats()
	if astF != 2 || fbF != 1 {
		t.Errorf("chunkingStats = (%d, %d), want (2, 1)", astF, fbF)
	}
}

func TestExcludedDocs(t *testing.T) {
	ix := newMemIndex("/r")
	add := func(path, text string) {
		d := ChunkDoc{Path: path, Line: 1, Text: text}
		ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
	}
	add("a.go", "func helper() int {\n\treturn 1\n}")
	add("a_test.go", "func TestHelper() {\n\thelper()\n}")
	add("b.py", "def helper():\n    return 2  # test it")
	// single value, single token
	excl := ix.excludedDocs([][]string{{"test"}})
	if len(excl) != 2 {
		t.Errorf("exclude [test]: got %d docs, want 2 (a_test.go, b.py)", len(excl))
	}
	// AND within a value: helper+test
	excl = ix.excludedDocs([][]string{{"helper", "test"}})
	if len(excl) != 2 {
		t.Errorf("exclude [helper,test]: got %d docs, want 2", len(excl))
	}
	// camelCase value tokenizes: helperTest -> [helper test]
	excl = ix.excludedDocs([][]string{tokenize(splitCamel("helperTest"))})
	if len(excl) != 2 {
		t.Errorf("exclude helperTest: got %d docs, want 2", len(excl))
	}
	// OR across values
	excl = ix.excludedDocs([][]string{{"nomatch"}, {"test"}})
	if len(excl) != 2 {
		t.Errorf("exclude [nomatch],[test]: got %d docs, want 2", len(excl))
	}
	// empty exclusion matches nothing
	if excl := ix.excludedDocs(nil); len(excl) != 0 {
		t.Errorf("nil exclusion: got %d docs, want 0", len(excl))
	}
}

func TestSearchBodyExclusion(t *testing.T) {
	ix := newMemIndex("/r")
	add := func(path, text string) {
		d := ChunkDoc{Path: path, Line: 1, Text: text}
		ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
	}
	add("a.go", "func helper() int {\n\treturn 1\n}")
	add("a_test.go", "func TestHelper() {\n\thelper()\n}")
	total, scored := ix.searchBody(parseQuery("helper"), [][]string{{"test"}}, 50)
	if total != 1 || len(scored) != 1 {
		t.Errorf("searchBody with exclusion: total=%d scored=%d, want 1,1",
			total, len(scored))
	}
	if p := ix.chunks[scored[0].doc].Path; p != "a.go" {
		t.Errorf("surviving chunk = %s, want a.go", p)
	}
	// likeSearch honors exclusion too
	total, _ = ix.likeSearch("helper()", [][]string{{"test"}}, 50)
	if total != 1 {
		t.Errorf("likeSearch with exclusion: total=%d, want 1", total)
	}
}

func TestDefRegexGoReceiver(t *testing.T) {
	// Go method definitions with receivers must match (regression found
	// by dogfooding: --defs-only "searchBody" missed
	// "func (ix *memIndex) searchBody").
	matchAny := func(res []*regexp.Regexp, text string) bool {
		for _, re := range res {
			if re.MatchString(text) {
				return true
			}
		}
		return false
	}
	cases := []struct{ query, text string }{
		{"searchBody", "func (ix *memIndex) searchBody(p queryPlan) {"},
		{"searchBody", "func searchBody(p queryPlan) {"}, // no receiver still works
		{"Start", "func (s *Server) Start() {"},
	}
	for _, c := range cases {
		for _, res := range [][]*regexp.Regexp{
			compileDefRes([]string{c.query}, c.query),
			defIdentifierPatterns(parseQuery(c.query)),
		} {
			if !matchAny(res, c.text) {
				t.Errorf("query %q: def regex should match %q", c.query, c.text)
			}
		}
	}
	res := defIdentifierPatterns(parseQuery("searchBody"))
	if matchAny(res, "x := searchBody(nil)") {
		t.Error("def regex should NOT match a call site")
	}
}

func TestDefPatternPackageNotDefinition(t *testing.T) {
	// Regression 2026-10-26: "package main" must not count as a
	// definition of "main" (neither the ranking bonus regex nor the
	// --defs-only filter). A package declaration names a module.
	res := defIdentifierPatterns(parseQuery("main"))
	if len(res) != 1 {
		t.Fatalf("main: got %d patterns, want 1", len(res))
	}
	if res[0].MatchString("package main\n") {
		t.Error("def pattern should NOT match 'package main'")
	}
	if !res[0].MatchString("func main() {") {
		t.Error("def pattern should still match 'func main() {'")
	}
}

func TestDiversifyLimitZero(t *testing.T) {
	// Regression 2026-10-26: --limit 0 must show nothing, not 1 hit.
	ix := newMemIndex("root")
	ix.chunks = []ChunkDoc{
		{Path: "a.go", Line: 1, Text: "func foo() {}"},
		{Path: "b.go", Line: 1, Text: "func bar() {}"},
	}
	sc := []scoredChunk{{0, 9.0}, {1, 5.0}}
	if got := diversify(sc, ix, 0, 2, false); len(got) != 0 {
		t.Errorf("diversify limit 0: got %d chunks, want 0", len(got))
	}
	// limit 1 and 2 keep their exact behaviour (no off-by-one shift).
	if got := diversify(sc, ix, 1, 2, false); len(got) != 1 {
		t.Errorf("diversify limit 1: got %d chunks, want 1", len(got))
	}
	if got := diversify(sc, ix, 2, 2, false); len(got) != 2 {
		t.Errorf("diversify limit 2: got %d chunks, want 2", len(got))
	}
}

func TestDedupContained(t *testing.T) {
	ix := newMemIndex("root")
	ix.chunks = []ChunkDoc{
		{Path: "lib.py", Line: 1, Text: "class C:\n    a = 1\n    def inc(self):\n        pass\n    b = 2\n    c = 3\n    d = 4\n    e = 5\n    f = 6\n    g = 7"},
		{Path: "lib.py", Line: 3, Text: "def inc(self):\n        pass"},
		{Path: "other.py", Line: 1, Text: "x = 1\ny = 2"},
	}
	// Higher score wins regardless of nesting direction: class wins.
	sc := []scoredChunk{{0, 9.0}, {1, 5.0}, {2, 1.0}}
	got := dedupContained(sc, ix)
	if len(got) != 2 || got[0].doc != 0 || got[1].doc != 2 {
		t.Errorf("dedup = %v, want docs [0 2] (higher score wins)", got)
	}
	// Method wins when it scores higher.
	sc = []scoredChunk{{0, 5.0}, {1, 9.0}, {2, 1.0}}
	got = dedupContained(sc, ix)
	if len(got) != 2 || got[0].doc != 1 || got[1].doc != 2 {
		t.Errorf("dedup = %v, want docs [1 2] (higher score wins)", got)
	}
	// Tie: the smaller (more specific) chunk wins.
	sc = []scoredChunk{{0, 5.0}, {1, 5.0}, {2, 1.0}}
	got = dedupContained(sc, ix)
	if len(got) != 2 || got[0].doc != 1 || got[1].doc != 2 {
		t.Errorf("dedup = %v, want docs [1 2] (tie goes to smaller)", got)
	}
	// Overlapping but non-containing chunks are kept.
	ix.chunks = []ChunkDoc{
		{Path: "a.py", Line: 1, Text: "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10"},
		{Path: "a.py", Line: 5, Text: "l5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14"},
	}
	sc = []scoredChunk{{0, 9.0}, {1, 5.0}}
	if got := dedupContained(sc, ix); len(got) != 2 {
		t.Errorf("overlap without containment must be kept, got %v", got)
	}
	// Identical ranges are kept (exact dupes are removed at chunk time).
	ix.chunks = []ChunkDoc{
		{Path: "a.py", Line: 1, Text: "l1\nl2"},
		{Path: "a.py", Line: 1, Text: "l1\nl2"},
	}
	sc = []scoredChunk{{0, 9.0}, {1, 5.0}}
	if got := dedupContained(sc, ix); len(got) != 2 {
		t.Errorf("identical ranges must be kept, got %v", got)
	}
}

func TestDiversifyNestedDedupRegime(t *testing.T) {
	ix := newMemIndex("root")
	ix.chunks = []ChunkDoc{
		{Path: "lib.py", Line: 1, Text: "class C:\n    def inc(self):\n        pass"},
		{Path: "lib.py", Line: 2, Text: "def inc(self):\n        pass"},
	}
	sc := []scoredChunk{{0, 9.0}, {1, 5.0}}
	// Capped regime: higher score wins (class here).
	got := diversify(sc, ix, 5, 2, false)
	if len(got) != 1 || got[0].doc != 0 {
		t.Errorf("capped diversify = %v, want [doc 0]", got)
	}
	// Unlimited regime (--all): both kept.
	got = diversify(sc, ix, -1, 0, false)
	if len(got) != 2 {
		t.Errorf("unlimited diversify = %v, want both", got)
	}
}

func TestDiversifyFull(t *testing.T) {
	ix := newMemIndex("root")
	ix.chunks = []ChunkDoc{
		{Path: "add.go", Line: 1, Kind: "sig", Text: "// Add adds.\nfunc Add(a, b int) int {"},
		{Path: "add.go", Line: 1, Kind: "body", Text: "// Add adds.\nfunc Add(a, b int) int {\n\treturn a + b\n}"},
	}
	// Default: signature wins on score (dense, short).
	sc := []scoredChunk{{0, 9.0}, {1, 5.0}}
	got := diversify(sc, ix, 5, 2, false)
	if len(got) != 1 || got[0].doc != 0 {
		t.Errorf("default diversify = %v, want [sig doc 0]", got)
	}
	// --full: signature dropped, body kept.
	got = diversify(sc, ix, 5, 2, true)
	if len(got) != 1 || got[0].doc != 1 {
		t.Errorf("--full diversify = %v, want [body doc 1]", got)
	}
	// --full with no body present: signature kept, never lost.
	sc = []scoredChunk{{0, 9.0}}
	got = diversify(sc, ix, 5, 2, true)
	if len(got) != 1 || got[0].doc != 0 {
		t.Errorf("--full without body = %v, want [sig doc 0]", got)
	}
}
