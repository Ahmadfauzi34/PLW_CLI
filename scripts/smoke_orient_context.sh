#!/usr/bin/env bash
# Standalone smoke test for the dynamic context matrix (overlay
# codesearch_engine_v2): `plw agent orient` executes KONTEKS_DAN_RILIS.md
# section 2 — context A/B from target languages + engine availability,
# with routes (never from the task text: capability selection is
# explicitly not keyword-based).
#
# Usage: bash scripts/smoke_orient_context.sh [plw-binary]
# The engine binary comes from CODESEARCH_BIN or PATH; the no-engine
# cases run the binary with a PATH that cannot discover it.
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

mkdir -p "$TMP/go" "$TMP/ts/src" "$TMP/mixed/src" "$TMP/kt/src" "$TMP/run"
cat > "$TMP/kt/src/Main.kt" <<'EOF'
fun main() {
    println(greet("Ahmad"))
}
EOF
cat > "$TMP/kt/src/Greeter.kt" <<'EOF'
fun greet(name: String): String {
    return "hi " + name + "!"
}
EOF
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
cp "$TMP/go/main.go" "$TMP/go/greet.go" "$TMP/mixed/"
cp "$TMP/ts/src/util.ts" "$TMP/ts/src/main.ts" "$TMP/mixed/src/"

cp "$BIN" "$TMP/run/plw"
cd "$TMP/run"
export HOME="$TMP/empty-home"
mkdir -p "$HOME"
export XDG_STATE_HOME="$HOME/state"
export XDG_CACHE_HOME="$HOME/cache"

# Case 1: pure Go, no engine -> context A, structural unavailable.
env -u CODESEARCH_BIN PATH=/usr/bin:/bin ./plw agent orient "$TMP/go" \
  --task "di mana Greet didefinisikan" --json > "$TMP/go-noengine.json"
# Case 2: pure Go, engine -> context B, structural via plw ast.
CODESEARCH_BIN="$ENGINE" ./plw agent orient "$TMP/go" \
  --task "di mana Greet didefinisikan" --json > "$TMP/go-engine.json"
# Case 3: TS-only, no engine -> context A, no engine needed.
env -u CODESEARCH_BIN PATH=/usr/bin:/bin ./plw agent orient "$TMP/ts" \
  --task "topology repo ini" --json > "$TMP/ts-noengine.json"
# Case 4: mixed Go+TS, engine -> context B, topology still available.
CODESEARCH_BIN="$ENGINE" ./plw agent orient "$TMP/mixed" \
  --task "di mana Greet didefinisikan" --json > "$TMP/mixed-engine.json"
# Case 5: Kotlin-only, no engine -> context A, structural unavailable.
env -u CODESEARCH_BIN PATH=/usr/bin:/bin ./plw agent orient "$TMP/kt" \
  --task "di mana greet didefinisikan" --json > "$TMP/kt-noengine.json"
# Case 6: Kotlin-only, engine -> context B, structural via plw ast.
CODESEARCH_BIN="$ENGINE" ./plw agent orient "$TMP/kt" \
  --task "di mana greet didefinisikan" --json > "$TMP/kt-engine.json"

python3 - "$TMP/go-noengine.json" "$TMP/go-engine.json" "$TMP/ts-noengine.json" "$TMP/mixed-engine.json" "$TMP/kt-noengine.json" "$TMP/kt-engine.json" <<'PY'
import json
import sys
from pathlib import Path

def load(p):
    return json.loads(Path(p).read_text())

go_no, go_yes, ts_no, mixed_yes, kt_no, kt_yes = (load(p) for p in sys.argv[1:])

for doc in (go_no, go_yes, ts_no, mixed_yes, kt_no, kt_yes):
    # Additive change: the old orient contract is untouched.
    assert doc["portable_agent_surface"]["entrypoint"] == "plw agent orient", doc
    assert "target_support" in doc and "portable_agent_state" in doc, doc
    assert "dynamic_context" in doc, doc

ctx = go_no["dynamic_context"]
assert ctx["context"] == "A", ctx
assert ctx["languages"] == {"go": 2}, ctx
assert ctx["needs_engine"] is True, ctx
assert ctx["engine"]["available"] is False, ctx
assert ctx["structural_query"]["available"] is False, ctx
assert ctx["structural_query"]["route"] is None, ctx
assert "codesearch-go" in ctx["reason"], ctx
assert ctx["topology"]["available"] is False, ctx
assert go_no["portable_agent_state"] == "TARGET_TOPOLOGY_LANGUAGE_UNSUPPORTED", go_no

ctx = go_yes["dynamic_context"]
assert ctx["context"] == "B", ctx
assert ctx["needs_engine"] is True, ctx
assert ctx["engine"]["available"] is True, ctx
assert ctx["engine"]["schema"] == "go11", ctx
assert ctx["structural_query"]["available"] is True, ctx
assert "plw ast" in ctx["structural_query"]["route"], ctx
assert "Context B" in ctx["reason"], ctx

ctx = ts_no["dynamic_context"]
assert ctx["context"] == "A", ctx
assert ctx["languages"] == {"typescript": 2}, ctx
assert ctx["needs_engine"] is False, ctx
assert ctx["structural_query"]["available"] is True, ctx
assert "topology" in ctx["structural_query"]["route"], ctx
assert "no engine needed" in ctx["reason"], ctx
assert ctx["topology"]["available"] is True, ctx

ctx = mixed_yes["dynamic_context"]
assert ctx["context"] == "B", ctx
assert ctx["languages"] == {"go": 2, "typescript": 2}, ctx
assert ctx["needs_engine"] is True, ctx
assert ctx["structural_query"]["available"] is True, ctx
assert ctx["topology"]["available"] is True, ctx

# Track 1 (K3): Kotlin is a matrix language — counted, engine-gated,
# but topology-covered (unlike Go, whose topology is unavailable).
ctx = kt_no["dynamic_context"]
assert ctx["context"] == "A", ctx
assert ctx["languages"] == {"kotlin": 2}, ctx
assert ctx["needs_engine"] is True, ctx
assert ctx["engine"]["available"] is False, ctx
assert ctx["structural_query"]["available"] is False, ctx
assert ctx["structural_query"]["route"] is None, ctx
assert "codesearch-go" in ctx["reason"], ctx
assert ctx["topology"]["available"] is True, ctx

ctx = kt_yes["dynamic_context"]
assert ctx["context"] == "B", ctx
assert ctx["languages"] == {"kotlin": 2}, ctx
assert ctx["needs_engine"] is True, ctx
assert ctx["engine"]["available"] is True, ctx
assert ctx["engine"]["schema"] == "go11", ctx
assert ctx["structural_query"]["available"] is True, ctx
assert "plw ast" in ctx["structural_query"]["route"], ctx
assert "Context B" in ctx["reason"], ctx
assert ctx["topology"]["available"] is True, ctx

print("SMOKE_ORIENT_CONTEXT: PASS")
PY
