#!/usr/bin/env bash
# Smoke: Phase 4 check_signal_hardening_v1 (line preservation + noise calibration for hott.homotopy, perf.async, perf.gc, perf.cache).
#
# Verifies all mandatory invariants and negative controls:
#   1. strip_js_comments preserves exact newline counts for multiline block comments,
#      preserves token separation on inline /*...*/ comments, and handles /* https://... */
#      without letting // inside /*...*/ swallow the closing */.
#   2. perf.async await_in_loop reports exact post-comment line numbers and downgrades
#      bounded retry loops to severity="info" (bounded_retry_guard=True), while leaving
#      unbounded for(;;) and .length collection loops as severity="high".
#   3. perf.async sync_io downgrades CLI/tooling scripts (package_script/package_bin/
#      cli_shebang/tool_config_entry, #!/usr/bin/env, /scripts/, /bin/) to severity="info"
#      with execution_context="cli_or_tooling_script", while keeping server sync_io="high".
#   4. perf.gc unclosed_event_listener ignores multi-line { once: true }, { signal: ... },
#      and { signal } shorthand listeners while still flagging { passive: true } at exact line.
#   5. perf.gc / perf.cache ignore non-escaping function-local Map/Set containers (even when
#      returning `localCache.get(k)`), still flag module-level / escaped / TS-generic class
#      field containers, suppress unbounded_cache and downgrade missing_ttl to "low" with
#      eviction_guard=True when bounded by .size + .delete/.clear.
#   6. hott.homotopy diamond_dependency keeps pure constants, as const, readonly, immutable
#      builtins (RegExp/URL), and commented exports at severity="info", while escalating
#      export let / var and stateful `new Monitor()` singletons to severity="medium".
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

python3 - "$ROOT" <<'PY'
import sys

sys.path.insert(0, sys.argv[1])

from language_semantics import strip_js_comments
from codebase.hott_analyzers import analyze_homotopy
from codebase.performance_analyzers import (
    analyze_async_waterfall,
    analyze_cache,
    analyze_gc_pressure,
)

# 1. Comment stripping & line-number integrity
src_comments = (
    "/**\n"
    " * Multi-line JSDoc\n"
    " * @see https://example.com\n"
    " */\n"
    "const a = /* inline */ 1;\n"
    "const x = /* https://example.com/a */ 2;\n"
    "// /* do not start block\n"
    "const y = 3;\n"
)
stripped = strip_js_comments(src_comments)
assert stripped.count("\n") == src_comments.count("\n"), (stripped.count("\n"), src_comments.count("\n"))
stripped_lines = stripped.splitlines()
assert "const a =   1;" in stripped_lines[4], stripped_lines
assert "const x =   2;" in stripped_lines[5], stripped_lines
assert "const y = 3;" in stripped_lines[7], stripped_lines

# 2. perf.async: await_in_loop (exact lines + bounded retry vs unbounded) & sync_io (CLI vs server)
retry_bounded = """/**
 * Header comment
 * spans three lines
 */
async function withLock() {
  for (let attempt = 0; ; attempt++) {
    try {
      await takeLock();
      break;
    } catch (e) {
      if (attempt === 2) throw e;
    }
    await sleepRetry();
  }
}
"""

retry_small_try = """async function pollSmall() {
  for (let i = 0; i < 3; i++) {
    try {
      return await fetchOnce();
    } catch {}
  }
}
"""

unbounded_forever = """async function lockForever() {
  for (;;) {
    await sleepForever();
  }
}
"""

collection_retry_name = """async function badLoopName(items: string[]) {
  for (let attempt = 0; attempt < items.length; attempt++) {
    await work(items[attempt]);
  }
}
"""

cli_pkg_script = """import { readFileSync } from "node:fs";
export function runTool() { return readFileSync("in.json", "utf8"); }
"""

server_sync = """import { readFileSync } from "node:fs";
export function handleReq() { return readFileSync("cfg.json", "utf8"); }
"""

async_res = analyze_async_waterfall({
    "file_map": {
        "src/config-lock.ts": retry_bounded,
        "src/small-try.ts": retry_small_try,
        "src/install-lock.ts": unbounded_forever,
        "src/items-loop.ts": collection_retry_name,
        "math compesor/helper/bin/tool.mjs": cli_pkg_script,
        "scripts/audit.mjs": cli_pkg_script,
        "src/server.ts": server_sync,
    },
    "node_metadata": {
        "math compesor/helper/bin/tool.mjs": {"is_entrypoint": True, "entrypoint_kind": "package_script"},
    },
})

by_file_async = {}
for f in async_res["findings"]:
    by_file_async.setdefault(f["file"], []).append(f)

cfg_rows = by_file_async["src/config-lock.ts"]
assert [r["line"] for r in cfg_rows] == [8, 13], cfg_rows
assert all(r["severity"] == "info" and r.get("bounded_retry_guard") is True for r in cfg_rows), cfg_rows

assert len(by_file_async["src/small-try.ts"]) == 1
assert by_file_async["src/small-try.ts"][0]["severity"] == "info"

assert by_file_async["src/install-lock.ts"][0]["severity"] == "high"
assert not by_file_async["src/install-lock.ts"][0].get("bounded_retry_guard")

assert by_file_async["src/items-loop.ts"][0]["severity"] == "high"

for cli_f in ("math compesor/helper/bin/tool.mjs", "scripts/audit.mjs"):
    rows = by_file_async[cli_f]
    assert len(rows) == 1 and rows[0]["severity"] == "info", rows
    assert rows[0].get("execution_context") == "cli_or_tooling_script", rows

assert by_file_async["src/server.ts"][0]["severity"] == "high"
assert "execution_context" not in by_file_async["src/server.ts"][0]

# 3. perf.gc + perf.cache: listeners + local non-escaping containers vs escaping/global + LRU eviction
listeners_ok = """function setup(sig: AbortSignal) {
  window.addEventListener(
    "load",
    () => console.log("ok"),
    { once: true },
  );
  sig?.addEventListener("abort", () => cleanup(), { signal: sig });
  document.addEventListener("close", onClose, { signal });
}
"""

listeners_mixed = """/**
 * Header comment
 */
function setupMixed() {
  window.addEventListener("load", onLoad, { once: true });
  document.addEventListener("scroll", onScroll, { passive: true });
}
"""

local_non_escaping = """export function parseTree(items: string[], id: string) {
  const parsedCache = new Map<string, number>();
  for (const item of items) {
    parsedCache.set(item, item.length);
  }
  return parsedCache.get(id) ?? null;
}
"""

local_escaping_snapshot = """export function buildSnapshot(events: any[]) {
  const restoresById = new Map();
  for (const e of events) {
    restoresById.set(e.id, e);
  }
  return { events, restoresById };
}
"""

class_field_ts_generic = """export class PolicyKernel {
  private readonly npcRegistry = new Map<string, string>();
  register(k: string, v: string) { this.npcRegistry.set(k, v); }
}
"""

lru_bounded_cache = """const _embedCache = new Map<string, number[]>();
const MAX_CACHE = 256;
export function setCached(k: string, v: number[]) {
  if (_embedCache.size >= MAX_CACHE) {
    const oldest = _embedCache.keys().next().value;
    if (oldest !== undefined) _embedCache.delete(oldest);
  }
  _embedCache.set(k, v);
}
"""

unbounded_module_cache = """const globalCache = new Map<string, string>();
export function putVal(k: string, v: string) {
  globalCache.set(k, v);
}
"""

gc_res = analyze_gc_pressure({
    "file_map": {
        "listeners_ok.ts": listeners_ok,
        "listeners_mixed.ts": listeners_mixed,
        "local_ok.ts": local_non_escaping,
        "escaped_store.mjs": local_escaping_snapshot,
        "kernel_registry.ts": class_field_ts_generic,
    }
})
gc_by_file = {}
for f in gc_res["findings"]:
    gc_by_file.setdefault(f["file"], []).append(f)

assert "listeners_ok.ts" not in gc_by_file, gc_by_file
assert "local_ok.ts" not in gc_by_file, gc_by_file
assert len(gc_by_file["listeners_mixed.ts"]) == 1
assert gc_by_file["listeners_mixed.ts"][0]["line"] == 6, gc_by_file["listeners_mixed.ts"]
assert gc_by_file["escaped_store.mjs"][0]["type"] == "unbounded_collection"
assert gc_by_file["kernel_registry.ts"][0]["type"] == "unbounded_collection"

cache_res = analyze_cache({
    "file_map": {
        "local_ok.ts": local_non_escaping,
        "lru_ok.ts": lru_bounded_cache,
        "unbounded_cache.ts": unbounded_module_cache,
    }
})
cache_by_file = {}
for f in cache_res["findings"]:
    cache_by_file.setdefault(f["file"], []).append(f)

assert "local_ok.ts" not in cache_by_file, cache_by_file
assert len(cache_by_file["lru_ok.ts"]) == 1, cache_by_file["lru_ok.ts"]
assert cache_by_file["lru_ok.ts"][0]["type"] == "missing_ttl"
assert cache_by_file["lru_ok.ts"][0]["severity"] == "low"
assert cache_by_file["lru_ok.ts"][0].get("eviction_guard") is True

ub_types = {(f["type"], f["severity"]) for f in cache_by_file["unbounded_cache.ts"]}
assert ub_types == {("missing_ttl", "medium"), ("unbounded_cache", "medium")}, ub_types

# 4. hott.homotopy diamond_dependency escalation precision
hom_res = analyze_homotopy({
    "vertices": ["a.ts", "b.ts", "c.ts", "const_conv.ts", "commented_conv.ts", "mutable_conv.ts"],
    "edges": [
        ["a.ts", "b.ts"], ["a.ts", "c.ts"],
        ["b.ts", "const_conv.ts"], ["c.ts", "const_conv.ts"],
        ["b.ts", "commented_conv.ts"], ["c.ts", "commented_conv.ts"],
        ["b.ts", "mutable_conv.ts"], ["c.ts", "mutable_conv.ts"],
    ],
    "file_map": {
        "a.ts": "import './b'; import './c';",
        "b.ts": "export const x = 1;",
        "c.ts": "export const y = 2;",
        "const_conv.ts": (
            'export const LEVELS = ["a", "b"] as const;\n'
            'export const FIELDS: readonly string[] = ["id"];\n'
            'export const SYSTEM_RULES = { mode: "strict" };\n'
            'export const URL_RE = new RegExp("^https://");\n'
            'export const BASE_URL = new URL("https://example.com");\n'
        ),
        "commented_conv.ts": (
            '// export let disabledCounter = 0;\n'
            '/* export const commentedMonitor = new Monitor(); */\n'
            'export const OK = 1;\n'
        ),
        "mutable_conv.ts": "export const activityMonitor = new ActivityMonitor();\n",
    },
})

sev_by_conv = {
    f["convergence"]: f["severity"]
    for f in hom_res["findings"]
    if f["type"] == "diamond_dependency"
}
assert sev_by_conv["const_conv.ts"] == "info", sev_by_conv
assert sev_by_conv["commented_conv.ts"] == "info", sev_by_conv
assert sev_by_conv["mutable_conv.ts"] == "medium", sev_by_conv

print("SMOKE_CHECK_SIGNAL_HARDENING: PASS")
PY
