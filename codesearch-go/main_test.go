package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestIsEmptyQuery(t *testing.T) {
	// Regression 2026-10-26: a single empty query must be rejected,
	// not routed to literal search where "" matches every chunk.
	for _, q := range []string{"", "   ", "\t\n", `""`, `"   "`} {
		if !isEmptyQuery(q) {
			t.Errorf("isEmptyQuery(%q) = false, want true", q)
		}
	}
	for _, q := range []string{"main", "foo bar", "---", "100%", `"exact phrase"`} {
		if isEmptyQuery(q) {
			t.Errorf("isEmptyQuery(%q) = true, want false", q)
		}
	}
}

// Batch 2-6 (2026-10-26): --version must report the true schema always,
// and build fields honestly — "unknown" when built without ldflags, the
// injected values when built with them. A stale value would be worse
// than "unknown": it would misidentify the binary.
func TestVersionLine(t *testing.T) {
	oldCommit, oldDate := buildCommit, buildDate
	defer func() { buildCommit, buildDate = oldCommit, oldDate }()

	buildCommit, buildDate = "unknown", "unknown"
	line := versionLine()
	if !strings.Contains(line, "schema=go9") {
		t.Errorf("versionLine default = %q, want schema=go9", line)
	}
	if !strings.Contains(line, "commit=unknown") || !strings.Contains(line, "built=unknown") {
		t.Errorf("versionLine default = %q, want commit=unknown built=unknown", line)
	}
	buildCommit, buildDate = "abc123def456", "2026-10-26T00:00:00Z"
	line = versionLine()
	if !strings.Contains(line, "commit=abc123def456") || !strings.Contains(line, "built=2026-10-26T00:00:00Z") {
		t.Errorf("versionLine injected = %q, want injected values", line)
	}
}

// Tahap 1 maintainability (2026-10-31): parseArgs previously had no unit
// coverage at all — its only net was the black-box suite. These table
// tests pin the parser's contract (defaults, --flag val vs --flag=val,
// missing values, invalid numbers, unknown flags) so a future refactor
// of the CLI layer fails in go test, not only in black-box.
func TestParseArgsTable(t *testing.T) {
	def := options{root: ".", limit: 5, snip: 120, perFileCap: 2}
	cases := []struct {
		name    string
		args    []string
		want    options
		wantErr string // substring; "" means no error expected
	}{
		{
			name: "defaults",
			args: nil,
			want: def,
		},
		{
			name: "separated values and one query",
			args: []string{"--root", "/tmp/x", "--limit", "10", "foo"},
			want: func() options {
				o := def
				o.root = "/tmp/x"
				o.limit, o.limitSet = 10, true
				o.queries = []string{"foo"}
				return o
			}(),
		},
		{
			name: "equals form and bool flags, two queries",
			args: []string{"--limit=3", "--json", "--plain", "foo", "bar"},
			want: func() options {
				o := def
				o.limit, o.limitSet = 3, true
				o.jsonOut, o.plain = true, true
				o.queries = []string{"foo", "bar"}
				return o
			}(),
		},
		{
			name: "mode flag takes a value, no positional query",
			args: []string{"--callers", "getGraphParametersFlat"},
			want: func() options {
				o := def
				o.callers, o.callersSet = "getGraphParametersFlat", true
				return o
			}(),
		},
		{
			name: "exclude-term tokenizes camelCase per value",
			args: []string{"--exclude-term", "fooBar, test"},
			want: func() options {
				o := def
				o.excludeTerms = [][]string{{"foo", "bar"}, {"test"}}
				return o
			}(),
		},
		{
			name:    "missing value",
			args:    []string{"--limit"},
			wantErr: "requires a value",
		},
		{
			name:    "negative limit rejected",
			args:    []string{"--limit", "-1"},
			wantErr: `invalid --limit "-1"`,
		},
		{
			name:    "non-numeric limit rejected",
			args:    []string{"--limit=abc"},
			wantErr: `invalid --limit "abc"`,
		},
		{
			name:    "unknown flag rejected",
			args:    []string{"--nope"},
			wantErr: `unknown flag "--nope"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseArgs(c.args)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("parseArgs(%v) err = %v, want containing %q", c.args, err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArgs(%v) err = %v, want nil", c.args, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseArgs(%v) =\n%+v\nwant\n%+v", c.args, got, c.want)
			}
		})
	}
}

// Tahap 3 maintainability (2026-10-31): run() was split into helpers, so
// the validation and config logic that used to be inline (and untested)
// now has unit coverage. Error precedence is part of the contract here:
// query checks run before root resolution, and mode checks run after it.
func TestCheckQueryAndFlags(t *testing.T) {
	cases := []struct {
		name    string
		o       options
		wantErr string
	}{
		{"no query and no mode", options{}, "query is required"},
		{"single empty query", options{queries: []string{"  "}}, "query must not be empty"},
		{"json and paths-only conflict", options{queries: []string{"x"}, jsonOut: true, pathsOnly: true}, "--json and --paths-only are mutually exclusive"},
		{"all with limit conflict", options{queries: []string{"x"}, all: true, limitSet: true}, "--all is mutually exclusive with --limit and --per-file-cap"},
		{"valid single query", options{queries: []string{"x"}}, ""},
		{"callers mode needs no query", options{callersSet: true, callers: "foo"}, ""},
		{"batch blank query is allowed", options{queries: []string{"", "foo"}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkQueryAndFlags(c.o)
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("checkQueryAndFlags = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("checkQueryAndFlags = %v, want containing %q", err, c.wantErr)
			}
		})
	}
}

func TestCheckModeFlags(t *testing.T) {
	cases := []struct {
		name    string
		o       options
		mode    string
		wantErr string
	}{
		{"callers with query", options{queries: []string{"x"}}, "callers", "--callers takes no positional query"},
		{"related with query", options{queries: []string{"x"}}, "related", "--related takes no positional query"},
		{"callers with defs-only", options{defsOnly: true}, "callers", "--callers is incompatible with"},
		{"related with limit", options{limitSet: true}, "related", "--related is incompatible with"},
		{"valid callers", options{}, "callers", ""},
		{"valid related", options{}, "related", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkModeFlags(c.o, c.mode)
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("checkModeFlags = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("checkModeFlags = %v, want containing %q", err, c.wantErr)
			}
		})
	}
}

func TestResolveRootAndConfig(t *testing.T) {
	dir := t.TempDir()
	root, cfg, err := resolveRootAndConfig(options{root: dir})
	if err != nil {
		t.Fatalf("resolveRootAndConfig(%q) err = %v, want nil", dir, err)
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
	if cfg == nil || !cfg.UseGitignore {
		t.Errorf("cfg = %+v, want non-nil with UseGitignore true", cfg)
	}

	if _, _, err := resolveRootAndConfig(options{root: dir + "/missing"}); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("missing root err = %v, want not-a-directory error", err)
	}

	// Ext override replaces the default extension set.
	_, cfg, err = resolveRootAndConfig(options{root: dir, ext: ".go"})
	if err != nil {
		t.Fatalf("resolveRootAndConfig ext err = %v", err)
	}
	if !cfg.Exts[".go"] || cfg.Exts[".rs"] {
		t.Errorf("cfg.Exts = %v, want only .go", cfg.Exts)
	}
}
