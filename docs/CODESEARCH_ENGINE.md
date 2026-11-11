# Codesearch engine (tree-sitter)

PLW analysis of Go, Rust, Python, JavaScript, and TypeScript runs
through an external tree-sitter engine binary, `codesearch-go`. The
engine is **paired with the PLW binary, never bundled inside it**:
the agent harness supplies it, and PLW discovers it via an explicit
path, `CODESEARCH_BIN`, or `PATH`.

## Capabilities

| Command | What it answers |
| --- | --- |
| `plw ast defs <root> <name>` | AST-verified definitions of a name |
| `plw ast callers <root> <name>` | Name-based call sites of a name |
| `plw ast related <root> <name>` | Definitions, re-exports, and Rust↔JS bridge relations |
| `plw ast search <root> <query>` | Ranked (BM25) search over the codebase |
| `plw ast index <root> status\|warm\|clean` | The engine index under PLW's cache root |
| `plw ast doctor <root>` | Engine availability, version, schema, pin status |
| `plw topology <root>` | Import graph plus the engine symbol layer |
| `plw agent orient <root> --task ...` | Dynamic context matrix (engine-backed when available) |

Batch queries answer several names in one scan:
`plw ast callers <root> "Run,Greet"` and multi-query
`plw ast defs <root> "Run Greet"`. On a pre-batch engine the
adapter falls back to one query per symbol and records
`batch_mode: "fallback"`.

## Authority

```text
Candidate ≠ Patch
Topology ≠ Causality
Callers/related rows are leads, not proof
```

`--callers` and `--related` match on the trailing identifier, not
on types. Two files defining the same name produce two separate
rows; a call row names its file and line so the reader can verify
it. Read the cited file before treating a call or relation as
proven. The topology symbol layer is additive: without the engine,
`plw topology` output is identical plus an explicit `engine` block
reporting the layer unavailable — the engine never changes the
import graph.

## Discovery

1. explicit `--binary` path (when the command accepts one);
2. `CODESEARCH_BIN` environment variable;
3. `codesearch` on `PATH` — tried in order as `codesearch`,
   `codesearch-go`, then `codesearch-go-linux-x86_64`, so a binary
   downloaded from the GitHub release works as-is, without a
   symlink or env var.

If none resolves, the engine is reported unavailable with the reason
(`plw doctor` / `plw ast doctor`); commands that need it fail closed
instead of guessing.

## Limit

`plw ast defs|callers|related|search <root> <query> --limit N`
returns at most N rows per list. For `search` the limit is passed
to the engine; for the structural modes the adapter slices the
rows itself (the engine is untouched). In a batch query the limit
applies per symbol, and the flat row list is filtered per symbol
too, so it stays the union of the per-symbol lists.

Every envelope reports both numbers: `summary.total` is the rows
found before slicing, `summary.shown` the rows actually returned.
A negative limit is rejected. Without `--limit`, `summary.shown`
is null (except `search`, where the engine reports it).

## Schema contract

The adapter accepts exactly the schemas listed in
`SUPPORTED_SCHEMAS` (`core/codesearch_engine.py`), currently `go9`.
The machine-checked contract lives in
[codesearch-go/ENGINE_CONTRACT.md](../codesearch-go/ENGINE_CONTRACT.md)
and is verified in CI by `scripts/check_engine_contract.sh`. A
newer engine (`go10`) fails with a message saying the engine is
newer than the PLW build; an older engine fails saying it is stale.
A schema bump means a new entry in the adapter **and** a new
contract section, plus the black-box suite and the battery re-run —
never a bare constant change.

## Digest pin

The engine binary is accepted only when its SHA-256 matches an
entry in `PINNED_ENGINE_DIGESTS` (the adapter). The pin only works
because `codesearch-go/build.sh` builds reproducibly (`-trimpath`,
`-buildvcs=false`, no wall-clock build date): the same source and
toolchain produce byte-identical output, so the digest is computable
locally before pushing. A stub that merely prints a pinned version
string is refused — the bytes are what is trusted.

To pin a new engine build:

1. rebuild with `codesearch-go/build.sh`;
2. take the SHA-256 of the built binary;
3. add one line to `PINNED_ENGINE_DIGESTS` **in the same commit** as
   the engine change (the CI pin check fails the build otherwise).

The escape hatch is explicit and recorded, never silent:
`CODESEARCH_ALLOW_UNPINNED=1` runs an unpinned engine, and every
envelope then carries `pinned: false` plus a `pin_warning`.

## Index and cache

The engine's search index lives under PLW's cache root
(`codebase/codesearch` — see `plw doctor`, which reports
`cache_dir`), overridable per call with `--cache-dir` or
`CODESEARCH_CACHE_DIR`. `plw ast index status` reports whether an
index exists for a root and whether it is stale (changed / added /
removed files); `warm` builds it; `clean` deletes it. Engines that
predate index management keep answering queries from their own
default location; only the index commands refuse them, with a
plain message.

## Troubleshooting

- `codesearch binary not found` — supply the engine via
  `CODESEARCH_BIN` or put it on `PATH`; the engine is not bundled.
- `binary digest ... is not pinned` — rebuild with
  `codesearch-go/build.sh` and pin the digest (above), or set
  `CODESEARCH_ALLOW_UNPINNED=1` explicitly for a development run.
- `index schema 'go10' is not the known-good 'go9'` — the engine is
  newer than this PLW build; upgrade PLW or pin the matching engine.
- `engine predates index management` — the engine binary is older
  than index support; rebuild it, or use queries without
  `plw ast index`.
- Empty `plw ast callers` results on an old engine — the adapter
  falls back per symbol and records `batch_mode`; check
  `batch_mode` in the envelope before trusting an empty batch.

## Tests

- Adapter unit tests: `tests/test_codesearch_engine.py`
  (`python3 -m unittest discover -s tests -v`), run in CI by the
  `adapter-unit-tests` job.
- Engine black-box suite: `codesearch-go` (`build.sh` runs gofmt,
  vet, test, then build), run in CI by the `codesearch-go-x86_64` job.
- Paired-binary smoke: `scripts/smoke_codesearch_engine.sh`,
  `smoke_orient_context.sh`, `smoke_topology_engine.sh`,
  `smoke_batch_engine.sh`, `smoke_index.sh`, and
  `scripts/check_engine_contract.sh`, run in CI by the
  `codesearch-engine-smoke` job.
- Cross-release regression batteries live outside the repo in the
  working notes (`test_g6/g8/g9/g2_battery.py`); they are local
  regression evidence, not CI gates.
