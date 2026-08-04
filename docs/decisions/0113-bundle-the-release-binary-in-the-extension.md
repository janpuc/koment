# 0113 — Bundle the released binary in the extension

Date: 2026-08-04
Status: Accepted

## Context

ADR 0112 named this the target state and left it unbuilt. The extension started
`koment` from `PATH`, so installing it from a marketplace produced a listed,
activated extension that could do nothing until the user separately installed a
CLI. The failure appeared as a server that would not start, which is the least
diagnosable shape for the audience least equipped to diagnose it.

VS Code supports platform-specific extensions. Publishing a package per target
lets the marketplace hand each user the one build that runs on their machine,
and a package published with no target "will be used as a fallback for all
platforms that have no platform-specific package".

koment already cross-compiles six binaries, and the VS Code target identifiers
map onto them exactly: `linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`,
`win32-x64` and `win32-arm64`. There is no koment platform without a VS Code
target and none the other way.

ADR 0109 rejected letting channels build their own binaries, because that lets
one channel distribute something other than what was signed and attested. A
bundled binary is a distribution channel, so it must carry the canonical
artifact rather than a rebuild that merely ought to be identical.

## Decision

Each release publishes seven extension packages: one per platform target,
carrying that platform's koment binary at `bin/koment` (`bin/koment.exe` on
Windows), and one universal package carrying none.

The bundled binary is extracted from the release archive and verified against
the release checksum manifest before packaging. The extension job depends on the
binaries job for exactly this reason: it consumes that job's output rather than
compiling its own.

The extension resolves its server in one order:

1. `koment.binaryPath`, when set to a non-empty value;
2. the bundled binary, when the installed package carries one;
3. `koment` on `PATH`.

`koment.binaryPath` now defaults to empty rather than `"koment"`, because a
default that is also a valid path cannot be distinguished from a choice.

Before spawning a bundled server the extension asserts its executable bit. A
VSIX is a zip and nothing guarantees the mode survives every extraction path; a
server that cannot be executed is indistinguishable from one that is missing.

Every package is cosign-signed and attached to the GitHub release before any
marketplace push, so the release remains canonical and each marketplace is a
mirror.

## Consequences

- Installing the extension is a complete installation on the six supported
  platforms.
- A release carries seven extension artifacts and seven signature bundles
  instead of one, each around five megabytes.
- Users on `alpine`, `linux-armhf` and any future platform receive the universal
  package and still need a binary on `PATH`; the extension says which one it
  started and where it came from.
- The extension and the binary can no longer disagree about version, because
  they ship in the same file.
- Release publication is more serial: the extension cannot be built until the
  binaries exist.
- Marketplace review has more bytes to scan, and a large binary in a VSIX draws
  a size warning that is expected rather than a defect.

## Alternatives rejected

- **Keep requiring a separately installed CLI.** Smallest artifact and no new
  packaging, but it is the status quo whose failure mode motivated this decision.
- **Download the binary on first activation.** One small package, and it is what
  several language extensions do, but it moves an authenticated download into
  the editor process, needs its own checksum and signature verification there,
  and fails in the proxied and air-gapped environments where a bundled binary
  still works.
- **Rebuild the binary inside the extension job.** Avoids downloading the
  release archives and is a few lines shorter, but it makes the marketplace
  distribute a build that was never signed or attested, which ADR 0109 rejected
  precisely to prevent.
- **Publish only the universal package.** One artifact and no matrix, but it
  keeps the broken first-install experience for every user in exchange for
  simplicity in the release workflow.
- **Ship only platform-specific packages, with no universal fallback.** Slightly
  fewer artifacts, but it makes the extension silently uninstallable on any
  platform not in the matrix rather than degraded on it.
- **Distribute the binary through npm and depend on it.** Reuses an existing
  package manager, but adds a second supply chain and a runtime the product
  otherwise does not need, which ADR 0109 already rejected for MCP discovery.
