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

# Case 1: pure Go, engine -> v2 compact layer with Greet def + calls edge.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/go" --json > "$TMP/go-engine.json"
# Case 2: pure Go, no engine -> unavailable block + issue, exit 0.
env -u CODESEARCH_BIN PATH=/usr/bin:/bin ./plw topology "$TMP/go" --json > "$TMP/go-noengine.json"
# Case 3: TS-only, engine -> import edges unchanged, layer with formatName.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/ts" --json > "$TMP/ts-engine.json"
# Case 4: TS-only, no engine -> inventory identical to case 3.
env -u CODESEARCH_BIN PATH=/usr/bin:/bin ./plw topology "$TMP/ts" --json > "$TMP/ts-noengine.json"
# Case 5: same-name symbols, engine -> two def nodes, candidates both.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/dup" --json > "$TMP/dup-engine.json"
# Case 6: cache hit -> layer present on the repeat call, engine cache hit.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/go" --json > "$TMP/go-engine-again.json"
# Case 7: impact on a Go file -> separate lead section, import part unresolved.
CODESEARCH_BIN="$ENGINE" ./plw impact greet.go "$TMP/go" --json > "$TMP/go-impact.json" || test $? -eq 2
# Case 8: explicit --full-engine-edges preserves legacy plw-topology-engine-v1 shape.
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/dup" --full-engine-edges --json > "$TMP/dup-v1.json"

# Case 9: Guardrail #2 — 1-hop neighborhood on resolved single target keeps direct callers + excludes unrelated files.
mkdir -p "$TMP/hop/src"
cat > "$TMP/hop/src/entry.ts" <<'EOF'
import { midHelper } from "./mid";
export function bootHop(): string {
	return midHelper();
}
EOF
cat > "$TMP/hop/src/mid.ts" <<'EOF'
import { leafCore } from "./leaf";
export function midHelper(): string {
	return leafCore();
}
EOF
cat > "$TMP/hop/src/leaf.ts" <<'EOF'
export function leafCore(): string {
	return "leaf";
}
EOF
cat > "$TMP/hop/src/unrelated.ts" <<'EOF'
export function unrelatedThing(): string {
	return "nope";
}
EOF
CODESEARCH_BIN="$ENGINE" ./plw agent orient "$TMP/hop" --task "audit mid.ts" --json > "$TMP/hop-orient.json"
CODESEARCH_BIN="$ENGINE" ./plw work "audit mid.ts" "$TMP/hop" --full-engine --full-engine-edges --json > "$TMP/hop-work-full.json"

# Case 10: Guardrail #3 (un-defined stopword dropped pre-query vs internally-defined Box retained when callers <= 80).
mkdir -p "$TMP/stopword_keep/src"
cat > "$TMP/stopword_keep/src/box.ts" <<'EOF'
export class Box {
	value: number = 1;
}
export function externalMissing(): void {
	// Text is in COMMON_NOISE_SYMBOLS and NOT defined anywhere here.
}
EOF
cat > "$TMP/stopword_keep/src/user.ts" <<'EOF'
import { Box, Text } from "./external";
export function renderBox(): Box {
	void Text;
	return new Box();
}
EOF
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/stopword_keep" --json > "$TMP/stopword-keep.json"

# Case 11: Guardrail #3 (>80 callers suppresses noisy stopword edges while keeping def node).
mkdir -p "$TMP/stopword_noisy/src"
cat > "$TMP/stopword_noisy/src/box.ts" <<'EOF'
export function Box(n: number): number {
	return n + 1;
}
EOF
python3 - "$TMP/stopword_noisy/src/caller.ts" <<'PY'
import sys
from pathlib import Path
lines = ['import { Box } from "./box";', 'export function fanoutBox(): number {', '  let total = 0;']
for idx in range(85):
    lines.append(f'  total += Box({idx});')
lines.extend(['  return total;', '}'])
Path(sys.argv[1]).write_text("\n".join(lines) + "\n", encoding="utf-8")
PY
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/stopword_noisy" --json > "$TMP/stopword-noisy.json"

# Case 12: generated wasm-bindgen & public/assets files excluded and reported.
mkdir -p "$TMP/gen/src" "$TMP/gen/public/assets"
cat > "$TMP/gen/src/app.ts" <<'EOF'
export function appMain(): number { return 1; }
EOF
cat > "$TMP/gen/src/gen.js" <<'EOF'
/* @ts-self-types="./gen.d.ts" */
export function wasmGenFn() { return 42; }
EOF
cat > "$TMP/gen/src/gen.d.ts" <<'EOF'
export function wasmGenFn(): number;
EOF
cat > "$TMP/gen/public/assets/harness.js" <<'EOF'
export function harnessHelper() { return 99; }
EOF
CODESEARCH_BIN="$ENGINE" ./plw topology "$TMP/gen" --json > "$TMP/gen-topo.json"
CODESEARCH_BIN="$ENGINE" ./plw agent orient "$TMP/gen" --task "inspect gen.js" --target "src/gen.js" --json > "$TMP/gen-orient.json"

python3 - "$TMP/go-engine.json" "$TMP/go-noengine.json" "$TMP/ts-engine.json" "$TMP/ts-noengine.json" "$TMP/dup-engine.json" "$TMP/go-engine-again.json" "$TMP/go-impact.json" "$TMP/dup-v1.json" "$TMP/hop-orient.json" "$TMP/hop-work-full.json" "$TMP/stopword-keep.json" "$TMP/stopword-noisy.json" "$TMP/gen-topo.json" "$TMP/gen-orient.json" <<'PY'
import json
import sys
from pathlib import Path

def load(p):
    return json.loads(Path(p).read_text())

(
    go_yes, go_no, ts_yes, ts_no, dup_yes, go_again, go_imp, dup_v1,
    hop_orient, hop_work_full, stop_keep, stop_noisy, gen_topo, gen_orient,
) = (load(p) for p in sys.argv[1:])

for doc in (go_yes, go_no, ts_yes, ts_no, dup_yes, go_again, dup_v1):
    # Additive change: the old topology contract is untouched.
    assert doc["schema_version"] == "4.3.0-target-topology", doc
    assert "target_orientation" in doc and "graph_cache" in doc, doc
    assert "engine" in doc["target_orientation"], doc

# Case 1: Go + engine (v2 compact default).
o = go_yes["target_orientation"]
assert o["inventory"]["files"] == 0, o["inventory"]
assert o["inventory"]["internal_import_edges"] == 0, o["inventory"]
assert o.get("engine_layer_schema") == "plw-topology-engine-v2", o
e = o["engine"]
assert e["available"] is True, e
assert e["error"] is None, e
greet_nodes = [n for n in e["symbol_nodes"]
               if n["symbol"] == "Greet" and n["file"] == "greet.go" and n["line"] == 3
               and n["authority"] == "definition"]
assert len(greet_nodes) == 1 and isinstance(greet_nodes[0].get("id"), int), e
greet_id = greet_nodes[0]["id"]
assert "provenance" not in greet_nodes[0], greet_nodes[0]
assert any(x["kind"] == "calls" and x["caller"]["file"] == "main.go"
           and x["authority"] == "lead-name-based"
           and "provenance" not in x
           and x["callee"]["candidate_count"] == 1
           and greet_id in x["callee"]["candidate_ids"]
           for x in e["symbol_edges"]), e
assert {"source": "main.go", "target": "greet.go", "via": "calls",
        "authority": "lead-name-based"} in e["engine_file_edges"], e
assert not any(fe["source"] == fe["target"] for fe in e["engine_file_edges"]), e
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

# Case 5: same-name definitions stay separate and are referenced by id in v2.
e = dup_yes["target_orientation"]["engine"]
run_nodes = [n for n in e["symbol_nodes"] if n["symbol"] == "Run"]
assert len(run_nodes) == 2, run_nodes
run_ids = {n["id"] for n in run_nodes}
run_edges = [x for x in e["symbol_edges"]
             if x.get("kind") == "calls" and (x.get("callee") or {}).get("symbol") == "Run"]
assert run_edges and all(
    x["callee"]["candidate_count"] == 2 and set(x["callee"]["candidate_ids"]) == run_ids
    for x in run_edges
), run_edges

# Case 6: cache hit keeps the layer and reports engine cache hit.
assert (go_again.get("graph_cache") or {}).get("status") == "hit", go_again
assert go_again["target_orientation"]["engine"]["available"] is True
assert go_again["target_orientation"]["engine"]["coverage"]["cache_status"] == "hit", go_again
assert go_again["target_orientation"]["engine"]["symbol_nodes"] == \
    go_yes["target_orientation"]["engine"]["symbol_nodes"]

# Case 7: impact keeps the import verdict and adds the lead section.
lead = go_imp.get("engine_lead_impact") or {}
assert go_imp.get("error") == "file_not_found_in_graph", go_imp
assert lead.get("authority") == "lead-name-based", lead
assert "main.go" in (lead.get("downstream") or []), lead

# Case 8: --full-engine-edges preserves legacy plw-topology-engine-v1 shape.
o_v1 = dup_v1["target_orientation"]
assert o_v1.get("engine_layer_schema") == "plw-topology-engine-v1", o_v1
e_v1 = o_v1["engine"]
run_edges_v1 = [x for x in e_v1["symbol_edges"]
                if x.get("kind") == "calls" and (x.get("callee") or {}).get("symbol") == "Run"]
assert run_edges_v1 and all(
    len(x["callee"]["candidates"]) == 2 and "provenance" in x
    for x in run_edges_v1
), run_edges_v1

# Case 9: 1-hop neighborhood + plw work --full-engine --full-engine-edges.
hop_e = hop_orient["target_orientation"]["engine"]
hop_files = {n["file"] for n in hop_e["symbol_nodes"]}
assert hop_e["coverage"]["target_files"] == 3, hop_e["coverage"]
assert {"src/entry.ts", "src/mid.ts", "src/leaf.ts"} <= hop_files, hop_files
assert "src/unrelated.ts" not in hop_files, hop_files
assert any(
    x["kind"] == "calls" and x["caller"]["file"] == "src/entry.ts"
    and x["callee"]["symbol"] == "midHelper"
    for x in hop_e["symbol_edges"]
), hop_e["symbol_edges"]
full_e = hop_work_full["target_orientation"]["engine"]
assert full_e["schema_version"] == "plw-topology-engine-v1", full_e["schema_version"]
assert "src/unrelated.ts" in {n["file"] for n in full_e["symbol_nodes"]}, full_e

# Case 10: Guardrail #3 — internally defined Box kept; external-only Text dropped.
sk_e = stop_keep["target_orientation"]["engine"]
assert "Text" not in sk_e["coverage"]["symbols_queried"], sk_e["coverage"]
assert sk_e["coverage"]["symbols_filtered_common"] >= 1, sk_e["coverage"]
assert any(n["symbol"] == "Box" for n in sk_e["symbol_nodes"]), sk_e["symbol_nodes"]

# Case 11: Guardrail #3 — >80 callers triggers high_caller_suppressed while keeping definition node.
sn_e = stop_noisy["target_orientation"]["engine"]
assert "Box" in sn_e["coverage"]["high_caller_suppressed"], sn_e["coverage"]
assert any(n["symbol"] == "Box" for n in sn_e["symbol_nodes"]), sn_e["symbol_nodes"]
assert not any(
    x["kind"] == "calls" and (x.get("callee") or {}).get("symbol") == "Box"
    for x in sn_e["symbol_edges"]
), sn_e["symbol_edges"]

# Case 12: Generated files excluded & counted in both whole-repo and targeted runs.
gt_e = gen_topo["target_orientation"]["engine"]
assert gt_e["coverage"]["generated_files_excluded"] >= 2, gt_e["coverage"]
assert all(
    n["file"] not in {"src/gen.js", "src/gen.d.ts", "public/assets/harness.js"}
    for n in gt_e["symbol_nodes"]
), gt_e["symbol_nodes"]
go_gen_e = gen_orient["target_orientation"]["engine"]
assert go_gen_e["coverage"]["generated_files_excluded"] >= 1, go_gen_e["coverage"]
assert go_gen_e["symbol_nodes"] == [], go_gen_e["symbol_nodes"]

print("SMOKE_TOPOLOGY_ENGINE: PASS")
PY
