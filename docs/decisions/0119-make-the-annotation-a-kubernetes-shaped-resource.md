# 0119 — Make the annotation a Kubernetes-shaped resource, cut off v1 records, and freeze the API

Date: 2026-08-05
Status: Accepted

## Context

ADR 0115 added the `title` field; that was the last record-shape change in a series
that started with the v1 record (`docs/decisions/0100`). The current shape
(`version`, `id`, `file`, `kind`, `body`, `anchor`, `created`, `git`, `author`,
`policy`) is verbose, single-purpose and easy to mis-edit. Two problems made
the change a decision and not an obvious call:

1. **Versioning is one-dimensional.** `version: 1` accepts only the literal value 1
   (see `internal/store/record.go:226`). There is no path to v2 because no field
   exists to carry the version *and* the API group. Records on disk would need a
   full rewrite with no marker to detect the old shape.
2. **Reason-kind shares its field with the K8s-style `kind` we want to introduce.**
   Today `kind: why` carries the rationale category. A K8s-shaped record puts a
   resource `kind` at the top of the document. They collide on the field name.

Meanwhile the project is moving from 0.x to 1.0.0: the design (per `DESIGN.md`)
is approved and the operational checklist (per `docs/decisions/0106`) is in place.
The 1.0.0 line is where the API contract becomes permanent. The schema change is
the natural anchor for that promotion.

## Decision

Wrap the record in a Kubernetes-shaped resource:

- `apiVersion` is the API group and version. Initial value `koment.dev/v1alpha`.
  The group is the canonical domain — K8s groups are DNS-1123 paths; we use ours
  for the same reason.
- `kind: Annotation` is the resource type. Fixed.
- `metadata.id` is the ULID. K8s uses `metadata.name`; we use `id` because a ULID
  is not a DNS-1123 name. Recorded as a deliberate divergence so the next reader
  does not try to "fix" it.
- `metadata.created` is an RFC3339 UTC timestamp. v1 carried a date; the binary
  parses both and emits the timestamp form.
- `spec.target.file` is the annotated source path. Wrapped (not bare) so future
  `target.function`/`target.member`/etc. can be added without another shape
  change.
- `spec.type` is the reason category (formerly `kind: why|gotcha|invariant|anti-pattern`).
- `spec.title`, `spec.body`, `spec.anchor`, `spec.author`, `spec.git?`, `spec.policy?`
  continue as before, now under `spec`.
- `status.lastSeenLine`, `status.resolution`, `status.resolvedAt`,
  `status.resolvedCommit` are persisted.
- `anchor.last_seen_line` is gone. It moves to `status.lastSeenLine` (renamed)
  because it is observed state, not anchor truth.

Who writes `status`, and when:

- **Only the commands that already write a record** — `add`, `reanchor` and
  their MCP equivalents. A read never writes. ADR 0116 rejected refreshing a
  record during resolution on the grounds that it makes reads write, and
  nothing here reverses that; a read-only deployment must stay read-only.
- **`status.resolvedAt` moves only when the observation changes.**
  `Status.Observe(resolution, commit, at)` leaves `resolvedAt` alone when both
  the resolution and the commit already match, so the field answers "since when
  has this been true" rather than "when did a command last run". That is the
  equality filter.
- **Nothing reads `status` back as a verdict.** Every surface resolves the
  anchor against the file in front of it. `status.resolvedCommit` exists so a
  reader can see how old the recorded observation is; a stored `ok` next to a
  commit that is not `HEAD` is visibly an observation, not a claim.

The JSON surfaces — the MCP tool results and the static publication — are views
of a record, not records. They keep `kind` for the rationale category and are
not reshaped. The one field that claimed to be the record's version,
`koment_add`'s `record.version`, becomes `record.api_version` and carries
`koment.dev/v1alpha`.

The API group is an identifier, not a URL. `koment.dev` is claimed but serves
nothing, and nothing in this decision requires it to: Kubernetes groups such as
`cert-manager.io` name an owner without being fetched. The
`# yaml-language-server:` directive keeps pointing at the schema on
`raw.githubusercontent.com`, because that URL is fetched and must resolve.

Old `version: 1` records are auto-migrated in place by this binary's loader.
Detection: the absence of `apiVersion` plus the presence of `version: 1`. The
migration is atomic (existing `temp+rename` pattern at
`internal/store/store.go:264`), concurrent-process safe (POSIX rename atomicity,
deterministic encoding), and idempotent (a re-read sees v1alpha). After this
release, future versions can drop auto-migrate — the build-tagged wrapper is
marked `// Deprecated:` to surface in scans.

`v1alpha` is an implementation-level stability annotation, not a stability
promise. The next breaking shape change is itself an ADR; `v1` follows when an
agent would otherwise need to write "v2 is now v1."

`apiVersion: koment.dev/v1alpha` is exact-match (`const`). Unknown versions are
rejected with a one-line actionable error pointing at the migration:

```
incompatible .koment/annotations/01JZ....yaml: record version 1 is no longer
supported. Run `koment migrate records` (deprecated alias) or simply read this
repository with a v1alpha-capable binary, which performs the migration on
first read.
```

## Consequences

What becomes easier:

- The schema is self-describing. New tooling can parse `apiVersion`/`kind` to
  dispatch on the shape without a separate contract document.
- Future API evolution has a home. New fields go into `metadata` or `spec`;
  new observed state goes into `status`. Versioned by `apiVersion`.
- A `metadata.id` is stable across renames of any other field. Reading code
  keeps the right anchor regardless of which fields around it change.

What becomes harder:

- Every read of a v1 record rewrites the file in place. Operators see one burst
  of changes per repository on first run with this binary, then never again.
  The upgrade happens in memory before the rewrite is attempted, so a
  repository mounted read-only still reads correctly; the rewrite is logged as
  a warning and retried on the next writable read rather than failing the read.
- Auto-migrate must be deleted in 1.1.x. Until then, every release carries the
  dead-code path. Marked `// Deprecated:` to surface in scans.
- Strict-mode decoding (`KnownFields(true)`) becomes two-phase: a structural
  pass to detect v1 vs v1alpha, then the strict pass on v1alpha. The first
  failure path is the migration, not an error.
- `created` carrying a date in the past and a time of 00:00:00 UTC changes how
  some downstream code sorts records (date-only → date+time). Any code that
  compares on the date alone needs to read the calendar date, not the timestamp.

What this commits us to:

- A future `koment migrate records` command becomes unnecessary. Auto-migrate is
  the only mechanism. After 1.0.0, the next release can drop it.
- The schema stops accepting `version: 1` records for new writes — only the
  binary, on read, fills the new shape. New annotations are written in v1alpha
  from day one.
- A `metadata.id` value is a ULID; not a DNS-1123 name. Tools that treat
  `metadata.id` as a name (e.g. some K8s UI assumptions) will need adaptation.
  Documented inline at the schema call site.
- The release-please config setting `bump-minor-pre-major` flips to `false` in
  the same change as this ADR's implementation, so the feat! ships as 1.0.0 not
  0.9.0.

## Alternatives rejected

- **Keep bare `version: 1`, add `apiVersion: koment.dev/v1` alongside it.** Two
  version fields per record, ambiguous which one wins, and the path to the next
  version is still structural. An annotation would carry both `version: 1` *and*
  `apiVersion: koment.dev/v1` until the day we cut `version`. Rejected because
  it doubles the surface with no extra information.

- **`oneOf v1 ∪ v1alpha` in the JSON Schema forever.** The schema stays
  ambiguous indefinitely. Every consumer has to dispatch on shape; tooling can
  never assume v1alpha. Rejected because it preserves the old shape's
  *correctness* rather than its history.

- **Rename the existing `kind` to `type` but keep the record flat (no `spec`).**
  A one-field rename is not a Kubernetes-shaped record; it is a flat record
  with a K8s-style field name. The whole point of the change is the wrapper
  (`apiVersion`/`kind`/`metadata`/`spec`/`status`). Rejected for the same
  reason a flat YAML document is not a Kubernetes resource.

- **Use `apiVersion: koment.dev/v1` and skip the `v1alpha` step.** This is the
  move-to-1.0.0 release. A v1-on-day-one label would mislead consumers into
  expecting long-term API stability. We have not yet earned that. `v1alpha`
  says "this is the first generation of a new shape; the next breaking change
  is itself an ADR." `v1` comes later, on its own merits.

- **`metadata.name` instead of `metadata.id`.** K8s uses `name`. A ULID is not
  a DNS-1123 name and would be reformatted if we ever shipped a K8s-side UI
  that assumes names. Keeping `id` documents the divergence and avoids a future
  migration. Recorded inline at the call site so the next reader does not try
  to align it.

- **Auto-migrate as a one-shot CLI (`koment migrate records`) instead of
  loader-side.** A separate command is more controllable (test, dry-run, log)
  but has to be remembered. Loader-side means the migration is just the act of
  reading. Failed migrations are still fatal — the loader refuses to return a
  partial record. Rejected because the loader is the only required entry
  point anyway; making the migration automatic eliminates a forgotten-step
  class of errors.
