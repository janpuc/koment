# OpenClaw

[OpenClaw](https://docs.openclaw.ai) declares MCP servers under `mcp.servers` in
its config, which is JSON5 — so comments and trailing commas are fine.

## Configure

```json5
{
  mcp: {
    servers: {
      koment: {
        command: "koment",
        args: ["mcp"],
        cwd: "/path/to/your/repo",
        transport: "stdio",
        enabled: true,
      },
    },
  },
}
```

Three things OpenClaw is strict about:

- `transport: "stdio"` is explicit, and requires a non-empty `command`.
- `command` must resolve in the **Gateway process** environment, not your shell.
  If the Gateway runs as a service or in a container, `koment` needs to be on
  *its* `PATH` — an absolute path is the reliable answer.
- `cwd` must be a valid path. Set it to your repository: this is how koment
  finds `.koment/`.

## Configure — remote

If the Gateway runs somewhere the code doesn't, serve over HTTP from the
checkout instead:

```sh
cd /path/to/your/repo
koment mcp --http 8765
```

```json5
{
  mcp: {
    servers: {
      koment: {
        url: "http://koment.internal:8765",
        enabled: true,
      },
    },
  },
}
```

There is no authentication on the v0.2 HTTP transport. That is legacy
behaviour; the approved served tier authenticates every non-loopback request
([ADR 0105](../decisions/0105-authenticated-outbox-settles-through-git.md)).

## Timeouts

koment reads local files and answers in milliseconds, so the defaults are
generous. If you are tuning them anyway:

```json5
connectionTimeoutMs: 5000,
requestTimeoutMs: 20000,
```

## Notes

- One Gateway serving several repositories needs one server entry per
  repository, each with its own `cwd` and a distinct name — `koment-api`,
  `koment-web`. A single entry cannot span checkouts, because the repository is
  resolved from the working directory.
