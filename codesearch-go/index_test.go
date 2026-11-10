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
		t.Fatal("loadDiskIndex rejected the just-saved go9 index")
	}
	if di.SchemaV != "go9" {
		t.Errorf("SchemaV = %q, want go9", di.SchemaV)
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
