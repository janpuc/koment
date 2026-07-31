# 0004 — Do not use a memory store (memini) as the backend

Date: 2026-07-31
Status: Accepted

## Context

memini is already deployed in this environment, already has a per-file recall
hook that fires on `Read`/`Edit`/`Write`/`Grep`, and is already wired into every
agent client. Reusing it would have meant zero new infrastructure, and it was the
first design considered.

It was rejected after checking memini's own behaviour against the live
configuration.

## Decision

koment does not use memini, or any consolidating memory store, as its backend.
Annotations live in git (ADR 0002).

## Evidence

Three mechanisms in the running deployment are individually disqualifying for a
store that must hold verbatim, durable records:

| setting | value | consequence |
|---|---|---|
| `MEMINI_DEDUP_TIERS` | `semantic,procedural` | the tiers annotations would use |
| `MEMINI_DEDUP_LLM_MERGE` | `true` | similar records are LLM-merged |
| `MEMINI_DEDUP_SIMILARITY` | `0.80` | a low bar; related annotations collide |
| `MEMINI_DEMOTE_AFTER` | `1440h` | never-recalled durable rows are demoted |

On merge, the surviving record's content is overwritten with no in-store
history. Demoted records fall to `episodic`, which carries a 30-day TTL — so an
annotation on a stable file that nobody recalls for 60 days is silently deleted.
Write-time distillation additionally paraphrases content before it is stored.

## Consequences

- koment owns its storage and its guarantees: nothing rewrites an annotation
  except a person or an agent making an explicit edit.
- Annotations are reviewable in a pull request and restorable from git history.
- The per-file recall hook is not reused; koment ships its own delivery path
  (ADR 0005).
- memini keeps the job it is good at — preferences, incident learnings,
  cross-project facts — where consolidation is a feature rather than corruption.

## Alternatives rejected

- **memini with dedup and demotion disabled for a dedicated namespace.**
  Possible, but it turns a memory system into a database by switching off the
  behaviour that defines it, and the settings are global to the deployment. One
  future config change silently eats the annotations.
- **A general-purpose database** (SQLite, Postgres). Solves durability, loses
  reviewability and adds an availability dependency to reading your own code.
- **Any external service.** Reading a checkout must never depend on a network
  call.
