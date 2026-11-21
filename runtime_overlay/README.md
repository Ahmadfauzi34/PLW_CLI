# Runtime Overlays

Verified, deterministic runtime patches applied on top of `runtime_source/plw-runtime-src.tar.gz` by `scripts/apply_runtime_overlays.sh`.

## Baked Capsule Checkpoint (`v0.2.41`)

All **95** historical overlays accumulated through `v0.2.41` (base commit `dc15d8e`) were baked into a fresh, deterministic base capsule with verified byte-for-byte parity across all **162** unpacked runtime files:

- **Previous base capsule SHA-256** (149 files): `58084d32ea5afe395d57ad876e171942ede6473647c5222952d6448392c743fa`
- **Current baked base capsule SHA-256** (162 files): `8847c999756832bab5b03db18969cb96c65c360c3bb8908b239658eaf8af729b`
- **Active incremental overlays**: `5`
  - `plw_cli_modular_v1` (`ef7d29ee7925a59f24627286fc36f7719c24a3cb708a6143719fb15d2c901cf2`): decomposes `plw_cli.py` (6,014 -> 391 lines) into 10 acyclic domain modules + `common.py` + `help_text.py` under `core/cli/`, preserving 100% backward-compatible re-exports on `plw_cli`, declarative `_CLI_COMMAND_DISPATCH` parity (184 aliases), and `cyclic_sccs == 0`.
  - `engine_pin_v16` (`e31bc014faf6020211375da357eb00852982d360709f0326f3b4e2da59966f54`): rotates reproducible `go15` engine binary digest in `core/codesearch_engine.py` to `b47b9f9f1ca432f161af8fa09bd8d6350755aab5acb65d9987a9f81e1f6d9487` (`commit=2c99d505c774`) after splitting `codesearch-go/related.go` per language (`related.go`, `related_jsts.go`, `related_kotlin.go`, `related_rust.go`, `related_jni.go`).
  - `solid_architecture_v3` (`bb31ed910d9483992855441070685928257ffc4742bc43feb4da4cca6b7d7642`): decomposes `hott_kernel.py` (5,783 -> 311 lines) into 8 acyclic modules under `core/kernel/` separating core inspection (`inspection.py`) from work/policy orchestration (`work_policy.py`, `context_pack.py`, `task_chain.py`, `receipts.py`), memory/fibration (`memory_ops.py`), runtime (`runtime.py`), and CLI dispatch (`cli_dispatch.py`); and decomposes `core/simplification_analyzer.py` (3,448 -> 549 lines) into 5 acyclic modules under `core/simplification/` (`common.py`, `js_frontend.py`, `brace_frontend.py`, `python_frontend.py`, `adjudication.py`), preserving 100% symbol re-export parity and `cyclic_sccs == 0`.
  - `engine_state_v8` (`adeb84ba40d7ef54d7fe3f3ff31888e9dca0df23e942b0bea7dd337242a5d98c`): bumps `KNOWN_SCHEMA` to `"go16"` and registers `"go16": "plw-codesearch-engine-v1"` in `SUPPORTED_SCHEMAS` in `core/codesearch_engine.py`.
  - `engine_pin_v17` (`f984f4087d7adb464f1daf6b5df68d9e5df44ce4c6c2abe2f7f1b49aab4355d2`): rotates reproducible `go16` engine binary digest in `core/codesearch_engine.py` to `4ace319063b87e3c120ad5d971ea2aa12e0a3c94f009550ca48193982bdb78b3` (`commit=914305a51a89`) after multi-language AST/callers/related calibration.
- **Full per-overlay history & patches**: preserved in Git history at commit `dc15d8e` (`v0.2.41`).

## Adding a Future Incremental Overlay

1. Create `runtime_overlay/<overlay_name>/change.patch` (unified diff `-p1` against the unpacked runtime tree) and `runtime_overlay/<overlay_name>/PATCH_SHA256` (`sha256sum change.patch`).
2. Register `apply_reviewable_overlay "<overlay_name>"` in `scripts/apply_runtime_overlays.sh` plus targeted post-apply `grep` sanity checks.
3. Note: `scripts/check_engine_contract.sh` automatically resolves `core/codesearch_engine.py` from the verified capsule plus any active overlays touching `core/codesearch_engine.py`, so future `engine_state_*` / `engine_pin_*` overlays work without editing `check_engine_contract.sh`.

## Baked Overlays Manifest (`v0.2.41`, 95 overlays in application order)

| # | Format | Overlay Name |
|---:|---|---|
| 1 | `b64` | `js_ts_v2` |
| 2 | `b64` | `portable_agent_v1` |
| 3 | `b64` | `validation_runtime_cli_v1` |
| 4 | `b64` | `portable_topology_scope_v1` |
| 5 | `b64` | `candidate_selection_v1` |
| 6 | `b64` | `shared_graph_determinism_v1` |
| 7 | `b64` | `candidate_agent_handoff_v1` |
| 8 | `b64` | `candidate_affordance_projection_v1` |
| 9 | `b64` | `operation_contract_reference_v1` |
| 10 | `b64` | `operation_plan_preflight_v1` |
| 11 | `b64` | `operation_plan_preflight_repair_v1` |
| 12 | `reviewable` | `invocation_snapshot_v1` |
| 13 | `reviewable` | `readiness_version_v1` |
| 14 | `reviewable` | `browserless_ui_reference_v1` |
| 15 | `reviewable` | `ui_source_precision_v1` |
| 16 | `reviewable` | `ui_render_conditions_v1` |
| 17 | `reviewable` | `ui_style_reference_v1` |
| 18 | `reviewable` | `ui_agent_handoff_v1` |
| 19 | `reviewable` | `codesearch_engine_v1` |
| 20 | `reviewable` | `codesearch_engine_v2` |
| 21 | `reviewable` | `topology_engine_v1` |
| 22 | `reviewable` | `batch_engine_v1` |
| 23 | `reviewable` | `engine_state_v1` |
| 24 | `reviewable` | `engine_pin_v1` |
| 25 | `reviewable` | `engine_pin_v2` |
| 26 | `reviewable` | `engine_pin_v3` |
| 27 | `reviewable` | `engine_pin_v4` |
| 28 | `reviewable` | `engine_pin_v5` |
| 29 | `reviewable` | `engine_pin_v6` |
| 30 | `reviewable` | `engine_state_v2` |
| 31 | `reviewable` | `engine_pin_v7` |
| 32 | `reviewable` | `engine_pin_v8` |
| 33 | `reviewable` | `discovery_v1` |
| 34 | `reviewable` | `limit_v1` |
| 35 | `reviewable` | `brokenpipe_v1` |
| 36 | `reviewable` | `topology_honesty_v1` |
| 37 | `reviewable` | `kotlin_topology_v1` |
| 38 | `reviewable` | `agent_friendly_v1` |
| 39 | `reviewable` | `root_flag_v1` |
| 40 | `reviewable` | `receipt_bias_v1` |
| 41 | `reviewable` | `namespace_alias_v1` |
| 42 | `reviewable` | `namespace_alias_help_v1` |
| 43 | `reviewable` | `registry_alias_hint_v1` |
| 44 | `reviewable` | `help_exit_codes_v1` |
| 45 | `reviewable` | `ast_help_root_first_v1` |
| 46 | `reviewable` | `orient_engine_answer_v1` |
| 47 | `reviewable` | `doctor_ready_scope_v1` |
| 48 | `reviewable` | `describe_alias_human_v1` |
| 49 | `reviewable` | `describe_group_op_alias_v1` |
| 50 | `reviewable` | `describe_first_contact_v1` |
| 51 | `reviewable` | `topology_files_scope_v1` |
| 52 | `reviewable` | `ledger_memory_disambiguation_v1` |
| 53 | `reviewable` | `doctor_engine_required_note_v1` |
| 54 | `reviewable` | `rust_topology_v1` |
| 55 | `reviewable` | `engine_answer_bridge_v1` |
| 56 | `reviewable` | `silent_wrong_answers_v1` |
| 57 | `reviewable` | `remove_install_v1` |
| 58 | `reviewable` | `parser_fail_closed_v1` |
| 59 | `reviewable` | `adapter_build_v1` |
| 60 | `reviewable` | `simplify_surface_v1` |
| 61 | `reviewable` | `simplify_rs_kt_v1` |
| 62 | `reviewable` | `deopt_lang_gate_v1` |
| 63 | `reviewable` | `ts_js_ext_map_v1` |
| 64 | `reviewable` | `kotlin_label_v1` |
| 65 | `reviewable` | `android_manifest_entrypoint_v1` |
| 66 | `reviewable` | `check_fair_envelope_v1` |
| 67 | `reviewable` | `simplify_precision_v1` |
| 68 | `reviewable` | `topology_engine_v2` |
| 69 | `reviewable` | `engine_state_v3` |
| 70 | `reviewable` | `engine_pin_v9` |
| 71 | `reviewable` | `engine_pin_v10` |
| 72 | `reviewable` | `engine_pin_v11` |
| 73 | `reviewable` | `engine_state_v4` |
| 74 | `reviewable` | `engine_pin_v12` |
| 75 | `reviewable` | `topology_resolution_v1` |
| 76 | `reviewable` | `check_signal_hardening_v1` |
| 77 | `reviewable` | `receipt_lease_hardening_v1` |
| 78 | `reviewable` | `mermaid_graph_v1` |
| 79 | `reviewable` | `product_polish_v1` |
| 80 | `reviewable` | `agent_ux_hardening_v1` |
| 81 | `reviewable` | `agent_ux_hardening_v2` |
| 82 | `reviewable` | `python_topology_v1` |
| 83 | `reviewable` | `solid_architecture_v1` |
| 84 | `reviewable` | `solid_architecture_v2` |
| 85 | `reviewable` | `calibration_ux_consolidated_v1` |
| 86 | `reviewable` | `topo_circular_opt_v1` |
| 87 | `reviewable` | `kotlin_rust_calibration_v1` |
| 88 | `reviewable` | `rust_mod_cycle_calibration_v1` |
| 89 | `reviewable` | `engine_state_v5` |
| 90 | `reviewable` | `engine_pin_v13` |
| 91 | `reviewable` | `engine_state_v6` |
| 92 | `reviewable` | `engine_pin_v14` |
| 93 | `reviewable` | `type_only_downstream_consistency_v1` |
| 94 | `reviewable` | `engine_state_v7` |
| 95 | `reviewable` | `engine_pin_v15` |
