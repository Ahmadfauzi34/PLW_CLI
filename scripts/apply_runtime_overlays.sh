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

echo "RUNTIME_OVERLAYS_APPLIED: 2 (plw_cli_modular_v1, engine_pin_v16 on baked capsule v0.2.41)"

# Post-unpack invariant checks on the baked runtime baseline
grep -Fq 'plw-portable-agent-v1' core/portable_agent.py
grep -Fq 'candidate-selection-provenance-v1' core/candidate_provenance.py
grep -Fq 'def _non_cyclic_rust_mod_edges(' codebase/cycle_break.py
grep -Fq '"optional_type_only_cuts"' codebase/cycle_break.py
grep -Fq '"type_only_cyclic_sccs"' core/target_topology.py
grep -Fq 'shlex.quote(' codebase/mermaid_graph.py
grep -Fq 'KNOWN_SCHEMA = "go15"' core/codesearch_engine.py
grep -Fq '"go15": "plw-codesearch-engine-v1"' core/codesearch_engine.py
grep -Fq 'b47b9f9f1ca432f161af8fa09bd8d6350755aab5acb65d9987a9f81e1f6d9487' core/codesearch_engine.py

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
