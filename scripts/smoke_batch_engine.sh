#!/usr/bin/env bash
# Standalone smoke test for batch engine mode (overlay batch_engine_v1):
# the engine answers comma-separated --callers/--related names and
# multi-query --defs-only in one scan; `plw ast` accepts a comma-separated
# query; `plw topology` covers the whole repo in one batch (scope
# whole-repo) instead of the 20-symbol target-grounded cap.
#
# Usage: bash scripts/smoke_batch_engine.sh [plw-binary]
# The engine binary comes from CODESEARCH_BIN or PATH.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${1:-$ROOT/artifacts/plw-linux-x86_64}"

if [[ ! -x "$BIN" ]]; then
  echo "binary not executable: $BIN" >&2
  exit 2
fi

ENGINE="${CODESEARCH_BIN:-$(command -v codesearch || true)}"
if [[ -z "$ENGINE" ]]; then
  echo "no codesearch engine binary: set CODESEARCH_BIN or put it on PATH" >&2
  exit 2
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/go" "$TMP/ts/src" "$TMP/run"
cat > "$TMP/go/main.go" <<'EOF'
package main

func main() {
	_ = Greet("Ahmad")
}
EOF
cat > "$TMP/go/greet.go" <<'EOF'
package main

func Greet(name string) string {
	return "hi " + name + "!"
}
EOF
cat > "$TMP/ts/src/util.ts" <<'EOF'
export function formatName(first: string, last: string): string {
	return first + " " + last;
}
EOF
cat > "$TMP/ts/src/main.ts" <<'EOF'
import { formatName } from "./util";

export function run(): string {
	return formatName("Ahmad", "Fauzi");
}
EOF

cp "$BIN" "$TMP/run/plw"
cd "$TMP/run"
export HOME="$TMP/empty-home"
mkdir -p "$HOME"
export XDG_STATE_HOME="$HOME/state"
export XDG_CACHE_HOME="$HOME/cache"

# Case 1: batch callers on Go (two names, one scan).
CODESEARCH_BIN="$ENGINE" ./plw ast callers "$TMP/go" "Greet,main" --json > "$TMP/callers-batch.json"
# Case 2: batch defs on Go.
CODESEARCH_BIN="$ENGINE" ./plw ast defs "$TMP/go" "Greet,main" --json > "$TMP/defs-batch.json"
# Case 3: batch related on TS.
CODESEARCH_BIN="$ENGINE" ./plw ast related "$TMP/ts" "formatName,run" --json > "$TMP/related-batch.json"
# Case 4: whole-repo topology scope on the Go fixture.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/go" --json > "$TMP/topo-batch.json"
# Case 5: single-name callers still behaves as before (no batch keys).
CODESEARCH_BIN="$ENGINE" ./plw ast callers "$TMP/go" "Greet" --json > "$TMP/callers-single.json"

python3 - "$TMP/callers-batch.json" "$TMP/defs-batch.json" "$TMP/related-batch.json" "$TMP/topo-batch.json" "$TMP/callers-single.json" <<'PY'
import json
import sys
from pathlib import Path

def load(p):
    return json.loads(Path(p).read_text())

callers, defs, related, topo, single = (load(p) for p in sys.argv[1:])

# Case 1: one scan, both names, grouped rows.
assert callers["ok"] is True, callers
assert callers.get("batch_mode") == "batch", callers
assert set(callers["grouped"]) == {"Greet", "main"}, callers
assert len(callers["grouped"]["Greet"]) == 1, callers
assert callers["grouped"]["Greet"][0]["path"] == "main.go", callers
assert callers["grouped"]["main"] == [], callers
assert callers["engine_result"].get("batch") is True, callers

# Case 2: batch defs group hits per query.
assert defs["ok"] is True, defs
assert defs.get("batch_mode") == "batch", defs
assert set(defs["grouped"]) == {"Greet", "main"}, defs
assert any(h["path"] == "greet.go" for h in defs["grouped"]["Greet"]), defs
assert any(h["path"] == "main.go" for h in defs["grouped"]["main"]), defs

# Case 3: batch related groups rows per symbol.
assert related["ok"] is True, related
assert related.get("batch_mode") == "batch", related
assert set(related["grouped"]) == {"formatName", "run"}, related
assert any(r["path"] == "src/util.ts" for r in related["grouped"]["formatName"]), related
assert all(r.get("symbol") == "formatName" for r in related["grouped"]["formatName"]), related

# Case 4: whole-repo scope, no cap, both symbols covered.
o = topo["target_orientation"]
e = o["engine"]
assert e["available"] is True, e
assert e["coverage"]["scope"] == "whole-repo", e["coverage"]
assert e["coverage"]["batch_mode"] == "batch", e["coverage"]
assert len(e["coverage"]["symbols_queried"]) == e["coverage"]["symbols_total"], e["coverage"]
assert {"Greet", "main"} <= set(e["coverage"]["symbols_queried"]), e["coverage"]

# Case 5: single-name callers has no batch keys and the same row.
assert single["ok"] is True, single
assert "batch_mode" not in single and "grouped" not in single, single
assert single["summary"]["total"] == 1, single

print("SMOKE_BATCH_ENGINE: PASS")
PY
