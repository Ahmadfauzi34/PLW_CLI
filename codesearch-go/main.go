// Command codesearch: BM25-ranked code search (context-hygiene grep).
//
// Go implementation (hand-rolled BM25) over a
// persistent gob index (no SQLite). Single binary, no cgo, cross-compiles
// cleanly (GOOS=android GOARCH=arm64 for Termux). Function-aware chunking
// uses tree-sitter via gotreesitter (pure Go, no cgo).
//
// Ranking model:
//   - Documents are function-aware chunks: one chunk per function/method/
//     class definition for Go, Rust, Python, JavaScript, TypeScript
//     (tree-sitter, pure Go, no cgo);
//     lines outside any definition are covered by fixed windows, and long
//     definitions (>80 lines) are split with overlap. Other languages and
//     parse failures fall back to overlapping 30-line windows (stride 15).
//   - camelCase is split symmetrically at index and query time.
//   - Path field is BM25-weighted 10x.
//   - At most 2 chunks per file shown (diversification); the 2nd chunk
//     from one file gets a 0.5 saturation decay.
//   - Baked-in boosts: exact-literal x1.5, definition-pattern x2.0.
//   - Test/generated files (*_test.*, *.pb.go, ...): body TF penalized
//     1/5 (Zoekt-style); path matches are NOT penalized.
//   - Queries containing %, \, or :: go straight to literal substring.
//   - Output headers carry anchor ranges (path:start-end) and BM25 scores.
//
// Query language: terms are AND-ed; "phrase" for phrases; path:foo
// restricts to the path field; foo::Bar routes to exact literal.
// (FTS5's OR/NOT/NEAR operators are NOT supported; everything is AND
// of terms.)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// buildCommit and buildDate are set at build time via -ldflags
// "-X main.buildCommit=... -X main.buildDate=...". They default to
// "unknown" so a build without ldflags reports honestly instead of a
// stale value: the only way to tell two same-named binaries apart is
// what --version prints, and a stale value would be worse than none.
var (
	buildCommit = "unknown"
	buildDate   = "unknown"
)

func usage() string {
	return `usage: codesearch [--root DIR] [--limit N] [--snip N] [--paths-only]
                  [--ext rs,py] [--exclude GLOBS] [--exclude-term T,..]
                  [--no-gitignore] [--once] [--json] [--plain]
                  [--per-file-cap N] [--all] [--defs-only] [--full]
                  [--callers NAME] [--related NAME] [--version]
                  [--cache-dir DIR] [--index-status | --index-warm | --index-clean]
                  query [query ...]

--version prints the index schema and build identity and exits; it needs
no root or query.

BM25-ranked code search (context-hygiene grep)

Human output shows a --snip token window (default 120) per hit with the
matched terms wrapped in >>> <<<. --plain prints the same window as clean
code without highlight markers. (--json always emits plain snippets.)

Recall control: at most 2 chunks per file and --limit total are shown by
default (context hygiene). --limit must be >= 0 (0 shows nothing);
negative limits are rejected. --per-file-cap N raises the per-file cap
(0 = unlimited); --all returns every matching chunk with pure BM25 scores
(no per-file cap, no total cap, no saturation decay) — for "find all
callers" style agent work. --all is mutually exclusive with --limit and
--per-file-cap. A single empty or whitespace-only query is rejected
(exit 2) instead of matching the whole index; in batch mode blank
queries are skipped.

--defs-only answers "where is X defined": only chunks whose text defines
one of the query's terms are returned (same definition-pattern as the x2.0
ranking bonus, multi-word aware). For literal queries (a%b, foo::Bar) the
defined name is the query's last segment. A package declaration
("package main") is not a definition of its name, so it never counts
as a --defs-only hit.

Chunks have kinds: a definition yields a signature chunk (doc comment +
declaration header, tagged [sig]) and a body chunk (the full definition);
everything else is a window chunk. By default a name query shows the
signature (dense, short) while a body-term query shows the body, because
nested matches resolve to the best score. --full shows full bodies
instead of signatures: signature chunks whose body is also matched are
dropped. JSON hits carry "kind": "sig", "body", or "window".
Definition hits also carry related rows (K15): the index keeps a symbol
map (definitions, re-export edges, wasm_bindgen bridges), and a hit on a
definition prints its re-exporters and FFI bridge as "related:" lines
under the snippet, and as "related" in JSON hits.

--callers NAME answers "who calls NAME": every call expression whose
callee's trailing name matches NAME's trailing name, as
path:line: caller: line-text (caller is "-" at top level). Structural,
not textual: it uses the tree-sitter AST, so pkg.Foo() matches a query
for Foo and the enclosing function is reported. Languages: Go, Rust,
Python, JavaScript (.js/.mjs/.cjs), TypeScript (.ts/.mts/.cts/.tsx),
Kotlin (.kt).
Name-based, not type-resolved. Takes no positional query; incompatible with --defs-only,
--all, --limit, --per-file-cap and --paths-only. The scan is always
fresh (no BM25 index); --ext, --exclude and --no-gitignore apply, and
--json selects {"query":...,"total":N,"calls":[...]} output.
Comma-separated names (--callers Foo,Bar) answer all of them in one
scan; JSON then adds "batch":true and each call row still carries its
callee, so the batch is the union of the individual runs.

--related NAME answers "where does NAME really live": local definitions,
JS/TS re-export chains (export ... from, export * from, local export
{..} lists), the Rust<->JS wasm_bindgen bridge (top-level
#[wasm_bindgen] functions; the JS name follows bindgen's default case
conversion unless js_name overrides it), and Kotlin (.kt) local
definitions (classes, functions, objects, type aliases) plus
top-level properties. Output rows are
path:line: kind: detail with kind def, re-export, or ffi. The generated
.d.ts is not parsed (often absent); the Rust source is the bridge's
source of truth. Same mode rules as --callers: no positional query,
incompatible with --defs-only, --all, --limit, --per-file-cap and
--paths-only, fresh scan, --json selects
{"query":...,"total":N,"related":[...]} output. Comma-separated names
(--related Foo,Bar) answer all of them in one scan; JSON then adds
"batch":true and each row carries its "symbol", so the batch is the
union of the individual runs.

Index state (G9): the BM25 gob index lives under a cache base dir —
--cache-dir DIR, else $CODESEARCH_CACHE_DIR, else $XDG_CACHE_HOME /
codesearch-go, else $HOME/.cache/codesearch-go — one file per (root,
scan config), named by a hash of both. --index-status reports it
(cache_dir, index_path, exists, schema, files, chunks, size_bytes,
newest, and a stale count of changed/added/removed files, probed
read-only without touching the index). --index-warm builds or syncs
and saves it, then reports the same shape. --index-clean deletes
that root's index file. All three take no query and no other mode,
and accept --json for the machine-readable shape.

--exclude-term t1,t2 removes chunks containing the term(s): each
comma-separated value is tokenized like query text (camelCase-aware); a
chunk is excluded when it contains all tokens of any value. Applied at
candidacy, so it composes with --limit, --all and --defs-only.

Multiple positional queries run in batch: the index is built once and each
query is answered from it. With --json, stdout is
{"queries":[{"query":...,"total":N,"shown":M,"hits":[...]}]}; in human mode
each query's block is preceded by "=== query: <q> ===". Blank queries are
skipped in batch mode.

query language: terms are AND-ed; "phrase" for an exact phrase;
  path:foo restricts to the path field; foo::Bar, a%b and C:\x route to
  literal substring search

--json prints machine-readable output to stdout: a JSON envelope
  {"total":N,"shown":M,"hits":[{"path":...,"start":N,"end":N,
  "score":N|null,"snippet":...,"kind":"sig"|"body"|"window"}]} (score is null for literal search);
  the human-readable summary goes to stderr so stdout stays pure JSON`
}

func parseArgs(args []string) (options, error) {
	o := options{root: ".", limit: 5, snip: 120, perFileCap: 2}
	i := 0
	takeVal := func(name string) (string, error) {
		if i+1 >= len(args) {
			return "", fmt.Errorf("flag %s requires a value", name)
		}
		i++
		return args[i], nil
	}
	for ; i < len(args); i++ {
		a := args[i]
		name, val, hasVal := a, "", false
		if strings.HasPrefix(a, "--") {
			if eq := strings.Index(a, "="); eq >= 0 {
				name, val, hasVal = a[:eq], a[eq+1:], true
			}
		}
		getVal := func() (string, error) {
			if hasVal {
				return val, nil
			}
			return takeVal(name)
		}
		switch name {
		case "--root":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			o.root = v
		case "--limit":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return o, fmt.Errorf("invalid --limit %q", v)
			}
			o.limit = n
			o.limitSet = true
		case "--per-file-cap":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return o, fmt.Errorf("invalid --per-file-cap %q", v)
			}
			o.perFileCap = n
			o.perFileCapSet = true
		case "--all":
			o.all = true
		case "--defs-only":
			o.defsOnly = true
		case "--full":
			o.full = true
		case "--callers":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			o.callers = v
			o.callersSet = true
		case "--related":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			o.related = v
			o.relatedSet = true
		case "--plain":
			o.plain = true
		case "--snip":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return o, fmt.Errorf("invalid --snip %q", v)
			}
			o.snip = n
		case "--paths-only":
			o.pathsOnly = true
		case "--json":
			o.jsonOut = true
		case "--ext":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			o.ext = v
		case "--exclude":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			o.exclude = v
		case "--exclude-term":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			// Comma-separated like --exclude; each value is tokenized like
			// query text (camelCase-aware). A chunk is excluded when it
			// contains all tokens of any value (AND within, OR across).
			for _, part := range strings.Split(v, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				if toks := tokenize(splitCamel(part)); len(toks) > 0 {
					o.excludeTerms = append(o.excludeTerms, toks)
				}
			}
		case "--no-gitignore":
			o.noGitignore = true
		case "--once":
			o.once = true
		case "--version":
			o.version = true
		case "--cache-dir":
			v, err := getVal()
			if err != nil {
				return o, err
			}
			o.cacheDir = v
		case "--index-status":
			o.indexStatus = true
		case "--index-warm":
			o.indexWarm = true
		case "--index-clean":
			o.indexClean = true
		case "-h", "--help", "-help", "help":
			o.help = true
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				return o, fmt.Errorf("unknown flag %q", a)
			}
			// multiple positional queries = batch mode (one index build)
			o.queries = append(o.queries, a)
		}
	}
	return o, nil
}

type hit struct {
	path    string
	line    int // 1-based start line of the chunk
	end     int // 1-based end line of the chunk (anchor range)
	score   float64
	snip    string
	kind    string
	related []relEntry // re-exporters + FFI bridge for def chunks
}

// makeHits converts shown chunks to display hits, attaching related
// rows (K15) to definition chunks via the index's symbol maps.
func makeHits(shown []scoredChunk, ix *memIndex, plan queryPlan, o options) []hit {
	var hits []hit
	for _, s := range shown {
		d := ix.chunks[s.doc]
		sn := o.humanSnippet(d.Text, plan)
		end := d.Line + strings.Count(d.Text, "\n")
		h := hit{path: d.Path, line: d.Line, end: end, score: s.score, snip: sn, kind: d.Kind}
		if d.Name != "" {
			h.related = ix.relatedFor(d.Name, d.Path, d.Line)
		}
		hits = append(hits, h)
	}
	return hits
}

// printHits renders hits in human form, with related rows (when any)
// indented under the snippet they belong to.
func printHits(hits []hit, plan queryPlan) {
	for _, h := range hits {
		tag := ""
		if h.kind == "sig" {
			tag = " [sig]"
		}
		if plan.likeMode {
			fmt.Printf("--- %s:%d-%d%s ---\n", h.path, h.line, h.end, tag)
		} else {
			fmt.Printf("--- %s:%d-%d%s (score %.2f) ---\n", h.path, h.line, h.end, tag, h.score)
		}
		fmt.Println(h.snip)
		for _, r := range h.related {
			fmt.Printf("  related: %s:%d: %s: %s\n", r.Path, r.Line, r.Kind, r.Detail)
		}
		fmt.Println()
	}
}

// isEmptyQuery reports whether a query carries no searchable content:
// empty, whitespace-only, or an empty quoted phrase ("" / "   ").
// Anything else — including punctuation-only strings like "---", which
// are meaningful literal searches — is not empty.
func isEmptyQuery(q string) bool {
	t := strings.TrimSpace(q)
	if t == "" {
		return true
	}
	if len(t) >= 2 && t[0] == '"' && t[len(t)-1] == '"' {
		return strings.TrimSpace(t[1:len(t)-1]) == ""
	}
	return false
}

// formatProof renders the shared proof-line accounting: content files
// ("files", the searchable set), chunks, AST/fallback mix, skip buckets,
// and manifest total. One definition across --once, rebuild, and cached
// lines; content + skipped == manifest by construction (see
// skipBreakdown). "Skipped" covers read-level skips only (oversize,
// binary, overchars) — walk-level filtering (extensions, hidden dirs,
// gitignore, excludes) is not counted here.
func formatProof(ix *memIndex, content, oversize, binary, overchars, manifest int) string {
	astF, fbF := ix.chunkingStats()
	return fmt.Sprintf("%d files, %d chunks; chunking: AST %d files, fallback %d files; skipped %d (oversize %d, binary %d, overchars %d); manifest %d files",
		content, ix.live, astF, fbF, oversize+binary+overchars, oversize, binary, overchars, manifest)
}

// versionLine is the single --version output line. Schema comes from the
// schemaV constant (always true); commit/built come from ldflags and stay
// "unknown" when the binary was built without them.
func versionLine() string {
	return fmt.Sprintf("codesearch schema=%s commit=%s built=%s go=%s", schemaV, buildCommit, buildDate, runtime.Version())
}

func run(args []string) int {
	o, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "codesearch:", err)
		return 2
	}
	if o.help {
		fmt.Println(usage())
		return 0
	}
	if o.version {
		// No root or query needed: version is build identity, not a search.
		fmt.Println(versionLine())
		return 0
	}
	// Index commands (G9) are their own modes: they take no query and
	// must be dispatched before checkQueryAndFlags (which would reject
	// the missing query). Exactly one may be set.
	if o.indexStatus || o.indexWarm || o.indexClean {
		modes := 0
		mode := ""
		if o.indexStatus {
			modes++
			mode = "index-status"
		}
		if o.indexWarm {
			modes++
			mode = "index-warm"
		}
		if o.indexClean {
			modes++
			mode = "index-clean"
		}
		if modes > 1 {
			fmt.Fprintln(os.Stderr, "codesearch: --index-status, --index-warm and --index-clean are mutually exclusive")
			return 2
		}
		if err := checkIndexFlags(o, mode); err != nil {
			fmt.Fprintln(os.Stderr, "codesearch:", err)
			return 2
		}
		root, cfg, err := resolveRootAndConfig(o)
		if err != nil {
			fmt.Fprintln(os.Stderr, "codesearch:", err)
			return 2
		}
		switch mode {
		case "index-status":
			return runIndexStatus(o, root, cfg)
		case "index-warm":
			return runIndexWarm(o, root, cfg)
		default:
			return runIndexClean(o, root, cfg)
		}
	}
	if err := checkQueryAndFlags(o); err != nil {
		fmt.Fprintln(os.Stderr, "codesearch:", err)
		return 2
	}
	root, cfg, err := resolveRootAndConfig(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "codesearch:", err)
		return 2
	}

	// --callers / --related are their own modes: a fresh AST scan, no
	// BM25 index. Same rules for both; only the mode name in the
	// error changes.
	if o.callersSet && o.relatedSet {
		fmt.Fprintln(os.Stderr, "codesearch: --related and --callers are mutually exclusive")
		return 2
	}
	if o.callersSet {
		if err := checkModeFlags(o, "callers"); err != nil {
			fmt.Fprintln(os.Stderr, "codesearch:", err)
			return 2
		}
		return runCallers(o, root, cfg)
	}
	if o.relatedSet {
		if err := checkModeFlags(o, "related"); err != nil {
			fmt.Fprintln(os.Stderr, "codesearch:", err)
			return 2
		}
		return runRelated(o, root, cfg)
	}

	ix, err := loadIndex(o, root, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "codesearch:", err)
		return 1
	}
	return renderResults(o, ix)
}

// checkQueryAndFlags validates the search-mode options, in the order
// run() used to check them inline: query presence, empty single query,
// then flag conflicts. Mode flags (--callers/--related) are NOT checked
// here — they are validated after the root is resolved (see
// checkModeFlags), preserving the original error precedence.
func checkQueryAndFlags(o options) error {
	if len(o.queries) == 0 && !o.callersSet && !o.relatedSet {
		return fmt.Errorf("query is required")
	}
	// A single empty query used to fall through to literal search, where
	// the empty substring matches every chunk — a plausible-looking
	// result set for an input that asked for nothing. Fail hard instead,
	// before any index build. Batch mode keeps its documented behaviour:
	// blank queries are skipped there, not rejected (see runBatch).
	if len(o.queries) == 1 && isEmptyQuery(o.queries[0]) {
		return fmt.Errorf("query must not be empty")
	}
	if o.jsonOut && o.pathsOnly {
		return fmt.Errorf("--json and --paths-only are mutually exclusive")
	}
	if o.all && (o.limitSet || o.perFileCapSet) {
		return fmt.Errorf("--all is mutually exclusive with --limit and --per-file-cap")
	}
	return nil
}

// resolveRootAndConfig turns --root into an absolute directory and the
// scan config for it. An error here is a usage error (exit 2 in run),
// not an index failure.
func resolveRootAndConfig(o options) (string, *ScanConfig, error) {
	root, err := filepath.Abs(o.root)
	if err != nil {
		return "", nil, err
	}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return "", nil, fmt.Errorf("not a directory: %s", root)
	}
	cfg := defaultScanConfig()
	if o.ext != "" {
		if m := parseExts(o.ext); len(m) > 0 {
			cfg.Exts = m
		}
	}
	cfg.Exclude = parsePatterns(o.exclude)
	cfg.UseGitignore = !o.noGitignore
	return root, cfg, nil
}

// checkModeFlags validates one structural mode (--callers or --related):
// no positional query, and none of the search-only flags. The mode name
// is the only difference between the two modes' errors.
func checkModeFlags(o options, mode string) error {
	if len(o.queries) > 0 {
		return fmt.Errorf("--%s takes no positional query", mode)
	}
	if o.defsOnly || o.all || o.limitSet || o.perFileCapSet || o.pathsOnly {
		return fmt.Errorf("--%s is incompatible with --defs-only, --all, --limit, --per-file-cap and --paths-only", mode)
	}
	return nil
}

// loadIndex builds or syncs the BM25 index for root. --once builds in
// memory only; otherwise the cached index is loaded, synced, and saved
// back only when the sync changed something. Proof lines go to stderr,
// exactly as before the split.
func loadIndex(o options, root string, cfg *ScanConfig) (*memIndex, error) {
	if o.once {
		return loadOnceIndex(root, cfg)
	}
	return loadCachedIndex(o, root, cfg)
}

func loadOnceIndex(root string, cfg *ScanConfig) (*memIndex, error) {
	t0 := time.Now()
	ix, st := buildFull(root, cfg)
	fmt.Fprintf(os.Stderr, "(index built in memory: %s; in %s)\n",
		formatProof(ix, st.content, st.oversize, st.binary, st.overchars, st.manifest), time.Since(t0).Round(time.Millisecond))
	return ix, nil
}

func loadCachedIndex(o options, root string, cfg *ScanConfig) (*memIndex, error) {
	db := cachePath(root, cfg.key(), cacheBaseDir(o.cacheDir))
	lock := db + ".lock"
	var ix *memIndex
	err := withLock(lock, func() error {
		di, ok := loadDiskIndex(db)
		if !ok {
			t0 := time.Now()
			var st buildStats
			ix, st = buildFull(root, cfg)
			if err := ix.save(db); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "(index rebuilt: %s; in %s)\n",
				formatProof(ix, st.content, st.oversize, st.binary, st.overchars, st.manifest), time.Since(t0).Round(time.Millisecond))
			return nil
		}
		ix = memFromDisk(di)
		t0 := time.Now()
		a, m, d := ix.sync(root, cfg)
		syncDur := time.Since(t0).Round(time.Millisecond)
		if a+m+d > 0 {
			if err := ix.save(db); err != nil {
				return err
			}
		}
		// Proof line, printed on EVERY cached run (not just on change):
		// what index is in use, the freshness delta the sync just
		// verified, and what the sync cost. This is what lets an agent
		// trust the cache instead of paying a full --once rebuild.
		// "Files" here means content files, the same definition as the
		// --once and rebuild lines; skipped/manifest reconcile it.
		content, ov, bi, oc, manifest := ix.skipBreakdown()
		fmt.Fprintf(os.Stderr, "(index: %s; synced +%d ~%d -%d in %s)\n",
			formatProof(ix, content, ov, bi, oc, manifest), a, m, d, syncDur)
		return nil
	})
	// Found during the Tahap 3 split: the old inline code declared
	// `ferr` but assigned withLock's result to `_`, so ferr was always
	// nil and every cache error was silently ignored. The split keeps
	// the observable behaviour for the case that matters — a save
	// failure must not fail a search whose in-memory index is valid —
	// but fixes the unsafe case: if the lock itself failed before any
	// index was built, ix is nil and rendering it would panic, so
	// that error is now returned (exit 1 in run) instead.
	if err != nil && ix == nil {
		return nil, err
	}
	return ix, nil
}

// renderResults renders search results for an already-loaded index.
// Batch mode: several queries, one index build, with its own rendering.
// The single-query path below stays byte-identical to before the split.
func renderResults(o options, ix *memIndex) int {
	if len(o.queries) > 1 {
		return runBatch(o, ix)
	}
	plan := parseQuery(o.queries[0])
	total, shown := searchOne(ix, plan, o)
	if o.jsonOut {
		return writeJSON(total, shown, ix, plan, o)
	}
	if total == 0 {
		fmt.Println("no hits")
		return 0
	}
	hits := makeHits(shown, ix, plan, o)
	if o.pathsOnly {
		seen := map[string]bool{}
		for _, h := range hits {
			if !seen[h.path] {
				seen[h.path] = true
				fmt.Println(h.path)
			}
		}
	} else {
		printHits(hits, plan)
	}
	fmt.Printf("%d hit(s), showing %d\n", total, len(hits))
	return 0
}

// jsonHit is one machine-readable search hit. Score is nil for literal
// (unranked) search, mirroring the scoreless human-readable header.
type jsonHit struct {
	Path    string         `json:"path"`
	Start   int            `json:"start"`
	End     int            `json:"end"`
	Score   *float64       `json:"score"`
	Snippet string         `json:"snippet"`
	Kind    string         `json:"kind"`
	Related []relEntryJSON `json:"related"`
}

type jsonEnvelope struct {
	Total int       `json:"total"`
	Shown int       `json:"shown"`
	Hits  []jsonHit `json:"hits"`
}

// searchOne runs one query against an already-built index: parse, search,
// diversify. Shared by the single-query and batch paths.
// effLimits resolves the display caps: --all lifts both caps (full recall,
// no saturation decay); --per-file-cap 0 lifts only the per-file cap.
// A returned limit of -1 means no total cap.
func (o options) effLimits() (limit, perFileCap int) {
	limit, perFileCap = o.limit, o.perFileCap
	if o.all {
		limit, perFileCap = -1, 0
	}
	return limit, perFileCap
}

// humanSnippet picks the snippet renderer for human output: term highlights
// (>>> <<<) by default, plain code with --plain.
func (o options) humanSnippet(text string, plan queryPlan) string {
	if plan.likeMode {
		if o.plain {
			return likeWindow(text, plan.raw)
		}
		return likeSnippet(text, plan.raw)
	}
	if o.plain {
		return plainSnippet(text, plan.runs, o.snip)
	}
	return ftsSnippet(text, plan.runs, o.snip)
}

func searchOne(ix *memIndex, plan queryPlan, o options) (int, []scoredChunk) {
	if o.defsOnly && plan.likeMode {
		// "Where is util::helper defined?" searches definitions of the
		// trailing name: the literal qualified path only occurs at use
		// sites, never at the definition, so literal candidacy would
		// always (and misleadingly) return nothing.
		name := lastNameSegment(plan.raw)
		if name == "" {
			return 0, nil
		}
		plan = parseQuery(name)
	}
	var scored []scoredChunk
	if plan.likeMode {
		_, scored = ix.likeSearch(plan.raw, o.excludeTerms, o.limit*fetchMult)
	} else {
		_, scored = ix.searchBody(plan, o.excludeTerms, o.limit*fetchMult)
	}
	if o.defsOnly {
		// Narrow to definitions BEFORE diversify, so the display caps
		// apply to definition chunks rather than hiding them.
		scored = filterDefChunks(scored, ix, plan)
	}
	total := len(scored)
	limit, cap := o.effLimits()
	return total, diversify(scored, ix, limit, cap, o.full)
}

// buildJSONHits converts shown chunks to machine-readable hits (plain
// snippets, no >>> <<< markers). Shared by writeJSON and runBatch.
func buildJSONHits(shown []scoredChunk, ix *memIndex, plan queryPlan, o options) []jsonHit {
	hits := []jsonHit{}
	for _, s := range shown {
		d := ix.chunks[s.doc]
		var sn string
		if plan.likeMode {
			sn = likeWindow(d.Text, plan.raw)
		} else {
			sn = plainSnippet(d.Text, plan.runs, o.snip)
		}
		var score *float64
		if !plan.likeMode {
			sc := s.score
			score = &sc
		}
		related := []relEntryJSON{}
		if d.Name != "" {
			for _, r := range ix.relatedFor(d.Name, d.Path, d.Line) {
				related = append(related, relEntryJSON{
					Path: r.Path, Line: r.Line, Kind: r.Kind, Detail: r.Detail,
				})
			}
		}
		hits = append(hits, jsonHit{
			Path:    d.Path,
			Start:   d.Line,
			End:     d.Line + strings.Count(d.Text, "\n"),
			Score:   score,
			Snippet: sn,
			Kind:    d.Kind,
			Related: related,
		})
	}
	return hits
}

// writeJSON prints the machine-readable envelope to stdout. Stdout stays
// pure JSON (parseable with json.loads/jq with no filtering); the human
// summary goes to stderr. Snippets are plain (no >>> <<< markers).
func writeJSON(total int, shown []scoredChunk, ix *memIndex, plan queryPlan, o options) int {
	env := jsonEnvelope{Total: total, Shown: len(shown), Hits: buildJSONHits(shown, ix, plan, o)}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false) // keep < > & readable in code snippets
	if err := enc.Encode(env); err != nil {
		fmt.Fprintf(os.Stderr, "codesearch: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "%d hit(s), showing %d\n", total, len(env.Hits))
	return 0
}

// batchQuery is one query's result inside the batch envelope.
type batchQuery struct {
	Query string    `json:"query"`
	Total int       `json:"total"`
	Shown int       `json:"shown"`
	Hits  []jsonHit `json:"hits"`
}

// runBatch answers several queries from one index build. Blank queries are
// skipped (a trailing newline on stdin-style input shouldn't fail the batch).
func runBatch(o options, ix *memIndex) int {
	if o.jsonOut {
		env := struct {
			Queries []batchQuery `json:"queries"`
		}{Queries: []batchQuery{}}
		for _, q := range o.queries {
			if strings.TrimSpace(q) == "" {
				continue
			}
			plan := parseQuery(q)
			total, shown := searchOne(ix, plan, o)
			env.Queries = append(env.Queries, batchQuery{
				Query: q,
				Total: total,
				Shown: len(shown),
				Hits:  buildJSONHits(shown, ix, plan, o),
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(env); err != nil {
			fmt.Fprintf(os.Stderr, "codesearch: %v\n", err)
			return 1
		}
		return 0
	}
	for _, q := range o.queries {
		if strings.TrimSpace(q) == "" {
			continue
		}
		plan := parseQuery(q)
		total, shown := searchOne(ix, plan, o)
		fmt.Printf("=== query: %s ===\n", q)
		if total == 0 {
			fmt.Println("no hits")
			continue
		}
		hits := makeHits(shown, ix, plan, o)
		if o.pathsOnly {
			seen := map[string]bool{}
			for _, h := range hits {
				if !seen[h.path] {
					seen[h.path] = true
					fmt.Println(h.path)
				}
			}
		} else {
			printHits(hits, plan)
		}
		fmt.Printf("%d hit(s), showing %d\n", total, len(hits))
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:]))
}
