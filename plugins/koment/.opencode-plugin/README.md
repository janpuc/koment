# koment for OpenCode

This plugin starts the writable koment MCP server, places the strict repository
procedure in session context, and prevents a session from completing while an
annotation, inline-comment policy, or generated agent adapter is invalid.

Install a released `koment` binary on `PATH`, then enable koment in the target
repository:

```sh
koment bootstrap --agents opencode --non-interactive
```

Or install manually by adding to your `opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "koment": {
      "type": "local",
      "command": ["koment", "mcp", "--write"]
    }
  },
  "plugin": ["koment-dev/koment/plugins/koment/.opencode-plugin"]
}
```

The plugin is intentionally project-scoped: its dispose hook enforces a koment
repository and should not run in unrelated workspaces. Restart OpenCode after
installation.

## What it does

The plugin registers two hooks:

1. **tool.execute.before** — intercepts `edit` and `write` tool calls. When the
   tool would add ordinary Go comment intent, the hook denies the operation and
   suggests using `koment_add` instead.

2. **dispose** — fires when the session ends. Runs `koment check`,
   `koment comments check`, and `koment agents check`. If any fail, the session
   refuses to complete.

## Requirements

- `koment` binary on `PATH`
- A koment-enabled repository (`.koment/policy.yaml` present)

## Verification

```sh
koment check
koment comments check
koment agents check
```

All three must pass before the session completes.
