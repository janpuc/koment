# 0127 — OpenCode plugin as a first-class installable

Date: 2026-08-07
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
parallel to Claude — and whether that plugin should be published to npm so
that `opencode plugin install @koment/opencode-koment` works the same way
`npm install` does for any other Node dependency. The previous draft of this
ADR said "no": the plugin would live only in the Git repository, installable
from the Git reference but not registered. That decision predated the
realisation that a user evaluating koment from inside an existing OpenCode
session has no one-command installation path that does not also ask them to
clone, bootstrap, or otherwise prepare the repository before the policy is
enforced.

The requirement is therefore a first-class installable:

1. `opencode plugin install @koment/opencode-koment` lands on npm and the
   user has the policy gate active without any other preparation step.
2. The plugin talks to `koment` exclusively through the MCP server
   (`koment mcp --write`) over stdio — no shellout to `koment agents hook
   pre-tool`, no PATH requirement for the comment gate.
3. The `dispose` session-end hook runs the three policy gates
   (`koment check`, `koment comments check`, `koment agents check`). These
   stay as CLI invocations because they are cheap and end-of-session, and
   because they do not need to be on the hot path of every edit.

## Decision

Ship an OpenCode plugin at `plugins/koment/.opencode-plugin/` and publish it
to npm as `@koment/opencode-koment` from the release pipeline. The plugin:

- `plugin.json` — manifest with name, version (matching the repository
  version), description, author, keywords, entrypoint, and registered hooks.
- `index.js` — a JavaScript module that, on session load, spawns the
  `koment` MCP server via stdio (`koment mcp --write`) and holds the
  JSON-RPC connection for the lifetime of the session. The two hooks call
  the MCP server rather than shelling out:

  - `tool.execute.before` — when the agent would `edit` or `write` a file,
    the hook calls `koment_pre_tool` on the MCP connection with the
    `tool_name` (`opencode_edit`) and `tool_input.filePath` / `tool_input.content`.
    A `deny` decision rejects the tool call; an `allow` decision passes
    through.
  - `dispose` — runs `koment check`, `koment comments check`, and
    `koment agents check` via the standard CLI invocations. A non-zero
    exit denies session completion.
- `package.json` — npm manifest with `name: "@koment/opencode-koment"`,
  `version` matching the repository version, `license: AGPL-3.0-or-later`,
  and a `postinstall` script that checks the user has `koment` on `PATH`
  and prints a one-line pointer at the install command if it is missing.
  The postinstall does not fail; it warns. The hard requirement is the
  plugin's MCP-driven hooks; the dispose CLI invocations are advisory.
- `README.md` — installation and verification instructions that surface the
  MCP-first design and the `PATH` requirement for the dispose hook.

The plugin is versioned by the repository. `release-please-config.json`
includes both `plugins/koment/.opencode-plugin/plugin.json` and
`plugins/koment/.opencode-plugin/package.json` in `extra-files` so the two
move in lockstep. The `release.yml` `plugins:` job runs `npm publish` on
the package (gated on `secrets.NPM_TOKEN`, `please.outputs.released`, and a
failing-the-build assertion that the published version matches
`please.outputs.version`).

A new MCP tool, `koment_pre_tool`, is added to `internal/mcp/` and registered
on the base server (no `--write` required — the pre-tool decision is a
read-only inspection). The tool delegates to `agentpolicy.PreToolOutput`,
the same function the CLI hook uses, so the rules live in one place. The
plugin therefore does not shell out for the pre-tool gate.

## Consequences

- OpenCode users install with one command and the policy gate is active
  the moment OpenCode loads the plugin. No clone, no bootstrap, no PATH
  dance for the hot path.
- The plugin and the generated adapter still share one JavaScript source
  for the hook logic — the difference is that the plugin reaches `koment`
  through the MCP server (auto-started on session load) while the generated
  adapter reaches it through a PATH shellout. ADR 0118's
  `apply_patch`-translation contract is unchanged; the new MCP tool
  preserves it.
- The dispose hook still shells out to the CLI. This is a deliberate
  boundary: the pre-tool decision is on the hot path of every edit and
  benefits from MCP, while the three gates are end-of-session and run
  once. Conflating them would require lifting the policy gate into the
  MCP server, which is a larger change with no immediate benefit.
- npm is a second supply chain for the plugin's JavaScript source. The
  package contents are byte-for-byte the directory on disk; the release
  pipeline's cosign-signed tarball is the alternative distribution for
  users who prefer not to depend on npm. ADR 0109's ordering (canonical
  artifacts first) is preserved: `binaries` and `image` publish before
  the `plugins` job's `npm publish` runs.
- The `@koment/opencode-koment` name places the package under the npm
  scope `@koment`, matching the publisher the marketplace rename
  (ADR 0126) uses for the VS Code extension and the GitHub organisation.
- `NPM_TOKEN` is a new required secret for the release pipeline. The
  `release.yml` `plugins:` job fails loudly if the secret is absent, the
  package version does not match `please.outputs.version`, or `npm publish`
  exits non-zero. The job does not silently swallow errors.
- The MCP tool `koment_pre_tool` is part of the read-only surface. The
  pre-tool decision does not mutate state, so the tool belongs in the
  base set rather than the write set. ADR 0127 records this; future
  readers adding hooks or checks should keep the same split.

## Alternatives rejected

- **Ship only the generated adapter.** Minimal surface area, but it
  forces OpenCode users to run `koment bootstrap` or `koment agents
  install` before the plugin is available. A user evaluating koment for
  the first time from an existing OpenCode session has no one-command
  installation path. This is what the previous draft of this ADR chose;
  the gap is the motivation for the new decision.
- **Reimplement the pre-tool classifier in pure JS inside the plugin.**
  Avoids the MCP dependency, but creates a second source of truth for the
  rules. The Go classifier (`agentpolicy.PreToolOutput`) and the JS
  classifier would drift. Lighter runtime, heavier maintenance.
- **Keep the plugin's pre-tool hook shelling out to `koment agents hook
  pre-tool` from PATH.** What the previous draft shipped. Two failures:
  the user needs `koment` on `PATH` for the hot path (not just the
  dispose hook), and the hook is a CLI invocation rather than a JSON-RPC
  call. The MCP-first design removes both.
- **Lift the three `koment check` / `comments check` / `agents check`
  gates into MCP tools and call them via JSON-RPC in `dispose`.**
  Symmetric with the pre-tool change, but the three gates are CLI-shaped
  (they print to stdout, exit non-zero on failure, and run against the
  working directory in place). Lifting them requires either a separate
  command output format or a process spawn — neither is cheaper than the
  existing shellout, and either widens the trust surface.
- **Publish to a non-npm registry.** npm is where OpenCode plugin
  consumers already look. A bespoke registry adds a one-off maintenance
  cost and breaks `opencode plugin install` ergonomics.
- **Bundle the `koment` binary into the plugin.** Violates ADR 0109's
  ordering (canonical artifacts first, wrappers second), creates a
  compatibility matrix between plugin and binary, and makes the npm
  package large. The plugin shells out to `koment` for the dispose hook
  and uses the MCP server for the pre-tool gate; the binary is not a
  runtime component.
- **Use the v2 OpenCode plugin API (Effect or Promise).** The v2 API is
  newer and offers typed drafts for agents, commands, and references, but
  the v1 hook surface (`tool.execute.before`, `dispose`) is sufficient for
  the two hooks koment needs. Adopting v2 for no added capability is a
  migration without a purpose.
