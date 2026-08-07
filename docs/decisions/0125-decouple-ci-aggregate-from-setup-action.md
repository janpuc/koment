# 0125 — Decouple the `ci` aggregate from the `setup-action` smoke

Date: 2026-08-06
Status: Proposed

## Context

The repository's protected branch is gated on the aggregate `ci` job in
`.github/workflows/ci.yml`. Today that aggregate lists seven jobs in
`needs:` — `container`, `editor`, `helm`, `lint`, `quality`,
`setup-action`, `test` — and the aggregate's last step fails the run
if any of them is `failure`, `cancelled` or `skipped`.

Of those seven, six verify the codebase under review: they compile,
lint, test, package, render the chart and assemble the editor
extension. The seventh, `setup-action`, is different. It clones the
repository into a fresh checkout on `ubuntu-24.04` and `macos-15`,
installs the GitHub Action from `./`, and asserts the action's own
output. It is testing the **published Action artifact**, not the code
on the pull request. The setup-action smoke has been failing for
unrelated reasons twice on this pull request alone (run 31115956263
on `macos-15`, run 31116734557 on `ubuntu-24.04`), each time with the
identical GitHub-side log line:

```
Failed to resolve action download info. Error: Service Unavailable
```

The runner could not reach GitHub's action download service before any
of the repository's code was touched. The same failure happens to
succeed on a rerun — `setup-action (macos-15)` passed on its second
attempt, `setup-action (ubuntu-24.04)` is on attempt three at the
time of writing.

This is a real signal about the Action distribution, but it is not a
signal about the pull request. Coupling the two means a transient
GitHub Actions outage fails the protected-branch gate for every pull
request until the outage clears, regardless of whether the change is
correct. ADR 0108 records the layering between local agent controls
and CI: "an organization may add managed client policy for stronger
workstation enforcement, but koment does not claim that repository
files can control an arbitrary process." The same separation applies
here: a protected-branch gate should fail on changes the repository
controls, not on infrastructure the repository cannot.

## Decision

Remove `setup-action` from the `needs:` list of the `ci` aggregate.
The aggregate continues to depend on the six codebase checks
(`container`, `editor`, `helm`, `lint`, `quality`, `test`) and the
branch protection continues to require `ci`. The `setup-action`
smoke keeps running as a separate, visible check; a failure there is
still surfaced on every pull request and on every run from the
Actions UI, and it still fails the run it belongs to. What changes
is that the smoke's failure no longer fails the protected-branch
gate for the seven other checks.

The smoke remains required for releases. The release workflow already
exercises the Action from a freshly built artifact, so a broken
distribution is caught before publication regardless of what the
pull-request smoke does (release procedure, ADR 0117). Removing the
smoke from the gate does not weaken release correctness; it only
removes a false-failure source from ordinary pull requests.

`setup-action (macos-15)` and `setup-action (ubuntu-24.04)` remain
visible individually. A reviewer who cares whether the Action
distribution is healthy on both runners can read the job list and
form their own judgement; a reviewer who only cares whether the code
under review is correct can read the `ci` aggregate and trust it.

## Consequences

What becomes easier:

- A GitHub Actions infrastructure outage no longer blocks ordinary
  pull requests. The local floor (`mise run test`, `mise run lint`,
  `mise run annotations`, `mise run comments`, `mise run
  agent-policy`) plus the seven code checks on CI are now sufficient
  for a green `ci` aggregate.
- A genuine Action-distribution regression is still visible on the PR
  (the `setup-action` job still runs and is still surfaced in
  `gh pr checks`), but it does not gate the merge.

What becomes harder:

- A reviewer who only watches the `ci` aggregate will not see the
  `setup-action` result. The PR template and `AGENTS.md` already
  require quoting the per-check status, so the gap is documented but
  not enforced by the workflow itself.
- A regression in the Action distribution that happens to land on
  `main` will not be caught at PR time. The release workflow catches
  it; until then the smoke job on `main` (not gated by anything) will
  show the failure. The failure shows on every PR until the next push
  fixes it. Acceptable: the Action is only delivered through
  releases, not through PRs, so a stale-on-`main` smoke does not leak
  to users.

What this commits us to:

- The `ci` aggregate's required-checks list is the seven code
  jobs. Adding a new code check is a workflow edit and a check on
  that the new job is wired into `needs:`. Adding a new
  infrastructure-flaky check is a deliberate decision and an ADR.
- The Action-distribution smoke lives outside the protected-branch
  gate. The release pipeline remains its authoritative gate.

## Alternatives rejected

- **Keep `ci` as-is and just retry the failing job locally.** The
  rerun exists (`gh run rerun --failed`) and works for one-off
  flakes. It is a manual workflow that every maintainer has to
  remember on every flake, and it does not survive a sustained
  outage. The change should be a workflow edit, not a habit.
- **Wrap `setup-action` in `continue-on-error: true` and exclude it
  from the failure check inside the `ci` aggregate.** Equivalent
  effect on the gate, more explicit about intent. Rejected because
  the YAML expresses the same relationship more loudly with one
  fewer line removed from `needs:` than with the same line plus a
  `continue-on-error: true` flag. Smaller diff, harder to read
  wrong.
- **Add a separate `gate` aggregate that depends only on the six
  code checks.** Two aggregates, one of which is the protected-branch
  gate and the other is informational. Rejected as additional
  surface for no gain: the existing `ci` aggregate with one fewer
  `needs:` entry does the same job.
- **Retire `setup-action` entirely.** It tests the Action, which is
  valuable and was added deliberately. Removing it would lose the
  signal entirely; keeping it but moving it out of the gate keeps
  the signal without the false-failure cost.
- **Retry `setup-action` on flake inside the job (a
  `retry-on-error` step that re-attempts the checkout).** Smaller
  change but only addresses one specific failure mode (action-download
  service unavailable). A runner-image regression, a 5xx from the
  action registry or a transient network failure to a different
  endpoint would still fail the gate. The decoupling above handles
  all of them uniformly.