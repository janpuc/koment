# 0025 — The MCP surface becomes repository-aware

Date: 2026-08-02
Status: Accepted

Amends ADR 0005, which fixed the tool surface at exactly two. It is now three,
and this ADR is the argument for why that is not the start of a slide.

## Context

ADR 0005 chose two tools and told the next reader to defend that number:

> MCP tool schemas cost context in every request. Keep the surface to two tools;
> resist growing it. An unfiltered MCP server measured elsewhere in this stack
> cost ~154k tokens of schema per request — tool count is a real budget.

That instruction was right and it is being tested now rather than ignored.

With a registry (ADR 0024) a deployment serves several repositories, and the
current tools cannot express which one. `koment_get("internal/store/store.go")`
is ambiguous the moment two repositories have that path, and today it resolves
to whichever repository the server's working directory happens to sit in — a
confident wrong answer, which is the failure mode this project exists to
prevent.

Adding an optional `repository` parameter to both tools fixes selection. It does
not fix **discovery**: an agent has no way to ask what is served, so it must
either guess a repository name or learn one by accident from a search result.
Guessing is what produces the confident wrong answer.

## Decision

```
koment_get(file, repository?)
koment_search(query, repository?)
koment_repositories()            ← new
```

**`koment_repositories`** returns the served repositories with their id, name,
default branch and annotation counts by status. It is a few hundred bytes of
schema and it is what turns "guess a repository" into "ask".

**When `repository` is omitted**, the behaviour differs per tool because the
right answer differs:

- `koment_get` resolves the path if exactly one repository has it, and otherwise
  **fails, listing the candidates**. It does not pick. A path that exists in two
  repositories is a genuinely ambiguous question and answering it silently is
  the failure above.
- `koment_search` searches **all** repositories, and every match carries its
  repository. Searching broadly and reporting where each hit came from is
  useful; refusing to search without a repository would not be.

Every annotation returned carries its repository, so a result is never
detached from its scope.

**The surface stops at three.** Three is not two because discovery is a distinct
question from retrieval and from search, and there is no fourth question of that
kind. A per-repository tool set was rejected precisely because it grows with the
data; this does not.

## Consequences

- An agent working across several repositories can find out what exists and ask
  about the right one, rather than inferring from a working directory it cannot
  see.
- **An ambiguous `koment_get` now fails where it previously answered.** That is
  the intended change and it will look like a regression to anyone whose
  deployment has two repositories sharing a path. The error names the
  candidates, so the fix is to pass `repository`.
- One more tool schema in every request. Measured against ADR 0005's concern:
  three small tools remain far below the budget that motivated the rule. The
  rule is unchanged — resist growth — and this ADR is the record of the one
  time it was overridden and why.
- Single-repository deployments are unaffected. `repository` is optional
  everywhere, and with one repository every omission resolves.
- The tool descriptions must now explain when `repository` is needed, which is
  more prose in the schema. Worth it: an agent that reads the description and
  passes the parameter never hits the ambiguity error.

## Alternatives rejected

- **A `repository` parameter with no discovery tool.** Keeps ADR 0005's number
  intact, and was tempting for that reason alone. Rejected because an agent
  would have to learn repository names by side effect — running a search and
  reading the tags — which is discovery by accident rather than by design, and
  leaves an agent that knows what it wants unable to ask for it.
- **One tool set per repository** (`koment_get_api`, `koment_get_web`, …).
  Unambiguous, no optional parameters, no error case. Rejected outright: schema
  cost grows with the number of repositories, which is exactly the unbounded
  growth ADR 0005's measurement was warning about.
- **Picking a default repository when `koment_get` is ambiguous** — first
  configured, or the working directory's. Rejected because it answers a question
  the caller did not ask with data from a repository they may not have meant,
  and it does so silently. Failing with the candidates listed costs one round
  trip and cannot be wrong.
- **Exposing repositories as an MCP *resource* rather than a tool.** Arguably
  the more correct protocol shape, and it would leave the tool count at two.
  Rejected because resource support is uneven across the clients koment
  targets, and ADR 0005 chose tools-only for that reason; a tool that works
  everywhere beats a resource that works in some clients.
- **Encoding the repository in the path** (`api:internal/store.go`). No new
  parameter, no new tool. Rejected because it overloads a field whose format is
  already meaningful, and every client would have to learn a koment-specific
  path syntax.
