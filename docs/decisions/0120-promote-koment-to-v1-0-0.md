# 0120 — Promote koment to v1.0.0

Date: 2026-08-05
Status: Accepted

## Context

The release series so far is `0.1.0` → `0.8.0` (eight minor releases plus
several patches). The semver contract below 1.0.0 is "anything may break." Two
forces make the move to 1.0.0 a decision and not an obvious call:

1. **The design is approved.** `DESIGN.md` carries the heading "approved;
   product and reference integrations implemented, external catalog acceptance
   pending." Stages 1–6 of the implementation sequence are listed as
   implemented. Dogfooding is done (see ADR 0107). The system is not a
   prototype.
2. **The API contract has just been upgraded.** ADR 0119 introduces
   `koment.dev/v1alpha`, the strict-mode schema, auto-migration from v1, and
   the cutover policy. The new shape is the right place to draw the stability
   line: `v1alpha` says "this is the first generation of a new shape; the next
   breaking change is itself an ADR; `v1` follows when an agent would otherwise
   need to write 'v2 is now v1.'"

Releasing as 1.0.0 commits the project to two things the 0.x line did not
promise: a public, versioned API group, and an explicit no-silent-compatibility
rule that makes future back-compat claims evidence-backed.

## Decision

Promote koment to 1.0.0 in the same release that lands ADR 0119.

Mechanics:

1. **One breaking feature PR.** Subject
   `feat!: reshape the annotation record and cut off v1` carries the
   breaking-change semantics release-please keys off. With the config flip in
   (2), this lands as 1.0.0 rather than 0.9.0.
2. **release-please config flip.** `release-please-config.json` moves
   `bump-minor-pre-major` from `true` to `false` in the same PR. While below
   1.0 the setting treated breaking changes as minor bumps; at 1.0.0 they
   become major again.
3. **AGENTS contract gain.** `.koment/policy.yaml` gains a principle recording
   the rule that back-compat claims require (a) a migration tool or auto-migrate
   path, (b) an ADR naming the cutoff version, and (c) a `feat!:` subject when
   either is missing. `koment agents install` regenerates AGENTS.md and the
   per-client adapters; `mise run agent-policy` confirms.
4. **`docs/releasing.md` is followed exactly.** Steps 0–3 run in this release:
   preconditions (the 13 `mise run` checks), feature merge, release-please
   opens the release PR, unblock the `action_required` run, wait for the four
   required checks. Steps 4 and 5 (merge the release PR; watch publication)
   require explicit human approval in the conversation per AGENTS.md rule 14.

Why this release and not the next one:

- The breaking-shape change is the largest single contract change in the
  project's lifetime. Drawing the v1.0.0 line at any other moment would require
  either rolling back the change and doing it again (worse churn) or shipping
  1.0.0 on a quieter day (false cleanliness).
- All twelve preconditions in `docs/releasing.md` step 0 are achievable in one
  PR cycle. The work is large but uniform in shape — schema, struct, consumers,
  auto-migrate, status writer, examples, AGENTS rule, config flip, single
  feat.

## Consequences

What becomes easier:

- Consumers can write integrations knowing the API group is `koment.dev` and
  the shape version is recorded on the file. A consumer tool can dispatch on
  `apiVersion` instead of guessing.
- The agent contract's back-compat rule makes "silent compatibility claim"
  detectable. A future commit that claims back-compat without the evidence
  surfaces as a CI gate, not as a post-release bug.
- Future `feat!` commits produce a major version. The pre-release period of
  ambiguity (0.9 → 1.0 vs 0.9) is over.

What becomes harder:

- A 1.0.0 release is the next release after the cutover PR merges. Until 1.0.0
  ships, the `feat!` PR is the *only* place the v1alpha cutover can land;
  merging other work on top of it makes the conflict graph harder. The PR has
  to be the next thing merged, or be rebased cleanly onto anything that landed
  on `main` first.
- The `1.0.0` tag is permanent. A mistake inside the v1.0.0 cutover that ships
  becomes a 1.0.1 patch.
- 1.0.x maintenance begins immediately. Any fix that lands during the v1alpha
  cutover is a 1.0.x patch candidate.

What this commits us to:

- A `1.0.0` `git tag` named `v1.0.0` with the canonical artifacts: signed
  binaries at GitHub Releases, OCI image at GHCR with SBOM and provenance,
  MCP Registry entry, signed Helm chart, marketplace-published VS Code and
  Open VSX packages.
- `release-please` keeps owning every version-bearing file. No hand-edits to
  `.release-please-manifest.json`, `charts/koment/Chart.yaml`,
  `editors/vscode/package.json`,
  `plugins/koment/.claude-plugin/plugin.json`, or `server.json`.
- The `bump-minor-pre-major: false` setting stays in place until/unless an ADR
  reverses it (which would itself need to be a `feat!`).
- AGENTS.md rule 14 ("releases follow the written procedure exactly") is in
  force; an agent never publishes a release by hand.

## What this needs that is not yet in place

These are the operational prerequisites for the cutover. They are not blockers
on the design, but the v1.0.0 PR cannot pass CI without them.

- **Explicit human approval** for the actual merge of `chore(release): 1.0.0`.
  AGENTS.md rule 14 reserves this for the project owner. Until they merge, the
  version does not become public.
- **CHANGELOG.md** populated by release-please from the `feat!` subject. The
  body should describe the cutover honestly: "annotation records adopt the
  Kubernetes shape (`apiVersion: koment.dev/v1alpha`) per ADR 0119; v1 records
  are auto-rewritten on first read; AGENTS contract gains a back-compat
  evidence principle per ADR 0120."

## Alternatives rejected

- **Ship the cutover as 0.9.0; promote to 1.0.0 in a follow-up minor release.**
  Two releases for one feature. Operators read the version and learn "still
  under 1.0." A two-release cutover doubles the chore without offering any
  extra safety, because `feat!` and release-please are already aligned to
  drive 1.0.0 directly.

- **Wait for a feature flag toggle.** Run 1.x with the legacy reader behind
  `--legacy-records=allow` for one cycle, then remove. Rejected. Feature flags
  add a permanent surface for a one-time migration. The point of v1.0.0 is
  that the migration is not optional; a flag would imply otherwise.

- **Promote first, change shape later.** Ship 1.0.0 on 0.8.0 contents; backport
  the shape into 1.0.1. Rejected. Drawing the stability line on 0.8.0 contents
  would invite consumers to write tools against the v1 record shape, which is
  then broken by 1.0.1. The promotion and the shape change must ship together.

- **Leave the project at 0.x indefinitely.** Keep adding features and call the
  production-readiness "1.0.0" at some never-defined future time. Rejected.
  The design is approved, the code is dogfooded, the operational layer
  (konflate, ADR 0106) is in place. The conditions for 1.0.0 are met; the
  version number is the artifact of that.

- **Force-push or amend the cutover PR after merge.** Use `git push
  --force-with-lease` to rewrite the v1.0.0 commit before the release PR is
  opened. Rejected. The merged commit is signed and CI-attested; re-writing it
  forfeits the attestations. If the cutover PR is wrong after merge, the next
  release is `1.0.1`, never a history rewrite.
