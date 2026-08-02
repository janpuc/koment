# 0021 — The demo is two repositories, one of them koment itself

Date: 2026-08-02
Status: Accepted

Refines ADR 0018, which established the static demo. The mechanism is unchanged;
what it shows is not.

## Context

ADR 0018 chose a fixture over koment's own repository on one argument: a green
repository can only ever show `ok` and `moved`, because `drifted` and `orphaned`
fail CI, and a demo that cannot show the failure modes is a poor demo.

That argument is sound and it threw away something more valuable than it saved.

A fixture is a toy. Nobody reading `demo/src/session.go` learns whether koment
is worth using, because the annotations were written to demonstrate koment
rather than because someone needed them. The most persuasive thing this project
has is that **it is used on itself**: the annotations in `internal/` were
written because a real edit was about to lose a real reason, and they are
readable by anyone deciding whether the idea works.

The demo should be self-referential and genuinely usable — a visitor should be
able to read koment's own reasoning, not a staged one. But dropping the fixture
loses the failure modes again.

ADR 0017 already resolved this shape for the product: the repository is a
first-class scope, and a deployment may serve several. The demo is simply the
first place that model has to be real.

## Decision

The demo serves **two repositories**:

| repository | what it is | shows |
|---|---|---|
| `koment` | koment's own store, exported from the repository root | `ok`, `moved` — real annotations about real code |
| `demo` | the fixture at `demo/` with its own store | all four, including `drifted` and `orphaned` |

The published site has a landing page listing both with their status tallies,
and each repository is exported under its own path. Every page carries the
repository it belongs to.

The fixture stops pretending to be the main event and becomes what it always
was: the place the failure modes are demonstrated, kept small and honest about
being a fixture. koment's own store becomes the thing a visitor actually reads.

This is the first exercise of ADR 0017's model. It is deliberately the cheap
version — two exports and an index, not a repository registry with sync state —
because a static site needs none of that, and building the registry to publish
two directories would be inventing the deployment before anything needs it.

## Consequences

- **The demo is worth reading.** It contains this project's actual reasoning:
  why the excerpt and the commit hash are different mechanisms, why the servers
  are injected rather than imported, why an unverified identity must not render
  like a checked one. That is a better argument for koment than any fixture.
- It is self-referential in a way that is the point rather than a gimmick: the
  tool's own explanation of itself is delivered by the tool.
- All four statuses remain visible, so ADR 0018's objection is still answered.
- Two exports to keep working, and a landing page that has to know about both.
  Small, and it is the shape the product is heading toward anyway.
- The demo's status tallies now change as koment changes, which makes the site a
  weak form of dogfooding signal: if `koment` ever shows drift there, CI has
  already failed and the site is telling on us.
- Multi-repository in the *UI* is still not built. The landing page links two
  independently exported trees; there is no repository switcher inside a page,
  no cross-repository search, and no shared navigation. Saying so here stops
  the demo being read as evidence that ADR 0017 is implemented.

## Alternatives rejected

- **Fixture only, as ADR 0018 decided.** Shows every status and needs one
  export. Rejected because it is unpersuasive: a visitor sees invented
  annotations about invented code, and learns nothing about whether real
  annotations are worth writing.
- **koment's own repository only.** Self-referential, honest, zero fixture to
  maintain — and it can never show `drifted` or `orphaned` while CI is green,
  which is exactly ADR 0018's argument and still correct.
- **Deliberately drifting one of koment's own annotations to demonstrate it.**
  Would give both properties in one repository. Rejected outright: it means
  committing a lie about koment's own code and permanently failing its own drift
  gate, to make a screenshot look better.
- **Building the real multi-repository UI now to serve the demo.** The demo
  would then be a true preview of the product. Rejected as backwards — that is a
  large feature driven by a publishing need rather than by use, and ADR 0017 is
  explicit that the registry should follow real multi-repository use.
