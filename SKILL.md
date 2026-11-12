---
name: plw-portable-agent
description: Official first-contact contract for using the standalone PLW binary against a target codebase.
---

# PLW Portable Agent

Use the binary as the canonical runtime surface. No neighboring PLW source tree is required.

## First contact

```bash
plw doctor <root>
plw agent orient <root> --task "<task>" --json
```

`agent orient` must establish target structure and evidence gaps before specialized capability selection. It is advisory/read-only and executes no selected capability.

## Discover capabilities

```bash
plw capability list --json
plw capability describe <command> --json
```

Governance commands also have grouped namespace aliases
(`plw lease validation|execution`, `plw proof obligations|postcondition`,
`plw ledger lineage|accountability|chain|memory|model|reconcile` — each maps
to one canonical command; see `plw --help` or `plw <group> --help`).
Aliases are CLI-only: `capability describe` answers an alias name with a
pointer (`alias_of`) to its canonical name; `skill show` accepts the
canonical names.

## Read exact embedded skill contracts

```bash
plw skill list --json
plw skill show <command-or-skills/path.md> --json
```

The exact operational modules are embedded in the standalone binary and are version-aligned with that executable.

## Direct structural first contact

```bash
plw topology <root> --json
```

## Authority

Portable-agent discovery/orientation does not authorize runtime execution, source mutation, external-repository mutation, stable promotion, or truth commit.
