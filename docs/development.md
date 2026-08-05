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
and Helm smoke checks, then reports one required `ci` status; every job is named
after its id, so a red check names the job that produced it.

## Working on koment inside koment

`mise install` puts a released `koment` on `PATH`, which is what `.mcp.json` and
`.vscode/mcp.json` invoke. Opening the repository in VS Code recommends the
`janpuc.koment` extension; it carries its own binary, so it needs nothing else.

Both of those run the **released** koment. The gates do not: `mise run
annotations`, `comments` and `agent-policy` all run `go run ./cmd/koment`, so
they judge the tree you are editing. Expect the two to disagree while you are
changing the record format or a mutation surface — CI is the one that is right.

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

## Dependency updates

Renovate runs on GitHub runners from `.github/workflows/renovate.yml` — daily,
on demand, and whenever the Renovate configuration changes. It reads
`.renovaterc.json5`, which extends the shared `home-operations` preset.
[ADR 0122](decisions/0122-run-renovate-on-github-runners-behind-our-own-app.md)
records why it is self-hosted rather than the hosted app.

It stays inert until a GitHub App is installed, because a pull request opened
with `GITHUB_TOKEN` starts no workflows and so could never satisfy the required
`ci` check. Until then the job reports that it is inert and exits.

To activate it:

1. Create a GitHub App (Settings → Developer settings → GitHub Apps). No webhook.
   Repository permissions: **Checks**, **Commit statuses**, **Contents**,
   **Issues**, **Pull requests** and **Workflows** read and write, plus
   **Dependabot alerts** read. Nothing else.

   Commit statuses is not optional. Renovate writes a `renovate/stability-days`
   status on its own branch, and a 403 there aborts the whole run — reported as
   `Repository has changed during renovation`, which names neither the
   permission nor the request that failed.
2. Install it on `janpuc/koment` only.
3. Generate a private key.
4. Add the app id as the repository variable `RENOVATE_BOT_APP_ID` and the whole
   PEM as the secret `RENOVATE_BOT_PRIVATE_KEY`:

   ```sh
   gh variable set RENOVATE_BOT_APP_ID --body '<app id>'
   gh secret set RENOVATE_BOT_PRIVATE_KEY < private-key.pem
   ```

5. Dry-run it before letting it open anything:

   ```sh
   gh workflow run renovate.yml -f dryRun=true -f logLevel=debug
   ```

Do not use a personal access token here. `Workflows: write` is required so
Renovate can advance a pinned action SHA, and a token carrying that scope
alongside a person's other scopes turns any workflow change into a path to
their whole account.

The shared preset attaches `helm-docs` as a post-upgrade task. Renovate's
container does not carry it, so `.github/renovate-entrypoint.sh` installs it
and `RENOVATE_ALLOWED_COMMANDS` permits it. Without that, a Renovate pull
request that touches the chart leaves `charts/koment/README.md` stale and fails
`mise run generate-check`.

The preset also attaches `helm-schema`, which this repository does not permit.
`charts/koment/values.schema.json` is hand-maintained and stricter than a
generator can infer; helm-schema rewrites it into something permissive, and no
gate would notice. Do not add it to the allowed commands.

Validate a configuration change before pushing it:

```sh
npx --yes --package renovate renovate-config-validator .renovaterc.json5
```

## Where to start reading

`DESIGN.md` for the architecture, then `docs/decisions/` in order. Active
decisions start at ADR 0100; the pre-reset prototype decisions remain in Git
history.

Then run `koment ui` and look at the repository through its own tool.
