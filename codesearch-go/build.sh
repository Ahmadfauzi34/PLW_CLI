#!/bin/bash
# Build codesearch-go (linux) with the version identity from REBUILD_LOG.md.
#
# Usage: ./build.sh
# Output: ./codesearch-go (overwrites the file /usr/local/bin/codesearch
# points to, so PATH picks it up with no extra install step).
#
# Gates, in order — a failure stops the build before any binary is written:
#   1. gofmt -l must be empty (no unformatted files; the script does NOT
#      auto-format, so a dirty tree fails loudly instead of silently
#      changing source during a build)
#   2. go vet ./...
#   3. go test ./...
# Then the binary is built with ldflags that inject the source tree hash
# and the UTC build time, so --version identifies exactly this source:
#   codesearch schema=go9 commit=<tree-hash> built=<UTC> go=<version>
# Without ldflags those fields honestly print "unknown" (see TestVersionLine).
#
# Go lookup: PATH first, then the local install at ~/.local/go/bin
# (the sandbox default PATH does not include it — see REBUILD_LOG.md
# 2026-10-20). Android builds are intentionally NOT part of this script:
# per user rule 2026-10-27, codesearch-android-arm64 is not a debt item
# and is only rebuilt when the user explicitly requests Android work.
set -euo pipefail
cd "$(dirname "$0")"

if command -v go >/dev/null 2>&1; then
  GO=go
elif [ -x "$HOME/.local/go/bin/go" ]; then
  GO="$HOME/.local/go/bin/go"
else
  echo "build.sh: go not found (PATH or \$HOME/.local/go/bin/go)" >&2
  exit 1
fi

# gofmt lives next to the go binary.
GOFMT="$(dirname "$(command -v "$GO")")/gofmt"
[ -x "$GOFMT" ] || GOFMT=gofmt

echo "== gofmt gate =="
DIRTY="$("$GOFMT" -l .)"
if [ -n "$DIRTY" ]; then
  echo "build.sh: unformatted files (run gofmt -w first): $DIRTY" >&2
  exit 1
fi

echo "== go vet =="
"$GO" vet ./...

echo "== go test =="
"$GO" test ./...

echo "== go build =="
COMMIT="$( (find . -name '*.go' -not -name '*_test.go' | sort | xargs sha256sum; sha256sum go.mod go.sum) | sha256sum | cut -c1-12 )"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
"$GO" build -ldflags "-X main.buildCommit=$COMMIT -X main.buildDate=$DATE" -o codesearch-go .
echo "built ./codesearch-go"
./codesearch-go --version
