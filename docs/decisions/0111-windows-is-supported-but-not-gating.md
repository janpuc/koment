# 0111 — Ship Windows without letting it gate the pipeline

Date: 2026-08-04
Status: Accepted

## Context

ADR 0109 promises to test each advertised installation path on its native
operating system. koment builds `windows/amd64` and `windows/arm64` archives,
and both the Scoop manifest and the WinGet bundle download them, so Windows is
advertised and must be exercised.

Windows is nonetheless the platform koment is least used on and the one its
maintainers cannot reproduce locally. The setup Action refuses Windows runners
outright and points at the checksum-listed archive instead. The Windows job can
only exercise the last published release, because no Windows runner builds the
pull request's own binary. Its failures are therefore as likely to describe a
runner image change or a release that has not happened yet as a defect in the
change being reviewed.

A required check that a reviewer cannot reproduce and does not know how to fix
is a check people learn to re-run until it passes. That is worse than no check,
because it teaches the team to dismiss red.

## Decision

Windows is a supported platform and a second-class one.

The `windows-archive` job runs on every pull request, is named advisory, and is
deliberately absent from the aggregate `test` job's `needs` list. It downloads
the published Windows archive, verifies it against the release checksum
manifest, expands it and runs `koment.exe version`. Its result is visible on
every pull request and blocks nothing.

Linux and macOS keep parity: both are covered by the setup Action matrix, which
does gate.

Windows keeps everything that costs nothing to keep — archives, checksums,
signatures, Scoop and WinGet manifests, and the naming contract enforced by
`packaging`. What it loses is the power to stop a merge.

## Consequences

- A Windows regression is reported, not enforced; someone has to read the job.
- Reviewers are never blocked by a platform they cannot reproduce.
- The advisory result is only as current as the last release, so a Windows
  packaging defect introduced by a change is found after that change ships.
- If Windows becomes a primary platform, promoting the job is one line, and the
  amendment belongs in a superseding decision rather than an edit here.
- ADR 0109's testing promise is now explicitly uneven, and this document is the
  record of where.

## Alternatives rejected

- **Keep the job required.** Honest about the promise, but makes an
  irreproducible, release-lagging check able to block unrelated work; the first
  false red would get it removed anyway, with less thought than this.
- **Delete the job.** Cheapest, and it is what "second class" is often taken to
  mean, but it would leave two shipped archives and two package manifests with
  nothing ever executing them.
- **Mark the job `continue-on-error: true`.** Equivalent effect, but it reports
  green when it failed, so the signal is destroyed rather than downgraded.
  Absence from `needs` keeps the red visible.
- **Build the pull request's own Windows binary on a Windows runner.** Removes
  the release lag, but stops testing the artifact users actually download, which
  is cross-compiled from Linux and is the thing Scoop and WinGet fetch.
- **Drop Windows entirely.** Smallest surface, but Scoop and WinGet manifests
  already exist and a Go static binary costs almost nothing to cross-compile;
  removing support would be a product decision, not a CI one.
