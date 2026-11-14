#!/usr/bin/env bash
# Smoke: Kotlin language label in the topology engine layer (G2).
#
# Symbol nodes from `.kt` files used to be labeled `unknown` because
# FILE_LANGUAGE_BY_EXTENSION in core/topology_engine.py had no `.kt`
# entry (1,327 of the 1,332 unknown nodes on oopbot-trial). The fix is
# one map entry: ".kt" -> "kotlin".
#
# Asserts the label function directly (no engine binary, no index):
#   - .kt -> kotlin; .ts/.tsx/.js/.jsx/.rs/.go/.py keep their labels
#   - unknown extensions stay unknown; .kts stays unknown on purpose
#     (Kotlin script is out of scope for G2, like the definition
#     patterns, which remain go/rust/python only)
#   - .kt stays OUT of ENGINE_FILE_EXTENSIONS: .kt files are stable
#     graph vertices already, the engine walk is only for files the
#     stable graph never sees (.go/.rs/.py)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

python3 - "$ROOT" <<'PY'
import sys

sys.path.insert(0, sys.argv[1])

from core.topology_engine import (
    ENGINE_FILE_EXTENSIONS,
    FILE_LANGUAGE_BY_EXTENSION,
    _file_language,
)

want = {
    "a.kt": "kotlin",
    "a.ts": "typescript",
    "a.tsx": "tsx",
    "a.js": "javascript",
    "a.jsx": "jsx",
    "a.rs": "rust",
    "a.go": "go",
    "a.py": "python",
    "a.xyz": "unknown",
    "a.kts": "unknown",
    "a.md": "unknown",
}
for path, language in want.items():
    got = _file_language(path)
    if got != language:
        raise SystemExit(f"_file_language({path!r}) = {got!r}, expected {language!r}")

if FILE_LANGUAGE_BY_EXTENSION.get(".kt") != "kotlin":
    raise SystemExit("FILE_LANGUAGE_BY_EXTENSION has no .kt -> kotlin entry")
if ".kt" in ENGINE_FILE_EXTENSIONS:
    raise SystemExit(".kt must not join ENGINE_FILE_EXTENSIONS (stable graph already covers it)")

print("SMOKE_TOPOLOGY_KOTLIN_LABEL: PASS")
PY
