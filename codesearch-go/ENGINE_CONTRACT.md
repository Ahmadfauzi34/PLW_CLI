# Engine contract — codesearch-go ↔ PLW

Source of truth for what a codesearch-go build guarantees to PLW.
If this document and the code disagree, the code wins and this
document must be fixed in the same change — a stale contract is
worse than none, because the adapter trusts it.

## Current schema: `go16`

`schemaV` (`index.go`) is `go16`. PLW's adapter
(`core/codesearch_engine.py`) accepts exactly the schemas listed in
`SUPPORTED_SCHEMAS` there; today that is `{"go9", "go10", "go11", "go12", "go13", "go14", "go15", "go16"}`.

### What `go16` guarantees

1. **Index format.** A gob index (`diskIndex`) with fields
   `SchemaV`, `Root`, `Chunks`, `Files` (with `FileMeta.Skip`
   carrying `""` / `"oversize"` / `"binary"` / `"overchars"` and
   `FileMeta.Generated` flagging machine-generated/minified/declaration
   assets), `Newest`, the K15 related maps (`RelDefs`, `RelEdges`,
   `RelBridges`, `RelJNI` since go14), and `CallSites` (`go11`). An index written by
   another schema is rejected (`loadDiskIndex` returns `ok=false`) and
   rebuilt, never read.
2. **Cached structural queries (`--cached`).** `--callers` and
   `--related` still scan fresh by default, and accept `--cached` to
   answer from the persisted `CallSites` / `RelDefs` / `RelEdges` /
   `RelBridges` / `RelJNI` in the synced gob index instead of re-parsing
   unchanged files (`--cached` is mutually exclusive with `--once` and
   with `--index-*` commands).
3. **Definition precision and asset exclusion.** `isGeneratedOrMinified`
   excludes declaration files (`*.d.ts`, `*.d.mts`, `*.d.cts`),
   `public/assets/` assets, and `wasm-bindgen`/`@ts-self-types`
   generated headers from AST chunking, `CallSites`, `Rel*`, and
   `--defs-only` hits (fail-open fallback window chunks remain
   searchable for plain text queries). In `--defs-only`, AST `"window"`
   chunks on AST-chunked files, documentation files (`.md`/`.txt`/
   `.rst`/`.adoc`), declaration files, and generated files are
   excluded, and fallback chunks match only non-comment lines.
4. **TS interface members are definitions.** `method_signature`
   nodes (interface members and object-type members) are definition
   units in chunking, callers, and related — one set everywhere,
   via `tsDefQuery` (`callers.go`). A go9 index lacks those chunks
   and is rejected by the schema bump, never silently served; a
   go9/go10 engine stays usable through the adapter's
   `SUPPORTED_SCHEMAS` (`go9` lacks interface members; `go9`/`go10`
   lack cached AST calls).
5. **Default extensions.** `defaultExts` includes `.cjs`, `.jsx`,
   `.mts`, `.cts` alongside `.js`, `.mjs`, `.ts`, `.tsx`.
6. **Version line.** `codesearch --version` prints exactly
   `codesearch schema=<schema> commit=<tree-hash|unknown>
   built=<UTC|unknown> go=<version>`. The schema always comes from
   the `schemaV` constant; commit/built come from ldflags and stay
   `unknown` when built without them. Reproducible builds
   (`build.sh`, G2) intentionally print `built=unknown`: the build
   date is release metadata, not binary identity — a wall-clock
   date in the binary would make every build byte-different and
   unpinnable.
7. **Digest pin (G2).** PLW's adapter refuses any engine binary
   whose SHA256 is not in its `PINNED_ENGINE_DIGESTS` (rejection
   order: parse → schema → digest), unless the explicit opt-out
   `CODESEARCH_ALLOW_UNPINNED=1` is set — which the envelope
   records as `pinned: false`, never silently. The pin covers the
   reproducible `build.sh` output (same source + toolchain =
   byte-identical) and the v0.2.5 release asset. What the pin does
   NOT prove: that the pinned source is good — trust in the source
   still comes from review + CI, the same as the capsule pin.
8. **Query envelope (JSON).** Search: `{"queries": [...]}` in
   batch, `{"total","shown","hits":[...]}` single. Callers:
   `{"query","batch","total","calls":[{"path","line","caller",
   "callee","text"}]}`. Related: `{"query","batch","total",
   "related":[{"path","line","kind","detail","symbol"}]}`. The
   `callee` in a call row is the **full** callee expression
   (`a.Run`), not the queried trailing name — the adapter reduces
   it with `_callee_base`, mirroring the engine's `calleeBase`.
9. **Batch (G8).** `--callers a,b,c` / `--related a,b,c` answer all
   names in one scan (fresh or `--cached`); JSON adds `"batch": true`
   when more than one name was given, and the batch is the union of
   the individual runs as a row set. Single-name output is
   byte-identical to the pre-batch contract (except the additive
   `symbol` field on related rows). Positional multi-query search
   (`--defs-only a b`) answers all queries in one index build.
10. **Index commands (G9).** `--index-status` (read-only probe),
    `--index-warm` (build/sync + save), `--index-clean` (delete this
    root's index file). All three print the same JSON shape:
    `{cache_dir, index_path, exists, schema, root, files, chunks,
    size_bytes, newest, stale: {changed, added, removed},
    removed?}` and exit 0 when the command ran. The cache dir
    resolves as: `--cache-dir` flag > `CODESEARCH_CACHE_DIR` env >
    `XDG_CACHE_HOME/codesearch-go` > `$HOME/.cache/codesearch-go`.
11. **Failure behaviour.** Unknown flags, conflicting modes, and
    empty single queries exit 2. A query that cannot run exits 1.
    `--callers` / `--related` scan fresh by default (and read the
    synced gob index with `--cached`), printing a proof line to stderr.
12. **Kotlin parity across defs/related/callers.** One shared helper
    (`kotlinASTAndFallbackDefs`) supplies definitions to `--defs-only`,
    `--related`, and `--callers`, so all three agree on names and on
    anchored lines (first non-annotation modifier, else the keyword)
    for declarations the grammar failed to parse (annotation before
    a visibility modifier, one-line layout). `--related` reports those
    declarations and keeps top-level properties exact: a local val
    inside a recovered def's body is inside its byte span and is not
    reported. `--callers` attributes calls inside a recovered def's
    body to that def, and drops the declaration header itself as a
    false call site (a call on a def's start line whose callee is
    that def's name, with no body delimiter `=`/`{` before it).
    A go11 index lacks those `RelDefs`/`CallSites` and is rejected
    by the schema bump, never silently served.

13. **JS/TS/JSX/TSX constructor calls (`new_expression`).** `jsCallQuery`
    captures `(new_expression constructor: (_) @callee) @call` alongside
    `call_expression` across `.js`, `.mjs`, `.cjs`, `.ts`, `.mts`, `.cts`,
    `.tsx`, and `.jsx`, so `new ClassName(...)` and `new ns.ClassName<T>(...)`
    invocations are extracted into `CallSites` and returned by `--callers`
    (fresh and `--cached`). A go12 index lacks constructor `CallSites`
    and is rejected by the schema bump, never silently served.

14. **Rust/Kotlin/JNI related coverage (go14).**
    a. **Callers: right-to-left callee base.** `calleeBase` strips
       balanced `<...>`/`[...]` suffixes from the END of the callee
       expression and takes the trailing identifier run, so indexed
       receivers (`items[0].execute()`) and turbofish (`Vec::<T>::new`)
       resolve to `execute`/`new` instead of the receiver's name. A
       go13 index is unaffected (CallSites are unchanged), but the
       query-side reduction changes, so the bump keeps the pair
       versioned together.
    b. **Rust `--related` definitions and re-exports.** `RelDefs`
       covers non-bindgen Rust symbols (top-level fns, impl methods,
       top-level `const`/`static`); `RelEdges` covers `pub use`
       re-exports (groups, aliases, globs, nested groups, `self`,
       `pub(crate)`; private `use` produces no edge), resolved by
       `resolveRustImport` (`crate::`/`self::`/`super::`/plain,
       `<mod>.rs` then `<mod>/mod.rs`).
    c. **Kotlin regex fallback always merged.** The regex def
       fallback (`kotlinDeclRe`, incl. one level of nested parens in
       annotation arguments) is merged into the AST defs on every
       file, not only on parse failure or empty AST defs, so
       declarations the grammar drops (e.g. `@Composable` before a
       modifier) are definitions in `--defs-only`, `--related`, and
       `--callers` alike.
    d. **JNI bridge (`--related`, two directions).** A Rust fn named
       `Java_<pkg>_<Class>_<method>[__<sig>]` is a `relBridge` with
       `Kind: "jni"` (no attribute needed; methods inside `impl` are
       not JNI entry points), and a Kotlin `external fun` is a
       `relJNIDecl` (`RelJNI` in the gob index) carrying its JNI base
       key, arity, and coarse type tokens per parameter (primitives
       1:1, `String`/primitive arrays fixed, nullable/generic/custom
       classes wildcard). `--related` matches decls to bridges by
       base key (Option 3): a Kotlin method query returns one `ffi`
       row per Rust overload for the base; a Rust JNI-name query
       returns the Kotlin decls for the base, narrowed by the
       signature's arity and coarse tokens when the query carries
       one; survivors that still collide are shown as overload
       candidates, never dropped. A go13 index lacks `RelJNI` and the
       new `RelDefs`/`RelEdges` and is rejected by the schema bump,
       never silently served.

15. **Rust `pub use` & JNI polish (go15).**
    a. **Rust `pub use` comments, `self as alias`, and bare parent/crate/self
       targets.** `parseRustUse` strips `//` line comments and nested
       `/* ... */` block comments before splitting, handles tabs/newlines
       around `as`, and preserves `self as alias` re-export names.
       `resolveRustImport` resolves bare `super` / `super::super` /
       `crate` / `self` targets (checking `<parent>.rs`, `mod.rs`,
       `lib.rs`, `main.rs`) instead of returning empty when no trailing
       module segment follows.
    b. **Kotlin JNI companion & package hygiene.** `@JvmStatic` on an
       `external fun` inside a `companion object` binds its JNI base to
       the enclosing outer class (omitting `$Companion`); named
       companion objects (`companion object Native`) use their declared
       name (`Outer$Native`) when non-`@JvmStatic`. Package extraction
       runs on `stripKotlinCode(src)` so `package ...` lines inside
       leading comments cannot hijack the JNI base key.
    c. **JNI `--related` resolution & explicit wasm bridge kind.**
       Rust-side JNI lookup runs on `calleeBase(query)` (so qualified
       Rust queries like `ffi::Java_...` resolve), preserves the exact
       overloaded Rust symbol in `Detail` (and coarse-filters short
       queries), clarifies direction wording (`bound to`), and dedups
       Kotlin→Rust bridges by `(RustPath, RustLine, RustName)` so
       `#[cfg(...)]` variants in the same file are not dropped.
       `wasm_bindgen` bridges now persist `Kind: "wasm"` explicitly.
       A go14 index carries pre-go15 `RelEdges`/`RelBridges`/`RelJNI`
       and is rejected by the schema bump, never silently served.

16. **Multi-language AST/callers/related calibration (go16).**
    a. **Go generic calls, type aliases, and interface methods.**
       `--callers` captures single-type-argument generic calls
       (`Fn[T](x)` / `pkg.Fn[T](x)`), which `tree-sitter-go` parses as
       `type_conversion_expression` with a `generic_type` target, without
       matching plain scalar type conversions. `--defs-only` and AST
       chunking capture Go `type_alias` (`type A = B`) and interface
       `method_elem` declarations (`DoWork(x int) error`).
    b. **TypeScript abstract declarations.**
       `tsDefQuery` captures `abstract_class_declaration` and
       `abstract_method_signature`, surfacing them in `--defs-only`,
       `--callers` enclosing-caller attribution, and `--related`.
    c. **Rust macros (`macro_rules!` & invocations).**
       `--defs-only` and `--related` capture `macro_definition`
       (`macro_rules!`), and `--callers` captures `macro_invocation`
       (both bare `m!(...)` and scoped `pkg::m!(...)`).
    d. **JS/TS re-export precision & qualified `wasm_bindgen` queries.**
       `export * as NS from './mod'` records a namespace re-export edge
       (`Names: {NS: "*"}`) rather than a wildcard star edge, eliminating
       false-positive top-level re-export rows while resolving `NS` as a
       live namespace re-export. `parseExportStmt`/`parseBraceNames`
       strip JS `//`/`/*...*/` comments (preserving quoted strings),
       handle whitespace/newlines around `type` and `as`, handle local
       `export type { ... }` lists, and record `export default ...` so
       `export { default as X } from './mod'` resolves. `wasm_bindgen`
       lookup in `resolveRelated` runs on `calleeBase(query)` so qualified
       Rust/JS queries (`wasm::parse_doc`, `api.parseDoc`) resolve.
       A `go15` index carries pre-go16 `Chunks`/`CallSites`/`RelDefs`/`RelEdges`
       and is rejected by the schema bump, never silently served.

### What `go16` does NOT guarantee

- **Type resolution.** `--callers` and `--related` remain name-based
  on the trailing identifier; rows are leads to verify against source.
- **JNI overload disambiguation beyond arity + coarse tokens.**
  Two overloads with the same arity whose parameters differ only in
  two non-primitive classes are both shown as candidates. `RegisterNatives`
  and `@JvmName`/`@file:JvmName` are not tracked (v2 scope).
- **Annotated visibility-modified Kotlin `external fun` (e.g.
  `@JvmStatic internal external fun`).** Tree-sitter-kotlin drops
  declarations where an annotation precedes a visibility modifier;
  the regex fallback recovers their def/anchor lines, while JNI
  parameter-type extraction relies on the AST `function_declaration`
  node.

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
   When the engine source changes, its reproducible digest changes
   too: rebuild, take the sha256, and add the pin to
   `PINNED_ENGINE_DIGESTS` in the same commit (the CI pin check
   fails the build otherwise).
4. Push together: engine source + adapter overlay in the same
   commit, so main never holds a mismatched pair.
5. The CI script `scripts/check_engine_contract.sh` fails the
   build whenever the engine's `schemaV` and the adapter's
   `KNOWN_SCHEMA` disagree, and whenever the just-built engine
   binary's digest is not in the adapter's pin list — a mismatch
   is caught at push time, not at runtime in a user's hands. If
   you are reading this because CI went red on that step, the fix
   is step 1, 2, or the pin update in step 3 above, not a change
   to the script.

## History

- `go16` (2026-11-21): Multi-language calibration — Go 1-arg generic
  calls (`type_conversion_expression` with `generic_type`), Go
  `type_alias` & interface `method_elem` defs; TS `abstract class` &
  `abstract` method defs; Rust `macro_rules!` defs & `macro_invocation`
  callers; JS/TS `export * as NS` namespace re-export (eliminating star
  false positives), comment/multiline/type/default export edges;
  qualified `wasm_bindgen` queries via `calleeBase`; `go15` indexes
  rejected and rebuilt. Still current.
- `go15` (2026-11-21): Rust `pub use` comment stripping, `self as alias`,
  and bare `super`/`crate`/`self` import resolution; Kotlin JNI
  `@JvmStatic` companion outer-class binding, named companion objects,
  and comment-safe `package` extraction; JNI `--related` qualified Rust
  queries, coarse-filtered detail, `#[cfg]` same-file line dedup, and
  explicit `Kind: "wasm"` on bindgen bridges; `go14` indexes rejected
  and rebuilt.
- `go14` (2026-11-21): P1-A calleeBase right-to-left (indexed/turbofish
  receivers), P1-B Rust `--related` defs + `pub use` edges, P1-C Kotlin
  regex fallback always merged (incl. one level of nested parens in
  annotation arguments), P2 JNI bridge two directions (base-key
  grouping, arity + coarse-token narrowing, fail-open candidates) via
  persisted `RelJNI` + `Kind: "jni"` bridges; `go13` indexes rejected
  and rebuilt.
- `go13` (2026-11-20): JS/TS/JSX/TSX constructor invocations
  (`new_expression`) captured in `CallSites` and `--callers`
  (fresh and `--cached`) via `jsCallQuery`; `go12` indexes rejected and
  rebuilt.
- `go12` (2026-11-20): Kotlin parity — `--related` and `--callers`
  share the AST+regex def helper with `--defs-only` (anchored lines,
  byte spans incl. regex-recovered defs); decl-header false call
  sites filtered; local vals inside recovered defs no longer leak as
  top-level properties.
- `go11` (2026-11-14): `CallSites` + `FileMeta.Generated` persisted
  for `--cached --callers`/`--related`; `filterDefChunks` excludes
  AST `"window"` chunks, `.md`/`.txt`/`.rst`/`.adoc`, `.d.ts`/`.d.mts`/
  `.d.cts`, and generated/minified assets; `isGeneratedOrMinified`
  covers `@ts-self-types`, `wasm-bindgen` internals, `public/assets/`,
  and declaration files; `defaultExts` adds `.cjs`/`.jsx`/`.mts`/`.cts`.
- `go10` (2026-11-13): TS interface members (`method_signature`)
  are definitions (chunking, callers, related via `tsDefQuery`);
  go9 indexes rejected and rebuilt.
- `go9` (2026-10-26): `FileMeta.Skip` persisted (oversize/binary/
  overchars); batch recommendation 2–6.
- G8 (2026-11-10): batch `--callers`/`--related` + `symbol` field
  on related rows, same schema (additive, no bump).
- G9 (2026-11-10): `--cache-dir` + `--index-status`/`--index-warm`/
  `--index-clean`, same schema (additive, no bump).
- G2 (2026-11-11): reproducible `build.sh` (`-trimpath`,
  `-buildvcs=false`, no wall-clock `built`) + adapter digest pin
  (`PINNED_ENGINE_DIGESTS`: reproducible build + v0.2.5 release
  asset) + `CODESEARCH_ALLOW_UNPINNED=1` opt-out, same schema.
