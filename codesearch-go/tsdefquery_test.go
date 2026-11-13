package main

// Locks the TS-only shape of tsDefQuery (callers.go): interface
// members are method_signature nodes, which exist in
// tree-sitter-typescript but NOT in tree-sitter-javascript — so the
// pattern must live in tsDefQuery, never in the shared jsDefQuery
// (a JS compile failure here is the guard, not a bug).

import (
	"reflect"
	"sort"
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const tsDefQuerySrc = `interface Shape {
  ifaceMethod(): void;
  name: string;
}
type Obj = {
  objMethod(): void;
};
class C {
  classMethod(): void {}
}
`

func matchDefNames(t *testing.T, q *gotreesitter.Query, lang *gotreesitter.Language, src string) []string {
	t.Helper()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	s := []byte(src)
	cursor := q.Exec(tree.RootNode(), lang, s)
	var names []string
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			if c.Name == "name" {
				names = append(names, string(s[c.Node.StartByte():c.Node.EndByte()]))
			}
		}
	}
	sort.Strings(names)
	return names
}

func TestTSDefQueryMethodSignature(t *testing.T) {
	// tsDefQuery must compile for TypeScript and TSX and match the
	// interface member, the object-type member, the class method,
	// and the class itself — one name set, no duplicates.
	for _, tc := range []struct {
		label string
		lang  func() *gotreesitter.Language
	}{
		{"typescript", grammars.TypescriptLanguage},
		{"tsx", grammars.TsxLanguage},
	} {
		lang := tc.lang()
		q, err := gotreesitter.NewQuery(tsDefQuery, lang)
		if err != nil {
			t.Fatalf("%s: tsDefQuery failed to compile: %v", tc.label, err)
		}
		want := []string{"C", "classMethod", "ifaceMethod", "objMethod"}
		if got := matchDefNames(t, q, lang, tsDefQuerySrc); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: def names = %v, want %v", tc.label, got, want)
		}
	}

	// The shared jsDefQuery must keep compiling for JavaScript (the
	// method_signature pattern must not leak into it), and tsDefQuery
	// must NOT compile there — method_signature is a TS-only node.
	jsLang := grammars.JavascriptLanguage()
	if _, err := gotreesitter.NewQuery(jsDefQuery, jsLang); err != nil {
		t.Fatalf("jsDefQuery failed to compile for JavaScript: %v", err)
	}
	if _, err := gotreesitter.NewQuery(tsDefQuery, jsLang); err == nil {
		t.Errorf("tsDefQuery compiled for JavaScript; method_signature must stay TS-only")
	}
}
