"""Tests for project-scoped recursive minor calibration (`.plw/calibration.json` + `plw calibrate`)."""
from __future__ import annotations

import io
import json
import os
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

import plw_cli
from codebase.topology_analyzers import analyze_circular, analyze_test_reachability
from core.analyzer_cache import run_cached_analyzers
from core.analyzer_registry import analyze_orphan
from core.graph_cache import build_cached_shared_graph, get_graph_cache_status
from core.portable_agent import doctor
from core.project_calibration import (
    ALLOWED_SUPPRESS_ANALYZERS,
    FORBIDDEN_SUPPRESS_ANALYZERS,
    calibration_digest,
    empty_calibration_config,
    load_project_calibration,
    validate_calibration_payload,
    write_project_calibration,
)
from core.shared_graph import build_shared_graph, graph_content_signature


def _run_cli(*argv: str) -> tuple[int, str]:
    buf = io.StringIO()
    with redirect_stdout(buf):
        try:
            rc = plw_cli._CLI_COMMAND_DISPATCH[argv[0]](list(argv[1:]))
        except SystemExit as exc:
            rc = int(exc.code) if exc.code is not None else 0
    return int(rc), buf.getvalue()


class ProjectCalibrationTests(unittest.TestCase):
    def _make_sample_repo(self, root: Path) -> None:
        (root / "src").mkdir(parents=True, exist_ok=True)
        (root / "src" / "domain").mkdir(parents=True, exist_ok=True)
        (root / "src" / "generated_sandbox").mkdir(parents=True, exist_ok=True)
        (root / "scripts").mkdir(parents=True, exist_ok=True)
        (root / "test").mkdir(parents=True, exist_ok=True)
        (root / "kotlin").mkdir(parents=True, exist_ok=True)

        (root / "src" / "index.ts").write_text(
            'import { computeValue } from "@domain/math";\n'
            "export function main(): number {\n"
            "  return computeValue();\n"
            "}\n",
            encoding="utf-8",
        )
        (root / "src" / "domain" / "math.ts").write_text(
            "export function computeValue(): number {\n"
            "  const stepOne = 20;\n"
            "  const stepTwo = 22;\n"
            "  return stepOne + stepTwo;\n"
            "}\n",
            encoding="utf-8",
        )
        (root / "src" / "generated_sandbox" / "junk.ts").write_text(
            "export const unusedGenerated = 1;\n",
            encoding="utf-8",
        )
        (root / "src" / "domain" / "orphan_mod.ts").write_text(
            "export const orphanHelper = 42;\n",
            encoding="utf-8",
        )
        (root / "scripts" / "build_bundle.py").write_text(
            "#!/usr/bin/env python3\n"
            "import sys\n"
            "def run_build() -> int:\n"
            "    return len(sys.argv)\n",
            encoding="utf-8",
        )
        (root / "test" / "global_setup.ts").write_text(
            "export async function setupSuite(): Promise<void> {}\n",
            encoding="utf-8",
        )
        (root / "test" / "index.test.ts").write_text(
            'import { main } from "../src/index";\n'
            "if (main() !== 42) throw new Error('boom');\n",
            encoding="utf-8",
        )
        (root / "kotlin" / "NativeBridge.kt").write_text(
            "package demo.host\n"
            "object NativeBridge {\n"
            "    external fun invokeNative(payload: String): String\n"
            "}\n",
            encoding="utf-8",
        )

    def test_zero_regression_when_calibration_absent(self) -> None:
        with tempfile.TemporaryDirectory(prefix="plw_cal_absent_") as tmp:
            root = Path(tmp)
            self._make_sample_repo(root)
            g1 = build_shared_graph(str(root))
            g2 = build_shared_graph(str(root))
            self.assertNotIn("calibration", g1)
            self.assertEqual(graph_content_signature(g1), graph_content_signature(g2))
            self.assertIsNone(load_project_calibration(str(root)))

            rc, out = _run_cli("calibrate", "status", str(root), "--json")
            self.assertEqual(rc, 0)
            payload = json.loads(out)
            self.assertEqual(payload["status"], "absent")
            self.assertFalse(payload["present"])

    def test_validation_fail_closed_security_and_invariant_boundaries(self) -> None:
        cfg = empty_calibration_config()
        validated = validate_calibration_payload(cfg)
        self.assertEqual(validated["schema_version"], "plw-project-calibration-v1")

        # Forbidden structural invariants must fail closed
        for forbidden in sorted(FORBIDDEN_SUPPRESS_ANALYZERS):
            bad = empty_calibration_config()
            bad["suppress_findings"] = [{
                "analyzer": forbidden,
                "file_pattern": "src/*.ts",
                "reason": "Trying to suppress invariant analyzer",
            }]
            with self.assertRaises(ValueError, msg=f"Should reject {forbidden}"):
                validate_calibration_payload(bad)

        # Short or missing reason must fail closed
        bad_reason = empty_calibration_config()
        bad_reason["suppress_findings"] = [{
            "analyzer": "topo.risk",
            "file_pattern": "src/*.ts",
            "reason": "short",
        }]
        with self.assertRaises(ValueError):
            validate_calibration_payload(bad_reason)

        # Path traversal and absolute paths must fail closed
        for bad_pat in ("../secret.ts", "/etc/passwd", "C:\\Windows\\System32\\a.ts", "src//a.ts"):
            bad_ep = empty_calibration_config()
            bad_ep["entrypoints"] = [{"pattern": bad_pat, "kind": "custom", "reason": "invalid path check"}]
            with self.assertRaises(ValueError, msg=f"Should reject pattern {bad_pat!r}"):
                validate_calibration_payload(bad_ep)

        # Bad ignore_dir containing slashes or globs
        for bad_dir in ("foo/bar", "..", "*.tmp", "build:out"):
            bad_ig = empty_calibration_config()
            bad_ig["ignore_dirs"] = [bad_dir]
            with self.assertRaises(ValueError):
                validate_calibration_payload(bad_ig)

        # Allowed analyzers validate cleanly
        for allowed in sorted(ALLOWED_SUPPRESS_ANALYZERS):
            good = empty_calibration_config()
            good["suppress_findings"] = [{
                "analyzer": allowed,
                "file_pattern": "src/*.ts",
                "reason": "Bounded project calibration rule for test",
            }]
            res = validate_calibration_payload(good)
            self.assertEqual(res["suppress_findings"][0]["analyzer"], allowed)

    def test_calibration_rules_affect_graph_aliases_entrypoints_tests_and_simplify(self) -> None:
        with tempfile.TemporaryDirectory(prefix="plw_cal_rules_") as tmp:
            root = Path(tmp)
            self._make_sample_repo(root)

            # Baseline graph: generated_sandbox included, @domain/math unresolved
            g0 = build_shared_graph(str(root))
            rels0 = {os.path.relpath(v, root).replace("\\", "/") for v in g0["vertices"]}
            self.assertIn("src/generated_sandbox/junk.ts", rels0)
            self.assertIn("src/domain/orphan_mod.ts", rels0)
            # Without alias, src/index.ts -> src/domain/math.ts edge does not exist
            edges0 = {
                (
                    os.path.relpath(u, root).replace("\\", "/"),
                    os.path.relpath(v, root).replace("\\", "/"),
                )
                for u, v in g0["edges"]
            }
            self.assertNotIn(("src/index.ts", "src/domain/math.ts"), edges0)

            cfg = empty_calibration_config()
            cfg["ignore_dirs"] = ["generated_sandbox"]
            cfg["exclude_paths"] = [{"pattern": "src/domain/orphan_mod.ts", "reason": "Deprecated generated stub pending removal"}]
            cfg["entrypoints"] = [
                {
                    "pattern": "kotlin/NativeBridge.kt",
                    "kind": "jni_bridge",
                    "reason": "Kotlin JNI bridge called dynamically from native runtime",
                },
                {
                    "pattern": "scripts/build_bundle.py",
                    "kind": "tooling",
                    "reason": "Build script verified in packaging workflow",
                },
            ]
            cfg["test_paths"] = [
                {
                    "pattern": "test/global_setup.ts",
                    "role": "test_helper",
                    "reason": "Test suite global bootstrap helper",
                }
            ]
            cfg["import_aliases"] = [
                {
                    "prefix": "@domain/*",
                    "target": "src/domain/*",
                    "reason": "Custom bundler alias outside tsconfig",
                }
            ]
            cfg["style_preferences"] = {
                "simplify_ignore_patterns": ["src/domain/math.ts"],
                "notes": "Keep explicit intermediate variables in math module",
            }
            write_project_calibration(str(root), cfg)

            g1 = build_shared_graph(str(root))
            rels1 = {os.path.relpath(v, root).replace("\\", "/") for v in g1["vertices"]}
            self.assertNotIn("src/generated_sandbox/junk.ts", rels1)
            self.assertNotIn("src/domain/orphan_mod.ts", rels1)

            edges1 = {
                (
                    os.path.relpath(u, root).replace("\\", "/"),
                    os.path.relpath(v, root).replace("\\", "/"),
                )
                for u, v in g1["edges"]
            }
            self.assertIn(("src/index.ts", "src/domain/math.ts"), edges1)

            # Explicit entrypoint calibration overrides cli_shebang on scripts/build_bundle.py
            build_meta = g1["node_metadata"][os.path.realpath(str(root / "scripts" / "build_bundle.py"))]
            self.assertTrue(build_meta["is_entrypoint"])
            self.assertEqual(build_meta["entrypoint_kind"], "project_calibration:tooling")

            # NativeBridge.kt & global_setup.ts leave topo.orphan, and global_setup.ts does not become isolated_test
            orphan_res = analyze_orphan(g1)
            orphan_rels = {os.path.relpath(f["file"], root).replace("\\", "/") for f in orphan_res["findings"]}
            self.assertNotIn("kotlin/NativeBridge.kt", orphan_rels)
            self.assertNotIn("test/global_setup.ts", orphan_rels)

            reach_res = analyze_test_reachability(g1)
            reach_by_file = {os.path.relpath(f["file"], root).replace("\\", "/"): f for f in reach_res["findings"] if f.get("file")}
            self.assertNotIn("scripts/build_bundle.py", reach_by_file)
            self.assertNotIn("test/global_setup.ts", reach_by_file)

            # simplify honors style_preferences.simplify_ignore_patterns
            rc_simp, out_simp = _run_cli("simplify", str(root), "--json")
            self.assertEqual(rc_simp, 0)
            simp_payload = json.loads(out_simp)
            cand_files = [
                os.path.relpath(f, root).replace("\\", "/")
                for row in simp_payload.get("candidates", [])
                for f in row.get("files", [])
            ]
            self.assertNotIn("src/domain/math.ts", cand_files)

    def test_recursive_cli_suggest_add_suppress_stale_prune_cache_and_doctor(self) -> None:
        with tempfile.TemporaryDirectory(prefix="plw_cal_cli_") as tmp:
            root = Path(tmp)
            cache_dir = root / ".tmp_cache"
            self._make_sample_repo(root)

            # Add an internal cycle to confirm topo.circular cannot be suppressed
            (root / "src" / "cyc_a.ts").write_text('import { b } from "./cyc_b"; export const a = b;\n', encoding="utf-8")
            (root / "src" / "cyc_b.ts").write_text('import { a } from "./cyc_a"; export const b = a;\n', encoding="utf-8")

            # Warm caches before calibration exists
            g0 = build_cached_shared_graph(str(root), mode="auto", cache_dir=str(cache_dir))
            a0 = run_cached_analyzers(
                g0,
                mode="auto",
                cache_dir=str(cache_dir / "analyzers"),
            )
            self.assertEqual(len(analyze_circular(g0)["findings"]), 1)
            self.assertEqual(len(a0["results"]["topo.circular"]["findings"]), 1)

            # Swapped-order `plw calibrate <root> suggest --json` must work smoothly
            rc_sug, out_sug = _run_cli("calibrate", str(root), "suggest", "--json")
            self.assertEqual(rc_sug, 0)
            sug = json.loads(out_sug)
            self.assertTrue(sug["authority"]["read_only"])
            self.assertFalse(sug["authority"]["auto_mutates_calibration"])
            cand_patterns = {c["pattern"] for c in sug["candidates"]}
            self.assertIn("kotlin/NativeBridge.kt", cand_patterns)
            self.assertIn("test/global_setup.ts", cand_patterns)
            self.assertIn("scripts/build_bundle.py", cand_patterns)

            # Initialize and apply suggestions via CLI
            rc_init, out_init = _run_cli("calibrate", "init", str(root), "--json")
            self.assertEqual(rc_init, 0)
            self.assertEqual(json.loads(out_init)["status"], "initialized")

            for cand in sug["candidates"]:
                cmd_args = cand["apply_command"][2:]
                rc_add, out_add = _run_cli("calibrate", *cmd_args, "--json")
                self.assertEqual(rc_add, 0, out_add)

            # Re-running suggest after applying rules should clear those suggestions
            rc_sug2, out_sug2 = _run_cli("calibrate", "suggest", str(root), "--json")
            self.assertEqual(rc_sug2, 0)
            remaining_pats = {c["pattern"] for c in json.loads(out_sug2)["candidates"]}
            self.assertNotIn("kotlin/NativeBridge.kt", remaining_pats)
            self.assertNotIn("test/global_setup.ts", remaining_pats)
            self.assertNotIn("scripts/build_bundle.py", remaining_pats)

            # CLI refuses topo.circular suppression fail-closed (exit 2)
            rc_forb, out_forb = _run_cli(
                "calibrate",
                "add",
                str(root),
                "--kind",
                "suppress",
                "--analyzer",
                "topo.circular",
                "--pattern",
                "src/cyc_a.ts",
                "--reason",
                "Attempted suppression of cycle invariant",
                "--json",
            )
            self.assertEqual(rc_forb, 2)
            self.assertIn("forbidden", json.loads(out_forb)["detail"].lower())

            # Unmatched glob rejected unless --allow-unmatched is passed
            rc_unm, out_unm = _run_cli(
                "calibrate",
                "add",
                str(root),
                "--kind",
                "exclude_path",
                "--pattern",
                "src/does_not_exist/*.ts",
                "--reason",
                "Check unmatched glob guard",
                "--json",
            )
            self.assertEqual(rc_unm, 2)
            self.assertEqual(json.loads(out_unm)["error"], "unmatched_pattern")

            # Add bounded suppression on topo.orphan and verify transparent accounting + cache invalidation
            rc_sup, _ = _run_cli(
                "calibrate",
                "add",
                str(root),
                "--kind",
                "suppress",
                "--analyzer",
                "topo.orphan",
                "--pattern",
                "src/domain/orphan_mod.ts",
                "--reason",
                "Loaded dynamically via external plugin manifest",
                "--json",
            )
            self.assertEqual(rc_sup, 0)

            g_sup = build_cached_shared_graph(str(root), mode="auto", cache_dir=str(cache_dir))
            a_sup = run_cached_analyzers(
                g_sup,
                mode="auto",
                cache_dir=str(cache_dir / "analyzers"),
            )
            orphan_sup = a_sup["results"]["topo.orphan"]
            self.assertEqual(orphan_sup["summary"]["calibrated_suppressed_count"], 1)
            self.assertEqual(len(orphan_sup["summary"]["calibrated_suppressed"]), 1)
            orphan_files = {os.path.relpath(f["file"], root).replace("\\", "/") for f in orphan_sup["findings"]}
            self.assertNotIn("src/domain/orphan_mod.ts", orphan_files)

            # Change ONLY suppress_findings again -> analyzer cache MUST miss and reflect removal
            rc_rem, _ = _run_cli(
                "calibrate",
                "remove",
                str(root),
                "--kind",
                "suppress",
                "--analyzer",
                "topo.orphan",
                "--pattern",
                "src/domain/orphan_mod.ts",
                "--json",
            )
            self.assertEqual(rc_rem, 0)
            g_after_rem = build_cached_shared_graph(str(root), mode="auto", cache_dir=str(cache_dir))
            a_after_rem = run_cached_analyzers(
                g_after_rem,
                mode="auto",
                cache_dir=str(cache_dir / "analyzers"),
            )
            self.assertNotIn("calibrated_suppressed_count", a_after_rem["results"]["topo.orphan"]["summary"])
            restored_orphans = {
                os.path.relpath(f["file"], root).replace("\\", "/")
                for f in a_after_rem["results"]["topo.orphan"]["findings"]
            }
            self.assertIn("src/domain/orphan_mod.ts", restored_orphans)

            # Stale detection & pruning
            rc_allow, _ = _run_cli(
                "calibrate",
                "add",
                str(root),
                "--kind",
                "exclude_path",
                "--pattern",
                "legacy/deleted/*.ts",
                "--reason",
                "Temporarily registered obsolete glob",
                "--allow-unmatched",
                "--json",
            )
            self.assertEqual(rc_allow, 0)
            _, st_out = _run_cli("calibrate", "status", str(root), "--json")
            self.assertEqual(json.loads(st_out)["stale_count"], 1)

            rc_prune, pr_out = _run_cli("calibrate", "prune", str(root), "--json")
            self.assertEqual(rc_prune, 0)
            self.assertEqual(json.loads(pr_out)["removed_count"], 1)
            _, st_out2 = _run_cli("calibrate", "status", str(root), "--json")
            self.assertEqual(json.loads(st_out2)["stale_count"], 0)

            # Doctor reports valid calibration; corrupt JSON fails doctor validation
            with mock.patch("sys.platform", "linux"):
                doc_ok = doctor(str(root))
            self.assertTrue(doc_ok["checks"]["project_calibration_valid"])
            self.assertTrue(doc_ok["project_calibration"]["present"])
            self.assertEqual(doc_ok["project_calibration"]["digest"], calibration_digest(load_project_calibration(str(root))["config"]))

            (root / ".plw" / "calibration.json").write_text("{corrupt", encoding="utf-8")
            doc_bad = doctor(str(root))
            self.assertFalse(doc_bad["checks"]["project_calibration_valid"])
            self.assertFalse(doc_bad["project_calibration"]["valid"])

            # Removing calibration clears graph cache state cleanly
            (root / ".plw" / "calibration.json").unlink()
            status = get_graph_cache_status(str(root), cache_dir=str(cache_dir))
            self.assertIn("cache_key", status)



    def test_dry_run_check_and_topology_envelopes(self) -> None:
        with tempfile.TemporaryDirectory(prefix="plw_cal_env_") as tmp:
            root = Path(tmp)
            self._make_sample_repo(root)

            # Dry-run init and add must not write .plw/calibration.json
            rc_di, out_di = _run_cli("calibrate", "init", str(root), "--dry-run", "--json")
            self.assertEqual(rc_di, 0)
            self.assertEqual(json.loads(out_di)["status"], "dry_run")
            self.assertFalse((root / ".plw" / "calibration.json").exists())

            rc_da, out_da = _run_cli(
                "calibrate",
                "add",
                str(root),
                "--kind",
                "suppress",
                "--analyzer",
                "topo.orphan",
                "--pattern",
                "src/domain/orphan_mod.ts",
                "--reason",
                "Dry-run check for bounded orphan suppression",
                "--dry-run",
                "--json",
            )
            self.assertEqual(rc_da, 0)
            self.assertEqual(json.loads(out_da)["status"], "dry_run")
            self.assertFalse((root / ".plw" / "calibration.json").exists())

            # Real write, then verify plw check + plw topology surface calibration metadata cleanly
            rc_real, _ = _run_cli(
                "calibrate",
                "add",
                str(root),
                "--kind",
                "suppress",
                "--analyzer",
                "topo.orphan",
                "--pattern",
                "src/domain/orphan_mod.ts",
                "--reason",
                "Loaded dynamically via external plugin manifest",
                "--json",
            )
            self.assertEqual(rc_real, 0)

            rc_chk, out_chk = _run_cli("check", str(root), "--json")
            self.assertEqual(rc_chk, 0)
            chk = json.loads(out_chk)
            self.assertTrue(chk["unified_summary"]["calibration_valid"])
            self.assertGreaterEqual(chk["unified_summary"]["calibrated_suppressed_count"], 1)

            rc_topo, out_topo = _run_cli("topology", str(root), "--json")
            self.assertEqual(rc_topo, 0)
            topo = json.loads(out_topo)
            cal_orient = topo.get("target_orientation", {}).get("calibration", {})
            self.assertTrue(cal_orient.get("present"))
            self.assertTrue(cal_orient.get("valid"))


    def test_optional_review_hints_for_jni_and_ffi_bridge_entrypoints(self) -> None:
        with tempfile.TemporaryDirectory(prefix="plw_cal_hints_") as tmp:
            root = Path(tmp)
            self._make_sample_repo(root)
            (root / "rust_ffi").mkdir(parents=True, exist_ok=True)
            (root / "rust_ffi" / "bridge.rs").write_text(
                "#[no_mangle]\npub extern \"C\" fn invoke_bridge() -> i32 { 1 }\n",
                encoding="utf-8",
            )
            # Regular untested entrypoint (not a bridge) must NOT receive a bridge review hint
            (root / "src" / "cli_main.ts").write_text(
                "export function runApp(): number { return 1; }\n",
                encoding="utf-8",
            )
            (root / "package.json").write_text(
                json.dumps({"name": "demo", "bin": {"demo": "src/cli_main.ts"}}),
                encoding="utf-8",
            )

            # Before calibration: bridge files appear in structural candidates, review_hints is empty
            rc0, out0 = _run_cli("calibrate", "suggest", str(root), "--json")
            self.assertEqual(rc0, 0)
            s0 = json.loads(out0)
            self.assertEqual(s0["review_hint_count"], 0)
            self.assertEqual(s0["review_hints"], [])
            cand_pats = {c["pattern"] for c in s0["candidates"]}
            self.assertIn("kotlin/NativeBridge.kt", cand_pats)
            self.assertIn("rust_ffi/bridge.rs", cand_pats)

            # Calibrate Kotlin JNI bridge & Rust FFI entry via suggested apply_commands
            self.assertEqual(_run_cli("calibrate", "init", str(root), "--json")[0], 0)
            for cand in s0["candidates"]:
                if cand["pattern"] in ("kotlin/NativeBridge.kt", "rust_ffi/bridge.rs"):
                    rc_a, out_a = _run_cli("calibrate", *cand["apply_command"][2:], "--json")
                    self.assertEqual(rc_a, 0, out_a)

            # Now suggest emits optional review_hints ONLY for the calibrated bridge entrypoints
            rc1, out1 = _run_cli("calibrate", "suggest", str(root), "--json")
            self.assertEqual(rc1, 0)
            s1 = json.loads(out1)
            self.assertEqual(s1["review_hint_count"], 2)
            hint_by_pat = {h["pattern"]: h for h in s1["review_hints"]}
            self.assertEqual(set(hint_by_pat.keys()), {"kotlin/NativeBridge.kt", "rust_ffi/bridge.rs"})
            self.assertNotIn("src/cli_main.ts", hint_by_pat)
            self.assertTrue(hint_by_pat["kotlin/NativeBridge.kt"]["optional"])
            self.assertEqual(hint_by_pat["kotlin/NativeBridge.kt"]["analyzer"], "topo.test_reachability")
            self.assertEqual(hint_by_pat["kotlin/NativeBridge.kt"]["rule_type"], "high_influence_without_test_path")
            self.assertEqual(hint_by_pat["kotlin/NativeBridge.kt"]["entrypoint_kind"], "project_calibration:jni_bridge")
            self.assertEqual(hint_by_pat["rust_ffi/bridge.rs"]["entrypoint_kind"], "project_calibration:ffi_entry")

            # Human output renders optional review hints section with shell-quoted reason
            rc_h, out_h = _run_cli("calibrate", "suggest", str(root))
            self.assertEqual(rc_h, 0)
            self.assertIn("Optional review hints", out_h)
            self.assertIn("kotlin/NativeBridge.kt", out_h)

            # Applying one optional hint clears ONLY that hint and records transparent suppression
            kt_cmd = hint_by_pat["kotlin/NativeBridge.kt"]["apply_command"][2:]
            rc_sup, out_sup = _run_cli("calibrate", *kt_cmd, "--json")
            self.assertEqual(rc_sup, 0, out_sup)

            rc2, out2 = _run_cli("calibrate", "suggest", str(root), "--json")
            self.assertEqual(rc2, 0)
            s2 = json.loads(out2)
            self.assertEqual(s2["review_hint_count"], 1)
            self.assertEqual(s2["review_hints"][0]["pattern"], "rust_ffi/bridge.rs")

            # Also verify an already-suppressed orphan is not re-suggested in candidates
            rc_sup_or, _ = _run_cli(
                "calibrate",
                "add",
                str(root),
                "--kind",
                "suppress",
                "--analyzer",
                "topo.orphan",
                "--pattern",
                "test/global_setup.ts",
                "--reason",
                "Suppressed orphan fixture must not re-appear in suggest",
                "--json",
            )
            self.assertEqual(rc_sup_or, 0)
            rc3, out3 = _run_cli("calibrate", "suggest", str(root), "--json")
            self.assertEqual(rc3, 0)
            self.assertNotIn("test/global_setup.ts", {c["pattern"] for c in json.loads(out3)["candidates"]})

if __name__ == "__main__":
    unittest.main()
