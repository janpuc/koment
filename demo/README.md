# demo fixture

Not a real project. A small codebase whose annotations are deliberately in every
resolution state, so the published demo can show what `drifted` and `orphaned`
look like — which koment's own repository never can, because CI keeps it green.

The drift here is intentional and permanent. `koment check` is run against
`cmd`, `internal` and `docs` only, so this directory does not fail the build.
See ADR 0018.
