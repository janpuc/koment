# 0109 — Distribute authenticated artifacts instead of Go invocations

Date: 2026-08-03
Status: Accepted

## Context

koment is written in Go, but the Go toolchain is an implementation detail. The
current README and several integration guides tell end users to use `go install`
or `go run` when a binary is unavailable. That creates slow, mutable builds,
binds installation to an unrelated development toolchain and bypasses the
checksums, signatures, SBOMs and provenance produced for releases.

koment also spans more than a CLI catalog. Humans should discover it through
their operating-system package manager and editor. Agents should discover the
same product through their marketplace or the MCP Registry. Publishing an npm
or Python wrapper solely to satisfy discovery would introduce a runtime and a
second supply chain without improving the Go binary.

The relevant channel contracts are documented by the
[VS Code Marketplace](https://code.visualstudio.com/api/working-with-extensions/publishing-extension),
[Open VSX](https://www.eclipse.org/legal/open-vsx-registry-faq/),
[Claude plugin marketplaces](https://code.claude.com/docs/en/plugin-marketplaces),
[mise GitHub backend](https://mise.jdx.dev/dev-tools/backends/github.html),
[Homebrew taps](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap),
[WinGet](https://learn.microsoft.com/en-us/windows/package-manager/package/)
and the [MCP Registry](https://modelcontextprotocol.io/registry/package-types).

## Decision

Treat the platform archives attached to a GitHub Release as the canonical CLI
artifacts. Treat GHCR as canonical for the container and Helm chart. A release
publishes checksums, signatures, SBOMs and provenance before any downstream
channel is updated.

End-user documentation, CI examples, agent configuration and editor setup must
never require `go install`, `go run` or a Go build container. Contributor-only
development documentation and repository tasks may use the pinned Go toolchain.

Promote each canonical release through:

- GitHub Releases and the setup Action;
- a maintained Homebrew tap, mise's GitHub backend and an Aqua/mise registry
  entry;
- WinGet and Scoop manifests for Windows;
- GHCR images and Helm OCI artifacts;
- a koment Claude marketplace followed by submission to the official plugin
  directory;
- the official MCP Registry using the labeled OCI image or a checksummed MCPB
  artifact;
- the VS Code Marketplace and Open VSX once the editor extension exists; and
- community catalogs such as Homebrew core, Nixpkgs, AUR and MacPorts when the
  project meets their acceptance and maintenance requirements.

Generate downstream manifests from one release version and checksum manifest.
Test each advertised installation path on its native operating system. External
catalogs that require review remain planned until accepted; documentation must
not present a submitted package as available.

## Consequences

- A user never needs Go to install or run koment.
- Agent and editor packages remain thin integration layers around the same
  versioned product.
- Release publication is ordered: canonical artifacts first, distribution
  metadata second.
- Registry credentials and publisher agreements become release-operational
  prerequisites and must use scoped automation identities where available.
- Some community channels can lag a release because their review belongs to an
  external project.
- The repository must test archive naming and contents as a public API because
  multiple package managers derive platform selection from them.

## Alternatives rejected

- **Keep `go install` as the universal fallback.** Easy for maintainers, but it
  pushes compilation, toolchain trust and version resolution onto every user.
- **Publish separate binaries from each package manager.** Native-looking, but
  allows channels to distribute non-identical builds with different provenance.
- **Wrap the binary in npm or PyPI solely for MCP discovery.** Gains a registry
  name at the cost of another runtime and supply chain; OCI and MCPB are already
  supported package types.
- **Publish only GitHub Releases.** Canonical artifacts remain necessary, but
  discovery and updates should meet humans and agents in the tools they already
  use.
- **Claim every submitted community package immediately.** Optimistic but
  false; acceptance and publication are controlled by external maintainers.
- **Use an unauthenticated curl-to-shell installer.** Short to document, but it
  combines download, trust and execution without a package manager or explicit
  artifact verification boundary.
