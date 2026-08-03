# 0100 — Keep one authoritative Git record per annotation

Date: 2026-08-03
Status: Accepted

## Context

The v0.2 store mirrors each source path to one YAML file containing a list of
annotations. Adding an annotation is a load, append and replace sequence.
Concurrent agents adding to the same source file can both load the same list and
the later replace loses the earlier addition. Moving an annotation between
source files changes two record files and can leave a duplicate if the process
stops between writes.

No live deployment or external record store exists. The only records to convert
are the repository's dogfood and demo data, so preserving the prototype layout
would keep its concurrency costs without protecting a user.

The record must stay in Git: review, merge, clone and ordinary history are the
durability and collaboration mechanisms. A cache or service may project it but
may not become the only copy.

## Decision

Store each annotation in `.koment/annotations/<id>.yaml`. The filename and
record id must agree. The record contains its source path, kind, body,
creation date, anchor, author claim and optional immutable Git context.
The reset record starts at version 1 because the list-shaped prototype was
never an externally supported format.

The id is a ULID generated without a new dependency. It is minted once and
survives reanchor and settlement. A directory listing makes duplicate ids
structurally impossible.

New records require an author. A prototype migration that cannot attribute a
record writes an explicit unknown legacy author instead of fabricating an
identity. Human and agent authors remain distinct, and the record states where
the identity came from and how it was verified.

Keep strict YAML through `go.yaml.in/yaml/v3`: unknown fields fail and output is
deterministic. Publish one strict JSON Schema and put its raw default-branch URL
in the editor directive at the top of every generated record. Git is the only
authoritative record. Read models can be deleted without an export or recovery
procedure.

## Consequences

- Concurrent additions create independent files rather than competing writes.
- Reanchor changes one record regardless of the source file move.
- A Git diff isolates one annotation and its provenance.
- Listing annotations for one source requires scanning records or a derived
  snapshot rather than opening a mirrored file.
- The working tree contains more small files. That is a deliberate exchange for
  safe merges and stable identity.
- Prototype records require a one-time conversion and are not read indefinitely
  by the new store.
- The schema URL can move from the default branch to an immutable published
  location once external compatibility exists.

## Alternatives rejected

- **Keep one list per source file and add locking.** Locking prevents local lost
  updates but does not reduce Git merge conflicts and a cross-file move still
  changes two authoritative records.
- **Use one directory per source file.** Concurrent adds improve, but reanchor
  still has to move the authoritative record between directories. ID-addressed
  storage keeps its path stable.
- **Make SQLite or Postgres authoritative.** Transactions become easy, but
  annotations stop being present, reviewable and mergeable in every clone.
- **Use a consolidating memory store.** Recall and deduplication are useful for
  beliefs, not exact rationale whose wording, author and history must survive.
- **Use JSON.** The standard library removes a dependency, but multiline prose
  and review diffs are worse. Strict YAML is already a small, understood
  boundary dependency.
- **Preserve the prototype's version number as historical version 1.** No live
  user consumed it, so naming the first supported record version 2 would imply
  a compatibility contract that never existed.
