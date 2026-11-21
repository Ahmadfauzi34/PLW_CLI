// JNI name mangling, signature parsing, and coarse type compatibility
// helpers for the Kotlin<->Rust --related bridge (split from related.go).

package main

import (
	"fmt"
	"strings"
)

// --- JNI bridge (go14, P2: Option 3, Base-Key Grouping) ---
//
// The JNI name mangling is per the JNI spec (docs.oracle.com, "Design
// Overview"): a native function is named
// Java_<pkg>_<Class>_<method>[__<mangled signature>], with _ -> _1,
// / -> _, ; -> _2, [ -> _3, and any other non-alphanumeric rune ->
// _0wxyz (lowercase hex). Because _ escapes to _1, the literal sequence
// __ never occurs inside the Java_<pkg>_<Class>_<method> prefix —
// splitting on the FIRST __ is exact, and the long (overloaded) form
// carries the full argument signature after it.

// jniEscape escapes one JNI name segment (a package segment, a class
// name, or a method name) per the spec. Alphanumerics are unchanged;
// _ -> _1, $ -> _00024 (inner-class separator); any other rune ->
// _0xxxx. The / -> _ rule never applies here: package separators are
// handled by joining escaped segments with _.
func jniEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '_':
			b.WriteString("_1")
		case r == '$':
			b.WriteString("_00024")
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "_0%04x", r)
		}
	}
	return b.String()
}

// jniKotlinBase builds the JNI base key for a Kotlin external fun:
// Java_<escaped pkg segments joined by _>_<escaped class chain>_<escaped
// method>. classChain is outermost-first (["Bridge"] or
// ["Outer", "Inner"]); the chain is joined with $ (the JVM binary
// name) before escaping, so Outer$Inner becomes Outer_00024Inner. An
// empty chain means top-level — the caller supplies the file facade
// <FileStem>Kt as the single element.
func jniKotlinBase(pkg string, classChain []string, method string) string {
	var parts []string
	if pkg != "" {
		for _, seg := range strings.Split(pkg, ".") {
			parts = append(parts, jniEscape(seg))
		}
	}
	parts = append(parts, jniEscape(strings.Join(classChain, "$")))
	parts = append(parts, jniEscape(method))
	return "Java_" + strings.Join(parts, "_")
}

// parseJNIRustName splits a Rust JNI function name on the FIRST __
// (the overload separator). Returns the base, the parsed signature
// tokens (nil for a short name), and whether the name is an overloaded
// (long) name. ok=false when the name is not a JNI name or its
// signature is malformed — the caller treats that as "not a JNI
// bridge" and skips it (fail-closed for matching, silent for indexing).
func parseJNIRustName(name string) (base string, tokens []string, overloaded, ok bool) {
	if !strings.HasPrefix(name, "Java_") {
		return "", nil, false, false
	}
	base, raw, cut := strings.Cut(name, "__")
	if !cut {
		return name, nil, false, true
	}
	tokens, ok = parseJNISig(raw)
	if !ok {
		return "", nil, false, false
	}
	return base, tokens, true, true
}

// parseJNISig parses a mangled JNI argument signature (after __) into
// one token per parameter. Exact, no classpath: _3 prefixes
// (repeatable) are array dimensions, primitives (B C D F I J S Z) are
// one token each, objects run L..._2. Returns ok=false on malformed
// input (dangling array prefix, unterminated object, unknown letter).
// An empty signature parses to zero tokens (a zero-arity function).
func parseJNISig(sig string) ([]string, bool) {
	var tokens []string
	for len(sig) > 0 {
		rest := sig
		for strings.HasPrefix(sig, "_3") {
			sig = sig[2:]
		}
		if len(sig) == 0 {
			return nil, false // dangling array prefix
		}
		switch sig[0] {
		case 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z':
			sig = sig[1:]
		case 'L':
			end := strings.Index(sig, "_2")
			if end < 0 {
				return nil, false
			}
			sig = sig[end+2:]
		default:
			return nil, false
		}
		tokens = append(tokens, rest[:len(rest)-len(sig)])
	}
	return tokens, true
}

// jniCoarseToken maps a Kotlin parameter type to its JNI signature
// token when that mapping is certain without a classpath, else "".
// Primitives map 1:1; String and primitive arrays have fixed JVM
// forms; nullable types, generics, and custom classes are wildcards
// ("" — the JVM type of Int? is Ljava/lang/Integer;, of Map<..> is
// Ljava/util/Map;, neither knowable from the source text alone). A
// vararg parameter is an array parameter in the JVM signature.
func jniCoarseToken(kotlinType string, vararg bool) string {
	t := strings.TrimSpace(kotlinType)
	if strings.HasSuffix(t, "?") {
		return "" // nullable: boxed — classpath knowledge
	}
	var tok string
	switch t {
	case "Byte":
		tok = "B"
	case "Char":
		tok = "C"
	case "Double":
		tok = "D"
	case "Float":
		tok = "F"
	case "Int":
		tok = "I"
	case "Long":
		tok = "J"
	case "Short":
		tok = "S"
	case "Boolean":
		tok = "Z"
	case "String":
		tok = "Ljava_lang_String_2"
	case "ByteArray":
		tok = "_3B"
	case "CharArray":
		tok = "_3C"
	case "DoubleArray":
		tok = "_3D"
	case "FloatArray":
		tok = "_3F"
	case "IntArray":
		tok = "_3I"
	case "LongArray":
		tok = "_3J"
	case "ShortArray":
		tok = "_3S"
	case "BooleanArray":
		tok = "_3Z"
	default:
		return ""
	}
	if vararg {
		return "_3" + tok
	}
	return tok
}

// jniCoarseCompatible reports whether a Kotlin decl's coarse tokens
// are compatible with a Rust JNI signature's tokens: same length, and
// every non-wildcard ("") Kotlin token equals the Rust token at the
// same position. A wildcard never vetoes — the source text cannot
// prove that parameter's JVM type.
func jniCoarseCompatible(coarse, rustTokens []string) bool {
	if len(coarse) != len(rustTokens) {
		return false
	}
	for i := range coarse {
		if coarse[i] != "" && coarse[i] != rustTokens[i] {
			return false
		}
	}
	return true
}
