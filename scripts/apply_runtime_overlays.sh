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

apply_b64_overlay "js_ts_v2"
grep -q 'self-simplification-v14-js-structural-v2' core/simplification_analyzer.py
grep -q 'python_ast+js_structural_v2' core/simplification_analyzer.py

apply_b64_overlay "portable_agent_v1"
grep -q 'plw-portable-agent-v1' core/portable_agent.py
grep -q 'plw agent orient' plw_cli.py

apply_b64_overlay "validation_runtime_cli_v1"
grep -q 'parser.add_argument("--evidence-id", action="append"' plw_cli.py
grep -q 'F138 still requires exact identity/content agreement' skills/validation-runtime-dispatch-workflow.md

apply_b64_overlay "portable_topology_scope_v1"
grep -q 'def target_topology_support' core/portable_agent.py
grep -q 'TARGET_TOPOLOGY_LANGUAGE_UNSUPPORTED' plw_cli.py
grep -q 'experimental_python_promoted' core/portable_agent.py

apply_b64_overlay "candidate_selection_v1"
grep -q 'candidate-selection-provenance-v1' core/candidate_provenance.py
grep -q 'INTERNAL_CANDIDATE_CAPABILITIES' core/candidate_provenance.py
grep -q '_raw_source_site_facts' core/candidate_provenance.py
grep -q 'def cmd_candidate' plw_cli.py

apply_b64_overlay "shared_graph_determinism_v1"
grep -q 'return sorted(set(usages))' core/shared_graph.py
grep -q 'semantic_usages.*sorted(set(resolved_edge_usages' core/shared_graph.py

apply_b64_overlay "candidate_agent_handoff_v1"
grep -Fq '"candidate": _contract(' core/semantic_affordance.py
grep -Fq '"capability": "candidate", "coverage": "CONDITIONAL_DISCOVERY"' core/capability_selection.py
grep -Fq 'plw candidate select' skills/candidate-selection-provenance-workflow.md

apply_b64_overlay "candidate_affordance_projection_v1"
grep -Fq '"information_role": row.get("role")' hott_kernel.py
grep -Fq '"expected_information_gain": list(row.get("expected_information_gain", []) or [])[:2]' hott_kernel.py
grep -Fq '"information_role": row.get("information_role")' core/agent_communication.py

apply_b64_overlay "operation_contract_reference_v1"
grep -Fq 'OPERATION_CONTRACT_SCHEMA_VERSION = "plw-operation-contract-v1"' core/semantic_affordance.py
grep -Fq '"operation_contract_available": bool(contract.get("operation_contracts"))' core/capability_selection.py
grep -Fq '"operation_contract_count": int(row.get("operation_contract_count", 0) or 0)' hott_kernel.py

apply_b64_overlay "operation_plan_preflight_v1"
apply_b64_overlay "operation_plan_preflight_repair_v1"
apply_reviewable_overlay "invocation_snapshot_v1"
apply_reviewable_overlay "readiness_version_v1"
apply_reviewable_overlay "browserless_ui_reference_v1"
apply_reviewable_overlay "ui_source_precision_v1"
apply_reviewable_overlay "ui_render_conditions_v1"
apply_reviewable_overlay "ui_style_reference_v1"
apply_reviewable_overlay "ui_agent_handoff_v1"
apply_reviewable_overlay "codesearch_engine_v1"
grep -Fq 'codesearch-engine-v1' core/codesearch_engine.py
grep -Fq 'def cmd_ast(' plw_cli.py
grep -Fq '"codesearch_engine"' core/portable_agent.py
apply_reviewable_overlay "codesearch_engine_v2"
grep -Fq 'plw-dynamic-context-v1' core/portable_agent.py
grep -Fq 'def dynamic_context(' core/portable_agent.py
grep -Fq '"dynamic_context"' plw_cli.py
apply_reviewable_overlay "topology_engine_v1"
grep -Fq 'plw-topology-engine-v1' core/topology_engine.py
grep -Fq 'def build_engine_layer(' core/topology_engine.py
grep -Fq '"engine_layer_schema"' core/target_topology.py
apply_reviewable_overlay "batch_engine_v1"
grep -Fq 'def run_batch_query(' core/codesearch_engine.py
grep -Fq 'batch_mode' core/topology_engine.py
grep -Fq 'run_batch_query' plw_cli.py
apply_reviewable_overlay "engine_state_v1"
grep -Fq 'def run_index_command(' core/codesearch_engine.py
grep -Fq 'def resolve_cache_dir(' core/codesearch_engine.py
grep -Fq 'SUPPORTED_SCHEMAS' core/codesearch_engine.py
grep -Fq '"index"' plw_cli.py
apply_reviewable_overlay "engine_pin_v1"
grep -Fq 'PINNED_ENGINE_DIGESTS' core/codesearch_engine.py
grep -Fq 'def _engine_digest(' core/codesearch_engine.py
grep -Fq 'CODESEARCH_ALLOW_UNPINNED' core/codesearch_engine.py
apply_reviewable_overlay "engine_pin_v2"
grep -Fq 'f7ef3778d9407a1daa5da223bc959d64ee85a7000597a9ca0fe3d228884dedca' core/codesearch_engine.py
grep -Fq '05c7fa3a8e63' core/codesearch_engine.py
apply_reviewable_overlay "engine_pin_v3"
grep -Fq 'c251454d970ae19b76f727e65476a9b945bd7f21711c86d5a9b41efe041ed1cb' core/codesearch_engine.py
grep -Fq '8da8dea19350' core/codesearch_engine.py
apply_reviewable_overlay "engine_pin_v4"
grep -Fq '54ff590871b6bce075e1dc46be65dae72eeeb8b4af80ab539a56b642074e9295' core/codesearch_engine.py
grep -Fq '8da8dea19350' core/codesearch_engine.py
grep -Fq '"kotlin"' core/codesearch_engine.py
apply_reviewable_overlay "engine_pin_v5"
grep -Fq 'b6f0ec48ec1bfba35a3b4f0a50906bc19bd0063caa33e26ae289069996a921a1' core/codesearch_engine.py
grep -Fq '9a3db30a5027' core/codesearch_engine.py
apply_reviewable_overlay "engine_pin_v6"
grep -Fq '62b532ec6b4d22ca223c452614a8db020566fc967678662b4a4ac4953906b177' core/codesearch_engine.py
grep -Fq 'e88c2e3f11f8' core/codesearch_engine.py
apply_reviewable_overlay "engine_state_v2"
grep -Fq 'KNOWN_SCHEMA = "go10"' core/codesearch_engine.py
grep -Fq '"go10": "plw-codesearch-engine-v1"' core/codesearch_engine.py
apply_reviewable_overlay "engine_pin_v7"
grep -Fq '828eeeab911764a71709e1bf8ed2724c21cb74d30f26ebebcc3b2111d04f3765' core/codesearch_engine.py
grep -Fq 'b6e9a95c06a9' core/codesearch_engine.py
apply_reviewable_overlay "discovery_v1"
grep -Fq 'PATH_CANDIDATES' core/codesearch_engine.py
grep -Fq 'codesearch-go-linux-x86_64' core/codesearch_engine.py
apply_reviewable_overlay "limit_v1"
grep -Fq '_apply_limit' core/codesearch_engine.py
grep -Fq '_apply_batch_limit' core/codesearch_engine.py
apply_reviewable_overlay "brokenpipe_v1"
grep -Fq 'signal.signal(signal.SIGPIPE, signal.SIG_DFL)' plw_cli.py
apply_reviewable_overlay "topology_honesty_v1"
grep -Fq 'reconvergent_dag' codebase/hott_analyzers.py
grep -Fq 'hub_stabilization_and_contract_hardening' core/synthesizer.py
grep -Fq 'intermediate_nodes' codebase/hott_analyzers.py
apply_reviewable_overlay "kotlin_topology_v1"
grep -Fq 'KOTLIN_SOURCE_EXTENSIONS' language_semantics.py
grep -Fq 'kotlin_symbol_index' core/shared_graph.py
grep -Fq 'stable_shared_graph (.ts/.tsx/.js/.jsx/.kt)' core/portable_agent.py
apply_reviewable_overlay "agent_friendly_v1"
grep -Fq '_top_check_findings' hott_kernel.py
grep -Fq '_resolve_ast_positionals' plw_cli.py
grep -Fq 'Task pipeline, in order' plw_cli.py
apply_reviewable_overlay "root_flag_v1"
grep -Fq '_root_flag' plw_cli.py
grep -Fq '_root_flag_from_argv' hott_kernel.py
grep -Fq 'Every command that takes [root] also accepts --root DIR' plw_cli.py
apply_reviewable_overlay "receipt_bias_v1"
grep -Fq '_TASK_RECEIPT_SCHEMA' hott_kernel.py
grep -Fq '_validate_task_receipt' plw_cli.py
grep -Fq 'Receipts: every stage issues a receipt' plw_cli.py
apply_reviewable_overlay "namespace_alias_v1"
grep -Fq 'def cmd_lease' plw_cli.py
grep -Fq 'def cmd_proof' plw_cli.py
grep -Fq 'def cmd_ledger' plw_cli.py
grep -Fq 'Namespace aliases (same commands, grouped names' plw_cli.py
apply_reviewable_overlay "namespace_alias_help_v1"
grep -Fq 'Operations map to canonical commands:' plw_cli.py
grep -Fq 'formatter_class=argparse.RawDescriptionHelpFormatter' plw_cli.py
apply_reviewable_overlay "registry_alias_hint_v1"
grep -Fq 'ALIAS_TO_CANONICAL' core/semantic_affordance.py
grep -Fq '"alias_of"' core/semantic_affordance.py
apply_reviewable_overlay "help_exit_codes_v1"
grep -Fq '_MODE_HELP_USAGE' hott_kernel.py
grep -Fq 'Usage: plw validation-coverage <plan.json>' plw_cli.py
grep -Fq 'Usage: plw check [root]' plw_cli.py
apply_reviewable_overlay "ast_help_root_first_v1"
grep -Fq '[root] [query]' plw_cli.py
grep -Fq 'plw ast is the exception: two positionals are root-first' plw_cli.py
apply_reviewable_overlay "orient_engine_answer_v1"
grep -Fq 'engine_answer' core/agent_communication.py
grep -Fq '_engine_answer_summary' hott_kernel.py
grep -Fq '# Engine answer: ' plw_cli.py
apply_reviewable_overlay "doctor_ready_scope_v1"
grep -Fq 'portable interface + target root; stable topology is a separate check' plw_cli.py
apply_reviewable_overlay "describe_alias_human_v1"
grep -Fq '# Alias of: ' plw_cli.py
apply_reviewable_overlay "describe_group_op_alias_v1"
grep -Fq '"obligations": "validation-coverage"' core/semantic_affordance.py
apply_reviewable_overlay "describe_first_contact_v1"
grep -Fq 'FIRST_CONTACT_HELP' core/semantic_affordance.py
apply_reviewable_overlay "topology_files_scope_v1"
grep -Fq 'stable-topology sources only' plw_cli.py
apply_reviewable_overlay "ledger_memory_disambiguation_v1"
grep -Fq 'transition-memory (not plw memory)' plw_cli.py
apply_reviewable_overlay "doctor_engine_required_note_v1"
grep -Fq 'required for structural questions' plw_cli.py
apply_reviewable_overlay "rust_topology_v1"
grep -Fq 'parse_rust_imports' language_semantics.py
grep -Fq 'rust_module_index' core/shared_graph.py
grep -Fq '"/".join(source_extensions())' plw_cli.py
apply_reviewable_overlay "engine_answer_bridge_v1"
grep -Fq 'single_candidate_content_identifiers' core/target_resolution.py
grep -Fq 'Only ``calls`` edges are projected as calls' hott_kernel.py
grep -Fq 'Raw flat edge' core/agent_communication.py
apply_reviewable_overlay "silent_wrong_answers_v1"
grep -Fq '".mts": "typescript"' core/portable_agent.py
grep -Fq '"kotlin", "rust"' core/portable_agent.py
grep -Fq 'base_to_queries' core/codesearch_engine.py
grep -Fq 'refers to the module the group opens' language_semantics.py
grep -Fq '".mjs", ".cjs", ".mts", ".cts"' language_semantics.py
apply_reviewable_overlay "remove_install_v1"
grep -Fq '5. Other' plw_cli.py
grep -Fq 'install_removed=1' scripts/smoke_portable_agent.sh
grep -Fq 'removed install did not fail as unknown_mode' scripts/smoke_portable_agent.sh
apply_reviewable_overlay "parser_fail_closed_v1"
grep -Fq '_PlwArgumentParser' plw_cli.py
grep -Fq '_cli_error' plw_cli.py
grep -Fq 'parse_intermixed_args' plw_cli.py
apply_reviewable_overlay "adapter_build_v1"
grep -Fq 'binary = Path(binary)' core/codesearch_engine.py
grep -Fq '{"static", "map"' plw_cli.py
grep -Fq 'adapter_runtime_unavailable' agent_tool.py
grep -Fq 'Track 4 (K5 follow-up)' core/shared_graph.py
grep -Fq 'BATCH_CHUNK_SIZE = 500' core/codesearch_engine.py
grep -Fq 'opts.cache_dir, opts.limit' plw_cli.py
grep -Fq '"stable_target_topology_ready"' core/portable_agent.py
grep -Fq '"task_postcondition": "UNPROVEN"' hott_kernel.py
grep -Fq '"plw-version-v1"' plw_cli.py
grep -Fq '"geometry_observed": False' codebase/static_ui_reference.py
grep -Fq '"ui-static": _contract(' core/semantic_affordance.py
grep -Fq 'def cmd_ui_static(' plw_cli.py
grep -Fq '"ui_source_handoff": ui_source_handoff' hott_kernel.py
grep -Fq 'PLAN_SCHEMA_VERSION = "plw-operation-plan-v1"' core/operation_plan.py
grep -Fq 'RESOLUTION_SCHEMA_VERSION = "plw-operation-plan-resolution-v1"' core/operation_plan.py
grep -Fq 'INPUT_VALUE_UNREPRESENTABLE_IN_ARGV' core/operation_plan.py
grep -Fq 'dirty_content_digest' core/candidate_provenance.py
apply_reviewable_overlay "simplify_surface_v1"
grep -Fq 'plw simplify [root] [--reference stable-root] [--json]' plw_cli.py
grep -Fq 'Structural simplification candidates (duplicate bodies, forwarding wrappers, redundant branches)' plw_cli.py
grep -Fq 'grouped.setdefault(str(row.get("kind", "unknown")), []).append(row)' plw_cli.py
grep -Fq 'loc_text = f" | loc={locs}" if locs else ""' plw_cli.py
apply_reviewable_overlay "simplify_rs_kt_v1"
grep -Fq 'rs_structural_v1' core/simplification_analyzer.py
grep -Fq 'kt_structural_v1' core/simplification_analyzer.py
grep -Fq 'EXPERIMENTAL_SOURCE_EXTENSIONS = SOURCE_EXTENSIONS + PYTHON_SOURCE_EXTENSIONS' language_semantics.py
grep -Fq 'RS semantic files=' plw_cli.py
grep -Fq 'rs_rules=' plw_cli.py
apply_reviewable_overlay "deopt_lang_gate_v1"
grep -Fq '_DEOPT_JS_TS_EXTENSIONS' codebase/performance_analyzers.py
grep -Fq 'probe = _strip_strings(line).strip()' codebase/performance_analyzers.py
apply_reviewable_overlay "ts_js_ext_map_v1"
grep -Fq '_TS_COMPILED_TO_SOURCE_EXTENSIONS' language_semantics.py
grep -Fq 'add(resolved_base[: -len(compiled)] + source)' language_semantics.py
apply_reviewable_overlay "kotlin_label_v1"
grep -Fq '".kt": "kotlin"' core/topology_engine.py
grep -Fq 'G2 (2026-11-14)' core/topology_engine.py
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
