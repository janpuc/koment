# 0105 — Authenticate remote access and settle writes through Git

Date: 2026-08-03
Status: Accepted

## Context

Read-only access still exposes source code and project rationale. v0.2 permits
public unauthenticated binds because it treats lack of mutation as the security
boundary. That protects integrity but not confidentiality. A remote write path
also needs attributable identity, repository authorization and a way to make
Git authoritative without letting an application pod push directly to a
default branch.

The project already distinguishes identity claims from proof. A served system
can strengthen that model only if it records which trusted boundary verified a
human or agent.

## Decision

Allow unauthenticated HTTP only on loopback. Every non-loopback human and MCP
request is authenticated and authorized for repository-scoped read or write
capabilities.

Use a trusted OIDC authentication proxy for human sessions and scoped bearer
credentials for agents. The application accepts forwarded identity only from
configured trusted proxies and records the issuer or credential mechanism in
the author's verification field. The deployment must prevent direct network
bypass of the trusted proxy.

An authenticated remote mutation writes an exact v2 record to a Postgres
outbox with repository id, base commit, stable annotation id and author. A
provider interface materializes it through a branch and pull request; GitHub is
the first implementation. Direct pushes to a default branch are not supported.

Settlement occurs when repository ingestion observes the same id and record
content in Git. A different committed record with the same id is a visible
conflict. Pending records are not merged, summarized, deduplicated, demoted or
expired.

## Consequences

- Remote readers no longer receive source merely because they can reach a port.
- Human and agent writes have authenticated provenance and reviewable Git diffs.
- The outbox is durable pending work but never a second authoritative annotation
  store.
- Deployments need an OIDC proxy, trusted-network configuration, agent
  credential management and a GitHub App for full write capability.
- Other forges require a materializer implementation but not a new annotation
  lifecycle.

## Alternatives rejected

- **No authentication while the API is read-only.** Confidential source and
  rationale still leak; integrity is not the only security property.
- **Implement an identity provider inside koment.** Self-contained, but it adds
  password, session and account security that a small annotation tool should not
  own.
- **Push directly to the default branch.** Fast settlement, but bypasses the
  review mechanism that makes Git records trustworthy.
- **Make pending rows authoritative forever.** Removes materialization work but
  creates two classes of annotation and makes clones incomplete.
- **Store only a prompt or summary in the outbox.** Smaller, but wording and
  provenance could change before Git settlement.
