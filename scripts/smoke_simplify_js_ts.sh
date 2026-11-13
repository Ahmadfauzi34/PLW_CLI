#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${1:-$ROOT/artifacts/plw-linux-x86_64}"

if [[ ! -x "$BIN" ]]; then
  echo "binary not executable: $BIN" >&2
  exit 2
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/home" "$TMP/run" "$TMP/project"
cp "$BIN" "$TMP/run/plw"
chmod +x "$TMP/run/plw"

cat > "$TMP/project/a.ts" <<'EOF'
function target(x: number) { return x; }
class A {
  private wrap(x: number) { return target(x); }
}
EOF

cat > "$TMP/project/b.tsx" <<'EOF'
function flag(x: boolean) {
  if (x) {
    return true;
  }
  return false;
}
EOF

cat > "$TMP/project/c.js" <<'EOF'
function choose(x) {
  if (x) { return 1; }
  else { return 2; }
}
EOF

cat > "$TMP/project/d.jsx" <<'EOF'
function compute(x) { return x + 1; }
function value(x) {
  const result = compute(x);
  return result;
}
EOF

cat > "$TMP/project/e.rs" <<'EOF'
fn inner(x: i32) -> i32 { x + 1 }

fn wrap_return(x: i32) -> i32 { return inner(x); }

fn wrap_tail(x: i32) -> i32 { inner(x) }

struct S;
impl S {
    fn inner(&self, x: i32) -> i32 { x + 1 }
    fn wrap_self(&self, x: i32) -> i32 { self.inner(x) }
}

fn dup_a(x: i32) -> i32 {
    let y = x * 2;
    let z = y + 3;
    z
}
fn dup_b(x: i32) -> i32 {
    let y = x * 2;
    let z = y + 3;
    z
}

fn choose(x: bool) -> i32 {
    if x { return 1; } else { return 2; }
}

fn get_str<'a>(x: &'a str) -> &'a str { x }

fn raw_braces() -> &'static str { r#"{"a": 1}"# }

trait T { fn declared(&self); }

macro_rules! gen_fn {
    () => {
        fn macro_generated(x: i32) -> i32 {
            let y = x * 2;
            let z = y + 3;
            z
        }
    };
}

fn not_forward(x: i32) -> i32 { inner(x); 0 }

fn expr_if(x: bool) -> i32 {
    let y = if x { 1 } else { 2 };
    y
}

fn tail_if(x: bool) -> i32 {
    if x { return 1; } else { 2 }
}

fn chain_if(x: i32) -> i32 {
    if x == 1 { return 10; } else if x == 2 { return 20; } else { return 30; }
}

pub fn pub_wrap(x: i32) -> i32 { return inner(x); }
EOF

cat > "$TMP/project/f.kt" <<'EOF'
fun inner(x: Int): Int { return x + 1 }

private fun wrap_expr(x: Int): Int = inner(x)

private fun wrap_block(x: Int): Int { return inner(x) }

fun dup_a(x: Int): Int {
    val y = x * 2
    val z = y + 3
    return z
}
fun dup_b(x: Int): Int {
    val y = x * 2
    val z = y + 3
    return z
}

fun choose(x: Boolean): Int {
    if (x) { return 1 } else { return 2 }
}

fun raw_braces(): String {
    val s = """{"a": 1}"""
    return s
}

data class Point(val x: Int, val y: Int)

interface Foo {
    fun declared(x: Int): Int
}

fun not_forward(x: Int): Int { inner(x); return 0 }

fun expr_if(x: Boolean): Int {
    val y = if (x) 1 else 2
    return y
}

fun tail_if(x: Boolean): Int {
    if (x) { return 1 } else { 2 }
}

fun chain_if(x: Int): Int {
    if (x == 1) { return 10 } else if (x == 2) { return 20 } else { return 30 }
}

fun pub_wrap(x: Int): Int = inner(x)

fun trap_chain(): String = JSONObject()
    .put("a", 1)
    .toString()

fun destructure(pair: Pair<Int, Int>): Int {
    val (a, b) = pair
    return a + b
}
EOF

cd "$TMP/run"
export HOME="$TMP/home"
export XDG_STATE_HOME="$TMP/home/state"
export XDG_CACHE_HOME="$TMP/home/cache"

./plw simplify "$TMP/project" --json > "$TMP/simplify.json"

python - "$TMP/simplify.json" "$TMP/project" <<'PY'
import json
from pathlib import Path
import sys

payload = json.loads(Path(sys.argv[1]).read_text())
project = Path(sys.argv[2])

bridge = payload.get("canonical_anatomy_bridge", {})
scope = bridge.get("semantic_frontend_scope", {})
inventory = bridge.get("language_inventory", {})
counts = inventory.get("reference_source_counts", {})

if bridge.get("semantic_frontend") != "python_ast+js_structural_v2":
    raise SystemExit(f"unexpected frontend: {bridge.get('semantic_frontend')}")
if scope.get("ts_js_rule_count") != 7:
    raise SystemExit(f"unexpected TS/JS rule count: {scope.get('ts_js_rule_count')}")

for ext in (".ts", ".tsx", ".js", ".jsx"):
    if counts.get(ext) != 1:
        raise SystemExit(f"expected exactly one {ext} fixture, got {counts.get(ext)}")
    if inventory.get("semantic_rule_frontends", {}).get(ext) != "js_structural_v2":
        raise SystemExit(f"{ext} not bound to js_structural_v2")

if scope.get("rs_rule_count") != 3:
    raise SystemExit(f"unexpected Rust rule count: {scope.get('rs_rule_count')}")
if scope.get("kt_rule_count") != 3:
    raise SystemExit(f"unexpected Kotlin rule count: {scope.get('kt_rule_count')}")
for ext, frontend in ((".rs", "rs_structural_v1"), (".kt", "kt_structural_v1")):
    if counts.get(ext) != 1:
        raise SystemExit(f"expected exactly one {ext} fixture, got {counts.get(ext)}")
    if inventory.get("semantic_rule_frontends", {}).get(ext) != frontend:
        raise SystemExit(f"{ext} not bound to {frontend}")

kinds = {row.get("kind") for row in payload.get("candidates", [])}
required = {
    "js_direct_forwarding_wrapper",
    "js_boolean_guard_return",
    "js_redundant_else_after_terminal",
    "js_terminal_expression_temporary",
    "rs_direct_forwarding_wrapper",
    "rs_exact_function_body_duplication",
    "rs_redundant_else_after_terminal",
    "kt_direct_forwarding_wrapper",
    "kt_exact_function_body_duplication",
    "kt_redundant_else_after_terminal",
}
missing = required - kinds
if missing:
    raise SystemExit(f"missing expected JS/TS v2 candidate kinds: {sorted(missing)}")

# Trap names must never appear as a candidate member: macro template text,
# non-forwarding calls, expression-position ifs, tail-value else blocks,
# else-if chain inner ifs, body-less declarations, and multi-line builder
# chains truncated to a first-line call (``= JSONObject()`` + ``.put``).
forbidden_names = {
    "macro_generated", "not_forward", "expr_if", "tail_if", "chain_if",
    "declared", "get_str", "raw_braces", "destructure", "trap_chain",
}
seen_names = {
    member.get("name")
    for row in payload.get("candidates", [])
    for member in row.get("members", [])
}
leaked = forbidden_names & seen_names
if leaked:
    raise SystemExit(f"structural v1 trap leaked into candidates: {sorted(leaked)}")

if payload.get("invariants", {}).get("js_structural_frontend_v2_is_not_a_full_js_ts_parser") is not True:
    raise SystemExit("JS/TS v2 parser boundary invariant missing")
if payload.get("invariants", {}).get("rs_structural_frontend_v1_is_not_a_full_rust_parser") is not True:
    raise SystemExit("Rust v1 parser boundary invariant missing")
if payload.get("invariants", {}).get("kt_structural_frontend_v1_is_not_a_full_kotlin_parser") is not True:
    raise SystemExit("Kotlin v1 parser boundary invariant missing")

for forbidden in ("data/runtime", "data/codebase/cache"):
    if (project / forbidden).exists():
        raise SystemExit(f"target repository polluted by PLW runtime state: {forbidden}")

print("STANDALONE_SIMPLIFY_JS_TS_V2: PASS")
PY
