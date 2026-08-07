# @koment/opencode-koment

OpenCode plugin for koment. Install with one command and the no-inline-comment
policy is enforced through the koment MCP server for the lifetime of every
session.

## Install

```sh
opencode plugin install @koment/opencode-koment
```

That's it. The plugin starts `koment mcp --write` over stdio on session load
and keeps the connection open. The pre-tool hook calls `koment_pre_tool` on
that connection, so the policy gate is active on the hot path of every edit
without a `koment` binary on `PATH`.

## Requirements

- Node.js 18 or newer
- OpenCode 0.4 or newer
- The `koment` binary on `PATH` for the session-end dispose hook, which runs
  `koment check`, `koment comments check`, and `koment agents check`. The
  `postinstall` script warns if the binary is missing. The plugin still
  loads; the pre-tool MCP gate does not need the binary.

## Install the `koment` binary

```sh
# macOS / Linux
brew install koment-dev/koment/koment
# or download a release archive from https://github.com/koment-dev/koment/releases
```

## What it does

1. **Session load** — spawns `koment mcp --write` and holds the JSON-RPC
   connection. The MCP server already ships with the strict repository
   procedure as its server instructions, so OpenCode sees the agent contract
   in the same session as the tools.
2. **`tool.execute.before`** — intercepts `edit` and `write` tool calls. When
   the proposed content adds ordinary Go comment intent, the hook calls
   `koment_pre_tool` on the MCP connection and denies the operation,
   suggesting `koment_add` instead.
3. **`dispose`** — runs `koment check`, `koment comments check`, and
   `koment agents check` at session end. A non-zero exit denies session
   completion.

## Verification

A repository must have `.koment/policy.yaml` for the policy gate to fire.
Bootstrap one with:

```sh
koment bootstrap --agents opencode --non-interactive
```

Once the repository is bootstrapped:

```sh
koment check
koment comments check
koment agents check
```

All three must pass before the session completes.

## Why a plugin, not the bootstrap adapter

`koment bootstrap` generates `.opencode/plugins/koment.js` for the repository.
That adapter and this plugin share the same hook logic — the difference is
distribution. The bootstrap adapter is the scriptable, CI-friendly surface
(it lives in the repository and is drift-checked). The plugin is a
registry-style install for users who want the policy gate without the
bootstrap step.