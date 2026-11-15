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


if __name__ == "__main__":
    unittest.main()
