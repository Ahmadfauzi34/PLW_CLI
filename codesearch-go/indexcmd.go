package main

// Index commands (G9): --index-status / --index-warm / --index-clean.
//
// The BM25 gob index is per (root, scan-config) and lives under the
// cache base dir (see cacheBaseDir in index.go). Before G9 the only way
// to learn anything about it was to run a query and read the stderr
// proof line, and the only way to rebuild it was to run a query that
// needed it. These three commands make the state visible and
// controllable:
//
//   - status: read-only probe. Loads the disk index if present and
//     counts how many files changed / appeared / disappeared since
//     it was written, using the same (mtime, size) comparison sync
//     uses — but without touching the index or the disk. A missing
//     index reports exists:false and every file as added.
//   - warm: build or sync the index and save it, then report the
//     same status shape. This is "build the index" without a query.
//   - clean: delete this root's index file (and its lock file).
//
// All three print JSON on stdout when --json is set (the shape PLW
// consumes) and a short key: value list otherwise. They exit 0 when
// the command itself ran; a missing index is a status fact, not an
// error. Errors that prevent the command from running at all (bad
// root, unwritable cache dir) exit 1 with the reason on stderr.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// staleCounts is the read-only freshness probe: files whose (mtime,
// size) differs from the manifest (changed), files on disk but not in
// the manifest (added), manifest entries no longer on disk (removed).
// It mirrors sync's accounting except that it never reads file
// contents: a file that became unreadable counts as changed here,
// while sync would drop it. The probe is a freshness indicator, not
// a sync preview — the counts are exact for the common case (files
// edited, created, deleted) and conservative otherwise.
type staleCounts struct {
	Changed int `json:"changed"`
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

// indexStatus is the JSON shape shared by all three commands.
type indexStatus struct {
	CacheDir  string      `json:"cache_dir"`
	IndexPath string      `json:"index_path"`
	Exists    bool        `json:"exists"`
	Schema    string      `json:"schema,omitempty"`
	Root      string      `json:"root"`
	Files     int         `json:"files"`
	Chunks    int         `json:"chunks"`
	SizeBytes int64       `json:"size_bytes"`
	Newest    int64       `json:"newest"`
	Stale     staleCounts `json:"stale"`
	Removed   *bool       `json:"removed,omitempty"` // clean only
}

// probeStale compares the current walk against a manifest without
// mutating anything. files may be nil (no index): everything counts
// as added.
func probeStale(root string, cfg *ScanConfig, files map[string]FileMeta) staleCounts {
	var st staleCounts
	seen := map[string]bool{}
	for _, rel := range iterFiles(root, cfg) {
		full := filepath.Join(root, rel)
		stat, err := os.Stat(full)
		if err != nil {
			continue
		}
		seen[rel] = true
		meta, ok := files[rel]
		if !ok {
			st.Added++
			continue
		}
		if meta.Mtime != stat.ModTime().UnixNano() || meta.Size != stat.Size() {
			st.Changed++
		}
	}
	for rel := range files {
		if !seen[rel] {
			st.Removed++
		}
	}
	return st
}

// statusFor loads the disk index (if any) and probes freshness. It
// never writes.
func statusFor(root string, cfg *ScanConfig, base string) indexStatus {
	db := cachePath(root, cfg.key(), base)
	st := indexStatus{CacheDir: base, IndexPath: db, Root: root}
	if fi, err := os.Stat(db); err == nil {
		st.SizeBytes = fi.Size()
	}
	di, ok := loadDiskIndex(db)
	if !ok {
		st.Stale = probeStale(root, cfg, nil)
		return st
	}
	st.Exists = true
	st.Schema = di.SchemaV
	st.Files = len(di.Files)
	st.Chunks = len(di.Chunks)
	st.Newest = di.Newest
	st.Stale = probeStale(root, cfg, di.Files)
	return st
}

// printIndexStatus renders one status in the requested form.
func printIndexStatus(o options, st indexStatus) int {
	if o.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(st); err != nil {
			fmt.Fprintf(os.Stderr, "codesearch: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Printf("cache_dir: %s\n", st.CacheDir)
	fmt.Printf("index_path: %s\n", st.IndexPath)
	fmt.Printf("exists: %v\n", st.Exists)
	if st.Schema != "" {
		fmt.Printf("schema: %s\n", st.Schema)
	}
	fmt.Printf("root: %s\n", st.Root)
	fmt.Printf("files: %d\n", st.Files)
	fmt.Printf("chunks: %d\n", st.Chunks)
	fmt.Printf("size_bytes: %d\n", st.SizeBytes)
	fmt.Printf("newest: %d\n", st.Newest)
	fmt.Printf("stale: changed=%d added=%d removed=%d\n", st.Stale.Changed, st.Stale.Added, st.Stale.Removed)
	if st.Removed != nil {
		fmt.Printf("removed: %v\n", *st.Removed)
	}
	return 0
}

// runIndexStatus implements --index-status: a read-only probe.
func runIndexStatus(o options, root string, cfg *ScanConfig) int {
	return printIndexStatus(o, statusFor(root, cfg, cacheBaseDir(o.cacheDir)))
}

// runIndexWarm implements --index-warm: build/sync the index and save
// it, then report status. It reuses loadCachedIndex (which already
// builds-or-syncs and saves on change); a missing index is built,
// a fresh one is left alone — either way the status afterwards is
// the truth about what is on disk.
func runIndexWarm(o options, root string, cfg *ScanConfig) int {
	if _, err := loadCachedIndex(o, root, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "codesearch:", err)
		return 1
	}
	return printIndexStatus(o, statusFor(root, cfg, cacheBaseDir(o.cacheDir)))
}

// runIndexClean implements --index-clean: delete this root's index
// file. The removal happens under the same file lock the index uses,
// so a concurrent query cannot observe a half-deleted file; the lock
// file itself is removed afterwards (a stale lock file is harmless,
// but leaving one per cleaned root would litter the cache dir).
func runIndexClean(o options, root string, cfg *ScanConfig) int {
	base := cacheBaseDir(o.cacheDir)
	db := cachePath(root, cfg.key(), base)
	lock := db + ".lock"
	removed := false
	err := withLock(lock, func() error {
		if err := os.Remove(db); err == nil {
			removed = true
		} else if !os.IsNotExist(err) {
			return err
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "codesearch:", err)
		return 1
	}
	_ = os.Remove(lock)
	st := statusFor(root, cfg, base)
	st.Removed = &removed
	return printIndexStatus(o, st)
}

// checkIndexFlags validates one index command: no query, no structural
// mode, none of the search-only flags. Only --root, --cache-dir,
// --ext, --exclude and --no-gitignore (which shape the index) plus
// --json apply.
func checkIndexFlags(o options, mode string) error {
	if len(o.queries) > 0 {
		return fmt.Errorf("--%s takes no positional query", mode)
	}
	if o.callersSet || o.relatedSet {
		return fmt.Errorf("--%s is incompatible with --callers and --related", mode)
	}
	if o.defsOnly || o.all || o.limitSet || o.perFileCapSet || o.pathsOnly || o.once || o.full || o.cached {
		return fmt.Errorf("--%s is incompatible with --defs-only, --all, --limit, --per-file-cap, --paths-only, --once, --full and --cached", mode)
	}
	return nil
}
