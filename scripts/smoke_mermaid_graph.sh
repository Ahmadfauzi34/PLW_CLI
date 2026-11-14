#!/usr/bin/env bash
# Smoke test for mermaid_graph_v1:
#   - capability contract & alias registration (mermaid, diagram, mermaid-graph)
#   - multi-depth Mermaid generation (--depth 1, 2, 3, 4)
#   - path-trie pass-through compaction on nested Kotlin + TypeScript packages
#   - progressive next_zoom_hints for humans & coding agents
#   - --direction, --out (.mmd and .md), and fail-closed validation

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


with tempfile.TemporaryDirectory(prefix="plw-mermaid-smoke-") as tmp:
    repo = Path(tmp) / "repo"
    # Nested Android Kotlin structure with pass-through dirs + same-package + cycle
    kt_base = repo / "android" / "app" / "src" / "main" / "kotlin" / "io" / "jeph" / "pipkin"
    (kt_base / "ui").mkdir(parents=True)
    (kt_base / "agent").mkdir(parents=True)
    (repo / "src" / "core").mkdir(parents=True)

    (repo / "android" / "app" / "src" / "main" / "AndroidManifest.xml").write_text(
        '<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="io.jeph.pipkin">\n'
        '  <application><activity android:name=".MainActivity" /><service android:name=".agent.AgentService" /></application>\n'
        '</manifest>\n',
        encoding="utf-8",
    )
    (kt_base / "MainActivity.kt").write_text(
        "package io.jeph.pipkin\n"
        "import io.jeph.pipkin.agent.AgentService\n"
        "import io.jeph.pipkin.ui.ChatScreen\n"
        "class MainActivity {\n"
        "    fun start() { AgentService(); ChatScreen() }\n"
        "}\n",
        encoding="utf-8",
    )
    (kt_base / "AppGraph.kt").write_text(
        "package io.jeph.pipkin\n"
        "import io.jeph.pipkin.agent.AgentService\n"
        "class AppGraph {\n"
        "    fun provide(): AgentService = AgentService()\n"
        "}\n",
        encoding="utf-8",
    )
    (kt_base / "agent" / "AgentService.kt").write_text(
        "package io.jeph.pipkin.agent\n"
        "import io.jeph.pipkin.AppGraph\n"
        "class AgentService {\n"
        "    fun graph(): AppGraph = AppGraph()\n"
        "    fun helper(): KeepAlive = KeepAlive()\n"
        "}\n",
        encoding="utf-8",
    )
    (kt_base / "agent" / "KeepAlive.kt").write_text(
        "package io.jeph.pipkin.agent\n"
        "class KeepAlive {\n"
        "    fun ping(): Boolean = true\n"
        "}\n",
        encoding="utf-8",
    )
    (kt_base / "ui" / "ChatScreen.kt").write_text(
        "package io.jeph.pipkin.ui\n"
        "import io.jeph.pipkin.agent.AgentService\n"
        "class ChatScreen {\n"
        "    fun bind(svc: AgentService) {}\n"
        "}\n",
        encoding="utf-8",
    )
    (repo / "src" / "core" / "types.ts").write_text(
        "export interface HubConfig { id: string; }\n",
        encoding="utf-8",
    )
    (repo / "src" / "core" / "index.ts").write_text(
        "import type { HubConfig } from './types';\n"
        "export function boot(cfg: HubConfig): string { return cfg.id; }\n",
        encoding="utf-8",
    )

    # 1. Capability contract & alias checks
    rc, out, err = run_cli("capability", "describe", "mermaid", "--json")
    assert rc == 0, (rc, out, err)
    cap = json.loads(out)
    assert cap.get("known") is True, cap
    assert cap.get("command") == "mermaid", cap
    assert cap.get("read_before_use") == "skills/target-codebase-topology-workflow.md", cap

    rc_alias, out_alias, _ = run_cli("capability", "describe", "diagram", "--json")
    assert rc_alias == 3, rc_alias
    alias_data = json.loads(out_alias)
    assert alias_data.get("alias_of") == "mermaid", alias_data

    rc_skill, out_skill, _ = run_cli("skill", "show", "mermaid", "--json")
    assert rc_skill == 0, rc_skill
    skill_data = json.loads(out_skill)
    assert skill_data.get("known") is True and "mermaid" in (skill_data.get("commands") or []), skill_data

    # 2. Depth 1: Subsystem Overview
    rc1, out1, err1 = run_cli("mermaid", str(repo), "--depth", "1", "--json")
    assert rc1 == 0, (rc1, out1, err1)
    d1 = json.loads(out1)
    assert d1["decision"] == "MERMAID_GRAPH_READY", d1
    assert d1["depth"] == 1 and d1["depth_name"] == "subsystem_overview", d1
    assert d1["mermaid_source"].startswith("flowchart LR"), d1["mermaid_source"]
    assert len(d1["nodes"]) >= 2, d1["nodes"]
    assert len(d1["next_zoom_hints"]) >= 2, d1["next_zoom_hints"]
    assert d1["authority"]["read_only"] is True

    # 3. Depth 2: Module Dependency Flow with path-trie compaction
    rc2, out2, err2 = run_cli("mermaid", str(repo), "--depth", "2", "--focus", "android/app", "--json")
    assert rc2 == 0, (rc2, out2, err2)
    d2 = json.loads(out2)
    assert d2["decision"] == "MERMAID_GRAPH_READY", d2
    assert d2["depth"] == 2 and d2["depth_name"] == "module_dependency_flow", d2
    mods = [n["module"] for n in d2["nodes"]]
    assert any(m.endswith("pipkin/agent") for m in mods), mods
    assert any(m.endswith("pipkin/ui") for m in mods), mods

    # 4. Depth 3: File Architecture Graph (Entrypoints, Hubs, Cycles, Typed Edges)
    rc3, out3, err3 = run_cli("mermaid", str(repo), "--depth", "3", "--direction", "TD", "--json")
    assert rc3 == 0, (rc3, out3, err3)
    d3 = json.loads(out3)
    assert d3["decision"] == "MERMAID_GRAPH_READY", d3
    assert d3["direction"] == "TD", d3
    assert d3["mermaid_source"].startswith("flowchart TD"), d3["mermaid_source"]
    assert "/n" not in d3["mermaid_source"], d3["mermaid_source"]
    natures = {e["nature"] for e in d3["edges"]}
    assert "runtime" in natures, natures
    assert "same_package" in natures or "type_only" in natures, natures

    # 5. Depth 4: File Micro-Neighborhood (Outline + Blast Radius) + --out
    out_md = Path(tmp) / "agent_service_graph.md"
    rc4, out4, err4 = run_cli(
        "mermaid", str(repo), "--depth", "4", "--focus", "AgentService.kt",
        "--out", str(out_md), "--json",
    )
    assert rc4 == 0, (rc4, out4, err4)
    d4 = json.loads(out4)
    assert d4["decision"] == "MERMAID_GRAPH_READY", d4
    assert d4["focus"].endswith("AgentService.kt"), d4
    assert d4["summary"]["declarations_count"] >= 1, d4["summary"]
    assert d4["summary"]["direct_upstream_count"] >= 1, d4["summary"]
    assert d4["summary"]["direct_downstream_count"] >= 1, d4["summary"]
    assert out_md.is_file() and "```mermaid" in out_md.read_text(encoding="utf-8")

    # 6. Fail-closed validation checks
    rc_bad_depth, out_bad_depth, _ = run_cli("mermaid", str(repo), "--depth", "9", "--json")
    assert rc_bad_depth == 3, rc_bad_depth
    bad_depth_data = json.loads(out_bad_depth)
    assert bad_depth_data.get("error_code") == "invalid_mermaid_depth", bad_depth_data

    rc_bad_focus, out_bad_focus, _ = run_cli("mermaid", str(repo), "--depth", "2", "--focus", "no_such_module_xyz", "--json")
    assert rc_bad_focus == 3, rc_bad_focus
    assert json.loads(out_bad_focus)["decision"] == "FOCUS_NOT_FOUND", out_bad_focus

print("SMOKE_MERMAID_GRAPH: PASS")
PY
