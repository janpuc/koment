<p align="center">
  <img src="https://raw.githubusercontent.com/koment-dev/koment/main/internal/ui/assets/koment-logo.png" alt="koment comment bubble" width="104">
</p>

# koment for VS Code

Read and write koment annotations where the related code lives. The extension
renders annotation prose as virtual inline text, adds hover and code-lens views,
and reports drifted anchors or prohibited comments as editor diagnostics. It
never inserts the rendered prose into the source file.

When newly written explanatory comment prose is saved, the extension offers to
convert it into a durable annotation. Commented-out Go code is left alone. A
comment that truly has to remain inline requires a modal acknowledgement and an
exact exception record, so it cannot silently bypass the repository policy.

## Requirements

Install a released `koment` binary and make it available on `PATH`. Set
`koment.binaryPath` when the binary lives elsewhere. The extension starts
`koment lsp` inside the workspace and does not download or execute a replacement
binary.

The workspace must contain `.koment/policy.yaml`. Multi-root workspaces are
supported; each source file discovers and mutates only its own repository.

## Commands

- `koment: Add annotation` — `⌘⌥K` on macOS, `Ctrl+Alt+K` elsewhere.
- `koment: Reanchor annotation` — refresh a moved or repaired anchor without
  changing its identity.
- `koment: Convert comment to annotation` — available as a diagnostic quick fix.
- `koment: Keep inline comment with acknowledgement` — available only through
  the explicit exception flow.

The CLI, MCP, browser and editor all use the same records and deterministic
anchor resolution. Source and support: [koment-dev/koment](https://github.com/koment-dev/koment).
