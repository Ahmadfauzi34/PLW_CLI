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
# Then the binary is built with ldflags that inject the source tree hash,
# so --version identifies exactly this source:
#   codesearch schema=go9 commit=<tree-hash> built=unknown go=<version>
#
# REPRODUCIBLE BUILD (G2): the build is byte-identical for the same
# source + toolchain — `-trimpath` strips the local build directory,
# `-buildvcs=false` strips the git stamp (a checkout's revision /
# dirty flag would otherwise change the bytes: measured 2026-11-11,
# same source built inside a git repo gave a different SHA256),
# `CGO_ENABLED=0` (the CI job builds with CGO_ENABLED=0; a local
# build with the default CGO_ENABLED=1 gives the same size but
# different bytes — measured 2026-11-11: the CI binary's digest
# 23719bda… vs the local pin ef9a5653…, same source, same size — and
# CI went red on the pin because of it), and NO wall-clock build date
# is injected (a date alone changes the binary bytes, so any digest
# pin would go stale on the next build; measured 2026-11-11: two
# builds differing only in buildDate gave different SHA256).
# `built` honestly prints `unknown`; the build date belongs to
# release metadata, not to the binary. Verify: build twice from
# different directories and compare sha256 — they must match.
# Without ldflags the commit also prints "unknown" (see TestVersionLine).
#
# Go lookup: PATH first, then the local install at ~/.local/go/bin
# (the sandbox default PATH does not include it — see REBUILD_LOG.md
# 2026-10-20). Android builds are intentionally NOT part of this script:
# per user rule 2026-10-27, codesearch-android-arm64 is not a debt item
# and is only rebuilt when the user explicitly requests Android work.
set -euo pipefail
export CGO_ENABLED=0
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
# vendor/ is upstream source (Track 4, K5): gofmt -w on it would dirty
# the vendored tree, so the gate covers our own files only.
DIRTY="$("$GOFMT" -l . | grep -v '^vendor/' || true)"
if [ -n "$DIRTY" ]; then
  echo "build.sh: unformatted files (run gofmt -w first): $DIRTY" >&2
  exit 1
fi

echo "== go vet =="
"$GO" vet -mod=vendor ./...

echo "== go test =="
"$GO" test -mod=vendor ./...

echo "== go build =="
COMMIT="$( (find . -name '*.go' -not -name '*_test.go' -not -path './vendor/*' | sort | xargs sha256sum; sha256sum go.mod go.sum) | sha256sum | cut -c1-12 )"
# -mod=vendor (Track 4, K5): the build never touches the module
# proxy — vendor/ is committed, so a build without network works
# and a vendor/modules.txt mismatch fails loudly instead of
# silently falling back to a download.
"$GO" build -mod=vendor -trimpath -buildvcs=false -ldflags "-X main.buildCommit=$COMMIT" -o codesearch-go .
echo "built ./codesearch-go"
./codesearch-go --version
