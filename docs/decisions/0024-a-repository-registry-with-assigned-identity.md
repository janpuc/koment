# 0024 — A repository registry, with assigned identity

Date: 2026-08-02
Status: Accepted

Supersedes the derived-identity detail of ADR 0017. The rest of 0017 — the
repository as the unit of storage, isolation being structural rather than a
filter — stands.

## Context

ADR 0017 made the repository a first-class scope in the *data model* and then
nothing made it first class anywhere else. The index is keyed by
`repository_id`, which was the easy half. Everything around it assumes one:

- Configuration is `KOMENT_REPO`, a single path.
- The MCP server serves whatever repository its working directory sits in.
- `RepositoryFor` derives the id by hashing the root path.

That last one is the part that quietly breaks. ADR 0017 recorded the failure and
then shipped it anyway: *"the URL is not stable: moving host, renaming the org,
or switching SSH to HTTPS would each fork the repository's identity and orphan
its sync state."* The same is true of a filesystem path. Move a checkout from
`/repos/api` to `/srv/api` and it becomes, as far as koment is concerned, a
different repository — new id, orphaned index, and any per-repository state
attached to the old one is stranded with no error to say so.

A single-repository deployment never notices, because there is nothing to
confuse it with. That is exactly why it survived this long.

## Decision

**Repositories are configured and their identity is assigned, not computed.**

A registry holds the set koment serves. Each entry carries what ADR 0017
specified:

```yaml
repositories:
  - id: api                      # assigned once, never derived
    name: Payments API           # display only
    root: /repos/api
    clone_url: https://github.com/you/api
    default_branch: main
```

Three sources, in precedence order, so the simple cases stay simple:

| source | for |
|---|---|
| `KOMENT_CONFIG` — a YAML file | several repositories, or per-repository settings |
| `KOMENT_REPOS` — `api=/r/api,web=/r/web` | a container with a few mounts and nothing else to say |
| `KOMENT_REPO`, or the working directory | one repository — unchanged from today |

The id is the identity. It comes from the config, or from the name in
`KOMENT_REPOS`. Moving the checkout, changing the remote, or renaming the
display name all leave it alone, so the index, the sync state and anything later
attached to a repository follow it.

**The single-repository case is unchanged and needs no configuration.** With no
registry configured, koment discovers one repository by walking up from the
working directory exactly as it does now, and gives it the directory's basename
as an id. Nothing about a laptop or a single-repo container changes.

**A configured id may not change.** Changing it in config produces a new
repository with no history, which is a rename rather than an edit. koment cannot
detect the difference, so the docs say it plainly rather than pretending.

## Consequences

- Repositories survive being moved, which is the whole point.
- Multi-repository becomes configurable rather than implied, and the MCP server
  and UI have something real to enumerate (ADR 0025).
- **Two more ways to configure the same thing.** Three sources is genuinely more
  surface than one, and it is justified by the gap between "a laptop, one repo,
  zero config" and "a deployment with per-repository clone URLs". Collapsing
  either into the other would penalise one of them.
- A YAML config file is a new format to version and validate. It reuses the YAML
  dependency already present for the record format, and it is validated on load
  rather than on first use.
- The registry is deployment-level state that lives outside every repository.
  Losing it costs identity, not annotations — the annotations are still in each
  repository's `.koment/`, and a rebuilt registry with the same ids restores
  everything. Losing it *and* reassigning different ids orphans indexed state,
  which a rebuild fixes.
- Nothing here writes to a repository. The registry describes what to serve; the
  record is still the YAML in git.

## Alternatives rejected

- **Keep deriving the id from the root path.** No configuration, works today,
  and it is what shipped. Rejected because moving a checkout silently forks a
  repository's identity — no error, no warning, just an empty index and a
  stranded old one. ADR 0017 named this failure and accepted it provisionally;
  a registry is the condition it was waiting for.
- **Derive the id from the git remote URL.** More stable than a path and
  survives a local move. Rejected for the reason ADR 0017 gave: remotes change
  host, org and protocol, and a repository with no remote has no identity at all.
- **A config file only, with no environment list.** One source of truth and
  nothing to parse. Rejected because it forces a mounted file on a container
  serving two repositories, which is the common small case.
- **Environment only, with no file.** Twelve-factor and no format to design.
  Rejected because there is nowhere to put a clone URL or a default branch, and
  those are in ADR 0017's model for reasons that have not gone away.
- **Auto-discovering repositories by scanning a directory for `.koment/`.**
  Zero configuration for the multi-repository case too. Rejected because
  identity would again be positional — the thing this ADR exists to fix — and a
  scan makes what koment serves depend on what happens to be on disk.
