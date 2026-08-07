# 0126 — Fix the VS Code extension marketplace name to `koment-dev`

Date: 2026-08-07
Status: Superseded by [0127](0127-fix-vscode-marketplace-display-name.md)

## Context

The repository moved to the `koment-dev` GitHub organisation, and the package's
current `name` field (`koment`) collides with that history. The
`repository.directory` (`editors/vscode`), the `homepage` and the Open VSX badge
already point at `github.com/koment-dev/koment`. The marketplace `name` is the
only field that still says `koment`.

`DESIGN.md` records that the marketplace publication step is "pending" because
no publisher account has been accepted yet. A live check confirms that the
candidate marketplace listings are not present:

  - `https://open-vsx.org/extension/koment/koment` → 404;
  - `https://marketplace.visualstudio.com/items?itemName=koment.koment` →
    page renders without an extension listing.

The Open VSX API endpoint hardcoded into the release verification command
(`docs/releasing.md`, step 6) is already wrong: it asks for
`https://open-vsx.org/api/koment-dev/koment`, which would resolve under a
publisher named `koment-dev`, not the publisher `koment` currently declared in
`editors/vscode/package.json`. That URL must agree with whatever
`editors/vscode/package.json` declares at release time, otherwise step 6 of the
release procedure probes an endpoint that cannot exist.

The marketplace identifies a package by the pair `(publisher, name)`. The two
identifiers are independent: changing one does not move the other, and the
existing publisher slot (`koment`) can publish any package name the publisher
controls. There is therefore a choice between three compositions:

| publisher | name | VS Code id | Open VSX id |
|---|---|---|---|
| `koment` | `koment` | `koment.koment` | `koment/koment` |
| `koment` | `koment-dev` | `koment.koment-dev` | `koment/koment-dev` |
| `koment-dev` | `koment-dev` | `koment-dev.koment-dev` | `koment-dev/koment-dev` |

The product name, CLI binary name, configuration namespace (`koment.*`), view id,
output channel, command ids, and LSP client identifier are unrelated to the
marketplace `(publisher, name)` pair and remain `koment`.

## Decision

Compose the marketplace identity as `koment.koment-dev` (VS Code) and
`koment/koment-dev` (Open VSX). Keep `publisher: "koment"` in
`editors/vscode/package.json`; change `name` from `"koment"` to
`"koment-dev"`. Make the matching change to `editors/vscode/package-lock.json`
root package name so the lockfile describes the same package.

Do not change `displayName`. The marketplace listing will read `koment` to a
user who searches by it from the editor's extension panel; the search-time
identity and the install-time identity stay aligned because the listing's
`displayName` is what users see, and the install-time identity is the
`(publisher, name)` pair.

Do not change any of: `engines.vscode`, command ids, configuration keys, the
view id, the output channel, the LSP client identifier, binary names, VSIX
artifact names, repository URL, or any version-owned field. `release-please`
owns versions; this change does not bump them.

The Open VSX endpoint queried by the release verification command becomes
`https://open-vsx.org/api/koment/koment-dev`. Add the matching VS Code
Marketplace probe to step 6 so the procedure verifies both mirrors of the
release, and update the README badge and link to `koment/koment-dev`. Keep
GitHub organisation references (`github.com/koment-dev/koment`) unchanged.

Because the marketplace listing has never published, there are no installed
users to migrate. The old `(koment, koment)` slot has no marketplace presence
that would need to be deprecated in the marketplace dashboards, and there is no
external surface that needs to be redirected. This is a pre-publication
correction, not a migration. The release procedure for the first publication
remains whatever `docs/releasing.md` already says.

## Consequences

What becomes easier:

- The marketplace identity matches the GitHub organisation and the
  installation identity matches the display name. The release verification
  command probes an endpoint that the manifest actually targets.
- A future rollback is possible without orphaning users on the marketplace
  (because the listing was never published), at the cost of reverting this
  change before any release PR is merged.

What becomes harder:

- Two new ADR-plus-design records need to be maintained in lockstep with the
  manifest: `DESIGN.md` and `docs/decisions/0126`. The marketplace identity is
  no longer derivable from the GitHub organisation name; the manifest is the
  source of truth and the design records spell out the composition.
- The first release PR after this change publishes a marketplace listing under
  the new id. Per `docs/releasing.md`, the human must approve the merge before
  publication; that explicit conversation is the only irreversible step.
- The Open VSX namespace must exist before the first publish. The release
  workflow already creates it from `package.json#publisher`; with the publisher
  unchanged as `koment`, the workflow's existing `ovsx create-namespace` step
  creates the right namespace (`koment`) without further edits.

What this commits us to:

- The publisher slot stays `koment`. Transferring it would require a separate
  decision and out-of-band marketplace work; do not change `publisher` without
  a new ADR.
- The package name stays `koment-dev`. Renaming it again before the first
  publish is cheap; renaming it after the first publish is a permanent
  marketplace change and must be its own ADR.

## Alternatives rejected

- **Change `publisher` from `koment` to `koment-dev`.** Aligns the namespace
  with the GitHub organisation, but the publisher slot has not been provisioned
  on either marketplace. Switching publishers before the first publish would
  orphan the publisher configuration on `koment` and require fresh PATs against
  a new publisher account that does not yet exist. Out of scope for a
  name-only correction.
- **Change both `publisher` and `name` to `koment-dev`.** Same problem as
  changing the publisher alone, applied twice. The marketplace identity would
  be `koment-dev.koment-dev` / `koment-dev/koment-dev`, but neither namespace
  exists. Rejected because it does the work twice without producing a better
  outcome.
- **Keep `name: "koment"` and align the marketplace identity some other way.**
  The only way to keep the package name as `koment` is to publish under the
  publisher `koment`, which is what the current manifest already does. That
  produces the same `(koment, koment)` slot this decision abandons. Rejected
  because it does not address the discrepancy between the GitHub organisation
  name and the marketplace id.
- **Change `displayName` to `koment-dev`.** Aligns every marketplace-visible
  string with the new package name. Rejected because `displayName` is the
  product brand that the CLI, the binary, the configuration namespace and
  every existing user-facing surface already carry as `koment`. Decoupling
  brand from package name would make a marketplace search for `koment` fail
  while keeping the installed extension labelled `koment-dev`, which is worse
  than the original problem.
- **Defer the change until after the first publish, then rename.** The
  marketplace identity becomes permanent on first publish. Renaming after the
  first publish is a different decision with a migration cost that this
  pre-publication correction avoids. Rejected because it forfeits the one
  chance to set the identity cleanly.
- **Hand-edit the version of `editors/vscode/package.json` to `3.0.0` for
  "backwards compatibility".** `release-please` owns every version-bearing
  field, per `docs/releasing.md` step 2 and AGENTS.md §14. Hand-editing the
  version would break the equality assertion the editor job performs before
  packaging and would force a manual re-run of release-please. Rejected
  because the change does not need a major version bump; there is no installed
  user base to break.