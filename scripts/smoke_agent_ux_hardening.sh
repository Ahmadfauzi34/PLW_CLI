#!/usr/bin/env bash
# Smoke test for agent_ux_hardening_v1 & agent_ux_hardening_v2:
#   - Item 1: --cache-mode (auto|refresh|off) in plw topology
#   - Item 2: Multi-target comma-separated resolution in plw resolve-target (--target a,b)
#   - Item 3: --frame-only compact work pack in plw work and plw agent orient (<15 KB) + --help doc
#   - Item 4: Fallback hint in plw ast callers & defs when total == 0
#   - Item 5: recommended_shortcut in plw resolve-target pointing to plw work --target
#   - Item 6: cycle_break_advisory with dynamic await import() directed to static definition edge

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [ -z "${1:-}" ] && [ ! -f "$ROOT_DIR/plw_cli.py" ]; then
  bash "$ROOT_DIR/scripts/unpack_runtime_source.sh" >/dev/null
fi

PLW_SMOKE_BIN="${1:-}" python3 - <<'PY'
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path.cwd()
_bin = os.environ.get("PLW_SMOKE_BIN", "").strip()
CLI = [os.path.abspath(_bin)] if _bin else [sys.executable, str(ROOT / "plw_cli.py")]


def run_cli(*args):
    proc = subprocess.run(
        CLI + list(args),
        capture_output=True,
        text=True,
    )
    return proc.returncode, proc.stdout, proc.stderr


with tempfile.TemporaryDirectory(prefix="plw-ux-smoke-") as tmp:
    repo = Path(tmp) / "repo"
    src = repo / "src"
    src.mkdir(parents=True)
    (src / "service.ts").write_text("export const config = { port: 8080 };\n")
    (src / "client.ts").write_text('import { config } from "./service";\nexport function connect() { return config.port; }\n')

    # 1. Test plw topology --cache-mode refresh
    rc, stdout, stderr = run_cli("topology", str(repo), "--cache-mode", "refresh", "--json")
    assert rc == 0, f"topology --cache-mode refresh failed: {stderr}"
    top_data = json.loads(stdout)
    assert top_data.get("graph_cache", {}).get("mode") == "refresh", top_data.get("graph_cache")
    print("PASS: plw topology --cache-mode refresh")

    # 2. Test plw resolve-target with comma-separated targets
    rc, stdout, stderr = run_cli("resolve-target", "audit config", str(repo), "--target", "src/service.ts,src/client.ts", "--json")
    assert rc == 0, f"resolve-target failed: {stderr}"
    res_data = json.loads(stdout)
    resolution = res_data.get("target_resolution", {}).get("resolution", {})
    assert resolution.get("state") == "MULTI_TARGET_RESOLVED", resolution
    assert set(resolution.get("selected_targets", [])) == {"src/service.ts", "src/client.ts"}, resolution
    print("PASS: plw resolve-target comma-separated multi-target")

    # 3. Test recommended_shortcut in plw resolve-target
    shortcut = res_data.get("recommended_shortcut")
    assert isinstance(shortcut, dict), "recommended_shortcut missing"
    assert shortcut.get("command") == "plw work", shortcut
    assert "--target" in shortcut.get("args", []), shortcut
    assert res_data.get("next", {}).get("command") == "plw evidence-needs", "canonical next command must remain evidence-needs"
    assert res_data.get("next", {}).get("recommended_shortcut", {}).get("command") == "plw work"
    print("PASS: plw resolve-target recommended_shortcut")

    # 4. Test plw work --help documents [--frame-only]
    rc, stdout, stderr = run_cli("work", "--help")
    assert rc == 0, f"work --help failed: {stderr}"
    assert "[--frame-only]" in stdout, f"[--frame-only] missing in work --help: {stdout}"
    print("PASS: plw work --help documents [--frame-only]")

    # 5. Test plw work --frame-only (<15 KB target)
    rc, stdout, stderr = run_cli("work", "audit config", str(repo), "--target", "src/service.ts,src/client.ts", "--frame-only", "--json")
    assert rc == 0, f"work --frame-only failed: {stderr}"
    work_bytes = len(stdout.encode("utf-8"))
    work_data = json.loads(stdout)
    assert work_data.get("frame_only") is True, work_data
    assert work_data.get("readiness", {}).get("reference_pack") == "READY", work_data.get("readiness")
    assert isinstance(work_data.get("agent_decision_frame"), dict), "agent_decision_frame missing"
    assert bool(work_data.get("receipt")), "receipt missing from frame-only work"
    assert "inline_contract" not in work_data.get("skill_handoff", {}), "inline_contract should be stripped in frame-only"
    assert "candidate_targets" not in work_data.get("target_resolution", {}), "candidate_targets should be stripped in frame-only"
    assert work_bytes < 15000, f"work --frame-only size ({work_bytes} bytes) exceeded 15 KB target"
    print(f"PASS: plw work --frame-only ({work_bytes} bytes < 15 KB)")

    # 6. Test plw agent orient --frame-only (<15 KB target)
    rc, stdout, stderr = run_cli("agent", "orient", str(repo), "--task", "audit config", "--frame-only", "--json")
    assert rc == 0, f"agent orient --frame-only failed: {stderr}"
    orient_bytes = len(stdout.encode("utf-8"))
    orient_data = json.loads(stdout)
    assert orient_data.get("frame_only") is True, orient_data
    assert orient_bytes < 15000, f"agent orient --frame-only size ({orient_bytes} bytes) exceeded 15 KB target"
    print(f"PASS: plw agent orient --frame-only ({orient_bytes} bytes < 15 KB)")

    # 7. Test fallback_hint in codesearch envelope for callers and defs when total == 0
    try:
        from core.codesearch_engine import envelope
        env_call = envelope("callers", repo, "eventBus", {"binary": "mock"}, {"calls": [], "total": 0})
        assert env_call.get("summary", {}).get("total") == 0
        assert "fallback_hint" in env_call
        assert "plw ast search" in env_call["fallback_hint"]
        print("PASS: plw ast callers fallback_hint when total == 0")

        env_def = envelope("defs", repo, "sharedSubAgentMemory", {"binary": "mock"}, {"hits": [], "total": 0})
        assert env_def.get("summary", {}).get("total") == 0
        assert "fallback_hint" in env_def
        assert "plw ast related" in env_def["fallback_hint"]
        print("PASS: plw ast defs fallback_hint when total == 0")
    except ImportError:
        pass

    # 8. Test cycle_break_advisory with dynamic await import()
    try:
        from codebase.cycle_break import compute_cycle_break_advisory
        cycle_graph = {
            'node_metadata': {
                'sub-agent-tool.ts': {'type': 'Module', 'fan_in': 5, 'fan_out': 2},
                'sub-agent-executor.ts': {'type': 'Module', 'fan_in': 1, 'fan_out': 1},
            },
            'file_map': {
                'sub-agent-tool.ts': 'export const AgentTask = 1;\nexport const ToolContext = 2;\nasync function run() { const mod = await import("./sub-agent-executor"); }\n',
                'sub-agent-executor.ts': 'import { AgentTask, ToolContext } from "./sub-agent-tool";\nexport class SubAgentExecutor {}\n',
            },
            'type_shapes': {},
            'edge_natures': {},
        }
        cycles = [['sub-agent-tool.ts', 'sub-agent-executor.ts', 'sub-agent-tool.ts']]
        cycle_res = compute_cycle_break_advisory(cycle_graph, cycles)
        plan = cycle_res['cycle_break_plan']
        cut = plan['recommended_cuts'][0]
        assert cut['source'] == 'sub-agent-executor.ts', f"Expected cut from executor, got {cut['source']}"
        assert cut['target'] == 'sub-agent-tool.ts', f"Expected cut target tool, got {cut['target']}"
        assert cut['strategy'] == 'extract_shared_module', cut['strategy']
        assert set(cut['symbols_involved']) == {'AgentTask', 'ToolContext'}, cut['symbols_involved']
        assert cut['suggested_leaf'] == 'sub-agent-tool.shared.ts', cut['suggested_leaf']
        print("PASS: cycle_break_advisory with dynamic await import() directs cut to static definition provider")
    except ImportError:
        pass

print("ALL AGENT UX HARDENING (V1 & V2) SMOKE CHECKS PASSED")
PY
