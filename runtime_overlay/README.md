# Runtime overlays

PLW's current distribution repository keeps the validated v0.1.0 runtime source as a digest-pinned capsule.

Small validated runtime updates are layered on top as digest-pinned overlays before source checks and binary packaging.

An overlay must:

1. live in its own directory;
2. declare the SHA-256 of the reconstructed patch;
3. reconstruct deterministically from committed chunks;
4. pass a dry-run before application;
5. apply only to a freshly unpacked verified capsule;
6. be covered by standalone binary smoke tests.

The `invocation_snapshot_v1` overlay keeps its patch in readable `change.patch`
and pins those exact bytes with `PATCH_SHA256`. The build verifies the digest,
dry-runs the patch, then applies it after the earlier ordered overlays. This
format makes the current invocation and snapshot changes directly reviewable.

The `readiness_version_v1` overlay follows the same readable, digest-pinned
patch format and separates reference-pack readiness from target topology and
action readiness. It also adds direct embedded version discovery.

The `browserless_ui_reference_v1` readable, digest-pinned overlay adds a
source-only Angular UI lookup and embedded skill. It never supplies geometry
or a runtime observation. Its binary smoke checks ambiguous targets, template
digests, inline templates, unavailable styles and explicit authority flags.

The `ui_source_precision_v1` readable, digest-pinned overlay keeps template
text with its immediate element or nearest interactive control, and exposes
the exact label/attribute terms and source ranking policy used for selection.
Its standalone smoke checks nested controls and ambiguous owners.

The `ui_render_conditions_v1` overlay adds source-only containment witnesses
for Angular control-flow blocks and inherited structural guards. Unsupported
or malformed control flow remains unresolved; route, state, and rendered
presence require external witnesses.

The `ui_style_reference_v1` overlay maps exact Angular CSS resources to bounded
selector-rule candidates. Missing global or component CSS, imports and
unsupported source syntax keep the style status unknown; no cascade or media
activation is inferred.

Candidate-selection updates must also bind exact source sites to the full
discovery candidate-set digest. Internal candidate-capability matching may
return `CAPABILITY_MATCHED`; it must not expose the mutation capability through
the public capability registry or grant authorization, execution, mutation,
correctness, or evidence-acceptance authority.

Shared-graph semantic-usage lists are sorted before hashing so candidate
discovery snapshots remain reproducible across separate Python processes.

The `codesearch_engine_v1` readable, digest-pinned overlay adds the codesearch-go tree-sitter engine as an external binary (never bundled): `plw ast` defs/callers/related/search with a fail-closed schema gate, and `plw doctor` engine status. The `codesearch_engine_v2` overlay builds on it: `plw agent orient` executes the dynamic context matrix (context A/B from target languages plus engine availability, with routes — never from the task text). Its standalone smoke checks pure-Go, TS-only, and mixed targets with and without the engine. The `topology_engine_v1` overlay builds on both: `plw topology` (and `plw impact`, separately) gains an engine symbol layer — AST-verified definition nodes plus name-based lead call/relation edges, kept apart from the import graph — with target-grounded scope and an explicit unavailable block when the engine is missing. Its standalone smoke checks pure-Go, TS-only, mixed, same-name, and cache-hit targets with and without the engine. The `batch_engine_v1` overlay builds on it: the engine answers comma-separated `--callers`/`--related` names and multi-query `--defs-only` in one scan, the adapter exposes `run_batch_query` (falling back to one query per symbol on a pre-batch engine, recorded as `batch_mode: fallback`), and the topology layer drops its 20-symbol cap to whole-repo scope. Its standalone smoke checks batch `ast` calls and whole-repo topology scope. The `engine_state_v1` overlay builds on it: the engine's gob index lives under PLW's cache root (overridable with `--cache-dir` or `CODESEARCH_CACHE_DIR`), `plw ast index` reports/builds/deletes it, `plw ast doctor` shows where it lives, and the engine contract (`codesearch-go/ENGINE_CONTRACT.md`) plus `scripts/check_engine_contract.sh` make the schema agreement machine-checked. Its standalone smoke checks index status/warm/clean, staleness, doctor, and the contract check. The `engine_pin_v1` overlay builds on it: the engine binary is verified by SHA256 against `PINNED_ENGINE_DIGESTS` (reproducible `build.sh` output plus the v0.2.5 release asset), with the explicit opt-out `CODESEARCH_ALLOW_UNPINNED=1` recorded as `pinned: false`; `build.sh` itself is reproducible (`-trimpath`, `-buildvcs=false`, no wall-clock build date). Its standalone smoke is the digest-pin battery case (stub rejected, tampered binary rejected, old engine still pinned), locked in-repo by `tests/test_codesearch_engine.py` (T2/T3/T5) and the CI "Check engine contract" step. The `engine_pin_v2` overlay rotates the pin to the F2 engine (reproducible `build.sh` digest `f7ef3778…`, commit `05c7fa3a8e63`): the superseded reproducible-build digest leaves the pin, the v0.2.5 release-asset entry stays; `scripts/check_engine_contract.sh` now reads the pin from `engine_pin_v2/change.patch`. The `engine_pin_v3` overlay rotates the pin to the Kotlin engine K1–K4 (reproducible `build.sh` digest `c251454d…`, commit `8da8dea19350`, schema still go9): the F2 reproducible-build digest leaves the pin, the v0.2.5 release-asset entry stays; it also adds `"kotlin"` to the adapter's `TREE_SITTER_LANGUAGES` so `plw ast doctor` advertises the new language; `scripts/check_engine_contract.sh` now reads the pin from `engine_pin_v3/change.patch`.

The `discovery_v1` overlay builds on it: PATH discovery tries `codesearch`, then `codesearch-go`, then `codesearch-go-linux-x86_64` in order, so a release-asset binary works without a symlink or `CODESEARCH_BIN`; the bare name stays first so existing setups keep working, and explicit paths plus `CODESEARCH_BIN` still name an exact file. Its tests are the discovery cases in `tests/test_codesearch_engine.py` (candidate order, first-match-wins, error naming all three candidates).

The `limit_v1` overlay builds on it: `--limit` is enforced by the adapter, not the engine (except `search`, where it passes through to the engine's own `--limit`) — `run_query` and `run_batch_query` slice every row list (`hits`/`calls`/`related`, and every `grouped` list in a batch) to at most `limit` rows per list; the flat batch list is filtered per symbol so it stays the union of `grouped`; `summary.total` is the count before slicing and `summary.shown` the count after; a negative limit is rejected. `cmd_ast` passes `--limit` to the batch path too, and the non-JSON header prints `# Shown:` when known. Its tests are the F3 cases in `tests/test_codesearch_engine.py` (single-query related/callers/defs, batch + fallback, limit 0, negative rejection).

The `brokenpipe_v1` overlay: `main()` in `plw_cli.py` restores `SIGPIPE` to `SIG_DFL` at startup (guarded by `hasattr`, so Windows is unaffected) — piping any long JSON output to `head` then dies silently with exit 141, the Unix-standard death, instead of a `BrokenPipeError` traceback (previously reproduced at `cmd_agent`, `plw_cli.py:4668`). Its tests are `test_brokenpipe_battery.py` (B1 orient pipe, B2 full run, B3 topology pipe).

The `topology_honesty_v1` overlay: (1) `_classify_archetype` in `codebase/hott_analyzers.py` takes the directed-cycle count (`cycle_orientation_counts["directed"]`, computed in the same function) — a graph with β₁ > 0 but 0 directed cycles is now `reconvergent_dag` / `fragmented_reconvergent_dag` ("Connected DAG with N reconvergent path(s), no directed cycles") instead of `mixed_cyclic`, because β₁ counts cycles of the underlying *undirected* graph and every diamond contributes one; (2) `steer_decoder` in `core/synthesizer.py` maps those labels to strategy `hub_stabilization_and_contract_hardening` (the names contain no "cyclic", so the old `cycle_breaking_and_layering` branch is never reached for them); (3) the `diamond_dependency` detector in `analyze_homotopy` emits one finding per (source, convergence) group with `intermediate_nodes` + `diamond_count` (46 → 5 groups on the reference project), skips barrel (`index.*`) and test (`*.spec.*`, `*.test.*`) sources, and defaults to severity `info` — escalating to `medium` only when the convergence file exports mutable shared state (`export let`/`var`, exported `new` instance, exported object/array literal), checked mechanically against `file_map`. Its tests are `test_topology_honesty_battery.py` (B1 DAG label + strategy, B2 directed cycle still cyclic, B3 grouping + barrel exclusion + info, B4 mutable-convergence falsifier, B5 standalone end-to-end: 5 info groups, health 0.299).

The `kotlin_topology_v1` overlay (K5+K6): Kotlin joins the shared graph. (1) `language_semantics.py` gains `KOTLIN_SOURCE_EXTENSIONS = (".kt",)` in the default `SOURCE_EXTENSIONS` (`.kts` Gradle scripts stay out), `source_language` returns `"kotlin"` for `.kt`, and a stdlib regex frontend `parse_kotlin_imports` (`import a.b.C`, `import a.b.C as D`, `import a.b.*`; nature always `runtime` — Kotlin has no `import type`) plus `kotlin_source_root` (source root derived per file from path minus package path), `kotlin_top_level_names` (column-0 declarations, tolerant of annotations, modifiers, type parameters, nullable/generic receivers), and `kotlin_module_candidates` (FQN-as-path under each root; no barrel/index files). (2) `core/shared_graph.py` dispatches `.kt` files to that frontend, builds a PASS 1.5 package/symbol index `(package, name) -> file` from the contents PASS 1 already read, and resolves Kotlin imports in PASS 3 in two stages — FQN-as-path, then the symbol index (so types declared in a differently-named file still resolve; FQN-as-path alone was measured at only 37% on the OOPBoT corpus, the two stages reach 1473/1602 resolved) — with classified outcomes instead of silent drops: `<pkg>.R` is `kotlin_generated` external (Android generated R class), foreign packages are `kotlin_external`, a known package that does not resolve is `kotlin_unresolved` (2 cases on OOPBoT, both AIDL-generated interfaces). (3) Test detection takes the full path now: `*Test.kt` or under a `src/test` source set (69/69 on OOPBoT). (4) The portable-agent topology scope string becomes `stable_shared_graph (.ts/.tsx/.js/.jsx/.kt)`. Deliberately unchanged: `codebase/file_scanner.py` (legacy, no callers), `_classify_node`, entrypoint rules, `.kts`, and the JNI bridge (K4 follow-up). Its tests are `test_k5_k6_battery.py` (B1 parse forms, B2 declaration index, B3 roots/candidates/classification, B4 fixture graph, B5 oopbot-trial end-to-end + independent accounting identity 1473+127+2 == 1602, B6 standalone regression byte-identical, B7 unresolved falsifier).

Overlays are a transport/build mechanism only. They are never required beside the released `plw` executable and are never copied into target repositories.

The long-term repository cleanup is to normalize the runtime source into ordinary browsable Git files after the binary contract is stable.

The `agent_friendly_v1` overlay (Fase A, agent misunderstandings are the tool's fault): (1) `hott_kernel.py` `kernel_analyze` summary mode now carries a bounded `findings` list (critical/error/high/medium, each tagged with its analyzer, deterministic order, cap 100) plus `findings_truncated` — the envelope used to carry only counts, so an agent had to open the analyzer cache file to read one finding; low/info stay in the cache. The same fix repairs the human path: `_format_check_output` used to read a `findings` key the summary never provided, so it always printed PASS and its note lied ("41 low/info" while 18 were medium); the note now lists real per-severity counts. (2) `plw_cli.py` `cmd_ast` accepts query-first (`plw ast callers <name> [root]`), legacy root-first (two positionals still mean root-first; a single positional naming an existing directory still means root), and `--root DIR`; `doctor`/`index` keep the legacy order; a missing query now errors with a teaching message. (3) The main `--help` is grouped (Orientation / Analysis / UI / Governance / Install) and teaches: a "Start here" block, the task-pipeline order (`orient -> resolve-target -> evidence-needs -> capability-match -> candidate select`, semantic route as side-branch), the F-stage order (F111 -> F121 -> F122 -> F123 -> F127 -> F128 -> F129 -> F131/F132/F133 -> F139 -> F141 -> F142), and plain-language descriptions for previously bare governance commands. No command was renamed or removed (78 command lines, distinct names unchanged). Its tests are `test_agent_friendly_battery.py` (30 checks: A1 findings equal the cache's actionable multiset on two fixtures incl. oopbot's 1033 actionable findings vs the 100 cap; A2 unit + live three-form equivalence; A3 help structure + no command lost).
