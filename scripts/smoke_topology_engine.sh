#!/usr/bin/env bash
# Standalone smoke test for the engine symbol layer (overlay
# topology_engine_v1): `plw topology` gains AST-verified definition nodes
# plus name-based lead call/relation edges, kept apart from the import
# graph — with target-grounded scope and an explicit unavailable block
# when the engine is missing. `plw impact` gains a separate
# engine_lead_impact section.
#
# Usage: bash scripts/smoke_topology_engine.sh [plw-binary]
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

mkdir -p "$TMP/go" "$TMP/ts/src" "$TMP/dup/pkg/a" "$TMP/dup/pkg/b" "$TMP/run"
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
cat > "$TMP/dup/pkg/a/a.go" <<'EOF'
package a

func Run() string { return "a" }
EOF
cat > "$TMP/dup/pkg/b/b.go" <<'EOF'
package b

func Run() string { return "b" }
EOF
cat > "$TMP/dup/main.go" <<'EOF'
package main

import ("x/a"; "x/b")

func main() { _ = a.Run(); _ = b.Run() }
EOF

cp "$BIN" "$TMP/run/plw"
cd "$TMP/run"
export HOME="$TMP/empty-home"
mkdir -p "$HOME"
export XDG_STATE_HOME="$HOME/state"
export XDG_CACHE_HOME="$HOME/cache"

# Case 1: pure Go, engine -> layer with Greet def + calls edge.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/go" --json > "$TMP/go-engine.json"
# Case 2: pure Go, no engine -> unavailable block + issue, exit 0.
env -u CODESEARCH_BIN PATH=/usr/bin:/bin ./plw topology "$TMP/go" --json > "$TMP/go-noengine.json"
# Case 3: TS-only, engine -> import edges unchanged, layer with formatName.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/ts" --json > "$TMP/ts-engine.json"
# Case 4: TS-only, no engine -> inventory identical to case 3.
env -u CODESEARCH_BIN PATH=/usr/bin:/bin ./plw topology "$TMP/ts" --json > "$TMP/ts-noengine.json"
# Case 5: same-name symbols, engine -> two def nodes, candidates both.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/dup" --json > "$TMP/dup-engine.json"
# Case 6: cache hit -> layer present on the repeat call.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/go" --json > "$TMP/go-engine-again.json"
# Case 7: impact on a Go file -> separate lead section, import part unresolved.
CODESEARCH_BIN="$ENGINE" ./plw impact greet.go "$TMP/go" --json > "$TMP/go-impact.json" || test $? -eq 2

python3 - "$TMP/go-engine.json" "$TMP/go-noengine.json" "$TMP/ts-engine.json" "$TMP/ts-noengine.json" "$TMP/dup-engine.json" "$TMP/go-engine-again.json" "$TMP/go-impact.json" <<'PY'
import json
import sys
from pathlib import Path

def load(p):
    return json.loads(Path(p).read_text())

go_yes, go_no, ts_yes, ts_no, dup_yes, go_again, go_imp = (load(p) for p in sys.argv[1:])

for doc in (go_yes, go_no, ts_yes, ts_no, dup_yes, go_again):
    # Additive change: the old topology contract is untouched.
    assert doc["schema_version"] == "4.3.0-target-topology", doc
    assert "target_orientation" in doc and "graph_cache" in doc, doc
    assert "engine" in doc["target_orientation"], doc

# Case 1: Go + engine.
o = go_yes["target_orientation"]
assert o["inventory"]["files"] == 0, o["inventory"]
assert o["inventory"]["internal_import_edges"] == 0, o["inventory"]
assert o.get("engine_layer_schema") == "plw-topology-engine-v1", o
e = o["engine"]
assert e["available"] is True, e
assert e["error"] is None, e
assert any(n["symbol"] == "Greet" and n["file"] == "greet.go" and n["line"] == 3
           and n["authority"] == "definition" for n in e["symbol_nodes"]), e
assert any(x["kind"] == "calls" and x["caller"]["file"] == "main.go"
           and x["authority"] == "lead-name-based"
           and any(c["file"] == "greet.go" for c in x["callee"]["candidates"])
           for x in e["symbol_edges"]), e
assert {"source": "main.go", "target": "greet.go", "via": "calls",
        "authority": "lead-name-based"} in e["engine_file_edges"], e
assert e["coverage"]["languages"] == ["go"], e["coverage"]
assert (o.get("reasoning_gaps") or []) == [], o
assert "symbol call graph (name-based leads)" in o["technical_model"]["not_equivalent_to"], o

# Case 2: Go, no engine.
o = go_no["target_orientation"]
e = o["engine"]
assert e["available"] is False, e
assert o["inventory"] == go_yes["target_orientation"]["inventory"], o
assert any(i["kind"] == "engine_topology_unavailable" and i["count"] == 2
           for i in (o.get("reasoning_gaps") or [])), o

# Cases 3+4: TS import edges unchanged by the layer.
assert ts_yes["target_orientation"]["inventory"]["internal_import_edges"] == 1
assert ts_yes["target_orientation"]["inventory"] == ts_no["target_orientation"]["inventory"]
e = ts_yes["target_orientation"]["engine"]
assert e["available"] is True, e
assert any(n["symbol"] == "formatName" for n in e["symbol_nodes"]), e
assert ts_no["target_orientation"]["engine"]["available"] is False

# Case 5: same-name definitions stay separate.
e = dup_yes["target_orientation"]["engine"]
run_nodes = [n for n in e["symbol_nodes"] if n["symbol"] == "Run"]
assert len(run_nodes) == 2, run_nodes
run_edges = [x for x in e["symbol_edges"]
             if x.get("kind") == "calls" and (x.get("callee") or {}).get("symbol") == "Run"]
assert run_edges and all(len(x["callee"]["candidates"]) == 2 for x in run_edges), run_edges

# Case 6: cache hit keeps the layer.
assert (go_again.get("graph_cache") or {}).get("status") == "hit", go_again
assert go_again["target_orientation"]["engine"]["available"] is True
assert go_again["target_orientation"]["engine"]["symbol_nodes"] == \
    go_yes["target_orientation"]["engine"]["symbol_nodes"]

# Case 7: impact keeps the import verdict and adds the lead section.
lead = go_imp.get("engine_lead_impact") or {}
assert go_imp.get("error") == "file_not_found_in_graph", go_imp
assert lead.get("authority") == "lead-name-based", lead
assert "main.go" in (lead.get("downstream") or []), lead

print("SMOKE_TOPOLOGY_ENGINE: PASS")
PY
