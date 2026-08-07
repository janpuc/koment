# Cutting a release

**This procedure is mandatory. Follow it in order. Do not improvise a release.**

A release publishes signed artifacts to GitHub, GHCR, the MCP Registry, the VS
Code Marketplace and Open VSX. Marketplace versions are permanent: a version
number cannot be reused, replaced or withdrawn. A mistake here is not
recoverable by editing a file, so every step below exists because skipping it
produces something a user installs and cannot undo.

Agents: you may run steps 1–3 and 6. **Steps 4 and 5 change the outside world
and require explicit human approval in the conversation before you act.**

---

## 0. Preconditions

Verify all of these before starting. If any fails, stop and report it.

```sh
mise run fmt-check && mise run vet && mise run tidy-check && mise run generate-check
mise run lint && mise run test
mise run annotations && mise run comments && mise run agent-policy
mise run workflow-lint && mise run helm-lint && mise run helm-template && mise run vulncheck
mise run extension-test
```

All fourteen must pass. `koment check` failing means a release would ship
annotations that no longer describe the code.

`main` is protected by a ruleset — pull request required, signed commits,
linear history, and one required status check: **`ci`**. That is the
aggregating job in `.github/workflows/ci.yml`; it depends on every other job,
so requiring it requires all of them, and adding a job to CI does not mean
editing the ruleset. `cla`, `codeql` and `scorecard` are not required — they
report, and `cla` cannot be required at all because a release pull request is
opened by `GITHUB_TOKEN` and never gets a `cla` run. The classic
branch-protection API returns 404 for this repository; that means the rules
live in a ruleset, not that the branch is unprotected. Check with:

```sh
gh api repos/koment-dev/koment/rulesets
```

## 1. Land the work

Merge every change through a pull request with a conventional subject. The
subject decides the version, so it is a release decision, not a formatting one:

| Subject | Effect |
|---|---|
| `feat:` | minor bump — 1.0.0 → 1.1.0 |
| `fix:`, `perf:`, `refactor:` | patch bump |
| `docs:`, `test:`, `build:`, `ci:` | patch bump, listed in the changelog |
| `chore:` | no release on its own |
| any `!` or `BREAKING CHANGE:` | major bump — 1.4.2 → 2.0.0 |

`bump-minor-pre-major` was turned off when 1.0.0 shipped (ADR 0120), so a `!`
is a major version and not a quiet minor one. Before writing `!`, check whether
the change is breaking at all: a claim of backward compatibility needs a
migration the binary performs or an ADR naming the version the old shape was
cut off at. Without either, it is breaking, and the subject has to say so.

## 2. Let release-please open the release pull request

Pushing to `main` runs the `release` workflow, whose first job opens or updates
a pull request titled `chore(release): <version>`. It edits the changelog, the
manifest, and every file that carries the version:

- `.release-please-manifest.json`
- `charts/koment/Chart.yaml`
- `editors/vscode/package.json`
- `plugins/koment/.claude-plugin/plugin.json`
- `server.json` — both `.version` and `.packages[0].version`

Do not edit these by hand and do not bump a version in a feature branch.
`packaging` fails the build when they disagree.

## 3. Unblock that pull request's checks

**Expect its CI to sit at `action_required` with a 0s duration.** GitHub does
not run workflows for events created by `GITHUB_TOKEN`, so the required checks
never start and the pull request cannot merge on its own. This is normal and is
not a failure.

```sh
gh run list --branch release-please--branches--main --limit 5
gh api -X POST repos/koment-dev/koment/actions/runs/<run-id>/approve
```

Then wait for `test`, `lint`, `container build` and `helm chart` to go green.
Never merge a release pull request whose checks did not run — an unapproved run
is not a passing run.

## 4. Merge the release pull request — human approval required

Merging tags the release and starts publication. Everything after this point is
public and permanent.

Before merging, confirm:

- the version in the title is the one you intend;
- the changelog describes real changes;
- `ci` is green and not skipped, and every job it aggregates ran.

## 5. Watch publication — human approval required to retry anything

Merging runs the rest of the `release` workflow in this order. The order is a
decision, not an accident (ADR 0109): canonical artifacts first, downstream
channels second.

```
please ──┬─> binaries ──> editor
         └─> image ──┬──> mcp-registry
                     └──> chart
```

| Job | Publishes |
|---|---|
| `binaries` | six archives, `koment_<version>_checksums.txt`, a cosign signature, and rendered Homebrew/Scoop/WinGet metadata |
| `image` | `ghcr.io/koment-dev/koment:<version>`, multi-arch, SBOM and provenance, cosign-signed |
| `editor` | seven VSIX — six carrying that platform's released binary, one universal — signed, attached, then pushed to both marketplaces |
| `mcp-registry` | MCP Registry metadata via GitHub OIDC |
| `chart` | `oci://ghcr.io/koment-dev/charts/koment`, cosign-signed |

```sh
gh run watch "$(gh run list --workflow=release --limit 1 --json databaseId --jq '.[0].databaseId')"
```

If `binaries` fails, `editor` does not run. That is deliberate: the extension
bundles the released binary, so an extension built without one would ship
something that was never signed (ADR 0113).

## 6. Verify the release, do not assume it

```sh
tag=v<version>
gh release view "$tag" --json assets --jq '.assets[].name' | sort
curl -fsSLI -o /dev/null -w '%{http_code}\n' "https://open-vsx.org/api/koment/koment-dev"
curl -fsSLI -o /dev/null -w '%{http_code}\n' "https://marketplace.visualstudio.com/items?itemName=koment.koment-dev"
```

Expect 6 archives, 1 checksum manifest, 2 signature bundles for the manifest and
binaries, 7 VSIX and 7 VSIX signatures. A release missing the archives breaks
every workflow using `koment-dev/koment@v<version>`, because the setup action
downloads them.

## 7. Bump the development pin

`.mise/config.toml` pins `github:koment-dev/koment` to a released version, which is
the `koment` a contributor gets in their shell. It is not what any gate runs —
every `mise run` task uses `go run ./cmd/koment` — so it lags a release rather
than blocking one. It still has to be caught up, because a pinned binary older
than the record shape in `.koment/` cannot read this repository at all.

```sh
mise use "github:koment-dev/koment@<version>"
mise run annotations
```

Land it as `chore:`, which release-please does not turn into a release of its
own.

---

## When something goes wrong

**Never delete a tag or a published release to "redo" it.** Republish nothing.
Cut the next patch version instead. A tag that once existed has been fetched by
someone.

| Symptom | Cause | Action |
|---|---|---|
| release pull request checks show `action_required`, 0s | `GITHUB_TOKEN` created the pull request | approve the run (step 3) |
| `editor` job skipped | `binaries` failed | fix the binaries, cut a new patch version |
| `ovsx publish` fails on the first ever publish | the namespace did not exist | the workflow now creates it; if it still fails, the token lacks the Publisher Agreement |
| `vsce publish` rejects the version | that version already exists on the marketplace | cut the next version, never reuse one |
| `Windows Archive (advisory)` is red | advisory by decision (ADR 0111) | it does not block; read it and file a task |
| version files disagree | someone hand-edited one | revert the edit, let release-please own them |

## What an agent must never do

- Publish a VSIX, image, chart or binary by hand, outside this workflow.
  ADR 0112 rejected out-of-band publishing: a marketplace would carry a version
  the release never produced.
- Hand-edit any version-bearing file listed in step 2.
- Merge a release pull request whose checks did not run.
- Delete, move or re-point a tag, or force-push `main`.
- Re-run a publish job hoping it works the second time, without first
  establishing why it failed.
- Claim a release succeeded without running step 6 and quoting its output.
