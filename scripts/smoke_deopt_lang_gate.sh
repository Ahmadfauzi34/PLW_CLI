#!/usr/bin/env bash
# Smoke: perf.deopt language gate + string blanking (G4).
#
# Standalone regression for the G4 false positive: `analyze_deopt` used to
# scan every file in file_map with JS-only regexes and no string handling,
# so a Kotlin `file.delete()` line — and the English word "delete" inside
# the string "Can't delete $FILE" on that same line — was reported as a V8
# `delete` operator (HelperPairing.kt, oopbot-trial audit).
#
# Runs analyze_deopt directly against an in-memory file_map (no binary,
# no graph build) and asserts exact finding counts:
#   - .kt/.rs files: 0 findings (language gate)
#   - JS/TS strings mentioning delete/eval: 0 findings (per-line blanking)
#   - a real `delete obj.prop` on the line AFTER a "don't" string line:
#     still found (proves blanking is per-line, not whole-content, so an
#     apostrophe inside a double-quoted string cannot hide later code)
#   - real JS/TS `delete` / `eval(` / `new Function(`: still found
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/project"
cat > "$TMP/project/a.kt" <<'EOF'
internal class Store(private val file: File) {
  fun clear() {
    if (file.exists() && !file.delete()) throw IOException("Can't delete $FILE")
  }
  fun raw(): String {
    val s = """delete the file"""
    return s
  }
  fun useEval(): String = "eval(x)"
}
EOF

cat > "$TMP/project/b.js" <<'EOF'
delete obj.prop;
const x = eval("1+1");
const f = new Function("x", "return x");
EOF

cat > "$TMP/project/c.js" <<'EOF'
const s = "please delete the file";
const t = 'eval(x)';
const a = "don't";
delete obj.y;
EOF

cat > "$TMP/project/d.rs" <<'EOF'
fn clear(file: &std::fs::File) {
    let _ = file.delete();
}
EOF

cat > "$TMP/project/e.ts" <<'EOF'
delete cache.entry;
EOF

python3 - "$ROOT" "$TMP/project" <<'PY'
import sys
from pathlib import Path

root, fixture = sys.argv[1], Path(sys.argv[2])
sys.path.insert(0, root)

from codebase.performance_analyzers import analyze_deopt

file_map = {
    p.name: p.read_text(encoding="utf-8")
    for p in sorted(fixture.iterdir())
}
result = analyze_deopt({"file_map": file_map})
findings = result.get("findings", [])
by_file = {}
for f in findings:
    by_file.setdefault(f["file"], []).append(f)

expected = {"b.js": 3, "c.js": 1, "e.ts": 1}
if set(by_file) != set(expected):
    raise SystemExit(
        f"deopt files mismatch: got {sorted(by_file)} expected {sorted(expected)}: "
        f"{findings}"
    )
for name, count in expected.items():
    if len(by_file[name]) != count:
        raise SystemExit(
            f"deopt count mismatch for {name}: got {len(by_file[name])} expected {count}"
        )

types = sorted(f["type"] for f in by_file["b.js"])
if types != ["delete_operator", "eval_usage", "new_function"]:
    raise SystemExit(f"b.js types mismatch: {types}")
if by_file["c.js"][0]["type"] != "delete_operator":
    raise SystemExit(f"c.js finding is not the real delete: {by_file['c.js']}")
if result.get("analyzer") != "perf.deopt":
    raise SystemExit(f"analyzer field mismatch: {result.get('analyzer')}")
if result["summary"]["total_findings"] != 5:
    raise SystemExit(f"total mismatch: {result['summary']}")

print("SMOKE_DEOPT_LANG_GATE: PASS")
PY
