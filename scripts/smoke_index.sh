#!/usr/bin/env bash
# Standalone smoke test for engine index state (overlay engine_state_v1,
# G9/G13): the gob index lives under PLW's cache root (not
# $HOME/.cache/codesearch-go), `plw ast index` reports/builds/deletes
# it, `plw ast doctor` shows where it lives, and the engine contract
# check agrees on the schema.
#
# Usage: bash scripts/smoke_index.sh [plw-binary]
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

mkdir -p "$TMP/go" "$TMP/run"
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

cp "$BIN" "$TMP/run/plw"
cd "$TMP/run"
export HOME="$TMP/empty-home"
mkdir -p "$HOME"
export XDG_STATE_HOME="$HOME/state"
export XDG_CACHE_HOME="$HOME/cache"

# Case 1: status on a fresh root — no index yet, under PLW's cache root.
CODESEARCH_BIN="$ENGINE" ./plw ast index "$TMP/go" status --json > "$TMP/status-fresh.json"
# Case 2: warm builds the index without a query.
CODESEARCH_BIN="$ENGINE" ./plw ast index "$TMP/go" warm --json > "$TMP/warm.json"
# Case 3: edit one file — the probe counts exactly one changed.
printf 'package main\n\nfunc Greet(name string) string {\n\treturn "hi " + name + "!"\n}\n\nfunc Bye() {}\n' > "$TMP/go/greet.go"
CODESEARCH_BIN="$ENGINE" ./plw ast index "$TMP/go" status --json > "$TMP/status-stale.json"
# Case 4: clean deletes the index file.
CODESEARCH_BIN="$ENGINE" ./plw ast index "$TMP/go" clean --json > "$TMP/clean.json"
# Case 5: doctor reports the cache dir and the (now absent) index.
CODESEARCH_BIN="$ENGINE" ./plw ast doctor "$TMP/go" --json > "$TMP/doctor.json"
# Case 6: the contract check agrees on the schema.
bash "$ROOT/scripts/check_engine_contract.sh" > "$TMP/contract.txt"

python3 - "$TMP/status-fresh.json" "$TMP/warm.json" "$TMP/status-stale.json" "$TMP/clean.json" "$TMP/doctor.json" "$TMP/contract.txt" <<'PY'
import json
import os
import sys
from pathlib import Path

def load(p):
    return json.loads(Path(p).read_text())

fresh, warm, stale, clean, doctor = (load(p) for p in sys.argv[1:6])
contract = Path(sys.argv[6]).read_text()

# Case 1: fresh root — no index, cache dir under the temp HOME (PLW root).
assert fresh["ok"] is True, fresh
assert fresh["engine_result"]["exists"] is False, fresh
cache_dir = fresh["engine_result"]["cache_dir"]
assert cache_dir == os.path.join(
    os.environ["XDG_CACHE_HOME"], "plw", "codebase", "codesearch"), fresh
assert fresh["engine_result"]["stale"]["added"] == 2, fresh

# Case 2: warm builds it; status afterwards is fresh.
assert warm["ok"] is True, warm
assert warm["engine_result"]["exists"] is True, warm
assert warm["engine_result"]["schema"] == "go9", warm
assert warm["engine_result"]["files"] == 2, warm
assert warm["engine_result"]["chunks"] > 0, warm
assert warm["engine_result"]["stale"] == {"changed": 0, "added": 0, "removed": 0}, warm

# Case 3: one edited file counts as exactly one changed.
assert stale["ok"] is True, stale
assert stale["engine_result"]["stale"] == {"changed": 1, "added": 0, "removed": 0}, stale

# Case 4: clean removes the file; the index is gone.
assert clean["ok"] is True, clean
assert clean["engine_result"]["removed"] is True, clean
assert clean["engine_result"]["exists"] is False, clean
assert not Path(clean["engine_result"]["index_path"]).exists(), clean

# Case 5: doctor carries the cache dir and the index summary.
assert doctor["ok"] is True, doctor
eng = doctor["engine"]
assert eng["cache_dir"] == cache_dir, doctor
assert eng["index"]["exists"] is False, doctor
assert eng["index"]["stale"]["added"] == 2, doctor

# Case 6: the contract check passed.
assert "CHECK_ENGINE_CONTRACT: PASS" in contract, contract

print("SMOKE_INDEX: PASS")
PY
