package main

// File scanning: walking, extension/exclude/gitignore filtering, binary
// detection, and overlapping line-chunking. Mirrors codesearch.py behavior.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var defaultExts = []string{".rs", ".js", ".mjs", ".ts", ".tsx", ".py", ".go",
	".java", ".c", ".h", ".cpp", ".md", ".txt", ".rst", ".adoc"}

var skipDirs = map[string]bool{
	".git": true, "target": true, "node_modules": true,
	"__pycache__": true, ".hg": true, ".svn": true,
}

const (
	chunkSize   = 30
	chunkStride = 15
)

// ScanConfig mirrors codesearch.py's ScanConfig.
type ScanConfig struct {
	Exts         map[string]bool
	Exclude      []string
	UseGitignore bool
}

func defaultScanConfig() *ScanConfig {
	m := map[string]bool{}
	for _, e := range defaultExts {
		m[e] = true
	}
	return &ScanConfig{Exts: m, UseGitignore: true}
}

func (c *ScanConfig) key() string {
	exts := make([]string, 0, len(c.Exts))
	for e := range c.Exts {
		exts = append(exts, e)
	}
	sort.Strings(exts)
	exc := append([]string{}, c.Exclude...)
	sort.Strings(exc)
	gi := 0
	if c.UseGitignore {
		gi = 1
	}
	return "ext=" + strings.Join(exts, ",") + "|exc=" + strings.Join(exc, ",") +
		"|gi=" + strconv.Itoa(gi)
}

func parseExts(s string) map[string]bool {
	m := map[string]bool{}
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(strings.ToLower(x))
		if x == "" {
			continue
		}
		if !strings.HasPrefix(x, ".") {
			x = "." + x
		}
		m[x] = true
	}
	return m
}

func parsePatterns(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// compileGlob translates an fnmatch-style glob to a regexp, like Python's
// fnmatch.translate: * matches everything (including '/'), ? matches one
// rune, [seq]/[!seq] are character classes. Case-sensitive like POSIX fnmatch.
func compileGlob(pat string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("(?s)^")
	rs := []rune(pat)
	for i := 0; i < len(rs); i++ {
		switch c := rs[i]; c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i + 1
			var cls strings.Builder
			cls.WriteByte('[')
			if j < len(rs) && (rs[j] == '!' || rs[j] == '^') {
				cls.WriteByte('^')
				j++
			}
			if j < len(rs) && rs[j] == ']' {
				cls.WriteString(`\]`)
				j++
			}
			for j < len(rs) && rs[j] != ']' {
				if rs[j] == '\\' {
					cls.WriteString(`\\`)
				} else {
					cls.WriteRune(rs[j])
				}
				j++
			}
			cls.WriteByte(']')
			b.WriteString(cls.String())
			i = j
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		// unreachable for sane patterns; fall back to literal match
		return regexp.MustCompile("(?s)^" + regexp.QuoteMeta(pat) + "$")
	}
	return re
}

// globMatch is the one-shot convenience wrapper; hot paths precompile.
func globMatch(pat, s string) bool { return compileGlob(pat).MatchString(s) }

// --- .gitignore subset -------------------------------------------------------

type giPattern struct {
	neg    bool
	isDir  bool
	pat    string
	re     *regexp.Regexp
	nameRe *regexp.Regexp // same pattern, applied to basename
}

func loadGitignore(root string) []giPattern {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return nil
	}
	var pats []giPattern
	for _, line := range strings.Split(string(data), "\n") {
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

func gitignored(rel string, pats []giPattern) bool {
	ignored := false
	parts := strings.Split(rel, "/")
	base := parts[len(parts)-1]
	for _, gp := range pats {
		var hit bool
		if gp.isDir {
			for _, d := range parts[:len(parts)-1] {
				if gp.re.MatchString(d) {
					hit = true
					break
				}
			}
			if !hit && strings.Contains(gp.pat, "/") {
				hit = gp.re.MatchString(rel) || strings.HasPrefix(rel, gp.pat+"/")
			}
		} else if strings.Contains(gp.pat, "/") {
			hit = gp.re.MatchString(rel)
		} else {
			hit = gp.nameRe.MatchString(base)
		}
		if hit {
			ignored = !gp.neg
		}
	}
	return ignored
}

// --- walking -----------------------------------------------------------------

func iterFiles(root string, cfg *ScanConfig) []string {
	var ig []giPattern
	if cfg.UseGitignore {
		ig = loadGitignore(root)
	}
	excludeRes := make([]*regexp.Regexp, 0, len(cfg.Exclude))
	for _, pat := range cfg.Exclude {
		excludeRes = append(excludeRes, compileGlob(pat))
	}
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if cfg.Exts[filepath.Ext(p)] != true {
			return nil
		}
		// skip hidden parts and well-known junk dirs (checked on the
		// absolute path, like codesearch.py's p.parts check)
		for _, part := range strings.Split(p, string(os.PathSeparator)) {
			if strings.HasPrefix(part, ".") || skipDirs[part] {
				return nil
			}
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		excluded := false
		for _, re := range excludeRes {
			if re.MatchString(rel) || re.MatchString(d.Name()) {
				excluded = true
				break
			}
		}
		if excluded {
			return nil
		}
		if ig != nil && gitignored(rel, ig) {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out
}

// --- reading -----------------------------------------------------------------

type fileRead struct {
	text       string // "" when skipped
	haveText   bool
	mtimeNanos int64
	size       int64
	statOK     bool
	// skip explains a manifest-only file: "", "oversize" (>2MB),
	// "binary" (null byte in first 8KB), "overchars" (>500K chars).
	// Files with statOK=false (stat/read failure) keep skip "" and never
	// enter the manifest — they are a build race, not a skip reason.
	skip string
}

func readSource(root, rel string) fileRead {
	var fr fileRead
	st, err := os.Stat(filepath.Join(root, rel))
	if err != nil {
		return fr
	}
	fr.statOK = true
	fr.mtimeNanos = st.ModTime().UnixNano()
	fr.size = st.Size()
	if st.Size() > 2_000_000 {
		fr.skip = "oversize"
		return fr
	}
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		fr.statOK = false
		return fr
	}
	head := raw
	if len(head) > 8192 {
		head = head[:8192]
	}
	for _, b := range head {
		if b == 0 {
			fr.skip = "binary"
			return fr // binary: manifest only
		}
	}
	t := string(raw)
	if !utf8.ValidString(t) {
		// errors="replace" equivalent (per-run replacement is fine)
		var b strings.Builder
		for len(t) > 0 {
			r, size := utf8.DecodeRuneInString(t)
			if r == utf8.RuneError && size <= 1 {
				b.WriteRune(unicode.ReplacementChar)
			} else {
				b.WriteRune(r)
			}
			t = t[size:]
		}
		t = b.String()
	}
	if len(t) > 500_000 {
		fr.skip = "overchars"
		return fr
	}
	fr.text, fr.haveText = t, true
	return fr
}

// --- chunking ------------------------------------------------------------------

// chunkKind distinguishes what a chunk is: a definition signature
// (doc comment + declaration header), a definition body, or a plain
// fixed window. Kind is preserved into ChunkDoc for --full and JSON.
type chunkKind string

const (
	kindWindow chunkKind = "window"
	kindBody   chunkKind = "body"
	kindSig    chunkKind = "sig"
)

type chunk struct {
	startLine int // 1-based
	lines     []string
	kind      chunkKind
	name      string // defined name for sig/body chunks, "" for windows
}

// chunkLines yields overlapping windows of 30 lines, stride 15.
func chunkLines(text string) []chunk {
	// normalize \r\n and \r like str.splitlines() does for line ends
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	n := len(lines)
	if n <= chunkSize {
		return []chunk{{startLine: 1, lines: lines, kind: kindWindow}}
	}
	var out []chunk
	for start := 0; start < n; {
		end := start + chunkSize
		if end > n {
			end = n
		}
		out = append(out, chunk{startLine: start + 1, lines: lines[start:end], kind: kindWindow})
		if end == n {
			break
		}
		start += chunkStride
	}
	return out
}

// splitCamel inserts spaces at camelCase boundaries, index and query alike.
// getGraph -> "get Graph"; HTMLParser -> "HTML Parser".
func splitCamel(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	rs := []rune(s)
	for i, r := range rs {
		if i > 0 {
			var next rune
			hasNext := i+1 < len(rs)
			if hasNext {
				next = rs[i+1]
			}
			if camelBoundary(rs[i-1], r, next, hasNext) {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}
