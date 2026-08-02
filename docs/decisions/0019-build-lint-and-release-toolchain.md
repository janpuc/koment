# 0019 — Build, lint and release toolchain

Date: 2026-08-02
Status: Accepted

## Context

koment has one CI job that runs `gofmt`, `vet`, `go test` and `koment check`.
That is a reasonable floor and it is now the ceiling too: no linter beyond vet,
no static analysis, no race detector, no released artefacts, no versioning.
Anyone wanting to run koment builds it from source.

ADR 0010 requires every dependency to earn an ADR. Everything below is
*development and release infrastructure* rather than a module the binary links,
which is a materially lower bar — none of it ships to a user, and dropping any
of it costs a workflow file rather than a rewrite. Recording the choices anyway,
because "why is there a `.golangci.yml` with these linters" is a question a
future reader will ask.

The reference is `home-operations/konflate`, inspected 2026-08-02: release-please
for versioning, `extra-files` to bump the Helm chart in lockstep, SHA-pinned
actions, mise for tool versions, GHCR multi-arch images with SBOM.

## Decision

**Linting — golangci-lint.** Beyond vet: `staticcheck` for real bug classes,
`errcheck` because AGENTS.md §5 forbids swallowing errors and a human reviewer
will not catch every one, `gosec` because the project now ships a network
listener, plus the formatting and simplification linters. Configuration is
checked in, so a local run and CI cannot disagree.

**Testing — race detector and coverage.** `go test -race` matters now that
there are two servers and an `http.Handler` shared across goroutines. Coverage
is reported, not gated: a threshold buys a number, and the tests that matter
here are the ones asserting behaviour, which a percentage cannot see.

**Versioning — release-please, `release-type: simple`.** Conventional commits
are already required by AGENTS.md §8, so the changelog and the version fall out
of work already being done. `extra-files` bumps the Helm chart's `version` and
`appVersion` in the same release commit, which is the specific problem that
otherwise produces a chart pointing at an image tag that does not exist.

**Distribution — GHCR, multi-arch, SBOM.** `linux/amd64` and `linux/arm64`,
because the author's cluster is arm64 and most CI is amd64. The image is
distroless and non-root: koment reads files and serves HTTP, and needs no shell
to do it.

**Packaging — a Helm chart at `charts/koment`.** Published to the same GHCR
registry as an OCI artefact, so there is no chart repository to host. Validated
in CI with `helm lint`, `helm template` and `kubeconform`, because a chart that
is never rendered is a chart that is broken.

**Action pinning — by SHA, not tag.** A tag is mutable and CI has a token.

## Consequences

- Bugs that vet does not see get caught, and `-race` covers the concurrency the
  HTTP servers introduced.
- Releases become a merge rather than a ritual, and the chart cannot drift from
  the image it deploys.
- **A container image and a Helm chart imply a deployment model that does not
  fully exist yet** (ADR 0016, ADR 0017). What ships today is the read-only
  server: `koment ui` and `koment mcp --http`. The chart deploys that and
  nothing more. It must not grow a write path before authentication exists —
  ADR 0011's condition applies to the chart as much as to the code.
- More CI to keep green, and linters that will occasionally be wrong. Rules get
  disabled in `.golangci.yml` with a reason, never with a blanket `//nolint`.
- SHA-pinned actions need Dependabot to move them, which is already configured.
- The toolchain is now a thing that can rot independently of the code. It is
  confined to `.github/`, `.golangci.yml` and `charts/` — deleting all of it
  would leave a working project.

## Alternatives rejected

- **`go vet` alone, as today.** Zero configuration and zero false positives.
  Rejected because it does not catch unchecked errors, and §5 makes those a
  correctness issue here rather than a style one.
- **Tag releases by hand.** No tooling, full control. Rejected because it drifts
  from the changelog and forgets the chart bump — the failure being designed
  against.
- **GoReleaser for binaries.** The standard answer, and it would give
  per-platform archives and checksums. Deferred rather than rejected: `go
  install` covers the Go audience and the container covers deployment, so
  binaries can wait until someone asks. Adding it later changes one workflow.
- **A `gh-pages` chart repository.** Conventional and works with plain `helm
  repo add`. Rejected because OCI needs no hosting, no index to regenerate, and
  no second branch, and the images are already going to GHCR.
- **Gating on a coverage threshold.** Common, and it makes the number visible in
  reviews. Rejected because it rewards tests that execute code over tests that
  assert behaviour, and this project's tests are deliberately the second kind.
