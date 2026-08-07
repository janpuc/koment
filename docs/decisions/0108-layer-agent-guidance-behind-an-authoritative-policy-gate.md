# 0108 — Layer agent guidance behind an authoritative policy gate

Date: 2026-08-03
Status: Superseded by 0124

## Context

A koment-enabled repository asks an agent to read external rationale before it
edits code, to create annotations instead of explanatory comments and to
acknowledge the rare inline exception. Merely installing MCP makes tools
available but does not make an agent use them. Hand-written setup pages also
leave each client with slightly different, eventually stale instructions.

Repository instruction files and agent hooks are useful but not universal
mediation. Clients can decline repository trust, disable custom instructions,
omit hooks for some write tools or run a shell outside the client. MCP server
instructions are advisory client context. A Git hook is bypassable. Repository
files therefore cannot guarantee what an arbitrary process types or writes.

The client boundaries above come from the current official documentation for
[MCP initialization](https://modelcontextprotocol.io/specification/2025-06-18/basic/lifecycle),
[GitHub Copilot repository instructions](https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions),
[Cursor rules](https://docs.cursor.com/context/rules) and
[Codex hooks](https://learn.chatgpt.com/docs/hooks.md). They are capabilities
koment integrates with, not assumptions that every client implements the same
control surface.

The meaningful guarantee is narrower and enforceable: prohibited commentary
cannot land in a protected branch. Local agent controls should make violations
rare and immediately repairable rather than being presented as the final
boundary.

## Decision

Store one version-1 machine-readable repository policy at
`.koment/policy.yaml`. It selects strict comment handling, intrinsic comment
classes, generated and vendored paths, and supported agent adapters. Comment
classification, local hooks and CI consume that same policy. It cannot grant a
path-wide exemption for ordinary source commentary.

Provide `koment agents install` and `koment agents check`. Installation adds or
refreshes a managed contract in root `AGENTS.md` and thin repository-owned
adapters for supported clients without replacing unrelated project
instructions. Checking renders the expected contract from the policy and fails
when an installed adapter omits a mandatory rule or has drifted.

The contract requires an agent to:

1. read annotations before editing an existing file and search them before a
   non-obvious structural decision;
2. treat unresolved annotations as history rather than current fact;
3. prefer names, extraction, types and structure, then create an attributed
   annotation instead of an explanatory comment;
4. use the explicit conversion or acknowledgement mutation for completed
   comment intent; and
5. run the annotation and comment-policy checks before completion.

Repeat the contract in MCP initialization instructions and expose the mutation
tools only in explicit write mode. Install client-specific early feedback where
a trusted repository can do so: an always-applied rule or session instruction,
a pre-write check for obvious new commentary and a completion hook that refuses
to finish while the authoritative local checks fail.

Make `koment comments check` a required protected-branch status. It parses each
supported language, accepts intrinsic comments and exact attributable
acknowledgements, and prints a concrete conversion or acknowledgement action
for every failure. This required status, not agent obedience or a workstation
hook, enforces the guarantee that prohibited commentary cannot land.

## Consequences

- A fresh supported agent session receives the procedure alongside the tools
  needed to follow it.
- Humans and agents receive the same policy result even when their clients have
  different hook capabilities.
- Generated adapters become product surface and need compatibility tests
  against the client formats koment claims to support.
- Repositories must review and commit adapter refreshes when the canonical
  contract changes.
- Local bypass remains possible and is stated honestly; protected-branch
  configuration is required for the landing guarantee.
- Enterprise-managed client policy can strengthen local enforcement without
  becoming a baseline requirement for ordinary repositories.

## Alternatives rejected

- **Rely on `AGENTS.md` alone.** Broadly useful and human-readable, but agents
  can ignore it and several clients use additional instruction surfaces.
- **Rely on MCP server instructions.** Places guidance near the tools, but the
  protocol presents instructions as optional client context and an agent can
  edit without invoking MCP.
- **Treat client hooks as universal enforcement.** They give excellent early
  feedback, but support and tool coverage differ and repository trust can be
  declined.
- **Maintain unrelated prose for every client by hand.** Easy to start, but the
  contracts drift and reviewers cannot prove that every mandatory rule remains
  present.
- **Use only a Git hook.** Helpful locally, but intentionally bypassable and not
  installed by an ordinary clone.
- **Intercept all filesystem writes.** Could mediate a narrowly controlled
  sandbox, but is hostile, platform-specific and still does not cover edits
  outside that sandbox. It would turn koment into an execution environment
  instead of a rationale system.
- **Silently rewrite every detected comment.** Loses directives and temporary
  code, hides ambiguity and violates the record-first conversion guarantee.
