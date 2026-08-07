#!/usr/bin/env bash
# scripts/commitlint.sh
#
# Single source of truth for the Conventional Commits 1.0.0 gate.
# Called by:
#   - .github/workflows/commit-lint.yml  (CI)
#   - .lefthook.toml  [commit-msg]        (local hook before push)
#   - mise run commitlint                  (manual)
#
# Reads subjects from stdin (one per line) and exits non-zero on any
# non-conforming subject. See ADR 0128.

set -euo pipefail

# Spec-listed types: feat, fix, docs, style, refactor, test, chore,
# perf, ci, build, revert. release-please ignores style and revert;
# that is a changelog-policy decision, not a gate decision.
# A single ! may appear after the type OR after the scope.
# Scope, when present, is lowercase alphanumerics, dot, dash, space.
# Description is non-empty. No requirement on case or trailing punctuation.
re='^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(!)?(\([a-z0-9 .\-]+\))?!?: .+'

failed=0
while IFS= read -r subject; do
  [ -z "$subject" ] && continue
  case "$subject" in
    'Merge '*) continue ;;
  esac
  if ! printf '%s\n' "$subject" | grep -qE "$re"; then
    echo "::error::non-conforming commit subject: $subject" >&2
    failed=1
  fi
done

if [ "$failed" -ne 0 ]; then
  cat >&2 <<'MSG'
Commit subjects MUST follow Conventional Commits 1.0.0:
  <type>(<scope>)?!: <description>
Types: feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert
Use ! after the type (or after the scope) for breaking changes.
See https://www.conventionalcommits.org/ and docs/decisions/0128.
MSG
  exit 1
fi
