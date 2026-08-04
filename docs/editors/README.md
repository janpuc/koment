# Editors

Annotation semantics live in one process. `koment lsp` speaks the Language
Server Protocol over stdio and is backed by the same snapshot and mutation
services as the CLI, the UI and MCP, so no editor reimplements record
validation, anchoring or comment classification (ADR 0110).

That has a practical consequence: **most editors need no koment package.** They
need three lines of configuration.

## What the server provides

Reported by `koment lsp` at initialize:

| Capability | What you get |
|---|---|
| `hoverProvider` | the annotation body under the cursor |
| `codeLensProvider` | annotations above the line they anchor to |
| `codeActionProvider` | convert a comment, acknowledge it, reanchor a stale record |
| `executeCommandProvider` | `koment.add`, `koment.reanchor`, `koment.convertComment`, `koment.acknowledgeComment` |
| `publishDiagnostics` | drift, orphaning, ambiguity and prohibited comments |

Text is synchronised in full on change, and saves include the text.

Rendering an annotation body *beside* its line, without putting it in the
buffer, is not something LSP can express. That is why one packaged extension
exists (ADR 0112).

## Packaged

**VS Code**, and through Open VSX also Cursor, Windsurf and VSCodium. It starts
`koment lsp` and gives rationale two surfaces (ADR 0114): an inline gloss after
the annotated line saying that rationale exists, and a **koment panel** in the
activity bar holding every annotation in the file with its complete body. Choose
an entry to jump to its line. `koment: Toggle full annotation text inline`
removes the inline truncation when you want the whole thing in place.

It also turns a freshly typed explanatory comment into an annotation. See
[the extension README](../../editors/vscode/README.md).

Diagnostics carry only what fails `koment check` — `ambiguous`, `drifted`,
`orphaned` and prohibited comments. A `moved` annotation resolves correctly, so
it is shown in the gloss and the panel and never marks the code.

The package for your platform **carries its own `koment` binary**, so installing
the extension installs koment (ADR 0113). Linux, macOS and Windows are covered
on x64 and arm64. Anywhere else you get the universal package, which needs a
binary on `PATH`. The output channel names which one it started and where it
came from.

Every release attaches cosign-signed packages for all seven. Marketplace
publication is enabled per repository; until it is, install from the release.

## Configured

Any LSP client can run the server directly. It needs the released `koment`
binary on `PATH` and a repository containing `.koment/`.

```sh
koment lsp
```

### Helix

`languages.toml`:

```toml
[language-server.koment]
command = "koment"
args = ["lsp"]

[[language]]
name = "go"
language-servers = ["gopls", "koment"]
```

Helix attaches language servers per language, so repeat `language-servers` for
each language you want annotations in.

### Neovim

Needs 0.11 or newer for `vim.lsp.config`:

```lua
vim.lsp.config['koment'] = {
  cmd = { 'koment', 'lsp' },
  filetypes = { 'go', 'lua', 'python', 'rust', 'typescript' },
  root_markers = { '.koment', '.git' },
}

vim.lsp.enable('koment')
```

Extend `filetypes` freely — anchoring is excerpt-based and language-agnostic, so
the server has no opinion about the language it is asked about.

### Anything else

An editor that can start a stdio language server can use koment. Point it at
`koment lsp`, root the workspace on `.koment`, and attach it to whichever
filetypes you want. Hover, code lenses, code actions and diagnostics work
without any koment-specific client code; inline decoration does not, and no
configuration will add it.

## Which one you get

| | hover, lenses, actions, diagnostics | inline annotation bodies | install |
|---|---|---|---|
| VS Code and its forks | yes | yes | extension |
| Helix, Neovim, Emacs, Sublime, Kate | yes | no | configuration |

For agent-facing MCP setup rather than human editing, see
[docs/agents](../agents/README.md).
