# 0123 — A published path never begins with a dot

Date: 2026-08-06
Status: Accepted

## Context

Six of the fifty pages on the published site returned 404: every annotation on
a file under `.github/` or `.mise/`. The index linked to them, `koment site`
wrote them correctly to disk, and CI passed. The failure was invisible to
everything except a reader clicking a link.

The first attempt blamed Jekyll and wrote a `.nojekyll` marker, which is the
documented remedy for GitHub Pages hiding dot paths. It did not work, and the
marker itself returned 404 — which turned out to be the clue.

`actions/upload-pages-artifact` builds the artifact with:

```
tar --exclude=.git --exclude=.github <--exclude=.[^/]* unless include-hidden-files> .
```

Two separate exclusions, and `tar --exclude` matches any path component rather
than only the root:

- `--exclude=.[^/]*` removes every dot-prefixed entry, including the
  `.nojekyll` marker meant to fix the problem.
- **`--exclude=.github` is unconditional.** The action's own input
  documentation says "Excludes .git and .github regardless." No setting
  recovers it.

Reproduced locally against the same tar invocation: with only the
unconditional excludes, `f/.mise/config.toml.html` survives and
`f/.github/workflows/ci.yml.html` does not.

So `include-hidden-files: true` would have recovered one of the six pages.
Annotating a workflow file — among the most natural things to annotate — could
not be published at all.

## Decision

koment maps every dot-prefixed path component to a `dot-` prefix when it
writes a published page and when it links to one. `.github/workflows/ci.yml`
publishes at `f/dot-github/workflows/ci.yml.html`.

`publishedPagePath` is the only function that produces either, so the written
file and the link cannot disagree. That mattered: they were previously
computed in two places, and only one of them escaped.

The `.nojekyll` marker is not written. It cannot reach the artifact without
`include-hidden-files`, and once no published path contains a dot it has
nothing left to protect.

This is deliberately a property of the output rather than a configuration of
one publisher. A published koment site is a directory of static files, and
several hosts treat dot paths specially — GitHub Pages, S3 website endpoints
and various CDNs among them. Producing output that no common host mangles is
worth more than a flag that fixes one of them.

The served tier is unchanged. It resolves real paths through
`escapedFilePath` and never writes files, so it has no such constraint. Only
the published tier maps.

## Consequences

What becomes easier:

- An annotation on `.github/workflows/*` publishes and is reachable, which was
  the entire reported bug.
- A published site can be hosted anywhere static without per-host flags.
- The pages workflow asserts that nothing dotted ships at all, so a regression
  fails the build rather than the website. The previous gate could not have
  caught this: it asserted a marker that the uploader was silently discarding.

What becomes harder:

- Published URLs for dot paths change shape. Nothing breaks, because those
  URLs have never successfully served a page.
- A repository containing both `.github/` and a literal `dot-github/` would
  collide. Vanishingly unlikely, and the collision is visible as two sources
  claiming one page rather than as silent loss.
- The published path is no longer the source path, so a reader constructing a
  URL by hand needs the mapping. The index links are correct, and nothing
  documents URLs as hand-constructible.

## Alternatives rejected

- **`include-hidden-files: true` on the upload step.** One line, and it fixes
  `.mise`. Rejected because it does not fix `.github`, which is five of the six
  broken pages and the common case — the exclusion is unconditional. A fix
  that leaves the reported bug in place is not a fix.

- **Stop using `actions/upload-pages-artifact` and deploy some other way.**
  Removes the exclusions entirely. Rejected as disproportionate: it replaces a
  supported, SHA-pinned action with bespoke deployment to solve a problem that
  a path mapping solves for every host at once.

- **Keep `.nojekyll` as well, for safety.** Rejected because it is now
  cargo: it cannot reach the artifact, and with no dot paths remaining there is
  nothing for it to do. A file that does nothing invites someone to reason
  about why it is there.

- **Encode the dot as `%2E`.** Percent-encoding survives the URL but not the
  filesystem: tar excludes on the decoded name, so the file still has a dot in
  it on disk and is still dropped. It also makes the links unreadable.

- **Refuse to publish dot paths and warn.** Honest, and consistent with failing
  loudly. Rejected because the annotation is legitimate and the reader wants
  it; the tool's job here is to publish it somewhere reachable, not to explain
  why it cannot.
