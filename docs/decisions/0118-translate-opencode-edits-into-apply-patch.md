# 0118 — Translate opencode edits into apply_patch so one comment walker serves every client

Date: 2026-08-05
Status: Accepted

## Context

`koment agents hook pre-tool` exists to deny ordinary Go comment intent before
an agent's tool runs. The codex adapter calls it from `.codex/hooks.json` with
a codex-shaped payload: `{"tool_name":"apply_patch","tool_input":{"command":
"<patch text>"}}`. `agentpolicy.PreToolOutput` unmarshals that and feeds the
patch to `addedCommentIntent`, which walks lines, identifies the file from
`*** Add File:` / `*** Update File:` markers, and flags added `+`-prefixed
lines that look like ordinary explanatory comments.

The opencode adapter runs in `.opencode/plugins/koment.js` and sees an
`edit` or `write` tool call whose `output.args` carries a full file path and
full post-edit content. There is no patch body. Two options were on the
table.

## Decision

The opencode plugin sends a synthetic `apply_patch`-shaped payload to
`koment agents hook pre-tool`:

```json
{
  "tool_name": "opencode_edit",
  "tool_input": { "filePath": "x.go", "content": "..." }
}
```

`agentpolicy.PreToolOutput` recognises the new `tool_name`, calls
`syntheticPatchFromEdit` to wrap the content in an apply_patch body whose
every line is marked added, and feeds that to the same
`addedCommentIntent` walker codex uses. One analyzer implementation
serves every client.

## Consequences

Easier:

- A new client whose tool surface does not match `apply_patch` only needs an
  adapter function; `addedCommentIntent` and its surrounding `PreToolOutput`
  contract are stable.
- Drift between client verdicts is structurally impossible: a file flagged
  by codex is flagged by opencode and vice versa, because the input shape
  is the same once the synthesis has happened.
- Tests added in `hooks_test.go` cover the synthesis path, the same
  `deny`/`allow` verdicts codex produces, and the non-Go short-circuit.

Harder:

- The hook protocol now has two `tool_name` values. The single Go function
  is wider than it was; the test surface is wider. Both are documented.
- The synthesis is a small but real adapter. If a future client emits a
  shape that the synthesis cannot approximate faithfully, the synthesis
  grows or a second input branch is added.

## Alternatives rejected

- **Parse opencode edits as full file content in a new walker.** A second
  implementation of the comment-intent check would let the two clients
  drift silently. The whole point of `addedCommentIntent` is that there is
  one rule. Rejected.
- **Teach `addedCommentIntent` to accept a `filePath + content` pair
  directly.** Possible, but it would force the function to know about a
  second caller and a second input shape, while the patch walker already
  handles a file-prefixed body cleanly. The synthesis keeps the walker
  the single-purpose tool it is.
- **Make opencode produce a real apply_patch.** Out of scope; opencode
  emits whole-file content by design, and we do not control its tool
  surface.
- **Skip the pre-tool guardrail for opencode and rely on
  `koment comments check` in CI only.** Violates the project's own
  principle (DESIGN.md, ADR 0108): every supported client gets the same
  in-editor hook layer. Opencode users would lose the inline deny that
  codex and cursor users have.
