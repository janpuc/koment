# CLI reference

```
koment add <file> [--excerpt <text>] --kind <kind> --body <text|->
koment show <file>
koment check [path...]
koment list [--kind <kind>] [path...]
koment search <query>
koment reanchor <id> [--excerpt <text>] [--file <path>]
koment comments check [path...]
koment comments convert <file> --excerpt <comment> [--kind <kind>]
koment comments acknowledge <file> --excerpt <comment> --body <text|-> --acknowledge-inline-comment
koment agents install|check
koment ui [--listen <addr>] [--write]
koment site --out <dir>                  # render a repository snapshot to static HTML
koment mcp [--write | --http <addr> | --streamable-http <addr>]
koment version
```

Exit codes: `0` fine, `1` drift or failure, `2` misuse.

koment finds your repository by walking up from the working directory looking
for `.koment/`, then `.git/`.

---

## add

Records a new annotation.

```sh
koment add internal/auth/token.go \
    --excerpt 'if token.Expiry.Before(now.Add(-clockSkew)) {' \
    --kind gotcha --body 'Bit us in #412.'
```

| flag | |
|---|---|
| `--excerpt <text>` | verbatim snippet to anchor to. Omit for a file-scoped annotation. |
| `--kind <kind>` | `why`, `gotcha`, `invariant` or `anti-pattern`. Required. |
| `--body <text>` | the rationale. `-` reads stdin, which is easier for prose. |

The excerpt must appear **exactly once**. Absent or ambiguous is refused, so a
bad anchor is caught while you are still there to fix it. Prints the new id.

The file may come before or after the flags.

## show

Annotations for one file, resolved against what is on disk now.

```sh
koment show internal/auth/token.go
```

Exits `1` if any annotation for that file is ambiguous, drifted or orphaned. A
file with no annotations says so and exits `0`.

## check

The drift gate. Resolves everything and fails on `ambiguous`, `drifted` or
`orphaned`.

```sh
koment check
koment check internal/ cmd/
```

Prints only failures, plus a summary line. With paths, only annotations for
files under those paths are checked — convenient in a monorepo, and a way to
miss drift elsewhere.

## list

Everything, for review.

```sh
koment list
koment list --kind invariant
koment list internal/store
```

Exits `1` if anything shown is ambiguous, drifted or orphaned.

## search

Searches annotation ids, files, kinds, bodies and author identity through the
same repository snapshot as every other reader.

```sh
koment search 'clock skew'
```

Matches include their current resolution status and full body. The command
exits `1` when a matching record is ambiguous, drifted or orphaned so scripted
readers cannot mistake stale rationale for current fact.

## reanchor

Repoints an annotation without touching YAML. Keeps its id and creation date —
it is the same annotation, only where it points changed.

```sh
koment reanchor 01KYW1ETE3CVB6S0ND70GGZVWM --excerpt 'if token.Expired(now) {'
koment reanchor 01KYW1ETE3CVB6S0ND70GGZVWM --file internal/auth/session.go
```

| flag | |
|---|---|
| `--excerpt <text>` | new snippet, in the target file |
| `--file <path>` | move to another file; keeps the existing excerpt unless `--excerpt` is also given |

At least one is required. The surrounding context and last seen line are
recaptured from source, never typed. The new excerpt is validated exactly as
`add` validates one. Ids come from `check` output, ready to paste.

## site

Renders a repository snapshot to static HTML — the published tier
([ADR 0103](decisions/0103-three-tiers-with-human-and-agent-capabilities.md)).
See [publishing](publishing.md) for the workflow to copy.

```sh
koment site --out dist
koment site --out dist --name myrepo --commit-link "$url/commit/$sha"
```

| | |
|---|---|
| `--out <dir>` | where to write. Required. |
| `--name <text>` | repository name on every page; defaults to the repository's own |
| `--commit <sha>` | the commit rendered; read from git when omitted |
| `--commit-link <url>` | make the commit clickable |
| `--banner <text>` · `--banner-link <url>` | a notice on every page |
| `--repository <id>` | which repository, when several are configured |
| `--repository-links <name=URL,...>` | contextual switcher entries for a grouped publication |

Every page names its commit, and `koment site` **refuses to render** when it
cannot determine one: a snapshot that does not say what it is a snapshot of is
how a stale rendering passes for the current tree. Pass `--commit` outside git.

It is a snapshot, not your working tree — use `koment ui` for that, which
re-resolves on every request. The shared target behaviour is defined by
[ADR 0102](decisions/0102-one-repository-snapshot-for-every-reader.md). A site
renders your source as well as your annotations.
The output directory is replaced atomically and also contains full
`annotations.json` and flattened `search.json` projections.

## comments

`koment comments check` is the authoritative gate preventing ordinary Go
comments from bypassing the repository procedure. Toolchain directives,
generated markers, upstream links, deprecation markers and public API
documentation are classified through `.koment/policy.yaml`.

```sh
koment comments check
koment comments convert internal/auth/token.go --excerpt '// Explain why.' --kind gotcha
koment comments acknowledge internal/auth/token.go \
  --excerpt '// Required external marker.' \
  --body 'The generator consumes this exact marker.' \
  --acknowledge-inline-comment
```

Conversion writes an attributed annotation before removing the comment.
Acknowledgement keeps the comment and creates an exact, attributed policy
record; omitting the explicit flag is always rejected.

## agents

`koment agents install` creates the strict default `.koment/policy.yaml` and
generates the selected repository instructions, MCP configs and supported
client hooks while preserving unrelated configuration. Run it again after a
policy or adapter change. `koment agents check` fails when any managed surface
is missing or stale.

## ui

Local web view — your code with its annotations in the margin.

```sh
koment ui
koment ui --listen 8080
koment ui --write
```

Binds loopback and prints the URL. `--write` prints a capability-bearing
bootstrap URL and is refused on non-loopback addresses. Opening that URL sets a
same-site, HTTP-only capability cookie; the visible form writes human-authored
annotations through the same application service as CLI and MCP.

## mcp

The MCP server. See [agent setup](agents/).

```sh
koment mcp                            # stdio, the default
koment mcp --write                    # stdio plus mutation tools
koment mcp --http 8765                # HTTP, JSON responses
koment mcp --streamable-http 8765     # HTTP, server-sent events
```

Exposes `koment_get(file, repository?)`, `koment_search(query, repository?)` and
`koment_repositories()`. stdio is what you want unless the agent cannot spawn a
subprocess.

`--write` adds `koment_add`, `koment_reanchor`, `koment_convert_comment` and
`koment_acknowledge_comment`. It is valid only over local stdio; HTTP transports
are always read-only.

With several repositories configured, an ambiguous `koment_get` fails and names
the candidates instead of picking one.

---

## Record format

`.koment/annotations/<id>.yaml`:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/janpuc/koment/main/schema/annotation.schema.json
version: 1
id: 01KYW1ETE3CVB6S0ND70GGZVWM
file: internal/store/ulid.go
kind: gotcha
body: |-
  26 Crockford characters carry 130 bits but a ULID holds 128, so the
  value is left-padded by two.
created: "2026-07-31"
anchor:
  scope: excerpt
  excerpt: "\tpaddingBits = ulidLength*bitsPerChar - 8*(...)"
  before: const (
  after: )
  last_seen_line: 18
author:
  name: Jan Pucilowski
  kind: human
  source: git-config
```

Hand-editing works, with schema completion in editors that support YAML schemas.
Unknown fields and a filename that differs from `id` are rejected. `reanchor`
exists so context is normally captured from source rather than typed.

`last_seen_line` is descriptive, never an anchor: exact text and the captured
`before` and `after` context choose identity. The line only distinguishes `ok`
from `moved` after one candidate remains.
