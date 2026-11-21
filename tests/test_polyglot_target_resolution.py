"""Polyglot outline, short-stem target resolution, impact/Kan analysis, and Angular/AST filtering."""
from __future__ import annotations

import io
import json
import unittest
from contextlib import redirect_stdout
from unittest import mock
import hott_kernel

try:
    from tests._baseline_fixture import ArchitectureBaselineCase
except ImportError:
    from _baseline_fixture import ArchitectureBaselineCase


class PolyglotTargetResolutionTests(ArchitectureBaselineCase):
    def test_calibration_ux_v15_polyglot_outline_stem_impact_kan_and_steer(self) -> None:
        import plw_cli

        poly_root = self.proj.parent / "v15_poly_repo"
        (poly_root / "src" / "services").mkdir(parents=True, exist_ok=True)
        (poly_root / "src" / "services" / "pyodide-client.ts").write_text(
            "export class PyodideClient { run(): number { return 1; } }\n",
            encoding="utf-8",
        )
        (poly_root / "src" / "services" / "file_scanner.py").write_text(
            "import os\n\n@staticmethod\ndef scan_directory(root: str) -> list[str]:\n    return os.listdir(root)\n\ndef _private_helper() -> None:\n    pass\n",
            encoding="utf-8",
        )
        (poly_root / "src" / "services" / "lib.rs").write_text(
            "use std::collections::HashMap;\npub mod engine;\n#[derive(Debug)]\npub struct KernelState { pub count: usize }\npub fn run_kernel() -> usize { 42 }\nfn internal_fn() {}\n",
            encoding="utf-8",
        )
        (poly_root / "src" / "services" / "engine.rs").write_text(
            "use crate::KernelState;\npub fn step(_s: &KernelState) {}\n",
            encoding="utf-8",
        )
        (poly_root / "src" / "services" / "SyncEngine.kt").write_text(
            "package com.example.app\nimport kotlin.math.max\n@Singleton\nclass SyncEngine {}\nfun runSync(): Int = max(1, 2)\nprivate fun secret(): Int = 0\n",
            encoding="utf-8",
        )
        (poly_root / "src" / "models").mkdir(parents=True, exist_ok=True)
        (poly_root / "src" / "models" / "alpha.types.ts").write_text(
            "export interface AlphaModel { id: string; }\n",
            encoding="utf-8",
        )
        (poly_root / "src" / "models" / "beta.types.ts").write_text(
            "export interface BetaModel { id: string; }\n",
            encoding="utf-8",
        )
        (poly_root / "src" / "models" / "gamma.types.ts").write_text(
            "export interface GammaModel { id: string; }\n",
            encoding="utf-8",
        )

        # 1. Polyglot query_outline for Python, Rust, and Kotlin
        out_py = hott_kernel.kernel_outline(str(poly_root), "src/services/file_scanner.py")
        self.assertTrue(out_py["exists"])
        self.assertIn("scan_directory", out_py["exports"])
        self.assertNotIn("_private_helper", out_py["exports"])
        self.assertEqual(out_py["stats"]["outline_entries"], 2)
        self.assertIn("os", out_py["imports"])

        out_rs = hott_kernel.kernel_outline(str(poly_root), "src/services/lib.rs")
        self.assertTrue(out_rs["exists"])
        self.assertEqual(out_rs["exports"], ["engine", "KernelState", "run_kernel"])
        self.assertIn("derive", out_rs["decorators"])
        self.assertEqual(out_rs["stats"]["outline_entries"], 4)

        out_kt = hott_kernel.kernel_outline(str(poly_root), "src/services/SyncEngine.kt")
        self.assertTrue(out_kt["exists"])
        self.assertEqual(out_kt["exports"], ["SyncEngine", "runSync"])
        self.assertIn("Singleton", out_kt["decorators"])

        # 2. Stem and PascalCase target resolution on impact/brief/outline/context
        imp_stem = hott_kernel.kernel_impact(str(poly_root), "PyodideClient")
        self.assertTrue(imp_stem["exists"])
        self.assertEqual(imp_stem["target_resolution"], "unique_stem_match")
        self.assertTrue(imp_stem["target"].endswith("src/services/pyodide-client.ts"))

        ctx_stem = hott_kernel.kernel_context(str(poly_root), "Audit PyodideClient", target_files=["PyodideClient"])
        self.assertNotIn("error", ctx_stem)
        self.assertTrue(any(p.endswith("src/services/pyodide-client.ts") for p in ctx_stem["selection"]["resolved_targets"]))

        # 3. Summary mode on kernel_impact and kernel_brief preserves error on missing targets
        imp_miss = hott_kernel.kernel_impact(str(poly_root), "NonExistentTarget", output_mode="summary")
        self.assertFalse(imp_miss["exists"])
        self.assertEqual(imp_miss["error"], "file_not_found_in_graph")
        brf_miss = hott_kernel.kernel_brief(str(poly_root), "NonExistentTarget", output_mode="summary")
        self.assertFalse(brf_miss["exists"])
        self.assertEqual(brf_miss["error"], "file_not_found_in_graph")

        # 4. plw kan supports <root> <stem> auto-swap, root-relative resolution, and clean no_match human text
        buf_kan = io.StringIO()
        with redirect_stdout(buf_kan):
            rc_kan = plw_cli.cmd_kan([str(poly_root), "PyodideClient", "--json"])
        self.assertEqual(rc_kan, 0)
        kan_payload = json.loads(buf_kan.getvalue())
        self.assertTrue(kan_payload["target_file"].endswith("src/services/pyodide-client.ts"))

        buf_nomatch = io.StringIO()
        with redirect_stdout(buf_nomatch):
            rc_nm = plw_cli.cmd_kan(["nonexistent_memory_concept_v15_xyz"])
        self.assertEqual(rc_nm, 0)
        self.assertIn("Status: no_match", buf_nomatch.getvalue())
        self.assertNotIn('"schema_version"', buf_nomatch.getvalue())

        # 5. plw steer deduplicates swapped isomorphic module pair hints
        steer_res = hott_kernel.kernel_steer(str(poly_root), "Audit models")
        hints = steer_res.get("kan_guidance", {}).get("guided_hints", [])
        iso_hints = [h for h in hints if h.startswith("Rujuk pola implementasi modul isomorphic:")]
        self.assertEqual(len(iso_hints), len(set(iso_hints)))

    def test_calibration_ux_v16_target_resolution_mermaid_cycle_break_angular_and_ast_filter(self) -> None:
        import plw_cli

        v16_root = self.proj.parent / "v16_repo"
        (v16_root / "src" / "services").mkdir(parents=True, exist_ok=True)
        (v16_root / "src" / "services" / "pyodide-client.ts").write_text(
            "import { Helper } from './worker-pool';\n"
            "export class PyodideClient {\n"
            "  clear(): void { const x = 1; return; }\n"
            "  run(): number { return Helper ? 1 : 0; }\n"
            "}\n"
            "export function isReady(v: { kind: string }): boolean {\n"
            "  if (v.kind === 'ok') {\n"
            "    return true;\n"
            "  }\n"
            "  return false;\n"
            "}\n",
            encoding="utf-8",
        )
        (v16_root / "src" / "services" / "worker-pool.ts").write_text(
            "import { PyodideClient } from './pyodide-client';\n"
            "export class Helper {\n"
            "  clear(): void { const x = 1; return; }\n"
            "  ref(): unknown { return PyodideClient; }\n"
            "}\n",
            encoding="utf-8",
        )
        (v16_root / "src" / "ui").mkdir(parents=True, exist_ok=True)
        (v16_root / "src" / "ui" / "stream-orchestrator.ts").write_text(
            "import { Component } from '@angular/core';\n"
            "@Component({ selector: 'app-unified-shell', template: `<div>Shell</div>` })\n"
            "export class AgentUnifiedShellComponent {}\n",
            encoding="utf-8",
        )

        # 1. Explicit stem/PascalCase target resolution in evidence-needs & capability-match + --targets alias
        buf_en = io.StringIO()
        with redirect_stdout(buf_en):
            rc_en = plw_cli.cmd_evidence_needs(["Audit PyodideClient", str(v16_root), "--targets", "PyodideClient", "--json"])
        self.assertEqual(rc_en, 0)
        en_payload = json.loads(buf_en.getvalue())
        self.assertEqual(en_payload["evidence_need_derivation"]["stage"], "EVIDENCE_NEEDS_DERIVED")

        # 2. Mermaid focus stem/PascalCase resolution & positional <focus> (<root> <focus> and <focus> <root>)
        buf_mm1 = io.StringIO()
        with redirect_stdout(buf_mm1):
            rc_mm1 = plw_cli.cmd_mermaid([str(v16_root), "PyodideClient", "--depth", "4", "--json"])
        self.assertEqual(rc_mm1, 0)
        mm1_payload = json.loads(buf_mm1.getvalue())
        self.assertEqual(mm1_payload["decision"], "MERMAID_GRAPH_READY")

        buf_mm2 = io.StringIO()
        with redirect_stdout(buf_mm2):
            rc_mm2 = plw_cli.cmd_mermaid(["PyodideClient", str(v16_root), "--depth", "3", "--json"])
        self.assertEqual(rc_mm2, 0)
        self.assertEqual(json.loads(buf_mm2.getvalue())["decision"], "MERMAID_GRAPH_READY")

        # 3. cycle-break stem target resolution and <target> <root> positional support
        cb_res = hott_kernel.kernel_cycle_break(str(v16_root), target_file="PyodideClient")
        self.assertEqual(cb_res["summary"]["cycles_affecting_target"], 1)
        self.assertEqual(cb_res["target"], "PyodideClient")

        # 4. angular-anchors --target alias, stem file filter, and class_name filter
        buf_ng1 = io.StringIO()
        with redirect_stdout(buf_ng1):
            rc_ng1 = plw_cli.cmd_angular_anchors([str(v16_root), "--target", "StreamOrchestrator", "--json"])
        self.assertEqual(rc_ng1, 0)
        self.assertEqual(json.loads(buf_ng1.getvalue())["summary"]["component_count"], 1)

        buf_ng2 = io.StringIO()
        with redirect_stdout(buf_ng2):
            rc_ng2 = plw_cli.cmd_angular_anchors([str(v16_root), "AgentUnifiedShell", "--json"])
        self.assertEqual(rc_ng2, 0)
        self.assertEqual(json.loads(buf_ng2.getvalue())["summary"]["component_count"], 1)

        # 5. simplify --target / --kind filtering
        buf_simp = io.StringIO()
        with redirect_stdout(buf_simp):
            rc_simp = plw_cli.cmd_simplify([str(v16_root), "--target", "PyodideClient", "--json"])
        self.assertEqual(rc_simp, 0)
        simp_payload = json.loads(buf_simp.getvalue())
        self.assertEqual(simp_payload["filters"]["target"], "PyodideClient")
        self.assertTrue(len(simp_payload["candidates"]) >= 1)

    def test_calibration_ux_v17_short_stem_target_only_ui_swap_ast_precision_check_filter_and_cycle_break_contract(self) -> None:
        import plw_cli
        v17_root = self.proj.parent / "v17_proj"
        (v17_root / "src" / "app").mkdir(parents=True)
        (v17_root / "src" / "core" / "rag").mkdir(parents=True)
        (v17_root / "src" / "dup_a").mkdir(parents=True)
        (v17_root / "src" / "dup_b").mkdir(parents=True)

        (v17_root / "src" / "app" / "app.ts").write_text(
            "import { Component } from '@angular/core';\n"
            "import { queryRag } from '../core/rag/rag';\n"
            "@Component({ selector: 'app-root', template: `<button class=\"btn-send\" aria-label=\"Send\">Send</button>` })\n"
            "export class App { run() { return queryRag(); } }\n",
            encoding="utf-8",
        )
        (v17_root / "src" / "app" / "app.spec.ts").write_text(
            "import { App } from './app';\n"
            "export class AppSpec { check() { return new App().run(); } }\n",
            encoding="utf-8",
        )
        (v17_root / "src" / "core" / "rag" / "rag.ts").write_text(
            "import { App } from '../../app/app';\n"
            "export function queryRag() { return App; }\n",
            encoding="utf-8",
        )
        # Ambiguous short stem 'db' in two directories -> must fail closed
        (v17_root / "src" / "dup_a" / "db.ts").write_text("export const dbA = 1;\n", encoding="utf-8")
        (v17_root / "src" / "dup_b" / "db.ts").write_text("export const dbB = 2;\n", encoding="utf-8")

        # 1. Short 2-3 char exact stem resolution (app, rag) + ambiguous fail-closed (db)
        out_app = hott_kernel.kernel_outline(str(v17_root), "app")
        self.assertTrue(out_app.get("exists"), out_app)
        self.assertTrue(str(out_app.get("file", "")).endswith("src/app/app.ts"), out_app)
        self.assertEqual(out_app.get("target_resolution"), "unique_stem_match")

        out_rag = hott_kernel.kernel_outline(str(v17_root), "rag")
        self.assertTrue(out_rag.get("exists"), out_rag)
        self.assertTrue(str(out_rag.get("file", "")).endswith("src/core/rag/rag.ts"), out_rag)

        out_db = hott_kernel.kernel_outline(str(v17_root), "db")
        self.assertFalse(out_db.get("exists"), out_db)
        self.assertEqual(out_db.get("target_resolution"), "ambiguous")
        self.assertEqual(len(out_db.get("candidate_targets", [])), 2)

        # 2. Target-only invocation without mandatory task on resolve-target & chain
        buf_rt = io.StringIO()
        with redirect_stdout(buf_rt):
            rc_rt = plw_cli.cmd_resolve_target([str(v17_root), "--target", "rag", "--json"])
        self.assertEqual(rc_rt, 0)
        rt_payload = json.loads(buf_rt.getvalue())
        self.assertEqual(rt_payload["target_resolution"]["resolution"]["state"], "RESOLVED")
        self.assertEqual(rt_payload["target_resolution"]["resolution"]["selected_targets"], ["src/core/rag/rag.ts"])

        buf_ch = io.StringIO()
        with redirect_stdout(buf_ch):
            rc_ch = plw_cli.cmd_chain(["--root", str(v17_root), "--file", "app", "--json"])
        self.assertEqual(rc_ch, 0)
        ch_payload = json.loads(buf_ch.getvalue())
        self.assertEqual(ch_payload["task"], "app")
        self.assertEqual(ch_payload["target_resolution"]["resolution"]["state"], "RESOLVED")

        # Missing both task and --target must still fail closed with SystemExit(2)
        with self.assertRaises(SystemExit) as ctx_err:
            plw_cli.cmd_resolve_target(["--root", str(v17_root), "--json"])
        self.assertEqual(ctx_err.exception.code, 2)

        # 3. ui static <root> <query> positional auto-swap
        buf_ui = io.StringIO()
        with redirect_stdout(buf_ui):
            rc_ui = plw_cli.cmd_ui_static([str(v17_root), "send", "--json"])
        self.assertEqual(rc_ui, 0)
        ui_payload = json.loads(buf_ui.getvalue())
        self.assertEqual(ui_payload["query"], "send")
        self.assertEqual(ui_payload["decision"]["status"], "RESOLVED")

        # 4. plw check --target / --file scoping
        buf_chk_all = io.StringIO()
        with redirect_stdout(buf_chk_all):
            rc_chk_all = plw_cli.cmd_check([str(v17_root), "--all", "--json"])
        self.assertEqual(rc_chk_all, 0)
        chk_all = json.loads(buf_chk_all.getvalue())

        buf_chk_rag = io.StringIO()
        with redirect_stdout(buf_chk_rag):
            rc_chk_rag = plw_cli.cmd_check([str(v17_root), "--target", "rag", "--json"])
        self.assertEqual(rc_chk_rag, 0)
        chk_rag = json.loads(buf_chk_rag.getvalue())
        self.assertEqual(chk_rag.get("target_filter"), ["rag"])
        self.assertEqual(chk_rag.get("unfiltered_findings_count"), len(chk_all.get("findings", [])))
        self.assertEqual(chk_rag["unified_summary"]["total_findings"], len(chk_rag["findings"]))

        # 5. plw ast --file strict exact stem vs spec/directory disambiguation (via mocked codesearch)
        fake_ast_res = {
            "ok": True,
            "mode": "defs",
            "root": str(v17_root),
            "query": "App",
            "summary": {"total": 3, "shown": 3},
            "engine_result": {
                "hits": [
                    {"path": "src/app/app.ts", "line": 4, "name": "App"},
                    {"path": "src/app/app.spec.ts", "line": 2, "name": "AppSpec"},
                    {"path": "src/app/other/helper.ts", "line": 1, "name": "AppHelper"},
                ]
            },
        }
        with mock.patch("core.codesearch_engine.discover_codesearch", return_value=("/bin/fake", "test")),              mock.patch("core.codesearch_engine.run_query", return_value=fake_ast_res):
            buf_ast = io.StringIO()
            with redirect_stdout(buf_ast):
                rc_ast = plw_cli.cmd_ast(["defs", str(v17_root), "App", "--file", "app", "--json"])
        self.assertEqual(rc_ast, 0)
        ast_payload = json.loads(buf_ast.getvalue())
        self.assertEqual([h["path"] for h in ast_payload["engine_result"]["hits"]], ["src/app/app.ts"])
        self.assertEqual(ast_payload["summary"]["total"], 1)

        # 6. cycle-break registered in semantic_affordance + semantic route
        from core.semantic_affordance import describe_command, route_task_semantics
        cb_desc = describe_command("cycle-break")
        self.assertTrue(cb_desc["known"], cb_desc)
        self.assertEqual(cb_desc["command"], "cycle-break")
        cb_route = route_task_semantics("find circular dependencies in modules")
        self.assertEqual(cb_route["selection_source"], "bounded_task_signal")
        self.assertEqual(cb_route["capability"], "cycle-break")


if __name__ == "__main__":
    unittest.main()
