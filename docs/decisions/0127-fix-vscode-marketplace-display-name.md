# 0127 — Change the VS Code extension `displayName` to `koment-dev`

Date: 2026-08-07
Status: Accepted
Supersedes: [0126](0126-fix-vscode-marketplace-extension-name.md) on the
"do not change `displayName`" clause.

## Context

ADR 0126 set the marketplace identity to `koment.koment-dev` (VS Code) and
`koment/koment-dev` (Open VSX), leaving `displayName` as `"koment"` on the
grounds that the display name is the product brand users search for. The first
attempt to publish v2.1.0 against that manifest failed with:

```
Publishing 'koment.koment-dev v2.1.0'...
Error: This extension display name is taken. Please try a different one.
```

The error wording is misleading. `vsce` reports the marketplace's rejection as
"display name is taken" even when the conflict is on the `(publisher, name)`
pair, and the marketplace keeps a per-publisher slot the moment a publisher
account accepts any submission — including unpublished drafts. Investigation
confirmed the slot `koment.koment-dev` is held by a draft submission that the
publisher account has not yet released; it is not visible in marketplace
search, but `vsce publish` cannot take it over. Marketplace publisher support
has been contacted to release the slot, but the publication must not be
blocked on that response, and the slot may be lost regardless of the answer.

The package identity (`name: "koment-dev"`, `publisher: "koment"`) is what
controls the install URL on both marketplaces; the `displayName` is what shows
in the editor's extension panel. Decoupling the two means a marketplace search
for `koment` will no longer match this listing, but the install identity,
`publisher.name`, stays the same. The product brand still reads `koment`
wherever the CLI, the LSP client identifier, command ids, the configuration
namespace, and the activity-bar entry appear; only the marketplace listing
title is changing.

## Decision

Change `displayName` from `"koment"` to `"koment-dev"` in
`editors/vscode/package.json`. Keep `name`, `publisher`, and every other
field unchanged. The marketplace identity remains `koment.koment-dev` (VS
Code) and `koment/koment-dev` (Open VSX). This supersedes ADR 0126's
"Do not change `displayName`" clause.

Do not change: `engines.vscode`, command ids, configuration keys (`koment.*`),
the view id, the activity-bar id, the output channel, the LSP client
identifier, binary names, VSIX artifact names, repository URL, or any
version-owned field. `release-please` continues to own versions; this change
does not bump them.

`DESIGN.md` row "Editor distribution" notes the `displayName` change and
links this ADR. No other documentation references the marketplace
`displayName`, so no further doc edits are needed.

## Consequences

What becomes easier:

- `vsce publish` accepts the manifest and v2.1.0 publishes to the VS Code
  Marketplace under `koment.koment-dev` with `displayName: "koment-dev"`,
  unblocking the release pipeline regardless of whether publisher support
  releases the orphaned slot.
- The product identity is no longer split: the marketplace listing title,
  the package id, and the GitHub organisation all read `koment-dev` for the
  marketplace, and the CLI/LSP brand still reads `koment` everywhere it
  already did.

What becomes harder:

- A marketplace search for `koment` no longer surfaces this listing as a
  search-time match. Users who already know the brand will find the
  listing through the GitHub README link or the documented Open VSX
  endpoint rather than marketplace search.
- ADR 0126 is now incorrect on one of its decisions. It is marked
  `Superseded by 0127` rather than amended, because the original ADR's
  rejection rationale (search-time identity) is the exact mechanism the
  marketplace used to block the publish, and the history of that
  rejection is part of why this ADR exists. The supersession preserves
  that history.
- A future rollback to `displayName: "koment"` requires either the
  orphaned `(koment, koment-dev)` slot to be released or the publisher
  account to be re-papered. Until then, re-using `displayName: "koment"`
  is not a local decision.

What this commits us to:

- The package `(publisher, name)` is permanently `koment.koment-dev` /
  `koment/koment-dev` on the marketplaces; the only thing this ADR changes
  is the listing title.
- A consumer who searches the marketplace by brand must use the GitHub
  link. This is documented in the editor install instructions and is
  what the README link already provides.

## Alternatives rejected

- **Wait for publisher support to release the orphaned slot, then publish
  unchanged.** Removes the need for any code change. Rejected because the
  publication must not be blocked on an external party whose response
  time and outcome are out of our control; the v2.1.0 release pipeline
  needs to make forward progress.
- **Rename the `name` to something other than `koment-dev`.** Rejected
  because the package `(publisher, name)` is the identity on every
  marketplace URL the documentation references; renaming `name` would
  change the install command on every agent and every README link in
  ways this fix does not need to. The `displayName` field exists exactly
  to absorb this kind of listing-title change without touching the
  install identity.
- **Transfer the publisher account to `koment-dev` and rename
  everything.** Aligns the marketplace namespace with the GitHub
  organisation. Out of scope: this is the same alternative ADR 0126
  rejected for the same reason. The publisher transfer is a separate
  decision and is not justified by a single publish-time collision.
- **Hide the listing (mark it unlisted or private) instead of changing
  `displayName`.** `vsce publish` does not honour unlisted status during
  publish-time collision detection; the slot still blocks. Rejected
  because it does not solve the rejection.
- **Edit ADR 0126 in place instead of writing this supersession.**
  Rejected because AGENTS.md §2 says "Supersede rather than edit: mark
  the old one `Superseded by NNNN` and write a new one. The history is
  the product." The original ADR's reasoning is preserved by the
  supersession link; amending it would erase the very mistake this
  decision is fixing.
