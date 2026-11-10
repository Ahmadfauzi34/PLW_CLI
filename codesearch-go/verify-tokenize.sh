#!/bin/bash
# Run the tokenize verification harness (Go skill, build tag "verify").
#
# Usage: ./verify-tokenize.sh
# Exit 0 only if the harness reports IsCompleteAndVerified + READY_TO_COMMIT.
#
# Why a file list, and why main.go is omitted:
#   tokenize_spec.go has its own main(), and main.go has the real main(),
#   so compiling the whole package with the "verify" tag would give two
#   mains. `go run` with an explicit file list compiles exactly those
#   files (build tags are ignored in file mode), so we list every package
#   file except main.go. The options type that callers.go/related.go need
#   is available via options.go, which was moved verbatim out of main.go.
#   Normal build/vet/test (no tag) ignore tokenize_spec.go, so this
#   script is the only way the spec runs.
set -euo pipefail
cd "$(dirname "$0")"

if command -v go >/dev/null 2>&1; then
  GO=go
elif [ -x "$HOME/.local/go/bin/go" ]; then
  GO="$HOME/.local/go/bin/go"
else
  echo "verify-tokenize.sh: go not found (PATH or \$HOME/.local/go/bin/go)" >&2
  exit 1
fi

"$GO" run harness.go tokenize_spec.go options.go astchunk.go callers.go index.go related.go scan.go search.go snippet.go specstore.go
