#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]

RUNTIME_PATHS = [
    ROOT / name
    for name in ("core", "codebase", "context", "bridge", "memory", "fullstack", "ui")
]
RUNTIME_FILES = [
    ROOT / name
    for name in ("agent_tool.py", "hott_kernel.py", "language_semantics.py", "plw_cli.py")
]

CHECKS = [
    (
        "source-byte-runtime-dependency",
        re.compile(r"Path\(__file__\).*?(?:read_bytes|with_name\([^\n]+\.py)", re.S),
        "runtime must not require neighboring Python source bytes",
    ),
    (
        "tool-root-runtime-state",
        re.compile(r"_TOOL_ROOT\s*/\s*[\"']data[\"']\s*/\s*[\"']runtime[\"']"),
        "runtime state must use PLW/XDG state paths, not the source tree",
    ),
    (
        # Track 4 (K2): spawning TOOL_ROOT/*.py assumes a source
        # checkout next to the wrapper — in a frozen binary the
        # .py files are not there, so the spawn fails cryptically.
        "tool-root-py-subprocess",
        re.compile(r"TOOL_ROOT\s*/[^\n]*\.py[\"']"),
        "subprocess must not assume a neighboring Python source file exists",
    ),
    (
        # Track 4 (K2): same shape for shell scripts; no current
        # site (verified by grep), this is preventive.
        "tool-root-sh-subprocess",
        re.compile(r"TOOL_ROOT\s*/[^\n]*\.sh[\"']"),
        "subprocess must not assume a neighboring shell script exists",
    ),
]

# Sites a check already sees that are guarded in code, not accidents.
# The regex proves the check detects the pattern; the allowlist proves
# the site is intentional. A new unguarded site still fails.
ALLOWLIST = {
    ("agent_tool.py", "tool-root-py-subprocess"):
        "guarded by the frozen/missing-runtime branch in _advanced()",
}


def iter_sources():
    for base in RUNTIME_PATHS:
        if base.exists():
            yield from base.rglob("*.py")
    for path in RUNTIME_FILES:
        if path.exists():
            yield path


def main() -> int:
    sources = list(iter_sources())
    if not sources:
        print("BINARY_SAFETY: source not imported yet; nothing to check")
        return 0

    failures = []
    allowed = []
    for path in sources:
        text = path.read_text(encoding="utf-8")
        rel = path.relative_to(ROOT).as_posix()
        for name, pattern, reason in CHECKS:
            if pattern.search(text):
                if (rel, name) in ALLOWLIST:
                    allowed.append((rel, name, ALLOWLIST[(rel, name)]))
                else:
                    failures.append((rel, name, reason))

    if failures:
        print("BINARY_SAFETY: FAIL")
        for path, name, reason in failures:
            print(f"- {path}: {name}: {reason}")
        return 1

    suffix = f" ({len(allowed)} allowlisted site{'s' if len(allowed) != 1 else ''})" if allowed else ""
    print(f"BINARY_SAFETY: PASS ({len(sources)} runtime Python files checked{suffix})")
    for path, name, reason in allowed:
        print(f"- allowlisted {path}: {name}: {reason}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
