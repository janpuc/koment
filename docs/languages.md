# Language support

koment does two different things to a file, and they have different reach.

**Anchoring** binds an annotation to a snippet of text. It is a verbatim search
with surrounding context, so it has no idea what language it is looking at and
works in every file in the repository — YAML, Terraform, Dockerfiles, SQL,
Markdown, shell.

**Comment detection** finds the comments already in a file so koment can flag
them, convert them, or offer to. It has to know where a comment ends and a
string begins, so it needs a parser per language.

Everything below is about the second one. If a language is not listed as
detected, you can still annotate it — you just do the migration by hand.

| | Anchoring | Comment detection |
|---|---|---|
| Go | yes | yes |
| everything else | yes | no |

## What detection gives you

Only in a detected language:

- `koment comments check` — the gate that fails when a prohibited comment lands
- `koment comments convert` — move a comment into an annotation and delete it
- `koment comments acknowledge` — keep a comment, attributably
- the editor prompt that offers to convert a comment as you finish typing it

Outside one, those commands report `no comment detector for <file>` and
`koment comments check` walks nothing.

## Go

Comments come from `go/ast`, so a comment group is the unit: a run of adjacent
`//` lines is one comment, converts to one annotation, and is asked about once.

Detection is skipped for text the toolchain requires — `//go:` directives,
generated-file markers, `Deprecated:` markers, links to upstream issues, and
godoc on exported identifiers. Those are configured per repository under
`comments.intrinsic` in `.koment/policy.yaml`.

A comment is only offered for conversion when it reads as prose. Text that
parses as Go statements is treated as commented-out code and left alone, because
converting it would turn disabled code into rationale.

**Caveat.** Detection covers `*.go` only. A Go repository's YAML, Makefiles and
shell scripts are annotatable but not detected.

## Everything else

Annotate by hand, and anchor to the code rather than to the comment:

```sh
koment add kubernetes/apps/app.yaml \
  --excerpt '- image: ghcr.io/example/app:1.2.3' \
  --kind why --title 'Pinned below 1.3.0' --body -
```

Anchoring to the comment text would orphan the annotation the moment the comment
is deleted, which is usually the next thing you do.

`koment check` works normally afterwards: resolution is language-independent.

**Caveat.** With no detector, nothing stops a prohibited comment landing in
these files. The repository convention has to carry that, not the gate.

## Planned

YAML is next, and it is the only one where the parser is already a dependency:
`go.yaml.in/yaml/v3` reports comments with positions and correctly ignores a `#`
inside a quoted scalar, a URL fragment or a block scalar. Python and TOML each
need a parser koment does not have yet, so each needs its own decision recorded
before it is added.

JSON is not planned. The format has no comments. JSONC and JSON5 do, and are a
separate question from JSON.

The interface a new language implements is `Detector` in
`internal/commentpolicy`. [ADR 0114](decisions/0114-show-rationale-in-a-panel-and-stop-diagnosing-health.md)
records why a regular expression is not an acceptable implementation: a scanner
that cannot tell a comment from a string produces false prohibited-comment
failures, and those block CI. A wrong gate is worse than an absent one.
