#!/usr/bin/env bash
set -euo pipefail

BIN="${1:-}"
if [[ -z "$BIN" || ! -x "$BIN" ]]; then
  echo "usage: $0 /path/to/plw-binary" >&2
  exit 2
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/plw-smoke-rl-hardening.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

REPO="$TMP_DIR/repo"
mkdir -p "$REPO/src"
cat > "$REPO/src/chronos.ts" <<'EOF'
export function computeWindow(x: number): number {
  if (x === 1) {
    return true ? 1 : 0;
  }
  return x + 1;
}
EOF
cat > "$REPO/src/caller.ts" <<'EOF'
import { computeWindow } from "./chronos.js";
export function runChronos(): number {
  return computeWindow(42);
}
EOF

python3 - "$BIN" "$REPO" "$TMP_DIR" <<'PY'
import hashlib
import json
import os
import subprocess
import sys
import time
from pathlib import Path

bin_path, repo_path, tmp_dir = sys.argv[1], sys.argv[2], sys.argv[3]

def run_plw(args, env_extra=None):
    env = os.environ.copy()
    if env_extra:
        env.update(env_extra)
    return subprocess.run(
        [bin_path, *args],
        capture_output=True,
        text=True,
        env=env,
        timeout=60,
    )

# 1. Valid orient receipt + R5/R6 fields + whitespace normalization
p_orient = run_plw(["agent", "orient", repo_path, "--task", "audit   computeWindow\nflow", "--json"])
assert p_orient.returncode == 0, p_orient.stderr
d_orient = json.loads(p_orient.stdout)
r_orient = d_orient["receipt"]
assert r_orient["schema_version"] == "plw-task-receipt-v1", r_orient
assert r_orient["normalized_task"] == "audit computeWindow flow", r_orient
assert isinstance(r_orient.get("graph_signature"), str) and r_orient["graph_signature"].startswith("sha256:"), r_orient
assert isinstance(r_orient.get("result_digest"), str) and r_orient["result_digest"].startswith("sha256:"), r_orient
assert isinstance(r_orient.get("provenance"), dict) and r_orient["provenance"].get("issuer") == "plw.workflow.task-receipt.v1", r_orient

# R6: Whitespace-normalized task accepted by next stage
p_res = run_plw([
    "resolve-target", "audit computeWindow flow", repo_path,
    "--receipt", json.dumps(r_orient), "--json"
])
assert p_res.returncode == 0, f"Expected rc=0 for whitespace-normalized task, got {p_res.returncode}: {p_res.stdout} {p_res.stderr}"

# 2. R1: Forged receipt (valid sha256 fingerprint, missing or tampered HMAC provenance) rejected
task_str = "audit computeWindow flow"
forged_fp = "sha256:" + hashlib.sha256(f"orient\n{task_str}\n{repo_path}".encode("utf-8")).hexdigest()
forged_receipt = {
    "schema_version": "plw-task-receipt-v1",
    "stage": "orient",
    "task": task_str,
    "normalized_task": task_str,
    "scan_root": repo_path,
    "resolved_scan_root": os.path.abspath(repo_path),
    "fingerprint": forged_fp,
    "graph_signature": r_orient["graph_signature"],
    "result_digest": r_orient["result_digest"],
    "provenance": None,
}
p_forged = run_plw([
    "resolve-target", task_str, repo_path,
    "--receipt", json.dumps(forged_receipt), "--json"
])
assert p_forged.returncode == 3, f"Expected rc=3 for forged receipt, got {p_forged.returncode}"
err_forged = json.loads(p_forged.stdout)
assert err_forged.get("error") == "receipt_mismatch", err_forged
assert "provenance seal" in err_forged.get("detail", ""), err_forged

# 3. R2: --strict-chain and PLW_RECEIPT_REQUIRED=1 reject missing receipt on non-entry stage
p_strict_flag = run_plw(["resolve-target", task_str, repo_path, "--strict-chain", "--json"])
assert p_strict_flag.returncode == 3, p_strict_flag.stdout
err_strict_flag = json.loads(p_strict_flag.stdout)
assert err_strict_flag.get("error") == "receipt_required", err_strict_flag

p_strict_env = run_plw(
    ["evidence-needs", task_str, repo_path, "--json"],
    env_extra={"PLW_RECEIPT_REQUIRED": "1"},
)
assert p_strict_env.returncode == 3, p_strict_env.stdout
err_strict_env = json.loads(p_strict_env.stdout)
assert err_strict_env.get("error") == "receipt_required", err_strict_env

# 4. R4: semantic-route detour preserves scan_root and graph_signature
p_sem = run_plw([
    "semantic", "route", "audit computeWindow flow",
    "--receipt", json.dumps(r_orient), "--json"
])
assert p_sem.returncode == 0, p_sem.stdout
d_sem = json.loads(p_sem.stdout)
r_sem = d_sem["receipt"]
assert r_sem["scan_root"] == repo_path, r_sem
assert r_sem["graph_signature"] == r_orient["graph_signature"], r_sem

p_res_from_sem = run_plw([
    "resolve-target", "audit computeWindow flow", repo_path,
    "--receipt", json.dumps(r_sem), "--json"
])
assert p_res_from_sem.returncode == 0, p_res_from_sem.stdout

# 5. R3: Mutating repository invalidates earlier receipt's graph_signature
chronos_file = Path(repo_path) / "src" / "chronos.ts"
orig_text = chronos_file.read_text(encoding="utf-8")
try:
    chronos_file.write_text(orig_text + "\nexport const EXTRA_FLAG = 99;\n", encoding="utf-8")
    p_stale = run_plw([
        "resolve-target", "audit computeWindow flow", repo_path,
        "--receipt", json.dumps(r_orient), "--json"
    ])
    assert p_stale.returncode == 3, f"Expected stale receipt rc=3, got {p_stale.returncode}: {p_stale.stdout}"
    err_stale = json.loads(p_stale.stdout)
    assert err_stale.get("error") == "receipt_mismatch", err_stale
    assert "graph_signature" in err_stale.get("detail", "") and "stale" in err_stale.get("detail", ""), err_stale
finally:
    chronos_file.write_text(orig_text, encoding="utf-8")

# 6. L1/L4: Doctor surfaces runtime.accountability_provenance (store_file vs env)
p_doc_local = run_plw(["doctor", repo_path, "--json"])
assert p_doc_local.returncode == 0, p_doc_local.stderr
doc_local = json.loads(p_doc_local.stdout)
ap_local = doc_local["runtime"]["accountability_provenance"]
assert ap_local["self_signed_local_key"] is True, ap_local
assert ap_local["provenance_key_source"] in ("store_file", "uninitialized_store_file"), ap_local
assert isinstance(ap_local.get("store_id"), str) and ap_local["store_id"].startswith("sha256:"), ap_local
assert "PROVENANCE_KEY_SELF_SIGNED_LOCAL_ONLY" in str(ap_local.get("warning")), ap_local

env_key_hex = "hex:" + ("ab" * 32)
p_doc_env = run_plw(
    ["doctor", repo_path, "--json"],
    env_extra={"PLW_ACCOUNTABILITY_PROVENANCE_KEY": env_key_hex},
)
assert p_doc_env.returncode == 0, p_doc_env.stderr
ap_env = json.loads(p_doc_env.stdout)["runtime"]["accountability_provenance"]
assert ap_env["provenance_key_source"] == "env", ap_env
assert ap_env["self_signed_local_key"] is False, ap_env
assert ap_env["warning"] is None, ap_env

# 7. L2/L5: Sub-second monotonic guard, backward clock guard, and lease_active/recovered_lease_expired
from fullstack.validation_lease import _claim_authorization_binding
from core.accountability_provenance import provenance_key_status

now_s = int(time.time())
# Simulate backward clock jump: evaluated_at_epoch in the future relative to current clock
future_elig = {
    "evaluated_at_epoch": now_s + 120,
    "proposed_expires_at_epoch": now_s + 135,
    "lease_request_digest": "sha256:" + "1" * 64,
    "authorization_binding_digest": "sha256:" + "2" * 64,
    "authorization_assertion_digest": "sha256:" + "3" * 64,
    "request_id": "sha256:" + "4" * 64,
    "request_set_digest": "sha256:" + "5" * 64,
    "scenario_id": "scen-1",
    "validator_id": "val-1",
    "executor_id": "exec-1",
    "operation_key": "op-1",
    "authorization_principal": "principal-1",
    "idempotency_key": "idem-clock-backward-0001",
}
res_back = _claim_authorization_binding(future_elig, store_dir=tmp_dir, eval_monotonic=time.monotonic())
assert res_back["status"] == "conflict", res_back
assert "system_clock_moved_backward" in res_back["failed_conditions"], res_back

# Simulate .999s integer boundary crossing with ttl=1 and fresh monotonic timer
now_s = int(time.time())
boundary_elig = {
    **future_elig,
    "evaluated_at_epoch": now_s - 1,
    "proposed_expires_at_epoch": now_s,
    "idempotency_key": "idem-subsec-boundary-0001",
}
res_subsec = _claim_authorization_binding(boundary_elig, store_dir=tmp_dir, eval_monotonic=time.monotonic())
assert res_subsec["status"] == "acquired", f"Expected sub-second guard to acquire lease, got {res_subsec}"
assert res_subsec["receipt"]["expires_at_epoch"] > res_subsec["receipt"]["acquired_at_epoch"], res_subsec

# 8. Part C: plw chain (and plw agent chain) single-process pipeline
p_chain = run_plw(["chain", "audit computeWindow flow", repo_path, "--with-candidates", "--json"])
assert p_chain.returncode == 0, p_chain.stderr
d_chain = json.loads(p_chain.stdout)
assert d_chain["schema_version"] == "plw-task-chain-v1", d_chain
assert d_chain["stages_executed"] == [
    "orient", "resolve-target", "evidence-needs", "capability-match", "candidate-capability-match"
], d_chain["stages_executed"]
assert len(d_chain["receipt_chain"]) == 5, len(d_chain["receipt_chain"])
for r in d_chain["receipt_chain"]:
    assert r["graph_signature"] == d_chain["graph_signature"], r
    assert isinstance(r.get("provenance"), dict) and r["provenance"].get("mac", "").startswith("hmac-sha256:"), r

print("SMOKE_RECEIPT_LEASE_HARDENING: PASS")
PY
