# koment documentation

## Using koment

- **[Getting started](quickstart.md)** — install, first annotation, first drift
- **[Writing good annotations](annotating.md)** — what earns one, what doesn't
- **[CLI reference](cli.md)** — every command and flag
- **[Agent setup](agents/)** — per-client MCP configuration
- **[CI and pre-commit](ci.md)** — wiring the drift gate
- **[Publishing](publishing.md)** — the annotations on GitHub Pages, in one workflow file

## Working on koment

- **[Bootstrap](bootstrap.md)** — start here: what it is, the data model,
  running it, the demo, releases

- **[Development](development.md)** — layout, build, test, conventions
- **[Decisions](decisions/)** — why koment is the way it is, and what was rejected
- **[AGENTS.md](../AGENTS.md)** — rules for agents contributing to this repository

## The short version

Annotations live in `.koment/annotations/`, mirroring your source tree, one file
per annotated source file, committed to git. Each is anchored to a verbatim
excerpt plus its SHA-256 — never a line number.

Resolution produces exactly one of four statuses. `drifted` and `orphaned` fail
the build, because an annotation nobody revisited is worse than none.

Agents read through MCP; humans read through the CLI, `koment ui`, or a static
site published by `koment site`. Those are three tiers of the same data, and
moving between them is not a migration — they all read `.koment/`.
