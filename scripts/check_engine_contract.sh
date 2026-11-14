#!/usr/bin/env bash
# check_engine_contract.sh — G13 + G2: the engine's schemaV and the
# adapter's KNOWN_SCHEMA must agree, ENGINE_CONTRACT.md must name
# the current schema, and the just-built engine binary's digest must
# be in the adapter's PINNED_ENGINE_DIGESTS. Run in CI (build-binary
# workflow, job codesearch-engine-smoke) so a mismatch is caught at
# push time, not at runtime in a user's hands. If this goes red, the
# fix is a schema bump (engine), a SUPPORTED_SCHEMAS update
# (adapter), or a pin update (adapter) — never a change to this
# script.
#
# NOTE: the adapter's KNOWN_SCHEMA and PINNED_ENGINE_DIGESTS are
# read from the overlay patches (runtime_overlay/engine_state_v1/
# and engine_pin_v4/change.patch), NOT from core/codesearch_engine.py
# — that file only exists after the capsule is unpacked, and this
# job runs on the bare repo checkout (a grep of core/ here fails
# with "No such file", CI run 42868246692). The patches are the
# repo-resident source of truth for the adapter: post-image lines
# are '+' (added) and ' ' (context); '-' lines are the pre-image
# and are excluded. The pin moved from engine_pin_v1 to
# engine_pin_v2 when F2 rotated the reproducible-build digest (the
# v1 patch still holds the superseded digest), then to engine_pin_v3
# when the Kotlin engine (K1-K4) rotated it again, then to
# engine_pin_v4 when the vendored build (Track 4, K5) rotated it:
# -mod=vendor drops the dep h1 hash from buildinfo, so the vendored
# binary can never match the module-mode pin; then to engine_pin_v5
# when the defs-only opsi 1 fix rotated it (digest b6f0ec48…,
# commit 9a3db30a5027; local + CI builds agree); then to engine_pin_v6
# when the SOLID refactor 1-3 rotated it (digest 62b532ec…,
# commit e88c2e3f11f8; local + CI builds agree); then to
# engine_state_v2 when the schema moved go9->go10 (interface members
# are definitions; go9 stays listed because the v0.2.5 release asset
# is still go9) and to engine_pin_v7 when that engine rotated the
# digest (828eeeab…, commit b6e9a95c06a9; local + CI builds agree)
# and to engine_pin_v8 when the G1 defs-only fix rotated the digest
# (0a3f6179…, commit efff2b7dd3ad; local + CI builds agree).
# If a later overlay moves either constant again, update the patch
# paths below — the script then fails closed with "could not read ...".
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ADAPTER_PATCH="$ROOT/runtime_overlay/engine_state_v2/change.patch"
PIN_PATCH="$ROOT/runtime_overlay/engine_pin_v8/change.patch"
ENGINE_SCHEMA="$(grep -E '^const schemaV = ' "$ROOT/codesearch-go/index.go" | sed -E 's/^const schemaV = "([^"]+)".*/\1/')"
ADAPTER_SCHEMA="$(grep -E '^[+ ]KNOWN_SCHEMA = ' "$ADAPTER_PATCH" | sed -E 's/^[+ ]KNOWN_SCHEMA = "([^"]+)".*/\1/')"

fail=0
if [ -z "$ENGINE_SCHEMA" ]; then
  echo "check_engine_contract: could not read schemaV from codesearch-go/index.go" >&2
  fail=1
elif [ -z "$ADAPTER_SCHEMA" ]; then
  echo "check_engine_contract: could not read KNOWN_SCHEMA from $ADAPTER_PATCH" >&2
  fail=1
elif [ "$ENGINE_SCHEMA" != "$ADAPTER_SCHEMA" ]; then
  echo "check_engine_contract: MISMATCH engine schemaV=$ENGINE_SCHEMA adapter KNOWN_SCHEMA=$ADAPTER_SCHEMA" >&2
  echo "  Fix: bump schemaV (engine) or update SUPPORTED_SCHEMAS (adapter) — see codesearch-go/ENGINE_CONTRACT.md" >&2
  fail=1
else
  echo "check_engine_contract: schema $ENGINE_SCHEMA agrees (engine == adapter)"
fi

if ! grep -q "Current schema: \`$ENGINE_SCHEMA\`" "$ROOT/codesearch-go/ENGINE_CONTRACT.md" 2>/dev/null; then
  echo "check_engine_contract: ENGINE_CONTRACT.md does not name current schema \`$ENGINE_SCHEMA\`" >&2
  fail=1
else
  echo "check_engine_contract: ENGINE_CONTRACT.md names schema $ENGINE_SCHEMA"
fi

# G2: the just-built engine binary's digest must be pinned. The
# binary lives at artifacts/ in the smoke job (downloaded artifact)
# or at codesearch-go/ in the build job (fresh build). If neither
# exists (e.g. a local run without a build), the pin check is
# skipped with a visible note — the schema checks above still run.
BIN=""
for candidate in "$ROOT/artifacts/codesearch-go-linux-x86_64" "$ROOT/codesearch-go/codesearch-go"; do
  if [ -f "$candidate" ]; then BIN="$candidate"; break; fi
done
if [ -z "$BIN" ]; then
  echo "check_engine_contract: pin check skipped (no engine binary found at artifacts/ or codesearch-go/)"
else
  BIN_DIGEST="$(sha256sum "$BIN" | cut -d' ' -f1)"
  if grep -qE "^[+ ] *\"$BIN_DIGEST\":" "$PIN_PATCH" 2>/dev/null; then
    echo "check_engine_contract: binary digest $BIN_DIGEST is pinned ($BIN)"
  else
    echo "check_engine_contract: PIN MISSING binary digest $BIN_DIGEST ($BIN) is not in PINNED_ENGINE_DIGESTS ($PIN_PATCH)" >&2
    echo "  Fix: rebuild with codesearch-go/build.sh and add the digest to PINNED_ENGINE_DIGESTS in the same commit" >&2
    fail=1
  fi
fi

if [ "$fail" -ne 0 ]; then
  echo "CHECK_ENGINE_CONTRACT: FAIL" >&2
  exit 1
fi
echo "CHECK_ENGINE_CONTRACT: PASS"
