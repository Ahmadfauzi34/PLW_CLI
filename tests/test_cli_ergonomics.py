"""CLI flags, compact output, help/submodes, uniform --json normalization, and overlay sync invariants."""
from __future__ import annotations

import io
import json
import os
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock
from core.shared_graph import _LANGUAGE_IMPORT_RESOLVERS, build_shared_graph
import hott_kernel

try:
    from tests._baseline_fixture import ArchitectureBaselineCase
except ImportError:
    from _baseline_fixture import ArchitectureBaselineCase


class CliErgonomicsTests(ArchitectureBaselineCase):
    def test_calibration_ux_v3_fixes(self) -> None:
        from codebase.static_ui_reference import QUERY_STOPWORDS, _score
        from codebase.cycle_break import _inspect_edge_import
        import plw_cli

        # 1. Stopwords & generic dev terms do not trigger false template matches
        for sw in ("this", "between", "audit", "architecture", "angular"):
            self.assertIn(sw, QUERY_STOPWORDS)
        score, matched, _ = _score(
            "Audit architecture, cycles, and Angular UI in this repo",
            "input",
            "",
            {"placeholder": "Filter between this repo"},
        )
        self.assertEqual(score, 0)
        self.assertEqual(matched, [])

        # 2. Multi-line named imports populate symbols_involved
        src_path = str((self.proj / "src" / "app" / "caller.ts").resolve())
        tgt_path = str((self.proj / "src" / "app" / "target.ts").resolve())
        info = _inspect_edge_import(
            src_path,
            tgt_path,
            {
                src_path: (
                    "import {\n"
                    "  alpha,\n"
                    "  beta as b,\n"
                    "  type Gamma,\n"
                    "} from './target';\n"
                ),
                tgt_path: "export const alpha = 1; export const beta = 2; export type Gamma = string;\n",
            },
            {},
        )
        self.assertTrue(info["has_import"])
        self.assertEqual(info["symbols"], ["alpha", "beta", "Gamma"])

        # 3. _top_check_findings compacts oversized cycle_basis (> 5)
        findings, _ = hott_kernel._top_check_findings(
            {
                "results": {
                    "hott.manifold": {
                        "findings": [
                            {
                                "type": "topological_cycle",
                                "severity": "medium",
                                "cycle_basis": [{"id": i} for i in range(12)],
                            }
                        ]
                    }
                }
            }
        )
        self.assertEqual(len(findings), 1)
        self.assertEqual(len(findings[0]["cycle_basis"]), 5)
        self.assertEqual(findings[0]["cycle_basis_omitted_count"], 7)
        self.assertTrue(findings[0]["cycle_basis_truncated"])

        # 4. cycle-break hyphen alias & root-first impact arg order
        def _run_cli(*args: str):
            buf = io.StringIO()
            exit_code = 0
            with mock.patch.object(sys, "argv", ["hott_kernel.py", *args]), redirect_stdout(buf):
                try:
                    hott_kernel.main()
                except SystemExit as exc:
                    exit_code = int(exc.code) if isinstance(exc.code, int) else 1
            return exit_code, json.loads(buf.getvalue())

        cb_code, cb_payload = _run_cli("cycle-break", str(self.proj))
        self.assertEqual(cb_code, 0)
        self.assertEqual(cb_payload["mode"], "cycle_break")

        imp_code, imp_payload = _run_cli("impact", str(self.proj), "src/app/utils.py")
        self.assertEqual(imp_code, 0)
        self.assertEqual(imp_payload["mode"], "impact")
        self.assertTrue(imp_payload["exists"])

        # 5. _load_receipt_arg accepts an existing file path directly
        rcpt_file = self._tmp / "sample_receipt.json"
        rcpt_file.write_text(json.dumps({"schema_version": "plw-task-receipt-v1"}), encoding="utf-8")
        loaded = plw_cli._load_receipt_arg(str(rcpt_file))
        self.assertEqual(loaded["schema_version"], "plw-task-receipt-v1")

    def test_calibration_ux_v4_compact_topology_chain_and_cycle_target(self) -> None:
        topo_compact = hott_kernel.kernel_topology(str(self.proj), include_engine=False)
        self.assertEqual(topo_compact["mode"], "topology")
        self.assertNotIn("engine", topo_compact["target_orientation"])

        chain_frame = hott_kernel.kernel_task_chain(
            str(self.proj),
            "Audit utils cycle",
            target_files=["src/app/utils.py"],
            frame_only=True,
        )
        self.assertTrue(chain_frame["frame_only"])
        self.assertIn("receipt", chain_frame)
        self.assertNotIn("receipt_chain", chain_frame)
        self.assertLess(len(json.dumps(chain_frame)), 15 * 1024)

        cb = hott_kernel.kernel_cycle_break(str(self.proj), target_file="src/app/utils.py")
        self.assertEqual(cb["target"], "src/app/utils.py")

    def test_calibration_ux_v5_root_relative_target_and_cli_ergonomics(self) -> None:
        from core.context_optimizer import _resolve_target
        from codebase.topology_analyzers import _resolve_query_target
        from codebase.cycle_break import _inspect_edge_import
        import plw_cli

        # 1. Exact root-relative target beats nested suffix duplicates
        root_str = str(self.proj.resolve()).replace("\\", "/")
        direct = f"{root_str}/src/app/dup.ts"
        nested = f"{root_str}/sub/src/app/dup.ts"
        vertices = {direct, nested}
        res_ctx, amb_ctx = _resolve_target("src/app/dup.ts", vertices, scan_root=root_str)
        self.assertEqual(res_ctx, direct)
        self.assertEqual(amb_ctx, [])

        res_q, amb_q, res_kind = _resolve_query_target("src/app/dup.ts", vertices, scan_root=root_str)
        self.assertEqual(res_q, direct)
        self.assertEqual(amb_q, [])
        self.assertEqual(res_kind, "root_relative_exact")

        # 2. frame_only=True stays compact (<15KB) even when kernel_work blocks on context error
        blocked = hott_kernel.kernel_work(
            str(self.proj),
            "Inspect missing target",
            target_files=["nonexistent/missing.ts"],
            frame_only=True,
        )
        self.assertTrue(blocked["frame_only"])
        self.assertEqual(blocked["work_status"], "blocked")
        self.assertEqual(blocked["error_code"], "context_target_not_found")
        self.assertNotIn("target_orientation", blocked)
        self.assertLess(len(json.dumps(blocked)), 15 * 1024)

        # 3. Directory barrel import ('./diagnostics' -> 'diagnostics/index.ts') matches and extracts symbols
        caller = str((self.proj / "src" / "app" / "caller.ts").resolve())
        idx_target = str((self.proj / "src" / "app" / "diagnostics" / "index.ts").resolve())
        info = _inspect_edge_import(
            caller,
            idx_target,
            {
                caller: "import { resolveProfile } from './diagnostics';\n",
                idx_target: "export * from './profiles';\n",
            },
            {},
        )
        self.assertTrue(info["has_import"])
        self.assertEqual(info["symbols"], ["resolveProfile"])

        # 4. Root-first normalization for brief/outline & --output/--detail on plw context
        def _run_kernel(*args: str):
            buf = io.StringIO()
            exit_code = 0
            with mock.patch.object(sys, "argv", ["hott_kernel.py", *args]), redirect_stdout(buf):
                try:
                    hott_kernel.main()
                except SystemExit as exc:
                    exit_code = int(exc.code) if isinstance(exc.code, int) else 1
            return exit_code, json.loads(buf.getvalue())

        b_code, b_payload = _run_kernel("brief", str(self.proj), "src/app/utils.py")
        self.assertEqual(b_code, 0)
        self.assertTrue(b_payload["exists"])

        o_code, o_payload = _run_kernel("outline", str(self.proj), "src/app/utils.py")
        self.assertEqual(o_code, 0)
        self.assertTrue(o_payload["exists"])

        buf = io.StringIO()
        with redirect_stdout(buf):
            ctx_rc = plw_cli.cmd_context(
                [str(self.proj), "Audit utils", "--detail", "outline", "--output", "summary", "--json"]
            )
        self.assertIn(ctx_rc, (0, 3))
        ctx_payload = json.loads(buf.getvalue())
        self.assertEqual(ctx_payload["output_mode"], "summary")
        self.assertEqual(ctx_payload["query"], "Audit utils")

    def test_calibration_ux_v6_cycle_break_help_and_check_compact(self) -> None:
        import plw_cli

        # 1. cycle-break --help / -h returns usage without running kernel_cycle_break
        with mock.patch.object(
            hott_kernel,
            "kernel_cycle_break",
            side_effect=AssertionError("cycle-break --help must not execute analysis"),
        ):
            for argv in (
                ["hott_kernel.py", "cycle-break", "--help"],
                ["hott_kernel.py", "cycle-break", "-h"],
                ["hott_kernel.py", "cycle_break", "--help"],
            ):
                buf = io.StringIO()
                with mock.patch.object(sys, "argv", argv), redirect_stdout(buf):
                    hott_kernel.main()
                payload = json.loads(buf.getvalue())
                self.assertEqual(payload["mode"], "cycle_break")
                self.assertIn("cycle-break", payload["usage"])

            buf_cli = io.StringIO()
            with mock.patch.object(sys, "argv", ["plw", "cycle-break", "--help"]), redirect_stdout(buf_cli):
                plw_cli.main()
            cli_help_payload = json.loads(buf_cli.getvalue())
            self.assertEqual(cli_help_payload["mode"], "cycle_break")
            self.assertIn("cycle-break", cli_help_payload["usage"])

        buf_main = io.StringIO()
        with mock.patch.object(sys, "argv", ["hott_kernel.py", "--help"]), redirect_stdout(buf_main):
            hott_kernel.main()
        main_help = json.loads(buf_main.getvalue())
        self.assertIn("cycle_break", main_help["usage"])

        # 2. _top_check_findings caps cycle_basis at 2 when compact=True vs 5 by default
        synthetic_basis = [
            {
                "basis_index": idx + 1,
                "closed_path": [f"n{idx}.ts", f"m{idx}.ts", f"n{idx}.ts"],
                "closing_edge": [f"m{idx}.ts", f"n{idx}.ts"],
                "vertices": [f"n{idx}.ts", f"m{idx}.ts"],
                "witness_edges": [{"edge": [f"n{idx}.ts", f"m{idx}.ts"]}],
                "length": 2,
                "orientation": "directed",
            }
            for idx in range(6)
        ]
        many_findings = [
            {
                "type": "async_waterfall",
                "severity": "high",
                "file": f"src/f{i:02d}.ts",
                "observation": f"Synthetic high finding #{i} for envelope compaction test.",
            }
            for i in range(39)
        ]
        fake_analyzer_output = {
            "results": {
                "hott.manifold": {
                    "findings": [
                        {
                            "type": "independent_cycles",
                            "severity": "medium",
                            "cycle_basis": synthetic_basis,
                        }
                    ]
                },
                "perf.async": {"findings": many_findings},
            },
            "errors": {},
            "analyzers_run": ["hott.manifold", "perf.async"],
            "analyzers_failed": 0,
            "cache": {
                "mode": "auto",
                "status": "hit",
                "cache_path": "/tmp/fake_analyzer_cache.json",
                "source_cache_key": "abc123",
                "graph_content_signature": "sha256:deadbeef",
                "engine_signature": "sha256:cafebabe",
                "analyzers_requested": ["hott.manifold", "perf.async"],
                "analyzers_reused": ["hott.manifold", "perf.async"],
                "analyzers_executed": [],
            },
        }
        compact_top, _ = hott_kernel._top_check_findings(fake_analyzer_output, limit=10, compact=True)
        basis_item_compact = [f for f in compact_top if f["type"] == "independent_cycles"][0]
        self.assertEqual(len(basis_item_compact["cycle_basis"]), 2)
        self.assertEqual(basis_item_compact["cycle_basis_omitted_count"], 4)
        self.assertTrue(basis_item_compact["cycle_basis_truncated"])

        default_top, _ = hott_kernel._top_check_findings(fake_analyzer_output, limit=100, compact=False)
        basis_item_default = [f for f in default_top if f["type"] == "independent_cycles"][0]
        self.assertEqual(len(basis_item_default["cycle_basis"]), 5)
        self.assertEqual(basis_item_default["cycle_basis_omitted_count"], 1)

        # 3. plw check --compact --json caps payload <15KB while preserving cache_path & limit precedence
        with mock.patch.object(hott_kernel, "_run_kernel_analyzers", return_value=fake_analyzer_output):
            buf_compact = io.StringIO()
            with redirect_stdout(buf_compact):
                rc_compact = plw_cli.cmd_check([str(self.proj), "--compact", "--json"])
            self.assertEqual(rc_compact, 0)
            raw_compact = buf_compact.getvalue()
            compact_payload = json.loads(raw_compact)
            self.assertTrue(compact_payload["compact"])
            self.assertEqual(len(compact_payload["findings"]), 10)
            self.assertTrue(compact_payload["findings_truncated"])
            self.assertNotIn("stat_trust_boundary", compact_payload["graph_cache"])
            self.assertNotIn("analyzers_requested", compact_payload["analyzer_cache"])
            self.assertEqual(compact_payload["analyzer_cache"]["cache_path"], "/tmp/fake_analyzer_cache.json")
            self.assertTrue(compact_payload["detail_contract"]["compact"])
            self.assertLess(len(raw_compact.encode("utf-8")), 15 * 1024)

            buf_limit = io.StringIO()
            with redirect_stdout(buf_limit):
                rc_limit = plw_cli.cmd_check([str(self.proj), "--compact", "--limit", "15", "--json"])
            self.assertEqual(rc_limit, 0)
            limit_payload = json.loads(buf_limit.getvalue())
            self.assertTrue(limit_payload["compact"])
            self.assertEqual(len(limit_payload["findings"]), 15)

            buf_all = io.StringIO()
            with redirect_stdout(buf_all):
                rc_all = plw_cli.cmd_check([str(self.proj), "--compact", "--all", "--json"])
            self.assertEqual(rc_all, 0)
            all_payload = json.loads(buf_all.getvalue())
            self.assertTrue(all_payload["compact"])
            self.assertEqual(len(all_payload["findings"]), 40)
            self.assertFalse(all_payload["findings_truncated"])

            buf_normal = io.StringIO()
            with redirect_stdout(buf_normal):
                rc_normal = plw_cli.cmd_check([str(self.proj), "--json"])
            self.assertEqual(rc_normal, 0)
            normal_payload = json.loads(buf_normal.getvalue())
            self.assertNotIn("compact", normal_payload)
            self.assertEqual(len(normal_payload["findings"]), 40)
            self.assertIn("stat_trust_boundary", normal_payload["graph_cache"])
            self.assertIn("analyzers_requested", normal_payload["analyzer_cache"])

        # 4. plw --help advertises --compact on check
        buf_cli_help = io.StringIO()
        with mock.patch.object(sys, "argv", ["plw", "--help"]), redirect_stdout(buf_cli_help):
            with self.assertRaises(SystemExit) as exit_ctx:
                plw_cli.main()
        self.assertEqual(exit_ctx.exception.code, 0)
        self.assertIn(
            "plw check [root] [--analyzers a,b] [--limit N] [--all] [--compact] [--json]",
            buf_cli_help.getvalue(),
        )

    def test_calibration_ux_v8_cli_submodes_sheaf_and_lru_cache(self) -> None:
        import plw_cli
        from codebase.hott_analyzers import analyze_sheaf
        from codebase.performance_analyzers import (
            analyze_async_waterfall,
            analyze_cache,
            analyze_gc_pressure,
        )

        # 1. plw mermaid --target alias for --focus
        buf_mmd = io.StringIO()
        with redirect_stdout(buf_mmd):
            rc_mmd = plw_cli.cmd_mermaid([str(self.proj), "--target", "src/app/utils.py", "--json"])
        self.assertEqual(rc_mmd, 0)
        mmd_payload = json.loads(buf_mmd.getvalue())
        self.assertEqual(mmd_payload["focus"], "src/app/utils.py")

        # 2. plw candidate list <root> --task ... and plw ledger status <root> --json
        buf_cand = io.StringIO()
        with redirect_stdout(buf_cand):
            rc_cand = plw_cli.cmd_candidate(
                ["list", str(self.proj), "--task", "Inspect utils", "--json"]
            )
        self.assertEqual(rc_cand, 0)
        cand_payload = json.loads(buf_cand.getvalue())
        self.assertIn("candidate_selection", cand_payload)

        buf_empty_cand = io.StringIO()
        with redirect_stdout(buf_empty_cand):
            rc_empty_cand = plw_cli.cmd_candidate(["select", "", str(self.proj), "--json"])
        self.assertEqual(rc_empty_cand, 0)
        self.assertEqual(json.loads(buf_empty_cand.getvalue())["candidate_selection"]["resolution"], "UNRESOLVED")

        buf_led = io.StringIO()
        with redirect_stdout(buf_led):
            rc_led = plw_cli.cmd_ledger(["status", str(self.proj), "--json"])
        self.assertEqual(rc_led, 0)
        led_payload = json.loads(buf_led.getvalue())
        self.assertIn("outcome_count", led_payload)

        # 3. hott_kernel memory status & memory list <root>
        buf_mem = io.StringIO()
        with mock.patch.object(sys, "argv", ["hott_kernel.py", "memory", "status"]), redirect_stdout(buf_mem):
            hott_kernel.main()
        mem_payload = json.loads(buf_mem.getvalue())
        self.assertIn("total_memories", mem_payload)

        buf_mem_list = io.StringIO()
        with mock.patch.object(sys, "argv", ["hott_kernel.py", "memory", "list", str(self.proj)]), redirect_stdout(buf_mem_list):
            hott_kernel.main()
        mem_list_payload = json.loads(buf_mem_list.getvalue())
        self.assertIsNone(mem_list_payload.get("query"))

        # 4. analyze_sheaf skips test files, barrel self-reexports, and nested parent/child boundaries
        sheaf_res = analyze_sheaf(
            {
                "boundaries": {
                    "/repo/pkg": {"barrel": "/repo/pkg/index.ts"},
                    "/repo/pkg/sub": {"barrel": "/repo/pkg/sub/index.ts"},
                    "/repo/other": {"barrel": "/repo/other/index.ts"},
                },
                "file_to_boundary": {
                    "/repo/pkg/index.ts": "/repo/pkg",
                    "/repo/pkg/core.ts": "/repo/pkg",
                    "/repo/pkg/sub/helper.ts": "/repo/pkg/sub",
                    "/repo/other/feature.spec.ts": "/repo/other",
                    "/repo/other/prod.ts": "/repo/other",
                },
                "resolved_imports": {
                    "/repo/pkg/index.ts": ["/repo/pkg/sub/helper.ts"],
                    "/repo/pkg/sub/helper.ts": ["/repo/pkg/core.ts"],
                    "/repo/other/feature.spec.ts": ["/repo/pkg/core.ts"],
                    "/repo/other/prod.ts": ["/repo/pkg/core.ts"],
                },
                "edge_natures": {},
                "node_metadata": {
                    "/repo/other/feature.spec.ts": {"is_test": True},
                },
            }
        )
        bv = [f for f in sheaf_res["findings"] if f["type"] == "boundary_violation"]
        cb = [f for f in sheaf_res["findings"] if f["type"] == "circular_boundary"]
        self.assertEqual(len(bv), 1)
        self.assertEqual(bv[0]["file"], "/repo/other/prod.ts")
        self.assertEqual(cb, [])

        # 5. perf.cache recognizes LRUCache & forEach(.delete), and perf.gc/perf.async skip test files
        lru_worker_ts = (
            "class LRUCache<K, V> { set(k: K, v: V) {} }\n"
            "const resultCache = new LRUCache<string, any>(50);\n"
            "export function save(k: string, v: any) { resultCache.set(k, v); }\n"
        )
        foreach_evict_ts = (
            "export class Canvas {\n"
            "  private panelCache = new Map<number, string>();\n"
            "  sync(n: number) {\n"
            "    this.panelCache.set(0, \"a\");\n"
            "    this.panelCache.forEach((_, idx) => { if (idx >= n) this.panelCache.delete(idx); });\n"
            "  }\n"
            "}\n"
        )
        cache_res = analyze_cache(
            {
                "file_map": {
                    "src/worker.ts": lru_worker_ts,
                    "src/canvas.ts": foreach_evict_ts,
                    "src/cache.spec.ts": "const cCache = new Map(); cCache.set(\"a\", 1);\n",
                }
            }
        )
        unbounded = [f for f in cache_res["findings"] if f["type"] == "unbounded_cache"]
        self.assertEqual(unbounded, [])

        gc_res = analyze_gc_pressure(
            {
                "file_map": {
                    "src/test.spec.ts": "for (const x of items) { JSON.stringify(x); }\n",
                    "scripts/check.js": "for (const x of items) { JSON.stringify(x); }\n",
                }
            }
        )
        self.assertEqual(len(gc_res["findings"]), 1)
        self.assertEqual(gc_res["findings"][0]["file"], "scripts/check.js")
        self.assertEqual(gc_res["findings"][0]["severity"], "info")

        async_res = analyze_async_waterfall(
            {
                "file_map": {
                    "src/ops.spec.ts": (
                        "async function testIt() {\n"
                        "  const a = await stepOne();\n"
                        "  const b = await stepTwo();\n"
                        "  for (const x of items) { await run(x); }\n"
                        "}\n"
                    )
                }
            }
        )
        self.assertEqual(async_res["findings"], [])

    def test_calibration_ux_v10_cli_ergonomics_and_stream_state_analyzers(self) -> None:
        import plw_cli
        from codebase.performance_analyzers import (
            analyze_async_waterfall,
            analyze_gc_pressure,
        )

        # 1. plw context <root> single-positional & --query flag
        buf_ctx = io.StringIO()
        with redirect_stdout(buf_ctx):
            rc_ctx = plw_cli.cmd_context([str(self.proj), "--output", "summary", "--json"])
        self.assertIn(rc_ctx, (0, 3))
        ctx_payload = json.loads(buf_ctx.getvalue())
        self.assertEqual(os.path.abspath(ctx_payload["scan_root"]), os.path.abspath(str(self.proj)))

        # 2. plw semantic <root>, plw semantic <cmd>, and plw semantic route <root> <task>
        buf_sem1 = io.StringIO()
        with redirect_stdout(buf_sem1):
            self.assertEqual(plw_cli.cmd_semantic([str(self.proj), "--json"]), 0)
        self.assertEqual(json.loads(buf_sem1.getvalue())["command"], "check")

        buf_sem2 = io.StringIO()
        with redirect_stdout(buf_sem2):
            self.assertEqual(
                plw_cli.cmd_semantic(["route", str(self.proj), "Inspect utils", "--json"]),
                0,
            )
        self.assertTrue(json.loads(buf_sem2.getvalue())["route_found"])

        # 3. plw operation-plan <root> <task> & plw reconcile-outcomes <root>
        buf_op = io.StringIO()
        with redirect_stdout(buf_op):
            self.assertEqual(
                plw_cli.cmd_operation_plan([str(self.proj), "Select isOne in utils.py", "--json"]),
                0,
            )
        self.assertEqual(json.loads(buf_op.getvalue())["status"], "RESOLVED")

        buf_rec = io.StringIO()
        with redirect_stdout(buf_rec):
            self.assertEqual(plw_cli.cmd_reconcile_outcomes([str(self.proj), "--json"]), 0)
        self.assertIn("outcome_count", json.loads(buf_rec.getvalue()))

        # 4. plw fiber <root> --json & plw fiber list <root> --json
        for argv_f, expected_mode in (
            (["hott_kernel.py", "fiber", str(self.proj), "--json"], "fiber_status"),
            (["hott_kernel.py", "fiber", "list", str(self.proj), "--json"], "fiber_list_archives"),
            (["hott_kernel.py", "fiber", "section-status", str(self.proj), "--json"], "fiber_section_status"),
        ):
            buf_f = io.StringIO()
            with mock.patch.object(sys, "argv", argv_f), redirect_stdout(buf_f):
                hott_kernel.main()
            self.assertEqual(json.loads(buf_f.getvalue())["mode"], expected_mode)

        # 5. perf.async & perf.gc stream/state loop precision
        stream_and_turn_ts = (
            "async function consumeStream(reader: any, session: any) {\n"
            "  while (true) {\n"
            "    const { done, value } = await reader.read();\n"
            "    if (done) break;\n"
            "    const lines = decoder.decode(value).split(\"\\n\");\n"
            "    for (const line of lines) {\n"
            "      try { const chunk = JSON.parse(line); } catch {}\n"
            "    }\n"
            "  }\n"
            "  while (!isDone) {\n"
            "    if (this.toolLoopCount >= this.maxToolLoops) break;\n"
            "    const s = await session.sendMessageStream(nextInput);\n"
            "  }\n"
            "}\n"
        )
        async_res = analyze_async_waterfall({"file_map": {"src/stream.ts": stream_and_turn_ts}})
        high_async = [f for f in async_res["findings"] if f["severity"] == "high"]
        self.assertEqual(high_async, [])

        gc_res = analyze_gc_pressure({"file_map": {"src/stream.ts": stream_and_turn_ts}})
        high_gc = [f for f in gc_res["findings"] if f["severity"] == "high"]
        self.assertEqual(high_gc, [])

    def test_calibration_ux_v12_perf_opt_in_default_structural_check(self) -> None:
        import plw_cli
        from core.adaptive_work import _GOAL_ANALYZERS
        from core.analyzer_registry import get_available_analyzers, get_structural_analyzers

        all_analyzers = get_available_analyzers()
        structural_analyzers = get_structural_analyzers()
        self.assertTrue(any(a.startswith("perf.") for a in all_analyzers))
        self.assertFalse(any(a.startswith("perf.") for a in structural_analyzers))
        self.assertTrue(any(a.startswith("topo.") for a in structural_analyzers))
        self.assertTrue(any(a.startswith("hott.") for a in structural_analyzers))
        self.assertFalse(any(a.startswith("perf.") for a in _GOAL_ANALYZERS["change"]))
        self.assertTrue(any(a.startswith("perf.") for a in _GOAL_ANALYZERS["performance"]))

        ts_repo = self._tmp / "perf_opt_in_ts"
        (ts_repo / "src").mkdir(parents=True)
        (ts_repo / "src" / "store.ts").write_text(
            "export const tokenCache = new Map<string, string>();\n"
            "export function put(k: string, v: string) { tokenCache.set(k, v); }\n"
            "export function run(x: string) { return eval(x); }\n",
            encoding="utf-8",
        )

        # Default `plw check` runs structural analyzers only (no perf.* heuristics)
        buf_def = io.StringIO()
        with redirect_stdout(buf_def):
            rc_def = plw_cli.cmd_check([str(ts_repo), "--cache-mode", "off", "--json"])
        self.assertEqual(rc_def, 0)
        def_payload = json.loads(buf_def.getvalue())
        self.assertFalse(
            any(str(f.get("analyzer", "")).startswith("perf.") for f in def_payload.get("findings", []))
        )

        # Opt-in via `--include-perf` runs perf.* heuristics
        buf_perf = io.StringIO()
        with redirect_stdout(buf_perf):
            rc_perf = plw_cli.cmd_check([str(ts_repo), "--include-perf", "--cache-mode", "off", "--json"])
        self.assertEqual(rc_perf, 0)
        perf_payload = json.loads(buf_perf.getvalue())
        perf_analyzers_found = {
            f.get("analyzer") for f in perf_payload.get("findings", []) if str(f.get("analyzer", "")).startswith("perf.")
        }
        self.assertIn("perf.cache", perf_analyzers_found)
        self.assertIn("perf.deopt", perf_analyzers_found)

        # Explicit `--analyzers perf.cache` works without --include-perf
        buf_exp = io.StringIO()
        with redirect_stdout(buf_exp):
            rc_exp = plw_cli.cmd_check([str(ts_repo), "--analyzers", "perf.cache", "--cache-mode", "off", "--json"])
        self.assertEqual(rc_exp, 0)
        exp_payload = json.loads(buf_exp.getvalue())
        self.assertEqual(
            {f.get("analyzer") for f in exp_payload.get("findings", [])},
            {"perf.cache"},
        )

        # `hott_kernel analyzers` advertises default_check_analyzers vs opt_in_analyzers
        buf_list = io.StringIO()
        with mock.patch.object(sys, "argv", ["hott_kernel.py", "analyzers"]), redirect_stdout(buf_list):
            hott_kernel.main()
        list_payload = json.loads(buf_list.getvalue())
        self.assertEqual(list_payload["available_analyzers"], all_analyzers)
        self.assertEqual(list_payload["default_check_analyzers"], structural_analyzers)
        self.assertEqual(
            set(list_payload["opt_in_analyzers"]),
            {"perf.async", "perf.deopt", "perf.gc", "perf.cache"},
        )

    def test_calibration_ux_v13_check_human_output_and_identifier_target_resolution(self) -> None:
        import plw_cli
        from core.target_resolution import build_target_resolution
        from core.shared_graph import build_shared_graph

        # 1. Human `plw check` output renders unified_summary.total_files + observation/cycle (never `?` or empty `()`)
        buf_human = io.StringIO()
        sample_result = {
            "unified_summary": {
                "total_files": 42,
                "total_findings": 2,
                "analyzers_failed": 0,
            },
            "findings": [
                {
                    "analyzer": "hott.sheaf",
                    "severity": "high",
                    "type": "circular_boundary",
                    "observation": "Circular dependency across 2 boundaries: src/a -> src/b -> src/a",
                    "cycle": ["src/a", "src/b", "src/a"],
                },
                {
                    "analyzer": "hott.sheaf",
                    "severity": "high",
                    "type": "boundary_violation",
                    "observation": "Import bypasses barrel of 'src/b'.",
                    "file": "src/a/caller.ts",
                },
            ],
        }
        with redirect_stdout(buf_human):
            plw_cli._format_check_output(sample_result, as_json=False)
        human_out = buf_human.getvalue()
        self.assertIn("Total Files: 42 | Findings: 2", human_out)
        self.assertIn("Circular dependency across 2 boundaries", human_out)
        self.assertIn("Import bypasses barrel of 'src/b'. (src/a/caller.ts)", human_out)
        self.assertNotIn("Total Files: ?", human_out)
        self.assertNotIn(":  ()", human_out)

        # 2. target_resolution matches PascalCase/camelCase identifiers to kebab/snake/dotted stems across TS/Rust/Kotlin
        poly_repo = self.proj.parent / "v13_poly_target"
        (poly_repo / "src" / "shared").mkdir(parents=True, exist_ok=True)
        (poly_repo / "rust" / "src").mkdir(parents=True, exist_ok=True)
        (poly_repo / "kotlin" / "src" / "main" / "kotlin").mkdir(parents=True, exist_ok=True)
        (poly_repo / "src" / "shared" / "custom-canvas.component.ts").write_text(
            "export class CanvasWidget { render() { return 1; } }\n", encoding="utf-8"
        )
        (poly_repo / "src" / "app.ts").write_text(
            "import { CanvasWidget } from './shared/custom-canvas.component';\n"
            "export const CustomCanvasComponent = CanvasWidget;\n",
            encoding="utf-8",
        )
        (poly_repo / "rust" / "src" / "user_profile_service.rs").write_text(
            "pub struct AccountManager;\n", encoding="utf-8"
        )
        (poly_repo / "kotlin" / "src" / "main" / "kotlin" / "user_profile_screen.kt").write_text(
            "fun renderProfile() {}\n", encoding="utf-8"
        )
        # Preserve generic lowercase falsifier: single file with lowercase words only stays AMBIGUOUS
        (poly_repo / "src" / "plain_holder.ts").write_text(
            "export const value = 1;\n", encoding="utf-8"
        )
        g = build_shared_graph(str(poly_repo))

        res_ts = build_target_resolution(g, "CustomCanvasComponent")
        self.assertEqual(res_ts["resolution"]["state"], "RESOLVED")
        self.assertEqual(res_ts["resolution"]["selected_targets"], ["src/shared/custom-canvas.component.ts"])

        res_rs = build_target_resolution(g, "UserProfileService")
        self.assertEqual(res_rs["resolution"]["state"], "RESOLVED")
        self.assertEqual(res_rs["resolution"]["selected_targets"], ["rust/src/user_profile_service.rs"])

        res_kt = build_target_resolution(g, "UserProfileScreen")
        self.assertEqual(res_kt["resolution"]["state"], "RESOLVED")
        self.assertEqual(res_kt["resolution"]["selected_targets"], ["kotlin/src/main/kotlin/user_profile_screen.kt"])

        res_lower_falsifier = build_target_resolution(g, "Find value export")
        self.assertEqual(res_lower_falsifier["resolution"]["state"], "AMBIGUOUS")

        # 3. _engine_answer_summary ranks exact symbol matches ahead of partial substring matches
        #    and returns {} when unresolved with zero term hits
        orient_mock = {
            "engine": {
                "symbol_nodes": [
                    {"symbol": "PatcherTracePoint", "file": "a.ts", "line": 1, "kind": "body", "authority": "definition"},
                    {"symbol": "buildChatHistory", "file": "b.ts", "line": 4, "kind": "sig", "authority": "definition"},
                ],
                "symbol_edges": [],
            }
        }
        ranked = hott_kernel._engine_answer_summary(
            orient_mock,
            terms=["trace", "buildchathistory", "build", "chat", "history"],
            target_resolved=False,
        )
        self.assertEqual(ranked["symbols"][0]["symbol"], "buildChatHistory")
        no_hit = hott_kernel._engine_answer_summary(
            orient_mock,
            terms=["frobnicatorquuxbaz"],
            target_resolved=False,
        )
        self.assertEqual(no_hit, {})

        # 4. plw agent orient accepts positional <task> alongside --task and fails closed when missing
        with mock.patch("hott_kernel.kernel_work", return_value={
            "mode": "work",
            "frame_only": True,
            "agent_decision_frame": {"target": {"state": "RESOLVED"}, "action_readiness": "AGENT_DECISION_REQUIRED"},
            "skill_handoff": {"skill_path": "skills/test.md"},
        }) as kw_mock:
            buf_orient = io.StringIO()
            with redirect_stdout(buf_orient):
                rc_orient = plw_cli.cmd_agent(["orient", "Trace buildChatHistory", str(poly_repo), "--frame-only"])
            self.assertEqual(rc_orient, 0)
            self.assertEqual(kw_mock.call_args.kwargs["task"], "Trace buildChatHistory")
            self.assertEqual(kw_mock.call_args.kwargs["scan_root"], str(poly_repo))

        buf_err = io.StringIO()
        with redirect_stdout(buf_err), mock.patch("sys.stderr", new=io.StringIO()):
            with self.assertRaises(SystemExit) as sys_exit:
                plw_cli.cmd_agent(["orient", str(poly_repo), "--json"])
            self.assertEqual(sys_exit.exception.code, 2)

    def test_calibration_ux_v18_uniform_output_json_flag_normalization(self) -> None:
        """Consolidated calibration overlay + uniform --output/--format json normalization."""
        import io
        import tempfile
        from contextlib import redirect_stderr, redirect_stdout

        import hott_kernel
        import plw_cli

        tmp_ctx = tempfile.TemporaryDirectory()
        self.addCleanup(tmp_ctx.cleanup)
        tmp_root = Path(tmp_ctx.name) / "v18_uniform_json_repo"
        (tmp_root / "src" / "app").mkdir(parents=True, exist_ok=True)
        (tmp_root / "src" / "app" / "app.ts").write_text(
            "import { helper } from './helper';\n"
            "export class App { run() { return helper(); } }\n",
            encoding="utf-8",
        )
        (tmp_root / "src" / "app" / "helper.ts").write_text(
            "export function helper() { return 42; }\n",
            encoding="utf-8",
        )

        # 1. plw_cli commands accept --output json, --output=json, --format json, --format=json uniformly
        cli_cases = [
            (plw_cli.cmd_resolve_target, ["--root", str(tmp_root), "--target", "app", "--output", "json"]),
            (plw_cli.cmd_resolve_target, ["--root", str(tmp_root), "--target", "app", "--output=json"]),
            (plw_cli.cmd_mermaid, [str(tmp_root), "app", "--format", "json"]),
            (plw_cli.cmd_mermaid, [str(tmp_root), "app", "--format=json"]),
            (plw_cli.cmd_check, [str(tmp_root), "--target", "app", "--output", "json"]),
            (plw_cli.cmd_simplify, [str(tmp_root), "--target", "app", "--format=json"]),
            (plw_cli.cmd_context, ["helper", str(tmp_root), "--output", "json"]),
            (plw_cli.cmd_context, ["helper", str(tmp_root), "--output=json"]),
            (plw_cli.cmd_context, ["helper", str(tmp_root), "--format", "json"]),
            (plw_cli.cmd_semantic, ["describe", "cycle-break", "--output", "json"]),
        ]
        for fn, argv in cli_cases:
            buf = io.StringIO()
            with redirect_stdout(buf):
                rc = fn(argv)
            self.assertEqual(rc, 0, f"{fn.__name__}({argv}) failed with rc={rc}: {buf.getvalue()}")
            payload = json.loads(buf.getvalue())
            self.assertIsInstance(payload, dict, f"{fn.__name__}({argv}) did not emit JSON object")

        # 2. plw_cli fails closed (exit 2) on invalid --output / --format values
        for fn, argv in [
            (plw_cli.cmd_resolve_target, ["--root", str(tmp_root), "--target", "app", "--output", "bogus"]),
            (plw_cli.cmd_mermaid, [str(tmp_root), "--format", "yaml"]),
            (plw_cli.cmd_context, ["helper", str(tmp_root), "--output", "bogus"]),
        ]:
            buf = io.StringIO()
            with redirect_stdout(buf), redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit) as ctx:
                    fn(argv)
            self.assertEqual(ctx.exception.code, 2, f"{fn.__name__}({argv}) should fail closed with rc=2")

        # 3. Missing required arg with --output json still emits structured JSON error
        buf_err = io.StringIO()
        with redirect_stdout(buf_err):
            with self.assertRaises(SystemExit) as err_ctx:
                plw_cli.cmd_context(["--root", str(tmp_root), "--output", "json"])
        self.assertEqual(err_ctx.exception.code, 2)
        err_payload = json.loads(buf_err.getvalue())
        self.assertIn("error", err_payload)

        # 4. hott_kernel CLI passthrough commands support --output json, --output=json, --format json, --format=json, --output=summary
        kernel_cases = [
            ("impact", ["hott_kernel.py", "impact", "app", str(tmp_root), "--output", "json"]),
            ("impact", ["hott_kernel.py", "impact", "app", str(tmp_root), "--output=json"]),
            ("brief", ["hott_kernel.py", "brief", "app", str(tmp_root), "--format", "json"]),
            ("brief", ["hott_kernel.py", "brief", "app", str(tmp_root), "--format=json"]),
            ("brief", ["hott_kernel.py", "brief", "app", str(tmp_root), "--output=summary"]),
            ("cycle_break", ["hott_kernel.py", "cycle-break", "app", str(tmp_root), "--output", "json"]),
            ("cycle_break", ["hott_kernel.py", "cycle-break", "app", str(tmp_root), "--format=json"]),
        ]
        for key, argv in kernel_cases:
            buf = io.StringIO()
            with redirect_stdout(buf):
                hott_kernel._KERNEL_CLI_DISPATCH[key](argv)
            payload = json.loads(buf.getvalue())
            self.assertIsInstance(payload, dict, f"kernel {key}({argv}) did not emit JSON")
            self.assertNotIn("error", payload, f"kernel {key}({argv}) returned error: {payload}")

        # 5. hott_kernel fails closed with exit 2 on invalid output mode
        for key, argv in [
            ("impact", ["hott_kernel.py", "impact", "app", str(tmp_root), "--output", "bogus"]),
            ("cycle_break", ["hott_kernel.py", "cycle-break", "app", str(tmp_root), "--format=yaml"]),
        ]:
            buf = io.StringIO()
            with redirect_stdout(buf):
                with self.assertRaises(SystemExit) as ctx:
                    hott_kernel._KERNEL_CLI_DISPATCH[key](argv)
            self.assertEqual(ctx.exception.code, 2)
            payload = json.loads(buf.getvalue())
            self.assertIn("Invalid output mode", payload.get("error", ""))

        # 6. Baked capsule & overlay integrity: superseded calibration overlays stay absent and capsule digest verifies
        import hashlib
        import re
        repo_root = Path(plw_cli.__file__).resolve().parent
        overlay_dir = repo_root / "runtime_overlay"
        capsule_path = repo_root / "runtime_source" / "plw-runtime-src.tar.gz"
        manifest_path = repo_root / "runtime_source" / "CAPSULE_SHA256"
        self.assertTrue(capsule_path.is_file())
        self.assertTrue(manifest_path.is_file())
        expected_sha = manifest_path.read_text(encoding="utf-8").split()[0]
        actual_sha = hashlib.sha256(capsule_path.read_bytes()).hexdigest()
        self.assertEqual(actual_sha, expected_sha)
        self.assertIn(expected_sha, (overlay_dir / "README.md").read_text(encoding="utf-8"))
        self.assertFalse(
            (overlay_dir / "calibration_ux_consolidated_v1").exists(),
            "Baked overlay calibration_ux_consolidated_v1 should be folded into runtime_source capsule",
        )
        for v in range(3, 18):
            self.assertFalse(
                (overlay_dir / f"calibration_ux_v{v}").exists(),
                f"Superseded overlay calibration_ux_v{v} should not exist",
            )
        # Every overlay directory on disk must be registered in scripts/apply_runtime_overlays.sh
        apply_script = (repo_root / "scripts" / "apply_runtime_overlays.sh").read_text(encoding="utf-8")
        registered = set(
            re.findall(r'^apply_(?:b64|reviewable)_overlay\s+"([^"]+)"', apply_script, re.M)
        )
        on_disk = {d.name for d in overlay_dir.iterdir() if d.is_dir()}
        self.assertEqual(on_disk, registered, "Orphan or missing runtime_overlay directories")


if __name__ == "__main__":
    unittest.main()
