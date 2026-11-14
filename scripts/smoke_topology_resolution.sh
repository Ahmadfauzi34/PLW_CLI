#!/usr/bin/env bash
# Smoke: Phase 3 topology_resolution_v1 (Kotlin same-package links + Cargo/package/shebang/config entrypoints).
#
# Verifies all mandatory invariants and guardrails:
#   1. Kotlin same-package resolution links public/internal top-level PascalCase
#      types, uppercase vals, and direct non-stdlib function calls (`fun`), while
#      never linking `private` symbols, dot-syntax extension calls, `Foo::bar` method
#      references, comments/strings, or ambiguous symbols duplicated across variant
#      roots (`src/debug` vs `src/release`).
#   2. `same_package` edges count for `topo.orphan`, `topo.test_reachability`, and
#      `impact`, while remaining excluded from `topo.circular`, `cyclic_sccs`,
#      `query_impact.circular_references`, `hott.manifold` directed cycles, and
#      `hott.sheaf` cross-boundary violations.
#   3. Cargo `[lib]`, explicit `[[bin]]`, implicit `autobins` `src/main.rs` +
#      `src/bin/*.rs`, `build.rs`, and `examples/*.rs` resolve accurately.
#   4. `package.json` `main`/`bin`/`exports`/`scripts` (including paths with spaces),
#      CLI shebangs, and curated tool configs are marked entrypoints while Angular
#      `src/app/app.config.ts` / `app.config.server.ts` stay non-entrypoints.
#   5. `scan_topology` (file_scanner.py) agrees with `build_shared_graph` on all entrypoints.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURE="$(mktemp -d)"
trap 'rm -rf "$FIXTURE"' EXIT

mkdir -p \
  "$FIXTURE/kt/src/main/kotlin/com/example/app" \
  "$FIXTURE/kt/src/debug/kotlin/com/example/app" \
  "$FIXTURE/kt/src/release/kotlin/com/example/app" \
  "$FIXTURE/kt/src/test/kotlin/com/example/app" \
  "$FIXTURE/rust/src/bin" \
  "$FIXTURE/rust/examples" \
  "$FIXTURE/js/math compesor/bin" \
  "$FIXTURE/js/scripts" \
  "$FIXTURE/js/src/app"

cat > "$FIXTURE/kt/src/main/kotlin/com/example/app/MainScreen.kt" <<'EOF'
package com.example.app

fun runMain() {
    val state = UiModel("ok")
    val x = DEFAULT_LIMIT
    renderHelper(state, x)
    // PrivateHelper and DupVariant must NOT create edges
    val s = "DotExt"
    val y = s.onDotExt()
    val z = String::memberRef
}
EOF

cat > "$FIXTURE/kt/src/main/kotlin/com/example/app/Models.kt" <<'EOF'
package com.example.app

data class UiModel(val name: String)
const val DEFAULT_LIMIT = 42

fun renderHelper(model: UiModel, limit: Int) {
    // Mutual usage back to MainScreen (must be same_package, never circular!)
    if (limit < 0) runMain()
}
EOF

cat > "$FIXTURE/kt/src/main/kotlin/com/example/app/PrivateAndExt.kt" <<'EOF'
package com.example.app

private class PrivateHelper
private fun secretFun() {}
fun String.onDotExt(): String = this
fun String.memberRef(): String = this
EOF

cat > "$FIXTURE/kt/src/debug/kotlin/com/example/app/DupVariant.kt" <<'EOF'
package com.example.app
class DupVariant
EOF

cat > "$FIXTURE/kt/src/release/kotlin/com/example/app/DupVariant.kt" <<'EOF'
package com.example.app
class DupVariant
EOF

cat > "$FIXTURE/kt/src/test/kotlin/com/example/app/ScreenTest.kt" <<'EOF'
package com.example.app

fun testScreen() {
    val m = UiModel("test")
}
EOF

cat > "$FIXTURE/rust/Cargo.toml" <<'EOF'
[package]
name = "demo-crate"
version = "0.1.0"
edition = "2021"

[lib]
name = "demo_crate"
path = "src/lib.rs"
EOF

cat > "$FIXTURE/rust/src/lib.rs" <<'EOF'
pub fn lib_api() {}
EOF
cat > "$FIXTURE/rust/src/main.rs" <<'EOF'
fn main() {}
EOF
cat > "$FIXTURE/rust/src/bin/cli.rs" <<'EOF'
fn main() {}
EOF
cat > "$FIXTURE/rust/build.rs" <<'EOF'
fn main() {}
EOF
cat > "$FIXTURE/rust/examples/demo.rs" <<'EOF'
fn main() {}
EOF
cat > "$FIXTURE/rust/src/dead_mod.rs" <<'EOF'
pub fn unused() {}
EOF

cat > "$FIXTURE/js/package.json" <<'EOF'
{
  "name": "demo-js",
  "main": "math compesor/burn.js",
  "bin": {
    "mycli": "math compesor/bin/mycli.mjs"
  },
  "scripts": {
    "audit": "npx tsx \"math compesor/run with space.mjs\" && node scripts/run-audit.mjs"
  }
}
EOF
cat > "$FIXTURE/js/math compesor/burn.js" <<'EOF'
export const wasmReady = true;
EOF
cat > "$FIXTURE/js/math compesor/bin/mycli.mjs" <<'EOF'
#!/usr/bin/env node
console.log("bin");
EOF
cat > "$FIXTURE/js/math compesor/run with space.mjs" <<'EOF'
#!/usr/bin/env node
console.log("script with space");
EOF
cat > "$FIXTURE/js/scripts/run-audit.mjs" <<'EOF'
#!/usr/bin/env node
console.log("audit");
EOF
cat > "$FIXTURE/js/scripts/shebang-only.mjs" <<'EOF'
#!/usr/bin/env node
console.log("shebang");
EOF
cat > "$FIXTURE/js/eslint.config.js" <<'EOF'
export default [];
EOF
cat > "$FIXTURE/js/vitest.config.ts" <<'EOF'
export default {};
EOF
cat > "$FIXTURE/js/src/app/app.config.ts" <<'EOF'
export const appConfig = {};
EOF
cat > "$FIXTURE/js/src/app/app.config.server.ts" <<'EOF'
export const serverConfig = {};
EOF
cat > "$FIXTURE/js/src/app/plain.ts" <<'EOF'
export const orphanVal = 1;
EOF

python3 - "$ROOT" "$FIXTURE" <<'PY'
import os
import sys

sys.path.insert(0, sys.argv[1])
fixture = sys.argv[2]

from core.shared_graph import build_shared_graph
from core.analyzer_registry import analyze_orphan
from core.target_topology import build_target_topology_orientation
from codebase.topology_analyzers import (
    analyze_circular,
    analyze_test_reachability,
    query_impact,
)
from codebase.hott_analyzers import analyze_manifold, analyze_sheaf
from codebase.file_scanner import scan_topology

# 1. Kotlin assertions
g_kt = build_shared_graph(os.path.join(fixture, "kt"))
edges = set(map(tuple, g_kt["edges"]))
natures = g_kt["edge_natures"]
by_base = {os.path.basename(k) if "DupVariant" not in k else ("debug/DupVariant.kt" if "/debug/" in k else "release/DupVariant.kt"): k for k in g_kt["vertices"]}

main_screen = by_base["MainScreen.kt"]
models = by_base["Models.kt"]
priv_ext = by_base["PrivateAndExt.kt"]
debug_dup = by_base["debug/DupVariant.kt"]
release_dup = by_base["release/DupVariant.kt"]
test_screen = by_base["ScreenTest.kt"]

assert (main_screen, models) in edges and natures[f"{main_screen} -> {models}"] == "same_package", (edges, natures)
assert (models, main_screen) in edges and natures[f"{models} -> {main_screen}"] == "same_package", (edges, natures)
assert (main_screen, priv_ext) not in edges, "private/extension/:: must not link"
assert (main_screen, debug_dup) not in edges and (main_screen, release_dup) not in edges, "duplicate symbol must fail closed"

circ = analyze_circular(g_kt)
assert circ["summary"]["total_cycles"] == 0, f"mutual same_package triggered false cycle: {circ}"
topo_kt = build_target_topology_orientation(g_kt)
assert topo_kt["inventory"]["cyclic_sccs"] == 0, topo_kt["inventory"]
assert topo_kt["inventory"]["same_package_edges"] >= 3, topo_kt["inventory"]

mani = analyze_manifold(g_kt)
assert mani["summary"]["cycle_orientation_counts"]["directed"] == 0, mani["summary"]

sheaf = analyze_sheaf(g_kt)
assert sheaf["summary"]["by_type"]["boundary_violation"] == 0, sheaf
assert sheaf["summary"]["by_type"]["circular_boundary"] == 0, sheaf

impact = query_impact(g_kt, main_screen)
assert models in impact["upstream"] and models in impact["downstream"], impact
assert impact["circular_references"] == [], impact["circular_references"]

orphans = {os.path.basename(f["file"]) for f in analyze_orphan(g_kt)["findings"]}
assert "Models.kt" not in orphans, orphans
assert "MainScreen.kt" not in orphans, orphans
assert "PrivateAndExt.kt" in orphans, orphans

tr = analyze_test_reachability(g_kt)
assert tr["summary"]["statically_reachable_files"] >= 2, tr["summary"]

# 2. Rust Cargo targets
g_rs = build_shared_graph(os.path.join(fixture, "rust"))
rs_eps = {os.path.basename(k): v for k, v in g_rs["node_metadata"].items()}
assert rs_eps["lib.rs"]["entrypoint_kind"] == "rust_lib_crate", rs_eps["lib.rs"]
assert rs_eps["main.rs"]["entrypoint_kind"] == "binary_entry", rs_eps["main.rs"]
assert rs_eps["cli.rs"]["entrypoint_kind"] == "binary_entry", rs_eps["cli.rs"]
assert rs_eps["build.rs"]["entrypoint_kind"] == "rust_build_script", rs_eps["build.rs"]
assert rs_eps["demo.rs"]["entrypoint_kind"] == "rust_example", rs_eps["demo.rs"]
assert rs_eps["dead_mod.rs"]["is_entrypoint"] is False, rs_eps["dead_mod.rs"]

# 3. JS/TS package.json + shebang + curated tool config (not app.config.ts)
g_js = build_shared_graph(os.path.join(fixture, "js"))
js_eps = {os.path.basename(k): v for k, v in g_js["node_metadata"].items()}
assert js_eps["burn.js"]["entrypoint_kind"] == "package_main", js_eps["burn.js"]
assert js_eps["mycli.mjs"]["entrypoint_kind"] == "package_bin", js_eps["mycli.mjs"]
assert js_eps["run with space.mjs"]["entrypoint_kind"] == "package_script", js_eps["run with space.mjs"]
assert js_eps["run-audit.mjs"]["entrypoint_kind"] == "package_script", js_eps["run-audit.mjs"]
assert js_eps["shebang-only.mjs"]["entrypoint_kind"] == "cli_shebang", js_eps["shebang-only.mjs"]
assert js_eps["eslint.config.js"]["entrypoint_kind"] == "tool_config_entry", js_eps["eslint.config.js"]
assert js_eps["vitest.config.ts"]["entrypoint_kind"] == "tool_config_entry", js_eps["vitest.config.ts"]
assert js_eps["app.config.ts"]["is_entrypoint"] is False, "Angular app.config.ts must NOT match tool config!"
assert js_eps["app.config.server.ts"]["is_entrypoint"] is False, "Angular app.config.server.ts must NOT match tool config!"
assert js_eps["plain.ts"]["is_entrypoint"] is False, js_eps["plain.ts"]

# 4. Parity with file_scanner.scan_topology
for sub in ("rust", "js"):
    sg = build_shared_graph(os.path.join(fixture, sub))
    st = scan_topology(os.path.join(fixture, sub))
    sg_ep_map = {k: (v["is_entrypoint"], v["entrypoint_kind"]) for k, v in sg["node_metadata"].items()}
    st_ep_map = {n["id"]: (n["is_entrypoint"], n["entrypoint_kind"]) for n in st["nodes"]}
    assert sg_ep_map == st_ep_map, (sub, sg_ep_map, st_ep_map)

print("SMOKE_TOPOLOGY_RESOLUTION: PASS")
PY
