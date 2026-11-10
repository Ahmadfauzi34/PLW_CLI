package main

// options is the CLI configuration parsed by parseArgs (main.go).
// It lives in its own file, not in main.go, so the verification harness
// (tokenize_spec.go, build tag "verify") can be compiled together with
// the rest of the package without main.go's main() — see
// verify-tokenize.sh, which lists files explicitly and omits main.go.
// Moving the type here changes no behavior: the definition is verbatim.
type options struct {
	root          string
	limit         int
	limitSet      bool
	snip          int
	pathsOnly     bool
	jsonOut       bool
	perFileCap    int
	perFileCapSet bool
	all           bool
	defsOnly      bool
	full          bool
	ext           string
	exclude       string
	excludeTerms  [][]string
	noGitignore   bool
	once          bool
	plain         bool
	callers       string
	callersSet    bool
	related       string
	relatedSet    bool
	queries       []string
	help          bool
	version       bool
	// G9: index state control. cacheDir is the explicit --cache-dir
	// value ("": resolve via cacheBaseDir). The three index modes are
	// their own commands: status (read-only probe), warm (build/save),
	// clean (delete this root's index file).
	cacheDir    string
	indexStatus bool
	indexWarm   bool
	indexClean  bool
}
