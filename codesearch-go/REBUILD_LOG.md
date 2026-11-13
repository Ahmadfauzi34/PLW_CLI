# codesearch rebuild log

## 2026-10-20 — rebuild-regression investigation closed

**Context.** On 2026-10-10 a clean rebuild from the "finalized" source produced a
binary failing 54/55 black-box tests (MRR 0.000); the experimental `codesearch-gate3`
binary (55/55) was restored instead, leaving source and installed binary divergent.
Root cause was never investigated; the decision sat open since 2026-10-11.

**This run (2026-10-20 ~09:50 WIB):**
- VM replacement had wiped `/usr/local/bin/codesearch`; restored via
  `~/workspace/bin/restore-env.sh` (also reinstalled cairosvg). `codesearch` back on PATH.
- Verified `search.go` md5 `b66397e98241e41f8e024f85dfa134ad` — identical to the
  `search.go.bm25-opt-20261010` snapshot.
- Installed Go 1.27.3 locally (`~/.local/go`; sandbox previously had no Go).
- `go build` of the current source → 36,208,890-byte linux/amd64 binary.
- Full black-box suite (`bin/tests/test_codesearch_go.py`): **55/55 OK** —
  once with an isolated HOME/cache, once with the real cache. The defect does NOT
  reproduce.
- `GOOS=android GOARCH=arm64 go build` → 35,975,701-byte aarch64 ELF (builds clean;
  NOT installed — on-device verification needs phone-ssh with the user's permission).

**Conclusion.** The current source is good and the rebuild path is trustworthy again.
The 2026-10-10 failure is attributed to the transient mid-cleanup source state
(the `isTokenRune` deletion/repair window documented in `~/memory/2026-10-10.md`),
not to a latent defect in the finalized source.

**Installed:** `codesearch-go` = fresh verified build (md5 `b986c42f184d7dc1d30d03fd8dc5c198`).
Previous gate3 experimental binary kept as `codesearch-go.bak-gate3-20261010`
(md5 `abb976928e9cb72d3ec24bbc932e27e3`). Source/binary divergence is closed.

## 2026-10-26 — batch rekomendasi 2–6 (analisa disetujui user, eksekusi "Oke kerjakan")

**Perubahan.** (1) Definisi "files" tunggal: baris bukti `--once`/rebuild/
cached kini memakai file ber-konten + rincian skip level-baca
(oversize/biner/overchars) + total manifest (content + skipped ==
manifest); `FileMeta.Skip` dipersist di gob, skema go8→go9 (cache lama
auto-rebuild). (2) Baris bukti `--callers`/`--related` (parsed + skipped
per alasan: unsupported/ast-oversize/parse-fail/timeout/source-skip/
unreadable). (3) Timeout 5 dtk/file untuk `--callers`/`--related` via
helper `runTimed` bersama (dipakai juga chunking AST); timeout membatasi
per file, bukan total run. (4) Flag `--version`
(`schema=go9 commit=<tree-hash|unknown> built=<UTC|unknown> go=...`).
(5) `references/codesearch-usage.md` disinkronkan (angka 2026-10-26,
flag --full/--callers/--related/--version, seksi Agent rules).
(6) Eval presisi: probe def_rank limit 20 + agregasi mean def_rank dan
top-1-is-definition rate + header git rev.

**Build (lokal, Go 1.27.3 di ~/.local/go — bukan gup, karena Go lokal
sudah ada sejak 2026-10-20):**
- `COMMIT=$( (find . -name '*.go' -not -name '*_test.go' | sort | xargs sha256sum; sha256sum go.mod go.sum) | sha256sum | cut -c1-12 )`
- `DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)`
- `go build -ldflags "-X main.buildCommit=$COMMIT -X main.buildDate=$DATE" -o codesearch-go .`
- Tanpa ldflags, `--version` jujur mencetak `unknown` (diuji di unit test).
- `GOOS=android GOARCH=arm64 go build` untuk batch ini DIVERIFIKASI di
  langkah akhir (lihat bawah); instalasi ke HP tetap butuh izin user.
  HASIL VERIFIKASI: build bersih, ELF 64-bit ARM aarch64 (BuildID sha1
  34df3521…), TIDAK diinstal — sama seperti kebijakan sebelumnya.

**Verifikasi.** `go vet` bersih; `go test` OK; black-box 84/84 (80 lama +
3 proof-line baru + 1 `--version`); smoke ai_agent_builder: baris bukti
370 file/13535 chunk, `--callers getGraphParametersFlat` 9 call site,
`--related` 1 baris ffi facade/graph.rs:121. Biner ini: commit
`f906edaddf54`, built 2026-10-26T15:36:41Z, 36.359.586 byte.

**Penyimpangan dari analisa (dicatat, bukan disembunyikan).** (a) Bucket
`unreadable` TIDAK masuk baris bukti indeks: file gagal stat/read tidak
pernah masuk manifest (perilaku lama dipertahankan), sehingga identitas
content + skipped == manifest hanya exact dengan tiga bucket; unreadable
tetap ada di baris bukti callers/related (di sana ia terhitung dari
walk). (b) Timeout tidak punya black-box end-to-end: dua pola input
patologis yang dicoba (parens tak berpasangan, nesting 3000) parse
<1 dtk; wrapper terverifikasi di unit test + bucket diuji di proof test
— komposisional, bukan end-to-end. (c) Counter level-walk (Tier 2)
tetap ditunda sesuai analisa.

## 2026-10-31 — Tahap 1 maintainability (evaluasi 2026-10-31, eksekusi "Oke, kerjakan Tahap 1")

**Perubahan, tanpa ubah perilaku.** (1) `gofmt -w search.go scan_test.go
search_test.go` — diff sebelumnya diverifikasi hanya format (spasi
komentar + satu baris kosong sebelum komentar di `search.go`); sesudahnya
`gofmt -l` kosong. (2) Hapus fungsi mati `byteOffset` (`snippet.go`,
satu baris `return 0`, nol pemanggil — diverifikasi grep sebelum hapus).
(3) Snapshot `search.go.bm25-opt-20261010` dipindah keluar dari direktori
sumber ke `../references/codesearch-history/search.go.bm25-opt-20261010`
(sha256 sama sebelum/sesudah pindah:
`791c8037cb9ce62c87dac883ed3f7d316f5a161d96a37edec5648f554a8c80bf`).
(4) `build.sh` baru: gerbang `gofmt -l` (gagal keras, tanpa auto-format)
→ `go vet` → `go test` → build dengan perintah ldflags yang sama seperti
entri 2026-10-26 (tree-hash sumber non-test + go.mod/go.sum, tanggal UTC);
entri log ini merujuk ke skrip, bukan menyalin perintah lagi. Go dicari
di PATH dulu, lalu `~/.local/go/bin/go`. Build Android sengaja TIDAK
masuk skrip (aturan user 2026-10-27: bukan debt, hanya bila diminta).
(5) Unit test tabel `TestParseArgsTable` di `main_test.go` (9 subtest:
defaults, nilai terpisah, bentuk `--flag=val` + flag bool, mode
`--callers`, tokenisasi `--exclude-term`, nilai hilang, `--limit`
negatif, `--limit` non-numerik, flag tak dikenal).

**Verifikasi.** `./build.sh` lolos semua gerbang; `go vet` bersih;
`go test` OK (69 fungsi test, naik dari 68); black-box 84/84 dalam
9,1 dtk. Biner baru: commit `cbbee4ee9a1b`, built
`2026-10-31T13:16:17Z` (20:16 WIB), 36.359.570 byte, skema tetap go9 —
commit berubah dari `f906edaddf54` karena sumber berubah (gofmt +
hapus byteOffset), dan itu benar: biner harus dilacak dari sumbernya.
Symlink `/usr/local/bin/codesearch` menunjuk file ini, jadi PATH
langsung memakai biner baru tanpa langkah instal terpisah.

**Temuan sampingan, TIDAK diperbaiki (di luar scope Tahap 1):** saat
memeriksa diff gofmt, terlihat blok komentar di sekitar `diversify`
di `search.go` yang terduplikasi/terpotong (paragraf yatim diawali
"and decays the 2nd chunk..." diikuti paragraf lengkap yang sama, lalu
komentar `dedupContained` dalam dua gaya). Hanya komentar, bukan kode —
kandidat pembersihan Tahap 2, bukan perbaikan diam-diam di Tahap 1.

## 2026-10-31 — Tahap 2 maintainability (evaluasi 2026-10-31, eksekusi "Ya oke lanjut tahap 2")

**Perubahan, tanpa ubah perilaku, dikerjakan satu langkah per satu dengan
`go vet` + `go test` sesudah tiap langkah.** (1) `extOf` bersama: 3 situs
inline `LastIndex(path, ".")` di `astchunk.go`/`related.go` (2×) diganti
`extOf` dari `callers.go`; kini hanya `extOf` sendiri yang memuat pola
itu. (2) `lineIndex` bersama: blok `lineStarts` inline di
`astDefinitions` dan `extractCalls` diganti `newLineIndex(src).lineOf`
dari `related.go`; grep `lineStarts` kini kosong di luar definisi
`lineIndex`. (3) `parseBraceNames` baru di `related.go`: dua blok parse
daftar `{..}` di `parseExportStmt` (re-export bernama vs export lokal,
hanya `Target` yang beda) menjadi satu helper; perilaku dipertahankan
persis termasuk kasus braces hilang dan daftar kosong. (4) `specstore.go`
baru: satu tipe generik `specStore[T]` menggantikan 6 pola cache
bermutex + nil-caching (`specCache` astchunk, `callSpecCache`,
`callerDefSpecCache`, `relExportSpecCache`, `relVarSpecCache`, dan trio
`relRustSpecMu/OK/Up`); `compileSpec` lama diganti `compileCallSpec` +
`lookupCallLang`; tiap tabel tetap instans terpisah karena ekstensi yang
sama memetakan ke query berbeda per tabel — satu cache global akan
salah melayani query. Import `sync` kini hanya ada di `specstore.go`.
(5) `snippet_test.go` baru: `TestCamelSymmetry` mengunci simetri token
camel di tiga sisi (index `tokenize(splitCamel)`, query
`tokenizeQuery(splitCamel)`, highlight `fineSpans`) untuk 11 bentuk
identifier termasuk akronim (`URLValue`, `XMLHttpRequest`).

**Verifikasi.** Tiap langkah lolos `go vet` + `go test`; langkah cache
juga lolos `go test -race` (penting karena pola locking yang diubah).
Akhir: `./build.sh` lolos semua gerbang; `go test` OK (70 fungsi test,
dari 69); black-box 84/84 dalam 9,3 dtk. Sumber non-test kini 9 file,
4.562 baris (dari 8 file / 4.577 — specstore +43 baris, dedup −58).
Biner baru: commit `4eb63a39ae0d`, built `2026-10-31T13:30:27Z`
(20:30 WIB), 36.352.744 byte, skema tetap go9.

## 2026-10-31 — Tahap 3 maintainability (evaluasi 2026-10-31, eksekusi "Oke lanjut tahap 3")

**Perubahan.** `run()` di `main.go` (192 baris) dipecah menjadi orkestrasi
54 baris + helper: `checkQueryAndFlags` (query required, empty, konflik
json/paths dan all/limit), `resolveRootAndConfig` (Abs + Stat + ScanConfig),
`checkModeFlags(o, mode)` (satu validasi untuk --callers dan --related),
`loadIndex` → `loadOnceIndex`/`loadCachedIndex`, dan `renderResults`
(batch vs single, byte-identik). Urutan validasi dipertahankan persis:
query/flag dulu, root/config, lalu eksklusivitas mode, lalu validasi
mode, lalu indeks, lalu render — termasuk pesan error dan exit code
(2 untuk usage, 1 untuk kegagalan indeks, 0 untuk sukses).

**Bug laten yang ditemukan split ini (bukan sekadar refactor).** Kode
lama mendeklarasikan `var ferr error` lalu menulis `_ = withLock(...)`,
sehingga `ferr` selalu nil dan SEMUA error jalur cache (termasuk gagal
ambil lock) diam-diam diabaikan. Perilaku yang dipertahankan: gagal
simpan cache tidak menggagalkan pencarian yang indeks in-memory-nya
valid. Perilaku yang diperbaiki: bila lock gagal sebelum indeks
terbentuk (`ix == nil`), error kini dikembalikan (exit 1) alih-alih
me-render indeks nil yang akan panic. Komentar di `loadCachedIndex`
mendokumentasikan pembedaan ini.

**Test baru.** `TestCheckQueryAndFlags` (7 subtest),
`TestCheckModeFlags` (6), `TestResolveRootAndConfig` (dir valid,
dir hilang, override ext) — logika yang dulu inline dan 0% unit kini
teruji langsung.

**Tidak dikerjakan (sengaja, sesuai roadmap).** Item 10 Tahap 3 — tabel
flag untuk `parseArgs` — di roadmap bersyarat ("bila flag baru
ditambahkan"). Tahap 3 ini tidak menambah flag, dan `parseArgs` kini
sudah dijaga `TestParseArgsTable` (Tahap 1) + helper baru, jadi refactor
tabel ditunda sampai flag berikutnya tiba — saat itu tabel menjadi
alasan perubahan, bukan perubahan tanpa pemicu.

**Verifikasi.** `./build.sh` lolos semua gerbang; `go vet` bersih;
`go test` OK (73 fungsi test, dari 70); black-box 84/84 dalam 5,9 dtk;
exit code manual terverifikasi: no-query 2, both-modes 2, bad-root 2,
json+paths 2, callers+limit 2; smoke `--defs-only
getGraphParametersFlat` di ai_agent_builder tetap tepat di
`src/facade/graph.rs:121-124` dengan baris ffi. Biner baru: commit
`515274ac4937`, built `2026-10-31T13:50:22Z` (20:50 WIB),
36.355.508 byte, skema tetap go9.

## 2026-11-01 — Penempatan full skill Go verification-harness untuk tokenize (atas "Oke lanjut penempatan nya full")

**Keputusan penempatan (diambil Muse, sesuai arahan user agar tidak
ditawari pilihan penempatan).** File verifikasi tinggal di direktori
sumber codesearch-go, bukan di scratch dan bukan di direktori skill:
- `harness.go` — salinan VERBATIM skill local-verification-harness-go
  (sha256 identik dengan skill, core tidak diubah).
- `tokenize_spec.go` — satu-satunya file yang dicustom, dengan build
  tag `//go:build verify` di baris pertama. Build/vet/test normal
  (tanpa tag) mengabaikannya, sehingga tidak ada main() ganda.
- `verify-tokenize.sh` — satu-satunya cara menjalankan spec: `go run`
  dengan daftar file eksplisit (mode file mengabaikan tag) dan TANPA
  main.go, jadi main() di spec adalah satu-satunya entry point.
- `options.go` (baru) — tipe `options` dipindah VERBATIM dari main.go
  ke file ini, tanpa perubahan perilaku. Alasannya: callers.go dan
  related.go memakai tipe itu, jadi daftar file verify-tokenize.sh
  (yang tidak mencantumkan main.go) tetap bisa dikompilasi. Satu-satunya
  perubahan perilaku-nol di luar file verifikasi adalah pemindahan ini.

**Kenapa bukan penempatan lain.** Percobaan di trial membuktikan:
build tag saja tidak cukup — menjalankan spec dengan tag "verify"
dalam mode paket tetap akan memuat main.go (tanpa tag) sehingga main()
ganda; dan search.go tidak bisa dikompilasi sendirian karena
`options` (main.go), `relDef` (related.go), dan `jsDefQuery`
(callers.go) tersebar di paket. Kombinasi build tag + daftar file
eksplisit + options.go adalah yang lolos semua gerbang.

**Konsekuensi yang dicatat, bukan disembunyikan.** `build.sh`
menghitung tree-hash dari SEMUA file .go non-test, termasuk
harness.go dan tokenize_spec.go, jadi commit biner berubah
(`515274ac4937` -> `8c5d6ffe8685`) meski logika pencarian tidak
berubah sama sekali. Itu perilaku yang benar menurut definisi
--version (biner mengidentifikasi tepat sumbernya), bukan regresi.

**Verifikasi kanonis.** gofmt -l kosong; `go vet ./...` bersih;
`go test ./...` OK (73 fungsi test, tidak berubah); `./build.sh`
lolos semua gerbang; black-box 84/84 dalam 5,9 dtk; `./verify-tokenize.sh`
20 kasus (primary 4x2, poison 3, differential 6, regression 3),
4 gerbang, IsCompleteAndVerified true, READY_TO_COMMIT, exit 0.
Probe negatif DI kanonis (spec rusak sementara, lalu dihapus):
REG-TOKENIZE-CAMEL-02 dirusak jadi camel-split -> primary lulus tapi
verified false, FALSE_GREEN_ILLUSION + NEIGHBOR_PATH_REGRESSION,
REVERT_OR_ISOLATE_BREAKING_BRANCH, exit 1; verify bersih ulang hijau.
Insiden lingkungan saat verifikasi: symlink /usr/local/bin/codesearch
hilang (VM replacement) sehingga `codesearch` di PATH sempat tidak
ditemukan; dipulihkan dengan restore-env.sh (RESTORE_OK 11/11), lalu
`codesearch --version` menunjukkan commit baru. Biner baru: commit
`8c5d6ffe8685`, built `2026-10-31T19:05:43Z` (02:05 WIB 2026-11-01),
skema tetap go9.

## G8 batch (2026-11-10, PLW G8)
`--callers a,b,c` / `--related a,b,c`: one fresh AST scan answers all names
(`splitBatchNames`, comma-split, dedup, blank-drop). Flat envelope unchanged;
JSON adds `"batch":true` when >1 name; related rows gain additive `"symbol"`.
Single-name output byte-identical (callers) / identical except `symbol`
(related). `go vet` bersih, `go test` OK (+unit TestSplitBatchNames),
black-box 88/88 (+4 TestBatchModes). Binary commit `57178abf1063`, schema
tetap go9. Terukur tree PLW: batch callers 7 nama 3.7 dtk vs individual
28.7 dtk, himpunan baris identik; 20 simbol via `plw ast callers` 3.9 dtk
vs 78.2 dtk. Backup biner lama: codesearch-go.bak-pre-g8-batch.

## G9 index state + G13 contract (2026-11-10, PLW G9/G13)
`--cache-dir` flag + `CODESEARCH_CACHE_DIR` / `XDG_CACHE_HOME` env
(`cacheBaseDir`: flag > env > XDG > $HOME/.cache; default unchanged).
`--index-status` (read-only freshness probe: changed/added/removed
vs the manifest), `--index-warm` (build/sync + save), `--index-clean`
(delete this root's index file) — JSON shape
`{cache_dir, index_path, exists, schema, root, files, chunks,
size_bytes, newest, stale, removed?}`. `ENGINE_CONTRACT.md` written:
what go9 guarantees, compatibility rule, upgrade procedure.
`go vet` bersih, `go test` OK (+4 unit test indexcmd),
black-box 91/91 (+3 TestIndexModes). Binary commit `d81e06b64a49`,
schema tetap go9.

## G2 reproducible build (2026-11-11, PLW G2)
`build.sh` now builds byte-identically for the same source +
toolchain: `-trimpath` strips the local build directory,
`-buildvcs=false` strips the git stamp, and NO wall-clock
`buildDate` is injected anymore (`--version` honestly prints
`built=unknown`). Measured: two builds differing only in
buildDate gave different SHA256 (`558fcbc6…` vs `f384feb3…`), so
no digest pin could ever hold; and the same source built inside
a git repo gave yet another digest (`d532c8d4…` — the checkout's
revision/dirty flag is stamped into the binary unless
`-buildvcs=false`). With all three, two builds from different
directories — one inside a git repo, one outside — are identical
(`ef9a5653df2add49a4807e43476c6cf8db57d4d5187d9a7ca967b2f4821b2972`).
The build date belongs to release metadata, not to the binary.
**Fix 2026-11-11 (same day, after CI went red):** `build.sh` now
also exports `CGO_ENABLED=0`. The CI job builds with CGO_ENABLED=0;
a local build with the default CGO_ENABLED=1 gives the same size
but different bytes — the CI binary's digest was `23719bda…` vs the
local pin `ef9a5653…`, same source, same size — and the first G2 push
went red in `codesearch-engine-smoke` on the pin for exactly that.
A local CGO_ENABLED=0 build is byte-identical to the CI artifact
(verified by downloading it). The pin is now `23719bda…`.
`go vet` bersih, `go test` OK (TestVersionLine unchanged — it
tests versionLine(), not build.sh), black-box 91/91. Binary
commit `d81e06b64a49` (same source as before), schema tetap go9.

## F2 related: TS class + barrel noise (2026-11-11, PLW F2)
Two fixes in one engine change, schema stays go9. (a) `jsDefQuery`'s
class pattern now accepts `type_identifier` as well as `identifier` —
in the TS grammar a class name is a `type_identifier`, so `--related`
never saw a TS class definition (interface/type-alias already used
`type_identifier` in tsChunkExtra, enum's `(identifier)` verified
correct empirically). The alternation cannot live in the shared query:
tree-sitter-javascript has no `type_identifier` node, so the query
would fail to compile for .js/.mjs/.cjs — hence `tsDefQuery`
(callers.go, built from jsDefQuery by one string replacement) for
TS/TSX grammars only; caught live by TestExtractRelatedJS going red,
fixed by the split. (b) `resolveRelated` is now two-phase: re-export
rows are collected with the node they point at, then only rows on
live chains are emitted — a node is live when its file defines the
name as it arrives there (local name, alias-aware) or re-exports it
onward to a live node (fixpoint, cycle-safe). When no chain reaches a
definition of the queried name and there is no def row, the whole
trace is kept (it is the only information). Falsifier: a barrel of 20
`export *` went from 20 "(definition not found there)" rows to def +
1 live re-export; no-def query keeps its full 3-row trace.
Real bug caught by the tests during this change: `traceAdd` shared
the `seen` dedup map with `add`, so every trace row counted as
already-emitted and phase 2 emitted nothing — now a separate
`traceSeen` map (the first `go test` run after the change failed 5
related tests on exactly this). `go vet` bersih, `go test` OK (+3
unit test F2), black-box 91/91, F2 battery 7/7. Binary commit
`05c7fa3a8e63`, digest
`f7ef3778d9407a1daa5da223bc959d64ee85a7000597a9ca0fe3d228884dedca`,
schema tetap go9.

## Kotlin engine K1-K4 (2026-11-12, PLW Kotlin target)
Target: oopbot-trial (341 .kt files). Schema stays go9 (no ChunkDoc
change; the default-exts change alters the scan-config key, so old
caches rebuild automatically).
(a) K1: ".kt" joins defaultExts (scan.go) — Kotlin files are indexed
by default; --ext kt already worked as text-only before.
(b) K2: ktDefQuery (astchunk.go) — class/function/object/type-alias.
Node names verified by probe against tree-sitter-kotlin (gotreesitter
v0.55.1, grammars.KotlinLanguage()): interface/enum/data/sealed/
annotation/fun-interface are ALL class_declaration; typealias is
type_alias. The grammar carries NO "name"/"body" field
(ChildByFieldName returns nil), so names and bodies are captured
positionally (first type_identifier/simple_identifier child is the
name — supertypes live in delegation_specifier, never as a direct
type_identifier child; function_body is the optional body child,
single-expression functions wrap theirs in a function_body too).
Properties are NOT defs (a local val would become its own chunk, the
way JS plain vars are not defs either) — top-level vals are found by
--related instead. KDoc (/** */) attaches via an explicit ".kt"
docMarkers case. defKeywords gains "fun", "object", "typealias",
"val" (search.go) — without "fun", --defs-only could not match a
Kotlin function at all; "val" mirrors "const"/"var" (a local val
inside a function body matches the same way a local const does).
(c) K3: ktCallQuery (callers.go) — plain calls capture the direct
simple_identifier; qualified calls (client.connect(), super.foo(),
this.bar()) capture the whole navigation_expression, like Go's
"x.Method", so calleeBase reduces it to the trailing name.
Constructor calls (Foo()) match the plain pattern. Caller column
comes from ktDefQuery def ranges via defQueryFor (no change needed).
(d) K4: extractRelatedKotlin (related.go) — defs from ktDefQuery
plus top-level properties (property_declaration declarators outside
every def range; member properties sit inside their class range and
are skipped, like JS class fields). No re-export edges (Kotlin has
no re-export syntax); the Kotlin<->Rust JNI bridge (external fun <->
Java_...) is a later stage, not v1. runRelated admits .kt.
Usage text updated (--callers languages + --related Kotlin note).
Tests: +3 Go unit tests (TestASTChunkKotlin: chunk starts incl.
KDoc/sig, local val + member property NOT defs; TestExtractCallsKotlin:
plain/qualified/super/top-level constructor; TestExtractRelatedKotlin:
interface/enum/typealias defs, top-level val, member property + enum
entries NOT defs), +6 black-box tests (TestKotlin: default-ext search,
AST proof line, --defs-only class/typealias/function, --callers
qualified + property-reference negative, --related defs + count/ok
negatives). `go vet` bersih, `go test` OK, black-box 97/97 (91+6),
eval MRR 1.000 (def_rank 1.60, top-1-is-definition 4/5 — unchanged).
Live on oopbot-trial: index 282 -> 623 files, AST 241 -> 569 files;
`plw ast defs HostLauncher` finds HostLauncher.kt (KDoc chunk, with
a K15 related row); `plw ast related HostLauncher` -> def at :22.
Binary commit `8da8dea19350`, digest
`c251454d970ae19b76f727e65476a9b945bd7f21711c86d5a9b41efe041ed1cb`
(reproducible: rebuilt from a different directory, same digest).
Pin rotated to engine_pin_v3 (same commit: pin line replaced +
"kotlin" added to the adapter's TREE_SITTER_LANGUAGES; the
fail-closed pin refused the new binary until the rotation, as
designed). Real bugs caught by the tests during this change:
compile error dq.lang (defQueryFor returns a bare Query, not a
spec — fixed to getSpec); black-box --defs-only "start" shown 1 not
>= 2 — the nested-chunk rule keeps only the signature (K12), the
test was wrong, not the engine.

## defs-only opsi 1 (2026-11-13): filter percaya nama AST

Blind spot dari audit v0.2.9 pada ~/workspace/rp: `--defs-only` melewatkan
method TS (`rebuildCSR` :438, `indexDocument` :325, total 0) karena
`filterDefChunks` (search.go) hanya memakai regex `defIdentifierPatterns`
yang menuntut keyword definisi sebelum nama — method TS/JS tidak membawa
keyword. Perbaikan: filter meloloskan chunk bila regex cocok ATAU nama
yang dicatat AST untuk chunk itu sama dengan identifier yang dicari
(`defNameMatches`, penyamaan dinormalisasi: case-insensitive, `_`/`-`
diabaikan, equality penuh — bukan prefix, pelajaran K5). Name dipakai
bukan hanya Kind "sig": method satu baris dan member interface tidak
punya chunk sig (astchunk.go: sigEnd == endLine -> tanpa sig), tapi body
chunk definisinya membawa Name yang sama; window/fallback chunk Name-nya
"" sehingga tak terpengaruh. Name juga yang menjaga semantik AND antar
identifier: chunk sig `func foo(bar string)` tidak bocor ke query "bar".
`defIdentifierTerms` diekstrak dari `defIdentifierPatterns` (refactor
tanpa perubahan perilaku) agar regex dan cek Name memakai daftar
identifier yang sama.
Verifikasi: `go vet` bersih, `go test` 85/85 (1 unit test baru
`TestFilterDefChunksTrustsASTName`: sig chunk, body chunk one-liner,
member interface, Name separator-insensitive, jebakan parameter `bar`,
AND "foo bar"; kontrol negatif — test baru GAGAL di kode lama, 4 assert),
black-box 99/99 (+2 test baru `TestDefsOnlyTSMethods`, gagal di biner
rilis lama), eval MRR 1.000 (tidak berubah), skrip regresi
`audit-v0.2.9/defs-repro/repro_defs_only_methods.py` 24/25 (sebelumnya
14/25), `plw`-level: rebuildCSR 4 hit, indexDocument 6, syncKnowledgeGraph
6, SimHash tetap 0, `export` tetap 0. Biner baru: commit `9a3db30a5027`,
digest `b6f0ec48ec1bfba35a3b4f0a50906bc19bd0063caa33e26ae289069996a921a1`
(reproducible: build ulang dari direktori berbeda, digest sama).
SISA (bukan bagian perbaikan ini): member interface (`ifaceMethod`)
tetap 0 — `jsDefQuery` hanya mengenal `method_definition`, bukan
`method_signature` (node tree-sitter untuk member interface), jadi tidak
ada def chunk sama sekali; memperbaikinya berarti memperluas query AST
(chunking + callers + related) dan butuh bump skema indeks — tahap
berikut yang terpisah. Bonus ranking definisi (compileDefRes) masih
regex-only: method TS tidak mendapat bonus x2.0 di `search` biasa.
Belum di-push ke PLW_CLI; pin engine belum dirotasi (menunggu kata user).
