#!/usr/bin/env bash
# Check that codesearch-go and the Python adapter agree on the
# engine contract (G1, G2, G13). Runs before the smoke test in CI
# and is cheap to run locally.
#
# Resolves the effective core/codesearch_engine.py directly from the
# SHA-256 verified runtime capsule plus any active overlays registered
# in scripts/apply_runtime_overlays.sh that touch core/codesearch_engine.py,
# so it works cleanly on both bare checkouts and unpacked trees without
# hardcoding overlay patch filenames.
#
# Fails non-zero on:
#   - runtime_source/plw-runtime-src.tar.gz digest mismatch
#   - engine schemaV != adapter KNOWN_SCHEMA
#   - engine schemaV not listed in adapter SUPPORTED_SCHEMAS
#   - codesearch-go/ENGINE_CONTRACT.md does not name the current schema
#   - when an engine binary exists at artifacts/codesearch-go-linux-x86_64
#     or codesearch-go/codesearch-go, its SHA256 is not in PINNED_ENGINE_DIGESTS
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CAPSULE="$ROOT/runtime_source/plw-runtime-src.tar.gz"
MANIFEST="$ROOT/runtime_source/CAPSULE_SHA256"

if [[ ! -f "$CAPSULE" || ! -f "$MANIFEST" ]]; then
  echo "check_engine_contract: runtime capsule or CAPSULE_SHA256 missing" >&2
  echo "CHECK_ENGINE_CONTRACT: FAIL" >&2
  exit 1
fi

EXPECTED_CAPSULE="$(awk '{print $1}' "$MANIFEST")"
ACTUAL_CAPSULE="$(sha256sum "$CAPSULE" | awk '{print $1}')"
if [[ "$ACTUAL_CAPSULE" != "$EXPECTED_CAPSULE" ]]; then
  echo "check_engine_contract: runtime capsule digest mismatch (expected=$EXPECTED_CAPSULE actual=$ACTUAL_CAPSULE)" >&2
  echo "CHECK_ENGINE_CONTRACT: FAIL" >&2
  exit 1
fi

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Extract core/codesearch_engine.py from the verified capsule into an isolated staging dir
tar --no-same-owner -xzf "$CAPSULE" -C "$TMPDIR" core/codesearch_engine.py

# Apply any active overlays in scripts/apply_runtime_overlays.sh that modify core/codesearch_engine.py
while IFS= read -r line; do
  kind=""
  name=""
  if [[ "$line" =~ ^apply_reviewable_overlay[[:space:]]+\"([^\"]+)\" ]]; then
    kind="reviewable"
    name="${BASH_REMATCH[1]}"
  elif [[ "$line" =~ ^apply_b64_overlay[[:space:]]+\"([^\"]+)\" ]]; then
    kind="b64"
    name="${BASH_REMATCH[1]}"
  else
    continue
  fi

  overlay_dir="$ROOT/runtime_overlay/$name"
  tmp_patch="$TMPDIR/_overlay_${name}.patch"
  if [[ "$kind" == "reviewable" ]]; then
    patch_file="$overlay_dir/change.patch"
    patch_manifest="$overlay_dir/PATCH_SHA256"
    if [[ ! -f "$patch_file" || ! -f "$patch_manifest" ]]; then
      echo "check_engine_contract: missing reviewable overlay $name" >&2
      exit 1
    fi
    exp="$(awk '{print $1}' "$patch_manifest")"
    act="$(sha256sum "$patch_file" | awk '{print $1}')"
    if [[ "$act" != "$exp" ]]; then
      echo "check_engine_contract: digest mismatch in overlay $name" >&2
      exit 1
    fi
    cp "$patch_file" "$tmp_patch"
  else
    patch_manifest="$overlay_dir/OVERLAY_SHA256"
    [[ -f "$patch_manifest" ]] || continue
    cat "$overlay_dir"/part*.b64 | tr -d '\r\n' | base64 --decode > "$tmp_patch"
    exp="$(awk '{print $1}' "$patch_manifest")"
    act="$(sha256sum "$tmp_patch" | awk '{print $1}')"
    if [[ "$act" != "$exp" ]]; then
      echo "check_engine_contract: digest mismatch in b64 overlay $name" >&2
      exit 1
    fi
  fi

  if grep -qE '^(\+\+\+|---) [ab]/core/codesearch_engine\.py$' "$tmp_patch"; then
    # Extract any companion files referenced by this patch that exist in the capsule
    while IFS= read -r target; do
      [[ -n "$target" && ! -f "$TMPDIR/$target" ]] || continue
      tar --no-same-owner -xzf "$CAPSULE" -C "$TMPDIR" "$target" 2>/dev/null || true
    done < <(grep -E '^(\+\+\+|---) [ab]/' "$tmp_patch" | sed -E 's#^(\+\+\+|---) [ab]/##' | sort -u)
    patch --batch --forward -p1 -d "$TMPDIR" < "$tmp_patch" >/dev/null
  fi
  rm -f "$tmp_patch"
done < "$ROOT/scripts/apply_runtime_overlays.sh"

ENGINE_SCHEMA="$(grep -E '^const schemaV = ' "$ROOT/codesearch-go/index.go" | sed -E 's/^const schemaV = "([^"]+)".*/\1/')"

BIN=""
for candidate in "$ROOT/artifacts/codesearch-go-linux-x86_64" "$ROOT/codesearch-go/codesearch-go"; do
  if [[ -f "$candidate" ]]; then BIN="$candidate"; break; fi
done
BIN_DIGEST=""
if [[ -n "$BIN" ]]; then
  BIN_DIGEST="$(sha256sum "$BIN" | cut -d' ' -f1)"
fi

python3 - "$TMPDIR/core/codesearch_engine.py" "$ENGINE_SCHEMA" "$ROOT/codesearch-go/ENGINE_CONTRACT.md" "$BIN" "$BIN_DIGEST" << 'PY'
import ast, pathlib, sys

adapter_path, engine_schema, contract_path, bin_path, bin_digest = sys.argv[1:6]
tree = ast.parse(pathlib.Path(adapter_path).read_text(encoding="utf-8"), filename=adapter_path)

values = {}
for node in tree.body:
    if isinstance(node, ast.Assign) and len(node.targets) == 1 and isinstance(node.targets[0], ast.Name):
        name = node.targets[0].id
        if name in {"KNOWN_SCHEMA", "SUPPORTED_SCHEMAS", "PINNED_ENGINE_DIGESTS"}:
            values[name] = ast.literal_eval(node.value)

fail = False
adapter_schema = values.get("KNOWN_SCHEMA")
supported_schemas = values.get("SUPPORTED_SCHEMAS")
pinned_digests = values.get("PINNED_ENGINE_DIGESTS")

if not engine_schema:
    print("check_engine_contract: could not read schemaV from codesearch-go/index.go", file=sys.stderr)
    fail = True
elif not isinstance(adapter_schema, str) or not adapter_schema:
    print("check_engine_contract: could not read KNOWN_SCHEMA from effective core/codesearch_engine.py", file=sys.stderr)
    fail = True
elif engine_schema != adapter_schema:
    print(f"check_engine_contract: MISMATCH engine schemaV={engine_schema} adapter KNOWN_SCHEMA={adapter_schema}", file=sys.stderr)
    print("  Fix: bump schemaV (engine) or update SUPPORTED_SCHEMAS (adapter) — see codesearch-go/ENGINE_CONTRACT.md", file=sys.stderr)
    fail = True
else:
    print(f"check_engine_contract: schema {engine_schema} agrees (engine == adapter)")

if isinstance(supported_schemas, dict) and engine_schema in supported_schemas:
    print(f"check_engine_contract: SUPPORTED_SCHEMAS includes {engine_schema}")
else:
    print(f"check_engine_contract: SUPPORTED_SCHEMAS does not include {engine_schema}", file=sys.stderr)
    fail = True

contract_text = pathlib.Path(contract_path).read_text(encoding="utf-8") if pathlib.Path(contract_path).is_file() else ""
if f"Current schema: `{engine_schema}`" not in contract_text:
    print(f"check_engine_contract: ENGINE_CONTRACT.md does not name current schema `{engine_schema}`", file=sys.stderr)
    fail = True
else:
    print(f"check_engine_contract: ENGINE_CONTRACT.md names schema {engine_schema}")

if not bin_path:
    print("check_engine_contract: pin check skipped (no engine binary found at artifacts/ or codesearch-go/)")
else:
    if isinstance(pinned_digests, dict) and bin_digest in pinned_digests:
        print(f"check_engine_contract: binary digest {bin_digest} is pinned ({bin_path})")
    else:
        print(f"check_engine_contract: PIN MISSING binary digest {bin_digest} ({bin_path}) is not in PINNED_ENGINE_DIGESTS", file=sys.stderr)
        print("  Fix: rebuild with codesearch-go/build.sh and add the digest to PINNED_ENGINE_DIGESTS in the same commit", file=sys.stderr)
        fail = True

if fail:
    print("CHECK_ENGINE_CONTRACT: FAIL", file=sys.stderr)
    sys.exit(1)
print("CHECK_ENGINE_CONTRACT: PASS")
PY
