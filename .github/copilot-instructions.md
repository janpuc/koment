<!-- koment:managed-start -->
## koment procedure

- Before editing an existing file, call `koment_get` for it and read every annotation. Search with `koment_search` before changing a non-obvious decision.
- Treat ambiguous, drifted and orphaned annotations as history, never as current fact.
- Do not add explanatory inline comments. Prefer a better name, extraction, a named type or constant, or clearer structure; then record local rationale with `koment_add` and honest agent authorship.
- Completed comment intent must use `koment_convert_comment`. Keeping an inline comment requires `koment_acknowledge_comment` with the explicit acknowledgement set to true.
- Before finishing, run `koment check`, `koment comments check` and `koment agents check`. Do not report success while any fails.
- Releases follow `docs/releasing.md` exactly. Published versions are permanent. Never publish an artifact by hand, never hand-edit a version, and get explicit human approval before merging a release pull request.
<!-- koment:managed-end -->
