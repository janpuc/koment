# 0105 — Authenticate remote access and materialize writes through Git

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

An authenticated remote mutation creates an exact annotation record with its
repository id, base commit, stable annotation id and author, then synchronously
asks a provider materializer to create a deterministic branch, commit and pull
request. GitHub is the first implementation. Direct pushes to a default branch
are not supported.

The request reports success only after the pull request exists. The provider's
branch, commit and pull request are the durable pending state. If a provider
call fails part way through, retrying the same record inspects and resumes that
state instead of creating a second record or pull request.

Settlement occurs when synchronization observes the same id and record content
on the default branch. A different committed record with the same id is a
visible conflict. Pending records are not merged, summarized, deduplicated,
demoted or expired.

## Consequences

- Remote readers no longer receive source merely because they can reach a port.
- Human and agent writes have authenticated provenance and reviewable Git diffs.
- koment needs no durable application store, migration system or background
  outbox worker.
- Remote mutation latency includes provider commit and pull-request creation.
- A provider outage makes new remote writes fail loudly instead of accepting
  work that koment cannot durably queue.
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
- **Persist a Postgres outbox and materialize asynchronously.** It can accept a
  request during a provider outage, but introduces a database, worker,
  migrations and recovery protocol solely for pending Git operations. Before a
  live deployment demonstrates that synchronous provider latency is
  unacceptable, deterministic retries use Git itself as the durable boundary.
- **Make pending application rows authoritative forever.** Removes
  materialization work but creates two classes of annotation and makes clones
  incomplete.
- **Store only a prompt or summary before materialization.** Smaller, but
  wording and provenance could change before Git settlement.
