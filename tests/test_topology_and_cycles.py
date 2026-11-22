"""Topology soundness, cycle-breaking, Kotlin/Android & Rust graph semantics, type_only consistency, and Mermaid quoting."""
from __future__ import annotations

import io
import json
import shutil
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from codebase.cycle_break import analyze_cycle_breaks, compute_cycle_break_advisory
from codebase.topology_analyzers import analyze_circular
from core.shared_graph import _LANGUAGE_IMPORT_RESOLVERS, build_shared_graph

try:
    from tests._baseline_fixture import ArchitectureBaselineCase
except ImportError:
    from _baseline_fixture import ArchitectureBaselineCase


class TopologyAndCyclesTests(ArchitectureBaselineCase):
    def test_topo_circular_opt_v1_soundness_semantics_and_scale(self) -> None:
        """Regression tests for topo.circular Iterative Tarjan+Johnson, type_only semantics, and scale."""
        from codebase.cycle_break import (
            analyze_cycle_breaks,
            find_directed_cycles_with_metadata,
        )
        from codebase.topology_analyzers import analyze_circular, query_impact
        from core.target_topology import _cyclic_sccs

        # 1. Overlapping figure-8 cycles: old 3-color DFS missed a->c->a behind BLACK node c
        fig8 = {
            "vertices": ["a.ts", "b.ts", "c.ts"],
            "edges": [("a.ts", "b.ts"), ("b.ts", "c.ts"), ("c.ts", "a.ts"), ("a.ts", "c.ts")],
            "edge_natures": {
                "a.ts -> b.ts": "type_only",
                "b.ts -> c.ts": "type_only",
                "c.ts -> a.ts": "runtime",
                "a.ts -> c.ts": "runtime",
            },
            "node_metadata": {v: {"fan_in": 1, "fan_out": 1} for v in ["a.ts", "b.ts", "c.ts"]},
            "file_map": {},
            "type_shapes": {},
        }
        circ1 = analyze_circular(fig8)
        self.assertEqual(
            [f["cycle"] for f in circ1["findings"]],
            [["a.ts", "c.ts", "a.ts"], ["a.ts", "b.ts", "c.ts", "a.ts"]],
        )
        self.assertEqual(circ1["findings"][0]["cycle_nature"], "runtime")
        self.assertEqual(circ1["findings"][0]["severity"], "high")
        self.assertEqual(circ1["findings"][1]["cycle_nature"], "type_only")
        self.assertEqual(circ1["findings"][1]["severity"], "low")
        self.assertEqual(circ1["summary"]["runtime_cycles"], 1)
        self.assertEqual(circ1["summary"]["type_only_cycles"], 1)
        self.assertEqual(circ1["summary"]["cyclic_scc_count"], 1)
        self.assertFalse(circ1["summary"]["truncated"])
        # Shared single source of truth with analyze_cycle_breaks
        self.assertEqual(analyze_cycle_breaks(fig8)["summary"], circ1["summary"]["cycle_break_plan"])

        # 2. Mixed runtime + type_only edge breaks runtime circularity (compiler-elided)
        mixed = {
            "vertices": ["a.ts", "b.ts"],
            "edges": [("a.ts", "b.ts"), ("b.ts", "a.ts")],
            "edge_natures": {"a.ts -> b.ts": "runtime", "b.ts -> a.ts": "type_only"},
            "node_metadata": {v: {"fan_in": 1, "fan_out": 1} for v in ["a.ts", "b.ts"]},
            "file_map": {},
            "type_shapes": {},
        }
        circ2 = analyze_circular(mixed)
        self.assertEqual(len(circ2["findings"]), 1)
        f2 = circ2["findings"][0]
        self.assertTrue(f2["is_type_only"])
        self.assertEqual(f2["cycle_nature"], "type_only")
        self.assertEqual(f2["severity"], "low")
        self.assertEqual(f2["type_only_edge_count"], 1)
        self.assertEqual(f2["runtime_edge_count"], 1)

        # 3. Exact combinatorics on K_4 (20 elementary cycles) and K_5 exact boundary (84)
        k4v = [f"v{i}.ts" for i in range(4)]
        k4g = {
            "vertices": k4v,
            "edges": [(u, v) for u in k4v for v in k4v if u != v],
            "edge_natures": {},
            "node_metadata": {},
            "file_map": {},
            "type_shapes": {},
        }
        rk4 = analyze_circular(k4g)
        self.assertEqual(rk4["summary"]["total_cycles"], 20)
        self.assertFalse(rk4["summary"]["truncated"])

        k5v = [f"v{i}.ts" for i in range(5)]
        k5g = {
            "vertices": k5v,
            "edges": [(u, v) for u in k5v for v in k5v if u != v],
            "edge_natures": {},
            "node_metadata": {},
            "file_map": {},
            "type_shapes": {},
        }
        exact5 = find_directed_cycles_with_metadata(k5g, max_cycles_per_scc=84, max_total_cycles=200)
        self.assertEqual(len(exact5["cycles"]), 84)
        self.assertFalse(exact5["truncated"])
        capped5 = find_directed_cycles_with_metadata(k5g, max_cycles_per_scc=64, max_total_cycles=200)
        self.assertEqual(len(capped5["cycles"]), 64)
        self.assertTrue(capped5["truncated"])

        # 4. Bounded dense mega-SCC K_10 prioritizes 2-cycles and preserves exact files_in_cycles
        k10v = [f"n{i:02d}.ts" for i in range(10)]
        k10g = {
            "vertices": k10v,
            "edges": [(u, v) for u in k10v for v in k10v if u != v],
            "edge_natures": {},
            "node_metadata": {},
            "file_map": {},
            "type_shapes": {},
        }
        rk10 = analyze_circular(k10g)
        self.assertTrue(rk10["summary"]["truncated"])
        self.assertEqual(rk10["summary"]["total_cycles"], 64)
        self.assertEqual(rk10["summary"]["files_in_cycles"], 10)
        self.assertIn("covers_all_graph_cycles", rk10["summary"])
        two_cycles = [f for f in rk10["findings"] if f["cycle_length"] == 2]
        self.assertEqual(len(two_cycles), 45)

        # 5. Deep 1500-node DAG & 1200-node cyclic ring+chord: zero RecursionError in both Tarjan & Johnson
        dag_v = [f"d{i:04d}.ts" for i in range(1500)]
        dag_e = [(dag_v[i], dag_v[i + 1]) for i in range(1499)]
        dag_g = {"vertices": dag_v, "edges": dag_e, "edge_natures": {}, "node_metadata": {}, "file_map": {}, "type_shapes": {}}
        self.assertEqual(analyze_circular(dag_g)["summary"]["total_cycles"], 0)
        self.assertEqual(_cyclic_sccs(dag_v, dag_e), [])

        ring_v = [f"r{i:04d}.ts" for i in range(1200)]
        ring_e = [(ring_v[i], ring_v[(i + 1) % 1200]) for i in range(1200)] + [(ring_v[600], ring_v[0])]
        ring_g = {"vertices": ring_v, "edges": ring_e, "edge_natures": {}, "node_metadata": {}, "file_map": {}, "type_shapes": {}}
        r_ring = analyze_circular(ring_g)
        self.assertEqual([f["cycle_length"] for f in r_ring["findings"]], [601, 1200])
        self.assertEqual(r_ring["summary"]["files_in_cycles"], 1200)
        self.assertEqual(len(_cyclic_sccs(ring_v, ring_e)), 1)

        # 6. Self-loop + same_package exclusion + exact target filter in query_impact (a.ts != ba.ts)
        sl_g = {
            "vertices": ["a.ts", "ba.ts", "c.ts", "kt1.kt", "kt2.kt"],
            "edges": [
                ("a.ts", "a.ts"),
                ("ba.ts", "c.ts"),
                ("c.ts", "ba.ts"),
                ("kt1.kt", "kt2.kt"),
                ("kt2.kt", "kt1.kt"),
            ],
            "edge_natures": {
                "kt1.kt -> kt2.kt": "same_package",
                "kt2.kt -> kt1.kt": "same_package",
            },
            "node_metadata": {v: {"fan_in": 1, "fan_out": 1} for v in ["a.ts", "ba.ts", "c.ts", "kt1.kt", "kt2.kt"]},
            "file_map": {},
            "type_shapes": {},
        }
        r_sl = analyze_circular(sl_g)
        self.assertEqual([f["cycle"] for f in r_sl["findings"]], [["a.ts", "a.ts"], ["ba.ts", "c.ts", "ba.ts"]])
        imp_a = query_impact(sl_g, "a.ts")
        self.assertEqual(imp_a["cycle_break_advisory"]["total_cycles"], 1)

    def test_calibration_kotlin_android_and_rust_v19(self) -> None:
        """Regression suite for Kotlin Android and Rust graph, cycle-break, outline, mermaid, and topology calibration."""
        import tempfile
        import plw_cli
        from codebase.cycle_break import _suggest_leaf_module, compute_cycle_break_advisory
        from codebase.topology_analyzers import analyze_circular, query_outline
        from core.portable_agent import dynamic_context
        from core.shared_graph import build_shared_graph
        from core.target_topology import _external_package, build_target_topology_orientation
        from language_semantics import rust_module_path_of

        tmp_ctx = tempfile.TemporaryDirectory()
        self.addCleanup(tmp_ctx.cleanup)
        repo = Path(tmp_ctx.name) / "poly_cal_repo"
        repo.mkdir(parents=True, exist_ok=True)

        # 1. Rust crate layout with nested src/rpc/tests/{mod,protocol}.rs, pub use, private super::, inline mod tests
        (repo / "Cargo.toml").write_text('[package]\nname = "cal-demo"\nversion = "0.1.0"\n', encoding="utf-8")
        (repo / "src" / "config").mkdir(parents=True)
        (repo / "src" / "rpc" / "tests").mkdir(parents=True)
        (repo / "tests").mkdir(parents=True)

        (repo / "src" / "lib.rs").write_text(
            "pub mod config;\npub mod fs;\npub mod rpc;\n",
            encoding="utf-8",
        )
        (repo / "src" / "fs.rs").write_text(
            "pub fn read_cfg() {}\n"
            "#[cfg(test)]\n"
            "mod tests {\n"
            "    use super::*;\n"
            "    #[test]\n"
            "    fn it_works() { read_cfg(); }\n"
            "}\n",
            encoding="utf-8",
        )
        (repo / "src" / "config" / "catalog.rs").write_text(
            "pub struct ModelCatalog;\n",
            encoding="utf-8",
        )
        (repo / "src" / "config" / "helper.rs").write_text(
            "pub struct LocalOnly;\n",
            encoding="utf-8",
        )
        (repo / "src" / "config" / "mod.rs").write_text(
            "pub mod catalog;\n"
            "mod helper;\n"
            "pub use catalog::ModelCatalog;\n"
            "use helper::LocalOnly;\n",
            encoding="utf-8",
        )
        (repo / "src" / "config" / "sub.rs").write_text(
            "use super::LocalOnly;\n"
            "pub fn use_local(_v: LocalOnly) {}\n",
            encoding="utf-8",
        )
        (repo / "src" / "rpc" / "runtime.rs").write_text(
            "use serde_json::Value;\n"
            "pub struct RpcRuntime { pub v: Option<Value> }\n",
            encoding="utf-8",
        )
        (repo / "src" / "rpc" / "mod.rs").write_text(
            "pub mod runtime;\n"
            "#[cfg(test)]\n"
            "mod tests;\n"
            "use crate::config::ModelCatalog;\n"
            "use crate::config::LocalOnly;\n"
            "use crate::config::DoesNotExist;\n"
            "pub fn boot(_c: ModelCatalog, _l: LocalOnly, _d: DoesNotExist) {}\n",
            encoding="utf-8",
        )
        (repo / "src" / "rpc" / "tests" / "mod.rs").write_text(
            "mod protocol;\n"
            "pub(super) use super::*;\n",
            encoding="utf-8",
        )
        (repo / "src" / "rpc" / "tests" / "protocol.rs").write_text(
            "use super::*;\n"
            "use crate::rpc::runtime::RpcRuntime;\n"
            "pub fn check(_r: RpcRuntime) {}\n",
            encoding="utf-8",
        )
        # Genuine sibling cycle for Rust cycle_break inspection
        (repo / "src" / "cycle_a.rs").write_text(
            "use crate::cycle_b::BetaType;\npub struct AlphaType;\npub fn make_b(_: BetaType) {}\n",
            encoding="utf-8",
        )
        (repo / "src" / "cycle_b.rs").write_text(
            "use crate::cycle_a::AlphaType;\npub struct BetaType;\npub fn make_a(_: AlphaType) {}\n",
            encoding="utf-8",
        )
        # Integration test at <pkg>/tests/ remains standalone crate
        (repo / "tests" / "integration.rs").write_text("use cal_demo::config::ModelCatalog;\n", encoding="utf-8")

        # 2. Android Kotlin + AIDL + @Composable + object members
        aidl_dir = repo / "android" / "app" / "src" / "main" / "aidl" / "io" / "example" / "app"
        kt_dir = repo / "android" / "app" / "src" / "main" / "kotlin" / "io" / "example" / "app"
        aidl_dir.mkdir(parents=True)
        kt_dir.mkdir(parents=True)

        (aidl_dir / "IEngine.aidl").write_text("package io.example.app;\ninterface IEngine {}\n", encoding="utf-8")
        (kt_dir / "Client.kt").write_text(
            "package io.example.app\n"
            "import androidx.compose.ui.Modifier\n"
            "import io.example.app.R\n"
            "import io.example.app.IEngine\n"
            "import io.example.app.MissingInternal\n"
            "class Client\n",
            encoding="utf-8",
        )
        (kt_dir / "LoadingPill.kt").write_text(
            "package io.example.app\n"
            "import androidx.compose.runtime.Composable\n"
            "@Composable\n"
            "internal fun LoadingPill(text: String) {\n"
            "  val localTmp = text.length\n"
            "}\n",
            encoding="utf-8",
        )
        (kt_dir / "OopbotBridge.kt").write_text(
            "package io.example.app\n"
            "internal object OopbotBridge {\n"
            "  val isAvailable: Boolean = true\n"
            "  @JvmStatic\n"
            "  external fun version(): String\n"
            "  private fun secret(): Unit {}\n"
            "}\n",
            encoding="utf-8",
        )

        g = build_shared_graph(str(repo))
        norm = lambda rel: str((repo / rel).resolve()).replace("\\", "/")

        # Rust nested tests path resolution vs standalone root tests/integration.rs
        all_files_norm = g["vertices"]
        self.assertEqual(
            rust_module_path_of(norm("src/rpc/tests/mod.rs"), all_files_norm)[1],
            ("rpc", "tests"),
        )
        self.assertEqual(
            rust_module_path_of(norm("src/rpc/tests/protocol.rs"), all_files_norm)[1],
            ("rpc", "tests", "protocol"),
        )
        self.assertEqual(
            rust_module_path_of(norm("tests/integration.rs"), all_files_norm)[1],
            (),
        )

        # Rust edges: pub use + super:: private + inline mod tests self-resolve + crate pkg integration test
        edges_set = set(g["edges"])
        self.assertIn((norm("src/rpc/mod.rs"), norm("src/config/catalog.rs")), edges_set)
        self.assertIn((norm("src/config/sub.rs"), norm("src/config/helper.rs")), edges_set)
        self.assertIn((norm("src/rpc/mod.rs"), norm("src/rpc/tests/mod.rs")), edges_set)
        self.assertIn((norm("src/rpc/tests/protocol.rs"), norm("src/rpc/tests/mod.rs")), edges_set)
        self.assertIn((norm("tests/integration.rs"), norm("src/config/catalog.rs")), edges_set)
        # Inline mod tests { use super::*; } in fs.rs must NOT create self-edge or edge to lib.rs
        self.assertNotIn((norm("src/fs.rs"), norm("src/lib.rs")), edges_set)
        self.assertNotIn((norm("src/fs.rs"), norm("src/fs.rs")), edges_set)

        # Fail-closed unresolved checks: private `LocalOnly` and missing `DoesNotExist` remain unresolved via `crate::config::`
        unres_pairs = {(u["reason"], u["raw_import"]) for u in g["unresolved_imports"]}
        self.assertNotIn(("rust_glob", "super"), unres_pairs)
        self.assertNotIn(("rust_mod_unresolved", "tests"), unres_pairs)
        self.assertIn(("rust_unresolved", "crate::config::DoesNotExist"), unres_pairs)
        self.assertIn(("rust_unresolved", "crate::config::LocalOnly"), unres_pairs)
        # Android AIDL resolves cleanly, while non-existent internal Kotlin class still fails closed
        self.assertIn(("kotlin_unresolved", "io.example.app.MissingInternal"), unres_pairs)
        self.assertNotIn(("kotlin_unresolved", "io.example.app.IEngine"), unres_pairs)
        ext_reasons = {(e.get("reason"), e["raw_import"]) for e in g["external_imports"]}
        self.assertIn(("kotlin_aidl_generated", "io.example.app.IEngine"), ext_reasons)
        self.assertIn(("kotlin_generated", "io.example.app.R"), ext_reasons)

        # Rust #[cfg(test)] mod tests does NOT produce spurious mod.rs <-> tests.rs cycles
        circ = analyze_circular(g)
        cycle_sets = [set(f["files"]) for f in circ["findings"]]
        self.assertNotIn({norm("src/rpc/mod.rs"), norm("src/rpc/tests/mod.rs")}, cycle_sets)
        self.assertIn({norm("src/cycle_a.rs"), norm("src/cycle_b.rs")}, cycle_sets)

        # Cycle-break leaf naming and Rust symbols_involved
        self.assertEqual(_suggest_leaf_module("src/agent/mod.rs", [], False), "src/agent/shared.rs")
        self.assertEqual(_suggest_leaf_module("src/agent/mod.rs", ["MyType"], True), "src/agent/shared_types.rs")
        self.assertEqual(_suggest_leaf_module("src/agent/runner.rs", [], False), "src/agent/runner_shared.rs")
        self.assertEqual(_suggest_leaf_module("src/host/OopbotBridge.kt", [], False), "src/host/OopbotBridgeShared.kt")
        cb = compute_cycle_break_advisory(g, [[norm("src/cycle_a.rs"), norm("src/cycle_b.rs"), norm("src/cycle_a.rs")]])
        rec_cut = cb["cycle_break_plan"]["recommended_cuts"][0]
        self.assertTrue(rec_cut["symbols_involved"], rec_cut)

        # Rust query_outline classifies bare `super` as internal_import
        out_proto = query_outline(g, norm("src/rpc/tests/protocol.rs"))
        self.assertIn("super", out_proto["internal_imports"])
        self.assertNotIn("super", out_proto["external_imports"])

        # Kotlin query_outline fixes @Composable line/sig offset and extracts object members without locals
        out_lp = query_outline(g, norm("android/app/src/main/kotlin/io/example/app/LoadingPill.kt"))
        self.assertEqual([d["name"] for d in out_lp["declarations"]], ["LoadingPill"])
        self.assertEqual(out_lp["declarations"][0]["line"], 4)
        self.assertTrue(out_lp["declarations"][0]["signature"].startswith("internal fun LoadingPill"))

        out_br = query_outline(g, norm("android/app/src/main/kotlin/io/example/app/OopbotBridge.kt"))
        self.assertEqual(out_br["exports"], ["OopbotBridge"])
        br_decls = {d["name"]: d for d in out_br["declarations"]}
        self.assertIn("isAvailable", br_decls)
        self.assertIn("version", br_decls)
        self.assertEqual(br_decls["version"]["kind"], "function")
        self.assertTrue(br_decls["version"]["exported"])
        self.assertIn("secret", br_decls)
        self.assertFalse(br_decls["secret"]["exported"])

        # Target topology external package grouping excludes synthetic R/AIDL
        self.assertEqual(_external_package("serde_json::Value", "rust_external"), "serde_json")
        self.assertEqual(_external_package("androidx.compose.ui.Modifier", "kotlin_external"), "androidx.compose.ui")
        topo = build_target_topology_orientation(g)
        top_pkgs = {r["package"] for r in topo["import_resolution"]["top_external_packages"]}
        self.assertNotIn("io.example.app.R", top_pkgs)
        self.assertNotIn("io.example.app.IEngine", top_pkgs)
        self.assertIn("androidx.compose.ui", top_pkgs)
        self.assertIn("serde_json", top_pkgs)

        # Mermaid depth=2 disambiguates mod.rs hubs with parent directory name
        buf_mm = io.StringIO()
        with redirect_stdout(buf_mm):
            rc_mm = plw_cli.cmd_mermaid([str(repo), "--depth", "2", "--json"])
        self.assertEqual(rc_mm, 0)
        mm_src = json.loads(buf_mm.getvalue()).get("mermaid_source", "")
        self.assertIn("config/mod.rs", mm_src)

        # Portable agent dynamic_context reflects polyglot SOURCE_EXTENSIONS
        dc = dynamic_context(str(repo))
        self.assertNotIn("TS/JS-only", dc.get("reason", ""))

    def test_rust_pure_mod_excluded_from_cycles_v20(self):
        """Pure Rust `mod <child>;` module-tree declarations stay in the graph for
        reachability/impact/orphan analysis, but do not fabricate directed cycles."""
        from codebase.cycle_break import compute_cycle_break_advisory
        from codebase.hott_analyzers import analyze_manifold
        from codebase.topology_analyzers import analyze_circular, query_impact
        from core.shared_graph import build_shared_graph
        from core.target_topology import build_target_topology_orientation

        root = Path(tempfile.mkdtemp(prefix="plw_rust_mod_cyc_"))
        self.addCleanup(shutil.rmtree, root, True)

        # Case 1: Pure `mod` impl-split + string literal containing "<mod>.com" +
        # nested `#[cfg(test)] mod tests;` -> `mod protocol;` -> `use super::*;`
        pure_repo = root / "pure_repo"
        (pure_repo / "src" / "llm").mkdir(parents=True)
        (pure_repo / "src" / "rpc" / "tests").mkdir(parents=True)
        (pure_repo / "Cargo.toml").write_text('[package]\nname = "pure_demo"\nversion = "0.1.0"\n', encoding="utf-8")
        (pure_repo / "src" / "lib.rs").write_text("pub mod llm;\npub mod rpc;\n", encoding="utf-8")
        (pure_repo / "src" / "llm" / "mod.rs").write_text(
            'mod provider;\n'
            'pub struct LlmClient { pub host: String }\n'
            'impl LlmClient {\n'
            '    pub fn endpoint(&self) -> &str { "provider.com" }\n'
            '    pub fn unrelated(&self) { let _ = other::provider::noop(); }\n'
            '}\n',
            encoding="utf-8",
        )
        (pure_repo / "src" / "llm" / "provider.rs").write_text(
            "use super::LlmClient;\n"
            "impl LlmClient {\n"
            "    pub fn send(&self) -> usize { self.host.len() }\n"
            "}\n",
            encoding="utf-8",
        )
        (pure_repo / "src" / "rpc" / "mod.rs").write_text(
            "pub struct RpcState;\n"
            "#[cfg(test)]\n"
            "mod tests;\n",
            encoding="utf-8",
        )
        (pure_repo / "src" / "rpc" / "tests" / "mod.rs").write_text(
            "mod protocol;\n"
            "pub(super) fn helper() {}\n",
            encoding="utf-8",
        )
        (pure_repo / "src" / "rpc" / "tests" / "protocol.rs").write_text(
            "use super::*;\n"
            "#[test]\n"
            "fn ok() { helper(); }\n",
            encoding="utf-8",
        )

        g_pure = build_shared_graph(str(pure_repo))
        norm_p = lambda rel: str((pure_repo / rel).resolve()).replace("\\", "/")
        ed_pure = {(e["source"], e["target"]): e["semantic_usages"] for e in g_pure["edge_details"]}

        # Pure mod edges remain in edges (for reachability/impact/orphan) with exact ["rust_mod"] usage
        self.assertIn((norm_p("src/llm/mod.rs"), norm_p("src/llm/provider.rs")), set(g_pure["edges"]))
        self.assertEqual(
            ed_pure[(norm_p("src/llm/mod.rs"), norm_p("src/llm/provider.rs"))],
            ["rust_mod"],
        )
        self.assertEqual(
            ed_pure[(norm_p("src/rpc/tests/mod.rs"), norm_p("src/rpc/tests/protocol.rs"))],
            ["rust_mod"],
        )
        self.assertGreaterEqual(g_pure["node_metadata"][norm_p("src/llm/provider.rs")]["fan_in"], 1)
        self.assertGreaterEqual(g_pure["node_metadata"][norm_p("src/rpc/tests/protocol.rs")]["fan_in"], 1)

        # Zero directed cycles across all cycle-aware consumers
        circ_pure = analyze_circular(g_pure)
        self.assertEqual(circ_pure["summary"]["total_cycles"], 0, circ_pure["findings"])
        self.assertEqual(circ_pure["summary"]["cyclic_scc_count"], 0)
        topo_pure = build_target_topology_orientation(g_pure)
        self.assertEqual(topo_pure["inventory"]["cyclic_sccs"], 0)
        mani_pure = analyze_manifold(g_pure)
        self.assertEqual(mani_pure["summary"]["cycle_orientation_counts"]["directed"], 0)

        imp_prov = query_impact(g_pure, norm_p("src/llm/provider.rs"))
        self.assertIn(norm_p("src/llm/mod.rs"), imp_prov["direct_downstream"])
        self.assertEqual(imp_prov["circular_references"], [])
        self.assertIsNone(imp_prov["cycle_break_advisory"])

        # Case 2: Genuine cycles MUST still be detected:
        #   (a) mod + `use` / `pub use` from child while child uses `super::`
        #   (b) mod + inline `child::fn()` path ref while child uses `super::`
        #   (c) peer-to-peer `use` cycle (`peer_a <-> peer_b`)
        real_repo = root / "real_repo"
        (real_repo / "src" / "use_cyc").mkdir(parents=True)
        (real_repo / "src" / "path_cyc").mkdir(parents=True)
        (real_repo / "Cargo.toml").write_text('[package]\nname = "real_demo"\nversion = "0.1.0"\n', encoding="utf-8")
        (real_repo / "src" / "lib.rs").write_text(
            "pub mod use_cyc;\npub mod path_cyc;\npub mod peer_a;\npub mod peer_b;\n",
            encoding="utf-8",
        )
        (real_repo / "src" / "use_cyc" / "mod.rs").write_text(
            "mod runner;\n"
            "use runner::run_step;\n"
            "pub struct ParentCtx;\n"
            "pub fn start() { run_step(); }\n",
            encoding="utf-8",
        )
        (real_repo / "src" / "use_cyc" / "runner.rs").write_text(
            "use super::ParentCtx;\n"
            "pub fn run_step() -> ParentCtx { ParentCtx }\n",
            encoding="utf-8",
        )
        (real_repo / "src" / "path_cyc" / "mod.rs").write_text(
            "mod worker;\n"
            "pub struct Job;\n"
            "pub fn go() { let _ = worker::exec(); }\n",
            encoding="utf-8",
        )
        (real_repo / "src" / "path_cyc" / "worker.rs").write_text(
            "use super::Job;\n"
            "pub fn exec() -> Job { Job }\n",
            encoding="utf-8",
        )
        (real_repo / "src" / "peer_a.rs").write_text(
            "use crate::peer_b::Beta;\npub struct Alpha(pub Beta);\n",
            encoding="utf-8",
        )
        (real_repo / "src" / "peer_b.rs").write_text(
            "use crate::peer_a::Alpha;\npub struct Beta;\npub fn consume(_: Alpha) {}\n",
            encoding="utf-8",
        )

        g_real = build_shared_graph(str(real_repo))
        norm_r = lambda rel: str((real_repo / rel).resolve()).replace("\\", "/")
        ed_real = {(e["source"], e["target"]): e["semantic_usages"] for e in g_real["edge_details"]}

        self.assertIn("rust_use", ed_real[(norm_r("src/use_cyc/mod.rs"), norm_r("src/use_cyc/runner.rs"))])
        self.assertIn("rust_mod", ed_real[(norm_r("src/use_cyc/mod.rs"), norm_r("src/use_cyc/runner.rs"))])
        self.assertIn("rust_path_ref", ed_real[(norm_r("src/path_cyc/mod.rs"), norm_r("src/path_cyc/worker.rs"))])

        circ_real = analyze_circular(g_real)
        cycle_sets = [set(f["files"]) for f in circ_real["findings"]]
        self.assertIn({norm_r("src/use_cyc/mod.rs"), norm_r("src/use_cyc/runner.rs")}, cycle_sets)
        self.assertIn({norm_r("src/path_cyc/mod.rs"), norm_r("src/path_cyc/worker.rs")}, cycle_sets)
        self.assertIn({norm_r("src/peer_a.rs"), norm_r("src/peer_b.rs")}, cycle_sets)
        self.assertEqual(circ_real["summary"]["cyclic_scc_count"], 3)

        topo_real = build_target_topology_orientation(g_real)
        self.assertEqual(topo_real["inventory"]["cyclic_sccs"], 3)
        mani_real = analyze_manifold(g_real)
        directed_basis = [
            set(b["vertices"])
            for b in mani_real["findings"][0].get("cycle_basis", [])
            if b.get("orientation") == "directed"
        ]
        self.assertIn({norm_r("src/use_cyc/mod.rs"), norm_r("src/use_cyc/runner.rs")}, directed_basis)
        self.assertIn({norm_r("src/path_cyc/mod.rs"), norm_r("src/path_cyc/worker.rs")}, directed_basis)
        self.assertEqual(mani_real["summary"]["cycle_orientation_counts"]["directed"], 2)

        # Cycle-break on mod+use edge reports the imported symbol (`run_step`), not the `mod` name (`runner`)
        cb_use = compute_cycle_break_advisory(
            g_real,
            [[norm_r("src/use_cyc/mod.rs"), norm_r("src/use_cyc/runner.rs"), norm_r("src/use_cyc/mod.rs")]],
        )
        adv_edges = cb_use["advisories_by_cycle"][0]["candidate_edges"]
        mod_to_runner = next(
            e for e in adv_edges
            if e["source"] == norm_r("src/use_cyc/mod.rs") and e["target"] == norm_r("src/use_cyc/runner.rs")
        )
        self.assertIn("run_step", mod_to_runner["rationale"])
        self.assertNotIn("(runner", mod_to_runner["rationale"])

    def test_type_only_downstream_consistency_and_mermaid_shlex_quoting(self):
        import io
        import shlex
        from contextlib import redirect_stdout
        from pathlib import Path
        import tempfile
        import plw_cli
        from core.shared_graph import build_shared_graph
        from codebase.topology_analyzers import analyze_circular
        from codebase.cycle_break import compute_cycle_break_advisory
        from codebase.hott_analyzers import analyze_manifold
        from core.target_topology import build_target_topology_orientation
        from core.synthesizer import encode_topological_invariants
        from hott_kernel import kernel_cycle_break, kernel_xsteer

        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "ts type repo"
            (repo / "src" / "math compesor").mkdir(parents=True)
            # Cycle 1: pure `import type` + `export type`
            (repo / "src" / "a.ts").write_text(
                'import type { BSpec } from "./b";\n'
                'export interface ASpec { id: string; b?: BSpec }\n'
                'export function makeA(): ASpec { return { id: "a" }; }\n'
            )
            (repo / "src" / "b.ts").write_text(
                'import type { ASpec } from "./a";\n'
                'export type { ASpec } from "./a";\n'
                'export interface BSpec { a?: ASpec }\n'
            )
            # Cycle 2: `import { type ... }` inline type modifier
            (repo / "src" / "c.ts").write_text(
                'import { type DType } from "./d";\n'
                'export type CType = { d?: DType };\n'
            )
            (repo / "src" / "d.ts").write_text(
                'import { type CType } from "./c";\n'
                'export type DType = { c?: CType };\n'
            )
            # Directory with spaces for Mermaid zoom hint quoting + entry hub
            (repo / "src" / "math compesor" / "zip.ts").write_text(
                'export function compress(v: number): number { return v + 1; }\n'
            )
            (repo / "src" / "math compesor" / "index.ts").write_text(
                'import { compress } from "./zip";\n'
                'import { makeA } from "../a";\n'
                'export function runAll(): number { makeA(); return compress(1); }\n'
            )

            g = build_shared_graph(str(repo))
            circ = analyze_circular(g)
            self.assertEqual(circ["summary"]["total_cycles"], 2)
            self.assertEqual(circ["summary"]["type_only_cycles"], 2)
            self.assertEqual(circ["summary"]["runtime_cycles"], 0)
            self.assertEqual(circ["summary"]["cycle_break_plan"]["total_cuts_required"], 0)
            self.assertEqual(circ["summary"]["cycle_break_plan"]["recommended_cuts"], [])
            self.assertGreater(len(circ["summary"]["cycle_break_plan"]["optional_type_only_cuts"]), 0)

            cb = compute_cycle_break_advisory(g, [c["cycle"] for c in circ["findings"]])
            self.assertEqual(cb["cycle_break_plan"]["total_cuts_required"], 0)
            self.assertEqual(cb["cycle_break_plan"]["recommended_cuts"], [])
            self.assertEqual(cb["cycle_break_plan"]["runtime_cycles"], 0)
            self.assertEqual(cb["cycle_break_plan"]["type_only_cycles"], 2)
            for adv in cb["advisories_by_cycle"]:
                self.assertTrue(adv["is_type_only"], adv)
                self.assertEqual(adv["cycle_nature"], "type_only", adv)
                for cut in adv["candidate_edges"]:
                    self.assertEqual(cut["strategy"], "extract_types_leaf", cut)
                    steps_txt = " ".join(cut.get("actionable_steps", []))
                    self.assertNotIn("Change import in", steps_txt)

            kb = kernel_cycle_break(str(repo), target_file="src/a.ts")
            self.assertEqual(kb["summary"]["total_cuts_required"], 0)
            self.assertEqual(kb["summary"]["runtime_cycles_affecting_target"], 0)
            self.assertEqual(kb["summary"]["type_only_cycles_affecting_target"], 1)
            self.assertEqual(len(kb["summary"]["recommended_cuts"]), 0)
            self.assertGreater(len(kb["summary"]["optional_type_only_cuts"]), 0)

            topo = build_target_topology_orientation(g)
            self.assertEqual(topo["inventory"]["cyclic_sccs"], 0)
            self.assertEqual(topo["inventory"]["runtime_cyclic_sccs"], 0)
            self.assertEqual(topo["inventory"]["type_only_cyclic_sccs"], 2)
            self.assertEqual(len(topo["type_only_cyclic_sccs"]), 2)

            mani = analyze_manifold(g)
            self.assertEqual(mani["summary"]["runtime_cyclic_sccs"], 0)
            self.assertEqual(mani["summary"]["type_only_cyclic_sccs"], 2)
            self.assertFalse(mani["manifold"]["structural_archetype"].endswith("_cyclic"), mani["manifold"]["structural_archetype"])
            orient = mani["manifold"]["cycle_orientation_counts"]
            self.assertEqual(orient["directed"], 0)
            self.assertGreater(orient["type_only_directed"], 0)

            brief = encode_topological_invariants(str(repo), shared_graph=g, analyzer_output={"results": {"hott.manifold": mani}})
            self.assertGreater(brief["cycle_semantics"]["type_only_directed_basis_witnesses"], 0)

            xs = kernel_xsteer(str(repo))
            cb_steer = xs["codebase_steering"]
            self.assertFalse(cb_steer["archetype"].endswith("_cyclic"), cb_steer)
            self.assertNotEqual(cb_steer["strategy"], "cycle_breaking_and_layering")

            # CLI smoke: topology human output + mermaid shlex.quote on spaced dir/root
            buf_t = io.StringIO()
            with redirect_stdout(buf_t):
                rc_t = plw_cli.cmd_topology([str(repo)])
            self.assertEqual(rc_t, 0)
            self.assertIn("type-only SCCs: 2", buf_t.getvalue())

            for depth in ("1", "2", "3", "4"):
                buf_m = io.StringIO()
                with redirect_stdout(buf_m):
                    rc_m = plw_cli.cmd_mermaid([str(repo), "--depth", depth, "--json"])
                self.assertEqual(rc_m, 0)
                mm = json.loads(buf_m.getvalue())
                for hint in mm.get("next_zoom_hints", []):
                    tokens = shlex.split(hint["command"])
                    if "--focus" in tokens:
                        idx = tokens.index("--focus")
                        self.assertEqual(tokens[idx + 1], hint["target"])

    def test_unannotated_type_import_still_recommends_type_only_and_mixed_edge_priority(self):
        from pathlib import Path
        import tempfile
        from core.shared_graph import build_shared_graph
        from codebase.topology_analyzers import analyze_circular
        from codebase.cycle_break import compute_cycle_break_advisory
        from codebase.hott_analyzers import analyze_manifold
        from core.target_topology import build_target_topology_orientation

        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "mixed"
            (repo / "src").mkdir(parents=True)
            # Cycle A: unannotated runtime import importing only an interface -> MUST stay type_only_import cut!
            (repo / "src" / "ua.ts").write_text(
                'import { UBSpec } from "./ub";\n'
                'export interface UASpec { b?: UBSpec }\n'
            )
            (repo / "src" / "ub.ts").write_text(
                'import { UASpec } from "./ua";\n'
                'export interface UBSpec { a?: UASpec }\n'
            )
            # Cycle B: real runtime cycle + a type_only back-edge in same SCC
            (repo / "src" / "rx.ts").write_text(
                'import { ry } from "./ry";\n'
                'import type { RZ } from "./rz";\n'
                'export function rx(): number { return ry(); }\n'
                'export type RX = { z?: RZ };\n'
            )
            (repo / "src" / "ry.ts").write_text(
                'import { rx } from "./rx";\n'
                'export function ry(): number { return rx(); }\n'
            )
            (repo / "src" / "rz.ts").write_text(
                'import type { RX } from "./rx";\n'
                'export interface RZ { x?: RX }\n'
            )

            g = build_shared_graph(str(repo))
            circ = analyze_circular(g)
            cb = compute_cycle_break_advisory(g, [c["cycle"] for c in circ["findings"]])
            # Unannotated ua<->ub AND runtime rx<->ry must require cuts
            self.assertEqual(cb["cycle_break_plan"]["runtime_cycles"], 2)
            self.assertEqual(cb["cycle_break_plan"]["total_cuts_required"], 2)
            # Ensure ua<->ub gets type_only_import strategy (not extract_types_leaf)
            ua_adv = [a for a in cb["advisories_by_cycle"] if any("ua.ts" in n for n in a["cycle"])][0]
            self.assertFalse(ua_adv["is_type_only"])
            self.assertTrue(any(c["strategy"] == "type_only_import" for c in ua_adv["candidate_edges"]), ua_adv)

            # Ensure rx<->ry runtime SCC is still detected and classifies as cyclic despite type_only chord
            topo = build_target_topology_orientation(g)
            self.assertEqual(topo["inventory"]["runtime_cyclic_sccs"], 2)
            mani = analyze_manifold(g)
            self.assertEqual(mani["summary"]["runtime_cyclic_sccs"], 2)
            self.assertTrue(mani["manifold"]["structural_archetype"].endswith("_cyclic"), mani["manifold"]["structural_archetype"])



    def test_calibration_polyglot_topology_candidates_and_semantic_usages_v1(self) -> None:
        from core.topology_engine import (
            COMMON_NOISE_SYMBOLS,
            _definition_candidates,
            _harvest_graph_candidates,
            _source_top_level_definitions,
        )

        # 1. Go generic functions/methods/types & grouped type (...) blocks
        go_src = (
            "package main\n\n"
            "type Box[T any] struct { v T }\n"
            "type AliasID = string\n"
            "type (\n"
            "    GroupedAlias = int\n"
            "    GenericPair[K comparable, V any] struct {\n"
            "        fieldK K\n"
            "    }\n"
            ")\n"
            "func GenMap[T any, R any](v T) R { var r R; return r }\n"
            "func (b *Box[T]) Unwrap() T { return b.v }\n"
        )
        go_defs = _definition_candidates(go_src, "go")
        for expected in ("Box", "AliasID", "GroupedAlias", "GenericPair", "GenMap", "Unwrap"):
            self.assertIn(expected, go_defs, f"missing Go candidate {expected!r} in {go_defs}")
        self.assertNotIn("fieldK", go_defs)

        # 2. Python async def + TS abstract class + Rust pub(crate)/async/macro_rules!
        py_src = "async def run_async_job(x: int) -> int:\n    return x\ndef sync_fn():\n    pass\n"
        py_defs = _definition_candidates(py_src, "python")
        self.assertEqual(py_defs, {"run_async_job", "sync_fn"})

        ts_src = (
            "export abstract class AbstractWorker {\n  abstract step(): void;\n}\n"
            "abstract class LocalAbstract {}\n"
        )
        ts_defs = _source_top_level_definitions(ts_src, "src/worker.ts")
        self.assertEqual(ts_defs, {"AbstractWorker", "LocalAbstract"})

        rs_src = (
            "pub(crate) struct CrateCfg;\n"
            "pub(crate) async fn handle_async() {}\n"
            "macro_rules! emit_log { ($x:expr) => { $x }; }\n"
        )
        rs_defs = _definition_candidates(rs_src, "rust")
        self.assertEqual(rs_defs, {"CrateCfg", "handle_async", "emit_log"})

        # 3. _harvest_graph_candidates must NOT leak semantic_usages metadata tags (`rust_use`, `rust_mod`)
        with tempfile.TemporaryDirectory() as tmpdir:
            repo = Path(tmpdir)
            (repo / "src").mkdir()
            (repo / "src" / "lib.rs").write_text(
                "mod helper;\nuse crate::cfg::Settings;\npub fn entry() {}\n",
                encoding="utf-8",
            )
            (repo / "src" / "helper.rs").write_text("pub fn help() {}\n", encoding="utf-8")
            (repo / "src" / "cfg.rs").write_text("pub struct Settings;\n", encoding="utf-8")
            (repo / "src" / "app.py").write_text(
                "from __future__ import annotations\n"
                "from typing import TYPE_CHECKING\n"
                "if TYPE_CHECKING:\n    from .dep import Model\n",
                encoding="utf-8",
            )
            g = build_shared_graph(str(repo))
            cands, _, internal_defined, _, _, _ = _harvest_graph_candidates(
                g, None, str(repo.resolve()).replace("\\", "/")
            )
            for tag in ("rust_use", "rust_mod", "cfg_test_mod", "rust_path_ref"):
                self.assertNotIn(tag, cands)
            self.assertIn("Settings", cands)
            self.assertIn("TYPE_CHECKING", COMMON_NOISE_SYMBOLS)
            self.assertIn("annotations", COMMON_NOISE_SYMBOLS)
            self.assertNotIn("TYPE_CHECKING", internal_defined)

            # 4. Semantic usage calibration: TS generic calls, union/cast/satisfies, multi-arg generics
            (repo / "src" / "ts_types.ts").write_text(
                "export interface UnionA { a: number }\n"
                "export interface UnionB { b: string }\n"
                "export interface GenericArg { ok: boolean }\n"
                "export interface CastT { v: number }\n"
                "export interface KeyT { k: string }\n"
                "export interface ValT { v: string }\n"
                "export function realFn<T>(x: any): T { return x as T; }\n",
                encoding="utf-8",
            )
            (repo / "src" / "ts_consumer.ts").write_text(
                "import { type UnionA, type UnionB, type GenericArg, type CastT, type KeyT, type ValT, realFn } from './ts_types';\n"
                "export function demo(x: UnionA | UnionB, m: Map<KeyT, ValT>): GenericArg {\n"
                "  const z = x as CastT;\n"
                "  return realFn<GenericArg>(z);\n"
                "}\n",
                encoding="utf-8",
            )
            g_ts = build_shared_graph(str(repo))
            norm = str((repo / "src" / "ts_consumer.ts").resolve()).replace("\\", "/")
            norm_t = str((repo / "src" / "ts_types.ts").resolve()).replace("\\", "/")
            ed_ts = {(e["source"], e["target"]): e["semantic_usages"] for e in g_ts["edge_details"]}
            usages = set(ed_ts[(norm, norm_t)])
            for want_usage in (
                "called(realFn)",
                "typed(UnionA)",
                "typed(UnionB)",
                "typed(CastT)",
                "typed(KeyT)",
                "typed(ValT)",
                "typed(GenericArg)",
            ):
                self.assertIn(want_usage, usages, f"missing {want_usage!r} in {usages}")


    def test_calibration_rust_kotlin_topology_cycles_check_v1(self) -> None:
        """Regression suite for Rust & Kotlin topology, cycle-break, and test_reachability calibration."""
        from codebase.cycle_break import compute_cycle_break_advisory
        from codebase.hott_analyzers import analyze_manifold
        from codebase.performance_analyzers import _is_test_file as perf_is_test_file
        from codebase.topology_analyzers import analyze_circular, analyze_test_reachability
        from core.shared_graph import build_shared_graph
        from core.target_topology import build_target_topology_orientation

        # 1. performance_analyzers parity for Rust tests.rs / test.rs
        self.assertTrue(perf_is_test_file("src/module/tests.rs"))
        self.assertTrue(perf_is_test_file("src/module/test.rs"))

        with tempfile.TemporaryDirectory() as tmpdir:
            repo = Path(tmpdir)
            (repo / "Cargo.toml").write_text('[package]\nname = "demo_app"\nversion = "0.1.0"\n', encoding="utf-8")
            (repo / "src" / "bus").mkdir(parents=True)
            (repo / "src" / "rpc").mkdir(parents=True)
            (repo / "src" / "extra").mkdir(parents=True)
            (repo / "tests").mkdir(parents=True)

            (repo / "src" / "lib.rs").write_text(
                "pub mod bus;\npub mod rpc;\npub mod engine;\n",
                encoding="utf-8",
            )
            (repo / "src" / "main.rs").write_text(
                "fn main() { demo_app::engine::boot(); }\n",
                encoding="utf-8",
            )
            (repo / "src" / "util.rs").write_text(
                "pub fn pure_math(x: i32) -> i32 { x + 1 }\n",
                encoding="utf-8",
            )
            (repo / "src" / "engine.rs").write_text(
                "use crate::util::pure_math;\n"
                "pub fn boot() -> i32 { pure_math(1) }\n"
                "#[cfg(test)]\n"
                "mod tests {\n"
                "    #[test]\n"
                "    fn inline_ok() { assert_eq!(super::boot(), 2); }\n"
                "}\n",
                encoding="utf-8",
            )
            (repo / "src" / "extra" / "tests.rs").write_text(
                "use crate::engine::boot;\n#[test]\nfn mod_tests_rs_runs() { let _ = boot(); }\n",
                encoding="utf-8",
            )
            (repo / "src" / "extra" / "mod.rs").write_text(
                "#[cfg(test)]\nmod helper_only;\n#[cfg(test)]\nmod tests;\n",
                encoding="utf-8",
            )
            (repo / "src" / "extra" / "helper_only.rs").write_text(
                "pub fn fixture_seed() -> i32 { 42 }\n",
                encoding="utf-8",
            )
            (repo / "tests" / "cli_smoke.rs").write_text(
                '#[test]\nfn runs_bin() { let _bin = env!("CARGO_BIN_EXE_demo_app"); }\n',
                encoding="utf-8",
            )

            # Pure wildcard facade edge (`use helper::*;` with 0 helper symbols used in parent body)
            # + `#[cfg(test)] use crate::rpc::other::MockVal;` must NOT form a production cycle.
            (repo / "src" / "rpc" / "mod.rs").write_text(
                "mod helper;\n"
                "pub mod other;\n"
                "use helper::*;\n"
                "#[cfg(test)]\n"
                "use crate::rpc::other::MockVal;\n"
                "pub fn rpc_entry() { other::other_step(); }\n",
                encoding="utf-8",
            )
            (repo / "src" / "rpc" / "helper.rs").write_text(
                "use crate::rpc::rpc_entry;\n"
                "pub fn hidden_helper() { rpc_entry(); }\n",
                encoding="utf-8",
            )
            (repo / "src" / "rpc" / "other.rs").write_text(
                "pub struct MockVal;\npub fn other_step() {}\n",
                encoding="utf-8",
            )

            # Real Rust cycle with `use super::Sym` and wildcard `use worker::*;` (called in body)
            (repo / "src" / "bus" / "mod.rs").write_text(
                "mod worker;\n"
                "use worker::*;\n"
                "pub struct BusConfig { pub id: i32 }\n"
                "pub fn start_bus() { WorkerLoop::run(); }\n",
                encoding="utf-8",
            )
            (repo / "src" / "bus" / "worker.rs").write_text(
                "use super::BusConfig;\n"
                "pub struct WorkerLoop;\n"
                "impl WorkerLoop { pub fn run() { let _ = BusConfig { id: 1 }; } }\n",
                encoding="utf-8",
            )

            # Kotlin same-package extension function (`ext_fun`), top-level val (`appGraph`), & test helper
            kt_main = repo / "android" / "app" / "src" / "main" / "kotlin" / "com" / "example" / "app"
            kt_test = repo / "android" / "app" / "src" / "test" / "kotlin" / "com" / "example" / "app"
            kt_main.mkdir(parents=True)
            kt_test.mkdir(parents=True)

            (kt_main / "UiScreen.kt").write_text(
                "package com.example.app\n\n"
                "fun renderMain(state: UiState) {\n"
                "    state.openDrawerSheet()\n"
                "}\n",
                encoding="utf-8",
            )
            (kt_main / "UiExtensions.kt").write_text(
                "package com.example.app\n\n"
                "class UiState\n"
                "internal fun UiState.openDrawerSheet() {}\n",
                encoding="utf-8",
            )
            (kt_main / "sidebar").mkdir(parents=True)
            (kt_main / "sidebar" / "NavState.kt").write_text(
                "package com.example.app.sidebar\n\n"
                "import com.example.app.NavHostRef\n"
                "import com.example.app.appGraph\n\n"
                "class NavState {\n"
                "    fun bind() { val _g = NavHostRef.appGraph }\n"
                "}\n",
                encoding="utf-8",
            )
            (kt_main / "AppGraph.kt").write_text(
                "package com.example.app\n\n"
                "import com.example.app.sidebar.NavState\n\n"
                "object NavHostRef\n"
                "val NavHostRef.appGraph: NavState get() = NavState()\n",
                encoding="utf-8",
            )
            (kt_test / "FakePrefs.kt").write_text(
                "package com.example.app\n\n"
                "class FakePrefs { fun seed() = 1 }\n",
                encoding="utf-8",
            )
            (kt_test / "ScreenBehaviorTest.kt").write_text(
                "package com.example.app\n\n"
                "import org.junit.Test\n"
                "class ScreenBehaviorTest {\n"
                "    @Test fun renders() {\n"
                "        val p = FakePrefs(); p.seed()\n"
                "        renderMain(UiState())\n"
                "    }\n"
                "}\n",
                encoding="utf-8",
            )

            g = build_shared_graph(str(repo))
            nmeta = g["node_metadata"]
            edge_map = {(e["source"], e["target"]): set(e["semantic_usages"]) for e in g["edge_details"]}

            p = lambda rel: str((repo / rel).resolve()).replace("\\", "/")

            # Classification & inline test flags
            self.assertTrue(nmeta[p("src/extra/tests.rs")]["is_test"])
            self.assertTrue(nmeta[p("src/extra/helper_only.rs")]["is_test"])
            self.assertFalse(nmeta[p("src/engine.rs")]["is_test"])
            self.assertTrue(nmeta[p("src/engine.rs")]["has_inline_tests"])

            # CARGO_BIN_EXE resolution & Rust facade / cfg(test) semantics
            self.assertIn("cargo_bin_exe(demo_app)", edge_map[(p("tests/cli_smoke.rs"), p("src/main.rs"))])
            self.assertIn("rust_facade_reexport", edge_map[(p("src/rpc/mod.rs"), p("src/rpc/helper.rs"))])
            self.assertIn("cfg_test_use", edge_map[(p("src/rpc/mod.rs"), p("src/rpc/other.rs"))])

            # Kotlin same-package extension function dot-call edge
            self.assertIn(
                (p("android/app/src/main/kotlin/com/example/app/UiScreen.kt"), p("android/app/src/main/kotlin/com/example/app/UiExtensions.kt")),
                edge_map,
            )

            # Cycles & cycle-break symbols (must include real bus + kotlin cycle, NOT rpc facade, zero empty symbols)
            circ = analyze_circular(g)
            cycle_nodes = {n for f in circ["findings"] for n in f["cycle"]}
            self.assertNotIn(p("src/rpc/helper.rs"), cycle_nodes)
            self.assertIn(p("src/bus/mod.rs"), cycle_nodes)
            self.assertIn(p("src/bus/worker.rs"), cycle_nodes)

            cb = compute_cycle_break_advisory(g, [c["cycle"] for c in circ["findings"]])
            cuts = cb["cycle_break_plan"]["recommended_cuts"]
            self.assertGreaterEqual(len(cuts), 2)
            for cut in cuts:
                self.assertTrue(cut["symbols_involved"], f"empty symbols_involved in cut: {cut}")
            bus_syms = {sym for cut in cuts if "bus/" in cut["source"] for sym in cut["symbols_involved"]}
            self.assertTrue(bus_syms & {"BusConfig", "WorkerLoop"}, f"expected bus symbols in {cuts}")
            kt_syms = {sym for cut in cuts if cut["source"].endswith(".kt") for sym in cut["symbols_involved"]}
            self.assertTrue(kt_syms & {"appGraph", "NavState", "NavHostRef"}, f"expected kotlin symbols in {cuts}")

            # Test reachability: inline tests credit engine.rs & util.rs; test helper FakePrefs.kt not flagged isolated_test
            tr = analyze_test_reachability(g)
            isolated_test_files = {f["file"] for f in tr["findings"] if f["type"] == "isolated_test"}
            self.assertNotIn(p("android/app/src/test/kotlin/com/example/app/FakePrefs.kt"), isolated_test_files)
            self.assertTrue(any(w.get("inline_test") and w.get("test") == p("src/engine.rs") for w in tr["source_test_witnesses"].get(p("src/engine.rs"), [])), tr["source_test_witnesses"])
            self.assertIn(p("src/util.rs"), tr["source_test_witnesses"])
            self.assertTrue(any(w.get("test") == p("tests/cli_smoke.rs") for w in tr["source_test_witnesses"].get(p("src/main.rs"), [])))
            self.assertGreaterEqual(tr["summary"]["directly_tested_files"], 3)

            # Topology & manifold consistency
            topo = build_target_topology_orientation(g)
            mani = analyze_manifold(g)
            self.assertEqual(topo["inventory"]["cyclic_sccs"], mani["summary"]["runtime_cyclic_sccs"])
            self.assertEqual(topo["inventory"]["cyclic_sccs"], 2)


if __name__ == "__main__":
    unittest.main()
