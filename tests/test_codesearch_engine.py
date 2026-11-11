#!/usr/bin/env python3
"""Unit tests for the codesearch adapter (core/codesearch_engine.py).

G4: the adapter previously had no tests in the repo — CI only touched
it end-to-end through the built binary. These tests lock the adapter
logic directly: discovery, the version gate order (parse -> schema ->
digest pin), the opt-out, digest caching, callee reduction, batch
fallback, index-command refusal, and the never-raise doctor path.

Stubs are small executable Python programs written into a temp dir.
They answer --version (schema/commit controllable via env), query
modes, and index commands with fixed JSON — no real engine, no
network, no fixture repo. The pin-accept case patches
PINNED_ENGINE_DIGESTS with the stub's own digest (computed in the
test), so the suite never depends on a real pinned binary.

Run from the repo root, after unpacking the runtime source (the
adapter only exists in the unpacked tree):
    python3 -m unittest discover -s tests -v
"""

from __future__ import annotations

import hashlib
import json
import os
import stat
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path
from unittest import mock

import core.codesearch_engine as ce

STUB_SOURCE = textwrap.dedent(
    """\
    #!/usr/bin/env python3
    import json, os, sys, time

    args = sys.argv[1:]
    env = os.environ

    def has(flag):
        return flag in args

    def value_after(flag):
        try:
            return args[args.index(flag) + 1]
        except (ValueError, IndexError):
            return ""

    # --version answers immediately (even for the sleep variant: the
    # version gate must stay fast while queries may hang).
    if has("--version"):
        if env.get("STUB_NO_CACHE_DIR") == "1" and has("--cache-dir"):
            sys.exit(2)
        schema = env.get("STUB_SCHEMA", "go9")
        commit = env.get("STUB_COMMIT", "abc123def456")
        print(f"codesearch schema={schema} commit={commit} built=unknown go=go1.27.3")
        sys.exit(int(env.get("STUB_VERSION_EXIT", "0")))

    if env.get("STUB_SLEEP"):
        time.sleep(float(env["STUB_SLEEP"]))
    if env.get("STUB_BAD_JSON") == "1":
        print("this is not json")
        sys.exit(0)
    if env.get("STUB_QUERY_EXIT"):
        print("stub refuses the query", file=sys.stderr)
        sys.exit(int(env["STUB_QUERY_EXIT"]))

    def hit(name):
        return {"path": "a.go", "start": 1, "end": 3, "score": 1.0,
                "snippet": f"func {name}() {{}}"}

    if has("--index-status"):
        print(json.dumps({"exists": True, "schema": "go9", "files": 3,
                          "chunks": 10, "size_bytes": 1234,
                          "stale": {"changed": [], "added": [], "removed": []},
                          "index_path": "/tmp/stub/index.gob"}))
        sys.exit(0)
    if has("--index-warm") or has("--index-clean"):
        print(json.dumps({"ok": True}))
        sys.exit(0)

    if has("--defs-only"):
        names = []
        grab = False
        for a in args:
            if a == "--defs-only":
                grab = True
                continue
            if a == "--json":
                break
            if grab:
                names.append(a)
        if env.get("STUB_DEFS_FLAT") == "1" and len(names) == 1:
            print(json.dumps({"total": 1, "shown": 1, "hits": [hit(names[0])]}))
        else:
            print(json.dumps({"queries": [
                {"query": n, "total": 1, "hits": [hit(n)]} for n in names]}))
        sys.exit(0)

    if has("--callers"):
        names = value_after("--callers").split(",")
        rows = [{"path": "a.go", "line": 10, "caller": "main",
                 "callee": f"a.{n}"} for n in names if n]
        out = {"total": len(rows), "shown": len(rows), "calls": rows}
        if env.get("STUB_BATCH") == "1":
            out["batch"] = True
        print(json.dumps(out))
        sys.exit(0)

    if has("--related"):
        names = value_after("--related").split(",")
        rows = [{"path": "a.go", "line": 4, "symbol": n, "kind": "def",
                 "detail": f"defined in a.go"} for n in names if n]
        out = {"total": len(rows), "shown": len(rows), "related": rows}
        if env.get("STUB_BATCH") == "1":
            out["batch"] = True
        print(json.dumps(out))
        sys.exit(0)

    # plain search: first positional arg is the query
    query = next((a for a in args if not a.startswith("--")), "")
    print(json.dumps({"total": 1, "shown": 1, "hits": [hit(query or "Greet")]}))
    """
)


class AdapterTestCase(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.fixture = self.root / "target"
        self.fixture.mkdir()
        (self.fixture / "a.go").write_text("package a\nfunc Greet() {}\n")
        self.env_patch = mock.patch.dict(os.environ, {}, clear=False)
        self.env_patch.start()
        os.environ.pop("CODESEARCH_BIN", None)
        os.environ.pop("CODESEARCH_CACHE_DIR", None)
        os.environ.pop("CODESEARCH_ALLOW_UNPINNED", None)
        # Isolate PLW cache resolution from the real home dir.
        self.home = self.root / "home"
        self.home.mkdir()
        os.environ["HOME"] = str(self.home)
        os.environ.pop("XDG_CACHE_HOME", None)
        os.environ.pop("PLW_CACHE_HOME", None)
        os.environ.pop("PLW_HOME", None)

    def tearDown(self):
        self.env_patch.stop()
        self.tmp.cleanup()

    # -- helpers -----------------------------------------------------
    def stub(self, name="stub.py", env=None):
        path = self.root / name
        path.write_text(STUB_SOURCE)
        path.chmod(path.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
        if env:
            os.environ.update(env)
        return path

    def digest_of(self, path):
        return hashlib.sha256(Path(path).read_bytes()).hexdigest()

    def pinned(self, path):
        """Context manager patching PINNED_ENGINE_DIGESTS with this stub."""
        return mock.patch.dict(
            ce.PINNED_ENGINE_DIGESTS, {self.digest_of(path): "test stub"}
        )

    # -- T1: discovery ----------------------------------------------
    def test_discover_explicit_missing(self):
        with self.assertRaises(ce.EngineError) as ctx:
            ce.discover_codesearch(explicit="/nonexistent/codesearch")
        self.assertIn("not found at explicit path", str(ctx.exception))

    def test_discover_explicit_ok(self):
        stub = self.stub()
        path, via = ce.discover_codesearch(explicit=str(stub))
        self.assertEqual(path, stub)
        self.assertEqual(via, "explicit")

    def test_discover_env_ok(self):
        stub = self.stub()
        os.environ["CODESEARCH_BIN"] = str(stub)
        path, via = ce.discover_codesearch()
        self.assertEqual(via, "CODESEARCH_BIN")

    def test_discover_env_missing(self):
        os.environ["CODESEARCH_BIN"] = "/nonexistent/codesearch"
        with self.assertRaises(ce.EngineError) as ctx:
            ce.discover_codesearch()
        self.assertIn("CODESEARCH_BIN", str(ctx.exception))

    def test_discover_none_found(self):
        with mock.patch("core.codesearch_engine.shutil.which", return_value=None):
            with self.assertRaises(ce.EngineError) as ctx:
                ce.discover_codesearch()
        self.assertIn("not found", str(ctx.exception))

    # -- T1b: F1 PATH candidates --------------------------------------
    def test_discover_path_tries_release_names_in_order(self):
        # Only codesearch-go on PATH: found (the user's exact case).
        def which_go(name):
            return "/opt/bin/codesearch-go" if name == "codesearch-go" else None
        with mock.patch("core.codesearch_engine.shutil.which", side_effect=which_go):
            path, via = ce.discover_codesearch()
        self.assertEqual(str(path), "/opt/bin/codesearch-go")
        self.assertEqual(via, "PATH")

    def test_discover_path_tries_release_asset_name(self):
        def which_asset(name):
            return "/opt/bin/codesearch-go-linux-x86_64" \
                if name == "codesearch-go-linux-x86_64" else None
        with mock.patch("core.codesearch_engine.shutil.which", side_effect=which_asset):
            path, via = ce.discover_codesearch()
        self.assertEqual(str(path), "/opt/bin/codesearch-go-linux-x86_64")
        self.assertEqual(via, "PATH")

    def test_discover_path_bare_name_wins(self):
        # All three present: the bare name stays first, so existing
        # setups keep working.
        def which_all(name):
            return f"/opt/bin/{name}"
        with mock.patch("core.codesearch_engine.shutil.which", side_effect=which_all):
            path, via = ce.discover_codesearch()
        self.assertEqual(str(path), "/opt/bin/codesearch")
        self.assertEqual(via, "PATH")

    def test_discover_path_error_names_all_candidates(self):
        with mock.patch("core.codesearch_engine.shutil.which", return_value=None):
            with self.assertRaises(ce.EngineError) as ctx:
                ce.discover_codesearch()
        msg = str(ctx.exception)
        for name in ce.PATH_CANDIDATES:
            self.assertIn(repr(name), msg)

    def test_discover_candidates_constant(self):
        self.assertEqual(
            ce.PATH_CANDIDATES,
            ("codesearch", "codesearch-go", "codesearch-go-linux-x86_64"),
        )

    # -- T2/T3/T4/T5: version gate -----------------------------------
    def test_version_pinned_stub_accepted(self):
        stub = self.stub()
        with self.pinned(stub):
            info = ce.version_info(stub)
        self.assertTrue(info["pinned"])
        self.assertEqual(info["pin_label"], "test stub")
        self.assertEqual(info["schema"], "go9")
        self.assertEqual(info["digest"], self.digest_of(stub))

    def test_version_stub_claiming_pinned_commit_rejected(self):
        # G2 falsifier, direct: the stub prints the pinned commit
        # (d81e06b64a49) but its bytes are not a pinned digest.
        stub = self.stub(env={"STUB_COMMIT": "d81e06b64a49"})
        with self.assertRaises(ce.EngineError) as ctx:
            ce.version_info(stub)
        self.assertIn("not pinned", str(ctx.exception))
        self.assertIn("digest", str(ctx.exception))

    def test_version_schema_checked_before_digest(self):
        # go10 stub with an unpinned digest must fail with the schema
        # message (G9 W5), not the pin message.
        stub = self.stub(env={"STUB_SCHEMA": "go10"})
        with self.assertRaises(ce.EngineError) as ctx:
            ce.version_info(stub)
        self.assertIn("go10", str(ctx.exception))
        self.assertIn("engine is newer", str(ctx.exception))
        self.assertNotIn("not pinned", str(ctx.exception))

    def test_version_old_schema_message(self):
        stub = self.stub(env={"STUB_SCHEMA": "go8"})
        with self.assertRaises(ce.EngineError) as ctx:
            ce.version_info(stub)
        self.assertIn("engine is older", str(ctx.exception))

    def test_version_unparsable(self):
        stub = self.stub(env={"STUB_SCHEMA": "go9"})
        # Rewrite the version line to something unparsable by making
        # the stub print garbage for --version: reuse STUB_BAD_JSON
        # path is query-only, so patch the stub text instead.
        text = Path(stub).read_text().replace(
            'print(f"codesearch schema={schema} commit={commit} built=unknown go=go1.27.3")',
            'print("hello, no version here")',
        )
        Path(stub).write_text(text)
        with self.assertRaises(ce.EngineError) as ctx:
            ce.version_info(stub)
        self.assertIn("unparsable", str(ctx.exception))

    def test_version_nonzero_exit(self):
        stub = self.stub(env={"STUB_VERSION_EXIT": "3"})
        with self.assertRaises(ce.EngineError) as ctx:
            ce.version_info(stub)
        self.assertIn("exited 3", str(ctx.exception))

    def test_version_opt_out_records_unpinned(self):
        stub = self.stub()
        os.environ["CODESEARCH_ALLOW_UNPINNED"] = "1"
        info = ce.version_info(stub)
        self.assertFalse(info["pinned"])
        self.assertIn("pin_warning", info)
        self.assertIn("CODESEARCH_ALLOW_UNPINNED=1", info["pin_warning"])

    def test_version_opt_out_other_value_is_not_opt_out(self):
        stub = self.stub()
        os.environ["CODESEARCH_ALLOW_UNPINNED"] = "yes"
        with self.assertRaises(ce.EngineError):
            ce.version_info(stub)

    # -- T6: digest cache --------------------------------------------
    def test_engine_digest_cache_and_invalidation(self):
        stub = self.stub()
        first = ce._engine_digest(stub)
        second = ce._engine_digest(stub)
        self.assertEqual(first, second)
        # One byte appended: size changes, so the cache key changes
        # and the digest is recomputed.
        with open(stub, "ab") as f:
            f.write(b"#")
        third = ce._engine_digest(stub)
        self.assertNotEqual(first, third)
        self.assertEqual(third, hashlib.sha256(Path(stub).read_bytes()).hexdigest())

    def test_engine_digest_unreadable_is_none(self):
        # A directory cannot be read as a file; a missing path cannot
        # be stat'ed. Both are "unpinned", never "pinned".
        self.assertIsNone(ce._engine_digest(self.root))
        self.assertIsNone(ce._engine_digest(self.root / "nope"))

    # -- T7: callee reduction -----------------------------------------
    def test_callee_base(self):
        cases = {
            "a.Run": "Run",
            "a::b::Run": "Run",
            "Foo[T]": "Foo",
            "pkg.Foo[int]": "Foo",
            "Run": "Run",
            "": "",
            "a.": "a",
            "a..b": "b",
        }
        for expr, want in cases.items():
            with self.subTest(expr=expr):
                self.assertEqual(ce._callee_base(expr), want)
        self.assertEqual(ce._callee_base(None), "")
        self.assertEqual(ce._callee_base(42), "")

    # -- T8: run_query -------------------------------------------------
    def _run(self, mode, stub, **kw):
        return ce.run_query(
            mode, self.fixture, kw.pop("query", "Greet"),
            stub, "explicit", kw.pop("timeout_s", 30), **kw,
        )

    def test_run_query_defs(self):
        stub = self.stub()
        with self.pinned(stub):
            out = self._run("defs", stub)
        self.assertTrue(out["ok"])
        self.assertEqual(out["mode"], "defs")
        # The defs envelope carries per-query totals, not a top-level
        # one: envelope() derives summary.total only from calls/
        # related. Lock that behaviour rather than assuming a total.
        self.assertIsNone(out["summary"]["total"])
        self.assertEqual(out["engine_result"]["queries"][0]["total"], 1)
        self.assertEqual(out["engine_result"]["queries"][0]["query"], "Greet")
        self.assertIn("leads", out["authority_note"])

    def test_run_query_search_with_limit(self):
        stub = self.stub()
        with self.pinned(stub):
            out = ce.run_query("search", self.fixture, "Greet", stub,
                               "explicit", 30, limit=5)
        self.assertTrue(out["ok"])
        self.assertEqual(out["engine_result"]["hits"][0]["path"], "a.go")

    def test_run_query_unknown_mode(self):
        stub = self.stub()
        with self.pinned(stub):
            with self.assertRaises(ce.EngineError) as ctx:
                ce.run_query("nope", self.fixture, "Greet", stub, "explicit", 30)
        self.assertIn("unknown mode", str(ctx.exception))

    def test_run_query_non_json_stdout(self):
        stub = self.stub(env={"STUB_BAD_JSON": "1"})
        with self.pinned(stub):
            with self.assertRaises(ce.EngineError) as ctx:
                self._run("search", stub)
        self.assertIn("not JSON", str(ctx.exception))

    def test_run_query_nonzero_exit(self):
        stub = self.stub(env={"STUB_QUERY_EXIT": "2"})
        with self.pinned(stub):
            with self.assertRaises(ce.EngineError) as ctx:
                self._run("search", stub)
        self.assertIn("exited 2", str(ctx.exception))

    def test_run_query_timeout(self):
        stub = self.stub(env={"STUB_SLEEP": "5"})
        with self.pinned(stub):
            with self.assertRaises(ce.EngineError) as ctx:
                self._run("search", stub, timeout_s=1)
        self.assertIn("timed out after 1s", str(ctx.exception))

    def test_run_query_root_must_be_dir(self):
        stub = self.stub()
        with self.pinned(stub):
            with self.assertRaises(ce.EngineError):
                ce.run_query("search", self.root / "nope", "Greet",
                             stub, "explicit", 30)

    # -- T9: batch + fallback ------------------------------------------
    def test_batch_callers_with_marker(self):
        stub = self.stub(env={"STUB_BATCH": "1"})
        with self.pinned(stub):
            out = ce.run_batch_query("callers", self.fixture, ["Run", "Greet"],
                                     stub, "explicit", 30)
        self.assertEqual(out["batch_mode"], "batch")
        self.assertEqual([r["callee"] for r in out["grouped"]["Run"]], ["a.Run"])
        self.assertEqual([r["callee"] for r in out["grouped"]["Greet"]], ["a.Greet"])

    def test_batch_callers_without_marker_falls_back(self):
        # Old engine: answers a comma-joined --callers as one name and
        # returns rows without the batch marker. Must fall back to one
        # query per name instead of presenting the empty batch.
        stub = self.stub()
        with self.pinned(stub):
            out = ce.run_batch_query("callers", self.fixture, ["Run", "Greet"],
                                     stub, "explicit", 30)
        self.assertEqual(out["batch_mode"], "fallback")
        self.assertEqual(len(out["grouped"]["Run"]), 1)
        self.assertEqual(len(out["grouped"]["Greet"]), 1)
        self.assertEqual(out["summary"]["total"], 2)

    def test_batch_defs_flat_single_name(self):
        stub = self.stub(env={"STUB_DEFS_FLAT": "1"})
        with self.pinned(stub):
            out = ce.run_batch_query("defs", self.fixture, ["Greet"],
                                     stub, "explicit", 30)
        self.assertEqual(out["batch_mode"], "batch")
        self.assertEqual(len(out["grouped"]["Greet"]), 1)

    def test_batch_requires_a_query(self):
        stub = self.stub()
        with self.pinned(stub):
            with self.assertRaises(ce.EngineError):
                ce.run_batch_query("defs", self.fixture, [], stub, "explicit", 30)
            with self.assertRaises(ce.EngineError):
                ce.run_batch_query("search", self.fixture, ["Greet"],
                                   stub, "explicit", 30)

    def test_batch_related_grouping(self):
        stub = self.stub(env={"STUB_BATCH": "1"})
        with self.pinned(stub):
            out = ce.run_batch_query("related", self.fixture, ["Greet", "Run"],
                                     stub, "explicit", 30)
        self.assertEqual(out["batch_mode"], "batch")
        self.assertEqual(out["grouped"]["Greet"][0]["symbol"], "Greet")
        self.assertEqual(out["grouped"]["Run"][0]["symbol"], "Run")

    # -- T10: index commands -------------------------------------------
    def test_index_status(self):
        stub = self.stub()
        with self.pinned(stub):
            out = ce.run_index_command("status", self.fixture, stub, "explicit", 30)
        self.assertTrue(out["ok"])
        self.assertEqual(out["mode"], "index")
        self.assertEqual(out["operation"], "status")
        self.assertTrue(out["engine_result"]["exists"])
        self.assertEqual(out["engine_result"]["files"], 3)

    def test_index_unknown_operation(self):
        stub = self.stub()
        with self.pinned(stub):
            with self.assertRaises(ce.EngineError) as ctx:
                ce.run_index_command("explode", self.fixture, stub, "explicit", 30)
        self.assertIn("unknown index operation", str(ctx.exception))

    def test_index_old_engine_refused_plainly(self):
        stub = self.stub(env={"STUB_NO_CACHE_DIR": "1"})
        with self.pinned(stub):
            with self.assertRaises(ce.EngineError) as ctx:
                ce.run_index_command("status", self.fixture, stub, "explicit", 30)
        self.assertIn("predates index management", str(ctx.exception))

    # -- T11: doctor never raises ---------------------------------------
    def test_doctor_missing_binary(self):
        out = ce.doctor_status(explicit="/nonexistent/codesearch",
                               root=self.fixture)
        self.assertFalse(out["available"])
        self.assertIn("reason", out)
        self.assertIsNone(out["binary"])
        self.assertEqual(out["cache_dir"], str(ce.resolve_cache_dir()))

    def test_doctor_schema_mismatch_fields(self):
        stub = self.stub(env={"STUB_SCHEMA": "go10"})
        out = ce.doctor_status(explicit=str(stub), root=self.fixture)
        self.assertFalse(out["available"])
        self.assertEqual(out.get("engine_schema"), "go10")
        self.assertEqual(out.get("known_schema"), "go9")

    def test_doctor_envelope(self):
        stub = self.stub()
        with self.pinned(stub):
            out = ce.doctor_envelope(self.fixture, explicit=str(stub))
        self.assertTrue(out["ok"])
        self.assertTrue(out["engine"]["available"])
        self.assertTrue(out["engine"]["pinned"])
        self.assertEqual(out["engine"]["engine_digest"],
                         self.digest_of(stub)[:12])
        self.assertIn("authority_note", out)

    # -- T12: cache-dir probe never raises ------------------------------
    def test_cache_dir_probe_false_on_nonzero_exit(self):
        stub = self.stub(env={"STUB_NO_CACHE_DIR": "1"})
        self.assertFalse(ce._cache_dir_supported(stub, self.root / "cache"))

    def test_cache_dir_probe_true_for_new_engine(self):
        stub = self.stub()
        self.assertTrue(ce._cache_dir_supported(stub, self.root / "cache"))

    # -- T13: cache-dir precedence ---------------------------------------
    def test_resolve_cache_dir_precedence(self):
        explicit = self.root / "explicit-cache"
        self.assertEqual(ce.resolve_cache_dir(str(explicit)), explicit.resolve())
        os.environ["CODESEARCH_CACHE_DIR"] = str(self.root / "env-cache")
        self.assertEqual(
            ce.resolve_cache_dir(),
            (self.root / "env-cache").resolve(),
        )
        default = ce.resolve_cache_dir()
        # explicit beats env
        self.assertEqual(ce.resolve_cache_dir(str(explicit)), explicit.resolve())
        os.environ.pop("CODESEARCH_CACHE_DIR", None)
        self.assertTrue(str(ce.resolve_cache_dir()).endswith("codebase/codesearch"))

    # -- T14: envelopes ---------------------------------------------------
    def test_envelopes(self):
        engine = {"name": "codesearch-go"}
        out = ce.envelope("search", self.fixture, "Greet", engine,
                          {"total": 2, "shown": 1, "hits": []})
        self.assertTrue(out["ok"])
        self.assertEqual(out["summary"], {"total": 2, "shown": 1})
        self.assertIn("authority_note", out)
        # total derived from calls/related when absent
        out = ce.envelope("callers", self.fixture, "Run", engine,
                          {"calls": [{}, {}]})
        self.assertEqual(out["summary"]["total"], 2)
        err = ce.error_envelope("search", str(self.fixture), "Greet", "boom")
        self.assertFalse(err["ok"])
        self.assertEqual(err["error"], "boom")
        self.assertIsNone(err["engine"])
        self.assertIn("authority_note", err)


if __name__ == "__main__":
    unittest.main()
