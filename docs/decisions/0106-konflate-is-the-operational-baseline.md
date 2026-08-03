# 0106 — Use konflate as the operational baseline

Date: 2026-08-03
Status: Accepted

## Context

koment and `home-operations/konflate` are small Go services shipped as static
binaries, containers and Helm charts. The project owner treats konflate as the
gold standard. The v0.2 repository adopted SHA-pinned Actions, a distroless
image, race tests, golangci-lint and basic chart validation, but tool versions,
workflow security, Helm E2E and artifact signing remain inconsistent.

The reference inspected for this reset is konflate commit
`0bfd78869bd920269ef138a0a06156d00bfe337a` from 2026-08-02. A baseline is a
source of defaults, not permission to copy features or comments that conflict
with koment's product.

## Decision

Adopt konflate's operational shape:

- mise and its lock file are the source of truth for Go and project tools;
- Lefthook provides shared local checks;
- Renovate keeps toolchain, Actions, images and chart dependencies current;
- CI runs formatting, tidy, vet, race tests, lint, govulncheck, Actionlint,
  Zizmor, generated-file checks and one aggregate status;
- the chart has a values schema, generated documentation, unit tests, a Kind
  installation and `helm test` against the image built from the change;
- containers run static binaries as non-root with an exact Go patch version;
- images, charts, binaries and checksums are digest-addressable and signed;
- workflow Actions and downloaded executables are pinned and authenticated.

Retain koment-specific differences: no Node frontend, race testing remains
mandatory, metrics use a separate listener, and implementation rationale moves
to annotations or ADRs rather than explanatory inline comments.

## Consequences

- Local and CI commands share pinned versions and task names.
- Operational changes become larger initially because the existing workflows
  and chart are replaced rather than patched piecemeal.
- Renovate and generated-file checks add routine maintenance pull requests.
- Helm behaviour is tested by installation rather than inferred from rendering.
- Updating the baseline still requires inspecting current konflate files; this
  ADR does not freeze commands from the audited commit forever.

## Alternatives rejected

- **Keep bespoke Actions and developer commands.** Fewer files, but versions and
  behaviour already differ across Docker, CI and local machines.
- **Copy konflate wholesale.** Fastest apparent parity, but imports its Svelte
  toolchain, product-specific chart surface and inline-comment conventions.
- **Use mutable tool and Action tags.** Convenient updates, but CI and release
  credentials make mutable executable references an avoidable supply-chain
  risk.
- **Validate Helm only with lint and template.** Syntax can pass while probes,
  startup, permissions or the selected application mode fail at runtime.
