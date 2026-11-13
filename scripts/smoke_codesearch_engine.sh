#!/usr/bin/env bash
# Standalone smoke test for the codesearch-go tree-sitter engine port
# (overlay codesearch_engine_v1): `plw ast` + `plw doctor` engine status.
# The engine binary itself is supplied by the harness (CODESEARCH_BIN or
# PATH); this script never bundles it.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${1:-$ROOT/artifacts/plw-linux-x86_64}"

if [[ ! -x "$BIN" ]]; then
  echo "binary not executable: $BIN" >&2
  exit 2
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/fixture/src" "$TMP/run"
cat > "$TMP/fixture/main.go" <<'EOF'
package main

func main() {
	_ = Greet("Ahmad")
	_ = Add(1, 2)
}
EOF
cat > "$TMP/fixture/greet.go" <<'EOF'
package main

func Greet(name string) string {
	return "hi " + name + "!"
}
EOF
cat > "$TMP/fixture/calc.go" <<'EOF'
package main

func Add(a, b int) int {
	return a + b
}
EOF
cat > "$TMP/fixture/src/util.ts" <<'EOF'
export function formatName(first: string, last: string): string {
	return first + " " + last;
}
EOF
cat > "$TMP/fixture/src/main.ts" <<'EOF'
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

./plw ast doctor "$TMP/fixture" --json > "$TMP/doctor.json"
./plw ast defs "$TMP/fixture" Greet --json > "$TMP/defs.json"
./plw ast callers "$TMP/fixture" formatName --json > "$TMP/callers.json"
./plw ast related "$TMP/fixture" formatName --json > "$TMP/related.json"
./plw doctor "$TMP/fixture" --json > "$TMP/plw-doctor.json"
if ./plw ast defs "$TMP/fixture" Greet --binary /nonexistent/codesearch --json > "$TMP/negative.json"; then
  echo "missing-binary probe must fail closed, but it succeeded" >&2
  exit 1
fi

python3 - "$TMP/doctor.json" "$TMP/defs.json" "$TMP/callers.json" "$TMP/related.json" "$TMP/plw-doctor.json" "$TMP/negative.json" <<'PY'
import json
import sys
from pathlib import Path

def load(p):
    return json.loads(Path(p).read_text())

doctor, defs, callers, related, plw_doctor, negative = (load(p) for p in sys.argv[1:])

assert doctor["ok"] is True, doctor
assert doctor["engine"]["available"] is True, doctor
assert doctor["engine"]["schema"] == "go10", doctor
assert doctor["engine"]["tree_sitter_ready"] is True, doctor

hits = defs["engine_result"]["hits"]
assert hits and hits[0]["path"] == "greet.go" and hits[0]["kind"] == "sig", defs

calls = callers["engine_result"]["calls"]
assert len(calls) == 1, callers
assert calls[0]["path"] == "src/main.ts" and calls[0]["line"] == 4, callers
assert calls[0]["caller"] == "run", callers

rows = related["engine_result"]["related"]
assert len(rows) == 1 and rows[0]["path"] == "src/util.ts", related

engine = plw_doctor["runtime"]["codesearch_engine"]
assert engine["available"] is True and engine["schema"] == "go10", plw_doctor

assert negative["ok"] is False and negative["engine_result"] is None, negative
assert negative["error"], negative

print("SMOKE_CODESEARCH_ENGINE: PASS")
PY
