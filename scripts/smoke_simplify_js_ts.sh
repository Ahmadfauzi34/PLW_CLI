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

cat > "$TMP/project/g.mjs" <<'EOF'
function targetMjs(x) { return x; }
function runMjs(fn) { return Promise.resolve().then(fn); }

function wrap_mjs(x) {
  return targetMjs(x);
}

function trap_try_if(u) {
  try {
    if (!u) return null;
  } catch {
    return null;
  }
  return u;
}

function trap_lit_arg_fwd(x) {
  return targetMjs("lit", x);
}

function trap_lit_zero(x) {
  return targetMjs("lit");
}

function trap_else_if(a, b) {
  if (a === 1) {
    return 1;
  } else if (b === 2) {
    return 2;
  } else {
    return 3;
  }
}

function trap_nested_arrow_else(a) {
  if (a) {
    runMjs().catch(() => {
      return 1;
    });
  } else {
    return 2;
  }
  return 0;
}

function trap_braceless_else(a, b) {
  if (a) {
    if (b) return 1;
  } else {
    return 2;
  }
  return 0;
}
EOF

cat > "$TMP/project/h.mts" <<'EOF'
export function calcMtsA(x: number): number {
  const a = x + 10;
  const b = a * 2;
  return b - 3;
}
export function calcMtsB(x: number): number {
  const a = x + 10;
  const b = a * 2;
  return b - 3;
}
export function trap_long_ws(x: number): number {
  /* """                                                                                                                                                                                                                                                                  */
  const longComment =
    /*                                                                                                                                                                                                                                                                     */
    x + 1;
  return longComment + 2;
}
EOF

cat > "$TMP/project/types.d.mts" <<'EOF'
export declare function dts_trap(x: number): number;
EOF

cat > "$TMP/project/gen_glue.js" <<'EOF'
/* @ts-self-types="./gen_glue.d.ts" */
export function gen_trap(x) {
  if (x) {
    return true;
  }
  return false;
}
EOF

mkdir -p "$TMP/project/public/assets" "$TMP/project/data" "$TMP/project/src/test/kotlin"
cat > "$TMP/project/public/assets/vendor.js" <<'EOF'
export function asset_trap(x) {
  if (x) {
    return true;
  }
  return false;
}
EOF

cat > "$TMP/project/f.kt" <<'EOF'
fun inner(x: Int): Int { return x + 1 }

data class ChatKey(val value: String)

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

override fun trap_override(x: Int): Int = inner(x)

@Test
fun trap_test_fn(x: Int): Int = inner(x)

fun trap_ctor(path: String): ChatKey = ChatKey(path)

fun trap_lit_kt(x: Int): Int = inner("k", x)

fun trap_elvis_else(x: Int?): Int {
    if (x != null) {
        val y = x ?: return 0
    } else {
        return 1
    }
    return 2
}
EOF

cat > "$TMP/project/data/DataRepo.kt" <<'EOF'
fun data_helper(x: Int): Int = x + 10
private fun data_wrap(x: Int): Int = data_helper(x)
EOF

cat > "$TMP/project/src/test/kotlin/WrapTest.kt" <<'EOF'
fun trap_test_dir(x: Int): Int = inner(x)
EOF

cd "$TMP/run"
export HOME="$TMP/home"
export XDG_STATE_HOME="$TMP/home/state"
export XDG_CACHE_HOME="$TMP/home/cache"

./plw simplify "$TMP/project" --json > "$TMP/simplify.json"

python3 - "$TMP/simplify.json" "$TMP/project" <<'PY'
import json
from pathlib import Path
import sys

payload = json.loads(Path(sys.argv[1]).read_text())
project = Path(sys.argv[2])

bridge = payload.get("canonical_anatomy_bridge", {})
scope = bridge.get("semantic_frontend_scope", {})
inventory = bridge.get("language_inventory", {})
counts = inventory.get("reference_source_counts", {})
metrics = payload.get("metrics", {})

if bridge.get("semantic_frontend") != "python_ast+js_structural_v2":
    raise SystemExit(f"unexpected frontend: {bridge.get('semantic_frontend')}")
if scope.get("ts_js_rule_count") != 7:
    raise SystemExit(f"unexpected TS/JS rule count: {scope.get('ts_js_rule_count')}")

expected_counts = {
    ".ts": 1, ".tsx": 1, ".js": 3, ".jsx": 1,
    ".mjs": 1, ".mts": 1, ".rs": 1, ".kt": 3,
}
for ext, expected in expected_counts.items():
    if counts.get(ext) != expected:
        raise SystemExit(f"expected {expected} {ext} fixtures, got {counts.get(ext)}")
    expected_frontend = (
        "rs_structural_v1" if ext == ".rs"
        else "kt_structural_v1" if ext == ".kt"
        else "js_structural_v2"
    )
    if inventory.get("semantic_rule_frontends", {}).get(ext) != expected_frontend:
        raise SystemExit(f"{ext} not bound to {expected_frontend}")

if int(metrics.get("generated_files_excluded", 0)) != 2:
    raise SystemExit(f"expected generated_files_excluded == 2, got {metrics.get('generated_files_excluded')}")

if scope.get("rs_rule_count") != 3:
    raise SystemExit(f"unexpected Rust rule count: {scope.get('rs_rule_count')}")
if scope.get("kt_rule_count") != 3:
    raise SystemExit(f"unexpected Kotlin rule count: {scope.get('kt_rule_count')}")

kinds = {row.get("kind") for row in payload.get("candidates", [])}
required = {
    "js_direct_forwarding_wrapper",
    "js_exact_function_body_duplication",
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

# Positive regression checks: .mjs and data/*.kt must be in scope.
seen_names = {
    member.get("name")
    for row in payload.get("candidates", [])
    for member in row.get("members", [])
}
for expected_name in ("wrap_mjs", "calcMtsA", "data_wrap"):
    if expected_name not in seen_names:
        raise SystemExit(f"expected candidate missing from new scope: {expected_name}")

# Trap names must never appear as a candidate member.
forbidden_names = {
    "macro_generated", "not_forward", "expr_if", "tail_if", "chain_if",
    "declared", "get_str", "raw_braces", "destructure", "trap_chain",
    "dts_trap", "gen_trap", "asset_trap",
    "trap_try_if", "trap_lit_arg_fwd", "trap_lit_zero",
    "trap_else_if", "trap_nested_arrow_else", "trap_braceless_else",
    "trap_override", "trap_test_fn", "trap_ctor", "trap_lit_kt",
    "trap_elvis_else", "trap_test_dir",
}
leaked = forbidden_names & seen_names
if leaked:
    raise SystemExit(f"structural precision trap leaked into candidates: {sorted(leaked)}")

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
