# 0023 — The store round-trips through the index

Date: 2026-08-02
Status: Accepted

Builds on ADR 0022, which made the index derived. This ADR makes the derivation
reversible, and says what that costs.

## Context

ADR 0022 established one direction: `.koment/**.yaml` → index. That is enough
for serving and it leaves two things unanswered.

**Bootstrapping.** An empty or discarded index has to come back from somewhere.
Today that happens because something calls `Rebuild`, which is fine when a
person runs `koment index` and useless when a container starts with a cold cache
and nobody tells it to. A derived store that does not derive itself is a store
with a manual step nobody remembers.

**Recovery in the other direction.** Once annotations can be created against the
index — the interactive path ADR 0016 describes — the index holds records that
`.koment/` does not, and materialising them back into YAML is how they become
part of the repository. Without that, ADR 0016's outbox has no outlet.

There is also an immediate reason, independent of the interactive path: if
`.koment/` is lost or damaged and the index survives, the annotations are still
there. A one-way derivation makes that unrecoverable for no reason.

Attempting the export exposed the real problem. The index stored nine of the
eighteen fields a record carries. `last_seen_line`, the git path and line range,
and four of the six author fields were absent. An export built on that would
have produced valid-looking YAML that silently dropped provenance — data loss
wearing the costume of a round-trip, and far worse than having no export at all.

## Decision

**The index stores every field a record has, and the round-trip is exact.**

Reading `.koment/`, writing it to the index, and exporting it back produces
byte-identical YAML. A test asserts that on a record exercising every optional
field, and it is the property the rest of this ADR depends on.

The schema gains the missing columns and `schemaVersion` becomes 2. Per ADR
0022 that discards existing indexes and rebuilds them, which costs nothing
because they are derived.

**Bootstrap is automatic.** Opening an index for a repository that has no rows
builds it from `.koment/` before serving anything. A cold cache, a wiped volume
and a fresh container are all the same case, and none of them needs a person.

**`koment export` writes `.koment/` from the index.** The static site export,
which was `koment export`, becomes `koment site` — it is scoped to publishing
the demo (ADR 0018, 0021), while reconstructing the store is the more central
operation and deserves the plainer name.

Export writes through the same `store.Save` the CLI uses, so there is one
writer, one validation path and one YAML formatting path. An export that
produced YAML by templating strings would drift from what `add` writes.

## Consequences

- `.koment/` bootstraps the index, and the index can reconstruct `.koment/`.
  Either can be lost without losing annotations, which is what makes the
  two-representation design safe rather than merely fast.
- **Nothing about authority changes.** Git remains the record; the index remains
  derived. Export is recovery and, later, materialisation — it is not a licence
  to treat the index as the source of truth, and `koment check` still reads YAML.
- The round-trip test is now load-bearing. If a field is added to `Annotation`
  and not to the schema, that test fails rather than an export quietly dropping
  it — which is the failure this ADR exists to prevent.
- Renaming a published command breaks anyone scripting `koment export` for the
  site. It is a 0.1.x tool and the old name was documented as demo-only, so the
  cost is a line in the changelog.
- The index carries provenance it never queries. That is the price of exactness,
  and it is small.

## Alternatives rejected

- **Export only the fields the index already had.** No schema change, no
  migration, works immediately. Rejected outright: it produces YAML that looks
  complete and has lost provenance. A lossy export is worse than none, because
  the loss is discovered when the original is already gone.
- **Reconstruct missing fields at export time** — re-derive `last_seen_line` by
  resolving, re-read git for the commit context. Rejected because it fabricates:
  `last_seen_line` would become wherever the excerpt is *now*, and the git
  context would describe today rather than when the annotation was written,
  which is precisely what ADR 0014 forbids.
- **Bootstrap on a timer instead of on open.** Simpler control flow. Rejected
  because it serves an empty index until the first tick, which reads as "this
  repository has no annotations" — a confident wrong answer.
- **Keep `koment export` for the site and name this `koment materialise`.** No
  rename, no breakage. Rejected because it leaves the central operation with the
  obscure name and the demo-only one with the obvious name, and every reader
  after today pays that.
- **Have export write YAML directly rather than through `store.Save`.** Fewer
  layers. Rejected because it would be a second writer that has to be kept in
  step with the first, and the two would diverge on the first formatting change.
