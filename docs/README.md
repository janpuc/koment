# koment documentation

## Using koment

- **[Getting started](quickstart.md)** — install, first annotation, first drift
- **[Writing good annotations](annotating.md)** — what earns one, what doesn't
- **[CLI reference](cli.md)** — every command and flag
- **[Agent setup](agents/)** — per-client MCP configuration
- **[Editors](editors/)** — the extension, and the three lines every other editor needs
- **[Language support](languages.md)** — where comment detection reaches, and where it does not
- **[CI and pre-commit](ci.md)** — wiring the drift gate
- **[Publishing](publishing.md)** — the annotations on GitHub Pages, in one workflow file

## Working on koment

- **[Bootstrap](bootstrap.md)** — start here: what it is, the data model,
  running it, the maintained workspace, releases

- **[Development](development.md)** — layout, build, test, conventions
- **[Releasing](releasing.md)** — the mandatory step-by-step procedure
- **[Decisions](decisions/)** — why koment is the way it is, and what was rejected
- **[AGENTS.md](../AGENTS.md)** — rules for agents contributing to this repository

## The short version

Annotations live in `.koment/annotations/<id>.yaml`, one Git record per stable
annotation id. Each excerpt anchor stores verbatim source plus captured context;
the last seen line describes movement but never chooses identity.

Resolution produces exactly one of four statuses. `ambiguous`, `drifted` and
`orphaned` fail the build, because uncertain rationale is worse than none.

Agents read through MCP; humans read through the CLI, `koment ui`, or a static
site published by `koment site`. Those are three tiers of the same data, and
moving between them is not a migration — they all read `.koment/`.
