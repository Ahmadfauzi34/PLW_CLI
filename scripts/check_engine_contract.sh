#!/usr/bin/env bash
# check_engine_contract.sh — G13: the engine's schemaV and the
# adapter's KNOWN_SCHEMA must agree, and ENGINE_CONTRACT.md must
# name the current schema. Run in CI (build-binary workflow, job
# codesearch-go) so a mismatch is caught at push time, not at
# runtime in a user's hands. If this goes red, the fix is a schema
# bump (engine) or a SUPPORTED_SCHEMAS update (adapter) — never a
# change to this script.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENGINE_SCHEMA="$(grep -E '^const schemaV = ' "$ROOT/codesearch-go/index.go" | sed -E 's/^const schemaV = "([^"]+)".*/\1/')"
ADAPTER_SCHEMA="$(grep -E '^KNOWN_SCHEMA = ' "$ROOT/core/codesearch_engine.py" | sed -E 's/^KNOWN_SCHEMA = "([^"]+)".*/\1/')"

fail=0
if [ -z "$ENGINE_SCHEMA" ]; then
  echo "check_engine_contract: could not read schemaV from codesearch-go/index.go" >&2
  fail=1
elif [ -z "$ADAPTER_SCHEMA" ]; then
  echo "check_engine_contract: could not read KNOWN_SCHEMA from core/codesearch_engine.py" >&2
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

if [ "$fail" -ne 0 ]; then
  echo "CHECK_ENGINE_CONTRACT: FAIL" >&2
  exit 1
fi
echo "CHECK_ENGINE_CONTRACT: PASS"
