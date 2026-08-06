# Security policy

## Reporting a vulnerability

Report privately through GitHub:
<https://github.com/koment-dev/koment/security/advisories/new>.

Do not open a public issue, and do not describe the problem in a pull request
or a discussion. A koment repository contains the reasoning behind a codebase,
so a flaw that leaks or forges an annotation leaks or forges what a team
believes about its own code.

Please include the koment version (`koment version`), how koment was installed,
and the smallest reproduction you have. You will get an acknowledgement within
three working days and a decision — fix, mitigation, or "not a vulnerability,
here is why" — within fourteen.

## What is in scope

- The `koment` binary and every command it exposes.
- The MCP server, the language server, and the HTTP surface.
- The published container image, Helm chart, and release archives, including
  their signatures and provenance.
- The VS Code and Open VSX packages published from this repository.
- The agent policy gate — specifically, any way to land an inline comment or an
  annotation that `koment comments check` or `koment check` should have
  rejected.

## What is not

- Vulnerabilities in a dependency that koment does not reach. Report those
  upstream; `mise run vulncheck` is what koment uses to decide reachability.
- Anything that requires an attacker who can already write to the repository or
  to `.koment/`. koment trusts its own repository; Git is the trust boundary.
- Denial of service against a `koment serve` instance exposed to the public
  internet. It is designed for loopback and authenticated networks (ADR 0105).

## Supported versions

koment follows semantic versioning from 1.0.0. Only the latest published
release receives fixes; there are no maintenance branches. A security fix ships
as the next patch release through the normal published pipeline, never as a
hand-built artifact and never by re-pointing an existing tag.

| Version | Supported |
|---|---|
| latest 1.x | yes |
| 0.x | no — upgrade; records are rewritten in place on first read |

## Verifying what you install

Every release artifact is signed with Sigstore and carries build provenance.
Verification steps for the binaries, the container image and the chart are in
[docs/publishing.md](docs/publishing.md). If a signature does not verify, treat
the artifact as hostile and report it here rather than installing it.
