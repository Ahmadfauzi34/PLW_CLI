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

The `codesearch_engine_v1` readable, digest-pinned overlay adds the codesearch-go tree-sitter engine as an external binary (never bundled): `plw ast` defs/callers/related/search with a fail-closed schema gate, and `plw doctor` engine status. The `codesearch_engine_v2` overlay builds on it: `plw agent orient` executes the dynamic context matrix (context A/B from target languages plus engine availability, with routes — never from the task text). Its standalone smoke checks pure-Go, TS-only, and mixed targets with and without the engine. The `topology_engine_v1` overlay builds on both: `plw topology` (and `plw impact`, separately) gains an engine symbol layer — AST-verified definition nodes plus name-based lead call/relation edges, kept apart from the import graph — with target-grounded scope and an explicit unavailable block when the engine is missing. Its standalone smoke checks pure-Go, TS-only, mixed, same-name, and cache-hit targets with and without the engine. The `batch_engine_v1` overlay builds on it: the engine answers comma-separated `--callers`/`--related` names and multi-query `--defs-only` in one scan, the adapter exposes `run_batch_query` (falling back to one query per symbol on a pre-batch engine, recorded as `batch_mode: fallback`), and the topology layer drops its 20-symbol cap to whole-repo scope. Its standalone smoke checks batch `ast` calls and whole-repo topology scope. The `engine_state_v1` overlay builds on it: the engine's gob index lives under PLW's cache root (overridable with `--cache-dir` or `CODESEARCH_CACHE_DIR`), `plw ast index` reports/builds/deletes it, `plw ast doctor` shows where it lives, and the engine contract (`codesearch-go/ENGINE_CONTRACT.md`) plus `scripts/check_engine_contract.sh` make the schema agreement machine-checked. Its standalone smoke checks index status/warm/clean, staleness, doctor, and the contract check. The `engine_pin_v1` overlay builds on it: the engine binary is verified by SHA256 against `PINNED_ENGINE_DIGESTS` (reproducible `build.sh` output plus the v0.2.5 release asset), with the explicit opt-out `CODESEARCH_ALLOW_UNPINNED=1` recorded as `pinned: false`; `build.sh` itself is reproducible (`-trimpath`, `-buildvcs=false`, no wall-clock build date). Its standalone smoke is the digest-pin battery case (stub rejected, tampered binary rejected, old engine still pinned), locked in-repo by `tests/test_codesearch_engine.py` (T2/T3/T5) and the CI "Check engine contract" step.

Overlays are a transport/build mechanism only. They are never required beside the released `plw` executable and are never copied into target repositories.

The long-term repository cleanup is to normalize the runtime source into ordinary browsable Git files after the binary contract is stable.
