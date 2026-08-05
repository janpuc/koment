# Any other MCP client

koment implements MCP with no client-specific behaviour, so anything that speaks
the protocol works. This page is the generic recipe.

## stdio — the default

Most clients spawn the server as a subprocess:

| field | value |
|---|---|
| command | `koment` |
| args | `["mcp", "--write"]` |
| transport | stdio |
| working directory | your repository |

Whatever the config format, that is what it needs to express. Two examples of
the same thing:

```json
{ "mcpServers": { "koment": { "command": "koment", "args": ["mcp", "--write"] } } }
```

```yaml
mcp_servers:
  koment:
    command: "koment"
    args: ["mcp", "--write"]
```

**Working directory matters.** koment walks up from it looking for `.koment/`,
then `.git/`. A client that launches servers from your home directory will serve
nothing useful. If yours does, pin the repository with whatever `cwd` option it
offers.

## HTTP — when the agent isn't on the same machine

For containers, remote runners and hosted clients, run the server yourself from
inside the checkout:

```sh
cd /path/to/your/repo
koment mcp --http 8765              # JSON responses
koment mcp --streamable-http 8765   # server-sent events
```

Both are the MCP Streamable HTTP transport; they differ only in whether
responses come back as `application/json` or `text/event-stream`. Pick whichever
your client speaks — try `--http` first, since a client that wants SSE will
usually say so.

A bare port binds loopback. To accept connections from elsewhere you must say so
explicitly, and koment warns when you do:

```sh
koment mcp --http 0.0.0.0:8765
koment: WARNING serving on 0.0.0.0:8765, which is not loopback. There is no
authentication; anyone who can reach this port can read every annotation in
the repository.
```

**There is no authentication on the current HTTP transport.** Put it behind
something that authenticates before exposing it beyond a trusted network. This is transitional
behaviour; the approved served tier authenticates every non-loopback request
([ADR 0105](../decisions/0105-authenticated-writes-materialize-through-git.md)).

## The tools

```
koment_get(file: string)
  → { file, annotations: [ { id, kind, body, scope, excerpt, line,
                             created, status, warning } ] }

koment_search(query: string)
  → { query, matches: [ ...same shape, plus file ] }

koment_repositories()
  → { repositories: [ { id, name, default_branch, clone_url, files,
                         annotations: { status: count } } ] }

koment_add(file, excerpt?, kind, body, repository?)
koment_reanchor(id, file?, excerpt?, repository?)
koment_convert_comment(file, comment, kind?, repository?)
koment_acknowledge_comment(file, comment, body,
  acknowledge_inline_comment: true, repository?)
```

`status` is one of `ok`, `ambiguous`, `drifted`, `orphaned`. When it is
a failing one, `warning` carries prose saying so — a client should surface it
rather than present the body as current fact.

The four mutation tools are registered only by local `--write` stdio servers.
HTTP transports are read-only.

## Check it works without a client

```sh
cd /path/to/your/repo
koment mcp --http 8765 &

curl -sS -X POST http://127.0.0.1:8765 \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":
       {"protocolVersion":"2025-06-18","capabilities":{},
        "clientInfo":{"name":"curl","version":"0"}}}'
```

A JSON-RPC result naming `koment` means the server is fine and any remaining
problem is in the client's config. Take the `Mcp-Session-Id` header from that
response and pass it back on subsequent calls.

## No MCP support at all?

The CLI covers it. `koment show <file>` prints the same annotations, and a
pre-tool hook or a shell wrapper can feed that to an agent as plain text. Less
elegant, works everywhere.
