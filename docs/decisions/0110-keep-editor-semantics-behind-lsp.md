# 0110 — Keep editor semantics behind LSP

Date: 2026-08-03
Status: Accepted

## Context

Editors need to render annotations inline, diagnose stale anchors and turn a
new explanatory comment into a durable annotation. The browser, CLI and MCP
already share the Go application and snapshot services. Reimplementing record
validation, anchoring or comment conversion in each extension would create a
second product contract and could let an editor delete prose before its
annotation was durable.

The Language Server Protocol supplies portable hover, diagnostic, code-lens,
code-action and command primitives. Editor-native APIs are still needed for
virtual inline text and the confirmation flow around completed comment intent.

## Decision

`koment lsp` owns repository discovery, resolution, comment policy and every
mutation. It uses standard LSP methods for portable behavior and one read-only
custom request for the resolved annotation view needed by rich decorations.

Editor packages are clients and presentation adapters. They may observe edits,
prompt people and render virtual text, but they do not parse annotation YAML or
write source and annotation files independently. A conversion or acknowledgement
is complete only when the language server's application service reports success.

The reference VS Code client implements the small protocol subset it consumes
with the editor's built-in Node runtime. It does not add a general language
client framework to the product dependency graph.

## Consequences

All editors receive identical status and mutation behavior, multi-root
workspaces discover repositories independently and source prose is never
removed before the record exists. The Go binary remains required beside the
extension, which the extension must locate and version-check. Rich inline
decorations are editor-specific even though hover and diagnostics are portable.

The small VS Code transport must be tested against koment's protocol framing.
If the extension later consumes enough of LSP that maintaining the transport is
riskier than a framework dependency, that dependency requires a superseding ADR.

## Alternatives rejected

**Implement annotation logic in every extension.** This would make validation,
anchoring and durable write ordering diverge between the CLI, agents and humans.

**Expose only shell commands to editors.** Process-per-hover has poor latency,
cannot maintain unsaved document state and does not provide portable diagnostics
or cancellation semantics.

**Add a general VS Code language-client dependency immediately.** The reference
client needs a bounded subset of request, notification and framing behavior.
The extra dependency tree and extension build toolchain are not justified by
that current surface.
