#!/usr/bin/env bash
# Smoke: TS compiled-extension import mapping (G3a).
#
# TypeScript ESM source imports the compiled name (`./framing.js`) while
# the file on disk is the source (`./framing.ts`). `candidate_targets`
# used to return only the literal path for extension-bearing bases, so
# all 500 such imports on oopbot-trial were unresolved. The fix adds the
# compiled->source mapping as a fallback candidate (literal first: a
# real `.js` file next to the `.ts` source still wins).
#
# Builds a small fixture graph with build_shared_graph and asserts:
#   - `./b.js` resolves to b.ts (mapping)
#   - `./c.js` resolves to c.js, not c.ts (literal wins when both exist)
#   - `./e.jsx` resolves to e.tsx, `./g.mjs` resolves to g.mts
#   - `./pkg.json` and `./missing.js` stay unresolved (JSON is not a
#     source module; a missing source stays honestly unresolved)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/project"
cat > "$TMP/project/a.ts" <<'EOF'
import { f } from "./b.js";
import { g } from "./c.js";
export const a = f() + g();
EOF
cat > "$TMP/project/b.ts" <<'EOF'
export function f(): number { return 1; }
EOF
cat > "$TMP/project/c.ts" <<'EOF'
export function g(): number { return 2; }
EOF
cat > "$TMP/project/c.js" <<'EOF'
export function g() { return 3; }
EOF
cat > "$TMP/project/d.tsx" <<'EOF'
import { h } from "./e.jsx";
export const d = h();
EOF
cat > "$TMP/project/e.tsx" <<'EOF'
export function h(): number { return 4; }
EOF
cat > "$TMP/project/f.mts" <<'EOF'
import { i } from "./g.mjs";
export const f2 = i();
EOF
cat > "$TMP/project/g.mts" <<'EOF'
export function i(): number { return 5; }
EOF
cat > "$TMP/project/h.ts" <<'EOF'
import data from "./pkg.json";
export const h2 = data;
EOF
cat > "$TMP/project/pkg.json" <<'EOF'
{}
EOF
cat > "$TMP/project/i.ts" <<'EOF'
import { z } from "./missing.js";
export const i2 = z;
EOF

python3 - "$ROOT" "$TMP/project" <<'PY'
import sys
from pathlib import Path

root, fixture = sys.argv[1], Path(sys.argv[2])
sys.path.insert(0, root)

from core.shared_graph import build_shared_graph

g = build_shared_graph(str(fixture))
edges = {(Path(u).name, Path(v).name) for u, v in g["edges"]}
unresolved = {(Path(r["importer"]).name, r["raw_import"]) for r in g["unresolved_imports"]}

want_edges = {
    ("a.ts", "b.ts"),
    ("a.ts", "c.js"),
    ("d.tsx", "e.tsx"),
    ("f.mts", "g.mts"),
}
missing = want_edges - edges
if missing:
    raise SystemExit(f"missing edges: {sorted(missing)} (got {sorted(edges)})")
if ("a.ts", "c.ts") in edges:
    raise SystemExit("literal .js lost to .ts: a.ts -> c.ts must not exist")

want_unresolved = {("h.ts", "./pkg.json"), ("i.ts", "./missing.js")}
if unresolved != want_unresolved:
    raise SystemExit(
        f"unresolved mismatch: got {sorted(unresolved)} expected {sorted(want_unresolved)}"
    )
if len(g["unresolved_imports"]) != 2:
    raise SystemExit(f"unresolved count: {len(g['unresolved_imports'])}")

print("SMOKE_TS_JS_EXT_MAP: PASS")
PY
