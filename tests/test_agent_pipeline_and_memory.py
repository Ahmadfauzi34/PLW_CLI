"""Agent orient/chain pipeline, UI handoff & stopwords, memory transitions/steer, context budget, and drift guards."""
from __future__ import annotations

import io
import json
import os
import sys
import unittest
from contextlib import redirect_stdout
from unittest import mock
from core.shared_graph import _LANGUAGE_IMPORT_RESOLVERS, build_shared_graph
import hott_kernel

try:
    from tests._baseline_fixture import ArchitectureBaselineCase
except ImportError:
    from _baseline_fixture import ArchitectureBaselineCase


class AgentPipelineAndMemoryTests(ArchitectureBaselineCase):
    def test_calibration_ux_v5_ui_stopwords_barrel_reexport_and_check_basis(self) -> None:
        from codebase.static_ui_reference import QUERY_STOPWORDS, _score
        from codebase.cycle_break import (
            _extract_imported_symbols,
            _inspect_edge_import,
            evaluate_edge_cut,
        )

        # 1. Non-UI refactoring tasks must not false-match via 2-char words like "to"
        for sw in ("to", "of", "remove", "extract", "dependency"):
            self.assertIn(sw, QUERY_STOPWORDS)
        score, matched, _ = _score(
            "Refactor patcher.ts to remove HomotopyTelemetryRegistry cycle",
            "p",
            "Simulate critical thread anomalies to protect the host",
            {},
        )
        self.assertEqual(score, 0)
        self.assertEqual(matched, [])

        # Short label distinction (e.g. "Sign in" vs "Sign up") remains intact
        s_in, m_in, _ = _score("Sign in", "button", "Sign in", {})
        s_up, _, _ = _score("Sign in", "button", "Sign up", {})
        self.assertGreater(s_in, s_up)
        self.assertIn("in", m_in)

        # 2. Wildcard barrel re-export (`export * from './x'`) uses barrel_reexport
        self.assertEqual(_extract_imported_symbols("*"), [])
        barrel_path = str((self.proj / "src" / "app" / "index.ts").resolve())
        leaf_path = str((self.proj / "src" / "app" / "leaf.ts").resolve())
        consumer_path = str((self.proj / "src" / "app" / "consumer.ts").resolve())
        fmap = {
            barrel_path: "export * from './leaf';\n",
            leaf_path: "export const LeafVal = 1;\n",
            consumer_path: "import { LeafVal } from './leaf';\n",
        }
        info = _inspect_edge_import(barrel_path, leaf_path, fmap, {})
        self.assertTrue(info["has_import"])
        self.assertTrue(info["is_reexport"])
        self.assertTrue(info["is_wildcard_reexport"])
        self.assertEqual(info["symbols"], [])

        fake_graph = {
            "file_map": fmap,
            "node_metadata": {
                barrel_path: {"type": "Other", "fan_in": 1, "fan_out": 1},
                leaf_path: {"type": "Other", "fan_in": 1, "fan_out": 1},
                consumer_path: {"type": "Other", "fan_in": 1, "fan_out": 1},
            },
            "type_shapes": {},
            "edge_natures": {},
        }
        barrel_cut = evaluate_edge_cut(barrel_path, leaf_path, fake_graph, cycle_overlap_count=1)
        concrete_cut = evaluate_edge_cut(consumer_path, leaf_path, fake_graph, cycle_overlap_count=1)
        self.assertEqual(barrel_cut["strategy"], "barrel_reexport")
        self.assertEqual(barrel_cut["symbols_involved"], [])
        self.assertTrue(barrel_cut["is_wildcard_reexport"])
        self.assertGreater(barrel_cut["cut_cost"], concrete_cut["cut_cost"])

        # 3. _top_check_findings strips bulky witness_edges & vertices from cycle_basis
        findings, _ = hott_kernel._top_check_findings(
            {
                "results": {
                    "hott.manifold": {
                        "findings": [
                            {
                                "type": "independent_cycles",
                                "severity": "medium",
                                "cycle_basis": [
                                    {
                                        "basis_index": 1,
                                        "closed_path": ["a.ts", "b.ts", "a.ts"],
                                        "closing_edge": ["b.ts", "a.ts"],
                                        "vertices": ["a.ts", "b.ts"],
                                        "witness_edges": [{"edge": ["a.ts", "b.ts"]}],
                                        "length": 2,
                                        "orientation": "directed",
                                    }
                                ],
                            }
                        ]
                    }
                }
            }
        )
        basis0 = findings[0]["cycle_basis"][0]
        self.assertNotIn("witness_edges", basis0)
        self.assertNotIn("vertices", basis0)
        self.assertEqual(basis0["witness_edge_count"], 1)
        self.assertEqual(basis0["closed_path"], ["a.ts", "b.ts", "a.ts"])

        # 4. kernel_cycle_break preserves hidden dot-directory prefix on target
        cb_hidden = hott_kernel.kernel_cycle_break(str(self.proj), target_file="./.hidden/ci.ts")
        self.assertEqual(cb_hidden["target"], ".hidden/ci.ts")

    def test_calibration_ux_v7_chain_frame_ui_handoff_cli_and_analyzers(self) -> None:
        import plw_cli
        from codebase.static_ui_reference import _score
        from codebase.performance_analyzers import (
            analyze_async_waterfall,
            analyze_cache,
            analyze_deopt,
            analyze_gc_pressure,
        )
        from codebase.topology_analyzers import analyze_test_reachability

        # 1. kernel_task_chain(frame_only=True) populates non-null target_resolution, evidence_need_derivation, capability_selection
        chain_frame = hott_kernel.kernel_task_chain(
            str(self.proj),
            "Investigate utils in src/app/utils.py",
            target_files=["src/app/utils.py"],
            frame_only=True,
        )
        self.assertTrue(chain_frame["frame_only"])
        self.assertEqual(chain_frame["target_resolution"]["status"], "RESOLVED")
        self.assertEqual(chain_frame["target_resolution"]["targets"], ["src/app/utils.py"])
        self.assertEqual(
            chain_frame["target_resolution"]["resolution"]["selected_targets"],
            ["src/app/utils.py"],
        )
        self.assertIsNotNone(chain_frame["evidence_need_derivation"]["task_intent"])
        self.assertTrue(len(chain_frame["evidence_need_derivation"]["required_evidence"]) >= 1)
        self.assertIsNotNone(chain_frame["capability_selection"]["selected"])
        self.assertTrue(len(chain_frame["capability_selection"]["recommended_capabilities"]) >= 1)

        # 2. CLI ergonomics: plw chain <root> --task ..., plw resolve-target <root> --task ..., and plw ast index warm <root>
        buf_chain = io.StringIO()
        with redirect_stdout(buf_chain):
            rc_chain = plw_cli.cmd_chain(
                [str(self.proj), "--task", "Audit utils", "--target", "src/app/utils.py", "--frame-only", "--json"]
            )
        self.assertEqual(rc_chain, 0)
        chain_cli_payload = json.loads(buf_chain.getvalue())
        self.assertEqual(chain_cli_payload["target_resolution"]["status"], "RESOLVED")

        buf_rt = io.StringIO()
        with redirect_stdout(buf_rt):
            rc_rt = plw_cli.cmd_resolve_target(
                [str(self.proj), "--task", "Audit utils", "--target", "src/app/utils.py", "--json"]
            )
        self.assertEqual(rc_rt, 0)
        rt_payload = json.loads(buf_rt.getvalue())
        self.assertEqual(
            rt_payload["target_resolution"]["resolution"]["selected_targets"],
            ["src/app/utils.py"],
        )

        root_a, op_a, err_a = plw_cli._resolve_ast_positionals("index", "warm", str(self.proj), None)
        self.assertIsNone(err_a)
        self.assertEqual(root_a, self.proj.resolve())
        self.assertEqual(op_a, "warm")

        root_b, op_b, err_b = plw_cli._resolve_ast_positionals("index", str(self.proj), "status", None)
        self.assertIsNone(err_b)
        self.assertEqual(root_b, self.proj.resolve())
        self.assertEqual(op_b, "status")

        # 3. static_ui_reference _score ignores file paths, snake_case identifiers, and standalone 2-char prepositions on multi-word labels
        backend_query = "Investigate perf.gc json_in_loop in src/app/agent/agent-executor.ts"
        score_sign_in, _, _ = _score(backend_query, "a", "Sign in", {"routerlink": "/login"})
        self.assertEqual(score_sign_in, 0)
        score_json_btn, _, _ = _score(backend_query, "button", "Export JSON (.json)", {})
        self.assertEqual(score_json_btn, 0)
        score_real_ui, matched_ui, _ = _score("Sign in navigation link", "a", "Sign in", {"routerlink": "/login"})
        self.assertGreater(score_real_ui, 0)
        self.assertEqual(matched_ui, ["in", "sign"])

        # 4. Analyzer precision hardening (multiline template literals, WeakMap, Service Worker, Promise.all, tool config reachability)
        embedded_py_ts = (
            "export const PY_CODE = `\n"
            "def eval(x):\n"
            "    return eval(x)\n"
            "`;\n"
            "delete realObj.prop;\n"
        )
        deopt_res = analyze_deopt(
            {
                "file_map": {
                    "src/bridge.ts": embedded_py_ts,
                    "src/bridge.spec.ts": "const x = eval(\"1+1\");\n",
                }
            }
        )
        self.assertEqual(len(deopt_res["findings"]), 1)
        self.assertEqual(deopt_res["findings"][0]["type"], "delete_operator")
        self.assertEqual(deopt_res["findings"][0]["line"], 5)

        weak_cache_ts = (
            "const receiptCache = new WeakMap<object, string>();\n"
            "export function mark(k: object) { receiptCache.set(k, \"ok\"); }\n"
        )
        cache_res = analyze_cache({"file_map": {"src/weak.ts": weak_cache_ts}})
        self.assertEqual(cache_res["findings"], [])

        sw_and_template_gc = {
            "file_map": {
                "public/service-worker.js": "self.addEventListener(\"install\", () => {});\n",
                "src/worker-entry.ts": "self.addEventListener(\"message\", () => {});\n",
                "src/canvas.ts": "const html = `\n<script>window.addEventListener(\"message\", fn);</script>\n`;\n",
            }
        }
        gc_res = analyze_gc_pressure(sw_and_template_gc)
        self.assertEqual(gc_res["findings"], [])

        batch_async_ts = (
            "async function runBatches(batches: any[][]) {\n"
            "  for (const b of batches) {\n"
            "    await Promise.all(b.map(x => x()));\n"
            "  }\n"
            "}\n"
        )
        spec_sync_io = "import { readFileSync } from \"fs\";\nreadFileSync(\"a.txt\");\n"
        async_res = analyze_async_waterfall(
            {
                "file_map": {
                    "src/batch.ts": batch_async_ts,
                    "src/my.spec.ts": spec_sync_io,
                },
                "node_metadata": {"src/my.spec.ts": {"is_test": True}},
            }
        )
        high_async = [f for f in async_res["findings"] if f["severity"] == "high"]
        self.assertEqual(high_async, [])

        topo_reach = analyze_test_reachability(
            {
                "vertices": ["eslint.config.js", "src/service.ts", "src/service.spec.ts"],
                "edges": [["src/service.spec.ts", "src/service.ts"]],
                "node_metadata": {
                    "eslint.config.js": {"is_entrypoint": True, "fan_in": 0, "is_test": False},
                    "src/service.ts": {"is_entrypoint": False, "fan_in": 3, "is_test": False},
                    "src/service.spec.ts": {"is_entrypoint": False, "fan_in": 0, "is_test": True},
                },
            }
        )
        high_inf_files = [
            f["file"]
            for f in topo_reach["findings"]
            if f["type"] == "high_influence_without_test_path"
        ]
        self.assertNotIn("eslint.config.js", high_inf_files)

    def test_calibration_ux_v9_memory_transitions_steer_and_async_precision(self) -> None:
        import plw_cli
        import hott_kernel
        from codebase.performance_analyzers import analyze_async_waterfall, analyze_gc_pressure

        # 1. plw memory store <content> --type architectural & plw memory bridge-candidates
        buf_mem = io.StringIO()
        with redirect_stdout(buf_mem):
            handled = hott_kernel._cli_memory_operation(
                "store",
                ["hott_kernel.py", "memory", "store", "Sample arch note", "--type", "architectural", "--tags", "arch"],
                arg_offset=3,
            )
        self.assertTrue(handled)
        mem_payload = json.loads(buf_mem.getvalue())
        self.assertNotIn("error", mem_payload)

        # 2. plw transitions <root> <query> & plw steer <goal> <root>
        buf_tr = io.StringIO()
        with redirect_stdout(buf_tr):
            rc_tr = plw_cli.cmd_transitions([str(self.proj), "Inspect utils", "--target", "src/app/utils.py", "--json"])
        self.assertEqual(rc_tr, 0)

        buf_st = io.StringIO()
        with redirect_stdout(buf_st):
            rc_st = plw_cli.cmd_steer(["Inspect utils", str(self.proj), "--json"])
        self.assertEqual(rc_st, 0)

        # 3. perf.async sequential_await does not cross if/else or callback scopes, and .map(async) is not await_in_loop
        async_res = analyze_async_waterfall(
            {
                "file_map": {
                    "src/service.ts": (
                        "async function run(flag, items) {\n"
                        "  if (flag) {\n"
                        "    const res = await stepOne();\n"
                        "    return res;\n"
                        "  } else {\n"
                        "    const res = await stepTwo();\n"
                        "    return res;\n"
                        "  }\n"
                        "  for (let i = 0; i < items.length; i += 4) {\n"
                        "    const batch = items.slice(i, i + 4);\n"
                        "    const jobs = batch.map(async (x) => {\n"
                        "      const out = await work(x);\n"
                        "      return JSON.stringify(out);\n"
                        "    });\n"
                        "    await Promise.all(jobs);\n"
                        "  }\n"
                        "}\n"
                    )
                }
            }
        )
        self.assertEqual(async_res["findings"], [])

        # 4. perf.gc json_in_loop skips return JSON.stringify and downgrades camelCase maxRetries retry loop to info
        gc_res = analyze_gc_pressure(
            {
                "file_map": {
                    "src/retry.ts": (
                        "function retryWork(maxRetries) {\n"
                        "  for (let attempt = 0; attempt <= maxRetries; attempt++) {\n"
                        "    const payload = JSON.stringify({ attempt });\n"
                        "    if (attempt === maxRetries) return JSON.stringify({ error: payload });\n"
                        "  }\n"
                        "}\n"
                    )
                }
            }
        )
        self.assertEqual(len(gc_res["findings"]), 1)
        self.assertEqual(gc_res["findings"][0]["severity"], "info")

    def test_calibration_ux_v11_cross_language_drift_guards(self) -> None:
        from codebase.hott_analyzers import analyze_homotopy, analyze_sheaf
        from codebase.performance_analyzers import (
            _is_test_file as perf_is_test_file,
            analyze_async_waterfall,
            analyze_cache,
            analyze_deopt,
            analyze_gc_pressure,
        )
        from core.shared_graph import _is_test_file as graph_is_test_file

        # 1. Performance analyzers gate JS/V8/DOM heuristics strictly to JS/TS files
        poly_file_map = {
            "src/main/kotlin/com/app/ApiClient.kt": (
                "package com.app\n"
                "class ApiClient(private val cache: MemoryCache) {\n"
                "  suspend fun run(ids: List<String>) {\n"
                "    for (id in ids) {\n"
                "      cache.set(id, id)\n"
                "      window.addEventListener(\"click\", cb)\n"
                "    }\n"
                "  }\n"
                "  fun clean(f: File) { f.delete() }\n"
                "}\n"
            ),
            "src/net/client.rs": (
                "pub async fn sync_all(items: &[String]) {\n"
                "  let cache = HashMap::new();\n"
                "  for item in items {\n"
                "    cache.set(item, item);\n"
                "    let _ = JSON.parse(item);\n"
                "  }\n"
                "}\n"
            ),
            "app/worker.py": (
                "async def run(items):\n"
                "  for item in items:\n"
                "    await fetch(item)\n"
                "    cache.set(item, item)\n"
            ),
        }
        self.assertEqual(analyze_async_waterfall({"file_map": poly_file_map})["findings"], [])
        self.assertEqual(analyze_deopt({"file_map": poly_file_map})["findings"], [])
        self.assertEqual(analyze_gc_pressure({"file_map": poly_file_map})["findings"], [])
        self.assertEqual(analyze_cache({"file_map": poly_file_map})["findings"], [])

        # JS/TS negative control: real JS/TS issue still flags
        ts_cache_res = analyze_cache(
            {
                "file_map": {
                    "src/store.ts": (
                        "export const tokenCache = new Map<string, string>();\n"
                        "export function put(k: string, v: string) { tokenCache.set(k, v); }\n"
                    )
                }
            }
        )
        self.assertTrue(any(f["type"] == "unbounded_cache" for f in ts_cache_res["findings"]))

        # 2. Test-file convention recognition without false-matching e.g. Contest.kt
        for test_path in (
            "src/main/kotlin/com/app/ScreenTest.kt",
            "src/main/kotlin/com/app/ApiTests.kt",
            "src/androidTest/kotlin/com/app/Flow.kt",
            "src/cache/store_test.rs",
            "tests/integration.rs",
            "tests/test_helper.py",
        ):
            self.assertTrue(perf_is_test_file(test_path), f"perf missed test: {test_path}")
            self.assertTrue(graph_is_test_file(test_path), f"graph missed test: {test_path}")

        for prod_path in (
            "src/main/kotlin/com/app/Contest.kt",
            "src/main/kotlin/com/app/Latest.kt",
            "src/net/request.rs",
        ):
            self.assertFalse(perf_is_test_file(prod_path), f"false test: {prod_path}")
            self.assertFalse(graph_is_test_file(prod_path), f"false test: {prod_path}")

        # 3. hott.sheaf skips boundary_without_public_api for Kotlin/Rust boundaries, preserves JS/TS
        sheaf_res = analyze_sheaf(
            {
                "boundaries": {
                    "/repo/kt/ui": {"barrel": None, "files": ["/repo/kt/ui/Screen.kt"]},
                    "/repo/kt/data": {"barrel": None, "files": ["/repo/kt/data/Repo.kt"]},
                    "/repo/rs/cache": {"barrel": None, "files": ["/repo/rs/cache/store.rs"]},
                    "/repo/rs/net": {"barrel": None, "files": ["/repo/rs/net/client.rs"]},
                    "/repo/ts/core": {"barrel": None, "files": ["/repo/ts/core/svc.ts"]},
                    "/repo/ts/web": {"barrel": None, "files": ["/repo/ts/web/page.ts"]},
                },
                "file_to_boundary": {
                    "/repo/kt/ui/Screen.kt": "/repo/kt/ui",
                    "/repo/kt/data/Repo.kt": "/repo/kt/data",
                    "/repo/kt/data/RepoTest.kt": "/repo/kt/data",
                    "/repo/rs/cache/store.rs": "/repo/rs/cache",
                    "/repo/rs/net/client.rs": "/repo/rs/net",
                    "/repo/rs/net/client_test.rs": "/repo/rs/net",
                    "/repo/ts/core/svc.ts": "/repo/ts/core",
                    "/repo/ts/web/page.ts": "/repo/ts/web",
                },
                "resolved_imports": {
                    "/repo/kt/ui/Screen.kt": ["/repo/kt/data/Repo.kt"],
                    "/repo/kt/data/RepoTest.kt": ["/repo/kt/ui/Screen.kt"],
                    "/repo/rs/net/client.rs": ["/repo/rs/cache/store.rs"],
                    "/repo/rs/net/client_test.rs": ["/repo/rs/cache/store.rs"],
                    "/repo/ts/web/page.ts": ["/repo/ts/core/svc.ts"],
                },
                "edge_natures": {},
            }
        )
        no_api_boundaries = [
            f["boundary"]
            for f in sheaf_res["findings"]
            if f["type"] == "boundary_without_public_api"
        ]
        self.assertEqual(no_api_boundaries, ["/repo/ts/core"])
        self.assertEqual(
            [f for f in sheaf_res["findings"] if f["type"] == "circular_boundary"],
            [],
        )

        # 4. hott.homotopy ignores Rust/Kotlin test files & Rust mod.rs/lib.rs barrels as diamond sources
        homotopy_res = analyze_homotopy(
            {
                "vertices": [
                    "src/lib.rs",
                    "src/a.rs",
                    "src/b.rs",
                    "src/leaf.rs",
                    "src/leaf_test.rs",
                    "ScreenTest.kt",
                ],
                "edges": [
                    ["src/lib.rs", "src/a.rs"],
                    ["src/lib.rs", "src/b.rs"],
                    ["src/a.rs", "src/leaf.rs"],
                    ["src/b.rs", "src/leaf.rs"],
                    ["src/leaf_test.rs", "src/a.rs"],
                    ["src/leaf_test.rs", "src/b.rs"],
                    ["ScreenTest.kt", "src/a.rs"],
                    ["ScreenTest.kt", "src/b.rs"],
                ],
                "file_map": {
                    "src/lib.rs": "pub mod a; pub mod b;",
                    "src/a.rs": "",
                    "src/b.rs": "",
                    "src/leaf.rs": "",
                    "src/leaf_test.rs": "",
                    "ScreenTest.kt": "",
                },
            }
        )
        self.assertEqual(
            [f for f in homotopy_res["findings"] if f["type"] == "diamond_dependency"],
            [],
        )

    def test_calibration_ux_v14_context_budget_ui_static_describe_and_help(self) -> None:
        import json
        import plw_cli
        from core.context_optimizer import build_context_pack
        from core.shared_graph import build_shared_graph

        # 1. Context optimizer keeps within_budget honest against adaptive token estimates
        ctx_repo = self.proj.parent / "v14_ctx_repo"
        (ctx_repo / "src").mkdir(parents=True, exist_ok=True)
        for idx in range(5):
            body = "\n".join(f"export const sym{idx}_{j} = {j}; // dense_code_line_{j}" for j in range(45))
            (ctx_repo / "src" / f"mod_{idx}.ts").write_text(body + "\n", encoding="utf-8")
        g_ctx = build_shared_graph(str(ctx_repo))
        pack = build_context_pack(
            g_ctx,
            query="dense_code_line mod_0 mod_1 mod_2 mod_3",
            budget_tokens=400,
            memory_context=None,
        )
        budget = pack["budget"]
        self.assertEqual(
            budget["within_budget"],
            budget["used_chars"] <= budget["char_budget"]
            and budget["estimated_tokens"] <= budget["requested_estimated_tokens"],
        )

        # 2. plw ui static human output renders @decl:<line> and attribute hint instead of :None ''
        ui_repo = self.proj.parent / "v14_ui_repo"
        (ui_repo / "src").mkdir(parents=True, exist_ok=True)
        (ui_repo / "src" / "composer.component.ts").write_text(
            "import { Component } from '@angular/core';\n"
            "@Component({\n"
            "  selector: 'app-composer',\n"
            "  standalone: true,\n"
            "  template: `\n"
            "    <button type=\"submit\" class=\"btn-send\" aria-label=\"Send prompt\"></button>\n"
            "  `\n"
            "})\n"
            "export class ComposerComponent {}\n",
            encoding="utf-8",
        )
        buf_ui_human = io.StringIO()
        with redirect_stdout(buf_ui_human):
            rc_ui = plw_cli.cmd_ui_static(["Send", str(ui_repo)])
        self.assertEqual(rc_ui, 0)
        ui_human = buf_ui_human.getvalue()
        self.assertIn("@decl:2", ui_human)
        self.assertIn("btn-send", ui_human)
        self.assertNotIn(":None", ui_human)
        buf_ui_json = io.StringIO()
        with redirect_stdout(buf_ui_json):
            rc_ui_j = plw_cli.cmd_ui_static(["Send", str(ui_repo), "--json"])
        self.assertEqual(rc_ui_j, 0)
        self.assertIsNone(json.loads(buf_ui_json.getvalue())["decision"]["selected"]["template_line"])

        # 3. plw describe surfaces hint/alias_of/available_commands & handles command names matching CWD dirs
        desc_cwd = self.proj.parent / "v14_desc_cwd"
        (desc_cwd / "ui").mkdir(parents=True, exist_ok=True)
        old_cwd = os.getcwd()
        try:
            os.chdir(str(desc_cwd))
            buf_desc_ui = io.StringIO()
            with redirect_stdout(buf_desc_ui):
                rc_du = plw_cli.cmd_semantic(["describe", "ui", "static"])
            self.assertEqual(rc_du, 3)
            self.assertIn("# Command: ui static", buf_desc_ui.getvalue())
            self.assertIn("# Hint:", buf_desc_ui.getvalue())
            self.assertIn("plw ui --help", buf_desc_ui.getvalue())

            buf_desc_pack = io.StringIO()
            with redirect_stdout(buf_desc_pack):
                rc_dp = plw_cli.cmd_semantic(["describe", "pack"])
            self.assertEqual(rc_dp, 3)
            self.assertIn("# Alias of: context", buf_desc_pack.getvalue())
            self.assertIn("# Hint:", buf_desc_pack.getvalue())
        finally:
            os.chdir(old_cwd)

        # 4. plw impact --help rewrites kernel usage prefix to `plw ` without altering hott_kernel itself
        buf_help = io.StringIO()
        with mock.patch.object(sys, "argv", ["plw", "impact", "--help"]), redirect_stdout(buf_help):
            plw_cli.main()
        self.assertIn('"plw impact <file>', buf_help.getvalue())
        self.assertNotIn("python3 hott_kernel.py", buf_help.getvalue())


if __name__ == "__main__":
    unittest.main()
