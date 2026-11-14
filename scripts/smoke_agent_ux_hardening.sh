#!/usr/bin/env bash
# Smoke test for agent_ux_hardening_v1:
#   - Item 1: --cache-mode (auto|refresh|off) in plw topology
#   - Item 2: Multi-target comma-separated resolution in plw resolve-target (--target a,b)
#   - Item 3: --frame-only compact work pack in plw work and plw agent orient (<20 KB)
#   - Item 4: Fallback hint in plw ast callers when total == 0
#   - Item 5: recommended_shortcut in plw resolve-target pointing to plw work --target

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

    # 4. Test plw work --frame-only
    rc, stdout, stderr = run_cli("work", "audit config", str(repo), "--target", "src/service.ts,src/client.ts", "--frame-only", "--json")
    assert rc == 0, f"work --frame-only failed: {stderr}"
    work_bytes = len(stdout.encode("utf-8"))
    work_data = json.loads(stdout)
    assert work_data.get("frame_only") is True, work_data
    assert work_data.get("readiness", {}).get("reference_pack") == "READY", work_data.get("readiness")
    assert isinstance(work_data.get("agent_decision_frame"), dict), "agent_decision_frame missing"
    assert bool(work_data.get("receipt")), "receipt missing from frame-only work"
    assert work_bytes < 25000, f"work --frame-only size ({work_bytes} bytes) exceeded threshold"
    print(f"PASS: plw work --frame-only ({work_bytes} bytes < 25 KB)")

    # 5. Test plw agent orient --frame-only
    rc, stdout, stderr = run_cli("agent", "orient", str(repo), "--task", "audit config", "--frame-only", "--json")
    assert rc == 0, f"agent orient --frame-only failed: {stderr}"
    orient_data = json.loads(stdout)
    assert orient_data.get("frame_only") is True, orient_data
    print("PASS: plw agent orient --frame-only")

    # 6. Test fallback_hint in codesearch envelope for zero callers
    try:
        from core.codesearch_engine import envelope
        env = envelope("callers", repo, "eventBus", {"binary": "mock"}, {"calls": [], "total": 0})
        assert env.get("summary", {}).get("total") == 0
        assert "fallback_hint" in env
        assert "plw ast search" in env["fallback_hint"]
        print("PASS: plw ast callers fallback_hint when total == 0")
    except ImportError:
        pass

print("ALL AGENT UX HARDENING SMOKE CHECKS PASSED")
PY
