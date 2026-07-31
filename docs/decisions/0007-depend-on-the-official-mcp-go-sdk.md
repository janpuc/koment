# 0007 — Depend on the official MCP Go SDK

Date: 2026-07-31
Status: Accepted

## Context

ADR 0005 makes MCP the delivery mechanism. That leaves how to speak it.

MCP over stdio is JSON-RPC 2.0 plus a handshake: `initialize`, then
`tools/list` and `tools/call`. Implementing that against the standard library is
perhaps three hundred lines, and for a two-tool server none of the protocol's
larger surface — resources, prompts, sampling, roots, HTTP transports, OAuth —
is exercised.

Verified against the module proxy on 2026-07-31:
`github.com/modelcontextprotocol/go-sdk` is at v1.7.0, is Anthropic-maintained,
and is past v1. After `go mod tidy` it pulls eight external modules:

```
github.com/google/jsonschema-go
github.com/segmentio/asm
github.com/segmentio/encoding
github.com/yosida95/uritemplate/v3
golang.org/x/oauth2
golang.org/x/sync
golang.org/x/sys
golang.org/x/time
```

None of those is needed by a stdio server exposing two tools. Weighed against
that: the protocol is versioned and moving. The SDK README documents protocol
version `2026-07-28` deprecating roots, sampling and logging, with support held
"for at least twelve months". A hand-rolled implementation makes tracking that
our problem, and a handshake that is subtly wrong fails at the client, in a
different codebase, with no useful error.

## Decision

Depend on `github.com/modelcontextprotocol/go-sdk` at v1.7.0.

Confine it to `internal/mcp`. The tool handlers take and return koment's own
types; no SDK type appears in a signature outside that package.

Keep the tool surface at exactly the two tools ADR 0005 names.

## Consequences

- Protocol correctness and version negotiation become upstream's problem, which
  is the entire reason to take the dependency.
- Eight transitive modules enter the graph to serve two tools. This is the real
  cost and it is not small. It is accepted because the alternative spends the
  same complexity budget on code we would have to maintain against a moving
  spec, rather than code someone else maintains against a spec they own.
- `golang.org/x/oauth2` ships in the binary despite stdio needing no auth. Dead
  weight, not a vulnerability surface — stdio opens no socket.
- The SDK is past v1, so breaking changes should arrive as v2 rather than as a
  patch bump. Pin the exact version; do not float.
- Confinement to `internal/mcp` means replacing the SDK later, if its cost stops
  being worth it, is one package's problem. That escape hatch is the condition
  on which this dependency is acceptable.

## Alternatives rejected

- **Hand-rolled JSON-RPC 2.0 over stdio, standard library only.** Zero
  dependencies, and small enough to actually read — genuinely close to winning.
  Rejected because the protocol is versioned and actively changing: we would own
  spec drift forever, and the failure mode of getting the handshake wrong is
  silent breakage inside someone else's agent. Revisit if the SDK's dependency
  graph grows further or it stops being maintained; the confinement above is
  what keeps that revisit cheap.
- **A third-party MCP library.** No advantage over the official SDK on
  correctness, and strictly worse on the thing that matters here — tracking the
  spec — since the official SDK is maintained by the people who publish it.
- **Vendoring the SDK's stdio path only.** Trims the graph, but forks it: we
  inherit the maintenance we were paying to avoid, and lose the upgrade path.
