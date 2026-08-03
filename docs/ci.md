# CI and pre-commit

Three checks enforce a koment-enabled repository:

- `koment check` rejects unresolved annotations.
- `koment comments check` rejects ordinary inline comments that bypass koment.
- `koment agents check` rejects missing or stale generated agent adapters.

## GitHub Actions

```yaml
name: annotations

on:
  push:
    branches: [main]
  pull_request:

jobs:
  koment:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: janpuc/koment@v0.2.0
      - run: koment check
      - run: koment comments check
      - run: koment agents check
```

The action downloads a released binary, verifies it against the release's
checksums and puts it on `PATH`. Pin a release with `with: { version: 0.2.0 }`;
it defaults to the latest. Linux and macOS runners.

Already have a job with koment installed? Add the same three gates:

```yaml
      - run: |
          koment check
          koment comments check
          koment agents check
```

To publish the annotations as well as check them, see
[publishing](publishing.md) — it is the same job with four more lines.

## GitLab CI

```yaml
annotations:
  script:
    - koment check
    - koment comments check
    - koment agents check
```

Install the checksum-listed release binary in the runner image or job bootstrap;
end-user and CI installations do not build koment from source.

## Pre-commit

Catching drift before it is pushed is kinder than catching it in review.

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: koment
        name: annotations still resolve
        entry: koment check
        language: system
        pass_filenames: false
      - id: koment-comments
        name: comments follow koment policy
        entry: koment comments check
        language: system
        pass_filenames: false
      - id: koment-agents
        name: agent adapters are current
        entry: koment agents check
        language: system
        pass_filenames: false
```

`pass_filenames: false` matters: `koment check` takes paths to *narrow* to, and
handing it the staged file list would silently skip everything else — including
the annotation you just broke in a file you didn't stage.

Plain git hook, no framework:

```sh
# .git/hooks/pre-commit
#!/bin/sh
koment check && koment comments check && koment agents check
```

## Narrowing

```sh
koment check internal/ cmd/
```

Useful in a monorepo where one team owns a subtree. Note the caveat above:
narrowing means annotations outside those paths are not checked, and an edit
inside your subtree can perfectly well drift an annotation outside it.

## What a failure looks like

```
internal/store/ulid.go
  drifted   gotcha        internal/store/ulid.go  01KYW1ETE3CVB6S0ND70GGZVWM
    26 Crockford characters carry 130 bits but a ULID holds 128, so the value
    is left-padded by two. Drop the padding and every character shifts.
11 annotations across 8 files: 8 ok, 2 moved, 1 drifted
koment: 1 annotations no longer resolve; revisit them or update the anchor
```

Failures print to stdout with the annotation body, so whoever reads the CI log
has the reasoning in front of them and can judge it without checking anything
out. The summary goes to stdout; the count of failures to stderr.

## Handling a red build

Do not delete the annotation to go green. Read it, then:

```sh
koment reanchor <id> --excerpt '<the code that replaced it>'
koment reanchor <id> --file <the new path>
```

Commit the updated record alongside the change that caused the drift. Reviewers
then see the code change and the reasoning change together, which is the entire
argument for keeping annotations in the repository.

## Should drift block a merge?

Yes — that is the design. The resolution, comment-policy and adapter checks
should be required together.

If a large refactor produces drift you genuinely intend to resolve later, make
that visible rather than silent: `continue-on-error: true` on the step keeps the
signal in the log while unblocking the merge. Turning the check off entirely
gets you back to comments that rot, with a YAML file.
