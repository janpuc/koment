# 0112 — Publish one editor package per marketplace, and configuration everywhere else

Date: 2026-08-04
Status: Accepted

## Context

ADR 0110 put annotation semantics behind `koment lsp`, so an editor integration
holds no record validation, anchoring or comment classification of its own. That
decision has an unexamined consequence for distribution: most editors need no
koment artifact at all. Helix, Neovim, Emacs, Sublime Text and Kate configure a
language server in a few lines of user configuration, and a published package
would add a release channel without adding a capability.

What LSP cannot express is the part that makes koment feel native: rendering an
annotation body beside the line it belongs to without inserting it into the
buffer, gutter status per resolution state, and the prompt that appears when a
freshly typed explanatory comment should become an annotation. Those need editor
APIs, so they need a package, and a package needs a marketplace to live in.

The reference VS Code extension already exists and is published to the VS Code
Marketplace and Open VSX. It is a thin client: it starts `koment lsp`, renders
decorations and forwards commands.

Two properties of the current pipeline conflict with ADR 0109. The `editor` job
depends only on `please`, so it can publish an extension in parallel with — or
despite the failure of — the `binaries` job that produces the canonical release
artifacts. And each marketplace step is guarded by `if: env.VSCE_PAT != ''`, so
an absent credential silently skips publication and the release still reports
success.

The extension also does not remove the dependency it was meant to remove.
`koment.binaryPath` defaults to `koment`, so a user who installs only the
extension has no working integration until they separately install the CLI.

## Decision

Treat editor support as two populations with different distribution costs.

**Packaged.** An editor gets a package in this repository only when it has a
marketplace *and* offers APIs that LSP cannot reach. Today that is VS Code,
whose package also serves Cursor, Windsurf and VSCodium through Open VSX. Adding
another packaged editor is a decision recorded in its own ADR, not a directory
someone adds.

**Configured.** Every other editor receives documentation: the exact
`koment lsp` invocation and a configuration snippet, kept in `docs/editors/`.
These are not published, not versioned and not signed, because there is no
artifact to publish, version or sign.

Editor packages carry the repository version. `release-please` already rewrites
`editors/vscode/package.json`, and the release job asserts equality before
packaging. A thin client of a specific `koment lsp` must not invite a
compatibility matrix between itself and the binary it speaks to.

Publication follows ADR 0109's order without exception. The `editor` job depends
on `binaries`, so no marketplace is updated for a release whose canonical
artifacts did not build. The VSIX is cosign-signed and attached to the GitHub
release before any marketplace push, making the release the canonical source and
the marketplace a mirror.

Marketplace publication is opt-in through an explicit repository variable rather
than inferred from whether a secret happens to exist. When publication is
enabled and a credential is missing, the release fails. A skipped publication is
a decision someone recorded, never an accident nobody noticed.

The packaged extension will carry the koment binary for its platform, built from
the same release, using platform-specific VSIX targets, with `koment.binaryPath`
retained as an override. Installing the extension is then a complete
installation. This is the target state; until it lands the extension continues
to require a separately installed binary, and `DESIGN.md` says so.

## Consequences

- Most editors are supported at the cost of a documentation page.
- One version spans the binary, the LSP and every editor package.
- A failed `binaries` job now also blocks the extension, making releases more
  serial and slower.
- Enabling marketplace publication becomes a deliberate repository change, and
  the first enablement needs the Open VSX namespace to exist already.
- Bundling multiplies release artifacts by the number of supported platforms and
  makes the extension large, which is the accepted price for an install that
  works on its own.
- Editors in the configured population get no inline annotation rendering; the
  gap between the two populations is real and is not hidden.

## Alternatives rejected

- **Publish a package for every editor.** Maximum reach on paper, but each
  marketplace has its own account, review, signing and deprecation policy, and
  an unmaintained plugin damages trust more than an absent one. LSP already
  covers those editors.
- **Version editor packages independently.** Allows an extension fix without a
  koment release, but creates a compatibility matrix between extension and
  binary that every bug report would have to establish first.
- **Let the extension download the binary on first activation.** Smaller VSIX
  and one artifact, but it moves an authenticated download into the editor
  process, needs its own checksum verification, and fails in exactly the
  air-gapped and proxied environments where a bundled binary still works.
- **Keep requiring a separately installed CLI.** Simplest to build, and it is
  the status quo, but it leaves the integration broken on first install for the
  audience least likely to diagnose it.
- **Infer publication from the presence of a secret.** Convenient, and it is
  what the pipeline does today, but it makes "nothing was published" and
  "publication succeeded" indistinguishable in a green release.
- **Publish editor packages outside the release workflow.** Decouples a slow
  marketplace from the release, but breaks the ordering ADR 0109 exists to
  guarantee and would let a marketplace carry a version the release never
  produced.
- **Move decorations into the language server.** Would let one implementation
  serve every editor, but LSP has no inline-decoration surface, so it would mean
  a private protocol extension that only koment's own clients understand — which
  is the duplication ADR 0110 rejected, relocated.
