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

A single-repository checkout — the only shape implemented today — is this model
with one repository, discovered by walking up from the working directory for
`.koment/`, then `.git/`.

An annotation on disk:

```yaml
version: 1
file: internal/store/ulid.go
annotations:
  - id: 01KYW1ETE3CVB6S0ND70GGZVWM   # ULID, stable forever, never reused
    scope: excerpt                    # or `file`
    excerpt: "\tpaddingBits = ulidLength*bitsPerChar - 8*(...)"
    excerpt_sha256: b3f62f10e117b769...
    last_seen_line: 18                # a hint, never an anchor
    kind: gotcha                      # why | gotcha | invariant | anti-pattern
    body: |-
      26 Crockford characters carry 130 bits but a ULID holds 128 …
    created: "2026-08-02"
    git:                              # what was true at creation; never rewritten
      commit: 9f3c1a4d8e2b7c5a...
      path: internal/store/ulid.go
      line: 18
    author:
      name: Jan Pucilowski
      kind: human                     # or `agent`
      source: git-config              # how much the identity is worth
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
applicability. ADR 0014 has the reasoning.

Resolution produces one of four statuses. `drifted` and `orphaned` exit
non-zero; `ok` and `moved` do not.

### Where the data lives

Two things, and it matters which is which:

| | is | lives in |
|---|---|---|
| `.koment/annotations/**.yaml` | **the record** — reviewed, merged, cloned | git |
| the index | **derived** — queried, searched, filtered | cache dir, or Postgres |

The index is rebuilt from YAML and is never authoritative. Delete it and run
`koment index --rebuild`; nothing is lost. It is gitignored because it is a
build artifact, and a database file in git would be unreviewable and unmergeable
(ADR 0022).

Resolution stays live: the index stamps each file with `(mtime, size)` and
re-resolves anything that changed before serving a status.

The derivation runs both ways, exactly (ADR 0023):

```sh
koment index      # .koment  ->  index   (automatic when the index is empty)
koment export     # index    ->  .koment (byte-identical)
```

So a wiped cache costs a rebuild, and a lost `.koment/` costs an export. Neither
loses an annotation.

## Run and test it locally

```sh
go build -o koment ./cmd/koment
go test ./...                 # add -race before pushing anything concurrent
go vet ./...
gofmt -l .                    # must print nothing
golangci-lint run ./...       # go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
./koment check                # koment's own annotations must resolve
```

CI runs exactly that, plus a container build and Helm chart validation. If those
five commands pass locally, CI will pass.

Try the tool on itself:

```sh
./koment show internal/store/ulid.go
./koment ui                   # then open the printed URL
```

### Layout

```
cmd/koment/          entrypoint, flag parsing only
internal/cli/        add, show, check, list, reanchor
internal/store/      records, ULIDs, prose wrapping, git context, authorship
internal/anchor/     resolution and drift status
internal/provenance/ captures git context and author identity
internal/listen/     bind address resolution, shared by both servers
internal/index/      derived index — SQLite and Postgres
internal/config/     KOMENT_* environment fallback for every flag
internal/metrics/    Prometheus instrumentation
internal/mcp/        MCP server — stdio and HTTP
internal/ui/         web view and static export
charts/koment/       Helm chart
demo/                the fixture behind the published demo
```

Dependencies point one way: `store` depends on nothing internal, `anchor` on
`store`, everything else on those. `cli` imports neither `mcp` nor `ui` — they
are injected into `cli.Run` as a `Servers` struct, which is what keeps the MCP
SDK out of the CLI's link graph. That is load-bearing, not stylistic (ADR 0007).

## Regenerate the demo

`demo/` is a fixture, not a real project. Its annotations are deliberately in
every state, including `drifted` and `orphaned`, so the published demo can show
what koment's own green repository never will.

It has **its own store** at `demo/.koment/`, so its intentional drift is
invisible to `koment check` at the root.

```sh
cd demo
../koment list                 # 1 ok, 2 moved, 1 drifted, 1 orphaned
../koment site --out /tmp/demo
open /tmp/demo/index.html
```

CI does the same on every push to `main` and deploys to Pages, failing if any of
the four statuses stops appearing. Changing `demo/` means checking that all four
survive — that assertion is the point of the fixture.

## Publish a release

Releases are a merge, not a ritual. Conventional commit subjects (`feat:`,
`fix:`, `docs:`, …) are already required by AGENTS.md §8, and release-please
turns them into a version and a changelog.

1. Merge work into `main` with conventional subjects.
2. release-please opens or updates a release pull request with the next version
   and the changelog. `charts/koment/Chart.yaml` is bumped in the same commit,
   so the chart can never point at an image tag that does not exist.
3. Merge that pull request. It tags the release, which triggers publishing:
   - binaries for linux, macOS and Windows on amd64 and arm64, plus a
     `koment_<version>_checksums.txt` the setup action verifies against
   - `ghcr.io/janpuc/koment:<version>` — multi-arch, distroless, non-root, SBOM
   - `oci://ghcr.io/janpuc/charts/koment` — the Helm chart

The binaries are not optional decoration: `janpuc/koment@v0.2.0` downloads them, so
a release that fails to attach them breaks every workflow that uses the action
(ADR 0026).

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
