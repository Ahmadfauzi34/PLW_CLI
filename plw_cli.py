#!/usr/bin/env python3
"""
PLW (Proof Logic Working) CLI — Native Linux Command Layer for HoTT Kernel
Provides zero-bloat, token-budgeted, single-command codebase intelligence.

Usage:
  plw --version | plw version [--json]
  plw doctor [root] [--json]
  plw agent orient [root] --task TASK [--target file] [--goal GOAL] [--depth DEPTH] [--json]
  plw capability list [--json]
  plw capability describe <command> [--json]
  plw operation-plan resolve <plan.json|-> [--json]
  plw skill list [--json]
  plw skill show <command-or-skill-path> [--json]
  plw topology [root] [--top N] [--full-edges] [--json]
  plw work <task> [root] [--target file] [--goal auto|understand|change|architecture|performance|runtime|history] [--depth auto|quick|standard|deep]
  plw semantic describe <command> [--json]
  plw semantic route <task> [--goal GOAL] [--json]
  plw context <query> [root] [--target file] [--budget N] [--cache-mode MODE] [--truth-scope ID] [--lineage sha256:id] [--no-proactive-truth] [--json]
  plw transitions <query> [root] [--target file] [--budget N] [--lineage sha256:id] [--json]
  plw kan <target_file_or_query> [root] [--mode lan|ran|both] [--json]
  plw check [root] [--analyzers a,b] [--json]
  plw steer [root] [--json]
  plw brief <file> [root] [--json]
  plw impact <file> [root] [--json]
  plw outline <file> [root] [--json]
  plw compact-output [--command CMD] [--kind KIND] [--budget N] [--lineage sha256:id] [--json]
  plw recover-output <sha256:id> [--lineage sha256:id] [--no-telemetry]
  plw attest-runtime <sha256:output-id> --exit-code N --command CMD --claim runtime_test_result|runtime_behavior [--root .] [--target file] [--truth-scope ID] [--json]
  plw output-telemetry [--store-dir DIR] [--json]
  plw transition-memory [--json]
  plw transition-model [--state-class sha256:...] [--json]
  plw q-values [--state-class sha256:...] [--json]
  plw reconcile-outcomes [--record] [--json]
  plw lineage <sha256:id> [--json]
  plw simplify [root] [--reference stable-root] [--json]
  plw action-lease acquire [--store-dir DIR] --allow-state-change [--json]
  plw action-lease status <sha256:lease-id> [--store-dir DIR] [--json]
  plw action-outcome register [--store-dir DIR] --allow-state-change [--json]
  plw action-outcome record [--store-dir DIR] --allow-state-change [--json]
  plw action-outcome status <sha256:lease-id> [--store-dir DIR] [--json]
  plw action-adapter prepare [--store-dir DIR] --allow-state-change [--json]
  plw action-adapter observe [--store-dir DIR] --allow-state-change [--json]
  plw action-adapter status <sha256:lease-id> [--store-dir DIR] [--json]
  plw angular-anchors [root] [--file FILE] [--selector SELECTOR] [--no-resources] [--cache-mode MODE] [--json]
  plw ui static <query> [root] [--selector SELECTOR] [--max-candidates N] [--json]
  plw ui map <snapshot.json> [root] [--min-gap PX] [--max-elements N] [--max-issues N] [--json]
  plw ui inspect <snapshot.json> <ui-id> [root] [--json]
  plw ui scenarios <snapshot1.json> <snapshot2.json> [...] [--root ROOT] [--min-gap PX] [--json]
  plw ui diff <baseline.json> <candidate.json> [--root ROOT] [--baseline-root ROOT] [--min-gap PX] [--json]
  plw ui hunt <candidate.json> [--baseline baseline.json] [--root ROOT] [--runtime-reference FILE] [--min-gap PX] [--json]
  plw ui diagnose <candidate.json> [--baseline baseline.json] [--root ROOT] [--runtime-reference FILE] [--json]
  plw ui reproduce <diagnosis.json> --root ROOT (--url URL | --html-file FILE) [--candidate-id ID] [--setup-js FILE] [--out FILE] [--json]
  plw ui probe-plan <diagnosis.json> [--candidate-id ID] [--json]
  plw ui isolate <diagnosis.json> <probe-plan.json> --root ROOT (--url URL | --html-file FILE) [--json]
  plw ui probe-plan <diagnosis.json> [--candidate-id ID] [--json]
  plw ui isolate <diagnosis.json> <probe-plan.json> --root ROOT (--url URL | --html-file FILE) [--candidate-id ID] [--setup-js FILE] [--json]
  plw ui fix-plan <diagnosis.json> <isolation.json> [--candidate-id ID] [--out FILE] [--json]
  plw ui verify-fix <diagnosis.json> <fix-plan.json> --root ROOT (--url URL | --html-file FILE) [--proposal-id ID] [--setup-js FILE] [--json]
  plw ui capture-script [root] --revision REV [--state NAME] [--fixture NAME]
  plw ui-reference <snapshot.json> [root] [--no-resources] [--cache-mode MODE] [--json]
  plw runtime-ui-reference <lease-id> <snapshot.json> [root] [--store-dir DIR] [--no-resources] [--cache-mode MODE] [--json]
  plw geometric-impact <snapshot.json> [root] [--focus UI_ID] [--constraints query.json] [--proximity PX] [--lease-id ID --store-dir DIR] [--no-resources] [--cache-mode MODE] [--json]
  plw validation-plan <snapshot.json> [root] [--scenario-catalog catalog.json] [--max-scenarios N] [--focus UI_ID] [--constraints query.json] [--lease-id ID --store-dir DIR] [--json]
  plw validation-evidence <plan.json> <evidence.json> [--store-dir DIR] [--json]
  plw validation-coverage <plan.json> <evidence.json> <obligations.json> [--store-dir DIR] [--json]
  plw validation-coverage execution-backed <f139-binding.json> <obligations.json> [--json]
  plw validation-request <plan.json> <evidence.json> <obligations.json> --validator-id ID --executor-id ID [--store-dir DIR] [--json]
  plw validation-authorization <plan.json> <evidence.json> <obligations.json> <assertion.json> --request-id ID --validator-id ID --executor-id ID [--store-dir DIR] [--json]
  plw validation-lease acquire <plan.json> <evidence.json> <obligations.json> <assertion.json> --request-id ID --validator-id ID --executor-id ID --idempotency-key KEY [--lease-ttl-seconds N] [--store-dir DIR] --allow-state-change [--json]
  plw validation-lease status <sha256:lease-id> [--request-id ID] [--executor-id ID] [--store-dir DIR] [--json]
  plw validation-attempt register <lease-id> --attempt-key KEY --validator-id ID --executor-id ID --allow-state-change [--store-dir DIR] [--json]
  plw validation-attempt status <lease-id> [--request-id ID] [--executor-id ID] [--store-dir DIR] [--json]
  plw validation-dispatch prepare <attempt-id> --dispatch-key KEY --validator-id ID --executor-id ID --allow-state-change [--store-dir DIR] [--json]
  plw validation-dispatch status <attempt-id> [--request-id ID] [--executor-id ID] [--store-dir DIR] [--json]
  plw validation-runtime claim <dispatch-intent-id> --runtime-key KEY --allow-state-change [--store-dir DIR] [--json]
  plw validation-runtime observe <dispatch-intent-id> --dispatch-state STATE --reconciliation-state STATE [--terminal-status STATUS] [--observation-kind KIND] --observation-json JSON [--evidence-id sha256:...]... --allow-state-change [--store-dir DIR] [--json]
  plw validation-runtime status <dispatch-intent-id> [--store-dir DIR] [--json]
  plw validation-result accept <runtime-observation-id> <result.json> <request-set.json> --allow-state-change [--store-dir DIR] [--json]
  plw validation-result status <runtime-observation-id> [--store-dir DIR] [--json]
      -> Accepts exact F137 validation.result content only; never executes runtime and never proves postconditions/truth
  plw validation-scenario-bind <acceptance-id> <plan.json> <result.json> [--store-dir DIR] [--json]
  plw postcondition-proof <f139-binding.json> <f140-coverage.json> <obligations.json> <postcondition-spec.json> --current-revision REV [--json]
  plw fullstack-state <root> <f139-binding.json> <f141-proof.json> <fullstack-spec.json> --current-revision REV [--geometry-impact FILE] [--ui-reference FILE] [--json]
      -> Binds one F138-accepted result to exact F128 scenario identity; read-only and does not prove obligation coverage/postconditions/truth
  plw install [--system | --bin-dir DIR] [--json]
  plw memory <subcommand> [args...]
  plw fiber <subcommand> [args...]
  plw <any hott_kernel mode> [args...]
"""

import sys
import os
import json
import subprocess
from pathlib import Path
from typing import Any, Dict, List, Optional

# Automatically locate and resolve tools/ai_studio_tool directory
def _resolve_tool_root() -> Path:
    candidates = [
        Path(__file__).resolve().parent,
        Path.cwd() / "tools" / "ai_studio_tool",
        Path("/app/applet/tools/ai_studio_tool"),
    ]
    for c in candidates:
        if (c / "hott_kernel.py").is_file():
            return c
    return Path(__file__).resolve().parent

TOOL_ROOT = _resolve_tool_root()
if str(TOOL_ROOT) not in sys.path:
    sys.path.insert(0, str(TOOL_ROOT))

# Import kernel functions
try:
    import hott_kernel
except ImportError as exc:
    print(f"[PLW Error] Unable to load hott_kernel from {TOOL_ROOT}: {exc}", file=sys.stderr)
    sys.exit(1)

try:
    from core.command_output import (
        OUTPUT_KINDS as COMMAND_OUTPUT_KINDS,
        compact_command_output,
        recover_raw_output,
        get_command_output_telemetry,
    )
    from core.context_accountability import load_accountability
    from core.capability_accountability_chain import query_accountability_chain
    from core.capability_action_lease import (
        acquire_action_lease,
        build_action_lease_request,
        inspect_action_lease,
    )
    from core.capability_executor_outcome import (
        build_executor_attempt_request,
        register_executor_attempt,
        build_executor_outcome_request,
        record_executor_outcome,
        inspect_executor_outcome,
    )
    from core.capability_executor_adapter import (
        build_adapter_dispatch_request,
        prepare_adapter_dispatch,
        build_adapter_observation,
        record_adapter_observation,
        inspect_adapter_dispatch,
    )
    from core.decision_lineage import trace_lineage
    from core.runtime_attestation import create_runtime_attestation, find_runtime_attestation_lineages_for_output
    from core.proof_transitions import transition_identity
    from core.semantic_affordance import describe_command, route_task_semantics
    from core.operation_plan import resolve_operation_plan
    from core.portable_agent import (
        build_version as portable_build_version,
        doctor as portable_doctor,
        dynamic_context as portable_dynamic_context,
        list_capabilities as portable_list_capabilities,
        describe_capability as portable_describe_capability,
        list_skills as portable_list_skills,
        read_skill as portable_read_skill,
        orientation_surface as portable_orientation_surface,
    )
    from memory.transition_outcomes import summarize_transition_outcomes
    from memory.state_transition_model import find_transition_intent, summarize_state_transition_model
    from memory.q_value_model import summarize_q_value_model
    from core.outcome_reconciliation import refresh_outcome_reconciliation, record_reconciliation_accountability
    from core.simplification_analyzer import analyze_simplification, compare_simplification
    from core.candidate_provenance import run_candidate_provenance
    from codebase.angular_semantics import analyze_angular_ownership
    from codebase.static_ui_reference import static_ui_reference
    from codebase.geometry_correspondence import (
        GeometrySnapshotError,
        build_ui_reference_state,
        load_geometry_snapshot,
    )
    from fullstack.runtime_geometry_capture import build_runtime_ui_reference_state
    from fullstack.geometric_impact import (
        GeometricImpactError,
        build_targeted_geometric_impact,
        load_geometric_impact_query,
    )
    from ui.mapper import (
        UIMapperError,
        build_browser_capture_script,
        build_ui_map,
        inspect_ui_element,
    )
    from ui.scenarios import build_ui_scenario_map
    from ui.diff import build_ui_geometry_diff
    from ui.bug_hunter import build_ui_bug_hunt
    from ui.diagnose import build_cross_domain_ui_diagnosis
    from ui.reproduce import UIReproductionError, execute_targeted_ui_reproduction
    from ui.causal_probe import UICausalProbeError, build_ui_causal_probe_plan, execute_ui_causal_isolation
    from ui.fix import UIFixError, build_bounded_ui_fix_plan, verify_bounded_ui_fix
    from fullstack.validation_scenarios import (
        ValidationScenarioError,
        build_targeted_validation_plan,
        load_validation_scenario_catalog,
    )
    from fullstack.validation_evidence import (
        ValidationEvidenceError,
        bind_validation_evidence,
        load_validation_evidence_bundle,
        load_validation_plan,
    )
    from fullstack.proof_obligations import (
        SCHEMA_VERSION as VALIDATION_PROOF_COVERAGE_SCHEMA_VERSION,
        EXECUTION_COVERAGE_SCHEMA_VERSION as EXECUTION_PROOF_COVERAGE_SCHEMA_VERSION,
        ProofObligationError,
        load_proof_obligation_set,
        project_validation_proof_obligations,
        project_execution_backed_proof_obligations,
    )
    from fullstack.validation_execution import (
        SCHEMA_VERSION as VALIDATION_EXECUTION_REQUEST_SET_SCHEMA_VERSION,
        ValidationExecutionError,
        build_validation_execution_requests,
    )
    from fullstack.validation_authorization import (
        SCHEMA_VERSION as VALIDATION_AUTHORIZATION_BINDING_SCHEMA_VERSION,
        ValidationAuthorizationError,
        build_validation_authorization_binding,
    )
    from fullstack.validation_lease import (
        SCHEMA_VERSION as VALIDATION_EXECUTION_LEASE_REQUEST_SCHEMA_VERSION,
        ValidationLeaseError,
        acquire_validation_execution_lease,
        build_validation_execution_lease_request,
        inspect_validation_execution_lease,
    )
    from fullstack.validation_attempt import (
        build_validation_attempt_request, evaluate_validation_attempt,
        register_validation_attempt, inspect_validation_attempt,
    )
    from fullstack.validation_dispatch import (
        ValidationDispatchError, build_validation_dispatch_request_from_store,
        prepare_validation_dispatch_intent, inspect_validation_dispatch_intent,
    )
    from fullstack.validation_runtime import (
        claim_validation_runtime_handoff, record_validation_runtime_observation,
        inspect_validation_runtime,
    )
    from fullstack.validation_result_evidence import (
        ValidationResultEvidenceError, load_validation_result_payload,
        load_validation_execution_request_set, accept_validation_result_evidence,
        inspect_validation_result_evidence,
    )
    from fullstack.execution_backed_evidence import (
        ExecutionBackedEvidenceError, load_execution_backed_scenario_evidence,
        bind_execution_backed_scenario_evidence,
    )
    from fullstack.postcondition_proof import (
        SCHEMA_VERSION as POSTCONDITION_PROOF_SCHEMA_VERSION,
        PostconditionProofError, load_postcondition_spec, load_postcondition_coverage,
        assemble_postcondition_proof,
    )
    from fullstack.full_stack_validated_state import (
        SCHEMA_VERSION as FULL_STACK_VALIDATED_STATE_SCHEMA_VERSION,
        FullStackValidatedStateError, load_json_artifact as load_full_stack_json_artifact,
        compose_full_stack_validated_state,
    )
except ImportError as exc:
    print(f"[PLW Error] Unable to load command output/accountability layer: {exc}", file=sys.stderr)
    sys.exit(1)


def _resolve_truth_scope(explicit_scope: Optional[str] = None) -> Optional[str]:
    """Resolve truth-delivery scope at the CLI orchestration layer.

    core truth delivery stays independent from the fibration implementation.
    PLW may bridge to an active fiber because this CLI is the integration layer.
    """
    if explicit_scope and str(explicit_scope).strip():
        return str(explicit_scope).strip()
    env_scope = os.environ.get("PLW_TRUTH_SCOPE")
    if env_scope and env_scope.strip():
        return env_scope.strip()
    try:
        from context.fibration import load_fiber_state
        state = load_fiber_state()
        fiber_id = state.get("fiber_id") if isinstance(state, dict) else None
        if fiber_id:
            return f"fiber:{fiber_id}"
    except Exception:
        pass
    return None


def _format_context_output(result: Dict[str, Any], as_json: bool = False) -> None:
    if as_json or "error" in result:
        print(json.dumps(result, indent=2))
        return

    context_block = result.get("context_block", "")
    budget = result.get("budget", {})
    selection = result.get("selection", {})
    paths = selection.get("selected_paths", [])

    print(f"# [PLW Context Pack]")
    print(f"# Query: {result.get('query')}")
    requested_budget = budget.get("budget_tokens", budget.get("requested_estimated_tokens", "?"))
    print(f"# Budget: {budget.get('estimated_tokens', '?')}/{requested_budget} tokens (within_budget={budget.get('within_budget')})")
    print(f"# Selected: {len(paths)} files: {', '.join(paths[:5])}{'...' if len(paths) > 5 else ''}")
    evidence_roles = result.get("evidence_roles", {}) or {}
    if evidence_roles:
        source_roles = evidence_roles.get("source", {}) or {}
        memory_roles = evidence_roles.get("memory", {}) or {}
        enrichment = evidence_roles.get("optional_enrichment", {}) or {}
        print(
            "# Evidence roles: "
            f"source[due={len(source_roles.get('proof_due_paths', []) or [])},"
            f"explicit={len(source_roles.get('explicit_target_paths', []) or [])},"
            f"relevance={len(source_roles.get('selected_by_relevance_paths', []) or [])}] | "
            f"memory[due={len(memory_roles.get('proof_due_ids', []) or [])},"
            f"continuity={len(memory_roles.get('continuity_ids', []) or [])}] | "
            f"optional_kan={len(enrichment.get('kan_targets', []) or [])}"
        )
        for row in (source_roles.get("selected", []) or [])[:5]:
            signals = list(row.get("selection_signals", []) or [])
            signal_text = ",".join(signals[:3]) or "none"
            print(
                f"# WHY {row.get('file')}: {row.get('primary_role')} | "
                f"signals={signal_text}"
            )
        missing_due_memory = memory_roles.get("required_but_not_included_ids", []) or []
        if missing_due_memory:
            print(
                "# MEMORY_DUE_UNDELIVERED="
                + ",".join(str(v) for v in missing_due_memory[:3])
            )
    truth_floor = result.get("truth_floor", {})
    if truth_floor:
        gate = truth_floor.get("claim_gate", {})
        print(
            f"# Truth floor: {truth_floor.get('status')} | "
            f"current_source_claims_allowed={gate.get('current_source_claims_allowed')} | "
            f"historical_claims_allowed={gate.get('historical_or_rationale_claims_allowed')}"
        )
        floor_ledger = truth_floor.get("ledger", {})
        if floor_ledger.get("id"):
            print(f"# TRUTH_FLOOR_AUDIT=plw accountability {floor_ledger.get('id')}")
    truth_delivery = result.get("truth_delivery", {})
    if result.get("truth_scope") and result.get("truth_scope") != "unscoped":
        print(f"# Truth scope: {result.get('truth_scope')}")
    if truth_delivery.get("delivery_status") == "delivered":
        delivered = truth_delivery.get("delivered_targets", [])
        ledger = truth_delivery.get("ledger", {})
        print(
            f"# Truth delivery: proactive evidence restored={len(delivered)} "
            f"targets={','.join(delivered[:3])}{'...' if len(delivered) > 3 else ''}"
        )
        if ledger.get("id"):
            print(f"# TRUTH_AUDIT=plw accountability {ledger.get('id')}")
    claim_obligations = result.get("claim_obligations", {})
    if claim_obligations:
        blocked = claim_obligations.get("blocked_claims", []) or []
        requested = claim_obligations.get("requested_claims", []) or []
        print(
            f"# Claim obligations: {claim_obligations.get('status')} | "
            f"requested={','.join(requested) or 'none'} | blocked={','.join(blocked) or 'none'}"
        )
        claim_ledger = claim_obligations.get("ledger", {}) or {}
        if claim_ledger.get("id"):
            print(f"# CLAIM_AUDIT=plw accountability {claim_ledger.get('id')}")
    proof_transitions = result.get("proof_transitions", {}) or {}
    preferred = proof_transitions.get("preferred_transition") or {}
    if preferred:
        print(
            f"# Preferred transition: {preferred.get('transition_type')} | "
            f"authority={preferred.get('authority_domain')} | "
            f"side_effect={preferred.get('side_effect_class')}"
        )
        if preferred.get("command_hint"):
            print(f"# NEXT={preferred.get('command_hint')}")
        transition_ledger = proof_transitions.get("ledger", {}) or {}
        if transition_ledger.get("id"):
            print(f"# TRANSITION_AUDIT=plw accountability {transition_ledger.get('id')}")
        preferred_lineage = ((preferred.get("lineage") or {}).get("id"))
        if preferred_lineage:
            print(f"# NEXT_LINEAGE={preferred_lineage}")
    if truth_delivery.get("delivery_status") == "delivery_incomplete":
        print(
            "# Truth delivery: INCOMPLETE; recovery required for "
            + ",".join(truth_delivery.get("unresolved_due_targets", [])[:3])
        )
    accountability = result.get("accountability", {})
    ledger = accountability.get("ledger", {})
    debt = accountability.get("evidence_debt", {})
    if ledger.get("id"):
        print(
            f"# Accountability: {ledger.get('id')} | owner={accountability.get('responsible_role')} "
            f"| debt=files:{debt.get('project_files_omitted', 0)},"
            f"lines:{debt.get('source_excerpt_lines_omitted', 0)},"
            f"memory:{debt.get('memory_records_omitted', 0)}"
        )
        if ledger.get("available"):
            print(f"# AUDIT={ledger.get('command')}")
        lineage_id = ((accountability.get("lineage") or {}).get("id"))
        if lineage_id:
            print(f"# LINEAGE={lineage_id}")
    print("-" * 60)
    print(context_block.strip())
    print("-" * 60)


def _format_kan_output(result: Dict[str, Any], as_json: bool = False) -> None:
    if as_json or "error" in result:
        print(json.dumps(result, indent=2))
        return

    query = result.get("target_file") or result.get("target") or result.get("query_fragment") or result.get("query", "")
    mode = result.get("mode", "both")
    print(f"# [PLW Kan Extension Imputation & Guidance]")
    print(f"# Target/Query: {query} (mode: {mode})")
    print("-" * 60)

    # Check for module imputation format
    lan = result.get("left_kan_extension") or result.get("lan_colimit", {})
    ran = result.get("right_kan_extension") or result.get("ran_limit", {})
    role = result.get("inferred_role") or (lan.get("role") if isinstance(lan, dict) else None) or "unknown"

    rendered_sections = False

    if lan and isinstance(lan, dict) and lan.get("status") != "no_match":
        rendered_sections = True
        print("[Left Kan Extension / Colimit (Imputed Topology)]")
        print(f"  Inferred Role: {role}")
        raw_isomorphs = result.get("isomorphic_modules", [])
        if raw_isomorphs:
            iso_strs = []
            for m in raw_isomorphs:
                if isinstance(m, dict):
                    f_name = m.get("file", "")
                    sim = m.get("similarity_score")
                    iso_strs.append(f"{f_name} (sim: {sim})" if sim else f_name)
                else:
                    iso_strs.append(str(m))
            print(f"  Isomorphic Modules: {', '.join(iso_strs[:4])}")

        deps = lan.get("imputed_dependencies", [])
        if deps:
            dep_strs = []
            for d in deps:
                if isinstance(d, dict):
                    conf = d.get("confidence")
                    freq = d.get("frequency")
                    label = f"{d.get('role')} (conf: {conf}, freq: {freq})" if conf and freq else d.get("role", str(d))
                    dep_strs.append(label)
                else:
                    dep_strs.append(str(d))
            print(f"  Expected Dependencies: {', '.join(dep_strs)}")

        callers = lan.get("imputed_dependents", [])
        if callers:
            caller_strs = []
            for c in callers:
                if isinstance(c, dict):
                    conf = c.get("confidence")
                    label = f"{c.get('role')} (conf: {conf})" if conf else c.get("role", str(c))
                    caller_strs.append(label)
                else:
                    caller_strs.append(str(c))
            print(f"  Expected Callers: {', '.join(caller_strs)}")

        comp_test = lan.get("expected_companion_test")
        if comp_test:
            print(f"  Expected Companion Test: {comp_test}")
        print()

    if ran and isinstance(ran, dict) and ran.get("status") != "no_match":
        rendered_sections = True
        print("[Right Kan Extension / Limit (Guided Hints & Conventions)]")
        hints = ran.get("guided_hints", [])
        for hint in hints:
            print(f"  * {hint}")
        conventions = ran.get("architectural_conventions", [])
        if conventions:
            print("  Architectural Conventions:")
            for conv in conventions:
                print(f"    - {conv}")
        print()

    synthesis = result.get("kan_synthesis")
    if synthesis and isinstance(synthesis, dict):
        summary = synthesis.get("summary")
        if summary:
            print(f"[Synthesis] {summary}\n")

    if not rendered_sections:
        # Fallback to general output
        print(json.dumps(result, indent=2))


def _format_check_output(result: Dict[str, Any], as_json: bool = False) -> None:
    if as_json or "error" in result:
        print(json.dumps(result, indent=2))
        return

    summary = result.get("unified_summary", {})
    findings = result.get("findings", [])
    failed = summary.get("analyzers_failed", 0)
    total = summary.get("total_findings", len(findings))

    print(f"# [PLW Architecture & Boundary Check]")
    print(f"# Total Files: {result.get('total_files', '?')} | Findings: {total}")
    print("-" * 60)

    critical_findings = [f for f in findings if f.get("severity") in ("critical", "high", "error")]
    if not critical_findings and failed == 0:
        print("✓ PASS: No critical architectural violations or circular imports detected.")
        if total > 0:
            print(f"  (Note: {total} low/info advisory findings present. Use --json to inspect).")
    else:
        print(f"✗ ISSUES FOUND: {len(critical_findings)} critical/high severity findings:")
        for f in critical_findings[:10]:
            print(f"  - [{f.get('severity', '').upper()}] {f.get('analyzer', '')}: {f.get('message', '')} ({f.get('file', '')})")
        if len(critical_findings) > 10:
            print(f"  ... and {len(critical_findings) - 10} more.")


def _format_steer_output(result: Dict[str, Any], as_json: bool = False) -> None:
    if as_json or "error" in result:
        print(json.dumps(result, indent=2))
        return

    prompt_block = result.get("steering_prompt_block") or result.get("cross_domain_prompt_block", "")
    print(f"# [PLW Steering Guidance]")
    print("-" * 60)
    if prompt_block:
        print(prompt_block.strip())
    else:
        # Cross-domain & codebase signals
        cb = result.get("codebase_steering", {})
        mem = result.get("memory_steering", {})
        x_sig = result.get("cross_domain_signal", {})
        kan_guide = result.get("kan_guidance", {})

        if cb or x_sig:
            strategy = x_sig.get("unified_strategy") or cb.get("strategy", "balanced")
            budget = x_sig.get("recommended_budget") or cb.get("budget", "standard")
            cb_health = x_sig.get("codebase_health", cb.get("health_score"))
            mem_health = x_sig.get("memory_health", mem.get("health_score"))

            print(f"  Unified Strategy: {strategy}")
            print(f"  Recommended Budget: {budget}")
            if cb_health is not None:
                print(f"  Codebase Health Score: {cb_health}")
            if mem_health is not None:
                print(f"  Memory Health Score: {mem_health}")
            if cb.get("archetype"):
                print(f"  Codebase Archetype: {cb.get('archetype')}")
            print()

        signals = result.get("steering_signals", [])
        if signals:
            print("Steering Signals:")
            for s in signals:
                print(f"  * {s}")
            print()

        if kan_guide and isinstance(kan_guide, dict):
            zero_count = kan_guide.get("zero_context_modules_count", 0)
            hints = kan_guide.get("guided_hints", [])
            if zero_count > 0:
                print(f"Kan Extension Imputation Active: {zero_count} zero-context module(s) detected.")
            if hints:
                print("Architecture Conventions & Guided Hints:")
                for h in hints:
                    print(f"  * {h}")
                print()

    print("-" * 60)


def cmd_context(args: List[str]) -> int:
    if not args or args[0] in ("-h", "--help"):
        print("Usage: plw context <query> [root] [--target file[,file]] [--budget N] [--max-hops N] [--cache-mode auto|refresh|off] [--truth-scope ID] [--lineage sha256:id] [--no-proactive-truth] [--json]")
        return 0

    query = args[0]
    root = "."
    target_files = []
    budget = 1200
    max_hops = 2
    cache_mode = None
    truth_scope = None
    lineage_ids: List[str] = []
    proactive_truth = True
    as_json = False

    i = 1
    if i < len(args) and not args[i].startswith("-"):
        root = args[i]
        i += 1

    while i < len(args):
        arg = args[i]
        if arg in ("--target", "--targets") and i + 1 < len(args):
            target_files.extend(v.strip() for v in args[i + 1].split(",") if v.strip())
            i += 1
        elif arg in ("--budget", "--budget-tokens") and i + 1 < len(args):
            try:
                budget = int(args[i + 1])
            except ValueError:
                pass
            i += 1
        elif arg in ("--hops", "--max-hops") and i + 1 < len(args):
            try:
                max_hops = int(args[i + 1])
            except ValueError:
                pass
            i += 1
        elif arg == "--cache-mode" and i + 1 < len(args):
            cache_mode = args[i + 1]
            i += 1
        elif arg == "--truth-scope" and i + 1 < len(args):
            truth_scope = args[i + 1]
            i += 1
        elif arg == "--lineage" and i + 1 < len(args):
            lineage_ids.extend(v.strip() for v in args[i + 1].split(",") if v.strip())
            i += 1
        elif arg == "--no-proactive-truth":
            proactive_truth = False
        elif arg == "--json":
            as_json = True
        i += 1

    if cache_mode is not None:
        cache_error = hott_kernel._set_graph_cache_mode(cache_mode)
        if cache_error:
            _format_context_output(cache_error, as_json=as_json)
            return 2

    # Bind runtime memory/fiber to the target project before resolving an
    # automatic fiber-backed truth scope.  This keeps task identity from another
    # project from leaking into the current context projection.
    try:
        hott_kernel._bind_memory_scope_to_scan(root)
    except Exception:
        pass
    truth_scope = _resolve_truth_scope(truth_scope)

    res = hott_kernel.kernel_context(
        scan_root=root,
        query=query,
        target_files=target_files if target_files else None,
        budget_tokens=budget,
        max_hops=max_hops,
        detail="source",
        output_mode="prompt",
        proactive_truth=proactive_truth,
        truth_scope=truth_scope,
        parent_lineage_ids=lineage_ids,
    )
    _format_context_output(res, as_json=as_json)
    if "error" in res:
        return 1
    if (
        res.get("truth_floor", {}).get("status") == "recovery_required"
        or res.get("claim_obligations", {}).get("status") == "recovery_required"
    ):
        return 3
    return 0



def cmd_transitions(args: List[str]) -> int:
    """Compile valid proof-recovery transitions without executing them."""
    if not args or args[0] in ("-h", "--help"):
        print(
            "Usage: plw transitions <query> [root] [--target file[,file]] "
            "[--budget N] [--max-hops N] [--cache-mode auto|refresh|off] "
            "[--truth-scope ID] [--lineage sha256:id] [--no-proactive-truth] [--json]"
        )
        return 0 if args else 2

    query = args[0]
    root = "."
    target_files: List[str] = []
    budget = 1200
    max_hops = 2
    cache_mode: Optional[str] = None
    truth_scope: Optional[str] = None
    lineage_ids: List[str] = []
    proactive_truth = True
    as_json = False

    i = 1
    if i < len(args) and not args[i].startswith("-"):
        root = args[i]
        i += 1
    while i < len(args):
        arg = args[i]
        if arg in ("--target", "--targets") and i + 1 < len(args):
            target_files.extend(v.strip() for v in args[i + 1].split(",") if v.strip())
            i += 1
        elif arg in ("--budget", "--budget-tokens") and i + 1 < len(args):
            try:
                budget = int(args[i + 1])
            except ValueError:
                print("[PLW Error] --budget must be an integer", file=sys.stderr)
                return 2
            i += 1
        elif arg == "--max-hops" and i + 1 < len(args):
            try:
                max_hops = int(args[i + 1])
            except ValueError:
                print("[PLW Error] --max-hops must be an integer", file=sys.stderr)
                return 2
            i += 1
        elif arg == "--cache-mode" and i + 1 < len(args):
            cache_mode = args[i + 1]
            i += 1
        elif arg == "--truth-scope" and i + 1 < len(args):
            truth_scope = args[i + 1]
            i += 1
        elif arg == "--lineage" and i + 1 < len(args):
            lineage_ids.extend(v.strip() for v in args[i + 1].split(",") if v.strip())
            i += 1
        elif arg == "--no-proactive-truth":
            proactive_truth = False
        elif arg == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown transitions argument: {arg}", file=sys.stderr)
            return 2
        i += 1

    if cache_mode is not None:
        cache_error = hott_kernel._set_graph_cache_mode(cache_mode)
        if cache_error:
            print(json.dumps(cache_error, indent=2) if as_json else cache_error.get("error"), file=sys.stderr)
            return 2
    try:
        hott_kernel._bind_memory_scope_to_scan(root)
    except Exception:
        pass
    truth_scope = _resolve_truth_scope(truth_scope)

    result = hott_kernel.kernel_context(
        scan_root=root,
        query=query,
        target_files=target_files or None,
        budget_tokens=budget,
        max_hops=max_hops,
        detail="source",
        output_mode="prompt",
        proactive_truth=proactive_truth,
        truth_scope=truth_scope,
        parent_lineage_ids=lineage_ids,
    )
    if "error" in result:
        if as_json:
            print(json.dumps(result, indent=2, ensure_ascii=False))
        else:
            print(f"[PLW Error] {result.get('error')}", file=sys.stderr)
        return 1

    registry = result.get("proof_transitions", {}) or {}
    payload = {
        "schema_version": "plw-proof-transitions-v1",
        "query_fingerprint": (result.get("claim_obligations", {}).get("ledger", {}) or {}).get("id"),
        "truth_scope": result.get("truth_scope"),
        "context_status": result.get("context_status"),
        "claim_status": result.get("claim_obligations", {}).get("status"),
        "requested_claims": result.get("claim_obligations", {}).get("requested_claims", []),
        "proof_state": result.get("proof_state", {}),
        "proof_transitions": registry,
        "state_transition_model": result.get("state_transition_model", {}),
        "q_value_model": result.get("q_value_model", {}),
        "shadow_policy": result.get("shadow_policy", {}),
    }
    if as_json:
        print(json.dumps(payload, indent=2, ensure_ascii=False))
        return 0

    print("# [PLW Proof Transition Registry]")
    print(f"# Status: {registry.get('status')} | claims={','.join(registry.get('blocked_claims', [])) or 'none'}")
    proof_state = result.get("proof_state", {}) or {}
    if proof_state.get("state_class_id"):
        print(
            f"# Proof state: phase={proof_state.get('phase')} class={proof_state.get('state_class_id')} "
            f"instance={proof_state.get('state_instance_id')}"
        )
    for item in registry.get("candidates", []) or []:
        pref = "*" if item.get("preferred") else "-"
        cost = item.get("cost", {}) or {}
        print(
            f"{pref} {item.get('transition_type')} -> {item.get('authority_domain')} "
            f"[context={cost.get('context')} compute={cost.get('compute')} mutation={cost.get('mutation_risk')}]"
        )
        lineage_id = ((item.get("lineage") or {}).get("id"))
        if lineage_id:
            print(f"  lineage={lineage_id}")
        prior = item.get("outcome_memory_prior", {}) or {}
        if prior.get("available"):
            print(f"  global experience: attempts={prior.get('attempts')} settlement_rate={prior.get('settlement_rate')} (strategy-only)")
        state_prior = item.get("state_conditioned_prior", {}) or {}
        if state_prior.get("available"):
            print(
                f"  state-conditioned experience: attempts={state_prior.get('attempts')} "
                f"settlement_rate={state_prior.get('settlement_rate')} "
                "(P_hat strategy only; Markov sufficiency not proven)"
            )
        q_prior = item.get("q_value_prior", {}) or {}
        if q_prior.get("available"):
            print(
                f"  Q prior: visits={q_prior.get('visits')} q={q_prior.get('q_value')} "
                "(strategy value only; not truth confidence)"
            )
        print(f"  {item.get('command_hint')}")
    shadow = result.get("shadow_policy", {}) or {}
    shadow_pref = shadow.get("shadow_preferred_transition") or {}
    if shadow_pref:
        marker = "DIFFERS" if shadow.get("diverges_from_deterministic") else "same"
        print(
            f"# Shadow policy: {shadow_pref.get('transition_type')} "
            f"p={shadow_pref.get('policy_probability')} ({marker}; advisory-only)"
        )
        print("# Shadow probability is action preference, NOT truth confidence; deterministic registry ranking remains authoritative in v1.")
    ledger = registry.get("ledger", {}) or {}
    if ledger.get("id"):
        print(f"# AUDIT=plw accountability {ledger.get('id')}")
    print("# Registry is reference-only; execute externally and revalidate the claim gate afterward.")
    return 0

def cmd_kan(args: List[str]) -> int:
    if not args or args[0] in ("-h", "--help"):
        print("Usage: plw kan <target_module_or_query> [root] [--mode lan|ran|both] [--depth N] [--cache-mode auto|refresh|off] [--json]")
        return 0

    target = args[0]
    root = "."
    mode = "both"
    max_depth = 2
    cache_mode = None
    as_json = False

    i = 1
    if i < len(args) and not args[i].startswith("-"):
        root = args[i]
        i += 1

    while i < len(args):
        arg = args[i]
        if arg == "--mode" and i + 1 < len(args):
            mode = args[i + 1]
            i += 1
        elif arg in ("--depth", "--max-depth") and i + 1 < len(args):
            try:
                max_depth = int(args[i + 1])
            except ValueError:
                pass
            i += 1
        elif arg == "--cache-mode" and i + 1 < len(args):
            cache_mode = args[i + 1]
            i += 1
        elif arg == "--json":
            as_json = True
        i += 1

    if cache_mode is not None:
        cache_error = hott_kernel._set_graph_cache_mode(cache_mode)
        if cache_error:
            _format_kan_output(cache_error, as_json=as_json)
            return 2

    # Check if target is a module file or path
    is_module_target = (
        Path(target).exists()
        or "/" in target
        or any(target.endswith(ext) for ext in (".ts", ".js", ".tsx", ".jsx", ".py"))
    )
    if is_module_target:
        try:
            from memory.kan_extension import impute_module_relations

            # Reuse the same canonical cached SharedGraph used by context/brief/impact.
            # This keeps module-level Kan imputation inside the incremental reference
            # machine instead of reopening and reparsing every source file.
            shared_graph = hott_kernel._build_kernel_graph(root)
            res = impute_module_relations(target, shared_graph=shared_graph)
            if isinstance(res, dict):
                res.setdefault("graph_cache", shared_graph.get("cache", {}))
            _format_kan_output(res, as_json=as_json)
            return 0 if "error" not in res else 1
        except Exception as exc:
            # A path-like target is an explicit module-level request. Do not silently
            # reinterpret a failed module analysis as a memory query; surface the
            # failed transition so the agent can recover deterministically.
            res = {
                "error": "module_kan_failed",
                "error_code": "module_kan_failed",
                "target": target,
                "scan_root": root,
                "exception_type": type(exc).__name__,
                "message": str(exc),
            }
            _format_kan_output(res, as_json=as_json)
            return 1

    res = hott_kernel.kernel_memory_kan(target, mode=mode, max_depth=max_depth)
    _format_kan_output(res, as_json=as_json)
    return 0 if "error" not in res else 1


def cmd_check(args: List[str]) -> int:
    root = "."
    analyzers = None
    cache_mode = None
    as_json = False

    i = 0
    if i < len(args) and not args[i].startswith("-"):
        root = args[i]
        i += 1

    while i < len(args):
        arg = args[i]
        if arg == "--analyzers" and i + 1 < len(args):
            analyzers = args[i + 1].split(",")
            i += 1
        elif arg == "--cache-mode" and i + 1 < len(args):
            cache_mode = args[i + 1]
            i += 1
        elif arg == "--json":
            as_json = True
        i += 1

    if cache_mode is not None:
        cache_error = hott_kernel._set_graph_cache_mode(cache_mode)
        if cache_error:
            _format_check_output(cache_error, as_json=as_json)
            return 2

    res = hott_kernel.kernel_analyze(root, analyzer_names=analyzers, output_mode="summary")
    _format_check_output(res, as_json=as_json)
    return 0 if "error" not in res else 1


def cmd_steer(args: List[str]) -> int:
    root = "."
    cache_mode = None
    as_json = False
    i = 0
    if i < len(args) and not args[i].startswith("-"):
        root = args[i]
        i += 1

    while i < len(args):
        if args[i] == "--cache-mode" and i + 1 < len(args):
            cache_mode = args[i + 1]
            i += 1
        elif args[i] == "--json":
            as_json = True
        i += 1

    if cache_mode is not None:
        cache_error = hott_kernel._set_graph_cache_mode(cache_mode)
        if cache_error:
            _format_steer_output(cache_error, as_json=as_json)
            return 2

    # Cross-domain steer preferred if available
    if getattr(hott_kernel, "CROSS_DOMAIN_AVAILABLE", False):
        res = hott_kernel.kernel_xsteer(root, output_mode="full")
    else:
        res = hott_kernel.kernel_steer(root, output_mode="full")
    _format_steer_output(res, as_json=as_json)
    return 0 if "error" not in res else 1



def _format_command_output(result: Dict[str, Any], as_json: bool = False) -> None:
    if as_json or "error" in result:
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return

    raw = result.get("raw", {})
    compacted = result.get("compacted", {})
    recovery = result.get("recovery", {})
    print("# [PLW Command Output Observation]")
    print(f"# Policy: {result.get('policy')} | kind={result.get('kind')}")
    print(
        f"# Tokens: {raw.get('estimated_tokens', '?')} -> "
        f"{compacted.get('estimated_tokens', '?')} "
        f"(saved={compacted.get('estimated_tokens_saved', 0)}, "
        f"within_budget={compacted.get('within_budget')})"
    )
    print(
        "# Evidence: hard_truncation=false | "
        f"budget_omission={compacted.get('evidence_omitted_for_budget', False)}"
    )
    if recovery.get("available"):
        print(f"# RECOVER={recovery.get('command')}")
    elif recovery.get("id"):
        print(f"# Recovery unavailable for {recovery.get('id')}")
    accountability = result.get("accountability", {})
    ledger = accountability.get("ledger", {})
    if ledger.get("id"):
        print(f"# ACCOUNTABILITY={ledger.get('id')} owner={accountability.get('responsible_role')}")
        if ledger.get("available"):
            print(f"# AUDIT={ledger.get('command')}")
    print("-" * 60)
    output = result.get("output", "")
    if output:
        # Preserve output's own newline semantics as much as a terminal print can.
        sys.stdout.write(output)
        if not output.endswith("\n"):
            sys.stdout.write("\n")
    print("-" * 60)


def cmd_compact_output(args: List[str]) -> int:
    """Normalize command stdout from stdin without executing the command itself."""
    command = ""
    kind = "auto"
    budget = 1200
    as_json = False
    store = True
    telemetry = True
    store_dir: Optional[str] = None
    lineage_ids: List[str] = []

    i = 0
    while i < len(args):
        arg = args[i]
        if arg in ("-h", "--help"):
            print(
                "Usage: <command> | plw compact-output "
                "[--command CMD] [--kind auto|exact|search|repetitive|arbitrary] "
                "[--budget N] [--lineage sha256:id] [--no-store] [--no-telemetry] [--store-dir DIR] [--json]"
            )
            return 0
        if arg == "--command" and i + 1 < len(args):
            command = args[i + 1]
            i += 1
        elif arg == "--kind" and i + 1 < len(args):
            kind = args[i + 1].strip().lower()
            i += 1
        elif arg in ("--budget", "--budget-tokens") and i + 1 < len(args):
            try:
                budget = max(1, int(args[i + 1]))
            except ValueError:
                print("[PLW Error] --budget must be an integer", file=sys.stderr)
                return 2
            i += 1
        elif arg == "--lineage" and i + 1 < len(args):
            lineage_ids.extend(v.strip() for v in args[i + 1].split(",") if v.strip())
            i += 1
        elif arg == "--no-store":
            store = False
        elif arg == "--no-telemetry":
            telemetry = False
        elif arg == "--store-dir" and i + 1 < len(args):
            store_dir = args[i + 1]
            i += 1
        elif arg == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown compact-output argument: {arg}", file=sys.stderr)
            return 2
        i += 1

    if kind not in COMMAND_OUTPUT_KINDS:
        print(
            f"[PLW Error] Unsupported output kind '{kind}'. "
            f"Expected one of: {', '.join(COMMAND_OUTPUT_KINDS)}",
            file=sys.stderr,
        )
        return 2

    raw = sys.stdin.buffer.read()
    try:
        result = compact_command_output(
            raw,
            command=command,
            kind=kind,
            budget_tokens=budget,
            store=store,
            store_dir=store_dir,
            telemetry=telemetry,
            parent_lineage_ids=lineage_ids,
        )
    except Exception as exc:
        result = {
            "error": "command_output_compaction_failed",
            "exception_type": type(exc).__name__,
            "message": str(exc),
        }
        _format_command_output(result, as_json=True)
        return 1

    _format_command_output(result, as_json=as_json)
    return 0


def cmd_recover_output(args: List[str]) -> int:
    if not args or args[0] in ("-h", "--help"):
        print("Usage: plw recover-output <sha256:id> [--store-dir DIR] [--lineage sha256:id] [--no-telemetry]")
        return 0 if args else 2

    output_id = args[0]
    store_dir: Optional[str] = None
    telemetry = True
    lineage_ids: List[str] = []
    i = 1
    while i < len(args):
        if args[i] == "--store-dir" and i + 1 < len(args):
            store_dir = args[i + 1]
            i += 1
        elif args[i] == "--lineage" and i + 1 < len(args):
            lineage_ids.extend(v.strip() for v in args[i + 1].split(",") if v.strip())
            i += 1
        elif args[i] == "--no-telemetry":
            telemetry = False
        else:
            print(f"[PLW Error] Unknown recover-output argument: {args[i]}", file=sys.stderr)
            return 2
        i += 1

    try:
        if not lineage_ids:
            exact_parents = find_runtime_attestation_lineages_for_output(output_id)
            if len(exact_parents) == 1:
                lineage_ids = exact_parents
        raw = recover_raw_output(
            output_id, store_dir=store_dir, telemetry=telemetry,
            parent_lineage_ids=lineage_ids,
        )
    except (ValueError, FileNotFoundError) as exc:
        print(f"[PLW Error] {exc}", file=sys.stderr)
        return 1
    except Exception as exc:
        print(f"[PLW Error] Recovery failed: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1

    sys.stdout.buffer.write(raw)
    return 0


def cmd_attest_runtime(args: List[str]) -> int:
    """Bind an already-observed command output to a scoped runtime proof state."""
    if not args or args[0] in ("-h", "--help"):
        print(
            "Usage: plw attest-runtime <sha256:output-id> --exit-code N --command CMD "
            "--claim runtime_test_result|runtime_behavior [--root .] [--target file[,file]] "
            "[--truth-scope ID] [--cache-mode auto|refresh|off] "
            "[--output-store-dir DIR] [--attestation-store-dir DIR] [--json]"
        )
        return 0 if args else 2

    output_id = args[0]
    root = "."
    exit_code: Optional[int] = None
    command = ""
    claim_type = ""
    targets: List[str] = []
    truth_scope: Optional[str] = None
    cache_mode: Optional[str] = None
    output_store_dir: Optional[str] = None
    attestation_store_dir: Optional[str] = None
    as_json = False

    i = 1
    while i < len(args):
        arg = args[i]
        if arg == "--exit-code" and i + 1 < len(args):
            try:
                exit_code = int(args[i + 1])
            except ValueError:
                print("[PLW Error] --exit-code must be an integer", file=sys.stderr)
                return 2
            i += 1
        elif arg == "--command" and i + 1 < len(args):
            command = args[i + 1]
            i += 1
        elif arg == "--claim" and i + 1 < len(args):
            claim_type = args[i + 1].strip()
            i += 1
        elif arg == "--root" and i + 1 < len(args):
            root = args[i + 1]
            i += 1
        elif arg in ("--target", "--targets") and i + 1 < len(args):
            targets.extend(v.strip() for v in args[i + 1].split(",") if v.strip())
            i += 1
        elif arg == "--truth-scope" and i + 1 < len(args):
            truth_scope = args[i + 1]
            i += 1
        elif arg == "--cache-mode" and i + 1 < len(args):
            cache_mode = args[i + 1]
            i += 1
        elif arg == "--output-store-dir" and i + 1 < len(args):
            output_store_dir = args[i + 1]
            i += 1
        elif arg == "--attestation-store-dir" and i + 1 < len(args):
            attestation_store_dir = args[i + 1]
            i += 1
        elif arg == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown attest-runtime argument: {arg}", file=sys.stderr)
            return 2
        i += 1

    if exit_code is None or not command or not claim_type:
        print("[PLW Error] --exit-code, --command, and --claim are required", file=sys.stderr)
        return 2
    if cache_mode is not None:
        cache_error = hott_kernel._set_graph_cache_mode(cache_mode)
        if cache_error:
            print(json.dumps(cache_error, indent=2) if as_json else f"[PLW Error] {cache_error}")
            return 2
    try:
        hott_kernel._bind_memory_scope_to_scan(root)
    except Exception:
        pass
    truth_scope = _resolve_truth_scope(truth_scope)
    if not truth_scope:
        print("[PLW Error] Runtime attestation requires --truth-scope or an active fiber", file=sys.stderr)
        return 2

    try:
        graph = hott_kernel._build_kernel_graph(root)
        graph_sig = (graph.get("cache", {}) or {}).get("graph_content_signature")
        if not graph_sig and getattr(hott_kernel, "graph_content_signature", None):
            graph_sig = hott_kernel.graph_content_signature(graph)

        # Resolve explicit targets to canonical graph paths before attestation.
        # Ambiguous basename/path matches are rejected rather than widened.
        graph_paths = [str(v) for v in (graph.get("file_map", {}) or {}).keys()]
        canonical_targets: List[str] = []
        for target in targets:
            norm = os.path.normpath(target).replace("\\", "/")
            if norm.startswith("./"):
                norm = norm[2:]
            matches = [
                path for path in graph_paths
                if os.path.normpath(path).replace("\\", "/") == norm
                or os.path.normpath(path).replace("\\", "/").endswith("/" + norm)
            ]
            if len(matches) != 1:
                raise ValueError(f"Runtime attestation target must resolve uniquely: {target}")
            canonical_targets.append(matches[0])

        transition_target = canonical_targets[0] if canonical_targets else claim_type
        transition_id = transition_identity(claim_type, "obtain_runtime_validation", transition_target, graph_sig)
        source_binding = find_transition_intent(transition_id=transition_id, truth_scope=truth_scope)
        result = create_runtime_attestation(
            output_id=output_id,
            exit_code=exit_code,
            command=command,
            claim_type=claim_type,
            truth_scope=truth_scope,
            graph_content_signature=graph_sig,
            target_paths=canonical_targets,
            output_store_dir=output_store_dir,
            attestation_store_dir=attestation_store_dir,
            source_proof_state_binding=source_binding,
        )
    except Exception as exc:
        payload = {
            "error": "runtime_attestation_failed",
            "exception_type": type(exc).__name__,
            "message": str(exc),
        }
        print(json.dumps(payload, indent=2) if as_json else f"[PLW Error] {payload['message']}")
        return 1

    if as_json:
        print(json.dumps(result, indent=2))
    else:
        print("# [PLW Runtime Evidence Attestation]")
        print(f"# Attestation: {result.get('attestation_id')}")
        print(f"# Claim: {result.get('claim_type')} | exit_code={result.get('exit_code')}")
        print(f"# Scope: {result.get('truth_scope')}")
        print(f"# Graph: {result.get('graph_content_signature')}")
        print(f"# Output: {result.get('output_id')} (recoverable exact evidence)")
        state_binding = result.get("source_proof_state", {}) or {}
        if state_binding.get("available"):
            print(f"# Source proof state: {state_binding.get('source_state_class_id')} (policy provenance only)")
        print("# Authority: current runtime observation only; re-run claim validation after attestation.")
    return 0

def cmd_simplify(args: List[str]) -> int:
    root = "."
    reference: Optional[str] = None
    as_json = False
    i = 0
    positional = []
    while i < len(args):
        arg = args[i]
        if arg == "--json":
            as_json = True
        elif arg == "--reference" and i + 1 < len(args):
            reference = args[i + 1]
            i += 1
        elif arg in ("-h", "--help"):
            print("Usage: plw simplify [root] [--reference stable-root] [--json]")
            return 0
        elif arg.startswith("-"):
            print(f"[PLW Error] Unknown simplify argument: {arg}", file=sys.stderr)
            return 2
        else:
            positional.append(arg)
        i += 1
    if positional:
        root = positional[0]
    try:
        result = compare_simplification(reference, root) if reference else analyze_simplification(root)
    except Exception as exc:
        print(f"[PLW Error] simplify failed: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1

    if as_json:
        print(json.dumps(result, indent=2))
        return 0

    if reference:
        ref = result.get("reference", {}).get("metrics", {})
        cand = result.get("candidate", {}).get("metrics", {})
        delta = result.get("delta_candidate_minus_reference", {})
        print("# [PLW Self-Simplification Comparison]")
        print(f"# reference={reference} candidate={root}")
        print(
            "# duplicate debt: "
            f"{ref.get('estimated_removable_duplicate_loc', 0)} -> "
            f"{cand.get('estimated_removable_duplicate_loc', 0)} "
            f"(delta={delta.get('estimated_removable_duplicate_loc', 0)})"
        )
        print(
            "# exact duplicate groups: "
            f"{ref.get('exact_duplicate_groups', 0)} -> {cand.get('exact_duplicate_groups', 0)}"
        )
        print(
            "# total source LOC: "
            f"{ref.get('source_loc', 0)} -> {cand.get('source_loc', 0)} "
            f"(delta={delta.get('source_loc', 0)})"
        )
        candidates = result.get("candidate", {}).get("candidates", [])
    else:
        metrics = result.get("metrics", {})
        print("# [PLW Simplification Candidates]")
        print(
            f"# Python files={metrics.get('python_files', 0)} | "
            f"LOC={metrics.get('source_loc', 0)} | AST={metrics.get('ast_nodes', 0)} | "
            f"TS/JS semantic files={metrics.get('js_semantic_files_scanned', 0)} | "
            f"duplicate_groups={metrics.get('exact_duplicate_groups', 0)} | "
            f"duplicate_debt_loc={metrics.get('estimated_removable_duplicate_loc', 0)} | "
            f"semantic_candidates={metrics.get('semantic_candidates_total', 0)}"
        )
        bridge = result.get("canonical_anatomy_bridge", {}) or {}
        inventory = bridge.get("language_inventory", {}) or {}
        counts = inventory.get("reference_source_counts", {}) or {}
        observed = ",".join(f"{ext}={count}" for ext, count in counts.items() if count) or "none"
        pending = ",".join(inventory.get("reference_covered_but_semantic_rules_pending", []) or []) or "none"
        frontend_scope = bridge.get("semantic_frontend_scope", {}) or {}
        print(
            f"# Anatomy bridge: inventory={bridge.get('inventory_authority', 'unknown')} | "
            f"observed={observed} | semantic_frontend={bridge.get('semantic_frontend', 'unknown')} | "
            f"ts_js_rules={frontend_scope.get('ts_js_rule_count', 0)} | rules_pending={pending}"
        )
        if not inventory.get("zero_candidate_is_clean_evidence", True):
            print("# Coverage note: zero candidates is NOT clean evidence for source languages without an active semantic simplification frontend.")
        frontier = result.get("frontier", {}) or {}
        print(
            f"# Frontier: {frontier.get('status', 'unknown')} | "
            f"recommended_local_candidates={frontier.get('recommended_local_candidates', '?')} | "
            f"remaining={frontier.get('remaining_candidate_count', '?')}"
        )
        adjudication = result.get("architectural_adjudication", {}) or {}
        counts = adjudication.get("disposition_counts", {}) or {}
        if adjudication:
            print(
                f"# Adjudication: {adjudication.get('status', 'unknown')} | "
                f"KEEP={counts.get('KEEP', 0)} | DEFER={counts.get('DEFER', 0)} | "
                f"NEUTRAL_OWNER={counts.get('NEUTRAL_OWNER_CANDIDATE', 0)} | "
                f"API_PROOF={counts.get('API_PROOF_CANDIDATE', 0)}"
            )
        candidates = result.get("candidates", [])

    print("# Authority: structural_candidate_generation_only; equivalence requires separate proof validation.")
    for row in candidates[:10]:
        names = ",".join(row.get("names", []))
        files = ",".join(row.get("files", []))
        priority = row.get("priority", {}) or {}
        surface = row.get("proof_surface", {}) or {}
        save_loc = row.get("estimated_removable_loc", row.get("estimated_removable_duplicate_loc", 0))
        witness = row.get("semantic_witness", {}) or {}
        witness_text = f" | witness={witness.get('type')}" if witness else ""
        adjudication = row.get("adjudication", {}) or {}
        disposition_text = (
            f" | disposition={adjudication.get('disposition')}:{adjudication.get('owner_decision')}"
            if adjudication else ""
        )
        print(
            f"- priority={float(priority.get('score', 0.0)):.3f} | "
            f"kind={row.get('kind')} | "
            f"risk={surface.get('risk_level', 'unknown')}({surface.get('risk_units', '?')}) | "
            f"save≈{save_loc} LOC | "
            f"owners={surface.get('owner_file_count', len(row.get('files', [])))} | "
            f"domains={','.join(surface.get('owner_top_level_domains', [])) or '.'} | "
            f"neighbors={surface.get('direct_neighbor_count', '?')} | "
            f"lane={row.get('execution_lane', 'target')} | "
            f"recommend={priority.get('recommendation', 'unknown')} | "
            f"{names} | {files} | candidate={row.get('candidate_id')}{witness_text}{disposition_text}"
        )
    return 0


def cmd_lineage(args: List[str]) -> int:
    """Trace explicit causal ancestors without inferring semantic causality."""
    if not args or args[0] in ("-h", "--help"):
        print("Usage: plw lineage <sha256:id> [--json]")
        return 0 if args else 2
    lineage_id = args[0]
    as_json = "--json" in args[1:]
    unknown = [arg for arg in args[1:] if arg != "--json"]
    if unknown:
        print(f"[PLW Error] Unknown lineage argument: {unknown[0]}", file=sys.stderr)
        return 2
    try:
        result = trace_lineage(lineage_id)
    except Exception as exc:
        print(f"[PLW Error] {exc}", file=sys.stderr)
        return 1
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return 0
    print("# [PLW Causal Decision Lineage]")
    print(f"# Root: {result.get('root_lineage_id')} | nodes={result.get('node_count')}")
    for node in result.get("nodes", []) or []:
        print(f"- {node.get('node_type')} {node.get('lineage_id')} ref={node.get('reference_type')}:{node.get('reference_id')}")
    if result.get("missing_parent_ids"):
        print(f"# Missing parents: {','.join(result.get('missing_parent_ids', []))}")
    print("# Only explicit parent links are causal; absent links remain unknown.")
    return 0


def cmd_output_telemetry(args: List[str]) -> int:
    """Summarize local command-output behavior without exposing raw commands/output."""
    store_dir: Optional[str] = None
    as_json = False
    i = 0
    while i < len(args):
        arg = args[i]
        if arg in ("-h", "--help"):
            print("Usage: plw output-telemetry [--store-dir DIR] [--json]")
            return 0
        if arg == "--store-dir" and i + 1 < len(args):
            store_dir = args[i + 1]
            i += 1
        elif arg == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown output-telemetry argument: {arg}", file=sys.stderr)
            return 2
        i += 1

    try:
        result = get_command_output_telemetry(store_dir=store_dir)
    except Exception as exc:
        result = {
            "error": "command_output_telemetry_failed",
            "exception_type": type(exc).__name__,
            "message": str(exc),
        }
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return 1

    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return 0

    events = result.get("events", {})
    efficiency = result.get("efficiency", {})
    behavior = result.get("behavior", {})
    pressure = result.get("pressure", {})
    print("# [PLW Command Output Telemetry]")
    print(
        f"# Observations={events.get('observations', 0)} | Recoveries={events.get('recoveries', 0)} "
        f"| Invalid={events.get('invalid', 0)}"
    )
    print(
        f"# Estimated tokens saved={efficiency.get('estimated_tokens_saved', 0)} "
        f"(rate={efficiency.get('estimated_savings_rate', 0)}) | "
        f"over_budget_rate={efficiency.get('over_budget_rate', 0)}"
    )
    print(
        f"# Recovery rate={behavior.get('recovery_rate', 0)} | "
        f"repeat command rate={behavior.get('repeated_command_rate', 0)} | "
        f"repeat search rate={behavior.get('repeated_search_rate', 0)}"
    )
    print(
        f"# Pressure={pressure.get('status')} | "
        f"reasons={','.join(pressure.get('reasons', [])) or 'none'} | auto_action=none"
    )
    print("# Telemetry stores fingerprints/metrics only; raw command text and stdout are excluded.")
    return 0


def cmd_transition_memory(args: List[str]) -> int:
    """Inspect procedural transition-outcome memory; never use it as truth evidence."""
    as_json = False
    for arg in args:
        if arg in ("-h", "--help"):
            print("Usage: plw transition-memory [--json]"); return 0
        if arg == "--json": as_json = True; continue
        print(f"[PLW Error] Unknown transition-memory argument: {arg}", file=sys.stderr); return 2
    try: result = summarize_transition_outcomes()
    except Exception as exc:
        payload={"error":"transition_outcome_memory_failed","exception_type":type(exc).__name__,"message":str(exc)}
        print(json.dumps(payload,indent=2,ensure_ascii=False) if as_json else f"[PLW Error] {payload['message']}"); return 1
    if as_json: print(json.dumps(result,indent=2,ensure_ascii=False)); return 0
    print("# [PLW Transition Outcome Memory]")
    print(f"# Outcomes: {result.get('outcome_count',0)} | authority={result.get('authority')}")
    for item in result.get("by_transition",[]) or []:
        print(f"- {item.get('claim_type')}::{item.get('transition_type')} attempts={item.get('attempts')} settled={item.get('settled')} rate={item.get('settlement_rate')}")
    print("# Policy memory only: cannot satisfy a claim, change transition validity, or override source/runtime evidence.")
    return 0


def cmd_transition_model(args: List[str]) -> int:
    """Inspect empirical P_hat(S'|S,A) policy memory; never treat it as truth confidence."""
    as_json = False
    state_class: Optional[str] = None
    i = 0
    while i < len(args):
        arg = args[i]
        if arg in ("-h", "--help"):
            print("Usage: plw transition-model [--state-class sha256:...] [--json]"); return 0
        if arg == "--state-class" and i + 1 < len(args):
            state_class = args[i + 1]; i += 1
        elif arg == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown transition-model argument: {arg}", file=sys.stderr); return 2
        i += 1
    try:
        result = summarize_state_transition_model(source_state_class_id=state_class)
    except Exception as exc:
        payload={"error":"state_transition_model_failed","exception_type":type(exc).__name__,"message":str(exc)}
        print(json.dumps(payload,indent=2,ensure_ascii=False) if as_json else f"[PLW Error] {payload['message']}"); return 1
    if as_json:
        print(json.dumps(result,indent=2,ensure_ascii=False)); return 0
    print("# [PLW State-Conditioned Transition Model]")
    print(f"# Observations: {result.get('observation_count',0)} | authority={result.get('authority')}")
    print(f"# MDP status: {result.get('mdp_status')}")
    for item in result.get("state_action_models",[]) or []:
        print(f"- {item.get('source_state_class_id')} :: {item.get('transition_type')} attempts={item.get('attempts')} settled={item.get('settled')} rate={item.get('settlement_rate')}")
        for outcome in item.get("settlement_distribution",[]) or []:
            print(f"    outcome {outcome.get('settlement')} p_hat={outcome.get('empirical_probability')}")
        for dest in item.get("destination_distribution",[]) or []:
            print(f"    -> {dest.get('state_class_id')} p_hat={dest.get('empirical_probability')} phase={dest.get('phase')}")
    print("# Empirical action/outcome frequencies only: not truth confidence; Markov sufficiency is not proven.")
    return 0


def cmd_q_values(args: List[str]) -> int:
    """Inspect proof-constrained empirical Q/value policy memory."""
    as_json = False
    state_class: Optional[str] = None
    i = 0
    while i < len(args):
        arg = args[i]
        if arg in ("-h", "--help"):
            print("Usage: plw q-values [--state-class sha256:...] [--json]"); return 0
        if arg == "--state-class" and i + 1 < len(args):
            state_class = args[i + 1]; i += 1
        elif arg == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown q-values argument: {arg}", file=sys.stderr); return 2
        i += 1
    try:
        refresh_outcome_reconciliation()
        result = summarize_q_value_model(source_state_class_id=state_class)
    except Exception as exc:
        payload={"error":"q_value_model_failed","exception_type":type(exc).__name__,"message":str(exc)}
        print(json.dumps(payload,indent=2,ensure_ascii=False) if as_json else f"[PLW Error] {payload['message']}"); return 1
    if as_json:
        print(json.dumps(result,indent=2,ensure_ascii=False)); return 0
    print("# [PLW Proof-Constrained Q/Value Model]")
    print(f"# Samples: {result.get('sample_count',0)} | authority={result.get('authority')}")
    print(f"# Reward policy: {result.get('reward_policy')} | gamma={result.get('gamma')}")
    for item in result.get("state_action_values",[]) or []:
        print(
            f"- {item.get('source_state_class_id')} :: {item.get('transition_type')} "
            f"visits={item.get('visits')} reward={item.get('immediate_reward_mean')} q={item.get('q_value')}"
        )
    print("# Q/value estimates rank strategy only: not truth confidence, proof status, or transition legality.")
    return 0



def cmd_reconcile_outcomes(args: List[str]) -> int:
    """Inspect post-settlement recovery/rerun costs attributed to transition outcomes."""
    as_json = False
    record = False
    i = 0
    while i < len(args):
        arg = args[i]
        if arg in ("-h", "--help"):
            print("Usage: plw reconcile-outcomes [--record] [--json]")
            return 0
        if arg == "--json":
            as_json = True
        elif arg == "--record":
            record = True
        else:
            print(f"[PLW Error] Unknown reconcile-outcomes argument: {arg}", file=sys.stderr)
            return 2
        i += 1
    try:
        result = refresh_outcome_reconciliation()
        if record:
            result = {**result, "accountability": record_reconciliation_accountability()}
    except Exception as exc:
        payload = {"error":"outcome_reconciliation_failed","exception_type":type(exc).__name__,"message":str(exc)}
        print(json.dumps(payload, indent=2, ensure_ascii=False) if as_json else f"[PLW Error] {payload['message']}")
        return 1
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return 0
    print("# [PLW Delayed Outcome Cost Reconciliation]")
    print(f"# Outcomes: {result.get('outcome_count',0)} | with_delayed_cost={result.get('with_delayed_cost',0)}")
    for item in result.get("reconciliations", []) or []:
        delayed = item.get("delayed_cost", {}) or {}
        print(
            f"- {item.get('claim_type')}::{item.get('transition_type')} "
            f"reruns+={delayed.get('repeat_observations',0)} "
            f"recoveries+={delayed.get('output_recoveries',0)} "
            f"compacted_tokens+={delayed.get('compacted_tokens_observed',0)} "
            f"causal_context_tokens+={delayed.get('context_reexpansion_tokens',0)}"
        )
        owner = item.get("representation_accountability", {}) or {}
        if owner.get("available"):
            print(f"    representation_owner={owner.get('responsible_role')} decision={owner.get('decision_id')}")
    coverage = result.get("coverage", {}) or {}
    if coverage.get("context_reexpansion_attribution") == "explicit_parent_lineage_only":
        print("# Cross-step context cost is attributed only through explicit parent lineage; semantic similarity/timing never establish causality.")
    if record:
        print(f"# accountability={((result.get('accountability') or {}).get('decision_id'))}")
    return 0

def cmd_accountability_chain(args: List[str]) -> int:
    """Read a verified F111 relevance -> F112 delivery audit chain."""
    if not args or args[0] in ("-h", "--help"):
        print("Usage: plw accountability-chain <sha256:decision-id> [--store-dir DIR] [--limit N] [--json]")
        return 0 if args else 2

    decision_id = args[0]
    store_dir: Optional[str] = None
    limit = 16
    as_json = False
    i = 1
    while i < len(args):
        arg = args[i]
        if arg == "--store-dir" and i + 1 < len(args):
            store_dir = args[i + 1]
            i += 1
        elif arg == "--limit" and i + 1 < len(args):
            try:
                limit = int(args[i + 1])
            except ValueError:
                print("[PLW Error] --limit must be an integer", file=sys.stderr)
                return 2
            i += 1
        elif arg == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown accountability-chain argument: {arg}", file=sys.stderr)
            return 2
        i += 1

    try:
        result = query_accountability_chain(decision_id, store_dir=store_dir, delivery_limit=limit)
    except (ValueError, FileNotFoundError) as exc:
        print(f"[PLW Error] {exc}", file=sys.stderr)
        return 1
    except Exception as exc:
        print(f"[PLW Error] Accountability-chain read failed: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1

    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return 0

    rel = result.get("relevance", {})
    print("# [PLW Capability Workflow Accountability Chain]")
    print(f"# Relevance: {rel.get('decision_id')} | capability={rel.get('capability_id')} | relevant={rel.get('relevant')}")
    print(f"# State: {rel.get('workflow_state')} | decision={rel.get('relevance_decision')}")
    print(f"# Evidence: {rel.get('workflow_evidence_digest')} | request={rel.get('request_digest')}")
    print(f"# Deliveries: {result.get('delivery_count')} | truncated={result.get('deliveries_truncated')}")
    for row in result.get("deliveries", []):
        selected = " *selected*" if row.get("decision_id") == result.get("selected_delivery_id") else ""
        print(f"# - {row.get('delivery_outcome')} {row.get('decision_id')}{selected} reason={row.get('delivery_reason')} chars={row.get('context_block_chars')}")
    print(f"# Chain digest: {result.get('chain_digest')}")
    print("# Authority: audit query only; no execution, transition, patch, promotion, or truth authority.")
    return 0


def cmd_accountability(args: List[str]) -> int:
    """Read one content-filtering/compaction decision ledger."""
    if not args or args[0] in ("-h", "--help"):
        print("Usage: plw accountability <sha256:decision-id> [--store-dir DIR] [--json]")
        return 0 if args else 2

    decision_id = args[0]
    store_dir: Optional[str] = None
    as_json = False
    i = 1
    while i < len(args):
        arg = args[i]
        if arg == "--store-dir" and i + 1 < len(args):
            store_dir = args[i + 1]
            i += 1
        elif arg == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown accountability argument: {arg}", file=sys.stderr)
            return 2
        i += 1

    try:
        result = load_accountability(decision_id, store_dir=store_dir)
    except (ValueError, FileNotFoundError) as exc:
        print(f"[PLW Error] {exc}", file=sys.stderr)
        return 1
    except Exception as exc:
        print(f"[PLW Error] Accountability read failed: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1

    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return 0

    debt = result.get("evidence_debt", {})
    obligation = result.get("recovery_obligation", {})
    print("# [PLW Context Accountability Ledger]")
    print(f"# Decision: {result.get('decision_id')}")
    print(f"# Owner: {result.get('responsible_component')} ({result.get('responsible_role')})")
    print(f"# Policy: {result.get('policy')} | type={result.get('decision_type')}")
    print(
        "# Evidence debt: "
        f"files={debt.get('project_files_omitted', debt.get('candidate_files_omitted', 0))}, "
        f"lines={debt.get('source_excerpt_lines_omitted', 0)}, "
        f"memory={debt.get('memory_records_omitted', 0)}, "
        f"externalized_exact={debt.get('raw_exactness_externalized', False)}"
    )
    print(f"# Recovery obligation: {obligation.get('required_transition') or obligation.get('required_when')}")
    print("# Authority: representation decision only; omission is never proof of irrelevance/absence.")
    return 0


def _bundle_schema_diff(
    bundle: Dict[str, Any], required: set[str], optional: set[str] | None = None,
) -> tuple[List[str], List[str]]:
    """Return missing/unknown keys for one exact CLI JSON bundle."""
    allowed = required | (optional or set())
    return sorted(required - set(bundle)), sorted(set(bundle) - allowed)


def cmd_action_lease(args: List[str]) -> int:
    """Acquire or inspect the F121 atomic nonce-pair execution lease."""
    usage = (
        "Usage:\n"
        "  plw action-lease acquire [--store-dir DIR] "
        "--allow-state-change [--json] < bundle.json\n"
        "  plw action-lease status <sha256:lease-id> [--executor-id ID] "
        "[--store-dir DIR] [--json]"
    )
    if not args or args[0] in ("-h", "--help", "help"):
        print(usage)
        return 0 if args else 2

    operation = args[0].lower()
    if operation not in {"acquire", "status"}:
        print(f"[PLW Error] Unknown action-lease operation: {operation}", file=sys.stderr)
        print(usage, file=sys.stderr)
        return 2

    store_dir: Optional[str] = None
    executor_id: Optional[str] = None
    allow_state_change = False
    as_json = False
    lease_id: Optional[str] = None
    index = 1
    if operation == "status":
        if index >= len(args) or args[index].startswith("--"):
            print("[PLW Error] action-lease status requires a lease id", file=sys.stderr)
            return 2
        lease_id = args[index]
        index += 1
    while index < len(args):
        argument = args[index]
        if argument == "--store-dir" and index + 1 < len(args):
            store_dir = args[index + 1]
            index += 1
        elif argument == "--executor-id" and index + 1 < len(args):
            executor_id = args[index + 1]
            index += 1
        elif argument == "--allow-state-change":
            allow_state_change = True
        elif argument == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown action-lease argument: {argument}", file=sys.stderr)
            return 2
        index += 1

    if operation == "status":
        if allow_state_change:
            print("[PLW Error] status is read-only; remove --allow-state-change", file=sys.stderr)
            return 2
        result = inspect_action_lease(
            str(lease_id or ""),
            executor_id=executor_id,
            store_dir=store_dir,
        )
        if as_json:
            print(json.dumps(result, indent=2, ensure_ascii=False))
        else:
            print("# [PLW Atomic Action Lease Status]")
            print(f"# Lease: {result.get('lease_id', lease_id)}")
            print(f"# Decision: {result.get('decision', result.get('error'))}")
            print("# Authority: inspection only; it cannot regrant executor start.")
        return 0 if result.get("lease_found") else 3

    if executor_id is not None:
        print("[PLW Error] acquire binds executor_id from the JSON bundle", file=sys.stderr)
        return 2
    if not allow_state_change:
        result = {
            "error": "explicit_state_change_acknowledgement_required",
            "message": "action-lease acquire requires literal --allow-state-change",
            "source_mutation_authorized": False,
            "runtime_execution_authorized": False,
        }
        print(json.dumps(result, indent=2, ensure_ascii=False) if as_json else result["message"])
        return 3

    raw = sys.stdin.read(1_048_577)
    if len(raw) > 1_048_576:
        print(json.dumps({"error": "lease_bundle_too_large"}), file=sys.stderr)
        return 2
    try:
        bundle = json.loads(raw)
    except json.JSONDecodeError as exc:
        print(json.dumps({"error": "invalid_lease_bundle_json", "message": exc.msg}), file=sys.stderr)
        return 2
    if not isinstance(bundle, dict):
        print(json.dumps({"error": "lease_bundle_must_be_object"}), file=sys.stderr)
        return 2
    required = {
        "preflight_accountability_id", "preflight_request", "dry_run_request",
        "gate_request", "authorization_request", "proposal_request",
        "current_envelope", "executor_id", "idempotency_key",
    }
    allowed = required | {"lease_ttl_seconds"}
    missing = sorted(required - set(bundle))
    unknown = sorted(set(bundle) - allowed)
    if missing or unknown:
        print(json.dumps({
            "error": "invalid_lease_bundle_schema",
            "missing": missing,
            "unknown": unknown,
        }, indent=2), file=sys.stderr)
        return 2

    request = build_action_lease_request(
        str(bundle.get("preflight_accountability_id") or ""),
        bundle.get("preflight_request"),
        str(bundle.get("executor_id") or ""),
        str(bundle.get("idempotency_key") or ""),
        lease_ttl_seconds=bundle.get("lease_ttl_seconds", 15),
    )
    result = acquire_action_lease(
        request,
        bundle.get("preflight_request"),
        bundle.get("dry_run_request"),
        bundle.get("gate_request"),
        bundle.get("authorization_request"),
        bundle.get("proposal_request"),
        bundle.get("current_envelope"),
        store_dir=store_dir,
    )
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Atomic Action Lease]")
        print(f"# Decision: {result.get('decision')}")
        print(f"# Lease: {result.get('lease_id') or '-'}")
        print(f"# Executor start authorized by this call: {result.get('executor_start_authorized')}")
        print("# Actual transition/runtime/patch/promotion/truth effects remain false.")
    return 0 if (
        result.get("lease_acquired") is True
        or result.get("idempotent_recovery") is True
    ) else 3



def cmd_action_outcome(args: List[str]) -> int:
    """Register or inspect the F122 lease-bound executor outcome contract."""
    usage = (
        "Usage:\n"
        "  plw action-outcome register [--store-dir DIR] "
        "--allow-state-change [--json] < bundle.json\n"
        "  plw action-outcome record [--store-dir DIR] "
        "--allow-state-change [--json] < bundle.json\n"
        "  plw action-outcome status <sha256:lease-id> [--executor-id ID] "
        "[--store-dir DIR] [--json]"
    )
    if not args or args[0] in ("-h", "--help", "help"):
        print(usage)
        return 0 if args else 2

    operation = args[0].lower()
    if operation not in {"register", "record", "status"}:
        print(f"[PLW Error] Unknown action-outcome operation: {operation}", file=sys.stderr)
        print(usage, file=sys.stderr)
        return 2

    store_dir: Optional[str]
    executor_id: Optional[str]
    lease_id: Optional[str]
    store_dir = executor_id = lease_id = None
    allow_state_change = as_json = False
    index = 1
    if operation == "status":
        if index >= len(args) or args[index].startswith("--"):
            print("[PLW Error] action-outcome status requires a lease id", file=sys.stderr)
            return 2
        lease_id = args[index]
        index += 1
    while index < len(args):
        argument = args[index]
        if argument == "--store-dir" and index + 1 < len(args):
            store_dir = args[index + 1]
            index += 1
        elif argument == "--executor-id" and index + 1 < len(args):
            executor_id = args[index + 1]
            index += 1
        elif argument == "--allow-state-change":
            allow_state_change = True
        elif argument == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown action-outcome argument: {argument}", file=sys.stderr)
            return 2
        index += 1

    if operation == "status":
        if allow_state_change:
            print("[PLW Error] status is read-only; remove --allow-state-change", file=sys.stderr)
            return 2
        result = inspect_executor_outcome(
            str(lease_id or ""), executor_id=executor_id, store_dir=store_dir,
        )
        if as_json:
            print(json.dumps(result, indent=2, ensure_ascii=False))
        else:
            print("# [PLW Lease-Bound Executor Outcome Status]")
            print(f"# Lease: {result.get('lease_id', lease_id)}")
            print(f"# Decision: {result.get('decision', result.get('error'))}")
            print(f"# Terminal: {result.get('terminal_status')}")
            print("# Authority: inspection only; unresolved state is not success or failure.")
        return 0 if result.get("attempt_found") else 3

    if executor_id is not None:
        print(f"[PLW Error] {operation} binds executor_id from the JSON bundle", file=sys.stderr)
        return 2
    if not allow_state_change:
        result = {
            "error": "explicit_state_change_acknowledgement_required",
            "message": f"action-outcome {operation} requires literal --allow-state-change",
            "source_mutation_authorized": False,
            "runtime_execution_authorized": False,
        }
        print(json.dumps(result, indent=2, ensure_ascii=False) if as_json else result["message"])
        return 3

    raw = sys.stdin.read(1_048_577)
    if len(raw) > 1_048_576:
        print(json.dumps({"error": "action_outcome_bundle_too_large"}), file=sys.stderr)
        return 2
    try:
        bundle = json.loads(raw)
    except json.JSONDecodeError as exc:
        print(json.dumps({"error": "invalid_action_outcome_bundle_json", "message": exc.msg}), file=sys.stderr)
        return 2
    if not isinstance(bundle, dict):
        print(json.dumps({"error": "action_outcome_bundle_must_be_object"}), file=sys.stderr)
        return 2

    if operation == "register":
        required = {"lease_id", "executor_id", "attempt_key"}
        missing = sorted(required - set(bundle))
        unknown = sorted(set(bundle) - required)
        if missing or unknown:
            print(json.dumps({
                "error": "invalid_executor_attempt_bundle_schema",
                "missing": missing,
                "unknown": unknown,
            }, indent=2), file=sys.stderr)
            return 2
        request = build_executor_attempt_request(
            str(bundle.get("lease_id") or ""),
            str(bundle.get("executor_id") or ""),
            str(bundle.get("attempt_key") or ""),
        )
        result = register_executor_attempt(request, store_dir=store_dir)
        if as_json:
            print(json.dumps(result, indent=2, ensure_ascii=False))
        else:
            print("# [PLW Lease-Bound Executor Attempt]")
            print(f"# Decision: {result.get('decision')}")
            print(f"# Attempt: {result.get('attempt_id')}")
            print("# Boundary: registration is a durable handoff, not proof the external action started.")
        return 0 if result.get("attempt_registered") or result.get("idempotent_recovery") else 3

    required = {
        "lease_id", "executor_id", "attempt_id", "terminal_status",
        "observation_kind", "observation",
    }
    missing, unknown = _bundle_schema_diff(bundle, required, {"evidence_ids"})
    if missing or unknown:
        print(json.dumps({
            "error": "invalid_executor_outcome_bundle_schema",
            "missing": missing,
            "unknown": unknown,
        }, indent=2), file=sys.stderr)
        return 2
    if not isinstance(bundle.get("observation"), dict):
        print(json.dumps({"error": "executor_outcome_observation_must_be_object"}), file=sys.stderr)
        return 2
    evidence_ids = bundle.get("evidence_ids", [])
    if not isinstance(evidence_ids, list):
        print(json.dumps({"error": "executor_outcome_evidence_ids_must_be_array"}), file=sys.stderr)
        return 2
    request = build_executor_outcome_request(
        str(bundle.get("lease_id") or ""),
        str(bundle.get("executor_id") or ""),
        str(bundle.get("attempt_id") or ""),
        str(bundle.get("terminal_status") or ""),
        str(bundle.get("observation_kind") or ""),
        bundle.get("observation"),
        evidence_ids=evidence_ids,
    )
    result = record_executor_outcome(
        request, bundle.get("observation"), store_dir=store_dir,
    )
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Lease-Bound Executor Outcome]")
        print(f"# Decision: {result.get('decision')}")
        print(f"# Terminal: {result.get('terminal_status')}")
        print("# Boundary: executor terminal report is not postcondition proof or truth settlement.")
    return 0 if result.get("outcome_recorded") or result.get("idempotent_recovery") else 3


def cmd_action_adapter(args: List[str]) -> int:
    """Prepare, observe, or inspect the F123 executor-adapter bridge."""
    usage = (
        "Usage:\n"
        "  plw action-adapter prepare [--store-dir DIR] "
        "--allow-state-change [--json] < bundle.json\n"
        "  plw action-adapter observe [--store-dir DIR] "
        "--allow-state-change [--json] < bundle.json\n"
        "  plw action-adapter status <sha256:lease-id> "
        "[--store-dir DIR] [--json]"
    )
    if not args or args[0] in ("-h", "--help", "help"):
        print(usage)
        return 0 if args else 2

    operation = args[0].lower()
    if operation not in {"prepare", "observe", "status"}:
        print(f"[PLW Error] Unknown action-adapter operation: {operation}", file=sys.stderr)
        print(usage, file=sys.stderr)
        return 2

    store_dir: Optional[str] = None
    lease_id: Optional[str] = None
    allow_state_change = as_json = False
    index = 1
    if operation == "status":
        if index >= len(args) or args[index].startswith("--"):
            print("[PLW Error] action-adapter status requires a lease id", file=sys.stderr)
            return 2
        lease_id = args[index]
        index += 1
    while index < len(args):
        argument = args[index]
        if argument == "--store-dir" and index + 1 < len(args):
            store_dir = args[index + 1]
            index += 1
        elif argument == "--allow-state-change":
            allow_state_change = True
        elif argument == "--json":
            as_json = True
        else:
            print(f"[PLW Error] Unknown action-adapter argument: {argument}", file=sys.stderr)
            return 2
        index += 1

    if operation == "status":
        if allow_state_change:
            print("[PLW Error] status is read-only; remove --allow-state-change", file=sys.stderr)
            return 2
        result = inspect_adapter_dispatch(str(lease_id or ""), store_dir=store_dir)
        if as_json:
            print(json.dumps(result, indent=2, ensure_ascii=False))
        else:
            print("# [PLW Executor Adapter Bridge Status]")
            print(f"# Lease: {result.get('lease_id', lease_id)}")
            print(f"# Decision: {result.get('decision', result.get('error'))}")
            print(f"# F122 terminal: {result.get('f122_terminal_status')}")
            print("# Authority: inspection only; missing adapter observation stays unresolved.")
        return 0 if result.get("dispatch_found") else 3

    if not allow_state_change:
        result = {
            "error": "explicit_state_change_acknowledgement_required",
            "message": f"action-adapter {operation} requires literal --allow-state-change",
            "source_mutation_authorized": False,
            "runtime_execution_authorized": False,
        }
        print(json.dumps(result, indent=2, ensure_ascii=False) if as_json else result["message"])
        return 3

    raw = sys.stdin.read(1_048_577)
    if len(raw) > 1_048_576:
        print(json.dumps({"error": "action_adapter_bundle_too_large"}), file=sys.stderr)
        return 2
    try:
        bundle = json.loads(raw)
    except json.JSONDecodeError as exc:
        print(json.dumps({"error": "invalid_action_adapter_bundle_json", "message": exc.msg}), file=sys.stderr)
        return 2
    if not isinstance(bundle, dict):
        print(json.dumps({"error": "action_adapter_bundle_must_be_object"}), file=sys.stderr)
        return 2

    if operation == "prepare":
        required = {"lease_id", "attempt_id", "executor_id", "adapter_id", "operation_key"}
        missing = sorted(required - set(bundle))
        unknown = sorted(set(bundle) - required)
        if missing or unknown:
            print(json.dumps({
                "error": "invalid_adapter_dispatch_bundle_schema",
                "missing": missing,
                "unknown": unknown,
            }, indent=2), file=sys.stderr)
            return 2
        request = build_adapter_dispatch_request(
            str(bundle.get("lease_id") or ""),
            str(bundle.get("attempt_id") or ""),
            str(bundle.get("executor_id") or ""),
            str(bundle.get("adapter_id") or ""),
            str(bundle.get("operation_key") or ""),
        )
        result = prepare_adapter_dispatch(request, store_dir=store_dir)
        if as_json:
            print(json.dumps(result, indent=2, ensure_ascii=False))
        else:
            print("# [PLW Executor Adapter Dispatch Preparation]")
            print(f"# Decision: {result.get('decision')}")
            print(f"# Dispatch: {result.get('dispatch_id')}")
            print("# Boundary: durable dispatch intent is not proof external execution started.")
        return 0 if result.get("dispatch_prepared") or result.get("idempotent_recovery") else 3

    required = {
        "dispatch_receipt", "dispatch_state", "terminal_status",
        "observation_kind", "observation",
    }
    missing, unknown = _bundle_schema_diff(bundle, required, {"evidence_ids"})
    if missing or unknown:
        print(json.dumps({
            "error": "invalid_adapter_observation_bundle_schema",
            "missing": missing,
            "unknown": unknown,
        }, indent=2), file=sys.stderr)
        return 2
    if not isinstance(bundle.get("dispatch_receipt"), dict):
        print(json.dumps({"error": "adapter_dispatch_receipt_must_be_object"}), file=sys.stderr)
        return 2
    if not isinstance(bundle.get("observation"), dict):
        print(json.dumps({"error": "adapter_observation_must_be_object"}), file=sys.stderr)
        return 2
    evidence_ids = bundle.get("evidence_ids", [])
    if not isinstance(evidence_ids, list):
        print(json.dumps({"error": "adapter_evidence_ids_must_be_array"}), file=sys.stderr)
        return 2
    envelope = build_adapter_observation(
        dispatch_receipt=bundle["dispatch_receipt"],
        dispatch_state=str(bundle.get("dispatch_state") or ""),
        terminal_status=str(bundle.get("terminal_status") or "") or None,
        observation_kind=str(bundle.get("observation_kind") or ""),
        observation=bundle.get("observation"),
        evidence_ids=evidence_ids,
    )
    result = record_adapter_observation(
        envelope, bundle.get("observation"), store_dir=store_dir,
    )
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Executor Adapter Observation]")
        print(f"# Decision: {result.get('decision')}")
        print(f"# Reconciliation: {result.get('reconciliation', {}).get('reconciliation_state')}")
        print("# Boundary: adapter observation may feed F122; it does not prove postconditions or truth.")
    return 0 if result.get("observation_recorded") or result.get("idempotent_recovery") else 3


def cmd_angular_anchors(args: List[str]) -> int:
    """Read-only Angular ownership sidecar over one canonical SharedGraph snapshot."""
    root = "."
    file_filter: Optional[str] = None
    selector_filter: Optional[str] = None
    read_resources = True
    as_json = False
    cache_mode: Optional[str] = None
    positional: List[str] = []
    i = 0
    while i < len(args):
        arg = args[i]
        if arg == "--json":
            as_json = True
            i += 1
            continue
        if arg == "--no-resources":
            read_resources = False
            i += 1
            continue
        if arg in ("--file", "--selector", "--cache-mode"):
            if i + 1 >= len(args):
                print(f"[PLW Error] {arg} requires a value", file=sys.stderr)
                return 2
            value = args[i + 1]
            if arg == "--file":
                file_filter = value
            elif arg == "--selector":
                selector_filter = value
            else:
                cache_mode = value
            i += 2
            continue
        if arg.startswith("-"):
            print(f"[PLW Error] Unknown angular-anchors argument: {arg}", file=sys.stderr)
            return 2
        positional.append(arg)
        i += 1
    if len(positional) > 1:
        print("[PLW Error] angular-anchors accepts at most one root path", file=sys.stderr)
        return 2
    if positional:
        root = positional[0]
    if cache_mode is not None:
        cache_error = hott_kernel._set_graph_cache_mode(cache_mode)
        if cache_error:
            print(json.dumps(cache_error, ensure_ascii=False) if as_json else f"[PLW Error] {cache_error.get('message')}", file=sys.stderr)
            return 2
    graph = hott_kernel._build_kernel_graph(root)
    result = analyze_angular_ownership(
        graph,
        file_filter=file_filter,
        selector_filter=selector_filter,
        read_resources=read_resources,
    )
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        summary = result.get("summary", {})
        print("# [PLW Angular Ownership Anchors]")
        print(f"# Components: {summary.get('component_count', 0)} | selectors: {summary.get('selector_count', 0)} | unresolved: {summary.get('unresolved_component_count', 0)}")
        for row in result.get("components", []):
            selector = row.get("selector", {}).get("value") or "<unresolved-selector>"
            template = row.get("template", {})
            template_label = template.get("resolved_path") or template.get("kind")
            print(f"{selector} -> {row.get('class_name')} @ {row.get('file')} | template={template_label} | proof={row.get('proof_class')}")
        print("# Boundary: static ownership only; no runtime, geometry, behavior, mutation, or truth authority.")
    return 0



def _parse_ui_reference_args(
    args: List[str], *, command: str, allow_store_dir: bool,
) -> Optional[Dict[str, Any]]:
    cache_mode: Optional[str] = None
    store_dir: Optional[str] = None
    as_json = False
    read_resources = True
    positional: List[str] = []
    i = 0
    value_flags = {"--cache-mode"}
    if allow_store_dir:
        value_flags.add("--store-dir")
    while i < len(args):
        arg = args[i]
        if arg == "--json":
            as_json = True
            i += 1
            continue
        if arg == "--no-resources":
            read_resources = False
            i += 1
            continue
        if arg in value_flags:
            if i + 1 >= len(args):
                print(f"[PLW Error] {arg} requires a value", file=sys.stderr)
                return None
            if arg == "--cache-mode":
                cache_mode = args[i + 1]
            else:
                store_dir = args[i + 1]
            i += 2
            continue
        if arg.startswith("-"):
            print(f"[PLW Error] Unknown {command} argument: {arg}", file=sys.stderr)
            return None
        positional.append(arg)
        i += 1
    return {
        "cache_mode": cache_mode,
        "store_dir": store_dir,
        "as_json": as_json,
        "read_resources": read_resources,
        "positional": positional,
    }


def _apply_ui_reference_cache_mode(cache_mode: Optional[str], *, as_json: bool) -> bool:
    if cache_mode is None:
        return True
    cache_error = hott_kernel._set_graph_cache_mode(cache_mode)
    if not cache_error:
        return True
    print(json.dumps(cache_error, ensure_ascii=False) if as_json else f"[PLW Error] {cache_error.get('message')}", file=sys.stderr)
    return False


def _resolve_geometry_query_positionals(parsed: Dict[str, Any], command: str) -> Optional[tuple[str, str, bool]]:
    positional = parsed["positional"]
    if not positional or len(positional) > 2:
        print(f"[PLW Error] {command} requires <snapshot.json> and accepts at most one root path", file=sys.stderr)
        return None
    snapshot_path = positional[0]
    root = positional[1] if len(positional) == 2 else "."
    return snapshot_path, root, bool(parsed["as_json"])


def _load_ui_reference_inputs(snapshot_path: str, root: str, *, read_resources: bool):
    payload = load_geometry_snapshot(snapshot_path)
    graph = hott_kernel._build_kernel_graph(root)
    ownership = analyze_angular_ownership(graph, read_resources=read_resources)
    return payload, graph, ownership


def _parse_ui_map_args(args: List[str], *, inspect: bool = False) -> Optional[Dict[str, Any]]:
    parsed: Dict[str, Any] = {
        "as_json": False, "read_resources": True, "cache_mode": None,
        "min_gap_px": 0.0, "max_elements": 200, "max_issues": 100,
        "positional": [],
    }
    i = 0
    while i < len(args):
        arg = args[i]
        if arg == "--json": parsed["as_json"] = True; i += 1; continue
        if arg == "--no-resources": parsed["read_resources"] = False; i += 1; continue
        if arg in {"--min-gap", "--max-elements", "--max-issues", "--cache-mode"}:
            if i + 1 >= len(args):
                print(f"[PLW Error] {arg} requires a value", file=sys.stderr); return None
            value = args[i+1]
            try:
                if arg == "--min-gap": parsed["min_gap_px"] = float(value)
                elif arg == "--max-elements": parsed["max_elements"] = int(value)
                elif arg == "--max-issues": parsed["max_issues"] = int(value)
                else: parsed["cache_mode"] = value
            except ValueError:
                print(f"[PLW Error] invalid value for {arg}: {value}", file=sys.stderr); return None
            i += 2; continue
        if arg.startswith("-"):
            print(f"[PLW Error] Unknown ui {'inspect' if inspect else 'map'} argument: {arg}", file=sys.stderr); return None
        parsed["positional"].append(arg); i += 1
    needed = 2 if inspect else 1
    max_pos = 3 if inspect else 2
    if len(parsed["positional"]) < needed or len(parsed["positional"]) > max_pos:
        usage = "<snapshot.json> <ui-id> [root]" if inspect else "<snapshot.json> [root]"
        print(f"[PLW Error] ui {'inspect' if inspect else 'map'} requires {usage}", file=sys.stderr); return None
    return parsed


def _build_practical_ui_map(parsed: Dict[str, Any], *, inspect_id: Optional[str] = None) -> Dict[str, Any]:
    positional = parsed["positional"]
    snapshot_path = positional[0]
    root = positional[2] if inspect_id is not None and len(positional) == 3 else (positional[1] if inspect_id is None and len(positional) == 2 else ".")
    if not _apply_ui_reference_cache_mode(parsed.get("cache_mode"), as_json=bool(parsed.get("as_json"))):
        raise UIMapperError("invalid graph cache mode")
    payload, graph, ownership = _load_ui_reference_inputs(snapshot_path, root, read_resources=bool(parsed["read_resources"]))
    ui_reference = build_ui_reference_state(payload, ownership)
    max_elements = 50000 if inspect_id is not None else int(parsed["max_elements"])
    return build_ui_map(
        payload, ui_reference=ui_reference, angular_ownership=ownership,
        min_gap_px=float(parsed["min_gap_px"]), max_elements=max_elements,
        max_issues=int(parsed["max_issues"]),
    )


def cmd_ui_map(args: List[str]) -> int:
    parsed = _parse_ui_map_args(args)
    if parsed is None: return 2
    try:
        result = _build_practical_ui_map(parsed)
    except (GeometrySnapshotError, UIMapperError) as exc:
        print(json.dumps({"schema_version":"ui-map-v1","error":str(exc),"error_code":"invalid_ui_map_request"}, indent=2) if parsed["as_json"] else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if parsed["as_json"]:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        summary=result.get("summary",{}); scenario=result.get("scenario",{})
        print("# [PLW UI Map]")
        print(f"# Route: {scenario.get('route')} | viewport={scenario.get('viewport')}")
        print(f"# Elements: {summary.get('node_count',0)} | visible={summary.get('visible_node_count',0)} | roots={summary.get('root_count',0)}")
        print(f"# Issues: {summary.get('issue_count',0)} | by kind={summary.get('issues_by_kind',{})}")
        for row in result.get("issues",[])[:12]:
            print(f"# {row.get('severity')} {row.get('kind')}: {', '.join(row.get('ui_ids',[]))}")
        print("# Boundary: derived UI spatial map; no browser execution, source mutation, behavior proof, or truth commit.")
    return 0


def cmd_ui_inspect(args: List[str]) -> int:
    parsed = _parse_ui_map_args(args, inspect=True)
    if parsed is None: return 2
    ui_id = str(parsed["positional"][1])
    try:
        ui_map = _build_practical_ui_map(parsed, inspect_id=ui_id)
        result = inspect_ui_element(ui_map, ui_id)
    except (GeometrySnapshotError, UIMapperError) as exc:
        print(json.dumps({"schema_version":"ui-inspect-v1","error":str(exc),"error_code":"invalid_ui_inspect_request"}, indent=2) if parsed["as_json"] else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if parsed["as_json"]: print(json.dumps(result,indent=2,ensure_ascii=False))
    else:
        el=result.get("element",{})
        print("# [PLW UI Inspect]")
        print(f"# UI: {ui_id} | tag={el.get('tag_name')} | bbox={el.get('bbox')}")
        print(f"# Hierarchy: {' > '.join(result.get('hierarchy_path',[]))}")
        print(f"# Code owner: {el.get('code_owner')}")
        print(f"# Related issues: {len(result.get('related_issues',[]))}")
    return 0


def cmd_ui_scenarios(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui scenarios")
    parser.add_argument("snapshots", nargs="+")
    parser.add_argument("--root", default=".")
    parser.add_argument("--min-gap", type=float, default=0.0)
    parser.add_argument("--max-elements", type=int, default=500)
    parser.add_argument("--max-issues", type=int, default=100)
    parser.add_argument("--max-scenarios", type=int, default=32)
    parser.add_argument("--max-issue-patterns", type=int, default=200)
    parser.add_argument("--cache-mode")
    parser.add_argument("--no-resources", action="store_true")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(tuple(args))
    if len(opts.snapshots) < 2:
        print("[PLW Error] ui scenarios requires at least two snapshot files", file=sys.stderr)
        return 2
    if not _apply_ui_reference_cache_mode(opts.cache_mode, as_json=opts.as_json):
        return 2
    try:
        graph = hott_kernel._build_kernel_graph(opts.root)
        ownership = analyze_angular_ownership(graph, read_resources=not opts.no_resources)
        payloads = [load_geometry_snapshot(path) for path in opts.snapshots]
        refs = [build_ui_reference_state(payload, ownership) for payload in payloads]
        result = build_ui_scenario_map(
            payloads, ui_references=refs, angular_ownership=ownership,
            min_gap_px=opts.min_gap, max_elements=opts.max_elements,
            max_issues=opts.max_issues, max_scenarios=opts.max_scenarios,
            max_issue_patterns=opts.max_issue_patterns,
        )
    except (GeometrySnapshotError, UIMapperError) as exc:
        payload = {"schema_version":"ui-scenario-map-v2","error":str(exc),"error_code":"invalid_ui_scenario_map_request"}
        print(json.dumps(payload, indent=2) if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        summary=result.get("summary",{}); axes=result.get("scenario_axes",{})
        print("# [PLW UI Scenario Map V2]")
        print(f"# Revision: {result.get('code_revision')} | scenarios={result.get('scenario_count',0)}")
        print(f"# Routes: {axes.get('routes',[])} | viewports={axes.get('viewports',[])}")
        print(f"# UI IDs: {summary.get('unique_ui_id_count',0)} | conditional={summary.get('conditional_ui_id_count',0)} | major shifts={summary.get('major_layout_shift_count',0)}")
        print(f"# Issue patterns: {summary.get('issue_pattern_count',0)} | scenario-specific={summary.get('scenario_specific_issue_pattern_count',0)}")
        print("# Boundary: one-revision scenario/responsive mapping only; no cross-revision regression claim, browser execution, mutation, or truth commit.")
    return 0


def cmd_ui_diff(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui diff")
    parser.add_argument("baseline")
    parser.add_argument("candidate")
    parser.add_argument("--root", default=".", help="candidate revision project root")
    parser.add_argument("--baseline-root", help="optional source tree matching baseline revision")
    parser.add_argument("--min-gap", type=float, default=0.0)
    parser.add_argument("--move-threshold", type=float, default=2.0)
    parser.add_argument("--size-threshold", type=float, default=2.0)
    parser.add_argument("--max-changes", type=int, default=500)
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("--cache-mode")
    parser.add_argument("--no-resources", action="store_true")
    opts = parser.parse_args(args)
    if not _apply_ui_reference_cache_mode(opts.cache_mode, as_json=opts.as_json):
        return 2
    try:
        baseline = load_geometry_snapshot(opts.baseline)
        candidate = load_geometry_snapshot(opts.candidate)
        candidate_graph = hott_kernel._build_kernel_graph(opts.root)
        candidate_ownership = analyze_angular_ownership(candidate_graph, read_resources=not opts.no_resources)
        candidate_ref = build_ui_reference_state(candidate, candidate_ownership)
        baseline_ownership = None
        baseline_ref = None
        if opts.baseline_root:
            baseline_graph = hott_kernel._build_kernel_graph(opts.baseline_root)
            baseline_ownership = analyze_angular_ownership(baseline_graph, read_resources=not opts.no_resources)
            baseline_ref = build_ui_reference_state(baseline, baseline_ownership)
        result = build_ui_geometry_diff(
            baseline, candidate,
            baseline_ui_reference=baseline_ref,
            candidate_ui_reference=candidate_ref,
            baseline_angular_ownership=baseline_ownership,
            candidate_angular_ownership=candidate_ownership,
            min_gap_px=opts.min_gap,
            move_threshold_px=opts.move_threshold,
            size_threshold_px=opts.size_threshold,
            max_changes=opts.max_changes,
        )
    except (GeometrySnapshotError, UIMapperError) as exc:
        payload = {"schema_version":"ui-geometry-diff-v3","error":str(exc),"error_code":"invalid_ui_geometry_diff_request"}
        print(json.dumps(payload, indent=2) if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        summary = result.get("summary", {})
        identity = result.get("scenario_identity", {})
        print("# [PLW UI Geometry Diff V3]")
        print(f"# Revisions: {result.get('baseline_revision')} -> {result.get('candidate_revision')}")
        print(f"# Scenario: {identity.get('route')} | viewport={identity.get('viewport')} | state={identity.get('ui_state')} | fixture={identity.get('data_fixture')}")
        print(f"# Changed UI IDs: {summary.get('changed_ui_id_count',0)} | changes={summary.get('total_change_count',0)}")
        print(f"# New issues: {summary.get('introduced_issue_count',0)} | resolved issues={summary.get('resolved_issue_count',0)}")
        print(f"# By kind: {summary.get('changes_by_kind',{})}")
        print("# Boundary: geometry change facts only; no automatic bug/regression verdict, mutation, or truth commit.")
    return 0


def cmd_ui_hunt(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui hunt")
    parser.add_argument("candidate")
    parser.add_argument("--baseline")
    parser.add_argument("--root", default=".", help="candidate revision project root")
    parser.add_argument("--baseline-root", help="optional source tree matching baseline revision")
    parser.add_argument("--runtime-reference", help="optional runtime-ui-reference JSON for exact capture provenance")
    parser.add_argument("--min-gap", type=float, default=0.0)
    parser.add_argument("--max-candidates", type=int, default=100)
    parser.add_argument("--cache-mode")
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("--no-resources", action="store_true")
    opts = parser.parse_args(tuple(args))
    if not _apply_ui_reference_cache_mode(opts.cache_mode, as_json=opts.as_json):
        return 2
    try:
        candidate = load_geometry_snapshot(opts.candidate)
        candidate_graph = hott_kernel._build_kernel_graph(opts.root)
        candidate_ownership = analyze_angular_ownership(candidate_graph, read_resources=not opts.no_resources)
        candidate_ref = build_ui_reference_state(candidate, candidate_ownership)
        baseline = None
        baseline_ownership = None
        baseline_ref = None
        if opts.baseline:
            baseline = load_geometry_snapshot(opts.baseline)
            if opts.baseline_root:
                baseline_graph = hott_kernel._build_kernel_graph(opts.baseline_root)
                baseline_ownership = analyze_angular_ownership(baseline_graph, read_resources=not opts.no_resources)
                baseline_ref = build_ui_reference_state(baseline, baseline_ownership)
        runtime_reference = None
        if opts.runtime_reference:
            runtime_reference = json.loads(Path(opts.runtime_reference).read_text(encoding="utf-8"))
        result = build_ui_bug_hunt(
            candidate,
            candidate_ui_reference=candidate_ref,
            candidate_angular_ownership=candidate_ownership,
            baseline_snapshot=baseline,
            baseline_ui_reference=baseline_ref,
            baseline_angular_ownership=baseline_ownership,
            runtime_reference=runtime_reference,
            min_gap_px=opts.min_gap,
            max_candidates=opts.max_candidates,
        )
    except (OSError, json.JSONDecodeError, GeometrySnapshotError, UIMapperError) as exc:
        payload = {"schema_version":"ui-bug-hunter-v1","error":str(exc),"error_code":"invalid_ui_bug_hunt_request"}
        print(json.dumps(payload, indent=2) if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        summary=result.get("summary",{})
        print("# [PLW UI Bug Hunter V1]")
        print(f"# Scenario: {result.get('scenario_id')} | revision={result.get('candidate_revision')} | baseline={result.get('baseline_revision')}")
        print(f"# Candidates: {summary.get('candidate_count',0)} | highest={summary.get('highest_severity')}")
        print(f"# Lifecycle: {summary.get('by_lifecycle',{})}")
        for row in result.get("candidates",[])[:10]:
            print(f"# {row.get('severity')} {row.get('lifecycle_state')} {row.get('category')} ui={row.get('ui_ids')}")
        print("# Boundary: bug candidates only; PROVEN is unavailable in V1 and no source mutation/truth commit occurs.")
    return 0


def cmd_ui_diagnose(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui diagnose")
    parser.add_argument("candidate")
    parser.add_argument("--root", default=".", help="candidate revision project root")
    parser.add_argument("--max-structural", type=int, default=24)
    parser.add_argument("--baseline")
    parser.add_argument("--runtime-reference", help="optional runtime-ui-reference JSON for exact capture provenance")
    parser.add_argument("--min-gap", type=float, default=0.0)
    parser.add_argument("--no-resources", action="store_true")
    parser.add_argument("--baseline-root", help="optional source tree matching baseline revision")
    parser.add_argument("--max-candidates", type=int, default=100)
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("--cache-mode")
    opts = parser.parse_args(args)
    if not _apply_ui_reference_cache_mode(opts.cache_mode, as_json=opts.as_json):
        return 2
    try:
        candidate = load_geometry_snapshot(opts.candidate)
        candidate_graph = hott_kernel._build_kernel_graph(opts.root)
        candidate_ownership = analyze_angular_ownership(candidate_graph, read_resources=not opts.no_resources)
        candidate_ref = build_ui_reference_state(candidate, candidate_ownership)
        baseline = baseline_ownership = baseline_ref = None
        if opts.baseline:
            baseline = load_geometry_snapshot(opts.baseline)
            if opts.baseline_root:
                baseline_graph = hott_kernel._build_kernel_graph(opts.baseline_root)
                baseline_ownership = analyze_angular_ownership(baseline_graph, read_resources=not opts.no_resources)
                baseline_ref = build_ui_reference_state(baseline, baseline_ownership)
        runtime_reference = None
        if opts.runtime_reference:
            runtime_reference = json.loads(Path(opts.runtime_reference).read_text(encoding="utf-8"))
        result = build_cross_domain_ui_diagnosis(
            candidate, ui_reference=candidate_ref, angular_ownership=candidate_ownership, shared_graph=candidate_graph,
            baseline_snapshot=baseline, baseline_ui_reference=baseline_ref, baseline_angular_ownership=baseline_ownership,
            runtime_reference=runtime_reference, min_gap_px=opts.min_gap, max_bug_candidates=opts.max_candidates,
            max_structural_candidates=opts.max_structural, root_hint=opts.root,
        )
    except (OSError, json.JSONDecodeError, GeometrySnapshotError, UIMapperError, ValueError, GeometricImpactError) as exc:
        payload = {"schema_version":"ui-cross-domain-diagnosis-v1","error":str(exc),"error_code":"invalid_ui_cross_domain_diagnosis_request"}
        print(json.dumps(payload, indent=2) if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        summary = result.get("summary", {})
        print("# [PLW Cross-Domain UI Diagnosis]")
        print(f"# Scenario: {result.get('scenario_id')} | revision={result.get('candidate_revision')} | baseline={result.get('baseline_revision')}")
        print(f"# Bug candidates: {summary.get('bug_candidate_count',0)} | component files={summary.get('affected_component_files',[])}")
        for row in result.get("diagnoses", [])[:8]:
            structural = row.get("structural_context", {})
            causes = structural.get("structural_cause_candidates", [])
            top = causes[0] if causes else {}
            print(f"# {row.get('severity')} {row.get('lifecycle_state')} {row.get('category')} ui={row.get('ui_ids')} top_structural={top.get('path')}")
        print("# Boundary: structural cause candidates are hypotheses, not causal proof; planner does not execute a browser or mutate source.")
    return 0



def cmd_ui_reproduce(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui reproduce")
    parser.add_argument("diagnosis")
    parser.add_argument("--root", required=True, help="candidate revision project root")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--url")
    source.add_argument("--html-file")
    parser.add_argument("--candidate-id")
    parser.add_argument("--setup-js")
    parser.add_argument("--browser")
    parser.add_argument("--timeout", type=int, default=20)
    parser.add_argument("--min-gap", type=float, default=0.0)
    parser.add_argument("--max-nodes", type=int, default=5000)
    parser.add_argument("--no-resources", action="store_true")
    parser.add_argument("--out")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    try:
        diagnosis = json.loads(Path(opts.diagnosis).read_text(encoding="utf-8"))
        graph = hott_kernel._build_kernel_graph(opts.root)
        ownership = analyze_angular_ownership(graph, read_resources=not opts.no_resources)
        result = execute_targeted_ui_reproduction(
            diagnosis,
            angular_ownership=ownership,
            candidate_id=opts.candidate_id,
            url=opts.url,
            html_file=opts.html_file,
            setup_js_file=opts.setup_js,
            browser=opts.browser,
            timeout_seconds=opts.timeout,
            min_gap_px=opts.min_gap,
            max_nodes=opts.max_nodes,
        )
        if opts.out and isinstance(result.get("observed_geometry_snapshot"), dict):
            Path(opts.out).write_text(json.dumps(result["observed_geometry_snapshot"], indent=2, ensure_ascii=False)+"\n", encoding="utf-8")
            result["observed_snapshot_written_to"] = opts.out
    except (OSError, json.JSONDecodeError, ValueError, UIReproductionError) as exc:
        payload={"schema_version":"ui-targeted-runtime-reproduction-v1","error":str(exc),"error_code":"invalid_ui_reproduction_request"}
        print(json.dumps(payload, indent=2) if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Targeted Runtime Reproduction V1]")
        print(f"# State: {result.get('reproduction_state')} | reason={result.get('reason')}")
        print(f"# Browser: {result.get('execution',{}).get('browser_executable')} | mode={result.get('execution',{}).get('execution_mode')} | elapsed_ms={result.get('execution',{}).get('elapsed_ms')}")
        print(f"# Runtime-bound owners: {[row.get('path') for row in result.get('runtime_bound_owners',[])]}")
        print("# Boundary: runtime symptom reproduction is not root-cause proof, postcondition proof, source mutation, promotion, or truth commit.")
    return 0

def cmd_ui_probe_plan(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui probe-plan")
    parser.add_argument("diagnosis")
    parser.add_argument("--max-candidates", type=int, default=12)
    parser.add_argument("--candidate-id")
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("--out")
    parser.set_defaults(_ui_operation="probe-plan")
    opts = parser.parse_args(args)
    try:
        diagnosis = json.loads(Path(opts.diagnosis).read_text(encoding="utf-8"))
        result = build_ui_causal_probe_plan(diagnosis, candidate_id=opts.candidate_id, max_candidates=opts.max_candidates)
        if opts.out:
            Path(opts.out).write_text(json.dumps(result, indent=2, ensure_ascii=False)+"\n", encoding="utf-8")
            result["probe_plan_written_to"] = opts.out
    except (OSError, json.JSONDecodeError, ValueError, UICausalProbeError) as exc:
        payload={"schema_version":"ui-causal-probe-plan-v1","error":str(exc),"error_code":"invalid_ui_causal_probe_plan_request"}
        print(json.dumps(payload, indent=2) if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW UI Causal Probe Plan V1]")
        print(f"# Candidate: {result.get('candidate_id')} | probes={len(result.get('probes',[]))}")
        print("# Fill each intervention.javascript with one narrow browser-side counterfactual before executing `plw ui isolate`.")
        print("# Boundary: generated probes are hypotheses, not execution authority or root-cause proof.")
    return 0


def cmd_ui_isolate(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui isolate")
    parser.add_argument("diagnosis")
    parser.add_argument("probe_plan")
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("--candidate-id")
    parser.add_argument("--root", required=True, help="candidate revision project root")
    parser.add_argument("--max-nodes", type=int, default=5000)
    parser.set_defaults(_ui_operation="causal-isolate")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--html-file")
    parser.add_argument("--min-gap", type=float, default=0.0)
    source.add_argument("--url")
    parser.add_argument("--no-resources", action="store_true")
    parser.add_argument("--timeout", type=int, default=20)
    parser.add_argument("--setup-js")
    parser.add_argument("--browser")
    opts = parser.parse_args(args)
    try:
        diagnosis = json.loads(Path(opts.diagnosis).read_text(encoding="utf-8"))
        probe_plan = json.loads(Path(opts.probe_plan).read_text(encoding="utf-8"))
        graph = hott_kernel._build_kernel_graph(opts.root)
        ownership = analyze_angular_ownership(graph, read_resources=not opts.no_resources)
        result = execute_ui_causal_isolation(
            diagnosis, probe_plan,
            angular_ownership=ownership,
            candidate_id=opts.candidate_id,
            url=opts.url,
            html_file=opts.html_file,
            base_setup_js_file=opts.setup_js,
            browser=opts.browser,
            timeout_seconds=opts.timeout,
            min_gap_px=opts.min_gap,
            max_nodes=opts.max_nodes,
        )
    except (OSError, json.JSONDecodeError, ValueError, UIReproductionError, UICausalProbeError) as exc:
        payload={"schema_version":"ui-causal-isolation-v1","error":str(exc),"error_code":"invalid_ui_causal_isolation_request"}
        print(json.dumps(payload, indent=2) if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW UI Causal Isolation V1]")
        print(f"# State: {result.get('isolation_state')} | support={result.get('summary',{}).get('counterfactual_support_count',0)}")
        for row in result.get("ranked_candidates", [])[:8]:
            print(f"# {row.get('causal_evidence_state')} {row.get('role')} {row.get('structural_path')}")
        print("# Boundary: counterfactual symptom sensitivity is causal support, not unique root-cause proof or truth.")
    return 0


def cmd_ui_fix_plan(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui fix-plan")
    parser.add_argument("diagnosis")
    parser.add_argument("isolation")
    parser.add_argument("--candidate-id")
    parser.add_argument("--max-proposals", type=int, default=3)
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("--out")
    opts = parser.parse_args(args)
    try:
        diagnosis = json.loads(Path(opts.diagnosis).read_text(encoding="utf-8"))
        isolation = json.loads(Path(opts.isolation).read_text(encoding="utf-8"))
        result = build_bounded_ui_fix_plan(diagnosis, isolation, candidate_id=opts.candidate_id, max_proposals=opts.max_proposals)
        if opts.out:
            Path(opts.out).write_text(json.dumps(result, indent=2, ensure_ascii=False)+"\n", encoding="utf-8")
            result["fix_plan_written_to"] = opts.out
    except (OSError, json.JSONDecodeError, ValueError, UIFixError) as exc:
        payload={"schema_version":"ui-bounded-fix-plan-v1","error":str(exc),"error_code":"invalid_ui_fix_plan_request"}
        print(json.dumps(payload, indent=2) if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Bounded UI Fix Plan V1]")
        print(f"# Candidate: {result.get('candidate_id')} | proposals={result.get('summary',{}).get('proposal_count',0)}")
        for row in result.get("proposals", [])[:8]:
            print(f"# {row.get('proposal_id')} {row.get('role')} {row.get('structural_path')}")
        print("# Boundary: plan is read-only; PLW does not mutate source. Apply the smallest justified source change externally, then run `plw ui verify-fix`.")
    return 0


def cmd_ui_verify_fix(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui verify-fix")
    parser.add_argument("diagnosis")
    parser.add_argument("fix_plan")
    parser.add_argument("--root", required=True, help="post-fix candidate project root")
    parser.add_argument("--candidate-id")
    parser.add_argument("--proposal-id")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--html-file")
    source.add_argument("--url")
    parser.add_argument("--browser")
    parser.add_argument("--min-gap", type=float, default=0.0)
    parser.add_argument("--setup-js")
    parser.add_argument("--timeout", type=int, default=20)
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("--no-resources", action="store_true")
    parser.add_argument("--max-nodes", type=int, default=5000)
    opts = parser.parse_args(args)
    try:
        diagnosis = json.loads(Path(opts.diagnosis).read_text(encoding="utf-8"))
        fix_plan = json.loads(Path(opts.fix_plan).read_text(encoding="utf-8"))
        graph = hott_kernel._build_kernel_graph(opts.root)
        ownership = analyze_angular_ownership(graph, read_resources=not opts.no_resources)
        result = verify_bounded_ui_fix(
            diagnosis, fix_plan, angular_ownership=ownership,
            candidate_id=opts.candidate_id, proposal_id=opts.proposal_id,
            url=opts.url, html_file=opts.html_file, setup_js_file=opts.setup_js,
            browser=opts.browser, timeout_seconds=opts.timeout,
            min_gap_px=opts.min_gap, max_nodes=opts.max_nodes,
        )
    except (OSError, json.JSONDecodeError, ValueError, UIReproductionError, UIFixError) as exc:
        payload={"schema_version":"ui-post-fix-verification-v1","error":str(exc),"error_code":"invalid_ui_fix_verification_request"}
        print(json.dumps(payload, indent=2) if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Post-Fix Exact-Scenario Verification V1]")
        print(f"# State: {result.get('verification_state')} | reason={result.get('reason')}")
        print(f"# Target: {result.get('structural_path')} | proposal={result.get('proposal_id')}")
        print("# Boundary: exact-scenario fix verification is not full regression proof, postcondition proof, stable promotion, or truth commit.")
    return 0


def cmd_ui_capture_script(args: List[str]) -> int:
    import argparse
    parser=argparse.ArgumentParser(prog="plw ui capture-script")
    parser.add_argument("root", nargs="?", default=".")
    parser.add_argument("--revision", required=True)
    parser.add_argument("--state", default="default")
    parser.add_argument("--fixture", default="default")
    parser.add_argument("--max-nodes", type=int, default=5000)
    parser.add_argument("--no-resources", action="store_true")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts=parser.parse_args(args)
    try:
        graph=hott_kernel._build_kernel_graph(opts.root)
        ownership=analyze_angular_ownership(graph, read_resources=not opts.no_resources)
        script=build_browser_capture_script(
            code_revision=opts.revision,
            graph_content_signature=ownership.get("graph_content_signature"),
            angular_ownership_digest=ownership.get("ownership_digest"),
            ui_state=opts.state, data_fixture=opts.fixture, max_nodes=opts.max_nodes,
        )
    except UIMapperError as exc:
        print(f"[PLW Error] {exc}",file=sys.stderr); return 2
    if opts.as_json:
        print(json.dumps({
            "schema_version":"ui-capture-script-v1",
            "capture_contract":"geometry-snapshot-v1",
            "scan_root":opts.root,
            "graph_content_signature":ownership.get("graph_content_signature"),
            "angular_ownership_digest":ownership.get("ownership_digest"),
            "javascript":script,
            "execution":"evaluate javascript in the target page/browser context and save the returned object as JSON",
            "authority":{"browser_executed_by_command":False,"source_mutated":False,"truth_committed":False},
        },indent=2,ensure_ascii=False))
    else:
        print(script)
    return 0


def cmd_ui_static(args: List[str]) -> int:
    """Locate Angular UI source witnesses without a browser or geometry snapshot."""
    import argparse
    parser = argparse.ArgumentParser(prog="plw ui static")
    parser.add_argument("query")
    parser.add_argument("root", nargs="?", default=".")
    parser.add_argument("--selector")
    parser.add_argument("--max-candidates", type=int, default=8)
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    try:
        graph = hott_kernel._build_kernel_graph(opts.root)
        result = static_ui_reference(opts.query, graph,
                                     selector=opts.selector, max_candidates=opts.max_candidates)
    except ValueError as exc:
        print(json.dumps({"error": str(exc), "error_code": "invalid_static_ui_query"})
              if opts.as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        decision = result["decision"]
        print("# [PLW UI Static Reference]")
        print("# Status: " + decision["status"])
        print("# Candidates: " + str(decision["matching_candidate_count"]))
        for item in result["candidates"]:
            print(f"- {item['component_file']} -> {item['template_file']}:{item['template_line']} "
                  f"{item['element']['tag']} {item['element']['label']!r}")
        print("# Runtime geometry: UNOBSERVED; browser execution: false")
    return 0


def cmd_ui(args: List[str]) -> int:
    if not args:
        print("Usage: plw ui <static|map|inspect|scenarios|diff|hunt|diagnose|reproduce|probe-plan|isolate|fix-plan|verify-fix|capture-script> ...", file=sys.stderr); return 2
    sub,args=args[0],args[1:]
    if sub == "static": return cmd_ui_static(args)
    if sub == "map": return cmd_ui_map(args)
    if sub == "inspect": return cmd_ui_inspect(args)
    if sub in {"scenarios", "scenario-map", "scenario_map", "responsive"}: return cmd_ui_scenarios(args)
    if sub in {"diff", "geometry-diff", "geometry_diff", "compare"}: return cmd_ui_diff(args)
    if sub in {"hunt", "bugs", "bug-hunt", "bug_hunt", "bug-hunter", "bug_hunter"}: return cmd_ui_hunt(args)
    if sub in {"diagnose", "diagnosis", "cross-domain", "cross_domain", "trace-bug", "trace_bug"}: return cmd_ui_diagnose(args)
    if sub in {"reproduce", "reproduction", "runtime-reproduce", "runtime_reproduce"}: return cmd_ui_reproduce(args)
    if sub in {"probe-plan", "probe_plan", "causal-plan", "causal_plan"}: return cmd_ui_probe_plan(args)
    if sub in {"isolate", "causal-isolate", "causal_isolate", "root-cause-probe", "root_cause_probe"}: return cmd_ui_isolate(args)
    if sub in {"fix-plan", "fix_plan", "bounded-fix", "bounded_fix"}: return cmd_ui_fix_plan(args)
    if sub in {"verify-fix", "verify_fix", "post-fix", "post_fix"}: return cmd_ui_verify_fix(args)
    if sub in {"capture-script","capture_script"}: return cmd_ui_capture_script(args)
    print(f"[PLW Error] unknown ui subcommand: {sub}", file=sys.stderr); return 2


def cmd_ui_reference(args: List[str]) -> int:
    """Validate geometry↔code reference state or expose practical UI mapping subcommands."""
    if args and args[0] in {"map", "inspect", "scenarios", "scenario-map", "scenario_map", "responsive", "diff", "geometry-diff", "geometry_diff", "compare", "hunt", "bugs", "bug-hunt", "bug_hunt", "bug-hunter", "bug_hunter", "diagnose", "diagnosis", "cross-domain", "cross_domain", "trace-bug", "trace_bug", "reproduce", "reproduction", "runtime-reproduce", "runtime_reproduce", "probe-plan", "probe_plan", "causal-plan", "causal_plan", "isolate", "causal-isolate", "causal_isolate", "root-cause-probe", "root_cause_probe", "fix-plan", "fix_plan", "bounded-fix", "bounded_fix", "verify-fix", "verify_fix", "post-fix", "post_fix", "capture-script", "capture_script"}:
        return cmd_ui(args)
    parsed = _parse_ui_reference_args(args, command="ui-reference", allow_store_dir=False)
    if parsed is None:
        return 2
    resolved = _resolve_geometry_query_positionals(parsed, "ui-reference")
    if resolved is None:
        return 2
    snapshot_path, root, as_json = resolved
    if not _apply_ui_reference_cache_mode(parsed["cache_mode"], as_json=as_json):
        return 2
    try:
        payload, graph, ownership = _load_ui_reference_inputs(
            snapshot_path, root, read_resources=bool(parsed["read_resources"]),
        )
        result = build_ui_reference_state(payload, ownership)
    except GeometrySnapshotError as exc:
        error = {
            "schema_version": "ui-reference-v1",
            "error": str(exc),
            "error_code": "invalid_geometry_snapshot",
            "source_mutated": False,
            "runtime_executed": False,
        }
        print(json.dumps(error, indent=2, ensure_ascii=False) if as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        geometry = result.get("geometry", {}).get("summary", {})
        corr = result.get("correspondence", {}).get("summary", {})
        static = result.get("correspondence", {}).get("static_context", {})
        print("# [PLW UI Reference State]")
        print(f"# Scenario: {result.get('scenario_id')}")
        print(f"# Geometry nodes: {geometry.get('node_count', 0)} | inside viewport: {geometry.get('fully_inside_viewport_count', 0)}")
        print(f"# Correspondence: exact={corr.get('exact_count', 0)} structural={corr.get('structural_count', 0)} unresolved={corr.get('unresolved_count', 0)}")
        print(f"# Static snapshot bound: {static.get('static_snapshot_bound', False)}")
        print("# Boundary: validated geometry payload + correspondence reference only; no browser execution, behavior proof, mutation authority, or truth commit.")
    return 0


def cmd_runtime_ui_reference(args: List[str]) -> int:
    """Bind one F125 UI reference state to verified F123 capture provenance."""
    parsed = _parse_ui_reference_args(args, command="runtime-ui-reference", allow_store_dir=True)
    if parsed is None:
        return 2
    positional = parsed["positional"]
    if len(positional) not in (2, 3):
        print("[PLW Error] runtime-ui-reference requires <lease-id> <snapshot.json> and accepts at most one root path", file=sys.stderr)
        return 2
    lease_id, snapshot_path = positional[0], positional[1]
    root = positional[2] if len(positional) == 3 else "."
    as_json = bool(parsed["as_json"])
    if not _apply_ui_reference_cache_mode(parsed["cache_mode"], as_json=as_json):
        return 2
    try:
        payload, graph, ownership = _load_ui_reference_inputs(
            snapshot_path, root, read_resources=bool(parsed["read_resources"]),
        )
        result = build_runtime_ui_reference_state(
            payload, ownership, lease_id, store_dir=parsed["store_dir"],
        )
    except GeometrySnapshotError as exc:
        error = {
            "schema_version": "runtime-ui-reference-v1",
            "error": str(exc),
            "error_code": "invalid_geometry_snapshot",
            "source_mutated": False,
            "runtime_executed_by_this_query": False,
        }
        print(json.dumps(error, indent=2, ensure_ascii=False) if as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        capture = result.get("runtime_capture", {})
        ui_ref = result.get("ui_reference", {})
        corr = ui_ref.get("correspondence", {}).get("summary", {})
        print("# [PLW Runtime UI Reference State]")
        print(f"# Scenario: {result.get('scenario_id')}")
        print(f"# Runtime capture: {capture.get('proof_class')} | reason={capture.get('reason')}")
        print(f"# Correspondence: exact={corr.get('exact_count', 0)} structural={corr.get('structural_count', 0)} unresolved={corr.get('unresolved_count', 0)}")
        print("# Boundary: verified adapter observation provenance only; this query performs no runtime execution and proves no geometry correctness, behavior, mutation authority, or truth.")
    return 0


def _parse_geometric_impact_args(args: List[str]) -> Optional[Dict[str, Any]]:
    parsed: Dict[str, Any] = {
        "cache_mode": None, "store_dir": None, "lease_id": None,
        "constraints_path": None, "proximity_px": None, "focus_ui_ids": [],
        "as_json": False, "read_resources": True, "positional": [],
    }
    i = 0
    value_flags = {"--cache-mode", "--store-dir", "--lease-id", "--constraints", "--proximity", "--focus"}
    while i < len(args):
        arg = args[i]
        if arg == "--json":
            parsed["as_json"] = True
            i += 1
            continue
        if arg == "--no-resources":
            parsed["read_resources"] = False
            i += 1
            continue
        if arg in value_flags:
            if i + 1 >= len(args):
                print(f"[PLW Error] {arg} requires a value", file=sys.stderr)
                return None
            value = args[i + 1]
            if arg == "--focus":
                parsed["focus_ui_ids"].append(value)
            elif arg == "--proximity":
                try:
                    parsed["proximity_px"] = float(value)
                except ValueError:
                    print("[PLW Error] --proximity requires a finite non-negative number", file=sys.stderr)
                    return None
            elif arg == "--constraints":
                parsed["constraints_path"] = value
            elif arg == "--lease-id":
                parsed["lease_id"] = value
            elif arg == "--store-dir":
                parsed["store_dir"] = value
            else:
                parsed["cache_mode"] = value
            i += 2
            continue
        if arg.startswith("-"):
            print(f"[PLW Error] Unknown geometric-impact argument: {arg}", file=sys.stderr)
            return None
        parsed["positional"].append(arg)
        i += 1
    return parsed


def cmd_geometric_impact(args: List[str]) -> int:
    """Evaluate explicit geometry constraints and project selected surfaces to topology."""
    parsed = _parse_geometric_impact_args(args)
    if parsed is None:
        return 2
    resolved = _resolve_geometry_query_positionals(parsed, "geometric-impact")
    if resolved is None:
        return 2
    snapshot_path, root, as_json = resolved
    if parsed["store_dir"] is not None and parsed["lease_id"] is None:
        print("[PLW Error] --store-dir requires --lease-id for runtime provenance binding", file=sys.stderr)
        return 2
    if not _apply_ui_reference_cache_mode(parsed["cache_mode"], as_json=as_json):
        return 2
    try:
        payload, graph, ownership = _load_ui_reference_inputs(
            snapshot_path, root, read_resources=bool(parsed["read_resources"]),
        )
        runtime_capture = None
        if parsed["lease_id"] is not None:
            runtime_ref = build_runtime_ui_reference_state(
                payload, ownership, str(parsed["lease_id"]), store_dir=parsed["store_dir"],
            )
            ui_reference = runtime_ref["ui_reference"]
            runtime_capture = runtime_ref["runtime_capture"]
        else:
            ui_reference = build_ui_reference_state(payload, ownership)
        query = load_geometric_impact_query(parsed["constraints_path"]) if parsed["constraints_path"] else None
        result = build_targeted_geometric_impact(
            ui_reference, ownership, graph, query,
            focus_ui_ids=parsed["focus_ui_ids"],
            proximity_px=parsed["proximity_px"],
            runtime_capture=runtime_capture,
        )
    except (GeometrySnapshotError, GeometricImpactError) as exc:
        error = {
            "schema_version": "geometric-impact-v1",
            "error": str(exc),
            "error_code": "invalid_geometric_impact_query",
            "source_mutated": False,
            "runtime_executed_by_query": False,
        }
        print(json.dumps(error, indent=2, ensure_ascii=False) if as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        evidence = result.get("geometric_evidence", {}).get("summary", {})
        projection = result.get("impact_projection", {})
        selection = result.get("selection", {})
        capture = result.get("runtime_capture", {})
        print("# [PLW Targeted Geometric Impact]")
        print(f"# Scenario: {result.get('scenario_id')}")
        print(f"# Selected UI: {len(selection.get('selected_ui_ids', []))} | omitted geometry nodes: {selection.get('omitted_geometry_nodes', 0)}")
        print(f"# Constraints: satisfied={evidence.get('satisfied_count', 0)} violated={evidence.get('violated_count', 0)} unresolved={evidence.get('unresolved_count', 0)}")
        print(f"# Projected component files: {projection.get('projected_component_file_count', 0)} | unresolved UI owners: {projection.get('unresolved_ui_count', 0)}")
        if capture.get("provided"):
            print(f"# Runtime capture provenance: {capture.get('proof_class')}")
        print("# Boundary: geometric facts and declared constraints are reference evidence only; no browser execution, behavior proof, mutation authority, or truth commit.")
    return 0



def _parse_validation_plan_args(args: List[str]) -> Optional[Dict[str, Any]]:
    catalog_path: Optional[str] = None
    max_scenarios = 8
    forwarded: List[str] = []
    i = 0
    while i < len(args):
        arg = args[i]
        if arg in {"--scenario-catalog", "--max-scenarios"}:
            if i + 1 >= len(args):
                print(f"[PLW Error] {arg} requires a value", file=sys.stderr)
                return None
            value = args[i + 1]
            if arg == "--scenario-catalog":
                catalog_path = value
            else:
                try:
                    max_scenarios = int(value)
                except ValueError:
                    print("[PLW Error] --max-scenarios requires an integer", file=sys.stderr)
                    return None
            i += 2
            continue
        forwarded.append(arg)
        i += 1
    parsed = _parse_geometric_impact_args(forwarded)
    if parsed is None:
        return None
    parsed["catalog_path"] = catalog_path
    parsed["max_scenarios"] = max_scenarios
    return parsed


def cmd_validation_plan(args: List[str]) -> int:
    """Compose F124-F128 into a bounded, non-executing validation scenario plan."""
    parsed = _parse_validation_plan_args(args)
    if parsed is None:
        return 2
    resolved = _resolve_geometry_query_positionals(parsed, "validation-plan")
    if resolved is None:
        return 2
    snapshot_path, root, as_json = resolved
    if parsed["store_dir"] is not None and parsed["lease_id"] is None:
        print("[PLW Error] --store-dir requires --lease-id for runtime provenance binding", file=sys.stderr)
        return 2
    if not _apply_ui_reference_cache_mode(parsed["cache_mode"], as_json=as_json):
        return 2
    try:
        payload, graph, ownership = _load_ui_reference_inputs(
            snapshot_path, root, read_resources=bool(parsed["read_resources"]),
        )
        runtime_capture = None
        if parsed["lease_id"] is not None:
            runtime_ref = build_runtime_ui_reference_state(
                payload, ownership, str(parsed["lease_id"]), store_dir=parsed["store_dir"],
            )
            ui_reference = runtime_ref["ui_reference"]
            runtime_capture = runtime_ref["runtime_capture"]
        else:
            ui_reference = build_ui_reference_state(payload, ownership)
        query = load_geometric_impact_query(parsed["constraints_path"]) if parsed["constraints_path"] else None
        impact = build_targeted_geometric_impact(
            ui_reference, ownership, graph, query,
            focus_ui_ids=parsed["focus_ui_ids"],
            proximity_px=parsed["proximity_px"],
            runtime_capture=runtime_capture,
        )
        catalog = load_validation_scenario_catalog(parsed["catalog_path"]) if parsed["catalog_path"] else None
        result = build_targeted_validation_plan(
            ui_reference, impact, catalog, max_scenarios=parsed["max_scenarios"],
        )
    except (GeometrySnapshotError, GeometricImpactError, ValidationScenarioError) as exc:
        error = {
            "schema_version": "validation-scenario-plan-v1",
            "error": str(exc),
            "error_code": "invalid_validation_plan_input",
            "source_mutated": False,
            "validation_executed": False,
        }
        print(json.dumps(error, indent=2, ensure_ascii=False) if as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        plan = result.get("planning", {})
        targets = result.get("targets", {})
        print("# [PLW Targeted Validation Scenario Plan]")
        print(f"# Current scenario: {result.get('scenario_id')}")
        print(f"# Selected scenarios: {plan.get('selected_count', 0)} / eligible {plan.get('eligible_count', 0)} | omitted: {plan.get('omitted_count', 0)}")
        print(f"# Targets: ui={len(targets.get('ui_ids', []))} components={len(targets.get('component_files', []))}")
        print(f"# Uncovered variation axes: {', '.join(plan.get('uncovered_variation_axes', [])) or 'none'}")
        print("# Boundary: this is a recoverable validation plan only; no render/test execution, mutation authority, postcondition proof, or truth commit.")
    return 0


def _parse_validation_evidence_args(args: List[str]) -> Optional[Dict[str, Any]]:
    store_dir: Optional[str] = None
    as_json = False
    positionals: List[str] = []
    i = 0
    while i < len(args):
        arg = args[i]
        if arg == "--json":
            as_json = True
            i += 1
            continue
        if arg == "--store-dir":
            if i + 1 >= len(args):
                print("[PLW Error] --store-dir requires a value", file=sys.stderr)
                return None
            store_dir = args[i + 1]
            i += 2
            continue
        if arg.startswith("--"):
            print(f"[PLW Error] unknown validation-evidence option: {arg}", file=sys.stderr)
            return None
        positionals.append(arg)
        i += 1
    if len(positionals) != 2:
        print("[PLW Error] validation-evidence requires <plan.json> <evidence.json>", file=sys.stderr)
        return None
    return {
        "plan_path": positionals[0],
        "evidence_path": positionals[1],
        "store_dir": store_dir,
        "as_json": as_json,
    }


def cmd_validation_evidence(args: List[str]) -> int:
    """Bind bounded external validator evidence to exact F128 scenario identities."""
    parsed = _parse_validation_evidence_args(args)
    if parsed is None:
        return 2
    try:
        plan = load_validation_plan(parsed["plan_path"])
        evidence = load_validation_evidence_bundle(parsed["evidence_path"])
        result = bind_validation_evidence(plan, evidence, store_dir=parsed["store_dir"])
    except ValidationEvidenceError as exc:
        error = {
            "schema_version": "scenario-validation-evidence-binding-v1",
            "error": str(exc),
            "error_code": "invalid_validation_evidence_input",
            "validator_executed_by_binder": False,
            "source_mutated": False,
        }
        print(json.dumps(error, indent=2, ensure_ascii=False) if parsed["as_json"] else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if parsed["as_json"]:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        summary = result.get("summary", {})
        print("# [PLW Scenario-Bound Validation Evidence]")
        print(f"# Plan: {result.get('validation_plan_digest')}")
        print(
            "# Scenarios: "
            f"observed={summary.get('observed_scenario_count', 0)} "
            f"missing={summary.get('missing_scenario_count', 0)} "
            f"conflicting={summary.get('conflicting_scenario_count', 0)}"
        )
        print(f"# Unplanned observations: {summary.get('unplanned_observation_count', 0)}")
        print(f"# F123 verified receipts: {summary.get('verified_F123_receipt_count', 0)}")
        print(f"# F123 content-bound provenance: {summary.get('verified_F123_provenance_count', 0)}")
        print("# Boundary: validator outcomes are scenario-bound evidence only; missing evidence is not failure and no postcondition/truth is committed.")
    return 0


def _parse_validation_coverage_args(args: List[str]) -> Optional[Dict[str, Any]]:
    store_dir: Optional[str]
    positionals: List[str]
    store_dir, as_json, positionals, i = None, False, [], 0
    while i < len(args):
        arg = args[i]
        if arg == "--json":
            as_json = True
            i += 1
            continue
        if arg == "--store-dir":
            if i + 1 >= len(args):
                print("[PLW Error] --store-dir requires a value", file=sys.stderr)
                return None
            store_dir = args[i + 1]
            i += 2
            continue
        if arg.startswith("--"):
            print(f"[PLW Error] unknown validation-coverage option: {arg}", file=sys.stderr)
            return None
        positionals.append(arg)
        i += 1

    if positionals and positionals[0] in {"execution-backed", "execution_backed"}:
        if len(positionals) != 3:
            print("[PLW Error] validation-coverage execution-backed requires <f139-binding.json> <obligations.json>", file=sys.stderr)
            return None
        if store_dir is not None:
            print("[PLW Error] --store-dir is not used for execution-backed coverage; F139 binding is already self-contained", file=sys.stderr)
            return None
        return {
            "mode": "execution-backed",
            "binding_path": positionals[1],
            "obligations_path": positionals[2],
            "store_dir": None,
            "as_json": as_json,
        }

    if len(positionals) != 3:
        print("[PLW Error] validation-coverage requires <plan.json> <evidence.json> <obligations.json>", file=sys.stderr)
        return None
    return {
        "mode": "legacy-f129",
        "plan_path": positionals[0],
        "evidence_path": positionals[1],
        "obligations_path": positionals[2],
        "store_dir": store_dir,
        "as_json": as_json,
    }


def cmd_validation_coverage(args: List[str]) -> int:
    """Re-derive proof-obligation coverage from F129 or F139 evidence."""
    parsed = _parse_validation_coverage_args(args)
    if parsed is None:
        return 2
    try:
        obligations = load_proof_obligation_set(parsed["obligations_path"])
        if parsed["mode"] == "execution-backed":
            binding = load_execution_backed_scenario_evidence(parsed["binding_path"])
            result = project_execution_backed_proof_obligations(binding, obligations)
        else:
            plan = load_validation_plan(parsed["plan_path"])
            evidence = load_validation_evidence_bundle(parsed["evidence_path"])
            binding = bind_validation_evidence(plan, evidence, store_dir=parsed["store_dir"])
            result = project_validation_proof_obligations(binding, obligations)
    except (ValidationEvidenceError, ExecutionBackedEvidenceError, ProofObligationError) as exc:
        error = {
            "schema_version": (
                EXECUTION_PROOF_COVERAGE_SCHEMA_VERSION
                if parsed.get("mode") == "execution-backed"
                else VALIDATION_PROOF_COVERAGE_SCHEMA_VERSION
            ),
            "error": str(exc),
            "error_code": "invalid_validation_coverage_input",
            "validator_executed_by_projector": False,
            "runtime_dispatched_by_projector": False,
            "source_mutated": False,
            "postcondition_proven": False,
            "truth_committed": False,
        }
        print(json.dumps(error, indent=2, ensure_ascii=False) if parsed["as_json"] else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if parsed["as_json"]:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        summary = result.get("summary", {})
        label = "Execution-Backed Proof Coverage" if parsed["mode"] == "execution-backed" else "Validation Evidence Coverage"
        print(f"# [PLW {label}]")
        print(f"# Plan: {result.get('validation_plan_digest')}")
        print(
            "# Obligations: "
            f"covered={summary.get('covered_count', 0)} "
            f"partial={summary.get('partial_count', 0)} "
            f"missing={summary.get('missing_count', 0)} "
            f"conflicting={summary.get('conflicting_count', 0)}"
        )
        print(f"# Next evidence requests: {len(result.get('next_evidence_requests', []))}")
        if parsed["mode"] == "execution-backed":
            print("# Boundary: F139 execution-backed evidence has been freshly re-derived over obligations; even complete coverage is not postcondition proof or truth.")
        else:
            print("# Boundary: evidence coverage is not postcondition proof or truth; conflicts are preserved and missing coverage is not validator failure.")
    return 0


def _parse_validation_request_args(args: List[str]) -> Optional[Dict[str, Any]]:
    store_dir: Optional[str] = None
    validator_id: Optional[str] = None
    executor_id: Optional[str] = None
    positionals: List[str] = []
    i = 0
    as_json = False
    while i < len(args):
        arg = args[i]
        if arg == "--json":
            as_json = True
            i += 1
            continue
        if arg in {"--store-dir", "--validator-id", "--executor-id"}:
            if i + 1 >= len(args):
                print(f"[PLW Error] {arg} requires a value", file=sys.stderr)
                return None
            value = args[i + 1]
            if arg == "--store-dir":
                store_dir = value
            elif arg == "--validator-id":
                validator_id = value
            else:
                executor_id = value
            i += 2
            continue
        if arg.startswith("--"):
            print(f"[PLW Error] unknown validation-request option: {arg}", file=sys.stderr)
            return None
        positionals.append(arg)
        i += 1
    if len(positionals) != 3:
        print("[PLW Error] validation-request requires <plan.json> <evidence.json> <obligations.json>", file=sys.stderr)
        return None
    if not validator_id or not executor_id:
        print("[PLW Error] validation-request requires --validator-id ID and --executor-id ID", file=sys.stderr)
        return None
    return {
        "plan_path": positionals[0],
        "evidence_path": positionals[1],
        "obligations_path": positionals[2],
        "validator_id": validator_id,
        "executor_id": executor_id,
        "store_dir": store_dir,
        "as_json": as_json,
    }


def cmd_validation_request(args: List[str]) -> int:
    """Build F132 validation execution requests without authorization or dispatch."""
    parsed = _parse_validation_request_args(args)
    if parsed is None:
        return 2
    try:
        plan = load_validation_plan(parsed["plan_path"])
        evidence = load_validation_evidence_bundle(parsed["evidence_path"])
        binding = bind_validation_evidence(plan, evidence, store_dir=parsed["store_dir"])
        obligations = load_proof_obligation_set(parsed["obligations_path"])
        result = build_validation_execution_requests(
            plan,
            binding,
            obligations,
            validator_id=parsed["validator_id"],
            executor_id=parsed["executor_id"],
        )
    except (ValidationEvidenceError, ProofObligationError, ValidationExecutionError) as exc:
        error = {
            "schema_version": VALIDATION_EXECUTION_REQUEST_SET_SCHEMA_VERSION,
            "error": str(exc),
            "error_code": "invalid_validation_request_input",
            "authorization_granted": False,
            "runtime_dispatched": False,
            "validator_executed": False,
            "source_mutated": False,
        }
        print(json.dumps(error, indent=2, ensure_ascii=False) if parsed["as_json"] else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if parsed["as_json"]:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        summary = result.get("summary", {})
        print("# [PLW Validation Execution Requests]")
        print(f"# Plan: {result.get('validation_plan_digest')}")
        print(f"# Validator: {result.get('validator_id')} | executor: {result.get('executor_id')}")
        print(
            "# Obligations: "
            f"covered={summary.get('covered_obligation_count', 0)} "
            f"unresolved={summary.get('unresolved_obligation_count', 0)} "
            f"requests={summary.get('execution_request_count', 0)}"
        )
        for row in result.get("execution_requests", []):
            print(
                f"{row.get('request_id')} {row.get('action_class')} "
                f"scenario={row.get('scenario_id')} obligations={len(row.get('obligations', []))}"
            )
        print("# Boundary: REQUESTED != AUTHORIZED != LEASED != DISPATCHED != EXECUTED != EVIDENCE-RETURNED != PROVEN.")
    return 0



def cmd_install(args: List[str]) -> int:
    """Install PLW wrapper commands into a user or system bin directory.

    `--system` maps to PLW_SYSTEM_BIN_DIR or /usr/local/bin. This command never
    invokes sudo or a shell command string; setup_plw.sh performs the bounded
    wrapper write and fails if the selected directory is not writable.
    """
    system = False
    bin_dir: Optional[str] = None
    as_json = False
    i = 0
    while i < len(args):
        arg = args[i]
        if arg == "--system":
            if bin_dir is not None:
                message = "--system and --bin-dir are mutually exclusive"
                if as_json:
                    print(json.dumps({"error": "install_location_conflict", "message": message}, indent=2))
                else:
                    print(f"[PLW Error] {message}", file=sys.stderr)
                return 2
            system = True
            i += 1
            continue
        if arg == "--bin-dir":
            if system:
                message = "--system and --bin-dir are mutually exclusive"
                if as_json:
                    print(json.dumps({"error": "install_location_conflict", "message": message}, indent=2))
                else:
                    print(f"[PLW Error] {message}", file=sys.stderr)
                return 2
            if i + 1 >= len(args):
                print("[PLW Error] --bin-dir requires DIR", file=sys.stderr)
                return 2
            bin_dir = args[i + 1]
            i += 2
            continue
        if arg == "--json":
            as_json = True
            i += 1
            continue
        if arg in ("-h", "--help", "help"):
            print("Usage: plw install [--system | --bin-dir DIR] [--json]")
            print("  --system installs wrappers in /usr/local/bin (or PLW_SYSTEM_BIN_DIR).")
            print("  The command never invokes sudo automatically.")
            return 0
        print(f"[PLW Error] Unknown install argument: {arg}", file=sys.stderr)
        return 2

    setup_script = TOOL_ROOT / "setup_plw.sh"
    if not setup_script.is_file():
        result = {
            "status": "rejected",
            "error": "setup_script_missing",
            "setup_script": str(setup_script),
        }
        print(json.dumps(result, indent=2, ensure_ascii=False) if as_json else f"[PLW Error] setup script missing: {setup_script}")
        return 2

    environment = os.environ.copy()
    setup_args = ["sh", str(setup_script)]
    if system:
        setup_args.append("--system")
        selected = environment.get("PLW_SYSTEM_BIN_DIR") or "/usr/local/bin"
    elif bin_dir is not None:
        setup_args.extend(["--bin-dir", bin_dir])
        selected = bin_dir
    else:
        selected = environment.get("PLW_BIN_DIR") or os.path.join(environment.get("HOME") or "", ".local", "bin")

    try:
        completed = subprocess.run(
            setup_args,
            cwd=str(TOOL_ROOT),
            env=environment,
            capture_output=True,
            text=True,
            timeout=30,
            check=False,
        )
    except subprocess.TimeoutExpired:
        result = {
            "status": "rejected",
            "error": "install_timeout",
            "bin_dir": selected,
            "system_location_requested": system,
            "sudo_invoked": False,
        }
        print(json.dumps(result, indent=2, ensure_ascii=False) if as_json else "[PLW Error] install timed out")
        return 3

    selected_path = str(Path(selected).expanduser())
    result = {
        "status": "installed" if completed.returncode == 0 else "rejected",
        "bin_dir": selected_path,
        "system_location_requested": system or os.path.normpath(selected_path) == "/usr/local/bin",
        "commands": [
            str(Path(selected_path) / "plw"),
            str(Path(selected_path) / "hott"),
            str(Path(selected_path) / "plw-call"),
        ],
        "path_contains_bin_dir": selected_path in (environment.get("PATH") or "").split(os.pathsep),
        "sudo_invoked": False,
        "setup_exit_code": completed.returncode,
        "setup_stdout": completed.stdout,
        "setup_stderr": completed.stderr,
        "authority": {
            "wrapper_files_changed": completed.returncode == 0,
            "source_mutated": False,
            "repository_mutated": False,
            "workflow_authority_granted": False,
            "external_runtime_executed": False,
        },
    }
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    elif completed.returncode == 0:
        print(completed.stdout, end="")
    else:
        if completed.stderr:
            print(completed.stderr, end="", file=sys.stderr)
        else:
            print(f"[PLW Error] install failed for {selected_path}", file=sys.stderr)
    return 0 if completed.returncode == 0 else completed.returncode



def _parse_validation_authorization_args(args: List[str]) -> Optional[Dict[str, Any]]:
    positionals: List[str] = []
    store_dir: Optional[str] = None
    request_id: Optional[str] = None
    executor_id: Optional[str] = None
    validator_id: Optional[str] = None
    as_json = False
    i = 0
    while i < len(args):
        arg = args[i]
        if arg == "--json":
            as_json = True
            i += 1
            continue
        if arg in {"--store-dir", "--request-id", "--validator-id", "--executor-id"}:
            if i + 1 >= len(args):
                print(f"[PLW Error] {arg} requires a value", file=sys.stderr)
                return None
            value = args[i + 1]
            if arg == "--store-dir": store_dir = value
            elif arg == "--request-id": request_id = value
            elif arg == "--validator-id": validator_id = value
            else: executor_id = value
            i += 2
            continue
        if arg.startswith("--"):
            print(f"[PLW Error] unknown validation-authorization option: {arg}", file=sys.stderr)
            return None
        positionals.append(arg)
        i += 1
    if len(positionals) != 4:
        print("[PLW Error] validation-authorization requires <plan.json> <evidence.json> <obligations.json> <assertion.json>", file=sys.stderr)
        return None
    if not request_id or not validator_id or not executor_id:
        print("[PLW Error] validation-authorization requires --request-id, --validator-id, and --executor-id", file=sys.stderr)
        return None
    return {
        "plan_path": positionals[0], "evidence_path": positionals[1],
        "obligations_path": positionals[2], "assertion_path": positionals[3],
        "request_id": request_id, "validator_id": validator_id, "executor_id": executor_id,
        "store_dir": store_dir, "as_json": as_json,
    }


def cmd_validation_authorization(args: List[str]) -> int:
    """Verify a host-issued F133 assertion and bind it to one exact F132 request."""
    parsed = _parse_validation_authorization_args(args)
    if parsed is None:
        return 2
    try:
        plan = load_validation_plan(parsed["plan_path"])
        evidence = load_validation_evidence_bundle(parsed["evidence_path"])
        evidence_binding = bind_validation_evidence(plan, evidence, store_dir=parsed["store_dir"])
        obligations = load_proof_obligation_set(parsed["obligations_path"])
        request_set = build_validation_execution_requests(
            plan, evidence_binding, obligations,
            validator_id=parsed["validator_id"], executor_id=parsed["executor_id"],
        )
        raw_assertion = json.loads(Path(parsed["assertion_path"]).read_text(encoding="utf-8"))
        if not isinstance(raw_assertion, dict):
            raise ValidationAuthorizationError("authorization assertion must be a JSON object")
        result = build_validation_authorization_binding(
            request_set, parsed["request_id"], raw_assertion,
            plan=plan, binding=evidence_binding, obligation_set=obligations,
            validator_id=parsed["validator_id"], executor_id=parsed["executor_id"],
            store_dir=parsed["store_dir"],
        )
    except (OSError, json.JSONDecodeError, ValidationEvidenceError, ProofObligationError, ValidationExecutionError, ValidationAuthorizationError) as exc:
        error = {
            "schema_version": VALIDATION_AUTHORIZATION_BINDING_SCHEMA_VERSION,
            "error": str(exc), "error_code": "invalid_validation_authorization_input",
            "authorization_bound": False, "execution_lease_acquired": False,
            "runtime_dispatched": False, "validator_executed": False,
        }
        print(json.dumps(error, indent=2, ensure_ascii=False) if parsed["as_json"] else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if parsed["as_json"]:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Validation Authorization Binding]")
        print(f"# Request: {result.get('request_id')}")
        print(f"# Principal: {result.get('authorization_principal')}")
        print(f"# State: {result.get('authorization_state')}")
        print("# Boundary: authorization is bound, but no lease/attempt/dispatch/runtime authority is granted.")
    return 0


def cmd_validation_lease(args: List[str]) -> int:
    """Acquire or inspect one F134 single-use validation execution lease."""
    usage = (
        "Usage:\n"
        "  plw validation-lease acquire <plan.json> <evidence.json> <obligations.json> <assertion.json> "
        "--request-id ID --validator-id ID --executor-id ID --idempotency-key KEY "
        "[--lease-ttl-seconds N] [--store-dir DIR] --allow-state-change [--json]\n"
        "  plw validation-lease status <sha256:lease-id> [--request-id ID] [--executor-id ID] "
        "[--store-dir DIR] [--json]"
    )
    if not args or args[0] in ("-h", "--help", "help"):
        print(usage)
        return 0 if args else 2
    operation = args[0].lower()
    if operation not in {"acquire", "status"}:
        print(f"[PLW Error] Unknown validation-lease operation: {operation}", file=sys.stderr)
        print(usage, file=sys.stderr)
        return 2

    positionals: List[str] = []
    store_dir = request_id = validator_id = executor_id = idempotency_key = None
    lease_ttl_seconds = 15
    allow_state_change = as_json = False
    index = 1
    while index < len(args):
        argument = args[index]
        if argument in {"--store-dir", "--request-id", "--validator-id", "--executor-id", "--idempotency-key", "--lease-ttl-seconds"}:
            if index + 1 >= len(args):
                print(f"[PLW Error] {argument} requires a value", file=sys.stderr)
                return 2
            value = args[index + 1]
            if argument == "--store-dir": store_dir = value
            elif argument == "--request-id": request_id = value
            elif argument == "--validator-id": validator_id = value
            elif argument == "--executor-id": executor_id = value
            elif argument == "--idempotency-key": idempotency_key = value
            else:
                try:
                    lease_ttl_seconds = int(value)
                except ValueError:
                    print("[PLW Error] --lease-ttl-seconds must be an integer", file=sys.stderr)
                    return 2
            index += 2
            continue
        if argument == "--allow-state-change":
            allow_state_change = True
            index += 1
            continue
        if argument == "--json":
            as_json = True
            index += 1
            continue
        if argument.startswith("--"):
            print(f"[PLW Error] unknown validation-lease option: {argument}", file=sys.stderr)
            return 2
        positionals.append(argument)
        index += 1

    if operation == "status":
        if len(positionals) != 1:
            print("[PLW Error] validation-lease status requires exactly one lease id", file=sys.stderr)
            return 2
        if allow_state_change or validator_id is not None or idempotency_key is not None or lease_ttl_seconds != 15:
            print("[PLW Error] validation-lease status is read-only; remove acquire-only options", file=sys.stderr)
            return 2
        result = inspect_validation_execution_lease(
            positionals[0], request_id=request_id, executor_id=executor_id, store_dir=store_dir,
        )
        if as_json:
            print(json.dumps(result, indent=2, ensure_ascii=False))
        else:
            print("# [PLW Validation Execution Lease Status]")
            print(f"# Lease: {result.get('lease_id', positionals[0])}")
            print(f"# Decision: {result.get('decision', result.get('error'))}")
            print("# Authority: inspection only; it cannot regrant attempt registration or dispatch.")
        return 0 if result.get("lease_found") else 3

    if len(positionals) != 4:
        print("[PLW Error] validation-lease acquire requires <plan.json> <evidence.json> <obligations.json> <assertion.json>", file=sys.stderr)
        return 2
    if not request_id or not validator_id or not executor_id or not idempotency_key:
        print("[PLW Error] validation-lease acquire requires --request-id, --validator-id, --executor-id, and --idempotency-key", file=sys.stderr)
        return 2
    if not allow_state_change:
        result = {
            "schema_version": VALIDATION_EXECUTION_LEASE_REQUEST_SCHEMA_VERSION,
            "error": "explicit_state_change_acknowledgement_required",
            "message": "validation-lease acquire requires literal --allow-state-change",
            "lease_acquired": False,
            "runtime_execution_authorized": False,
        }
        print(json.dumps(result, indent=2, ensure_ascii=False) if as_json else result["message"])
        return 3

    try:
        plan = load_validation_plan(positionals[0])
        evidence = load_validation_evidence_bundle(positionals[1])
        evidence_binding = bind_validation_evidence(plan, evidence, store_dir=store_dir)
        obligations = load_proof_obligation_set(positionals[2])
        request_set = build_validation_execution_requests(
            plan, evidence_binding, obligations,
            validator_id=validator_id, executor_id=executor_id,
        )
        assertion = json.loads(Path(positionals[3]).read_text(encoding="utf-8"))
        if not isinstance(assertion, dict):
            raise ValidationAuthorizationError("authorization assertion must be a JSON object")
        authorization_binding = build_validation_authorization_binding(
            request_set, request_id, assertion,
            plan=plan, binding=evidence_binding, obligation_set=obligations,
            validator_id=validator_id, executor_id=executor_id, store_dir=store_dir,
        )
        lease_request = build_validation_execution_lease_request(
            authorization_binding, idempotency_key, lease_ttl_seconds=lease_ttl_seconds,
        )
        result = acquire_validation_execution_lease(
            lease_request, authorization_binding, request_set, request_id, assertion,
            plan=plan, binding=evidence_binding, obligation_set=obligations,
            validator_id=validator_id, executor_id=executor_id, store_dir=store_dir,
        )
    except (OSError, json.JSONDecodeError, ValidationEvidenceError, ProofObligationError,
            ValidationExecutionError, ValidationAuthorizationError, ValidationLeaseError) as exc:
        error = {
            "schema_version": VALIDATION_EXECUTION_LEASE_REQUEST_SCHEMA_VERSION,
            "error": str(exc), "error_code": "invalid_validation_lease_input",
            "lease_acquired": False, "attempt_registered": False,
            "runtime_dispatched": False, "validator_executed": False,
        }
        print(json.dumps(error, indent=2, ensure_ascii=False) if as_json else f"[PLW Error] {exc}", file=sys.stderr)
        return 2
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Validation Execution Lease]")
        print(f"# Decision: {result.get('decision')}")
        print(f"# Lease: {result.get('lease_id') or '-'}")
        print(f"# Attempt registration authorized by this call: {result.get('attempt_registration_authorized')}")
        print("# Boundary: no attempt, dispatch, runtime, validator result, postcondition, or truth has occurred.")
    return 0 if result.get("lease_acquired") is True or result.get("idempotent_recovery") is True else 3

def _validation_boundary_parser(prog: str, operations: tuple[str, ...], identity_name: str):
    import argparse
    parser = argparse.ArgumentParser(prog=prog)
    parser.add_argument("operation", choices=operations)
    parser.add_argument(identity_name)
    return parser


def _add_validation_binding_options(parser) -> None:
    parser.add_argument("--validator-id")
    parser.add_argument("--executor-id")
    parser.add_argument("--request-id")


def _parse_validation_state_args(parser, args: List[str]):
    parser.add_argument("--store-dir")
    parser.add_argument("--allow-state-change", action="store_true")
    parser.add_argument("--json", action="store_true", dest="as_json")
    return parser.parse_args(args)


def cmd_validation_attempt(args: List[str]) -> int:
    """Register or inspect the sole F135 attempt for an exact F134 lease."""
    parser = _validation_boundary_parser("plw validation-attempt", ("register", "status"), "lease_id")
    parser.add_argument("--attempt-key")
    _add_validation_binding_options(parser)
    opts = _parse_validation_state_args(parser, args)
    if opts.operation == "status":
        if opts.attempt_key or opts.validator_id or opts.allow_state_change:
            parser.error("status is read-only; remove registration-only options")
        result = inspect_validation_attempt(
            opts.lease_id, request_id=opts.request_id, executor_id=opts.executor_id,
            store_dir=opts.store_dir,
        )
        code = 0 if result.get("attempt_found") else 3
    else:
        if not opts.attempt_key or not opts.validator_id or not opts.executor_id:
            parser.error("register requires --attempt-key, --validator-id and --executor-id")
        if not opts.allow_state_change:
            result = {"error": "explicit_state_change_acknowledgement_required",
                      "attempt_registered": False, "dispatch_authorized": False}
            code = 3
        else:
            parent = inspect_validation_execution_lease(opts.lease_id, store_dir=opts.store_dir)
            if not parent.get("lease_found"):
                result = {"error": parent.get("error"), "attempt_registered": False,
                          "dispatch_authorized": False}
                code = 3
            else:
                request = build_validation_attempt_request(parent["lease_receipt"], opts.attempt_key)
                request["executor_id"] = opts.executor_id
                request["validator_id"] = opts.validator_id
                if opts.request_id is not None:
                    request["request_id"] = opts.request_id
                result = register_validation_attempt(request, store_dir=opts.store_dir)
                code = 0 if result.get("attempt_registered") or result.get("idempotent_recovery") else 3
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Validation Attempt]")
        print("# Decision: " + str(result.get("decision", result.get("error", "REJECTED"))))
        print("# Attempt: " + str(result.get("attempt_id") or "-"))
        print("# Boundary: durable registration only; dispatch requires a separate contract.")
    return code



def cmd_validation_dispatch(args: List[str]) -> int:
    parser = _validation_boundary_parser("plw validation-dispatch", ("prepare", "status"), "attempt_id")
    parser.add_argument("--dispatch-key")
    _add_validation_binding_options(parser)
    opts = _parse_validation_state_args(parser, args)
    if opts.operation == "status":
        if opts.dispatch_key or opts.validator_id or opts.allow_state_change:
            parser.error("status is read-only; remove preparation-only options")
        result = inspect_validation_dispatch_intent(opts.attempt_id, request_id=opts.request_id,
                                                    executor_id=opts.executor_id, store_dir=opts.store_dir)
        code = 0 if result.get("dispatch_intent_found") else 3
    else:
        if not opts.dispatch_key or not opts.validator_id or not opts.executor_id:
            parser.error("prepare requires --dispatch-key, --validator-id and --executor-id")
        if not opts.allow_state_change:
            result = {"error":"explicit_state_change_acknowledgement_required","dispatch_intent_prepared":False}
            code = 3
        else:
            try:
                req = build_validation_dispatch_request_from_store(opts.attempt_id, opts.dispatch_key, store_dir=opts.store_dir)
                req["validator_id"] = opts.validator_id
                req["executor_id"] = opts.executor_id
                if opts.request_id is not None:
                    req["request_id"] = opts.request_id
                result = prepare_validation_dispatch_intent(req, store_dir=opts.store_dir)
                code = 0 if result.get("dispatch_intent_prepared") or result.get("idempotent_recovery") else 3
            except (ValidationDispatchError, OSError, ValueError) as exc:
                result = {"error":str(exc),"dispatch_intent_prepared":False,"runtime_dispatch_authorized":False}
                code = 3
    print(json.dumps(result, indent=2, ensure_ascii=False) if opts.as_json else str(result.get("decision", result.get("error"))))
    return code


def cmd_validation_runtime(args: List[str]) -> int:
    parser = _validation_boundary_parser("plw validation-runtime", ("claim", "observe", "status"), "dispatch_intent_id")
    parser.add_argument("--runtime-key")
    parser.add_argument("--dispatch-state")
    parser.add_argument("--reconciliation-state")
    parser.add_argument("--terminal-status", default="")
    parser.add_argument("--observation-kind", default="runtime_status")
    parser.add_argument("--observation-json")
    parser.add_argument("--evidence-id", action="append", default=[], dest="evidence_ids")
    opts = _parse_validation_state_args(parser, args)
    if opts.operation == "status":
        if opts.allow_state_change or opts.runtime_key or opts.observation_json:
            parser.error("status is read-only")
        result = inspect_validation_runtime(opts.dispatch_intent_id, store_dir=opts.store_dir)
        code = 0 if result.get("runtime_state_found") else 3
    elif opts.operation == "claim":
        if not opts.runtime_key:
            parser.error("claim requires --runtime-key")
        if not opts.allow_state_change:
            result={"error":"explicit_state_change_acknowledgement_required","handoff_claimed":False}
            code=3
        else:
            result=claim_validation_runtime_handoff(opts.dispatch_intent_id, opts.runtime_key, store_dir=opts.store_dir)
            code=0 if result.get("handoff_claimed") or result.get("idempotent_recovery") else 3
    else:
        if not opts.allow_state_change:
            result={"error":"explicit_state_change_acknowledgement_required","observation_recorded":False}
            code=3
        elif not opts.dispatch_state or not opts.reconciliation_state or opts.observation_json is None:
            parser.error("observe requires --dispatch-state, --reconciliation-state and --observation-json")
        else:
            try:
                payload=json.loads(opts.observation_json)
            except json.JSONDecodeError as exc:
                parser.error(f"invalid --observation-json: {exc}")
            result=record_validation_runtime_observation(opts.dispatch_intent_id, {
                "dispatch_state":opts.dispatch_state,
                "reconciliation_state":opts.reconciliation_state,
                "terminal_status":opts.terminal_status,
                "observation_kind":opts.observation_kind,
                "observation":payload,
                "evidence_ids":list(opts.evidence_ids),
            }, store_dir=opts.store_dir)
            code=0 if result.get("observation_recorded") or result.get("idempotent_recovery") else 3
    print(json.dumps(result, indent=2, ensure_ascii=False) if opts.as_json else str(result.get("decision", result.get("error"))))
    return code

def cmd_validation_result(args: List[str]) -> int:
    parser = _validation_boundary_parser(
        "plw validation-result", ("accept", "status"), "runtime_observation_id"
    )
    parser.add_argument("result_json", nargs="?")
    parser.add_argument("request_set_json", nargs="?")
    opts = _parse_validation_state_args(parser, args)
    if opts.operation == "status":
        if opts.allow_state_change or opts.result_json or opts.request_set_json:
            parser.error("status is read-only")
        result = inspect_validation_result_evidence(
            opts.runtime_observation_id, store_dir=opts.store_dir
        )
        code = 0 if result.get("acceptance_found") else 3
    else:
        if not opts.result_json or not opts.request_set_json:
            parser.error("accept requires result.json and request-set.json")
        if not opts.allow_state_change:
            result = {
                "error": "explicit_state_change_acknowledgement_required",
                "evidence_accepted": False,
                "postcondition_proven": False,
                "truth_committed": False,
            }
            code = 3
        else:
            try:
                payload = load_validation_result_payload(opts.result_json)
                request_set = load_validation_execution_request_set(opts.request_set_json)
                result = accept_validation_result_evidence(
                    opts.runtime_observation_id, payload, request_set, store_dir=opts.store_dir
                )
                code = 0 if result.get("evidence_accepted") else 3
            except (ValidationResultEvidenceError, OSError, ValueError) as exc:
                result = {
                    "error": str(exc),
                    "evidence_accepted": False,
                    "postcondition_proven": False,
                    "truth_committed": False,
                }
                code = 3
    print(json.dumps(result, indent=2, ensure_ascii=False) if opts.as_json else str(result.get("decision", result.get("error"))))
    return code


def cmd_validation_scenario_bind(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw validation-scenario-bind")
    parser.add_argument("acceptance_id")
    parser.add_argument("plan_json")
    parser.add_argument("result_json")
    parser.add_argument("--store-dir")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    try:
        plan = load_validation_plan(opts.plan_json)
        payload = load_validation_result_payload(opts.result_json)
        result = bind_execution_backed_scenario_evidence(
            plan, payload, opts.acceptance_id, store_dir=opts.store_dir
        )
        code = 0
    except (ExecutionBackedEvidenceError, ValidationEvidenceError, ValidationResultEvidenceError, OSError, ValueError) as exc:
        result = {
            "error": str(exc),
            "execution_backed_evidence_bound": False,
            "proof_obligation_coverage": False,
            "postcondition_proven": False,
            "truth_committed": False,
        }
        code = 3
    print(json.dumps(result, indent=2, ensure_ascii=False) if opts.as_json else str(result.get("execution_backed_evidence_binding_digest", result.get("error"))))
    return code


def cmd_postcondition_proof(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw postcondition-proof")
    parser.add_argument("binding_json")
    parser.add_argument("coverage_json")
    parser.add_argument("obligations_json")
    parser.add_argument("spec_json")
    parser.add_argument("--current-revision", required=True)
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    try:
        binding = load_execution_backed_scenario_evidence(opts.binding_json)
        coverage = load_postcondition_coverage(opts.coverage_json)
        obligations = load_proof_obligation_set(opts.obligations_json)
        spec = load_postcondition_spec(opts.spec_json)
        result = assemble_postcondition_proof(
            binding, coverage, obligations, spec, current_revision=opts.current_revision,
        )
        code = 0
    except (ExecutionBackedEvidenceError, ProofObligationError, PostconditionProofError, OSError, ValueError) as exc:
        result = {
            "schema_version": POSTCONDITION_PROOF_SCHEMA_VERSION,
            "error": str(exc),
            "proof_state": "UNRESOLVED",
            "postcondition_proven": False,
            "truth_committed": False,
        }
        code = 3
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print(f"# [PLW Postcondition Proof] {result.get('proof_state', 'UNRESOLVED')}")
        print(f"# Postcondition proven: {result.get('authority', {}).get('postcondition_proven', False)}")
        print("# Boundary: proof is scoped to declared change/revision/scenarios/checks/obligations; no source mutation attestation, stable promotion, or truth commit.")
    return code


def cmd_fullstack_state(args: List[str]) -> int:
    import argparse
    parser = argparse.ArgumentParser(prog="plw fullstack-state")
    parser.add_argument("root")
    parser.add_argument("behavior_json")
    parser.add_argument("postcondition_json")
    parser.add_argument("spec_json")
    parser.add_argument("--geometry-impact", action="append", default=[])
    parser.add_argument("--ui-reference", action="append", default=[])
    parser.add_argument("--current-revision", required=True)
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(list(args))
    try:
        behavior = load_execution_backed_scenario_evidence(opts.behavior_json)
        postcondition = load_full_stack_json_artifact(opts.postcondition_json)
        spec = load_full_stack_json_artifact(opts.spec_json)
        impacts = [load_full_stack_json_artifact(path) for path in opts.geometry_impact]
        refs = [load_full_stack_json_artifact(path) for path in opts.ui_reference]
        result = compose_full_stack_validated_state(
            opts.root, behavior, postcondition, spec, current_revision=opts.current_revision,
            geometric_impacts=impacts, ui_references=refs,
        )
        code = 0
    except (ExecutionBackedEvidenceError, FullStackValidatedStateError, OSError, ValueError) as exc:
        result = {
            "schema_version": FULL_STACK_VALIDATED_STATE_SCHEMA_VERSION,
            "error": str(exc), "composition_state": "UNRESOLVED",
            "truth_committed": False, "stable_promoted": False,
        }
        code = 3
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print(f"# [PLW Full-Stack State] {result.get('composition_state', 'UNRESOLVED')}")
        print("# Boundary: authority composition only; no graph merge, evidence voting, source mutation attestation, stable promotion, or truth commit.")
    return code


def cmd_topology(args: List[str]) -> int:
    """Bounded first-contact file→import→file orientation for a target repository."""
    import argparse
    parser = argparse.ArgumentParser(prog="plw topology")
    parser.add_argument("root", nargs="?", default=".")
    parser.add_argument("--top", type=int, default=8)
    parser.add_argument("--full-edges", action="store_true")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    result = hott_kernel.kernel_topology(
        opts.root, top_n=opts.top, include_edges=opts.full_edges
    )
    code = 0 if not result.get("error") else 3
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Target Codebase Topology]")
        print("# Authority: static structural orientation only")
        if result.get("error"):
            print("# Error: " + str(result.get("error")))
        else:
            orient = result.get("target_orientation", {}) or {}
            inv = orient.get("inventory", {}) or {}
            print(f"# Files: {inv.get('files', 0)} | internal import edges: {inv.get('internal_import_edges', 0)}")
            print(f"# Entrypoints: {inv.get('entrypoints', 0)} | tests: {inv.get('tests', 0)} | cyclic SCCs: {inv.get('cyclic_sccs', 0)}")
            print(f"# Unresolved imports: {inv.get('unresolved_imports', 0)} | external imports: {inv.get('external_imports', 0)}")
            print("# Next: resolve the task target inside this topology before selecting a specialized capability.")
    return code


def _parse_target_task_args(args: List[str], prog: str):
    import argparse
    parser = argparse.ArgumentParser(prog=prog)
    parser.add_argument("task")
    parser.add_argument("root", nargs="?", default=".")
    parser.add_argument("--target", action="append", default=[])
    parser.add_argument("--top", type=int, default=8)
    parser.add_argument("--neighbors", type=int, default=8)
    parser.add_argument("--json", action="store_true", dest="as_json")
    return parser.parse_args(args)


def cmd_resolve_target(args: List[str]) -> int:
    """Resolve task-relevant graph nodes before capability selection."""
    opts = _parse_target_task_args(args, "plw resolve-target")
    result = hott_kernel.kernel_resolve_target(
        opts.root, opts.task, target_files=opts.target or None,
        top_n=opts.top, neighborhood_limit=opts.neighbors,
    )
    code = 0 if not result.get("error") else 3
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Target Resolution]")
        print("# Authority: retrieval + graph-grounded structural relevance only")
        if result.get("error"):
            print("# Error: " + str(result.get("error")))
        else:
            resolution = (result.get("target_resolution") or {}).get("resolution", {})
            print("# State: " + str(resolution.get("state")))
            print("# Selected: " + ", ".join(resolution.get("selected_targets", []) or []))
            print("# Next: derive unknown/evidence needs; do not select capability from task wording alone.")
    return code


def cmd_evidence_needs(args: List[str]) -> int:
    """Derive known/unknown/evidence-needed after target grounding."""
    opts = _parse_target_task_args(args, "plw evidence-needs")
    result = hott_kernel.kernel_evidence_needs(
        opts.root, opts.task, target_files=opts.target or None,
        top_n=opts.top, neighborhood_limit=opts.neighbors,
    )
    code = 0 if not result.get("error") else 3
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Evidence-Need Derivation]")
        print("# Authority: target-grounded gap derivation only; no capability selected")
        if result.get("error"):
            print("# Error: " + str(result.get("error")))
        else:
            deriv = result.get("evidence_need_derivation", {}) or {}
            print("# Stage: " + str(deriv.get("stage")))
            for row in deriv.get("evidence_needed", [])[:8]:
                print("# Need: " + str(row.get("need")))
            print("# Next: match unsatisfied evidence needs to capability contracts in a separate stage.")
    return code


def cmd_capability_match(args: List[str]) -> int:
    """Match target-grounded evidence classes to PLW capability contracts."""
    opts = _parse_target_task_args(args, "plw capability-match")
    result = hott_kernel.kernel_capability_match(
        opts.root, opts.task, target_files=opts.target or None,
        top_n=opts.top, neighborhood_limit=opts.neighbors,
    )
    code = 0 if not result.get("error") else 3
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Evidence-Driven Capability Matching]")
        print("# Authority: candidate projection only; agent chooses; no capability executed")
        if result.get("error"):
            print("# Error: " + str(result.get("error")))
        else:
            selection = result.get("capability_selection", {}) or {}
            print("# Stage: " + str(selection.get("stage")))
            for row in selection.get("candidate_capabilities", [])[:8]:
                print("# Candidate: %s | covers=%s | aggregate=%s" % (
                    row.get("capability"),
                    ",".join(row.get("covers_evidence_classes", []) or []),
                    row.get("aggregate_score"),
                ))
            print("# Uncovered needs: " + str(len(selection.get("uncovered_needs", []) or [])))
            print("# Next: agent inspects information gain and exact skill paths before choosing any call.")
    return code


def cmd_operation_plan(args: List[str]) -> int:
    """Resolve a declarative operation plan without invoking the operation."""
    import argparse
    parser = argparse.ArgumentParser(prog="plw operation-plan")
    parser.add_argument("operation", choices=("resolve",))
    parser.add_argument("plan")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    try:
        if opts.plan == "-":
            raw = sys.stdin.read()
        else:
            raw = Path(opts.plan).read_text(encoding="utf-8")
        plan = json.loads(raw)
        if not isinstance(plan, dict):
            raise ValueError("operation plan must be a JSON object")
    except (OSError, json.JSONDecodeError, ValueError) as exc:
        result = {
            "schema_version": "plw-operation-plan-cli-error-v1",
            "error": "PLAN_DOCUMENT_UNREADABLE_OR_INVALID",
            "detail": str(exc),
            "authority": {
                "invocation_authorized": False,
                "executes_operation": False,
                "commits_truth": False,
            },
        }
        if opts.as_json:
            print(json.dumps(result, indent=2, ensure_ascii=True))
        else:
            print("# [PLW Operation Plan]")
            print("# Error: " + result["error"])
            print("# Detail: " + result["detail"])
        return 2
    result = resolve_operation_plan(plan)
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=True))
    else:
        print("# [PLW Operation Plan]")
        print("# Status: " + str(result.get("status")))
        print("# Reason: " + str(result.get("reason")))
        print("# Operation: " + str(result.get("resolved_operation_id") or "-"))
        print("# Compiled argv: " + json.dumps(result.get("compiled_argv"), ensure_ascii=False))
        print("# Authority: preflight only; no invocation, execution, mutation, correctness, evidence acceptance, or truth authority")
    return 0


def cmd_candidate(args: List[str]) -> int:
    """Select a discovery candidate and optionally match its internal contract."""
    import argparse
    parser = argparse.ArgumentParser(prog="plw candidate")
    parser.add_argument("operation", choices=("select", "capability-match"))
    parser.add_argument("task")
    parser.add_argument("root", nargs="?", default=".")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    result = run_candidate_provenance(
        opts.root,
        opts.task,
        include_capability_match=opts.operation == "capability-match",
    )
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        selection = result.get("candidate_selection", {}) or {}
        print("# [PLW Candidate Selection]")
        print("# Resolution: " + str(selection.get("resolution")))
        print("# Reason: " + str(selection.get("reason")))
        print("# Discovery snapshot: " + str(selection.get("discovery_snapshot_digest")))
        print("# Candidate set: " + str(selection.get("candidate_set_digest")))
        print("# Selected candidate: " + str(selection.get("selected_candidate_id") or "-"))
        print("# Symbol/path: %s | %s" % (
            selection.get("selected_symbol") or "-",
            selection.get("selected_source_path") or "-",
        ))
        if "candidate_capability_match" in result:
            route = result["candidate_capability_match"]
            print("# Capability match: " + str(route.get("status")))
            print("# Matched internal contract: " + str(route.get("matched_capability_id") or "-"))
        print("# Authority: selection/match only; no authorization, execution, mutation, correctness, or evidence acceptance")
    return 0


def _print_portable_result(result: Dict[str, Any], *, as_json: bool, title: str) -> int:
    code = 0 if not result.get("error") and result.get("ready", True) is not False else 3
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return code
    print(f"# [{title}]")
    if result.get("error"):
        print("# Error: " + str(result.get("error")))
        return code
    if "ready" in result:
        print("# Version: " + str(result.get("version")))
        print("# READY: " + ("true" if result.get("ready") else "false"))
        support = result.get("target_support", {}) or {}
        print("# Stable topology: %s | supported_files=%s | extensions=%s" % (
            "READY" if support.get("topology_ready") else "NOT_READY",
            support.get("supported_source_files", 0),
            ",".join(support.get("topology_extensions", []) or []),
        ))
        print("# Topology support reason: " + str(support.get("reason", "unknown")))
        for name, passed in (result.get("checks", {}) or {}).items():
            print(f"# {name}: {'PASS' if passed else 'FAIL'}")
        browser = ((result.get("runtime") or {}).get("browser") or {})
        print("# Chromium: " + (str(browser.get("executable")) if browser.get("available") else "optional / not found"))
        engine = ((result.get("runtime") or {}).get("codesearch_engine") or {})
        print("# Tree-sitter engine: " + (str(engine.get("binary")) + " schema=" + str(engine.get("schema")) if engine.get("available") else "optional / not found"))
        print("# First contact: " + str((result.get("agent") or {}).get("first_contact")))
        return code
    if "capabilities" in result:
        print("# Capabilities: " + str(result.get("capability_count", 0)))
        for row in result.get("capabilities", []):
            print(f"{row.get('command')}\t{row.get('skill_path')}")
        return code
    if "skills" in result:
        print(f"# Skills: {result.get('embedded_count', 0)}/{result.get('skill_count', 0)} embedded")
        for row in result.get("skills", []):
            print(f"{'OK' if row.get('embedded') else 'MISSING'}\t{row.get('skill_path')}")
        return code
    if "content" in result:
        print(result.get("content", ""), end="" if str(result.get("content", "")).endswith("\n") else "\n")
        return code
    print(json.dumps(result, indent=2, ensure_ascii=False))
    return code


def cmd_ast(args: List[str]) -> int:
    """Tree-sitter structural queries via the codesearch-go engine.

    Read-only discovery over the target root: where a symbol is
    defined, who calls it, what it relates to, or a ranked search.
    The engine is an external binary (explicit --binary, CODESEARCH_BIN,
    or PATH) supplied by the agent harness; a missing or incompatible
    engine fails closed with exit 3 and an error envelope.  Callers are
    name-based leads, not type-resolved proof: read the cited file
    before claiming a call.
    """
    import argparse
    parser = argparse.ArgumentParser(prog="plw ast")
    parser.add_argument("operation", choices=("defs", "callers", "related", "search", "doctor"))
    parser.add_argument("root", nargs="?", default=".")
    parser.add_argument("query", nargs="?")
    parser.add_argument("--binary", default=None)
    parser.add_argument("--timeout", type=int, default=120)
    parser.add_argument("--limit", type=int, default=None)
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    root = Path(opts.root).expanduser().resolve()
    try:
        from core.codesearch_engine import (
            DEFAULT_TIMEOUT_S,
            discover_codesearch,
            doctor_envelope,
            error_envelope,
            run_query,
            EngineError,
        )
    except Exception as exc:  # engine module must never take the CLI down
        result: Dict[str, Any] = error_envelope(
            opts.operation, str(root), opts.query or "",
            f"codesearch engine module unavailable: {exc}",
        )
        print(json.dumps(result, indent=2, ensure_ascii=False))
        return 3
    timeout_s = opts.timeout if opts.timeout is not None else DEFAULT_TIMEOUT_S
    try:
        if opts.operation == "doctor":
            result = doctor_envelope(root, opts.binary)
        else:
            if not opts.query:
                parser.error("plw ast defs|callers|related|search requires <query>")
            binary, discovered_via = discover_codesearch(opts.binary)
            result = run_query(
                opts.operation, root, opts.query,
                binary, discovered_via, timeout_s, opts.limit,
            )
    except EngineError as exc:
        result = error_envelope(opts.operation, str(root), opts.query or "", str(exc))
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW AST]")
        if not result.get("ok"):
            print("# Error: " + str(result.get("error")))
        else:
            engine = result.get("engine") or {}
            print("# Mode: " + str(result.get("mode")))
            print("# Engine: " + str(engine.get("binary")) + " schema=" + str(engine.get("schema")))
            print("# Total: " + str((result.get("summary") or {}).get("total")))
            rows = ((result.get("engine_result") or {}).get("hits")
                    or (result.get("engine_result") or {}).get("calls")
                    or (result.get("engine_result") or {}).get("related") or [])
            for row in rows:
                if "snippet" in row:
                    print(f"{row.get('path')}:{row.get('start')}\t{row.get('kind')}\t{row.get('snippet')}")
                elif "caller" in row:
                    print(f"{row.get('path')}:{row.get('line')}\tcaller={row.get('caller')}\t{row.get('text', '').strip()}")
                else:
                    print(f"{row.get('path')}:{row.get('line')}\t{row.get('kind')}\t{row.get('detail')}")
    return 0 if result.get("ok") else 3


def cmd_doctor(args: List[str]) -> int:
    """Verify standalone portable-agent readiness without touching target source."""
    import argparse
    parser = argparse.ArgumentParser(prog="plw doctor")
    parser.add_argument("root", nargs="?", default=".")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    result = portable_doctor(opts.root)
    return _print_portable_result(result, as_json=opts.as_json, title="PLW Doctor")


def cmd_capability(args: List[str]) -> int:
    """List or describe the embedded semantic capability registry."""
    import argparse
    parser = argparse.ArgumentParser(prog="plw capability")
    parser.add_argument("operation", choices=("list", "describe"))
    parser.add_argument("value", nargs="?")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    if opts.operation == "list":
        result = portable_list_capabilities()
    else:
        if not opts.value:
            parser.error("capability describe requires <command>")
        result = portable_describe_capability(opts.value)
    return _print_portable_result(result, as_json=opts.as_json, title="PLW Capability")


def cmd_skill(args: List[str]) -> int:
    """List or read exact version-aligned skill contracts bundled with PLW."""
    import argparse
    parser = argparse.ArgumentParser(prog="plw skill")
    parser.add_argument("operation", choices=("list", "show"))
    parser.add_argument("value", nargs="?")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    if opts.operation == "list":
        result = portable_list_skills()
    else:
        if not opts.value:
            parser.error("skill show requires <command-or-skill-path>")
        result = portable_read_skill(opts.value)
    return _print_portable_result(result, as_json=opts.as_json, title="PLW Skill")


def cmd_agent(args: List[str]) -> int:
    """Official target-first portable-agent orientation entrypoint."""
    import argparse
    parser = argparse.ArgumentParser(prog="plw agent")
    parser.add_argument("operation", choices=("orient",))
    parser.add_argument("root", nargs="?", default=".")
    parser.add_argument("--task", required=True)
    parser.add_argument("--target", action="append", default=[])
    parser.add_argument("--ui-selector", default=None)
    parser.add_argument("--goal", default="auto")
    parser.add_argument("--depth", default="auto")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    result = hott_kernel.kernel_work(
        scan_root=opts.root,
        task=opts.task,
        target_files=opts.target or None,
        goal=opts.goal,
        depth=opts.depth,
        ui_selector=opts.ui_selector,
    )
    support = (portable_doctor(opts.root).get("target_support", {}) or {})
    if isinstance(result, dict):
        result = dict(result)
        result["portable_agent_surface"] = portable_orientation_surface()
        result["target_support"] = support
        result["dynamic_context"] = portable_dynamic_context(opts.root)
        reason = str(support.get("reason") or "")
        if support.get("topology_ready"):
            portable_state = "TARGET_TOPOLOGY_READY"
        elif reason == "NO_STABLE_TOPOLOGY_SOURCE_FILES":
            portable_state = "TARGET_TOPOLOGY_LANGUAGE_UNSUPPORTED"
        else:
            portable_state = "TARGET_TOPOLOGY_UNAVAILABLE"
        result["portable_agent_state"] = portable_state
        result["portable_agent_reason"] = reason
    code = 0 if isinstance(result, dict) and not result.get("error") else 3
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Agent Orient]")
        if isinstance(result, dict) and result.get("error"):
            print("# Error: " + str(result.get("error")))
        else:
            frame = (result.get("agent_decision_frame", {}) or {}) if isinstance(result, dict) else {}
            target = (frame.get("target", {}) or {})
            handoff = (result.get("skill_handoff", {}) or {}) if isinstance(result, dict) else {}
            print("# Portable topology state: " + str(result.get("portable_agent_state", "unknown")))
            print("# Topology support reason: " + str(result.get("portable_agent_reason", "unknown")))
            ctx = result.get("dynamic_context", {}) or {}
            print("# Dynamic context: " + str(ctx.get("context", "unknown")))
            print("# Context reason: " + str(ctx.get("reason", "unknown")))
            sq = ctx.get("structural_query", {}) or {}
            if sq.get("route"):
                print("# Structural route: " + str(sq["route"]))
            print("# Target state: " + str(target.get("state", "unknown")))
            ui_handoff = result.get("ui_source_handoff", {}) or {}
            if ui_handoff.get("status") != "NOT_APPLICABLE":
                print("# UI source handoff: " + str(ui_handoff.get("status", "unknown")))
                if ui_handoff.get("source_target_hint"):
                    print("# UI source target hint: " + str(ui_handoff["source_target_hint"]["component_file"]))
            print("# Action readiness: " + str(frame.get("action_readiness", "unknown")))
            print("# Skill handoff: " + str(handoff.get("skill_path", "-")))
            print("# Authority: orientation only; no runtime execution or source mutation granted")
    return code


def cmd_semantic(args: List[str]) -> int:
    """Read-only predictive semantics for agents unfamiliar with PLW vocabulary."""
    import argparse
    parser = argparse.ArgumentParser(prog="plw semantic")
    parser.add_argument("operation", choices=("describe", "route"))
    parser.add_argument("value")
    parser.add_argument("--goal", default="auto")
    parser.add_argument("--json", action="store_true", dest="as_json")
    opts = parser.parse_args(args)
    result = (
        describe_command(opts.value)
        if opts.operation == "describe"
        else route_task_semantics(opts.value, goal=opts.goal)
    )
    code = 0 if result.get("known", result.get("route_found", False)) else 3
    if opts.as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("# [PLW Semantic Affordance]")
        print("# Authority: descriptive/documentation routing only")
        if opts.operation == "describe":
            print("# Command: " + str(result.get("command")))
            print("# Canonical: " + ", ".join(result.get("canonical_concepts", [])[:4]))
            gap = result.get("epistemic_gap", {}) or {}
            print("# Missing: " + str(gap.get("missing", result.get("error", "-"))))
            print("# Information gain: " + "; ".join(result.get("information_gain", [])[:4]))
            print("# Read before use: " + str(result.get("read_before_use", "-")))
        else:
            print("# Capability: " + str(result.get("capability", "-")))
            print("# Skill: " + str(result.get("skill_path", "-")))
            print("# Canonical: " + ", ".join(result.get("canonical_concepts", [])[:4]))
            print("# Information gain: " + "; ".join(result.get("information_gain", [])[:4]))
    return code

def main() -> None:
    if len(sys.argv) >= 2 and sys.argv[1] in ("--version", "-V", "version"):
        options = sys.argv[2:]
        if options not in ([], ["--json"]):
            print("[PLW Error] version accepts only --json", file=sys.stderr)
            sys.exit(2)
        version = portable_build_version()
        if options == ["--json"]:
            print(json.dumps({"schema_version": "plw-version-v1", "version": version}))
        else:
            print(f"plw {version}")
        sys.exit(0 if version != "unknown" else 1)
    if len(sys.argv) < 2 or sys.argv[1] in ("-h", "--help", "help"):
        print("""PLW — Proof Logic Working & HoTT Kernel CLI
Usage:
  plw doctor [root] [--json]
      -> Verifies standalone binary/embedded skill readiness; Chromium is optional for static capabilities
  plw ast <defs|callers|related|search|doctor> [root] [query] [--binary PATH] [--timeout N] [--limit N] [--json]
      -> Tree-sitter structural queries via the codesearch-go engine (external binary, fail-closed); callers are name-based leads, not proof
  plw agent orient [root] --task TASK [--target file] [--goal GOAL] [--depth DEPTH] [--json]
      -> Official target-first agent entrypoint; orientation only, no runtime/source mutation authority
  plw capability list [--json]
  plw capability describe <command> [--json]
      -> Self-describing semantic capability registry embedded in the executable
  plw skill list [--json]
  plw skill show <command-or-skill-path> [--json]
      -> Lists or reads exact version-aligned embedded skill contracts
  plw topology [root] [--top N] [--full-edges] [--json]
      -> First-contact target-codebase map: file→import→file graph, hubs, entrypoints/tests, SCCs, unresolved/external imports
  plw resolve-target <task> [root] [--target file] [--top N] [--neighbors N] [--json]
      -> Grounds task to graph nodes; ambiguity/unresolved blocks specialized routing
  plw evidence-needs <task> [root] [--target file] [--json]
      -> Derives known/unknown/technical evidence requirements from grounded target; selects no capability
      -> Resolves task-relevant graph nodes and structural neighborhoods; text similarity never proves ownership
  plw capability-match <task> [root] [--target file] [--json]
      -> Matches machine-readable evidence classes to capability contracts; projects candidates but executes nothing
  plw operation-plan resolve <plan.json|-> [--json]
      -> Resolves one declarative plan against public operation contracts; compiles reference argv but executes nothing
  plw candidate select <task> [root] [--json]
      -> Binds task selection to a complete simplification candidate-set snapshot; unresolved evidence stays ambiguous
  plw candidate capability-match <task> [root] [--json]
      -> Selects and matches source facts to an internal contract in one snapshot; no authorization or execution
  plw work <task> [root] [--target file] [--goal GOAL] [--depth DEPTH]
      -> Task work on the same graph; returns target_orientation + target_resolution before specialized capability semantics
  plw semantic describe <command> [--json]
  plw semantic route <task> [--goal GOAL] [--json]
      -> Read-only predictive semantics: canonical technical anchors, information gain, and exact skill handoff
  plw context <query> [root] [--target file] [--budget N] [--cache-mode MODE] [--json]
      -> Generates budget-bounded prompt context with Kan Extension imputation
  plw transitions <query> [root] [--target file] [--budget N] [--lineage sha256:id] [--json]
      -> Compiles valid proof-gap transitions; never executes them
  plw kan <file_or_query> [root] [--mode lan|ran|both] [--cache-mode MODE] [--json]
      -> Imputes expected imports, callers, and conventions for new/isolated files
  plw check [root] [--analyzers a,b] [--json]
      -> Quick architectural and boundary violation check
  plw steer [root] [--json]
      -> Cross-domain architectural steering guidance
  plw brief <file> [root] [--json]
      -> Brief profile of a single file
  plw impact <file> [root] [--json]
      -> Downstream impact analysis
  plw outline <file> [root] [--json]
      -> File symbols and exports outline
  plw compact-output [--command CMD] [--kind KIND] [--budget N] [--lineage sha256:id] [--json]
      -> Conservatively compacts stdin from Linux commands; never executes them
  plw recover-output <sha256:id>
      -> Restores exact raw command output from the content-addressed recovery store
  plw attest-runtime <sha256:output-id> --exit-code N --command CMD --claim runtime_test_result|runtime_behavior [--root .] [--target file]
      -> Binds exact runtime output to a truth scope + graph snapshot; never executes the command
  plw output-telemetry [--json]
      -> Summarizes recovery/repeat/over-budget behavior; never auto-mutates policy
  plw transition-memory [--json]
      -> Shows project-scoped transition outcome experience; never truth evidence
  plw transition-model [--state-class sha256:...] [--json]
  plw q-values [--state-class sha256:...] [--json]
  plw reconcile-outcomes [--record] [--json]
  plw lineage <sha256:id> [--json]
      -> Traces explicit parent-linked decision ancestry; never infers semantic causality
  plw accountability <sha256:decision-id> [--json]
      -> Audits who filtered/compacted evidence, the resulting evidence debt, and recovery duty
  plw accountability-chain <sha256:decision-id> [--limit N] [--json]
      -> Verifies and summarizes F111 relevance -> F112 delivery accountability without raw prompt/source
  plw action-lease acquire [--store-dir DIR] --allow-state-change [--json]
      -> Atomically consumes one authorization/freshness nonce pair for one allowlisted executor; reads the full chain bundle from stdin
  plw action-lease status <sha256:lease-id> [--json]
      -> Verifies lease state read-only; inspection never regrants executor-start authority
  plw action-outcome register [--store-dir DIR] --allow-state-change [--json]
      -> Claims one exact F121 lease handoff into a durable F122 attempt record; does not prove external execution started
  plw action-outcome record [--store-dir DIR] --allow-state-change [--json]
      -> Records one terminal executor observation as SUCCEEDED, FAILED, or INDETERMINATE without settling postconditions
  plw action-outcome status <sha256:lease-id> [--json]
      -> Reads attempt/outcome state; a registered attempt without terminal receipt remains unresolved
  plw action-adapter prepare [--store-dir DIR] --allow-state-change [--json]
      -> Persists one F123 dispatch intent for an exact unresolved F122 attempt; does not invoke a concrete adapter
  plw action-adapter observe [--store-dir DIR] --allow-state-change [--json]
      -> Records one adapter observation and forwards terminal observations into F122 without proving postconditions
  plw action-adapter status <sha256:lease-id> [--json]
      -> Reads dispatch/observation bridge state; missing observation remains unresolved
  plw angular-anchors [root] [--file FILE] [--selector SELECTOR] [--no-resources] [--json]
  plw ui static <query> [root] [--selector SELECTOR] [--max-candidates N] [--json]
  plw ui map <snapshot.json> [root] [--min-gap PX] [--json]
  plw ui inspect <snapshot.json> <ui-id> [root] [--json]
  plw ui scenarios <snapshot1.json> <snapshot2.json> [...] [--root ROOT] [--json]
  plw ui diff <baseline.json> <candidate.json> [--root ROOT] [--baseline-root ROOT] [--json]
  plw ui hunt <candidate.json> [--baseline baseline.json] [--root ROOT] [--runtime-reference FILE] [--json]
  plw ui diagnose <candidate.json> [--baseline baseline.json] [--root ROOT] [--runtime-reference FILE] [--json]
  plw ui reproduce <diagnosis.json> --root ROOT (--url URL | --html-file FILE) [--candidate-id ID] [--setup-js FILE] [--out FILE] [--json]
  plw ui capture-script [root] --revision REV
  plw ui-reference <snapshot.json> [root] [--no-resources] [--json]
  plw runtime-ui-reference <lease-id> <snapshot.json> [root] [--store-dir DIR] [--no-resources] [--json]
      -> Binds one F125 UI reference to verified F123 capture provenance without executing runtime
  plw geometric-impact <snapshot.json> [root] [--focus UI_ID] [--constraints query.json] [--proximity PX] [--lease-id ID --store-dir DIR] [--no-resources] [--json]
      -> Evaluates explicit geometric constraints and projects selected UI owners to bounded topology impact; read-only
  plw validation-plan <snapshot.json> [root] [--scenario-catalog catalog.json] [--max-scenarios N] [--focus UI_ID] [--constraints query.json] [--lease-id ID --store-dir DIR] [--json]
      -> Plans a bounded recoverable validation scenario set from F127 impact; never executes renders/tests
  plw validation-evidence <plan.json> <evidence.json> [--store-dir DIR] [--json]
      -> Binds external evidence to exact F128 scenarios and separates verified receipts from exact content provenance
  plw validation-coverage <plan.json> <evidence.json> <obligations.json> [--store-dir DIR] [--json]
  plw validation-coverage execution-backed <f139-binding.json> <obligations.json> [--json]
      -> Re-derives F129 or F139 proof obligations read-only; complete coverage is still not postcondition proof
  plw postcondition-proof <f139-binding.json> <f140-coverage.json> <obligations.json> <postcondition-spec.json> --current-revision REV [--json]
      -> F141 scoped postcondition assembly; no source mutation attestation, stable promotion, or truth commit
  plw fullstack-state <root> <f139-binding.json> <f141-proof.json> <fullstack-spec.json> --current-revision REV [--geometry-impact FILE] [--ui-reference FILE] [--json]
      -> F142 composes separate topology/geometry/correspondence/behavior/postcondition authorities; no graph merge or truth commit
  plw validation-request <plan.json> <evidence.json> <obligations.json> --validator-id ID --executor-id ID [--store-dir DIR] [--json]
      -> Re-derives F131 coverage and emits REQUESTED work without authorization or dispatch
  plw validation-authorization <plan.json> <evidence.json> <obligations.json> <assertion.json> --request-id ID --validator-id ID --executor-id ID [--store-dir DIR] [--json]
      -> Verifies a host-issued principal grant and binds it to one exact F132 request; still no lease or dispatch
  plw validation-lease acquire <plan.json> <evidence.json> <obligations.json> <assertion.json> --request-id ID --validator-id ID --executor-id ID --idempotency-key KEY [--lease-ttl-seconds N] [--store-dir DIR] --allow-state-change [--json]
      -> Atomically consumes one exact F133 binding into one short-lived lease; no attempt or dispatch occurs
  plw validation-lease status <sha256:lease-id> [--request-id ID] [--executor-id ID] [--store-dir DIR] [--json]
  plw validation-attempt register <lease-id> --attempt-key KEY --validator-id ID --executor-id ID --allow-state-change [--store-dir DIR] [--json]
  plw validation-attempt status <lease-id> [--request-id ID] [--executor-id ID] [--store-dir DIR] [--json]
  plw validation-dispatch prepare <attempt-id> --dispatch-key KEY --validator-id ID --executor-id ID --allow-state-change [--store-dir DIR] [--json]
  plw validation-dispatch status <attempt-id> [--request-id ID] [--executor-id ID] [--store-dir DIR] [--json]
  plw validation-runtime claim <dispatch-intent-id> --runtime-key KEY --allow-state-change [--store-dir DIR] [--json]
  plw validation-runtime observe <dispatch-intent-id> --dispatch-state STATE --reconciliation-state STATE [--terminal-status STATUS] [--observation-kind KIND] --observation-json JSON [--evidence-id sha256:...]... --allow-state-change [--store-dir DIR] [--json]
  plw validation-runtime status <dispatch-intent-id> [--store-dir DIR] [--json]
      -> Reads a validation lease without regranting attempt or dispatch authority
  plw install [--system | --bin-dir DIR] [--json]
      -> Verifies F129 integrity and projects v1/v2 obligations with optional same-observation witness groups; never executes validators
  plw memory <subcommand> [...]
      -> Episodic/semantic memory operations; use `plw memory get <id>` for exact debt recovery
  plw fiber <subcommand> [...]
      -> Fiber and parallel transport operations
  plw <hott_kernel command> [...]
      -> Passthrough to all standard HoTT Kernel modes
""")
        sys.exit(0)

    cmd = sys.argv[1].lower()
    sub_args = sys.argv[2:]

    if cmd == "doctor":
        sys.exit(cmd_doctor(sub_args))
    elif cmd in ("ast", "codesearch", "tree-sitter", "tree_sitter"):
        sys.exit(cmd_ast(sub_args))
    elif cmd == "agent":
        sys.exit(cmd_agent(sub_args))
    elif cmd in ("capability", "capabilities"):
        sys.exit(cmd_capability(sub_args))
    elif cmd in ("skill", "skills"):
        sys.exit(cmd_skill(sub_args))
    elif cmd in ("topology", "codebase-topology", "codebase_topology", "import-graph", "import_graph"):
        sys.exit(cmd_topology(sub_args))
    elif cmd in ("resolve-target", "resolve_target", "target-resolve", "target_resolve"):
        sys.exit(cmd_resolve_target(sub_args))
    elif cmd in ("evidence-needs", "evidence_needs", "evidence-need", "evidence_need"):
        sys.exit(cmd_evidence_needs(sub_args))
    elif cmd in ("capability-match", "capability_match"):
        sys.exit(cmd_capability_match(sub_args))
    elif cmd in ("operation-plan", "operation_plan"):
        sys.exit(cmd_operation_plan(sub_args))
    elif cmd == "candidate":
        sys.exit(cmd_candidate(sub_args))
    elif cmd in ("semantic", "semantics", "affordance"):
        sys.exit(cmd_semantic(sub_args))
    elif cmd in ("context", "pack"):
        sys.exit(cmd_context(sub_args))
    elif cmd in ("transitions", "transition", "proof-transitions", "proof_transitions"):
        sys.exit(cmd_transitions(sub_args))
    elif cmd == "kan":
        sys.exit(cmd_kan(sub_args))
    elif cmd == "check":
        sys.exit(cmd_check(sub_args))
    elif cmd == "steer":
        sys.exit(cmd_steer(sub_args))
    elif cmd in ("compact-output", "compact_output", "observe-output", "observe_output"):
        sys.exit(cmd_compact_output(sub_args))
    elif cmd in ("recover-output", "recover_output"):
        sys.exit(cmd_recover_output(sub_args))
    elif cmd in ("attest-runtime", "attest_runtime", "runtime-attest", "runtime_attest"):
        sys.exit(cmd_attest_runtime(sub_args))
    elif cmd in ("output-telemetry", "output_telemetry", "command-telemetry", "command_telemetry"):
        sys.exit(cmd_output_telemetry(sub_args))
    elif cmd in ("transition-memory", "transition_memory", "outcome-memory", "outcome_memory"):
        sys.exit(cmd_transition_memory(sub_args))
    elif cmd in ("transition-model", "transition_model", "state-transition-model", "state_transition_model"):
        sys.exit(cmd_transition_model(sub_args))
    elif cmd in ("q-values", "q_values", "q-value", "q_value", "policy-values", "policy_values"):
        sys.exit(cmd_q_values(sub_args))
    elif cmd in ("reconcile-outcomes", "reconcile_outcomes", "delayed-cost", "delayed_cost"):
        sys.exit(cmd_reconcile_outcomes(sub_args))
    elif cmd in ("simplify", "self-simplify", "self_simplify"):
        sys.exit(cmd_simplify(sub_args))
    elif cmd in ("lineage", "decision-lineage", "decision_lineage"):
        sys.exit(cmd_lineage(sub_args))
    elif cmd in ("accountability-chain", "accountability_chain", "workflow-audit-chain", "workflow_audit_chain"):
        sys.exit(cmd_accountability_chain(sub_args))
    elif cmd in ("accountability", "context-accountability", "context_accountability", "audit-context", "audit_context"):
        sys.exit(cmd_accountability(sub_args))
    elif cmd in ("action-lease", "action_lease", "execution-lease", "execution_lease"):
        sys.exit(cmd_action_lease(sub_args))
    elif cmd in ("action-outcome", "action_outcome", "executor-outcome", "executor_outcome"):
        sys.exit(cmd_action_outcome(sub_args))
    elif cmd in ("action-adapter", "action_adapter", "executor-adapter", "executor_adapter"):
        sys.exit(cmd_action_adapter(sub_args))
    elif cmd in ("angular-anchors", "angular_anchors", "angular-ownership", "angular_ownership"):
        sys.exit(cmd_angular_anchors(sub_args))
    elif cmd in ("ui", "ui-map", "ui_map"):
        if cmd in ("ui-map", "ui_map"):
            sys.exit(cmd_ui_map(sub_args))
        sys.exit(cmd_ui(sub_args))
    elif cmd in ("ui-static", "ui_static"):
        sys.exit(cmd_ui_static(sub_args))
    elif cmd in ("ui-reference", "ui_reference", "geometry-reference", "geometry_reference"):
        sys.exit(cmd_ui_reference(sub_args))
    elif cmd in ("runtime-ui-reference", "runtime_ui_reference", "runtime-geometry-reference", "runtime_geometry_reference"):
        sys.exit(cmd_runtime_ui_reference(sub_args))
    elif cmd in ("geometric-impact", "geometric_impact", "ui-impact", "ui_impact"):
        sys.exit(cmd_geometric_impact(sub_args))
    elif cmd in ("validation-plan", "validation_plan", "scenario-plan", "scenario_plan"):
        sys.exit(cmd_validation_plan(sub_args))
    elif cmd in ("validation-evidence", "validation_evidence", "scenario-evidence", "scenario_evidence"):
        sys.exit(cmd_validation_evidence(sub_args))
    elif cmd in ("validation-coverage", "validation_coverage", "proof-obligations", "proof_obligations"):
        sys.exit(cmd_validation_coverage(sub_args))
    elif cmd in ("validation-request", "validation_request", "validation-handoff", "validation_handoff"):
        sys.exit(cmd_validation_request(sub_args))
    elif cmd in ("validation-authorization", "validation_authorization", "validation-authorize", "validation_authorize"):
        sys.exit(cmd_validation_authorization(sub_args))
    elif cmd in ("validation-attempt", "validation_attempt", "validator-attempt", "validator_attempt"):
        sys.exit(cmd_validation_attempt(sub_args))
    elif cmd in ("validation-dispatch", "validation_dispatch", "validator-dispatch", "validator_dispatch"):
        sys.exit(cmd_validation_dispatch(sub_args))
    elif cmd in ("validation-runtime", "validation_runtime", "validator-runtime", "validator_runtime"):
        sys.exit(cmd_validation_runtime(sub_args))
    elif cmd in ("validation-result", "validation_result", "validation-evidence-acceptance", "validation_evidence_acceptance"):
        sys.exit(cmd_validation_result(sub_args))
    elif cmd in ("validation-scenario-bind", "validation_scenario_bind", "execution-backed-evidence", "execution_backed_evidence"):
        sys.exit(cmd_validation_scenario_bind(sub_args))
    elif cmd in ("postcondition-proof", "postcondition_proof", "prove-postcondition", "prove_postcondition"):
        sys.exit(cmd_postcondition_proof(sub_args))
    elif cmd in ("fullstack-state", "fullstack_state", "full-stack-state", "full_stack_state"):
        sys.exit(cmd_fullstack_state(sub_args))
    elif cmd in ("validation-lease", "validation_lease", "validator-lease", "validator_lease"):
        sys.exit(cmd_validation_lease(sub_args))
    elif cmd in ("install", "setup", "bootstrap"):
        sys.exit(cmd_install(sub_args))
    else:
        # Standard passthrough to hott_kernel.main()
        # Rewriting sys.argv so hott_kernel sees original arguments
        sys.argv = [sys.argv[0]] + sys.argv[1:]
        try:
            hott_kernel.main()
        except SystemExit as se:
            sys.exit(se.code)
        except Exception as exc:
            print(f"[PLW Error] {exc}", file=sys.stderr)
            sys.exit(1)


if __name__ == "__main__":
    main()
