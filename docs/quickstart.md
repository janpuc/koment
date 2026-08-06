# Getting started

## Install

Download the checksum-listed archive for your platform from the
[latest release](https://github.com/koment-dev/koment/releases/latest), or install
that release with mise:

```sh
mise use -g github:koment-dev/koment
```

One static binary, no runtime. Check it:

```sh
koment help
```

## Your first annotation

Work in a real repository — koment is not much use on a toy. Find a piece of
code where you know something the code doesn't say. The test is simple: *would a
competent stranger delete this, thinking it was pointless?*

First install the repository policy and the adapters used by supported agents:

```sh
koment agents install
```

```sh
koment add internal/auth/token.go \
    --excerpt 'if token.Expiry.Before(now.Add(-clockSkew)) {' \
    --kind gotcha \
    --body 'The skew subtraction is deliberate. Without it, clients whose clock
            runs a few seconds fast are logged out mid-request. Bit us in #412.'
```

koment prints the annotation's id and where it anchored:

```
01KYW1ETE3CVB6S0ND70GGZVWM  gotcha internal/auth/token.go:88
```

Long prose is painful to quote in a shell, so `--body -` reads from stdin:

```sh
koment add internal/auth/token.go --excerpt '...' --kind gotcha --body - <<'EOF'
The skew subtraction is deliberate. Without it, clients whose clock runs a
few seconds fast are logged out mid-request. Bit us in #412.
EOF
```

## The excerpt has to be unique

koment refuses an excerpt it can't pin down:

```
koment: excerpt matches 3 places in internal/auth/token.go (lines 44, 88, 131);
extend it until it is unique
```

Include the surrounding line, or a distinctive argument. This is deliberate: an
ambiguous anchor is rejected while you are there to fix it, rather than guessed
at later.

## Read it back

```sh
koment show internal/auth/token.go
```

```
internal/auth/token.go
  ok        gotcha        internal/auth/token.go:88  01KYW1ETE3CVB6S0ND70GGZVWM
    The skew subtraction is deliberate. Without it, clients whose clock runs
    a few seconds fast are logged out mid-request. Bit us in #412.
```

## Break it on purpose

Edit that line — rename the variable, change the condition. Then:

```sh
koment check
```

```
internal/auth/token.go
  drifted   gotcha        internal/auth/token.go  01KYW1ETE3CVB6S0ND70GGZVWM
    The skew subtraction is deliberate. ...
1 annotations across 1 files: 1 drifted
koment: 1 annotations no longer resolve; revisit them or update the anchor
```

Exit code 1. **This is the feature.** The reasoning you recorded no longer
describes the code, and koment refuses to keep serving it as though it did.

## Fix it

Read the annotation. Is it still true?

**Still true, code just moved or changed shape** — re-anchor it. The id comes
straight from the `check` output, and the hash and line are recomputed for you:

```sh
koment reanchor 01KYW1ETE3CVB6S0ND70GGZVWM --excerpt 'if token.Expired(now) {'
```

**File was renamed** — move it:

```sh
koment reanchor 01KYW1ETE3CVB6S0ND70GGZVWM --file internal/auth/session.go
```

The annotation keeps its id and its creation date. It is the same annotation;
only where it points changed.

**No longer true** — delete it from `.koment/annotations/<path>.yaml`. An
annotation that stopped being true has done its job and should go.

What you should *not* do is delete it to make the check pass without reading it.
That is the failure koment exists to prevent, just with extra steps.

## Move an existing comment into koment

Ordinary explanatory Go comments fail `koment comments check`. Convert one by
passing the complete comment group exactly as it appears in source:

```sh
koment comments convert internal/auth/token.go \
  --excerpt '// Keep the skew because clients have imperfect clocks.' \
  --kind gotcha
```

koment records the annotation first and removes the source comment second. A
comment can remain only through `koment comments acknowledge` with a rationale
and the explicit `--acknowledge-inline-comment` flag.

## Commit it

```sh
git add .koment internal/auth/token.go
git commit -m "fix: tolerate client clock skew in token expiry"
```

The annotation and the code it describes land in the same commit and the same
pull request, which is the point of storing them side by side.

## Next

- [Writing good annotations](annotating.md) — the hard part is judgement, not syntax
- [Agent setup](agents/) — give the reasoning to whatever edits your code
- [CI](ci.md) — make drift fail the build
