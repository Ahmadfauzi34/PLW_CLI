from __future__ import annotations

import os
import shutil
import tempfile
import unittest
from pathlib import Path

from codebase.cycle_break import analyze_cycle_breaks, compute_cycle_break_advisory
from codebase.mermaid_graph import generate_mermaid_diagram
from codebase.topology_analyzers import analyze_circular
from core.shared_graph import build_shared_graph
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


if __name__ == "__main__":
    unittest.main()
