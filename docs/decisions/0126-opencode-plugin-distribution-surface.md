# 0126 — OpenCode plugin distribution surface

Date: 2026-08-06
Status: Accepted

## Context

ADR 0109 established authenticated artifact distribution over Go invocations.
ADR 0112 drew the boundary between packaged editor integrations (VS Code) and
configured ones (everything else). The Claude marketplace plugin shipped in
`plugins/koment/.claude-plugin/` is a distribution surface for agent
instructions, MCP configuration, and pre-tool/stop hooks; it is neither a
packaged editor nor a runtime artifact, but a thin integration layer around the
versioned product.

OpenCode is the second agent runtime with first-class plugin support. The
bootstrap command (ADR 0124) already generates the OpenCode adapter
(`.opencode/plugins/koment.js` and `opencode.json`), which shells out to
`koment agents hook pre-tool` and re-runs the policy gate at session idle.
That adapter is a generated file, refreshed by `koment agents install` and
drift-checked by `koment agents check`. It is not a published plugin.

The question is whether OpenCode should also receive a marketplace plugin
parallel to Claude — a directory under `plugins/koment/` with its own manifest,
entrypoint, and README, installable through `opencode plugin install` from a
Git reference rather than a copied file.

## Decision

Ship an OpenCode plugin directory at `plugins/koment/.opencode-plugin/`. It
contains:

- `plugin.json` — manifest with name, version (matching the repository version),
  description, author, keywords, entrypoint, and registered hooks.
- `index.js` — a JavaScript module exporting the same two hooks the generated
  adapter uses (`tool.execute.before` to deny ordinary Go comment intent,
  `dispose` to run the policy gate).
- `README.md` — installation and verification instructions.

The plugin is identical in substance to the generated adapter. The distinction
is distribution: a user can install it with `opencode plugin install
koment-dev/koment/plugins/koment/.opencode-plugin` from the Git reference, or
the bootstrap command can generate it locally. The generated adapter remains
the scriptable, CI-friendly surface; the plugin is a convenience for users who
prefer a registry-style installation.

Versioning follows the repository version (currently 2.0.1). Release automation
rewrites `plugin.json` alongside `editors/vscode/package.json` and the Claude
manifest. The plugin is not published to npm or any language registry; it lives
in the Git repository, which is the canonical source.

## Consequences

- OpenCode users have two installation paths: the generated adapter (default,
  from `koment bootstrap`) and the plugin directory (manual, from Git).
- The plugin and the generated adapter share one JavaScript source
  (`opencodePluginSource` in `internal/agentpolicy/agentpolicy.go`). A change to
  the hook logic ships to both surfaces in the same commit.
- The plugin is not versioned independently of the repository. A change to
  `plugin.json` or `index.js` is a repository change, surfaced through normal
  release automation.
- The plugin directory is `plugins/koment/.opencode-plugin/`, not
  `plugins/koment/.opencode/` or `plugins/koment/opencode/`, because the
  directory is a child of the Claude plugin's parent and the leading dot makes
  the nesting obvious in a flat listing.
- Release automation must rewrite `plugin.json` to match the repository version.
  This is a small addition to the existing `release-please` configuration.

## Alternatives rejected

- **Ship only the generated adapter.** Minimal surface area, but it forces
  OpenCode users to run `koment bootstrap` or `koment agents install` before
  the plugin is available. A user evaluating koment for the first time from an
  existing OpenCode session has no one-command installation path.
- **Publish the plugin to npm.** Gains a registry name at the cost of another
  supply chain, a language runtime, and a second version to keep in lockstep
  with the repository. The plugin is a thin JavaScript module, not a library;
  npm is the wrong home.
- **Bundle the binary into the plugin.** Violates ADR 0109's ordering (canonical
  artifacts first, wrappers second), creates a compatibility matrix between
  plugin and binary, and makes the plugin large. The plugin shells out to
  `koment`; it is not a runtime.
- **Use the v2 plugin API (Effect or Promise).** The v2 API is newer and offers
  typed drafts for agents, commands, and references, but the v1 hook surface
  (`tool.execute.before`, `dispose`) is sufficient for the two hooks koment
  needs. Adopting v2 for no added capability is a migration without a purpose.
- **Place the plugin at the repository root.** Separates it from the Claude
  plugin and makes the directory structure less obvious. `plugins/koment/` is
  the home for marketplace integrations; the OpenCode plugin belongs there.
