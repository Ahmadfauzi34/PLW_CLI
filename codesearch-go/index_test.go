package main

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Batch 2-6 (2026-10-26): one definition of "files" (content) across the
// --once, rebuild, and cached proof lines, reconciled by skip buckets and
// the manifest total. FileMeta.Skip persists the reason a manifest-only
// file has no indexed text (schema go9).
func TestBuildFullSkipAccounting(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.rs", "fn alpha() {}\n")
	write("b.rs", "fn beta() {}\n")
	write("big.md", strings.Repeat("x", 600_000)) // overchars
	if err := os.WriteFile(filepath.Join(root, "bin.rs"), []byte("fn gamma() {}\x00\x00\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ix, st := buildFull(root, defaultScanConfig())
	if st.content != 2 || st.oversize != 0 || st.binary != 1 || st.overchars != 1 || st.manifest != 4 {
		t.Errorf("buildStats = %+v, want content 2, oversize 0, binary 1, overchars 1, manifest 4", st)
	}
	if st.content+st.oversize+st.binary+st.overchars != st.manifest {
		t.Errorf("content + skipped != manifest: %+v", st)
	}
	// skipBreakdown over the fresh index must agree with buildStats.
	c, ov, bi, oc, mf := ix.skipBreakdown()
	if c != st.content || ov != st.oversize || bi != st.binary || oc != st.overchars || mf != st.manifest {
		t.Errorf("skipBreakdown = %d %d %d %d %d, want buildStats %+v", c, ov, bi, oc, mf, st)
	}
	if got := ix.files["a.rs"].Skip; got != "" {
		t.Errorf("a.rs Skip = %q, want empty", got)
	}
	if got := ix.files["big.md"].Skip; got != "overchars" {
		t.Errorf("big.md Skip = %q, want overchars", got)
	}
	if got := ix.files["bin.rs"].Skip; got != "binary" {
		t.Errorf("bin.rs Skip = %q, want binary", got)
	}
}

func TestSkipPersistsAcrossSaveLoad(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bin.rs"), []byte("x\x00y\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ix, _ := buildFull(root, defaultScanConfig())
	db := filepath.Join(t.TempDir(), "index.gob")
	if err := ix.save(db); err != nil {
		t.Fatal(err)
	}
	di, ok := loadDiskIndex(db)
	if !ok {
		t.Fatal("loadDiskIndex rejected the just-saved go11 index")
	}
	if di.SchemaV != "go11" {
		t.Errorf("SchemaV = %q, want go11", di.SchemaV)
	}
	if got := di.Files["bin.rs"].Skip; got != "binary" {
		t.Errorf("reloaded bin.rs Skip = %q, want binary", got)
	}
}

func TestOldSchemaRejected(t *testing.T) {
	// A go10 cache (no CallSites/Generated fields) must not load under
	// schema go11: stale caches rebuild automatically instead of serving
	// incomplete rows.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(diskIndex{SchemaV: "go10", Files: map[string]FileMeta{}}); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "old.gob")
	if err := os.WriteFile(db, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadDiskIndex(db); ok {
		t.Error("loadDiskIndex accepted a go10 index, want rejection")
	}
}

// G9: cache base dir precedence — explicit flag > CODESEARCH_CACHE_DIR >
// XDG_CACHE_HOME > $HOME/.cache. t.Setenv keeps each case isolated.
func TestCacheBaseDirPrecedence(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("CODESEARCH_CACHE_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	if got := cacheBaseDir(""); got != "/home/tester/.cache/codesearch-go" {
		t.Errorf("default = %q, want $HOME/.cache/codesearch-go", got)
	}
	t.Setenv("XDG_CACHE_HOME", "/xdg/cache")
	if got := cacheBaseDir(""); got != "/xdg/cache/codesearch-go" {
		t.Errorf("XDG = %q, want /xdg/cache/codesearch-go", got)
	}
	t.Setenv("CODESEARCH_CACHE_DIR", "/env/cache")
	if got := cacheBaseDir(""); got != "/env/cache" {
		t.Errorf("env = %q, want /env/cache", got)
	}
	if got := cacheBaseDir("/flag/cache"); got != "/flag/cache" {
		t.Errorf("flag = %q, want /flag/cache", got)
	}
}

// G9: cachePath is per (root, config) inside the given base, and the
// same inputs always name the same file.
func TestCachePathUsesBase(t *testing.T) {
	root := t.TempDir()
	cfg := defaultScanConfig()
	a := cachePath(root, cfg.key(), "/base/a")
	b := cachePath(root, cfg.key(), "/base/b")
	if a == b {
		t.Errorf("different bases gave the same path %q", a)
	}
	if filepath.Dir(a) != "/base/a" {
		t.Errorf("path %q is not under /base/a", a)
	}
	if again := cachePath(root, cfg.key(), "/base/a"); again != a {
		t.Errorf("same inputs gave different paths: %q vs %q", a, again)
	}
}

// G9: status on a fresh root reports exists:false with every file
// added; after warm it reports exists:true with zero staleness; after
// editing one file the probe counts exactly one changed; after clean
// the file is gone and status is fresh again.
func TestIndexStatusWarmClean(t *testing.T) {
	root := t.TempDir()
	base := t.TempDir()
	cfg := defaultScanConfig()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.go", "package main\n\nfunc Greet() {}\n")

	st := statusFor(root, cfg, base)
	if st.Exists {
		t.Errorf("fresh status exists = true, want false")
	}
	if st.Stale.Added != 1 || st.Stale.Changed != 0 || st.Stale.Removed != 0 {
		t.Errorf("fresh stale = %+v, want {0 1 0}", st.Stale)
	}
	if _, err := os.Stat(st.IndexPath); !os.IsNotExist(err) {
		t.Errorf("index file exists before warm: %s", st.IndexPath)
	}

	o := options{root: root, jsonOut: true, cacheDir: base}
	if rc := runIndexWarm(o, root, cfg); rc != 0 {
		t.Fatalf("runIndexWarm returned %d", rc)
	}
	st = statusFor(root, cfg, base)
	if !st.Exists || st.Schema != schemaV {
		t.Errorf("after warm: exists=%v schema=%q, want true %q", st.Exists, st.Schema, schemaV)
	}
	if st.Files != 1 || st.Chunks == 0 {
		t.Errorf("after warm: files=%d chunks=%d, want 1 and >0", st.Files, st.Chunks)
	}
	if st.Stale != (staleCounts{}) {
		t.Errorf("after warm: stale = %+v, want zero", st.Stale)
	}

	write("a.go", "package main\n\nfunc Greet() {}\n\nfunc Bye() {}\n")
	st = statusFor(root, cfg, base)
	if st.Stale.Changed != 1 || st.Stale.Added != 0 || st.Stale.Removed != 0 {
		t.Errorf("after edit: stale = %+v, want {1 0 0}", st.Stale)
	}

	if rc := runIndexClean(o, root, cfg); rc != 0 {
		t.Fatalf("runIndexClean returned %d", rc)
	}
	st = statusFor(root, cfg, base)
	if st.Exists {
		t.Errorf("after clean: exists = true, want false")
	}
	if _, err := os.Stat(st.IndexPath); !os.IsNotExist(err) {
		t.Errorf("index file still exists after clean: %s", st.IndexPath)
	}
	// Clean again is not an error: removed=false, status still fresh.
	if rc := runIndexClean(o, root, cfg); rc != 0 {
		t.Fatalf("second runIndexClean returned %d", rc)
	}
}

// G9: index modes reject queries and search-only flags (exit-2 shape
// is checked at the run() level in black-box; here the validator).
func TestCheckIndexFlags(t *testing.T) {
	if err := checkIndexFlags(options{}, "index-status"); err != nil {
		t.Errorf("empty options rejected: %v", err)
	}
	if err := checkIndexFlags(options{queries: []string{"x"}}, "index-status"); err == nil {
		t.Error("positional query accepted, want rejection")
	}
	if err := checkIndexFlags(options{callersSet: true}, "index-warm"); err == nil {
		t.Error("--callers accepted with --index-warm, want rejection")
	}
	if err := checkIndexFlags(options{once: true}, "index-clean"); err == nil {
		t.Error("--once accepted with --index-clean, want rejection")
	}
	if err := checkIndexFlags(options{cached: true}, "index-warm"); err == nil {
		t.Error("--cached accepted with --index-warm, want rejection")
	}
}

func TestCachedCallsAndGeneratedPersistAcrossSync(t *testing.T) {
	root := t.TempDir()
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
	write("src/app.ts", "export function greet() { return helper(); }\nexport function helper() { return 1; }\n", t0)
	write("src/types.d.ts", "export declare function fakeCall(): void;\n", t0)
	write("public/assets/bundle.js", "/* @ts-self-types=\"./x.d.ts\" */\nfunction wasmCall() { helper(); }\n", t0)

	ix, _ := buildFull(root, defaultScanConfig())
	if ix.files["src/app.ts"].Generated {
		t.Error("src/app.ts should not be flagged generated")
	}
	if !ix.files["src/types.d.ts"].Generated {
		t.Error("src/types.d.ts should be flagged generated")
	}
	if !ix.files["public/assets/bundle.js"].Generated {
		t.Error("public/assets/bundle.js should be flagged generated")
	}
	for _, c := range ix.callSites {
		if c.Path == "src/types.d.ts" || c.Path == "public/assets/bundle.js" {
			t.Errorf("unexpected callSite from generated file: %+v", c)
		}
	}
	if len(ix.callSites) != 1 || ix.callSites[0].Callee != "helper" || ix.callSites[0].Caller != "greet" {
		t.Errorf("callSites = %+v, want [helper in greet]", ix.callSites)
	}

	db := filepath.Join(t.TempDir(), "index.gob")
	if err := ix.save(db); err != nil {
		t.Fatal(err)
	}
	di, ok := loadDiskIndex(db)
	if !ok {
		t.Fatal("loadDiskIndex rejected saved go11 index")
	}
	if !di.Files["src/types.d.ts"].Generated || !di.Files["public/assets/bundle.js"].Generated {
		t.Errorf("FileMeta.Generated lost across gob save/load: %+v", di.Files)
	}
	if len(di.CallSites) != 1 || di.CallSites[0].Callee != "helper" {
		t.Errorf("di.CallSites = %+v, want [helper]", di.CallSites)
	}

	ix2 := memFromDisk(di)
	t1 := t0.Add(2 * time.Second)
	write("src/app.ts", "export function greet() { return other(); }\nexport function other() { return 2; }\n", t1)
	write("src/extra.jsx", "export function Card() { return other(); }\n", t1)
	a, m, d := ix2.sync(root, defaultScanConfig())
	if a != 1 || m != 1 || d != 0 {
		t.Errorf("sync 1 counts = (%d,%d,%d), want (1,1,0)", a, m, d)
	}
	gotCallees := map[string]int{}
	for _, c := range ix2.callSites {
		gotCallees[c.Path+":"+calleeBase(c.Callee)]++
	}
	if gotCallees["src/app.ts:helper"] != 0 || gotCallees["src/app.ts:other"] != 1 || gotCallees["src/extra.jsx:other"] != 1 {
		t.Errorf("callSites after edit+add sync = %+v (counts=%v)", ix2.callSites, gotCallees)
	}

	// File becomes generated and extra file deleted -> old callSites and relDefs evicted.
	t2 := t1.Add(2 * time.Second)
	write("src/app.ts", "/* @ts-self-types=\"./app.d.ts\" */\nexport function greet() { return other(); }\n", t2)
	if err := os.Remove(filepath.Join(root, "src/extra.jsx")); err != nil {
		t.Fatal(err)
	}
	a, m, d = ix2.sync(root, defaultScanConfig())
	if a != 0 || m != 1 || d != 1 {
		t.Errorf("sync 2 counts = (%d,%d,%d), want (0,1,1)", a, m, d)
	}
	if !ix2.files["src/app.ts"].Generated {
		t.Error("src/app.ts should be flagged Generated after adding @ts-self-types header")
	}
	if len(ix2.callSites) != 0 {
		t.Errorf("callSites after generated+delete = %+v, want empty", ix2.callSites)
	}
	for _, rd := range ix2.relDefs {
		if rd.Path == "src/app.ts" || rd.Path == "src/extra.jsx" {
			t.Errorf("stale relDef survived sync: %+v", rd)
		}
	}
}
