# 0122 — Run Renovate on GitHub runners behind an app we own

Date: 2026-08-05
Status: Accepted

## Context

`.renovaterc.json5` has been committed since the operational floor landed, and
nothing has ever executed it. Dependency updates have been unmanaged since the
Dependabot configuration was deleted in favour of Renovate — through eight
minor releases and the 1.0.0 cutover. koment pins every GitHub Action by SHA,
pins its whole toolchain in `.mise/config.toml`, and ships a container image and
a Helm chart. Pins that nothing advances are pins that quietly rot.

Three things have to be decided together, because the answer to each constrains
the others.

1. **Who runs Renovate.** The hosted Renovate app, or a self-hosted run on
   GitHub runners.
2. **Which identity it uses.** Whatever runs it has to push branches, open
   pull requests, and edit `.github/workflows/*` to advance an action SHA.
3. **Where the workflow lives.** `home-operations`, the operational baseline
   (ADR 0106), keeps a thin dispatching workflow in each repository and one
   real Renovate workflow in the organisation's `.github` repository.

The identity question is the sharp one. A pull request opened with
`GITHUB_TOKEN` does not start any workflow — the same rule that leaves every
release pull request's checks at `action_required`. The branch ruleset requires
the `ci` check, so a Renovate pull request opened that way would be permanently
unmergeable. Renovate must therefore authenticate as something other than the
workflow's own token.

## Decision

**Self-hosted, on GitHub runners, in this repository.**
`.github/workflows/renovate.yml` runs `renovatebot/github-action` daily, on
`workflow_dispatch`, and on any push that touches the Renovate configuration or
the workflow itself. `RENOVATE_REPOSITORIES` names this repository explicitly;
there is no autodiscovery to widen the blast radius.

**Behind a GitHub App we own.** `actions/create-github-app-token` mints an
installation token scoped to `owner/${{ github.event.repository.name }}` with
exactly `checks`, `contents`, `issues`, `pull-requests` and `workflows` write.
The token lives for the length of one job. Its pull requests start CI, because
it is not `GITHUB_TOKEN`.

**Inert until configured.** The workflow keys off the `RENOVATE_BOT_APP_ID`
repository variable. Absent it, the job emits a `::notice::` naming what is
missing and does nothing else — the same opt-in shape the release workflow
already uses for marketplace publication. A scheduled workflow that fails every
night is a workflow people learn to ignore.

## Consequences

What becomes easier:

- Action SHAs, Go modules, mise tools, the npm dependencies under
  `editors/vscode`, the base image and the chart all get proposed updates that
  land as ordinary pull requests through the ordinary gate.
- Every update is reviewable as a diff with CI attached, which is the only
  reason to accept a dependency bump at all.
- The run is debuggable. A self-hosted run's full log is in Actions, with
  `logLevel: debug` and `dryRun` available on `workflow_dispatch`.

What becomes harder:

- Somebody has to create and install a GitHub App, which cannot be done from a
  terminal. Until then, Renovate stays inert; the workflow says so out loud
  rather than pretending.
- The Renovate version is now ours to advance. The action is SHA-pinned like
  every other, so Renovate ends up proposing its own upgrades — which is fine,
  and is exactly the same loop every other pin is in.
- Renovate holds a token that can write `.github/workflows/`. That is not
  incidental; without `workflows: write` it cannot advance a pinned action SHA,
  which is most of the value here. The mitigation is scope and lifetime: one
  repository, five permissions, one job.

What this commits us to:

- The app's private key is a repository secret. Rotating it is a maintenance
  task with no automation behind it.
- `local>home-operations/renovate-config` stays the shared preset. koment
  follows their grouping and scheduling decisions rather than restating them.

## Alternatives rejected

- **Install the hosted Renovate app.** Least work, no workflow to maintain, and
  it is what `eleboucher/memini` does — its `.renovaterc.json5` is the whole
  setup. Rejected because the request was explicitly for runners, and because
  the hosted app's runs are only observable through its own dashboard: when a
  update is not proposed, there is no log in this repository explaining why.

- **Use the maintainer's personal access token.** It exists, and it would work
  today with no app to create. Rejected on inspection: that token carries
  `admin:org`, `admin:enterprise`, `delete_repo`, `repo`, `workflow` and
  `write:packages`. Putting it in a public repository's secrets makes every
  future workflow change, and every action in the supply chain, a path to a
  credential that can delete repositories. The blast radius is the entire
  account to save one afternoon of setup.

- **A fine-grained PAT scoped to this repository.** Much closer to acceptable,
  and genuinely simpler than an app. Rejected because a fine-grained PAT still
  belongs to a person, expires on a date nobody remembers, and its pull requests
  are authored by that person — so `git log` stops distinguishing what a human
  decided from what a bot proposed. An app has its own identity, and that
  identity is the point.

- **Dispatch to a central `.github` repository, as `home-operations` does.**
  The baseline pattern, and better at their scale: one Renovate workflow, many
  repositories, one cache. Rejected because `janpuc` is a user account with no
  `.github` repository, so adopting it means creating and maintaining a second
  repository whose only job is to run a workflow for one consumer. ADR 0106
  makes konflate the baseline "unless an ADR records a deliberate difference";
  this is one. If a second repository ever needs Renovate, that is the moment
  to hoist it, and the workflow here is already the body they would hoist.

- **Let `GITHUB_TOKEN` do it.** Free, no secret, no app. Rejected on a fact
  rather than a preference: its pull requests start no workflows, so `ci` never
  reports, so the ruleset never lets them merge. Renovate would fill the
  repository with pull requests nobody could land.
