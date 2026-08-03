# 0026 — Three tiers of adoption, and the static one is a product

Date: 2026-08-03
Status: Accepted

Amends ADR 0018, which introduced the static renderer as a way to publish a
demo. The renderer stays; its purpose widens. Amends ADR 0019's list of release
artifacts.

## Context

koment can already be run three ways, and only one of them is presented as a way
to adopt it.

- The CLI and `koment mcp` on a laptop. No infrastructure at all.
- `koment site`, which renders a complete browsable view to static HTML.
- `koment ui`, the container and the Helm chart — a served instance.

The middle one is described in its own `--help` as *"This exists to publish the
demo site (ADR 0018, 0021)"*, and `koment --help` calls it "render the demo
site". That framing is wrong about what the thing does. It produces a full
rendering of a repository's annotations with no server, no authentication
surface, no database, no cost and no operational burden — and we describe it to
users as scaffolding for our own marketing.

The consequence is a gap where adoption should be. A team that starts annotating
gets tier 1 immediately. The next thing we offer them is: run a container, mount
a checkout, terminate TLS, decide who may reach an endpoint that has no
authentication (ADR 0013). Most teams evaluating a tool will not do that to see
whether it is worth using, and asking them to is how a tool gets abandoned at
step one.

Tier 1 also is not as easy as it should be. The only documented install is
`go install`, which presumes a Go toolchain. A reviewer who wants to read their
colleague's annotations should not need one.

## Decision

**koment has three tiers. Each is a supported destination, not a stage in a
funnel.**

| | you run | you get |
|---|---|---|
| **local** | the CLI, and `koment mcp` in your agent's config | annotate, `koment check`, agents read the reasoning. Nothing to host. |
| **published** | `koment site` in CI → GitHub Pages | everyone with repository access reads the annotations in a browser. No server, no authentication to design, no cost. |
| **served** | the container or the Helm chart | the live working tree, several repositories, index-backed search across them, metrics |

**Moving up a tier is not a migration.** All three read the same `.koment/` in
git. There is no export step, no import step, and nothing to back up that is not
already in the repository. A team can start at tier 1, add tier 2 by committing a
workflow file, and add tier 3 years later without touching a single annotation.
That property is what makes the tiering honest rather than a sales ladder, and it
is a direct consequence of ADR 0002 keeping the record in git.

**The published tier is single-repository.** A static site has no server to
resolve a repository reference against. Serving several from one site would take
either an invented path-prefix scheme or a client-side router, and both fake a
capability the tier does not have. One repository, one site. Publishing several
means publishing several sites, which is what this repository's own Pages
deployment does.

**Every tier installs without a Go toolchain.** Releases carry signed checksums
and binaries for linux, macOS and Windows on amd64 and arm64, alongside the
existing container image and Helm chart.

**A published site names the commit it was rendered from.** A snapshot that does
not say what it is a snapshot of invites exactly the staleness this project
exists to prevent, so `koment site` stamps the commit itself rather than leaving
it to whoever wrote the workflow.

**Coverage is the default; divergence has to be forced.** The published tier is
expected to render what the served tier renders. These are the only differences
the design compels, and each is a consequence of having no server:

| the served tier | the published tier |
|---|---|
| resolves against the working tree on every request | is a snapshot of one commit, and says so |
| serves several repositories with a switcher | one repository per site |
| searches an index across repositories | searches only within the site, in the browser |
| exposes metrics | does not |

Anything else that differs between them is a bug, not a tier boundary. In
particular the file tree, the annotation rendering, drift presentation and the
mobile layout are the same code and must stay that way.

**A composite action ships with the tool, versioned with it.** `janpuc/koment@v0.2.0`
installs a pinned koment onto a runner and puts it on `PATH`. It does one thing,
so it composes into a `koment check` gate and a `koment site` publish alike.

## Consequences

- The demo framing leaves the product surface. ADR 0018 and ADR 0021 still
  describe why *this* repository publishes a fixture alongside itself; that is
  now an instance of the documented pattern rather than the reason the pattern
  exists.
- **Two more things to keep working: an action and six release binaries.** The
  action is a thin installer precisely so that it stays cheap.
- **The action cannot be tested until a release carries binaries.** It installs
  a published release, and none exists yet, so a CI job added now would be red
  on arrival for a reason that is not a defect. Until the first such release the
  action is verified only against locally built artifacts of the same shape, and
  that gap is recorded rather than papered over with a conditional that would
  later hide a genuine break. This repository's own Pages workflow does not use
  the action either: it builds from source, because its site has to show what is
  on `main` rather than what was last released.
- Publishing a private repository's annotations makes its source public.
  Rendering a file means rendering its lines. The docs say this at the point of
  use, not in a footnote, because the failure is unrecoverable.
- A tier-2 site is only as fresh as its last build. The commit stamp bounds the
  staleness instead of hiding it, and `koment check` in the same workflow means a
  build that would publish drift fails first.
- Tier 3 keeps the features it earns by having a server. This ADR does not ask
  the static renderer to grow a backend.

## Alternatives rejected

- **Leave the static renderer as demo scaffolding.** No work, and it already
  builds our Pages site. Rejected because it is the only tier a team can adopt
  without an infrastructure conversation, and describing it as a demo hides the
  one path most evaluations would take.
- **One static site multiplexing several repositories**, with a client-side
  router. It would keep tier 2 and tier 3 feature-identical, which this ADR
  otherwise treats as the goal. Rejected because it would ship a router, a
  URL scheme and a search index into the browser to imitate a server — the
  divergence would move from "a missing feature, stated" to "a reimplementation
  that behaves subtly differently", which is worse.
- **A reusable workflow** (`uses: janpuc/koment/.github/workflows/pages.yml@v1`)
  instead of an action plus a documented workflow. It is genuinely one line for
  the user, and unlike a composite action it *can* declare `permissions` and
  `environment`. Rejected because every knob a user might need — banner text, a
  second repository, running their own build first, a different publish target —
  becomes an input we have to invent up front, and the deployment permissions
  they are granting become invisible in their own repository. Thirty lines they
  own beats one line they cannot read.
- **A composite action that performs the whole Pages deploy.** Rejected on a
  hard fact rather than taste: a composite action cannot declare job-level
  `permissions`, `concurrency` or `environment`, which is most of what a Pages
  workflow is. It would hide the trivial part and leave the fiddly part.
- **Require the served tier for any browser view.** One code path, one story,
  and it pushes people toward the tier with the most features. Rejected because
  it puts a hosting decision in front of a evaluation, and because the static
  renderer already exists and works.
- **A hosted koment service.** The lowest-friction tier imaginable. Rejected: it
  would mean taking custody of other people's source code, which contradicts the
  premise that the record lives in their git repository, and it is an operational
  commitment this project cannot honour.
- **Publish only a container and tell people to `docker run` locally.** No new
  release artifacts. Rejected because `koment add` is a working-tree command run
  next to an editor, and a container makes the most common operation the most
  awkward one.
