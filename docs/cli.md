# CLI reference

```
koment add <file> [--excerpt <text>] --kind <kind> --body <text|->
koment show <file>
koment check [path...]
koment list [--kind <kind>] [path...]
koment reanchor <id> [--excerpt <text>] [--file <path>]
koment index [--rebuild] [--database-url <url>]
koment ui [--listen <addr>]
koment mcp [--http <addr> | --streamable-http <addr>]
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

Exits `1` if any annotation for that file is drifted or orphaned. A file with no
annotations says so and exits `0`.

## check

The drift gate. Resolves everything and fails on `drifted` or `orphaned`.

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

Exits `1` if anything shown is drifted or orphaned.

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

At least one is required. The SHA-256 and line are recomputed, never typed. The
new excerpt is validated exactly as `add` validates one. Ids come from `check`
output, ready to paste.

## index

Builds or refreshes the derived index that the serving read paths query.

```sh
koment index              # refresh: re-resolve only files that changed
koment index --rebuild    # discard and rebuild from YAML
```

| flag | |
|---|---|
| `--rebuild` | throw the index away and rebuild it |
| `--database-url <url>` | use Postgres instead of SQLite |
| `--index <path>` | SQLite file; defaults to a per-repository file in the cache directory |
| `--name <name>` | repository name recorded in the index |

You rarely need to run this by hand — the servers keep the index current. It
exists for a cold start, for a Postgres deployment, and for when you want to see
what the index thinks.

The index is **derived**. Deleting it costs a rebuild and nothing else; the
annotations themselves are the YAML in git. It is gitignored for that reason.

## ui

Local read-only web view — your code with its annotations in the margin.

```sh
koment ui
koment ui --listen 8080
```

Binds loopback and prints the URL. A bare port gets `127.0.0.1` prefixed; bind
anywhere else and koment warns, because there is no authentication.

## mcp

The MCP server. See [agent setup](agents/).

```sh
koment mcp                            # stdio, the default
koment mcp --http 8765                # HTTP, JSON responses
koment mcp --streamable-http 8765     # HTTP, server-sent events
```

Exposes `koment_get(file)` and `koment_search(query)`. stdio is what you want
unless the agent cannot spawn a subprocess.

---

## Record format

`.koment/annotations/<mirrored source path>.yaml`:

```yaml
version: 1
file: internal/store/ulid.go
annotations:
  - id: 01KYW1ETE3CVB6S0ND70GGZVWM
    scope: excerpt
    excerpt: "\tpaddingBits = ulidLength*bitsPerChar - 8*(...)"
    excerpt_sha256: b3f62f10e117b769...
    last_seen_line: 18
    kind: gotcha
    body: |-
      26 Crockford characters carry 130 bits but a ULID holds 128, so the
      value is left-padded by two.
    created: "2026-07-31"
```

Hand-editing works, and `reanchor` exists so you rarely need to. If you do edit
by hand, `excerpt_sha256` must match the excerpt or koment refuses to load the
record rather than trusting a stale hash.

`last_seen_line` is a hint, never an anchor: resolution searches on the excerpt
alone and reads the line only to tell `ok` from `moved`. Delete it and nothing
stops resolving.
