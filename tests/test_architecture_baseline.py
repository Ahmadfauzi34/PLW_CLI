from __future__ import annotations

import ast
import io
import json
import os
import shutil
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

from codebase import cycle_break as cycle_break_mod
from codebase.cycle_break import analyze_cycle_breaks, compute_cycle_break_advisory
from codebase.mermaid_graph import generate_mermaid_diagram
from codebase.topology_analyzers import analyze_circular
from core.shared_graph import _LANGUAGE_IMPORT_RESOLVERS, build_shared_graph
import fullstack
import hott_kernel
import language_semantics


class ArchitectureBaselineTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = Path(tempfile.mkdtemp(prefix="plw_arch_base_"))
        self._prev_plw_home = os.environ.get("PLW_HOME")
        os.environ["PLW_HOME"] = str(self._tmp / "plw_home")

        self.proj = self._tmp / "sample_py"
        (self.proj / "src" / "app").mkdir(parents=True)
        (self.proj / "src" / "app" / "__init__.py").write_text(
            "from .router import route\n", encoding="utf-8"
        )
        (self.proj / "src" / "app" / "router.py").write_text(
            "from app.service import handle\nfrom .utils import fmt\n\n"
            "def route(x):\n    return fmt(handle(x))\n",
            encoding="utf-8",
        )
        (self.proj / "src" / "app" / "service.py").write_text(
            "from .utils import HelperState, fmt\n\n"
            "def handle(x):\n    return fmt(HelperState.VALUE)\n",
            encoding="utf-8",
        )
        (self.proj / "src" / "app" / "utils.py").write_text(
            "from app.service import handle\n\n"
            "class HelperState:\n    VALUE = 'ok'\n\n"
            "def fmt(v):\n    return str(v)\n",
            encoding="utf-8",
        )
        (self.proj / "main.py").write_text(
            "#!/usr/bin/env python3\nfrom app import route\n\n"
            "if __name__ == '__main__':\n    print(route('go'))\n",
            encoding="utf-8",
        )

    def tearDown(self) -> None:
        if self._prev_plw_home is None:
            os.environ.pop("PLW_HOME", None)
        else:
            os.environ["PLW_HOME"] = self._prev_plw_home
        shutil.rmtree(self._tmp, ignore_errors=True)

    def test_cycle_break_matches_circular_analyzer(self) -> None:
        graph = build_shared_graph(str(self.proj))
        circular = analyze_circular(graph)
        expected_cycles = [f["cycle"] for f in circular["findings"]]
        self.assertEqual(len(expected_cycles), 1)

        res_auto = analyze_cycle_breaks(graph)
        res_explicit = analyze_cycle_breaks(graph, expected_cycles)
        self.assertEqual(res_auto["summary"], res_explicit["summary"])
        self.assertEqual(res_auto["advisories"], res_explicit["advisories"])
        self.assertEqual(res_auto["summary"]["total_cycles"], 1)
        self.assertEqual(res_auto["summary"]["total_cuts_required"], 1)

        adv = compute_cycle_break_advisory(graph, expected_cycles)
        self.assertEqual(adv["cycle_break_plan"], res_auto["summary"])

    def test_cycle_break_custom_cycle_finder_injection(self) -> None:
        fake_graph = {
            "vertices": ["a.py", "b.py"],
            "edges": [("a.py", "b.py"), ("b.py", "a.py")],
            "node_metadata": {
                "a.py": {"fan_in": 1, "fan_out": 1, "is_entrypoint": False},
                "b.py": {"fan_in": 1, "fan_out": 1, "is_entrypoint": False},
            },
            "file_map": {
                "a.py": "from b import B\n",
                "b.py": "from a import A\n",
            },
            "edge_natures": {},
            "type_shapes": {},
        }
        called = []

        def fake_finder(sg):
            called.append(len(sg["vertices"]))
            return [["a.py", "b.py", "a.py"]]

        res = analyze_cycle_breaks(fake_graph, cycle_finder=fake_finder)
        self.assertEqual(called, [2])
        self.assertEqual(res["summary"]["total_cycles"], 1)
        self.assertEqual(res["summary"]["total_cuts_required"], 1)

    def test_mermaid_depths_and_validation_contracts(self) -> None:
        for depth in (1, 2, 3, 4):
            kwargs = {"depth": depth, "direction": "TD"}
            if depth == 4:
                kwargs["focus"] = "src/app/utils.py"
            elif depth == 3:
                kwargs["focus"] = "src/app"
            res = hott_kernel.kernel_mermaid(str(self.proj), **kwargs)
            self.assertEqual(res["schema_version"], "1.0.0-mermaid-graph")
            self.assertEqual(res["depth"], depth)
            self.assertEqual(res["direction"], "TD")
            self.assertIn("mermaid_source", res)
            self.assertTrue(res["mermaid_source"].startswith("flowchart TD"))
            self.assertGreaterEqual(res["summary"]["rendered_nodes"], 1)

        bad_depth = hott_kernel.kernel_mermaid(str(self.proj), depth=9)
        self.assertEqual(bad_depth.get("error_code"), "invalid_mermaid_depth")
        bad_dir = hott_kernel.kernel_mermaid(str(self.proj), direction="XX")
        self.assertEqual(bad_dir.get("error_code"), "invalid_mermaid_direction")
        bad_max = hott_kernel.kernel_mermaid(str(self.proj), max_nodes=2)
        self.assertEqual(bad_max.get("error_code"), "invalid_mermaid_max_nodes")

    def test_mermaid_graph_provider_injection(self) -> None:
        root_abs = "/virtual/repo"
        a_abs = f"{root_abs}/pkg/a.py"
        b_abs = f"{root_abs}/pkg/b.py"
        fake_graph = {
            "vertices": [a_abs, b_abs],
            "edges": [(a_abs, b_abs)],
            "edge_natures": {f"{a_abs} -> {b_abs}": "runtime"},
            "node_metadata": {
                a_abs: {"fan_in": 0, "fan_out": 1, "is_entrypoint": True, "language": "python", "lines": 10},
                b_abs: {"fan_in": 1, "fan_out": 0, "is_entrypoint": False, "language": "python", "lines": 20},
            },
            "cache": {"graph_content_signature": "fake-sig-123"},
        }
        res = generate_mermaid_diagram(
            root_abs,
            depth=4,
            focus="pkg/b.py",
            graph=fake_graph,
            orientation_builder=lambda g, **kw: {"inventory": {"files": 2, "internal_import_edges": 1, "entrypoints": 1, "cyclic_sccs": 0}, "strongly_connected_components": []},
            outline_querier=lambda g, p: {"declarations": [{"kind": "function", "name": "work", "line": 3, "exported": True}], "decorators": [], "stats": {"total_lines": 20}},
            impact_querier=lambda g, p: {"direct_upstream": [], "direct_downstream": [a_abs], "affected_entrypoints": [a_abs], "circular_references": [], "change_risk_level": "medium", "target_fan_in": 1, "target_fan_out": 0},
        )
        self.assertEqual(res["decision"], "MERMAID_GRAPH_READY")
        self.assertEqual(res["graph_signature"], "fake-sig-123")
        self.assertEqual(res["summary"]["target_file"], "pkg/b.py")
        self.assertIn("work (L3)", res["mermaid_source"])

    def test_work_frame_only_characterization(self) -> None:
        res = hott_kernel.kernel_work(
            str(self.proj),
            "Refactor cycle in utils",
            target_files=["src/app/utils.py"],
            depth="deep",
            frame_only=True,
        )
        self.assertTrue(res["frame_only"])
        self.assertEqual(res["work_status"], "ready")
        self.assertEqual(res["target_resolution"]["resolution"]["state"], "RESOLVED")
        self.assertNotIn("inline_contract", res["skill_handoff"])
        delivery = res["agent_decision_frame"]["skill_handoff"]["inline_contract_delivery"]
        self.assertEqual(delivery["status"], "OMITTED_IN_FRAME_ONLY")
        self.assertEqual(res["detail_contract"]["level"], "frame_only")

    def test_python_type_checking_imports_are_type_only(self) -> None:
        py_src = (
            "from typing import TYPE_CHECKING\n"
            "import typing\n"
            "if TYPE_CHECKING:\n"
            "    from app.types_mod import Thing\n"
            "if typing.TYPE_CHECKING:\n"
            "    import app.other_mod\n"
            "else:\n"
            "    import app.runtime_mod\n"
        )
        parsed = language_semantics.parse_python_imports(py_src)
        by_raw = {item["raw_import"]: item["import_nature"] for item in parsed}
        self.assertEqual(by_raw.get("app/types_mod"), "type_only")
        self.assertEqual(by_raw.get("app/other_mod"), "type_only")
        self.assertEqual(by_raw.get("app/runtime_mod"), "runtime")

        # Integration check via SharedGraph: TYPE_CHECKING cycle edge becomes type_only
        (self.proj / "src" / "app" / "type_a.py").write_text(
            "from typing import TYPE_CHECKING\n"
            "if TYPE_CHECKING:\n"
            "    from .type_b import B\n"
            "class A:\n    pass\n",
            encoding="utf-8",
        )
        (self.proj / "src" / "app" / "type_b.py").write_text(
            "from typing import TYPE_CHECKING\n"
            "if TYPE_CHECKING:\n"
            "    from .type_a import A\n"
            "class B:\n    pass\n",
            encoding="utf-8",
        )
        graph = build_shared_graph(str(self.proj))
        a_abs = str((self.proj / "src" / "app" / "type_a.py").resolve())
        b_abs = str((self.proj / "src" / "app" / "type_b.py").resolve())
        self.assertEqual(graph["edge_natures"].get(f"{a_abs} -> {b_abs}"), "type_only")
        self.assertEqual(graph["edge_natures"].get(f"{b_abs} -> {a_abs}"), "type_only")

    def test_fullstack_barrel_exports_intact(self) -> None:
        for attr in (
            "build_validation_observation_payload",
            "normalize_proof_obligation_set",
            "verify_execution_backed_scenario_evidence",
            "load_validation_result_evidence_acceptance_receipt",
            "build_targeted_geometric_impact",
            "project_execution_backed_proof_obligations",
            "verify_execution_backed_proof_coverage",
        ):
            self.assertTrue(hasattr(fullstack, attr), f"missing export: {attr}")

    def test_shared_python_type_checking_guard_is_single_source_of_truth(self) -> None:
        self.assertIs(
            cycle_break_mod._is_type_checking_guard,
            language_semantics.is_python_type_checking_guard,
        )
        mod_direct = ast.parse("if TYPE_CHECKING:\n    pass\n")
        mod_attr = ast.parse("if typing.TYPE_CHECKING:\n    pass\n")
        mod_other = ast.parse("if DEBUG:\n    pass\n")
        self.assertTrue(language_semantics.is_python_type_checking_guard(mod_direct.body[0].test))
        self.assertTrue(language_semantics.is_python_type_checking_guard(mod_attr.body[0].test))
        self.assertFalse(language_semantics.is_python_type_checking_guard(mod_other.body[0].test))

    def test_shared_graph_language_resolvers_and_snapshot_parity(self) -> None:
        self.assertEqual(set(_LANGUAGE_IMPORT_RESOLVERS), {"kotlin", "rust", "python"})

        poly = self._tmp / "polyglot_repo"
        poly.mkdir(parents=True)
        (poly / "tsconfig.json").write_text(
            json.dumps({"compilerOptions": {"baseUrl": ".", "paths": {"@core/*": ["src/core/*"]}}}),
            encoding="utf-8",
        )
        (poly / "src" / "core").mkdir(parents=True)
        (poly / "src" / "core" / "util.ts").write_text("export const x = 1;\n", encoding="utf-8")
        (poly / "src" / "app.ts").write_text("import { x } from '@core/util';\nconsole.log(x);\n", encoding="utf-8")

        (poly / "kotlin" / "com" / "example").mkdir(parents=True)
        (poly / "kotlin" / "com" / "example" / "Models.kt").write_text(
            "package com.example\nclass Model\n", encoding="utf-8"
        )
        (poly / "kotlin" / "com" / "example" / "Main.kt").write_text(
            "package com.example\nimport com.other.Ext\nimport com.example.*\nfun run(m: Model) = m\n",
            encoding="utf-8",
        )

        (poly / "rust_pkg" / "src").mkdir(parents=True)
        (poly / "rust_pkg" / "Cargo.toml").write_text(
            '[package]\nname = "demo-pkg"\nversion = "0.1.0"\n', encoding="utf-8"
        )
        (poly / "rust_pkg" / "src" / "helper.rs").write_text("pub fn ping() {}\n", encoding="utf-8")
        (poly / "rust_pkg" / "src" / "lib.rs").write_text(
            "mod helper;\nuse crate::helper::ping;\nuse std::fmt;\npub fn go() { ping(); }\n",
            encoding="utf-8",
        )

        g_live = build_shared_graph(str(poly))
        snapshot = {
            fp: {"content": content, "imports": []}
            for fp, content in g_live["file_map"].items()
        }
        g_snap = build_shared_graph(str(poly), _file_snapshot=snapshot)

        self.assertEqual(g_live["edges"], g_snap["edges"])
        self.assertEqual(g_live["edge_natures"], g_snap["edge_natures"])
        app_ts = str((poly / "src" / "app.ts").resolve())
        util_ts = str((poly / "src" / "core" / "util.ts").resolve())
        self.assertIn((app_ts, util_ts), g_live["edges"])
        kt_main = str((poly / "kotlin" / "com" / "example" / "Main.kt").resolve())
        kt_model = str((poly / "kotlin" / "com" / "example" / "Models.kt").resolve())
        self.assertEqual(g_live["edge_natures"].get(f"{kt_main} -> {kt_model}"), "same_package")
        rs_lib = str((poly / "rust_pkg" / "src" / "lib.rs").resolve())
        rs_helper = str((poly / "rust_pkg" / "src" / "helper.rs").resolve())
        self.assertIn((rs_lib, rs_helper), g_live["edges"])

    def test_hott_kernel_cli_dispatch_covers_all_public_modes(self) -> None:
        self.assertEqual(
            set(hott_kernel._KERNEL_CLI_DISPATCH),
            set(hott_kernel.PUBLIC_KERNEL_MODES),
        )

        def _run_cli(*args: str):
            buf = io.StringIO()
            exit_code = 0
            with mock.patch.object(sys, "argv", ["hott_kernel.py", *args]), redirect_stdout(buf):
                try:
                    hott_kernel.main()
                except SystemExit as exc:
                    exit_code = int(exc.code) if isinstance(exc.code, int) else 1
            return exit_code, json.loads(buf.getvalue())

        code, help_payload = _run_cli("--help")
        self.assertEqual(code, 0)
        self.assertEqual(help_payload["tool"], "hott_kernel")
        self.assertIn("[--frame-only]", help_payload["usage"]["work"])

        code, mode_help = _run_cli("work", "--help")
        self.assertEqual(code, 0)
        self.assertIn("[--frame-only]", mode_help["usage"])

        code, analyzers_payload = _run_cli("analyzers")
        self.assertEqual(code, 0)
        self.assertIn("topo.circular", analyzers_payload["available_analyzers"])

        code, bad_mode = _run_cli("nonexistent_mode")
        self.assertEqual(code, 2)
        self.assertEqual(bad_mode["error_code"], "unknown_mode")

        code, bad_out = _run_cli("analyze", str(self.proj), "--output", "bogus")
        self.assertEqual(code, 2)
        self.assertEqual(bad_out["error_code"], "invalid_output_mode")

        code, work_payload = _run_cli(
            "work", "Audit utils cycle", str(self.proj),
            "--target", "src/app/utils.py", "--frame-only",
        )
        self.assertEqual(code, 0)
        self.assertTrue(work_payload["frame_only"])
        self.assertIn("receipt", work_payload)

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

if __name__ == "__main__":
    unittest.main()
