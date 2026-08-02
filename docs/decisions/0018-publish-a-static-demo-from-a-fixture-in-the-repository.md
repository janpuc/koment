# 0018 — Publish a static demo from a fixture in the repository

Date: 2026-08-02
Status: Accepted

Reverses the rejection of static export in ADR 0013.

## Context

This decision has been made three times and deserves recording once properly.

ADR 0013 rejected a static site generator: *"it goes stale the moment the code
changes, which is the failure mode koment exists to prevent."* An exporter was
then built anyway for a demo, on the argument that a snapshot of a named commit
is not a stale view of a working tree. It was then deleted unshipped in favour
of a live deployment, on the argument that a tool arguing against snapshots
should not be demonstrated by one. The live deployment was then dropped when it
turned out that neither preferred host runs a Go server for free.

What survives all of that is a narrower question than "static or live": **what
should the demo show?**

A live deployment of koment's own repository shows `ok` and `moved` and nothing
else, because CI keeps `main` green — `drifted` and `orphaned` fail the build.
The two statuses a visitor most needs to understand are exactly the two the real
repository can never display. A demo that cannot show the failure modes is a
poor demo however it is served.

That reframes the trade entirely. A fixture repository built to exercise all
four statuses is *better* content than the real one, and a fixture does not
change between deploys, so nothing is gained by serving it live.

## Decision

The demo is static HTML on GitHub Pages, rendered from a fixture repository
committed at `demo/`.

- `demo/` is a small, self-contained, deliberately annotated codebase carrying
  `ok`, `moved`, `drifted` and `orphaned` annotations. Its drift is intentional
  and permanent.
- `koment export --out <dir>` renders it with the same templates, view model and
  `anchor.Resolve` calls that `koment ui` serves.
- CI regenerates and deploys on every push to `main`. The demo cannot drift from
  the code because it is rebuilt from it.
- The fixture keeps **its own store** at `demo/.koment/`. `store.FindRoot`
  stops at the nearest `.koment`, so the fixture's deliberate drift is invisible
  to the repository's own `koment check` and needs no path exclusion. It also
  makes the fixture a faithful example of a real single-repository store.
- Every page states that it is a fixture, not a live view.

`koment export` is documented as the mechanism behind the demo. It is not
offered as a way to read your own annotations — that is `koment ui`, which
re-reads the working tree per request. ADR 0013's objection stands for real
work; it simply does not apply to a fixture that is regenerated from source.

## Consequences

- A visitor can see all four statuses, including the two that matter most, which
  no live deployment of this repository could show.
- Free, no infrastructure, no host to keep alive, no credentials, nothing to
  page anyone at 3am for a demo.
- **A second rendering path exists.** Mitigated by sharing the view model and
  templates with the server: only link generation differs, because a static tree
  needs relative hrefs. A test asserts the two produce the same content.
- The fixture is a maintenance burden — it must keep exercising every status as
  the renderer changes, and its deliberate drift will look like a mistake to
  anyone who does not read this ADR.
- Two annotation stores exist in one working tree. That is unusual, and it is
  also exactly what `FindRoot` was designed to do, so it exercises a real code
  path rather than a special case.
- Export exists in the command surface, and someone will eventually point it at
  a private codebase and publish the result. The help text says so plainly.

## Alternatives rejected

- **A live `koment ui` deployment.** The most faithful demonstration, and it was
  the decision for about an hour. Rejected on two independent grounds: it cannot
  show `drifted` or `orphaned` for a green repository, and no free host among
  the candidates runs a container — Cloudflare Containers needs Workers Paid,
  Fly.io ended its free tier for new users in 2024, and Koyeb closed free
  signups in early 2026. Cloud Run would work and costs an operational
  dependency on a cloud account for a demo page.
- **Static export of koment's own repository rather than a fixture.** Honest,
  self-updating, and doubles as dogfooding. Rejected because a green repository
  can only ever show two of the four statuses, and the demo's job is to explain
  what drift looks like.
- **Screenshots or a recorded walkthrough.** Cheapest. Rejected because it
  cannot be clicked, and because it goes stale silently — with nothing that
  detects it, which is a poor look for this project specifically.
- **Generating the fixture at build time instead of committing it.** Keeps a
  deliberately broken codebase out of the tree. Rejected because a generated
  fixture is harder to reason about than a checked-in one, and its drift would
  become a property of a script rather than something a reader can open and see.
