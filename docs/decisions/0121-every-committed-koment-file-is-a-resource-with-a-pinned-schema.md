# 0121 — Every committed koment file is a resource, and its schema is pinned to the API version

Date: 2026-08-05
Status: Accepted

## Context

ADR 0119 reshaped the annotation record into a Kubernetes-shaped resource and
introduced the `koment.dev/v1alpha` API group. It stopped there, which left two
loose ends discovered while implementing it.

1. **`.koment/policy.yaml` was still flat.** It sits in the same directory as
   the records, is committed to the same repository, is read by every client,
   and is the file that decides what `koment comments check` and
   `koment agents check` enforce. After 0119 it was the only committed koment
   file that still began `version: 1`. Two shapes for two files in one
   directory is a contract a reader has to learn twice.
2. **The published schema URL is not versioned.**
   `schema/annotation.schema.json` on the default branch is a floating URL: it
   always describes whatever the newest generation is. Every record koment
   writes embeds that URL in its `# yaml-language-server:` directive. The
   moment a `v1beta` exists, every `v1alpha` record on disk points at a schema
   that no longer describes it, and the editor reports the record as invalid.
   `v1alpha` is by construction not the last generation, so this is a scheduled
   failure rather than a hypothetical one.

There is also a scope question these two force: which files belong to the
`koment.dev` group at all? `schema/credentials.schema.json` and
`schema/server.schema.json` describe the served token store and the served
repository list.

## Decision

**The `koment.dev` group covers what lives in `.koment/` and is committed.**
That is the annotation records and the repository policy. Server deployment
configuration — the credential store and the served-repository list — is
operator state on one machine, not repository content that another tool reads,
and stays an ordinary configuration file with an ordinary schema.

**`.koment/policy.yaml` becomes `kind: Policy` in the same group.**

```yaml
# yaml-language-server: $schema=.../schema/v1alpha/policy.schema.json
apiVersion: koment.dev/v1alpha
kind: Policy
spec:
  comments:
    mode: strict
    intrinsic: [toolchain-directive, ...]
    generatedPaths: ['**/*.gen.go']
    vendoredPaths: [vendor/**]
  agents:
    adapters: [agents, claude, copilot, cursor, codex, opencode]
    principles: [back-compat-evidence]
```

A policy carries **no `metadata`**. It is a singleton whose name is its path;
inventing an id for it would create an identifier nothing refers to.

**Schemas are published per API version**, at `schema/v1alpha/`. `SchemaBase`
in `internal/api` is the single place the path is written, so the two schema
URLs cannot disagree about which generation they belong to.

**The whole v1alpha surface is camelCase.** `generated_paths` becomes
`generatedPaths`, `vendored_paths` becomes `vendoredPaths`, and the annotation
record's last snake_case survivor, `spec.git.end_line`, becomes
`spec.git.endLine`. A frozen API with two casing conventions is a question
every reader has to ask once.

**`spec.agents.principles` is a closed vocabulary**, and the first member is
`back-compat-evidence`, which ADR 0120 requires. An enabled principle appends
one line to the generated agent contract. The wording lives in Go beside the
vocabulary, so a repository cannot state a principle koment does not enforce
the meaning of.

`agentpolicy.Contract()` remains the procedure alone and is what the MCP server
announces, because one server can front several repositories whose policies
differ. `agentpolicy.ContractFor(policy)` adds the principles and is what
writes `AGENTS.md`, the Copilot instructions and the Cursor rule.

**Both resources upgrade a flat `version: 1` file in place on read**, by the
same mechanism and with the same guarantees as ADR 0119: atomic, idempotent,
and skipped without failing the read when the filesystem is read-only.

## Consequences

What becomes easier:

- One shape to learn. A reader who can read an annotation can read the policy.
- A `v1beta` can ship without invalidating a single record already on disk,
  because `schema/v1alpha/` keeps describing v1alpha for as long as v1alpha
  records exist.
- A principle is a policy value rather than prose someone remembered to paste
  into `AGENTS.md`. `koment agents check` fails when the generated contract
  drifts from what the policy states.

What becomes harder:

- Two deprecated upgrade paths now have to be deleted in the release after
  1.0.0 rather than one. Both are marked `// Deprecated:`.
- `schema/` now has two kinds of thing in it: a versioned API directory and two
  unversioned configuration schemas. The boundary is stated above and in
  `internal/api`, but it is a boundary somebody can get wrong.
- Every repository using koment sees its `.koment/policy.yaml` rewritten once,
  on top of the annotation rewrite from ADR 0119. One commit, one time.

What this commits us to:

- `schema/v1alpha/` is frozen once 1.0.0 publishes. A change to a file under it
  changes the meaning of records already written against it, so it is only ever
  a correction to a schema that was wrong about its own generation. Anything
  else goes in the next version's directory.
- The `principles` vocabulary is part of the frozen API. Adding a member is a
  minor change; removing or renaming one is breaking.

## Alternatives rejected

- **Leave the policy flat.** It is configuration, not data, and nothing outside
  koment reads it. Rejected because "nothing outside koment reads it" was also
  true of the annotation record until it wasn't, and because the cost of the
  inconsistency is paid by every reader of `.koment/` forever while the cost of
  fixing it is paid once, now, by the same migration that is already running.

- **Give the policy `metadata.name: default`.** Closer to Kubernetes, where a
  singleton is still a named object. Rejected: the name would be a constant
  nothing selects on, and a constant that is required but carries no
  information is a field people get wrong.

- **Keep one floating schema URL and make it a `oneOf` over every generation.**
  The URL stays stable and old records keep validating. Rejected for the same
  reason ADR 0119 rejected a permanent `oneOf` for the record: the schema stops
  describing a generation and starts describing a union, so no consumer can
  ever assume a shape, and an editor's error message stops naming what was
  actually wrong.

- **Pin the schema URL to a release tag rather than a version directory**
  (`.../v1.0.0/schema/annotation.schema.json`). Genuinely immutable, and it is
  what a package registry would do. Rejected because it makes the directive
  churn on every release: every record written after 1.0.1 would carry a
  different URL from every record written before it, and `git log` would fill
  with schema-URL diffs that mean nothing. The API version is the thing that
  changes when the shape changes; the release number is not.

- **Move `credentials` and `server` into the group too.** Consistent, and it
  would make `schema/` uniform. Rejected as scope: they describe one operator's
  deployment rather than repository content, no second tool reads them, and
  including them would freeze two more shapes into a 1.0.0 API for no reader's
  benefit. If either ever becomes something a second tool reads, that is the
  moment it becomes a resource, and it will need its own ADR.
