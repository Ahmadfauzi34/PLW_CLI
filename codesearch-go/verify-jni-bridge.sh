#!/bin/bash
# Run the JNI bridge (--related two directions, go14 P2) verification
# harness (Go skill, build tag "verify").
#
# Usage: ./verify-jni-bridge.sh
# Exit 0 only if the harness reports IsCompleteAndVerified + READY_TO_COMMIT.
#
# Same file-list rationale as verify-kotlin-parity.sh: jni_bridge_spec.go
# has its own main(), so `go run` compiles exactly the listed files
# (build tags are ignored in file mode) and main.go is omitted.
# kotlin_fallback.go is production code and must be listed.
set -euo pipefail
cd "$(dirname "$0")"

if command -v go >/dev/null 2>&1; then
  GO=go
elif [ -x "$HOME/.local/go/bin/go" ]; then
  GO="$HOME/.local/go/bin/go"
else
  echo "verify-jni-bridge.sh: go not found (PATH or \$HOME/.local/go/bin/go)" >&2
  exit 1
fi

"$GO" run harness.go jni_bridge_spec.go options.go astchunk.go callers.go index.go kotlin_fallback.go related.go scan.go search.go snippet.go specstore.go
