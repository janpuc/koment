# Bootstrap

Everything needed to be productive here, in one page. Written for a new
developer and for a coding agent on its first task, who need the same things.

## What this is

koment stores the *why* of a codebase beside the code instead of inside it.

An annotation is prose attached to a snippet of source, kept in `.koment/` in
git, and **checked**. When the code an annotation describes changes, koment
fails the build rather than quietly serving a note that no longer applies. That
is the whole thesis: a stale explanation is worse than none, so staleness has to
be detectable.

Agents read annotations over MCP; humans read them with the CLI or `koment ui`.

The starting points, in order: `README.md` for the product, `DESIGN.md` for the
architecture, `docs/decisions/` for why it is that way. `AGENTS.md` is binding
on anyone writing code here, human or not.

## The data model

Five things, nested:

```
Deployment
└── Repository        id, name, clone URL, default branch, config, sync state
    └── Commit        the git context an annotation was created against
        └── File      path at that commit
            └── Annotation
```

A local checkout discovers its repository by walking up from the working
directory for `.koment/`, then `.git/`. Configured UI and MCP processes can
serve several assigned local roots.

An annotation on disk:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/koment-dev/koment/main/schema/v1alpha/annotation.schema.json
apiVersion: koment.dev/v1alpha
kind: Annotation
metadata:
  id: 01KYW1ETE3CVB6S0ND70GGZVWM
  created: "2026-08-02T14:02:00Z"
spec:
  target:
    file: internal/store/ulid.go
  type: gotcha
  body: |-
    26 Crockford characters carry 130 bits but a ULID holds 128 …
  anchor:
    scope: excerpt
    excerpt: paddingBits = ulidLength*bitsPerChar - 8*(...)
    before: const (
    after: )
  git:
    commit: 9f3c1a4d8e2b7c5a...
    path: internal/store/ulid.go
    line: 18
  author:
    name: Jan Pucilowski
    kind: human
    source: git-config
status:
  lastSeenLine: 18
```

**The one idea to internalise.** Two fields look like they do the same job and
do not:

| | asks | uses | changes |
|---|---|---|---|
| `excerpt` | does this still apply *now*? | text search | every resolution |
| `git` | what was true when it was written? | commit hash | never |

Resolution reads the excerpt and nothing else. Deleting every `git:` block would
change no status — there is a test asserting exactly that. The commit hash is
authoritative for reconstructing history; the excerpt is authoritative for
applicability. [ADR 0100](decisions/0100-one-git-record-per-annotation.md)
preserves that separation in the current record.

Resolution produces one of four statuses. `ambiguous`, `drifted` and `orphaned`
exit non-zero; `ok` does not. There is no status for an annotation that resolved
somewhere new — resolution is a search, so that is simply `ok`.

### Where the data lives

One authoritative thing:

| | is | lives in |
|---|---|---|
| `.koment/annotations/<id>.yaml` | **the record** — reviewed, merged, cloned | git |

Local readers build current resolution directly from these records and source.
Static and served read models are disposable projections; neither can restore
or overwrite Git. ADR 0102 records that boundary.

## Run and test it locally

```sh
mise install
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

The committed lock file pins Go and every project tool for local shells and CI,
including a released `koment` so the MCP configuration in `.mcp.json` and
`.vscode/mcp.json` resolves. Lefthook is installed by `mise install` and checks
formatting, annotations, comment policy and agent adapters before a commit. CI
runs the same tasks plus the setup action, container and Helm smoke checks
behind one required `ci` status.

Try the tool on itself:

```sh
./koment show internal/store/ulid.go
./koment ui --write
```

### Layout

```
cmd/koment/          entrypoint, flag parsing only
internal/cli/        add, show, check, list, reanchor
internal/application/ shared snapshot and mutation service
internal/agentpolicy/ generated instructions, client adapters and hooks
internal/store/      records, ULIDs, prose wrapping, git context, authorship
internal/anchor/     resolution and drift status
internal/commentpolicy/ deterministic source-comment classification
internal/policy/      strict repository policy
internal/provenance/ captures git context and author identity
internal/listen/     bind address resolution, shared by both servers
internal/config/     KOMENT_* environment fallback for every flag
internal/metrics/    Prometheus instrumentation
internal/mcp/        MCP server — stdio and HTTP
internal/lsp/        editor-neutral diagnostics, views and mutations over LSP
internal/server/     authenticated commit-snapshot UI and MCP service
internal/serving/    atomic multi-repository catalog and synchronization
internal/ui/         web view and static export
charts/koment/       Helm chart
editors/vscode/      dependency-free runtime client for koment lsp
plugins/koment/      Claude marketplace plugin, MCP and completion hooks
packaging/           release-derived Homebrew, Scoop and WinGet metadata
workspace/           maintained session package and independent koment store
```

Dependencies point one way: `store` depends on nothing internal, `anchor` on
`store`, everything else on those. `cli` imports neither `mcp` nor `ui` — they
are injected into `cli.Run` as a `Servers` struct, which is what keeps the MCP
SDK out of the CLI's link graph. CLI, UI, MCP and static publication consume the
shared application model defined by ADR 0102.

## Verify the maintained workspace

`workspace/` is a tested session package with current rationale. It has its own
store so local and published multi-repository behavior exercise the same
boundary as unrelated repositories without keeping deliberately broken product
content.

```sh
go test ./workspace/...
cd workspace
../koment check
../koment site --out /tmp/koment-workspace
```

CI does the same on every push to `main`. Pages opens koment itself at the root
and exposes the workspace through the normal repository switcher. Failing
resolution states live in `internal/anchor/testdata`, where each has a real
before and after source pair and cannot be mistaken for maintained rationale.

## Publish a release

**[docs/releasing.md](releasing.md) is the procedure, and it is mandatory.**
Read it before you touch a release. What follows is orientation, not
instructions.

A conventional commit subject decides the version, release-please turns merged
subjects into a version and a changelog, and merging its pull request tags the
release. The tag publishes signed binaries, the container image, the Helm chart,
seven editor packages, MCP Registry metadata and generated package-manager
submission files.

Two properties are worth carrying in your head before reading the procedure.
Publication is ordered — canonical artifacts first, downstream channels second
([ADR 0109](decisions/0109-distribute-authenticated-artifacts-instead-of-go-invocations.md))
— so the extension cannot be built for a release whose binaries failed. And
marketplace versions are permanent: nothing published can be replaced or
withdrawn, only superseded by the next version.

Nothing is published from an unmerged branch, and `main` requires a pull request
with green CI, so there is no path to releasing something that did not pass.

## Rules that will trip you up

Read `AGENTS.md` in full. The three that catch people:

**Comments explaining *why* are banned.** Rename the thing, extract a function
whose name is the comment, introduce a named constant, or restructure. If the
reasoning still needs saying, it goes in an ADR (project-wide) or a koment
annotation (bound to a place in the code) — not in a `//`. Godoc on exported
identifiers is API documentation and does not count.

**ADRs are immutable.** Changed your mind? Write a new one that supersedes the
old and say so in both. The record of what was rejected is the product.

**Design before code.** Anything beyond a bugfix updates `DESIGN.md` first.
Do not open a diff that also invents the design.

## Where to look when something is confusing

The annotations are the answer to "why is this like this", and they are what the
tool is for:

```sh
./koment show <the file you are about to edit>
./koment list --kind gotcha
```

With an MCP-capable client, `.mcp.json` is committed — `koment_get` and
`koment_search` are already available. See [docs/agents/](agents/).
