#!/usr/bin/env bash
# Smoke: AndroidManifest.xml entrypoints (G5a).
#
# Files declared as Android components in AndroidManifest.xml used to
# be labeled orphan (fan_in 0, no basename entrypoint rule matches a
# Kotlin class name). The fix parses the manifest and resolves each
# component class to its .kt file (name-only against the Kotlin symbol
# index, unique match required), then marks the file is_entrypoint with
# kind `android_manifest`.
#
# Fixture traps:
#   - `.Dup` is declared but exists in TWO packages -> ambiguous,
#     must NOT be marked (fail-closed)
#   - `.Missing` is declared but has no .kt file -> nothing marked
#   - `.Alias` is an activity-alias, not a class -> nothing marked
#   - `UnregisteredActivity.kt` exists but is NOT in the manifest ->
#     stays a non-entrypoint (and stays orphan)
#   - the service is declared by FULLY-QUALIFIED name
#     (`com.example.agent.Svc`) -> resolves via package check
#   - `main.ts` keeps its basename entrypoint; `plain.ts` stays plain
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURE="$(mktemp -d)"
trap 'rm -rf "$FIXTURE"' EXIT

mkdir -p "$FIXTURE/host" "$FIXTURE/agent" "$FIXTURE/a" "$FIXTURE/b"

cat > "$FIXTURE/AndroidManifest.xml" <<'EOF'
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application android:name=".App">
        <activity android:name=".MainActivity" />
        <activity android:name=".host.WakeActivity" />
        <activity android:name=".Dup" />
        <activity android:name=".Missing" />
        <activity-alias android:name=".Alias" android:targetActivity=".MainActivity" />
        <service android:name="com.example.agent.Svc" />
    </application>
</manifest>
EOF

cat > "$FIXTURE/App.kt" <<'EOF'
package com.example
class App
EOF
cat > "$FIXTURE/MainActivity.kt" <<'EOF'
package com.example
class MainActivity
EOF
cat > "$FIXTURE/host/WakeActivity.kt" <<'EOF'
package com.example.host
class WakeActivity
EOF
cat > "$FIXTURE/agent/Svc.kt" <<'EOF'
package com.example.agent
class Svc
EOF
cat > "$FIXTURE/UnregisteredActivity.kt" <<'EOF'
package com.example
class UnregisteredActivity
EOF
cat > "$FIXTURE/a/Dup.kt" <<'EOF'
package com.example.a
class Dup
EOF
cat > "$FIXTURE/b/Dup.kt" <<'EOF'
package com.example.b
class Dup
EOF
cat > "$FIXTURE/main.ts" <<'EOF'
console.log("bootstrap");
EOF
cat > "$FIXTURE/plain.ts" <<'EOF'
export const x = 1;
EOF

python3 - "$ROOT" "$FIXTURE" <<'PY'
import os
import sys

sys.path.insert(0, sys.argv[1])
fixture = sys.argv[2]

from language_semantics import android_manifest_entrypoints
from core.shared_graph import build_shared_graph
from core.analyzer_registry import analyze_orphan
from codebase.file_scanner import scan_topology

def kt_sources(root):
    out = []
    for dirpath, _dirs, files in os.walk(root):
        for f in sorted(files):
            if f.endswith(".kt"):
                p = os.path.join(dirpath, f)
                with open(p, encoding="utf-8") as fh:
                    out.append((p, fh.read()))
    return out

def base(p):
    return os.path.basename(p)

# --- 1. helper alone: exactly the 4 manifest classes resolve ---
eps = android_manifest_entrypoints(fixture, kt_sources(fixture))
got = sorted(base(p) for p in eps)
want = ["App.kt", "MainActivity.kt", "Svc.kt", "WakeActivity.kt"]
if got != want:
    raise SystemExit(f"android_manifest_entrypoints = {got}, expected {want}")

# --- 2. shared_graph: is_entrypoint flags + orphan list ---
graph = build_shared_graph(fixture)
meta = graph["node_metadata"]

def is_ep(path_suffix, kind=None):
    hits = [(k, v) for k, v in meta.items() if k.endswith(path_suffix)]
    if len(hits) != 1:
        raise SystemExit(f"expected 1 node for {path_suffix}, got {len(hits)}")
    m = hits[0][1]
    if not m.get("is_entrypoint"):
        raise SystemExit(f"{path_suffix} is not an entrypoint")
    if kind is not None and m.get("entrypoint_kind") != kind:
        raise SystemExit(
            f"{path_suffix} kind={m.get('entrypoint_kind')!r}, expected {kind!r}"
        )
    return m

for suffix in ("App.kt", "MainActivity.kt", "host/WakeActivity.kt", "agent/Svc.kt"):
    m = is_ep("/" + suffix, kind="android_manifest")
    if m.get("entrypoint_confidence") != 0.95:
        raise SystemExit(f"{suffix} confidence={m.get('entrypoint_confidence')!r}")

for suffix in ("UnregisteredActivity.kt", "a/Dup.kt", "b/Dup.kt", "plain.ts"):
    hits = [(k, v) for k, v in meta.items() if k.endswith("/" + suffix)]
    if len(hits) != 1 or hits[0][1].get("is_entrypoint"):
        raise SystemExit(f"{suffix} must stay a non-entrypoint")

m = is_ep("/main.ts", kind="browser_bootstrap")  # basename rule untouched

orphans = {base(f["file"]) for f in analyze_orphan(graph)["findings"]}
for gone in ("App.kt", "MainActivity.kt", "WakeActivity.kt", "Svc.kt"):
    if gone in orphans:
        raise SystemExit(f"{gone} is still orphan after G5a")
for stays in ("UnregisteredActivity.kt", "Dup.kt", "plain.ts"):
    if stays not in orphans:
        raise SystemExit(f"{stays} should stay orphan")

# --- 3. file_scanner: same flags, same orphan split ---
topo = scan_topology(fixture)
nodes = {base(n["id"]): n for n in topo["nodes"]}
for name in ("App.kt", "MainActivity.kt", "WakeActivity.kt", "Svc.kt"):
    n = nodes.get(name)
    if not n or not n.get("is_entrypoint") or n.get("entrypoint_kind") != "android_manifest":
        raise SystemExit(f"file_scanner: {name} is not an android_manifest entrypoint")
for name in ("UnregisteredActivity.kt", "Dup.kt", "plain.ts"):
    if nodes.get(name, {}).get("is_entrypoint"):
        raise SystemExit(f"file_scanner: {name} must stay a non-entrypoint")
unref = {base(x["id"]) for x in topo["diagnostics"]["unreferenced_files"]}
for gone in ("App.kt", "MainActivity.kt", "WakeActivity.kt", "Svc.kt"):
    if gone in unref:
        raise SystemExit(f"file_scanner: {gone} is still unreferenced after G5a")
for stays in ("UnregisteredActivity.kt", "Dup.kt", "plain.ts"):
    if stays not in unref:
        raise SystemExit(f"file_scanner: {stays} should stay unreferenced")

print("SMOKE_ANDROID_MANIFEST_ENTRYPOINT: PASS")
PY
