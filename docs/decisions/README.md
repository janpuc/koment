# Architecture decisions

This directory contains the active decisions for the approved design. They use
the 0100 series to keep their identities distinct from the pre-deployment
prototype decisions.

On 2026-08-03, before any live deployment or durable served database existed,
the project deliberately returned to design stage. The earlier 0001–0026 set
mixed implemented behaviour, future intent, demo choices and dependency notes
until it was no longer an honest description of the product. It was removed
with explicit project-owner approval rather than carried forward as 26 active
constraints.

The old documents remain available in Git:

```text
git ls-tree -r v0.2.0 docs/decisions
git show v0.2.0:docs/decisions/0022-index-annotations-in-a-database.md
```

`DESIGN.md` marks implemented and planned boundaries explicitly. Until a planned
stage lands, the code describes current behaviour and the active ADR describes
the approved destination.

## Active decisions

- [0100 — Keep one authoritative Git record per annotation](0100-one-git-record-per-annotation.md)
- [0101 — Fail deterministic resolution when an anchor is ambiguous](0101-fail-ambiguous-anchor-resolution.md)
- [0102 — Build every read surface from one repository snapshot](0102-one-repository-snapshot-for-every-reader.md)
- [0103 — Give humans and agents explicit capabilities in three tiers](0103-three-tiers-with-human-and-agent-capabilities.md)
- [0104 — Serve assigned repositories as transactional commit snapshots](0104-transactional-multi-repository-snapshots.md)
- [0105 — Authenticate remote access and settle writes through Git](0105-authenticated-outbox-settles-through-git.md)
- [0106 — Use konflate as the operational baseline](0106-konflate-is-the-operational-baseline.md)
- [0107 — Enforce the comment-free thesis on koment itself](0107-dogfood-the-comment-free-thesis.md)
- [0108 — Layer agent guidance behind an authoritative policy gate](0108-layer-agent-guidance-behind-an-authoritative-policy-gate.md)
- [0109 — Distribute authenticated artifacts instead of Go invocations](0109-distribute-authenticated-artifacts-instead-of-go-invocations.md)

Use [0000-template.md](0000-template.md) for a new decision. A new dependency or
a structural change still requires its own ADR when these decisions do not
already settle it.
