#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

apply_b64_overlay() {
  local overlay_name="$1"
  local overlay_dir="runtime_overlay/$overlay_name"
  local manifest="$overlay_dir/OVERLAY_SHA256"

  if [[ ! -f "$manifest" ]]; then
    echo "RUNTIME_OVERLAY_${overlay_name}: none"
    return 0
  fi

  local tmp_b64 tmp_patch expected actual
  tmp_b64="$(mktemp)"
  tmp_patch="$(mktemp)"
  cat "$overlay_dir"/part*.b64 > "$tmp_b64"
  tr -d '\r\n' < "$tmp_b64" | base64 --decode > "$tmp_patch"

  expected="$(awk '{print $1}' "$manifest")"
  actual="$(sha256sum "$tmp_patch" | awk '{print $1}')"
  if [[ "$actual" != "$expected" ]]; then
    echo "runtime overlay digest mismatch: $overlay_name" >&2
    echo "expected=$expected" >&2
    echo "actual=$actual" >&2
    rm -f "$tmp_b64" "$tmp_patch"
    exit 1
  fi

  patch --dry-run --batch --forward -p1 < "$tmp_patch" >/dev/null
  patch --batch --forward -p1 < "$tmp_patch" >/dev/null
  rm -f "$tmp_b64" "$tmp_patch"
  echo "RUNTIME_OVERLAY_${overlay_name}: PASS sha256:$actual"
}

apply_reviewable_overlay() {
  local overlay_name="$1"
  local overlay_dir="runtime_overlay/$overlay_name"
  local patch_file="$overlay_dir/change.patch"
  local manifest="$overlay_dir/PATCH_SHA256"
  if [[ ! -f "$patch_file" || ! -f "$manifest" ]]; then
    echo "reviewable overlay missing: $overlay_name" >&2
    exit 2
  fi
  local expected actual
  expected="$(awk '{print $1}' "$manifest")"
  actual="$(sha256sum "$patch_file" | awk '{print $1}')"
  if [[ "$actual" != "$expected" ]]; then
    echo "runtime overlay digest mismatch: $overlay_name" >&2
    exit 1
  fi
  patch --dry-run --batch --forward -p1 < "$patch_file" >/dev/null
  patch --batch --forward -p1 < "$patch_file" >/dev/null
  echo "RUNTIME_OVERLAY_${overlay_name}: PASS sha256:$actual"
}

# --- Incremental overlays on top of the v0.2.41 baked capsule ---
# All 95 historical overlays through v0.2.41 (js_ts_v2 .. engine_pin_v15)
# are baked into runtime_source/plw-runtime-src.tar.gz.
# Append new apply_reviewable_overlay "<name>" entries below as needed.

apply_reviewable_overlay "plw_cli_modular_v1"
grep -Fq "_CLI_COMMAND_DISPATCH" plw_cli.py
grep -Fq "from core.cli.common import" plw_cli.py
python3 -m py_compile plw_cli.py core/cli/*.py

apply_reviewable_overlay "engine_pin_v16"
grep -Fq "b47b9f9f1ca432f161af8fa09bd8d6350755aab5acb65d9987a9f81e1f6d9487" core/codesearch_engine.py
python3 -m py_compile core/codesearch_engine.py

apply_reviewable_overlay "solid_architecture_v3"
grep -Fq "from core.kernel.inspection import" hott_kernel.py
grep -Fq "from core.kernel.work_policy import" hott_kernel.py
grep -Fq "from core.simplification.js_frontend import" core/simplification_analyzer.py
python3 -m py_compile hott_kernel.py core/kernel/*.py core/simplification_analyzer.py core/simplification/*.py

apply_reviewable_overlay "engine_state_v8"
grep -Fq 'KNOWN_SCHEMA = "go16"' core/codesearch_engine.py
grep -Fq '"go16": "plw-codesearch-engine-v1"' core/codesearch_engine.py
python3 -m py_compile core/codesearch_engine.py

apply_reviewable_overlay "engine_pin_v17"
grep -Fq "4ace319063b87e3c120ad5d971ea2aa12e0a3c94f009550ca48193982bdb78b3" core/codesearch_engine.py
python3 -m py_compile core/codesearch_engine.py

apply_reviewable_overlay "calibration_py_topo_v1"
grep -Fq "_go_grouped_type_names" core/topology_engine.py
grep -Fq "_SEM_MULTI_ANGLE_TYPE_REGEX" core/shared_graph.py
grep -Fq "annotated_assignment" core/simplification/python_frontend.py
python3 -m py_compile core/topology_engine.py core/shared_graph.py core/simplification/python_frontend.py

apply_reviewable_overlay "kotlin_extension_attribution_v1"
grep -Fq "_kotlin_receiver_type" core/shared_graph.py
grep -Fq "kotlin_extension_declarations" language_semantics.py
grep -Fq "variant_shadow_suppressed" core/analyzer_registry.py
python3 -m py_compile language_semantics.py core/shared_graph.py core/analyzer_registry.py

apply_reviewable_overlay "calibration_rust_kt_topo_v1"
grep -Fq "def non_cyclic_rust_edges(" language_semantics.py
grep -Fq "def rust_has_inline_tests(" language_semantics.py
grep -Fq "def rust_cargo_bin_exe_refs(" language_semantics.py
grep -Fq "rust_facade_reexport" core/shared_graph.py
python3 -m py_compile language_semantics.py core/shared_graph.py core/target_topology.py codebase/hott_analyzers.py codebase/cycle_break.py codebase/topology_analyzers.py codebase/performance_analyzers.py

apply_reviewable_overlay "project_calibration_v1"
grep -Fq "CALIBRATION_SCHEMA_VERSION = \"plw-project-calibration-v1\"" core/project_calibration.py
grep -Fq "def cmd_calibrate(" core/cli/calibration_cmds.py
grep -Fq "FORBIDDEN_SUPPRESS_ANALYZERS" core/project_calibration.py
grep -Fq "project_calibration_valid" core/portable_agent.py
python3 -m py_compile core/project_calibration.py core/cli/calibration_cmds.py plw_cli.py core/shared_graph.py core/graph_cache.py core/analyzer_registry.py core/portable_agent.py

apply_reviewable_overlay "project_calibration_v2"
grep -Fq "BRIDGE_ENTRYPOINT_KINDS" core/project_calibration.py
grep -Fq "\"review_hints\"" core/project_calibration.py
grep -Fq "Optional review hints" core/cli/calibration_cmds.py
python3 -m py_compile core/project_calibration.py core/cli/calibration_cmds.py

echo "RUNTIME_OVERLAYS_APPLIED: 10 (plw_cli_modular_v1, engine_pin_v16, solid_architecture_v3, engine_state_v8, engine_pin_v17, calibration_py_topo_v1, kotlin_extension_attribution_v1, calibration_rust_kt_topo_v1, project_calibration_v1, project_calibration_v2 on baked capsule v0.2.41)"

# Post-unpack invariant checks on the baked runtime baseline
grep -Fq 'plw-portable-agent-v1' core/portable_agent.py
grep -Fq 'candidate-selection-provenance-v1' core/candidate_provenance.py
grep -Fq 'def _non_cyclic_rust_mod_edges(' codebase/cycle_break.py
grep -Fq '"optional_type_only_cuts"' codebase/cycle_break.py
grep -Fq '"type_only_cyclic_sccs"' core/target_topology.py
grep -Fq 'shlex.quote(' codebase/mermaid_graph.py
grep -Fq 'KNOWN_SCHEMA = "go16"' core/codesearch_engine.py
grep -Fq '"go16": "plw-codesearch-engine-v1"' core/codesearch_engine.py
grep -Fq '4ace319063b87e3c120ad5d971ea2aa12e0a3c94f009550ca48193982bdb78b3' core/codesearch_engine.py
grep -Fq 'plw-project-calibration-v1' core/project_calibration.py
grep -Fq '"calibrate"' plw_cli.py

python3 -m py_compile core/operation_plan.py
python3 - <<'PY'
from core.operation_plan import resolve_operation_plan

exact = resolve_operation_plan({
    "schema_version": "plw-operation-plan-v1",
    "capability": "candidate",
    "operation_id": "candidate.select.v1",
    "inputs": {"task": "Select isOne in src/guards.js", "root": "."},
})
assert exact["status"] == "RESOLVED", exact
assert exact["reason"] == "EXACT_OPERATION_BINDING_RESOLVED", exact
assert exact["next_gate"] == "AGENT_MAY_CHOOSE_INVOCATION", exact
assert exact["authority"]["invocation_authorized"] is False, exact
assert exact["executes_operation"] is False, exact

ambiguous = resolve_operation_plan({
    "schema_version": "plw-operation-plan-v1",
    "capability": "candidate",
    "inputs": {"task": "Select isOne in src/guards.js", "root": "."},
})
assert ambiguous["status"] == "AMBIGUOUS", ambiguous
assert ambiguous["next_gate"] == "SELECT_EXACT_OPERATION_ID", ambiguous
assert ambiguous["authority"]["invocation_authorized"] is False, ambiguous

rejected = resolve_operation_plan({
    "schema_version": "unsupported",
    "capability": "candidate",
    "inputs": {},
})
assert rejected["status"] == "REJECTED", rejected
assert rejected["reason"] == "PLAN_SCHEMA_UNSUPPORTED", rejected
assert rejected["authority"]["invocation_authorized"] is False, rejected

print("RUNTIME_OPERATION_PLAN_PREFLIGHT: PASS")
PY
