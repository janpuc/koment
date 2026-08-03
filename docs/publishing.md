# Publishing your annotations

Copy one file into your repository and everyone with access reads the reasoning
in a browser. No server, no database, no authentication to design, no cost.

This is the **published tier**
([ADR 0103](decisions/0103-three-tiers-with-human-and-agent-capabilities.md)).
It sits between running the CLI on a laptop and running a served instance, and
moving between them is not a migration — all three read the same `.koment/` in
git.

> **A site renders your source, not only your annotations.** Publishing one from
> a private repository to public Pages publishes that source. Check your Pages
> visibility before the first run; a private repository on GitHub Team or
> Enterprise can restrict Pages to people with repository access.

## The workflow

Save as `.github/workflows/koment.yml`. Then enable Pages: **Settings → Pages →
Build and deployment → Source: GitHub Actions**.

```yaml
name: koment

on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: koment-pages
  cancel-in-progress: false

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: janpuc/koment@v0.2.0
      - run: koment check

  publish:
    needs: check
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pages: write
      id-token: write
    environment:
      name: github-pages
      url: ${{ steps.deploy.outputs.page_url }}
    steps:
      - uses: actions/checkout@v5
      - uses: janpuc/koment@v0.2.0

      - run: |
          koment site --out dist \
            --commit-link "${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/commit/${GITHUB_SHA}"

      - uses: actions/configure-pages@v5
      - uses: actions/upload-pages-artifact@v4
        with:
          path: dist
      - id: deploy
        uses: actions/deploy-pages@v4
```

That is the whole thing. Push it, and the site appears at
`https://<you>.github.io/<repo>/`.

Pin the `uses:` lines to commit SHAs if your organisation requires it — this
repository does, and [its own workflows](../.github/workflows/) show the form.

## What each part is doing

**`koment check` gates the publish.** An annotation whose code has changed exits
non-zero, so a build that would publish drift fails instead. Running it on pull
requests too is the point of koment; publishing is the smaller half.

**`koment site` renders one repository snapshot.** It writes an `index.html`, a
page per annotated file and static assets, all linked relatively so the result
works under a project subpath, from a `file://` path, and behind any prefix. A
grouped publication renders each snapshot separately and connects them through
the same contextual repository switcher.

**Every page names the commit it was rendered from.** A snapshot that does not
say what it is a snapshot of is how a stale rendering passes for a current one.
`koment site` reads the commit from git; `--commit-link` makes it clickable, and
`--commit` sets it explicitly when git is not available.

**The `publish` job's `permissions` and `environment` are yours, not ours.** An
action cannot declare them on your behalf, and it should not — deploying to your
Pages site is a permission you grant explicitly and can audit in your own
repository.

## Options worth knowing

| | |
|---|---|
| `--out <dir>` | where to write. Required. |
| `--name <text>` | repository name shown on every page. Defaults to the repository's own. |
| `--commit <sha>` | the commit being rendered. Read from git when omitted. |
| `--commit-link <url>` | make the commit clickable. |
| `--banner <text>` | a notice on every page, beside the commit. |
| `--banner-link <url>` | a URL shown beside the banner. |
| `--repository <id>` | which repository to render, when several are configured. |
| `--repository-links <name=URL,...>` | repository switcher entries, relative to this snapshot root. |

Every flag is also an environment variable — `--commit-link` is
`KOMENT_COMMIT_LINK`.

## Publishing more than one repository

A static publication keeps one immutable snapshot per repository. Render the
default repository at the publication root and the others beneath it, passing
the links each snapshot needs to reach the same destinations:

```yaml
- run: |
    koment site --out dist --repository payments --name payments \
      --repository-links "payments=index.html,web=r/web/index.html"
    koment site --out dist/r/web --repository web --name web \
      --repository-links "payments=../../index.html,web=index.html"
```

The publication opens `payments` immediately. The repository switcher appears
inside the ordinary application rail, so discovery never becomes a selector
landing page. Each page still names the commit of the repository snapshot it
shows. Cross-repository search remains a served-tier capability.

## Moving to a served instance later

Nothing to migrate. The annotations were never in the site — they are in
`.koment/` in git, which is also what the served tier reads:

```bash
docker run --rm -p 8080:8080 -v "$PWD:/repo:ro" ghcr.io/janpuc/koment:latest
```

You gain live resolution against the working tree, several repositories behind
one switcher, cross-repository search and metrics. You take on hosting
and an access-control decision, because the v0.2 served UI has no
authentication. That is legacy behaviour: the approved served tier requires
authentication
([ADR 0105](decisions/0105-authenticated-outbox-settles-through-git.md)).
Keeping the published site as well is a reasonable thing to do; they do not
conflict.

## Not on GitHub

`koment site` writes plain files with relative links. Anything that serves a
directory will serve them — GitLab Pages, Cloudflare Pages, S3, nginx, or a
tarball someone opens locally. The action is a convenience for GitHub runners;
elsewhere, install the binary from the
[releases page](https://github.com/janpuc/koment/releases) and run the same
command.

## The action

`janpuc/koment@v0.2.0` downloads a released binary, verifies it against the
release's published checksums, and puts it on `PATH`. It supports Linux and
macOS runners; on Windows, use `go install`.

The tag on `uses:` picks the *action*; the `version` input picks the *koment
release* it installs, and defaults to the latest.

```yaml
- uses: janpuc/koment@v0.2.0
  with:
    version: 0.2.0   # pin the CLI too, so a new release cannot change your build
```

It deliberately does only that, so the same step serves a `check` gate, a
publish, or anything else you want to run.
