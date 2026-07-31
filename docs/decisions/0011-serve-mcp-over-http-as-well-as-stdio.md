# 0011 — Serve MCP over HTTP as well as stdio

Date: 2026-07-31
Status: Accepted

Amends ADR 0005, which rejected an HTTP server. The rest of ADR 0005 — MCP as
the delivery mechanism, and a surface of exactly two tools — stands unchanged.

## Context

ADR 0005 chose stdio and rejected HTTP with three reasons: it needs a port,
lifecycle management, and auth to read files the agent can already read.

The third reason contains the assumption that fails. "Files the agent can
already read" is only true when the agent runs as a process on the same
filesystem as the checkout, which is exactly the case stdio serves. An agent in
a container, on a remote runner, or in a hosted client cannot spawn `koment mcp`
as a subprocess at all, and for those the choice is not "stdio versus HTTP" but
"HTTP versus nothing".

The first two reasons remain true and are real costs, not arguments that were
wrong.

The SDK already implements this. Verified in v1.7.0:
`NewStreamableHTTPHandler` serves the 2025-03-26 Streamable HTTP transport, and
`StreamableHTTPOptions.JSONResponse` switches responses between
`text/event-stream` and `application/json`.

## Decision

Add two HTTP transports, both from the SDK's Streamable HTTP handler:

```
koment mcp                            stdio, unchanged, still the default
koment mcp --http <addr>              JSONResponse: true  → application/json
koment mcp --streamable-http <addr>   JSONResponse: false → text/event-stream
```

stdio remains the default and the recommended transport. Nothing about the
stdio path changes.

The address defaults to the loopback interface. A bare port (`8765`) and a
port with an empty host (`:8765`) both resolve to `127.0.0.1`. Binding anywhere
else is allowed but prints a warning naming the consequence, because the server
has no authentication.

## Consequences

- Agents that cannot spawn a subprocess can now read annotations. That is the
  whole point.
- koment gains a network listener, and with it the port and lifecycle costs ADR
  0005 correctly identified. They are paid only when an HTTP flag is passed.
- **There is no authentication.** Anyone who can reach the port can read every
  annotation in the repository. Loopback-by-default keeps the blast radius at
  the local machine; a non-loopback bind is the operator's decision and is
  warned about at startup. If koment ever gains a write path over MCP, this ADR
  must be revisited before that ships — read-only is what makes no-auth
  tolerable.
- Two transports mean two code paths that can diverge. They are kept honest by
  sharing `newServer`, and both are covered by end-to-end tests that drive a
  real client rather than the handler directly.
- The legacy 2024-11-05 SSE transport is not offered. Adding it later is a
  handler and a flag if a client ever needs it.

## Alternatives rejected

- **stdio only, as ADR 0005 decided.** Simplest and safest, and still right for
  every agent that can fork a process. Rejected because it leaves containerised
  and hosted agents with no path to the annotations at all, and koment's value
  is precisely that one implementation serves every agent.
- **The legacy SSE handler (`NewSSEHandler`, 2024-11-05).** Available in the
  SDK. Rejected because it is superseded, needs a second endpoint, and no client
  in this stack requires it. Cheap to add if that changes.
- **Binding all interfaces by default.** Friendlier for containers, where
  loopback is not reachable from outside. Rejected: publishing a repository's
  accumulated rationale to the local network must be a decision someone typed,
  not a default they inherited.
- **Adding authentication now.** The SDK ships OAuth support, already in the
  dependency graph via ADR 0007. Rejected as premature: it needs an identity
  provider and configuration to protect data that is already committed in a git
  repository the reader can probably clone. Revisit if a write path appears.
- **A separate `koment serve` command.** Cleaner flag surface, but it splits one
  concept across two commands and invites the two from drifting apart. The
  transport is a property of how `mcp` runs, so it is a flag on `mcp`.
