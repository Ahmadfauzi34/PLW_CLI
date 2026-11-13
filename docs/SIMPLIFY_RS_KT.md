# Rust / Kotlin simplification

PLW's Rust/Kotlin simplification frontend is `rs_structural_v1` (`.rs`) and
`kt_structural_v1` (`.kt`), sharing one structural implementation.

It is intentionally conservative. It produces mechanically witnessed
simplification candidates; it does not claim behavioral equivalence and it
does not patch source automatically.

## Active rule families

| Rule | Candidate kind | Purpose |
| --- | --- | --- |
| Direct forwarding wrapper | `rs_direct_forwarding_wrapper`, `kt_direct_forwarding_wrapper` | Detect exact parameter forwarding to a direct identifier callee (plus `self.<name>` in Rust) |
| Exact function-body duplication | `rs_exact_function_body_duplication`, `kt_exact_function_body_duplication` | Detect conservative exact structural body duplication |
| Redundant else after terminal | `rs_redundant_else_after_terminal`, `kt_redundant_else_after_terminal` | Detect a braced `else` following a directly terminal branch, in statement position only |

For V1, the rule count per language is:

```text
3
```

## How it works

The frontend reuses the JS/TS frontend's approach, not a full parser: a
length-preserving mask hides comments and string/char literals, then brace
matching finds function bodies and token comparison finds duplication.

Language-specific masking:

- Rust: `//` and `/* */` comments, `"..."` strings, char literals, and raw
  strings (`r"..."` / `r#"..."#` / `br#"..."#`). Lifetimes (`&'a str`) are
  NOT char literals: a quote only starts a char literal when a closing
  quote follows one character or one escape sequence.
- Kotlin: `//` and `/* */` comments, `"..."` strings, char literals, and
  triple-quoted raw strings (`"""..."""`).

Function discovery:

- Rust: `fn` declarations with `pub` / `async` / `unsafe` / `const` /
  `extern "..."` modifiers, including methods in `impl` blocks. Trait
  methods and `extern` items without a body (`;` instead of `{`) are
  skipped. Functions inside `macro_rules!` definitions are excluded:
  their bodies are template text, not top-level items.
- Kotlin: `fun` declarations with visibility and modifier prefixes,
  including methods and extension functions (`Receiver.name`). Two body
  forms: block bodies (`{ ... }`) and single-line `= expr` bodies
  (forwarding only). Interface members and `external fun` without a body
  are skipped.

Forwarding shapes accepted:

- Rust block body: `return callee(args);` or a bare tail-expression call
  `callee(args)`. A call followed by `;` without `return` discards its
  value and is NOT forwarding.
- Kotlin block body: `return callee(args);`.
- Kotlin expression body: `= callee(args)` on a single line. When the
  expression continues on the next line (a leading `.` member chain or
  infix operator), the body is multi-line and is NOT resolved as
  forwarding in v1 — a first-line truncation would misread builder
  chains such as `= JSONObject()` + `.put(...)` as a forwarding call.
- Callees are plain identifiers, plus `self.<name>` in Rust (method
  delegation). `Self::`, `super::`, and `crate::` paths are deferred.

Redundant-else guards (beyond the JS rule, because Rust/Kotlin `if` can be
an expression):

- the `if` must be in statement position (the previous non-space character
  is not `= ( [ ,`, it is not preceded by `return`, and it is not an
  `else if` chain member);
- the `else` block must not end in a bare tail expression, which would be
  the value of an expression-position `if`.

## Safety boundaries

The frontend deliberately defers or rejects cases where its structural
witness is insufficient. Important examples include:

- exported/public forwarding wrappers without compatibility proof;
- async forwarding wrappers;
- destructured/default parameter forwarding (Kotlin destructuring
  `(a, b): Pair` yields no simple parameter list);
- `Self::` / `super::` / `crate::` callee paths;
- multi-line Kotlin `= expr` bodies (only single-line expressions are
  resolved in v1; a continuation line starting with `.` or an infix
  operator excludes the record from forwarding — found live: 7
  `toJson`/`json` builder chains in oopbot-trial were first-line
  false positives and are now excluded);
- `panic!` as a terminal statement (only `return`, plus `throw` in Kotlin,
  counts as terminal in v1);
- nested block comments containing braces (only the first `*/` closes the
  mask — same limitation class as the JS mask);
- braces inside Kotlin string templates (`${...}` is masked as a whole);
- test-neighbor risk counting stays Python-centric in v1
  (`_proof_surface` recognizes `tests/` and `*_test.py`, not
  `*Test.kt` / `src/test/`).

A candidate means **"worth validating"**, not **"safe to edit"**.

## CLI

JSON output:

```bash
plw simplify /path/to/project --json
```

Useful fields:

```text
canonical_anatomy_bridge.semantic_frontend
canonical_anatomy_bridge.semantic_frontend_scope.rs_rule_count
canonical_anatomy_bridge.semantic_frontend_scope.kt_rule_count
canonical_anatomy_bridge.language_inventory
candidates[]
frontier
invariants
```

The `semantic_frontend` marker string is retained from the JS/TS era
(`python_ast+js_structural_v2`) for overlay-chain compatibility; the
`semantic_frontend_scope` object is the source of truth for per-language
coverage.

## Discovery contract fix

The Rust/Kotlin frontends read through the shared graph
(`core.shared_graph`), whose experimental/self-host extension set used to be
JS + Python only: `EXPERIMENTAL_SOURCE_EXTENSIONS` was defined before K5/R1
expanded the default contract to Kotlin/Rust, so a scan through the
experimental path silently dropped `.kt`/`.rs` files and the new frontends
saw zero files. The fix (same overlay) makes the experimental set the
default contract plus Python:
`EXPERIMENTAL_SOURCE_EXTENSIONS = SOURCE_EXTENSIONS + PYTHON_SOURCE_EXTENSIONS`.
The default path is unchanged; the only production caller of the
experimental path is the simplification analyzer.

## Authority boundary

`plw simplify` is a candidate generator.

It does not:

- mutate source;
- prove complete behavioral equivalence;
- prove an exported API can be removed;
- replace project tests/type-checks;
- treat zero candidates as universal code cleanliness.

Apply a candidate only after target-specific validation.
