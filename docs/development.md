# Development

## Build and test

```sh
mise install
mise tasks
mise run build
mise run fmt-check
mise run tidy-check
mise run vet
mise run test
mise run lint
mise run vulncheck
mise run workflow-lint
mise run annotations
mise run comments
mise run agent-policy
```

`.mise/config.toml` and `.mise/mise.lock` are the toolchain source of truth.
The Go version there must match `go.mod`. `mise install` also installs the
Lefthook pre-commit checks. CI runs these tasks plus the setup action, container
and Helm smoke checks, then reports one required `test` status.

## Layout

```
cmd/koment/          entrypoint, flag parsing only
internal/cli/        add, show, check, list, reanchor
internal/application/ shared snapshot and mutation service
internal/agentpolicy/ generated instructions, client adapters and hooks
internal/store/      read/write .koment/annotations, ULIDs, prose wrapping
internal/anchor/     resolution and drift status
internal/commentpolicy/ deterministic source-comment classification
internal/policy/      strict repository policy
internal/listen/     bind address resolution, shared by both servers
internal/mcp/        MCP server — stdio and HTTP
internal/ui/         local web view and atomic static publication
docs/decisions/      ADRs
```

Dependency direction is one-way: `store` depends on nothing internal, `anchor`
on `store`, everything else on those. `cli` deliberately imports neither `mcp`
nor `ui` — both are injected into `cli.Run` as function values, which is what
keeps the MCP SDK out of the CLI's link graph.

## Conventions

Read [AGENTS.md](../AGENTS.md). It applies to humans too; it is addressed to
agents because that is who mostly writes here.

The short version:

**Comments are a last resort.** Before writing one: rename the thing, extract a
function whose name is the comment, introduce a named constant, or restructure
so the invariant is obvious. Only when all four fail has a comment earned its
place — and then it explains *why*, never *what*. Godoc on exported identifiers
is API documentation and doesn't count.

**Rationale goes in an ADR or an annotation.** Project-wide reasoning becomes an
ADR in `docs/decisions/`. Reasoning bound to a place in the code becomes a
koment annotation. This repository is koment's own first user — see
[ADR 0107](decisions/0107-dogfood-the-comment-free-thesis.md).

**Every dependency needs an ADR.** Standard library first; a small
well-understood module over a framework. The bar for another direct dependency
is high.

**Active ADRs are immutable.** Changed your mind? Write a new one that
supersedes the old and mark the old one. The owner-authorized pre-deployment
reset that created the 0100 series is recorded in the
[decision index](decisions/README.md); it is not the normal workflow.

**Design before code.** For anything beyond a bugfix, update `DESIGN.md` first
and get it agreed. Don't open a large diff that also invents the design.

**Fail loudly.** Never swallow an error, never return a partial result that
looks complete, never serve an annotation without its status. A tool that
silently serves a stale annotation is worse than one that crashes.

## Tests

Every anchoring rule gets a test with a real before/after file pair —
`internal/anchor/testdata/` holds them. Drift detection has a test per status.

The servers are tested end-to-end against real clients rather than by calling
handlers: `internal/mcp` drives the official SDK client over an in-memory
transport and over HTTP, and `cmd/koment` builds the binary and speaks to it as
a subprocess over real stdio. That last one exists because the in-memory
transport never exercises the stdio wiring every agent actually connects
through.

## Annotating your own changes

```sh
./koment add <file> --excerpt '<snippet>' --kind gotcha --body -
./koment check
```

If a change drifts an existing annotation, fix the anchor rather than deleting
the annotation:

```sh
./koment reanchor <id> --excerpt '<new snippet>'
```

## Commits

Conventional subjects: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`.
One concern per commit — if reviewing needs a section-by-section walkthrough,
split it. Stage deliberately; never `git add -A` blindly.

`main` requires a pull request with CI green, signed commits and linear history.

## Where to start reading

`DESIGN.md` for the architecture, then `docs/decisions/` in order. Active
decisions start at ADR 0100; the pre-reset prototype decisions remain in Git
history.

Then run `koment ui` and look at the repository through its own tool.
