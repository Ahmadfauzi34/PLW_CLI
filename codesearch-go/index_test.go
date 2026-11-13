package main

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		t.Fatal("loadDiskIndex rejected the just-saved go10 index")
	}
	if di.SchemaV != "go10" {
		t.Errorf("SchemaV = %q, want go10", di.SchemaV)
	}
	if got := di.Files["bin.rs"].Skip; got != "binary" {
		t.Errorf("reloaded bin.rs Skip = %q, want binary", got)
	}
}

func TestOldSchemaRejected(t *testing.T) {
	// A go8 cache (no Skip field) must not load under schema go9: stale
	// caches rebuild automatically instead of serving Skip-less rows.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(diskIndex{SchemaV: "go8", Files: map[string]FileMeta{}}); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "old.gob")
	if err := os.WriteFile(db, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadDiskIndex(db); ok {
		t.Error("loadDiskIndex accepted a go8 index, want rejection")
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
}
