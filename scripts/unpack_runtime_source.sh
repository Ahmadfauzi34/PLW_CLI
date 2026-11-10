#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CAPSULE="runtime_source/plw-runtime-src.tar.gz"
MANIFEST="runtime_source/CAPSULE_SHA256"

if [[ ! -f "$CAPSULE" ]]; then
  echo "runtime source capsule is missing: $CAPSULE" >&2
  exit 2
fi

EXPECTED="$(awk '{print $1}' "$MANIFEST")"
ACTUAL="$(sha256sum "$CAPSULE" | awk '{print $1}')"

if [[ "$ACTUAL" != "$EXPECTED" ]]; then
  echo "runtime source capsule digest mismatch" >&2
  echo "expected=$EXPECTED" >&2
  echo "actual=$ACTUAL" >&2
  exit 1
fi

# Reset every file an overlay touches before extraction. The repository
# checkout may carry previously-applied runtime files (they are build
# outputs, not sources): the capsule only restores its own files, so an
# overlay-created file such as core/portable_agent.py would survive and
# break its forward-only creation patch ("already exists"). The reset
# set is derived from the patches themselves, so a new overlay extends
# it automatically; files the capsule provides are restored by the
# extraction below, and overlay-created files are recreated by their
# overlay in order.
reset_overlay_targets() {
  local patch_file decoded overlay_dir
  for patch_file in runtime_overlay/*/change.patch; do
    [[ -f "$patch_file" ]] || continue
    grep -E '^(\+\+\+|---) [ab]/' "$patch_file" 2>/dev/null \
      | sed -E 's#^(\+\+\+|---) [ab]/##' | sort -u \
      | while read -r target; do
        [[ -n "$target" ]] && rm -f "$target"
      done || true
  done
  for overlay_dir in runtime_overlay/*/; do
    [[ -f "${overlay_dir}OVERLAY_SHA256" ]] || continue
    [[ -e "${overlay_dir}part00.b64" ]] || continue
    decoded="$(mktemp)"
    if cat "${overlay_dir}"part*.b64 | tr -d '\r\n' | base64 --decode > "$decoded" 2>/dev/null; then
      grep -E '^(\+\+\+|---) [ab]/' "$decoded" 2>/dev/null \
        | sed -E 's#^(\+\+\+|---) [ab]/##' | sort -u \
        | while read -r target; do
          [[ -n "$target" ]] && rm -f "$target"
        done || true
    fi
    rm -f "$decoded"
  done
}
reset_overlay_targets

# Archive uid/gid are transport metadata, not a build requirement. Rootless
# builders may be unable to chown even when every archive byte is valid.
tar --no-same-owner -xzf "$CAPSULE" -C "$ROOT"

echo "RUNTIME_SOURCE_CAPSULE: PASS sha256:$ACTUAL"

bash scripts/apply_runtime_overlays.sh
