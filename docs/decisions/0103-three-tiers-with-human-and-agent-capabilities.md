# 0103 — Give humans and agents explicit capabilities in three tiers

Date: 2026-08-03
Status: Accepted

## Context

koment exists for humans and agents, but v0.2 divides them by surface. Humans
can write through the CLI and read the UI. Agents can read through MCP but have
no write tool. The Helm chart deploys either UI or MCP, so a nominal served
deployment does not serve both audiences at once. Static pages claim body
search but only filter file names.

The three deployment shapes have real differences. A static site cannot accept
a write, and a local stdio process does not need remote authentication. Hiding
those differences behind identical commands would be dishonest; letting each
surface define its own data would make them drift.

## Decision

Support three explicit tiers over the shared snapshot and application service:

- **Local:** CLI and UI for humans, stdio MCP for agents, with direct Git-record
  writes when an explicit write mode is enabled.
- **Published:** commit-stamped static UI plus body search and a stable
  `annotations.json`; always read-only and one repository per site.
- **Served:** one authenticated `koment serve` process exposing human UI and MCP
  for many repositories, with remote writes delegated to the outbox.

The local UI offers `--write` only on loopback and protects browser mutations
with a session capability, same-origin checks and CSRF protection. stdio MCP
offers `koment_add` and `koment_reanchor` only when write mode is explicit.
Unauthenticated HTTP never registers write tools.

`koment serve` owns `/`, `/mcp`, `/livez` and `/readyz`; metrics stay on a
separate listener. All long-running listeners share a signal-derived context,
bounded shutdown and fatal startup errors.

## Consequences

- Humans and agents use first-class read and write interfaces backed by one
  mutation service.
- The published tier exposes machine-readable data but cannot claim write
  parity.
- The prototype's mutually exclusive Helm modes and independent HTTP servers
  are removed.
- Local write mode requires browser security tests even though it binds only to
  loopback.
- MCP capability registration depends on transport and authorization.

## Alternatives rejected

- **Keep separate UI and MCP deployments.** Operationally simple, but every
  repository must be synchronized twice and humans and agents can observe
  different snapshots.
- **Keep all network surfaces read-only.** Avoids authentication but fails the
  requirement that hosted humans and agents can add rationale seamlessly.
- **Expose writes on every tier.** Static output cannot write and pretending it
  can would hide the tier's defining boundary.
- **Use the CLI as the only write interface.** Complete for a human with a
  shell, but not for a browser user or an MCP-only hosted agent.
