# Engine contract — codesearch-go ↔ PLW

Source of truth for what a codesearch-go build guarantees to PLW.
If this document and the code disagree, the code wins and this
document must be fixed in the same change — a stale contract is
worse than none, because the adapter trusts it.

## Current schema: `go9`

`schemaV` (`index.go`) is `go9`. PLW's adapter
(`core/codesearch_engine.py`) accepts exactly the schemas listed in
`SUPPORTED_SCHEMAS` there; today that is `{"go9"}`.

### What `go9` guarantees

1. **Index format.** A gob index (`diskIndex`) with fields
   `SchemaV`, `Root`, `Chunks`, `Files` (with `FileMeta.Skip`
   carrying `""` / `"oversize"` / `"binary"` / `"overchars"`),
   `Newest`, and the K15 related maps (`RelDefs`, `RelEdges`,
   `RelBridges`). An index written by another schema is rejected
   (`loadDiskIndex` returns `ok=false`) and rebuilt, never read.
2. **Version line.** `codesearch --version` prints exactly
   `codesearch schema=<schema> commit=<tree-hash|unknown>
   built=<UTC|unknown> go=<version>`. The schema always comes from
   the `schemaV` constant; commit/built come from ldflags and stay
   `unknown` when built without them.
3. **Query envelope (JSON).** Search: `{"queries": [...]}` in
   batch, `{"total","shown","hits":[...]}` single. Callers:
   `{"query","batch","total","calls":[{"path","line","caller",
   "callee","text"}]}`. Related: `{"query","batch","total",
   "related":[{"path","line","kind","detail","symbol"}]}`. The
   `callee` in a call row is the **full** callee expression
   (`a.Run`), not the queried trailing name — the adapter reduces
   it with `_callee_base`, mirroring the engine's `calleeBase`.
4. **Batch (G8).** `--callers a,b,c` / `--related a,b,c` answer all
   names in one fresh AST scan; JSON adds `"batch": true` when more
   than one name was given, and the batch is the union of the
   individual runs as a row set. Single-name output is byte-identical
   to the pre-batch contract (except the additive `symbol` field on
   related rows). Positional multi-query search (`--defs-only a b`)
   answers all queries in one index build.
5. **Index commands (G9).** `--index-status` (read-only probe),
   `--index-warm` (build/sync + save), `--index-clean` (delete this
   root's index file). All three print the same JSON shape:
   `{cache_dir, index_path, exists, schema, root, files, chunks,
   size_bytes, newest, stale: {changed, added, removed},
   removed?}` and exit 0 when the command ran. The cache dir
   resolves as: `--cache-dir` flag > `CODESEARCH_CACHE_DIR` env >
   `XDG_CACHE_HOME/codesearch-go` > `$HOME/.cache/codesearch-go`.
6. **Failure behaviour.** Unknown flags, conflicting modes, and
   empty single queries exit 2. A query that cannot run exits 1.
   `--callers` / `--related` always scan fresh (no BM25 index) and
   print a proof line to stderr with parsed/skipped buckets.

### What `go9` does NOT guarantee

- **No digest pin.** `--version` accepts any binary that claims
  `go9` (G2, open): a stub that prints the right line passes the
  schema gate and fails only at query time. Pinning the engine
  commit is a separate gap.
- **No AST cache.** `--callers` / `--related` rescan the tree on
  every call; only the BM25 search index is cached across
  invocations.

## Compatibility rule

The adapter accepts exactly the schemas in `SUPPORTED_SCHEMAS`.
A schema bump is a **breaking change by definition**: an engine
with a new schema must not be used by an adapter that does not
list it, and an adapter that lists a new schema must not be used
with an engine that does not implement it. Both directions fail
closed with a message that names the direction (engine newer vs
older).

## Upgrade procedure (schema bump)

1. Engine: bump `schemaV` in `index.go` (and update the comment
   naming what changed). Update this document's "Current schema"
   section and the guarantee list.
2. Adapter: add the new schema to `SUPPORTED_SCHEMAS` in
   `core/codesearch_engine.py` (keep the old one listed only while
   the old engine is still supported — otherwise replace it).
3. Verify: `bash codesearch-go/build.sh` (gofmt → vet → test →
   build), black-box suite (`bin/tests/test_codesearch_go.py`),
   the G9/G13 battery, and the G6/G8 batteries as regression.
4. Push together: engine source + adapter overlay in the same
   commit, so main never holds a mismatched pair.
5. The CI script `scripts/check_engine_contract.sh` fails the
   build whenever the engine's `schemaV` and the adapter's
   `KNOWN_SCHEMA` disagree — a mismatch is caught at push time,
   not at runtime in a user's hands. If you are reading this
   because CI went red on that step, the fix is step 1 or 2 above,
   not a change to the script.

## History

- `go9` (2026-10-26): `FileMeta.Skip` persisted (oversize/binary/
  overchars); batch recommendation 2–6. Still current.
- G8 (2026-11-10): batch `--callers`/`--related` + `symbol` field
  on related rows, same schema (additive, no bump).
- G9 (2026-11-10): `--cache-dir` + `--index-status`/`--index-warm`/
  `--index-clean`, same schema (additive, no bump).
