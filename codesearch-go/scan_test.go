package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitCamel(t *testing.T) {
	cases := map[string]string{
		"getGraphParametersFlat": "get Graph Parameters Flat",
		"HTMLParser":             "HTML Parser",
		"getGraph":               "get Graph",
		"snake_case":             "snake_case",
		"ABC":                    "ABC",
		"a1B2":                   "a1 B2",
	}
	for in, want := range cases {
		if got := splitCamel(in); got != want {
			t.Errorf("splitCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTokenize(t *testing.T) {
	if got := tokenize("foo_bar-baz qux"); !reflect.DeepEqual(got, []string{"foo", "bar", "baz", "qux"}) {
		t.Errorf("tokenize = %v", got)
	}
	if got := tokenize("ABC123"); !reflect.DeepEqual(got, []string{"abc123"}) {
		t.Errorf("tokenize = %v", got)
	}
	if got := tokenize("***"); len(got) != 0 {
		t.Errorf("tokenize(***) = %v, want empty", got)
	}
}

func TestChunkLines(t *testing.T) {
	mk := func(n int) string {
		s := ""
		for i := 0; i < n; i++ {
			if i > 0 {
				s += "\n"
			}
			s += "l"
		}
		return s
	}
	if cs := chunkLines(mk(10)); len(cs) != 1 || cs[0].startLine != 1 {
		t.Errorf("short file: %+v", cs)
	}
	cs := chunkLines(mk(100))
	if len(cs) == 0 || cs[0].startLine != 1 || cs[1].startLine != 16 {
		t.Fatalf("chunk starts wrong: %+v", cs[:2])
	}
	// 15-aligned starts: ..., 226, 241 for a 250-line marker region
	cs = chunkLines(mk(300))
	var starts []int
	for _, c := range cs {
		starts = append(starts, c.startLine)
	}
	for _, want := range []int{226, 241} {
		found := false
		for _, s := range starts {
			if s == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing chunk start %d in %v", want, starts)
		}
	}
	// last chunk ends exactly at n
	last := cs[len(cs)-1]
	if last.startLine+len(last.lines)-1 != 300 {
		t.Errorf("last chunk doesn't end at 300: %+v", last.startLine)
	}
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pat, s string
		want   bool
	}{
		{"*.rs", "src/a.rs", true}, // * crosses '/' like fnmatch
		{"*.rs", "src/a.py", false},
		{"noise.rs", "src/noise.rs", false}, // full-string match
		{"*noise.rs", "src/noise.rs", true},
		{"src/*.rs", "src/a.rs", true},
		{"a?c", "abc", true},
		{"[abc].rs", "a.rs", true},
		{"[!a].rs", "b.rs", true},
		{"[!a].rs", "a.rs", false},
	}
	for _, c := range cases {
		if got := globMatch(c.pat, c.s); got != c.want {
			t.Errorf("globMatch(%q,%q) = %v, want %v", c.pat, c.s, got, c.want)
		}
	}
}

func TestGitignored(t *testing.T) {
	pats := loadGitignoreFromText("noise.rs\n*.log\n/build/\n!important.log\n")
	cases := []struct {
		rel  string
		want bool
	}{
		{"src/noise.rs", true},
		{"src/app.rs", false},
		{"a.log", true},
		{"build/out", true},
		{"src/build/out", true},  // dir part match
		{"important.log", false}, // negated
	}
	for _, c := range cases {
		if got := gitignored(c.rel, pats); got != c.want {
			t.Errorf("gitignored(%q) = %v, want %v", c.rel, got, c.want)
		}
	}
}

// loadGitignoreFromText is a test helper: parse gitignore content directly.
func loadGitignoreFromText(content string) []giPattern {
	var pats []giPattern
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		neg := strings.HasPrefix(line, "!")
		if neg {
			line = strings.TrimSpace(line[1:])
		}
		isDir := strings.HasSuffix(line, "/")
		pat := strings.Trim(line, "/")
		if pat != "" {
			pats = append(pats, giPattern{neg, isDir, pat, compileGlob(pat), compileGlob(pat)})
		}
	}
	return pats
}
