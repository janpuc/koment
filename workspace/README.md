# Maintained workspace

This session package is part of koment's verification surface. It is small
enough to read in one sitting, useful enough to exercise real invariants, and
kept green rather than carrying intentionally stale rationale.

Run it from the repository root:

```sh
go test ./workspace/...
cd workspace && go run ../cmd/koment check
```

The published koment site exposes this workspace through the normal repository
switcher. It uses the same source, records, resolver and rendering path as any
other repository.
