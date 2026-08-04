---
name: koment
description: Apply the strict koment procedure whenever reading or changing a koment-enabled repository, including code review, refactoring, debugging, and adding rationale.
---

# koment procedure

Use the bundled writable MCP server as the first-class interface for repository
rationale.

Before editing an existing file, call `koment_get` for that file. Before
changing a non-obvious decision, call `koment_search` for the relevant topic.
An annotation reported as drifted, orphaned, or ambiguous is historical context
and must not be treated as current truth.

Do not add an explanatory inline comment. Try renaming, extraction, a named type
or constant, and restructuring in that order. If rationale still needs to be
recorded at a source location, call `koment_add` with agent authorship.

When ordinary comment prose already exists, call `koment_convert_comment` so
the annotation is durable before the source prose is removed. Keep a comment
only through `koment_acknowledge_comment`, with the explicit acknowledgement
set and a body explaining why all four code-clarity alternatives failed.

Before completing work, run `koment check`, `koment comments check`, and
`koment agents check`. Resolve every failure instead of suppressing or deleting
the rationale that exposed it.
