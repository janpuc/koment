# 0128 — Enforce Conventional Commits 1.0.0 subjects

Date: 2026-08-07
Status: Accepted

## Context

The repository's release pipeline keys the version and changelog off
conventional commit subjects — `release-please-config.json` line 45–83
maps the type to a changelog section, `release-please-config.json` line 4
pins `chore(release): ${version}` as the release PR title, and ADR 0120
records `bump-minor-pre-major: false` so `feat!:` is a major bump.
`docs/releasing.md` step 3 names the rule and the type set.

AGENTS.md §8, the "Code rules" bullet in `CONTRIBUTING.md`,
`docs/development.md` "Commits" and `docs/bootstrap.md` "Publish a
release" each restate the rule in prose, but enforcement is left to code
review. A reviewer who approves a non-conforming subject leaves the rule
unenforced; an agent writing commits without a human reviewing it may
not produce a conforming subject at all.

release-please silently ignores subjects it does not recognise, so a
non-conforming commit lands in `main` without bumping a version or
producing a changelog entry — the same failure mode as a stale annotation
(ADR 0100). The hidden `chore` section in `release-please-config.json`
line 79–82 makes this silent: a `chore:` commit produces no changelog
entry, and a non-conforming subject behaves as if it were chore.

A previous draft of this ADR (status `Proposed`) and a previous draft
of `.github/workflows/commit-lint.yml` already exist on disk. The
draft ADR contradicts itself on the `feat!:` clause, lists types
(`style`, `revert`) that release-please omits, and adds two spec-extras
(lowercase description, 89-character cap) that the Conventional Commits
1.0.0 specification does not require. The draft workflow pins a
synthetic `actions/setup-node` SHA, references a missing
`package-lock.json`, is not in the `ci` aggregate's `needs:` block, and
uses brittle shell parsing for the length check. This ADR finalises the
decision and the workflow rewrite.

AGENTS.md "Back-compat" requires a `feat!:` subject on breaking changes
that lack a migration path or an ADR naming the cutoff version
(ADR 0120). That rule is a human-discipline rule the gate cannot
second-guess. The gate enforces the syntactic shape, not whether the
`!` is justified; AGENTS.md keeps the policy.

## Decision

Every commit subject on a pull request to `main`, every commit on a
direct push to `main`, and the PR title itself MUST match the
[Conventional Commits 1.0.0] specification:

```
<type>[!][optional scope][!]: <description>
```

The type whitelist is the spec's full set: `feat`, `fix`, `docs`,
`style`, `refactor`, `test`, `chore`, `perf`, `ci`, `build`, `revert`.
The regex accepts `feat!:` (single `!` after the type, no scope) and
`feat(scope)!:` (single `!` after the scope). Description is non-empty;
no requirement on its case or trailing punctuation. Lines starting with
`Merge ` are skipped to allow GitHub-generated merge commits.

Enforcement lives in three places, all wired to one source of truth:

1. **`scripts/commitlint.sh`** — a single bash script that reads
   subjects from stdin and exits non-zero on any non-conforming
   subject. The regex, the merge-commit skip, and the error message
   live in one place.
2. **The `commit-lint` job in `.github/workflows/ci.yml`** — calls the
   script on every `pull_request` (opened, synchronize, reopened),
   `merge_group`, and `push` to `main`. The job is added to the
   `needs:` block of the `ci` aggregate in the same file, so the
   existing required-check wiring on `main` (the `ci` status,
   recorded in `docs/releasing.md` step 0) gates the merge. The job
   lives in `ci.yml` rather than a separate workflow file because
   `actionlint` is per-file and cannot validate cross-workflow
   `needs:` references; the existing `windows-archive` job follows
   the same in-file pattern. The job inherits the parent workflow's
   triggers, which already include `pull_request` (any type except
   `edited`), `merge_group`, and `push: main`; the `edited` body-edit
   trigger is not separately disabled because the parent workflow
   does not subscribe to it.
3. **`.lefthook.toml` `[commit-msg]`** — calls the same script locally
   before push. Bypassable by an individual contributor, but not by CI.

A `mise run commitlint` task is added so manual verification runs the
same script the gate runs. `.mise/config.toml` already wires
`postinstall = "lefthook install"` at line 120, so the hook is installed
by `mise install` with no extra wiring.

[Conventional Commits 1.0.0]: https://www.conventionalcommits.org/

## Consequences

What becomes easier:

- A non-conforming subject cannot reach `main`, regardless of which tool
  or agent wrote it. The failure mode of "release-please silently ignored
  this commit" becomes a red CI check, not a quiet loss.
- Documentation has a single source of truth: this ADR plus
  `scripts/commitlint.sh`. Every other reference (AGENTS.md,
  CONTRIBUTING.md, docs/development.md, docs/bootstrap.md, the PR
  template, the managed agent contract) points at the gate, not at a
  moving description of the rule.
- A future contributor who adds a regression — for example, a new
  client adapter that emits a `fix:` subject — sees the gate enforce
  the same rule across the codebase, with one signal and one
  remediation path.

What becomes harder:

- The release-please-managed PR (`chore(release): x.y.z`) lands via
  `GITHUB_TOKEN` and its checks sit at `action_required` until approved
  (`docs/releasing.md` step 3). The `commit-lint` workflow now
  participates in that approval gate. No new operational step: the
  existing approval step covers it.
- A future tightening of the spec (e.g. dropping `style` to match
  release-please's changelog-sections) requires either a regex change
  plus an ADR, or a release-please-side change. Both are ADR-shaped
  work; this ADR does not constrain that.
- The managed contract in `.github/copilot-instructions.md` is
  generated by `koment bootstrap` from `agentpolicy.Contract()`. This
  PR does not extend the contract; a follow-up ADR (mirroring the
  pattern ADR 0124 set for the comment rule and RFC 2119 keywords)
  will add the commit rule to `agentpolicy.Contract()` so it lands
  on every managed bootstrap. Until that lands, Copilot and other
  agents talking to this instruction file see the comment rule but
  not the commit rule; the gate in CI and the hook in lefthook still
  enforce it.

What this commits us to:

- The CI aggregate `ci` includes `commit-lint` in its `needs:` list
  (`.github/workflows/ci.yml` line 497–505). Any future change that
  removes the gate from `needs:` is itself an ADR.
- `scripts/commitlint.sh` is the single source of truth. The
  workflow, the lefthook hook, and `mise run commitlint` all call it.
  Editing the regex in one place and not the other is not supported.
- The Conventional Commits 1.0.0 specification is the rule of record.
  The whitelist is the spec's, not release-please's. Types
  release-please ignores (`style`, `revert`) still pass the gate; the
  changelog consequence is release-please's policy, not this ADR's.
- `feat!:` is the supported breaking marker. Removing it would break
  release-please's contract under `bump-minor-pre-major: false`
  (ADR 0120).

## Alternatives rejected

- **Hand-rolled bash regex inside the workflow (no shared script).**
  Drift between CI and local. The first time someone fixed a false
  positive in the workflow but forgot the lefthook hook (or vice
  versa) would re-introduce the failure mode this ADR removes.
  Rejected.
- **Marketplace action (`action-semantic-pull-request`,
  `wagoid/commitlint-github-action`).** Either pins the project to
  Node (the previous draft's defect) or adds a third-party action whose
  pin and upgrade cadence are outside the repo's control. The repo's
  toolchain is native binaries (`actionlint`, `golangci-lint`, `helm`,
  `helm-docs`, `zizmor`, `lefthook`, `govulncheck`, `go`). Adding Node
  for a one-line regex is unjustified.
- **`commitlint` (`@commitlint/config-conventional`).** Same Node
  objection, plus an additional config file that needs to stay in sync
  with the regex. Rejected for the same reason.
- **Separate required check (`commit-lint`) on `main`, not rolled into
  `ci`.** Splits the aggregate signal documented in
  `docs/releasing.md` step 0. Two required checks mean two truths to
  keep aligned; ADR 0125 records the cost. Rejected.
- **Separate workflow file for `commit-lint`.** `actionlint` is
  per-file and cannot validate cross-workflow `needs:` references,
  so a separate `.github/workflows/commit-lint.yml` referenced from
  `ci.yml`'s `needs:` would fail `mise run workflow-lint`. The
  in-file pattern (`windows-archive` lives in `ci.yml`) is the
  existing precedent. Rejected.
- **Enforce only the PR title (or only the squashed merge commit).**
  Per-commit enforcement is what `release-please` actually consumes on
  rebase-and-merge; a PR with five commits of which one is
  non-conforming produces a misleading changelog if only the title is
  checked. The spec is about subjects, not titles.
- **Add the lowercase description and 89-char cap that the previous
  draft specified.** Not in the Conventional Commits 1.0.0 spec. Adds
  maintenance burden for zero operational benefit. Rejected.
- **Drop `style` and `revert` from the whitelist to match
  release-please.** The gate enforces the spec, not release-please's
  changelog-sections. A `style:` commit is a conforming subject; the
  fact that release-please ignores it is a downstream concern. The
  previous draft listed both, and the spec lists both; the gate does
  too.
- **Drop the `feat!:` clause.** The previous draft said "the `!`
  convention for breaking changes is not used in this repository," then
  the same paragraph said "breaking changes are gated by the `feat!:`
  prefix." Contradictory. release-please is configured
  (`bump-minor-pre-major: false`, ADR 0120) to treat `!` as a major
  bump; removing it would break release-please's contract. The `!` is
  the supported marker.
- **Push-only via a commit-msg hook, not CI.** A local hook is
  bypassable and not inherited by cloned repositories. A CI gate behind
  branch protection is the only authoritative enforcement mechanism
  (mirrors the rationale ADR 0108 records for the comment-policy
  gate). The local hook is implemented alongside, not instead of, the
  CI gate.
