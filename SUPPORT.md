# PLW Support

PLW support is centered on reproducible behavior of the latest published standalone binary.

## What to report

Use GitHub issues for:

- standalone binary failures;
- incorrect or misleading analysis output;
- target-repository pollution;
- capability/skill discovery problems;
- `plw doctor` or `plw agent orient` failures;
- `plw ast`, `plw topology`, or engine failures (`plw ast doctor`
  output, engine version and digest);
- release/checksum problems;
- reproducible browser/runtime failures;
- documentation gaps or feature requests.

For security-sensitive reports, follow [SECURITY.md](SECURITY.md) instead of posting exploit details publicly.

## Include in a bug report

Please include:

- PLW version (`plw doctor <target> --json` includes the embedded version);
- platform and architecture;
- exact PLW command;
- smallest reproducible target or fixture that can be shared;
- expected versus observed result;
- relevant JSON output or bounded log excerpt;
- whether Chromium/Chrome was involved;
- whether the issue reproduces with the published standalone binary.

For codesearch engine failures, also include:

- `plw ast doctor /path/to/project --json` output (it carries the
  engine version, schema, `pinned` status, and `engine_digest`);
- how the engine was supplied (`CODESEARCH_BIN`, explicit path, or PATH);
- whether `CODESEARCH_ALLOW_UNPINNED=1` was set.

For browser-backed failures, also include the browser executable/version when available and distinguish static UI analysis from Chromium/CDP execution.

## Support boundary

PLW outputs are bounded evidence. Support does not assume that a candidate is a valid patch, that topology proves causality, or that one reproduced scenario proves global behavior.

The latest published release is the primary supported distribution. Development commits on `main` are pre-release work unless a release explicitly publishes them.
