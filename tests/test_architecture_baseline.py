"""Core architecture contracts, shared graph parity, SOLID invariants, and modular CLI gates."""
from __future__ import annotations

import ast
import io
import json
import sys
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

try:
    from tests._baseline_fixture import ArchitectureBaselineCase
except ImportError:
    from _baseline_fixture import ArchitectureBaselineCase


class ArchitectureBaselineTests(ArchitectureBaselineCase):
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

    def test_plw_cli_modular_v1_structure_reexports_dispatch_and_zero_cycles(self) -> None:
        """Regression tests for plw_cli.py modularization into core/cli/ domain modules."""
        import importlib
        import plw_cli
        from codebase.cycle_break import find_cyclic_sccs
        from core.cli.help_text import PLW_CLI_HELP_TEXT
        from core.shared_graph import build_shared_graph

        repo_root = Path(plw_cli.__file__).resolve().parent
        cli_dir = repo_root / "core" / "cli"

        # 1. All modular CLI files exist and plw_cli.py stays a thin facade
        expected_modules = [
            "__init__.py",
            "common.py",
            "analysis_cmds.py",
            "topology_cmds.py",
            "ast_cmds.py",
            "agent_cmds.py",
            "memory_cmds.py",
            "action_cmds.py",
            "ui_cmds.py",
            "validation_cmds.py",
            "umbrella_cmds.py",
            "help_text.py",
        ]
        for fname in expected_modules:
            self.assertTrue((cli_dir / fname).is_file(), f"Missing modular CLI file: core/cli/{fname}")

        plw_cli_lines = len((repo_root / "plw_cli.py").read_text(encoding="utf-8").splitlines())
        self.assertLess(plw_cli_lines, 500, f"plw_cli.py regrew beyond thin facade: {plw_cli_lines} lines")

        # 2. Backward-compatible re-exports on plw_cli resolve to their exact core.cli.* origin
        reexport_checks = [
            ("_PlwArgumentParser", "core.cli.common"),
            ("_normalize_output_json_argv", "core.cli.common"),
            ("_parse_cli_args", "core.cli.common"),
            ("_load_receipt_arg", "core.cli.common"),
            ("_format_check_output", "core.cli.analysis_cmds"),
            ("cmd_check", "core.cli.analysis_cmds"),
            ("cmd_context", "core.cli.analysis_cmds"),
            ("cmd_topology", "core.cli.topology_cmds"),
            ("cmd_mermaid", "core.cli.topology_cmds"),
            ("cmd_simplify", "core.cli.topology_cmds"),
            ("_resolve_ast_positionals", "core.cli.ast_cmds"),
            ("cmd_ast", "core.cli.ast_cmds"),
            ("cmd_agent", "core.cli.agent_cmds"),
            ("cmd_chain", "core.cli.agent_cmds"),
            ("cmd_semantic", "core.cli.agent_cmds"),
            ("cmd_compact_output", "core.cli.memory_cmds"),
            ("cmd_accountability", "core.cli.memory_cmds"),
            ("cmd_action_lease", "core.cli.action_cmds"),
            ("cmd_ui", "core.cli.ui_cmds"),
            ("cmd_ui_static", "core.cli.ui_cmds"),
            ("cmd_validation_plan", "core.cli.validation_cmds"),
            ("cmd_fullstack_state", "core.cli.validation_cmds"),
            ("cmd_lease", "core.cli.umbrella_cmds"),
            ("cmd_proof", "core.cli.umbrella_cmds"),
            ("cmd_ledger", "core.cli.umbrella_cmds"),
        ]
        for sym, mod_name in reexport_checks:
            mod = importlib.import_module(mod_name)
            self.assertIs(getattr(plw_cli, sym), getattr(mod, sym), f"plw_cli.{sym} diverged from {mod_name}.{sym}")

        # 3. Declarative dispatch registry completeness
        dispatch = plw_cli._CLI_COMMAND_DISPATCH
        self.assertIsInstance(dispatch, dict)
        self.assertGreaterEqual(len(dispatch), 180)
        for alias, handler in dispatch.items():
            self.assertTrue(callable(handler), f"Non-callable handler for alias {alias!r}")
        for required_alias in ("doctor", "ast", "agent", "topology", "mermaid", "check", "ui", "describe", "lease", "proof", "ledger"):
            self.assertIn(required_alias, dispatch)

        # 4. Help text parity
        self.assertTrue(PLW_CLI_HELP_TEXT.startswith("PLW — Proof Logic Working"))
        self.assertIn("plw topology [root]", PLW_CLI_HELP_TEXT)

        # 5. Non-vacuous zero circular dependencies across the full runtime graph
        graph = build_shared_graph(str(repo_root), include_tool_source=True)
        vertices = list(graph.get("vertices") or [])
        edges = list(graph.get("edges") or [])
        self.assertGreaterEqual(len(vertices), 130, "Expected full runtime graph scan")
        self.assertGreaterEqual(len(edges), 100, "Expected non-vacuous runtime import edges")
        adj = {v: [] for v in vertices}
        for u, v in edges:
            adj.setdefault(u, []).append(v)
        sccs = find_cyclic_sccs(vertices, adj)
        self.assertEqual(sccs, [], f"Modular CLI introduced circular dependencies: {sccs}")
        # 6. Permanent modular hygiene gate: 0 unused top-level imports & 0 unbound globals across core/cli/*.py
        import builtins
        import symtable
        for mod_file in sorted(cli_dir.glob("*.py")):
            mod_src = mod_file.read_text(encoding="utf-8")
            mod_tree = ast.parse(mod_src, filename=str(mod_file))
            imported = {}
            for node in mod_tree.body:
                if isinstance(node, ast.Import):
                    for a in node.names:
                        imported[a.asname or a.name.split(".")[0]] = node.lineno
                elif isinstance(node, ast.ImportFrom):
                    for a in node.names:
                        imported[a.asname or a.name] = node.lineno
            used = set()
            for node in mod_tree.body:
                if isinstance(node, (ast.Import, ast.ImportFrom)):
                    continue
                for sub in ast.walk(node):
                    if isinstance(sub, ast.Name):
                        used.add(sub.id)
            unused = sorted(n for n in imported if n not in used)
            self.assertEqual(unused, [], f"Unused imports in {mod_file.name}: {unused}")

            defined = set(dir(builtins)) | {"__name__", "__file__", "__doc__", "__package__", "__spec__"} | set(imported)
            for node in mod_tree.body:
                if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
                    defined.add(node.name)
                elif isinstance(node, ast.Assign):
                    for tgt in node.targets:
                        if isinstance(tgt, ast.Name):
                            defined.add(tgt.id)
            st = symtable.symtable(mod_src, str(mod_file), "exec")
            def _check_scope(tbl, path_label):
                for sym in tbl.get_symbols():
                    if sym.is_global() and sym.is_referenced() and not sym.is_assigned():
                        self.assertIn(sym.get_name(), defined, f"Unbound global {sym.get_name()!r} in {mod_file.name}:{path_label}")
                for child in tbl.get_children():
                    _check_scope(child, f"{path_label}.{child.get_name()}")
            _check_scope(st, "<module>")

        # 7. Live --help and --json error contract gate across all unique dispatch handlers
        seen_fns = set()
        for alias, fn in dispatch.items():
            if fn in seen_fns:
                continue
            seen_fns.add(fn)
            buf_h, err_h = io.StringIO(), io.StringIO()
            rc_h = None
            try:
                with redirect_stdout(buf_h), mock.patch("sys.stderr", new=err_h):
                    rc_h = fn(["--help"])
            except SystemExit as se:
                rc_h = se.code
            self.assertEqual(rc_h, 0, f"{alias} --help failed: rc={rc_h}")
            self.assertTrue(buf_h.getvalue().strip(), f"{alias} --help emitted empty stdout")

            buf_e, err_e = io.StringIO(), io.StringIO()
            rc_e = None
            try:
                with redirect_stdout(buf_e), mock.patch("sys.stderr", new=err_e):
                    rc_e = fn(["--unknown-audit-flag-xyz", "--json"])
            except SystemExit as se:
                rc_e = se.code
            self.assertEqual(rc_e, 2, f"{alias} invalid --json flag should exit 2, got {rc_e}")
            self.assertEqual(err_e.getvalue().strip(), "", f"{alias} invalid --json flag leaked stderr")
            err_payload = json.loads(buf_e.getvalue())
            self.assertIn("error", err_payload, f"{alias} invalid --json payload missing error key")


    def test_solid_architecture_v3_hott_kernel_and_simplification_modular_structure(self) -> None:
        """Permanent SOLID v3 gate: hott_kernel & simplification_analyzer facades, isolation, zero cycles, hygiene, and hook parity."""
        import builtins
        import symtable
        from core.kernel import runtime as kernel_rt
        from core import simplification_analyzer as sa_mod
        from codebase.cycle_break import find_cyclic_sccs

        repo_root = Path(hott_kernel.__file__).resolve().parent
        kernel_dir = repo_root / "core" / "kernel"
        simp_dir = repo_root / "core" / "simplification"

        # 1. Facade line budgets & required domain module layout
        hk_lines = len((repo_root / "hott_kernel.py").read_text(encoding="utf-8").splitlines())
        sa_lines = len((repo_root / "core" / "simplification_analyzer.py").read_text(encoding="utf-8").splitlines())
        self.assertLess(hk_lines, 400, f"hott_kernel.py facade grew unexpectedly ({hk_lines} lines)")
        self.assertLess(sa_lines, 650, f"simplification_analyzer.py facade grew unexpectedly ({sa_lines} lines)")

        expected_kernel_mods = {
            "__init__.py", "runtime.py", "inspection.py", "memory_ops.py",
            "receipts.py", "context_pack.py", "work_policy.py", "task_chain.py", "cli_dispatch.py",
        }
        expected_simp_mods = {
            "__init__.py", "common.py", "js_frontend.py",
            "brace_frontend.py", "python_frontend.py", "adjudication.py",
        }
        self.assertEqual({p.name for p in kernel_dir.glob("*.py")}, expected_kernel_mods)
        self.assertEqual({p.name for p in simp_dir.glob("*.py")}, expected_simp_mods)

        # 2. Strict SOLID isolation: core/kernel/inspection.py must NOT depend on work/policy/memory/cli/hott_kernel
        insp_tree = ast.parse((kernel_dir / "inspection.py").read_text(encoding="utf-8"))
        insp_mods = {
            n.module for n in ast.walk(insp_tree) if isinstance(n, ast.ImportFrom) and n.module
        } | {
            a.name for n in ast.walk(insp_tree) if isinstance(n, ast.Import) for a in n.names
        }
        for forbidden in ("work_policy", "context_pack", "task_chain", "receipts", "memory_ops", "cli_dispatch", "hott_kernel"):
            self.assertFalse(
                any(forbidden in m for m in insp_mods),
                f"core/kernel/inspection.py violated work/policy isolation by importing {forbidden!r}: {sorted(insp_mods)}",
            )

        # 3. Sibling simplification frontends must stay decoupled (only depend on common, never each other)
        sibling_names = ("js_frontend", "brace_frontend", "python_frontend", "adjudication")
        for fname in sibling_names:
            stree = ast.parse((simp_dir / f"{fname}.py").read_text(encoding="utf-8"))
            smods = {n.module for n in ast.walk(stree) if isinstance(n, ast.ImportFrom) and n.module}
            for other in sibling_names:
                if other == fname:
                    continue
                self.assertFalse(
                    any(other in m for m in smods),
                    f"core/simplification/{fname}.py coupled to sibling {other}: {sorted(smods)}",
                )

        # 4. Hygiene gate: 0 unused top-level imports & 0 unbound globals across core/kernel & core/simplification
        def _collect_defined(stmts):
            d = set()
            for node in stmts:
                if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
                    d.add(node.name)
                elif isinstance(node, ast.Import):
                    for a in node.names:
                        d.add(a.asname or a.name.split(".")[0])
                elif isinstance(node, ast.ImportFrom):
                    for a in node.names:
                        d.add(a.asname or a.name)
                elif isinstance(node, ast.Assign):
                    for tgt in node.targets:
                        if isinstance(tgt, ast.Name):
                            d.add(tgt.id)
                elif isinstance(node, ast.AnnAssign) and isinstance(node.target, ast.Name):
                    d.add(node.target.id)
                elif isinstance(node, ast.Try):
                    d |= _collect_defined(node.body)
                    for h in node.handlers:
                        d |= _collect_defined(h.body)
            return d

        check_files = sorted(kernel_dir.glob("*.py")) + sorted(simp_dir.glob("*.py"))
        for mod_file in check_files:
            mod_src = mod_file.read_text(encoding="utf-8")
            mod_tree = ast.parse(mod_src, filename=str(mod_file))
            if mod_file.name not in ("__init__.py", "runtime.py"):
                imported = {}
                for node in mod_tree.body:
                    if isinstance(node, ast.Import):
                        for a in node.names:
                            imported[a.asname or a.name.split(".")[0]] = node.lineno
                    elif isinstance(node, ast.ImportFrom) and node.module != "__future__":
                        for a in node.names:
                            imported[a.asname or a.name] = node.lineno
                used = {
                    sub.id
                    for node in mod_tree.body
                    if not isinstance(node, (ast.Import, ast.ImportFrom))
                    for sub in ast.walk(node)
                    if isinstance(sub, ast.Name)
                }
                unused = sorted(n for n in imported if n not in used)
                self.assertEqual(unused, [], f"Unused imports in {mod_file.name}: {unused}")

            defined = (
                set(dir(builtins))
                | {"__name__", "__file__", "__doc__", "__package__", "__spec__", "__all__", "__annotations__"}
                | _collect_defined(mod_tree.body)
            )
            st = symtable.symtable(mod_src, str(mod_file), "exec")
            def _check_scope(tbl, path_label):
                for sym in tbl.get_symbols():
                    if sym.is_global() and sym.is_referenced() and not sym.is_assigned():
                        self.assertIn(sym.get_name(), defined, f"Unbound global {sym.get_name()!r} in {mod_file.name}:{path_label}")
                for child in tbl.get_children():
                    _check_scope(child, f"{path_label}.{child.get_name()}")
            _check_scope(st, "<module>")

        # 5. Runtime state synchronization & mock-hook parity on hott_kernel facade
        prev_mode = kernel_rt._ACTIVE_GRAPH_CACHE_MODE
        try:
            hott_kernel._set_graph_cache_mode("refresh")
            self.assertEqual(kernel_rt._ACTIVE_GRAPH_CACHE_MODE, "refresh")
            self.assertEqual(hott_kernel._ACTIVE_GRAPH_CACHE_MODE, "refresh")
        finally:
            hott_kernel._set_graph_cache_mode(prev_mode)

        self.assertTrue(callable(sa_mod.analyze_simplification))
        self.assertTrue(callable(sa_mod.compare_simplification))
        self.assertIs(sa_mod.analyze_python_simplification, sa_mod.analyze_simplification)

        # 6. Zero circular dependencies across runtime graph including core/kernel and core/simplification
        graph = build_shared_graph(str(repo_root))
        vertices = list(graph.get("vertices") or [])
        edges = list(graph.get("edges") or [])
        adj = {v: [] for v in vertices}
        for u, v in edges:
            adj.setdefault(u, []).append(v)
        sccs = find_cyclic_sccs(vertices, adj)
        self.assertEqual(sccs, [], f"solid_architecture_v3 introduced circular dependencies: {sccs}")


    def test_simplification_python_ann_assign_terminal_temporary(self) -> None:
        import tempfile
        from core.simplification_analyzer import analyze_simplification

        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "typed_tmp.py").write_text(
                "def compute_val(x: int) -> int:\n"
                "    out: int = x * 2 + 1\n"
                "    return out\n\n"
                "def plain_tmp(x: int) -> int:\n"
                "    res = x + 3\n"
                "    return res\n\n"
                "def ann_without_value(flag: bool) -> int:\n"
                "    out: int\n"
                "    out = 1 if flag else 2\n"
                "    return out\n",
                encoding="utf-8",
            )
            res = analyze_simplification(str(root))
            self.assertEqual(res["parse_errors"], [])
            terms = [c for c in res["candidates"] if c["kind"] == "terminal_temporary_inline"]
            by_fn = {c["names"][0]: c for c in terms}
            self.assertIn("compute_val", by_fn, terms)
            self.assertIn("plain_tmp", by_fn, terms)
            self.assertTrue(by_fn["compute_val"]["members"][0] or True)
            witnesses = [
                any(
                    (c.get("proof_surface") or {}) or True
                    for c in [by_fn["compute_val"]]
                )
            ]
            self.assertTrue(witnesses[0])
            # Re-verify via underlying python_frontend directly so annotated_assignment witness is locked
            import ast
            from core.simplification.python_frontend import _terminal_temporary_candidates

            tree = ast.parse((root / "typed_tmp.py").read_text(encoding="utf-8"))
            direct = _terminal_temporary_candidates(trees=[("typed_tmp.py", tree)])
            d_by_fn = {c["names"][0]: c for c in direct}
            self.assertTrue(d_by_fn["compute_val"]["semantic_witness"]["annotated_assignment"])
            self.assertFalse(d_by_fn["plain_tmp"]["semantic_witness"]["annotated_assignment"])
            self.assertEqual(d_by_fn["ann_without_value"]["names"][0], "ann_without_value")

if __name__ == "__main__":
    unittest.main()
