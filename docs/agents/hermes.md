# Hermes Agent

[Hermes Agent](https://hermes-agent.nousresearch.com) is Nous Research's
self-hosted agent. It reads MCP servers from `mcp_servers` in its `config.yaml`,
typically at `~/.hermes/hermes-agent/config.yaml`.

## Configure — local

```yaml
mcp_servers:
  koment:
    command: "koment"
    args: ["mcp"]
```

Hermes launches the server as a subprocess, so the binary must be on the `PATH`
of the Hermes process, and Hermes must be running with your repository as its
working directory.

## Configure — remote

If Hermes runs somewhere other than the machine holding the code — a container,
a cluster — the subprocess approach cannot work: the server has to run next to
the repository. Serve koment over HTTP instead, from inside the checkout:

```sh
cd /path/to/your/repo
koment mcp --http 8765
```

```yaml
mcp_servers:
  koment:
    url: "http://koment.internal:8765"
```

**koment's v0.2 HTTP transport has no authentication.** Anything that can reach
the port can read every annotation in the repository. It binds loopback unless
you say otherwise, and warns at startup when you do. If Hermes is on another
host, put the port behind something that authenticates or restrict it at the
network level. The approved served tier replaces this behaviour
([ADR 0105](../decisions/0105-authenticated-outbox-settles-through-git.md)).

## Filter the tools

koment only exposes two, so this is rarely needed — but if you are trimming a
large tool surface:

```yaml
mcp_servers:
  koment:
    command: "koment"
    args: ["mcp"]
    tools:
      include: ["koment_get", "koment_search"]
```

## Make it read them

Hermes reads `AGENTS.md`. Add:

```markdown
Before editing any file, call `koment_get` on it and read the annotations.
Treat a `drifted` or `orphaned` annotation as history, not as current fact.
```

## Notes

- Hermes keeps its own long-term memory. That is a *different thing* from koment
  and the two do not overlap: a memory store consolidates, paraphrases and
  eventually forgets, which is the right behaviour for preferences and wrong for
  a verbatim record anchored to a line of code. koment deliberately does not use
  one as a backend. Git remains the exact record
  ([ADR 0100](../decisions/0100-one-git-record-per-annotation.md)).
