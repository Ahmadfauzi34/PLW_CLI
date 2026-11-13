package main

// Index: build, incremental sync, gob persistence, file locking.
// Mirrors codesearch.py's manifest/sync/atomic-rebuild design, but the
// ranking core is a hand-rolled BM25 (search.go) instead of SQLite FTS5.

import (
	"crypto/sha1"
	"encoding/gob"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
)

const schemaV = "go9" // go9: FileMeta carries skip reason (oversize/binary/overchars); was go8

// FileMeta is one manifest row: (path -> mtime, size, AST-chunked?).
type FileMeta struct {
	Mtime int64
	Size  int64
	AST   bool // true if the file was chunked via AST definitions (not fallback)
	// Skip explains a manifest-only file: "", "oversize", "binary",
	// "overchars". Content files have Skip == "".
	Skip string
}

// ChunkDoc is one indexed line-chunk. Tokens are recomputed on load
// (deterministic), so the gob stays small.
type ChunkDoc struct {
	Path string
	Line int // 1-based start line
	Text string
	Kind string // "sig", "body", or "window" ("" for pre-K13 indexes)
	Name string // defined name for sig/body chunks, "" for windows
	Dead bool   // in-memory only; save() compacts
}

type diskIndex struct {
	SchemaV string
	Root    string
	Chunks  []ChunkDoc
	Files   map[string]FileMeta
	Newest  int64
	// Related symbol maps (K15): the --related inputs, persisted so a
	// cached index can attach related rows to search hits without
	// re-parsing. relDef/relEdge/relBridge live in related.go.
	RelDefs    []relDef
	RelEdges   []relEdge
	RelBridges []relBridge
}

type posting struct {
	Doc int
	TF  int
}

type memIndex struct {
	root    string
	chunks  []ChunkDoc
	files   map[string]FileMeta
	newest  int64
	byPath  map[string][]int
	bodyIdx map[string][]posting
	pathIdx map[string][]posting
	bodyLen []int
	pathLen []int
	sumBody int64
	sumPath int64
	live    int
	// Related symbol maps, kept in step with chunks by buildFull/sync.
	relDefs    []relDef
	relEdges   []relEdge
	relBridges []relBridge
}

func newMemIndex(root string) *memIndex {
	return &memIndex{
		root:    root,
		files:   map[string]FileMeta{},
		byPath:  map[string][]int{},
		bodyIdx: map[string][]posting{},
		pathIdx: map[string][]posting{},
	}
}

func countTF(toks []string) map[string]int {
	m := make(map[string]int, len(toks))
	for _, t := range toks {
		m[t]++
	}
	return m
}

// addChunkDoc inserts an already-tokenized chunk.
func (ix *memIndex) addChunkDoc(doc ChunkDoc, bodyToks, pathToks []string) {
	idx := len(ix.chunks)
	ix.chunks = append(ix.chunks, doc)
	ix.byPath[doc.Path] = append(ix.byPath[doc.Path], idx)
	for t, tf := range countTF(bodyToks) {
		ix.bodyIdx[t] = append(ix.bodyIdx[t], posting{idx, tf})
	}
	for t, tf := range countTF(pathToks) {
		ix.pathIdx[t] = append(ix.pathIdx[t], posting{idx, tf})
	}
	ix.bodyLen = append(ix.bodyLen, len(bodyToks))
	ix.pathLen = append(ix.pathLen, len(pathToks))
	ix.sumBody += int64(len(bodyToks))
	ix.sumPath += int64(len(pathToks))
	ix.live++
}

func (ix *memIndex) removePostings(idxm map[string][]posting, doc int, toks []string) {
	seen := map[string]bool{}
	for _, t := range toks {
		if seen[t] {
			continue
		}
		seen[t] = true
		ps := idxm[t]
		kept := ps[:0]
		for _, p := range ps {
			if p.Doc != doc {
				kept = append(kept, p)
			}
		}
		if len(kept) == 0 {
			delete(idxm, t)
		} else {
			idxm[t] = kept
		}
	}
}

// removePath deletes all chunks of a file (stale-chunk safe).
func (ix *memIndex) removePath(path string) {
	for _, idx := range ix.byPath[path] {
		d := &ix.chunks[idx]
		if d.Dead {
			continue
		}
		d.Dead = true
		ix.removePostings(ix.bodyIdx, idx, tokenize(splitCamel(d.Text)))
		ix.removePostings(ix.pathIdx, idx, tokenize(d.Path))
		ix.sumBody -= int64(ix.bodyLen[idx])
		ix.sumPath -= int64(ix.pathLen[idx])
		ix.live--
	}
	delete(ix.byPath, path)
}

func (ix *memIndex) avgBody() float64 {
	if ix.live == 0 {
		return 0
	}
	return float64(ix.sumBody) / float64(ix.live)
}

func (ix *memIndex) avgPath() float64 {
	if ix.live == 0 {
		return 0
	}
	return float64(ix.sumPath) / float64(ix.live)
}

// --- persistence -------------------------------------------------------------

// cacheBaseDir resolves where gob indexes live. Precedence (G9):
// explicit --cache-dir > CODESEARCH_CACHE_DIR env > XDG_CACHE_HOME env
// (as $XDG_CACHE_HOME/codesearch-go) > $HOME/.cache/codesearch-go.
// The default (all empty) is unchanged, so standalone use keeps its
// existing indexes; PLW passes its own cache root explicitly.
func cacheBaseDir(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("CODESEARCH_CACHE_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "codesearch-go")
	}
	return filepath.Join(homeDir(), ".cache", "codesearch-go")
}

func cachePath(root, cfgKey, base string) string {
	abs, _ := filepath.Abs(root)
	h := sha1.Sum([]byte(abs + "\x00" + cfgKey))
	dir := base
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, hex.EncodeToString(h[:])[:16]+".gob")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "."
}

func withLock(lockPath string, fn func() error) error {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

// save compacts (drops dead chunks) and writes atomically via tmp+rename.
func (ix *memIndex) save(path string) error {
	var live []ChunkDoc
	for _, d := range ix.chunks {
		if !d.Dead {
			d.Dead = false
			live = append(live, d)
		}
	}
	di := diskIndex{
		SchemaV:    schemaV,
		Root:       ix.root,
		Chunks:     live,
		Files:      ix.files,
		Newest:     ix.newest,
		RelDefs:    ix.relDefs,
		RelEdges:   ix.relEdges,
		RelBridges: ix.relBridges,
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(di); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// load reads a gob index; ok=false when missing/corrupt/wrong schema.
func loadDiskIndex(path string) (di *diskIndex, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	di = &diskIndex{}
	if err := gob.NewDecoder(f).Decode(di); err != nil {
		return nil, false
	}
	if di.SchemaV != schemaV || di.Files == nil {
		return nil, false
	}
	return di, true
}

func memFromDisk(di *diskIndex) *memIndex {
	ix := newMemIndex(di.Root)
	ix.files = di.Files
	ix.newest = di.Newest
	ix.relDefs = di.RelDefs
	ix.relEdges = di.RelEdges
	ix.relBridges = di.RelBridges
	for _, d := range di.Chunks {
		ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
	}
	return ix
}

// removeRel drops one file's related rows (defs, edges, bridges).
func (ix *memIndex) removeRel(path string) {
	{
		var out []relDef
		for _, d := range ix.relDefs {
			if d.Path != path {
				out = append(out, d)
			}
		}
		ix.relDefs = out
	}
	{
		var out []relEdge
		for _, e := range ix.relEdges {
			if e.From != path {
				out = append(out, e)
			}
		}
		ix.relEdges = out
	}
	{
		var out []relBridge
		for _, b := range ix.relBridges {
			if b.RustPath != path {
				out = append(out, b)
			}
		}
		ix.relBridges = out
	}
}

// relatedFor resolves the related rows for one def chunk's name,
// excluding the chunk's own definition row (the hit IS the def; the
// interesting rows are its re-exporters and its FFI bridge).
func (ix *memIndex) relatedFor(name, selfPath string, selfLine int) []relEntry {
	files := map[string]bool{}
	for rel := range ix.files {
		files[rel] = true
	}
	rows := resolveRelated(name, ix.relDefs, ix.relEdges, ix.relBridges, files)
	var out []relEntry
	for _, r := range rows {
		if r.Kind == "def" && r.Path == selfPath && r.Line == selfLine {
			continue
		}
		out = append(out, r)
	}
	return out
}

// chunkingStats reports how many indexed files were chunked via AST
// definitions vs fixed-window fallback (K7 visibility). Files without
// indexed text (binary, unreadable) are never chunked and are excluded
// from both counts.
// skipBreakdown counts manifest files by skip reason. Content files are
// those with Skip == "". Invariant: content + oversize + binary +
// overchars == manifest, on fresh builds and after sync alike — this is
// what makes the three proof lines agree on one definition of "files".
func (ix *memIndex) skipBreakdown() (content, oversize, binary, overchars, manifest int) {
	for _, m := range ix.files {
		manifest++
		switch m.Skip {
		case "oversize":
			oversize++
		case "binary":
			binary++
		case "overchars":
			overchars++
		default:
			content++
		}
	}
	return
}

// buildStats is buildFull's accounting: content files plus the manifest-only
// skip buckets. manifest == content + oversize + binary + overchars.
type buildStats struct {
	content, oversize, binary, overchars, manifest int
}

func (ix *memIndex) chunkingStats() (astFiles, fallbackFiles int) {
	chunked := map[string]bool{}
	for _, d := range ix.chunks {
		if !d.Dead {
			chunked[d.Path] = true
		}
	}
	for path := range chunked {
		if ix.files[path].AST {
			astFiles++
		} else {
			fallbackFiles++
		}
	}
	return astFiles, fallbackFiles
}

// buildFull indexes every file, in parallel, merging in sorted path order
// so chunk indices (and hence tie-breaking) are deterministic.
// Returns the index and its build accounting (content + skip buckets).
// fileOutcome is the per-file indexing result shared by buildFull
// (parallel) and sync (sequential): the file read, its chunk docs and
// AST flag, and its related maps. relOK mirrors extractRelated's ok:
// every false return today carries nil slices, but the flag keeps the
// two callers' ok-handling exact — buildFull ignores it (as before),
// sync checks it (as before).
type fileOutcome struct {
	fr         fileRead
	docs       []ChunkDoc
	astOK      bool
	relDefs    []relDef
	relEdges   []relEdge
	relBridges []relBridge
	relOK      bool
}

// processFile reads one file and derives its index inputs: chunk docs
// (+AST flag) and related maps. Pure — it touches no index state — so
// buildFull can run it in goroutines and sync can call it inline.
func processFile(root, rel string) fileOutcome {
	fr := readSource(root, rel)
	out := fileOutcome{fr: fr}
	if !fr.haveText {
		return out
	}
	chunks, astOK := chunkFile(rel, fr.text)
	out.astOK = astOK
	for _, c := range chunks {
		out.docs = append(out.docs, ChunkDoc{
			Path: rel,
			Line: c.startLine,
			Text: joinLines(c.lines),
			Kind: string(c.kind),
			Name: c.name,
		})
	}
	// Second parse for the related maps (defs, re-export edges,
	// bridges). Cost is paid once per build; the gob persists the
	// maps, so cached runs skip it.
	out.relDefs, out.relEdges, out.relBridges, out.relOK = extractRelated(rel, fr.text)
	return out
}

func buildFull(root string, cfg *ScanConfig) (*memIndex, buildStats) {
	rels := iterFiles(root, cfg)
	results := make([]fileOutcome, len(rels))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, rel := range rels {
		wg.Add(1)
		go func(i int, rel string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = processFile(root, rel)
		}(i, rel)
	}
	wg.Wait()

	ix := newMemIndex(root)
	var st buildStats
	for i, r := range results {
		rel := rels[i]
		if !r.fr.statOK {
			continue
		}
		if r.fr.haveText {
			for _, d := range r.docs {
				ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
			}
			ix.relDefs = append(ix.relDefs, r.relDefs...)
			ix.relEdges = append(ix.relEdges, r.relEdges...)
			ix.relBridges = append(ix.relBridges, r.relBridges...)
			st.content++
		} else {
			switch r.fr.skip {
			case "oversize":
				st.oversize++
			case "binary":
				st.binary++
			case "overchars":
				st.overchars++
			}
		}
		ix.files[rel] = FileMeta{Mtime: r.fr.mtimeNanos, Size: r.fr.size, AST: r.astOK, Skip: r.fr.skip}
		if r.fr.mtimeNanos > ix.newest {
			ix.newest = r.fr.mtimeNanos
		}
	}
	st.manifest = len(ix.files)
	return ix, st
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	n := 0
	for _, l := range lines {
		n += len(l) + 1
	}
	var b = make([]byte, 0, n)
	for i, l := range lines {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, l...)
	}
	return string(b)
}

// --- incremental sync ----------------------------------------------------------

// sync incrementally re-indexes added/changed/removed files against the
// manifest. added vs modified is counted against the pre-sync manifest,
// like codesearch.py.
func (ix *memIndex) sync(root string, cfg *ScanConfig) (added, modified, deleted int) {
	old := make(map[string]bool, len(ix.files))
	for rel := range ix.files {
		old[rel] = true
	}
	rels := iterFiles(root, cfg)
	seen := map[string]bool{}
	for _, rel := range rels {
		full := filepath.Join(root, rel)
		st, err := os.Stat(full)
		if err != nil {
			continue
		}
		seen[rel] = true
		meta := FileMeta{Mtime: st.ModTime().UnixNano(), Size: st.Size()}
		// Freshness compares (mtime, size) only: the AST flag is index
		// metadata, not file identity — comparing it would re-chunk every
		// AST file on each sync.
		if o, ok := ix.files[rel]; ok && o.Mtime == meta.Mtime && o.Size == meta.Size {
			if meta.Mtime > ix.newest {
				ix.newest = meta.Mtime
			}
			continue
		}
		out := processFile(root, rel)
		fr := out.fr
		if !fr.statOK {
			delete(seen, rel)
			continue
		}
		ix.removePath(rel)
		ix.removeRel(rel)
		if fr.haveText {
			for _, d := range out.docs {
				ix.addChunkDoc(d, tokenize(splitCamel(d.Text)), tokenize(d.Path))
			}
			if out.relOK {
				ix.relDefs = append(ix.relDefs, out.relDefs...)
				ix.relEdges = append(ix.relEdges, out.relEdges...)
				ix.relBridges = append(ix.relBridges, out.relBridges...)
			}
			if old[rel] {
				modified++
			} else {
				added++
			}
		}
		ix.files[rel] = FileMeta{Mtime: fr.mtimeNanos, Size: fr.size, AST: out.astOK, Skip: fr.skip}
		if fr.mtimeNanos > ix.newest {
			ix.newest = fr.mtimeNanos
		}
	}
	for rel := range ix.files {
		if !seen[rel] {
			ix.removePath(rel)
			ix.removeRel(rel)
			delete(ix.files, rel)
			deleted++
		}
	}
	return added, modified, deleted
}
