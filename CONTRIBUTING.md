# Contributing

Thanks for considering a contribution to koment. The rules below are short on
purpose — the rest of the project's reasoning lives in `DESIGN.md`,
`docs/decisions/` and `.koment/` annotations.

## Before you open a pull request

1. Read [`AGENTS.md`](AGENTS.md). It is binding on every contributor, human
   or agent.
2. Read [`DESIGN.md`](DESIGN.md) and the ADRs that touch the area you are
   changing. Every non-obvious decision has an ADR; if yours is not obvious
   either, write one alongside the code.
3. Search `.koment/` and `docs/decisions/` for prior rationale on the file or
   behaviour you are about to touch. Run `./koment show <file>`.

## Contributor Licence Agreement

The project is dual-licensed. Every contributor must sign a
[CLA](CLA.md) before a pull request can be merged. The CLA grants the project
the right to license the contributed code under both:

- the GNU Affero General Public License, version 3 or later;
- a commercial licence offered to organisations whose policy excludes AGPL.

Without the CLA, the project cannot legally sell commercial licences of code
that includes your contribution, which closes the revenue door that
[ADR 0117](docs/decisions/0117-relicense-to-agpl-with-commercial-dual-licensing.md)
keeps open.

Sign by posting a comment on your pull request beginning with the phrase in
[CLA.md §7](CLA.md#7-how-to-sign). The `cla` status check turns green once the
comment is recorded, and a red `cla` blocks the merge button. It is not in the
repository's required-checks list, because a release pull request is opened by
`GITHUB_TOKEN` and therefore never gets a `cla` run at all; requiring it would
make every release unmergeable. The maintainer and any `[bot]` account are
exempt.

## Code rules

- Format with `mise run fmt`. The CI gate will reject anything else.
- Run the full local gate before pushing:
  ```sh
  mise run build
  mise run vet
  mise run test
  mise run lint
  mise run annotations
  mise run comments
  mise run agent-policy
  ```
- No new explanatory inline comments. Rename, extract, introduce a named type
  or constant, restructure, then use `koment add`. Inline comments survive only
  through an explicit `koment comments acknowledge`.
- One concern per commit. Conventional Commit subjects (`feat:`, `fix:`,
  `docs:`, `refactor:`, `test:`, `chore:`) decide how release-please turns
  merged work into a version. Use `!` after the type for breaking changes; from
  1.0.0 that produces a major version.
- A claim that a change is backward compatible needs evidence: a migration the
  binary performs, or an ADR naming the version the old shape was cut off at.
  Without either, the change is breaking and its subject says `feat!:`.
- Do not edit `go.mod` or `.release-please-manifest.json` versions by hand.
  Release-please owns those files; `packaging` and the chart will fail the
  build if they disagree.

## Reporting a security issue

Read [SECURITY.md](SECURITY.md). Report privately through a GitHub security
advisory at <https://github.com/janpuc/koment/security/advisories/new>. Never
in a public issue, a pull request or a discussion.
