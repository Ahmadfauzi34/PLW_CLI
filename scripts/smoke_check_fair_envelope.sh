#!/usr/bin/env bash
# Smoke: fair per-analyzer selection in the check envelope (G6 opsi A).
#
# `_top_check_findings` used to sort ALL actionable findings globally
# and cut the first 100, so one analyzer (perf.async, 458 high) filled
# the envelope and deopt/gc/circular/risk never appeared. The fix
# round-robins one finding per analyzer (each analyzer's findings
# severity-sorted internally), then re-sorts the selected set by the
# documented order — so the envelope stays sorted, only its
# membership becomes fair across analyzers. Low/info still never
# enter the envelope (they stay in the analyzer cache, by design).
#
# Fixture (synthetic analyzer output, no graph build):
#   A: 5 high, B: 5 high, C: 3 medium, D: 2 low, E: 1 info
#   limit 4  -> A,B,C,A (round 0: A,B,C; round 1: A), sorted for output
#   limit 100 -> all 13 actionable, truncated False, D/E still excluded
#   limit None (--all) -> all 13 actionable, truncated False
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

python3 - "$ROOT" <<'PY'
import sys

sys.path.insert(0, sys.argv[1])

from hott_kernel import _top_check_findings

RANK = {"critical": 0, "error": 0, "high": 1, "medium": 2}


def mk(analyzer, severity, n):
    return [
        {"severity": severity, "file": f"{analyzer}-{i}.ts", "message": f"{analyzer} {i}"}
        for i in range(n)
    ]


analyzer_output = {"results": {
    "a.aaa": {"findings": mk("a", "high", 5)},
    "b.bbb": {"findings": mk("b", "high", 5)},
    "c.ccc": {"findings": mk("c", "medium", 3)},
    "d.ddd": {"findings": mk("d", "low", 2)},
    "e.eee": {"findings": mk("e", "info", 1)},
}}

# --- limit 4: fair across analyzers, sorted on output ---
sel, trunc = _top_check_findings(analyzer_output, limit=4)
got = [(f["analyzer"], f["severity"]) for f in sel]
want = [("a.aaa", "high"), ("a.aaa", "high"), ("b.bbb", "high"), ("c.ccc", "medium")]
if got != want:
    raise SystemExit(f"limit=4 selection = {got}, expected {want}")
if trunc is not True:
    raise SystemExit("limit=4 must be truncated (13 actionable > 4)")
keys = [(RANK[f["severity"]], f["analyzer"], f.get("file", "")) for f in sel]
if keys != sorted(keys):
    raise SystemExit("selected set must be sorted by (severity, analyzer, file)")

# --- limit 100: everything actionable, low/info still excluded ---
sel, trunc = _top_check_findings(analyzer_output, limit=100)
if len(sel) != 13 or trunc is not False:
    raise SystemExit(f"limit=100: n={len(sel)} truncated={trunc}, expected 13/False")
if {f["severity"] for f in sel} != {"high", "medium"}:
    raise SystemExit("low/info must never enter the envelope")

# --- limit None (--all): same, uncapped ---
sel, trunc = _top_check_findings(analyzer_output, limit=None)
if len(sel) != 13 or trunc is not False:
    raise SystemExit(f"limit=None: n={len(sel)} truncated={trunc}, expected 13/False")

print("SMOKE_CHECK_FAIR_ENVELOPE: PASS")
PY
