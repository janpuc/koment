#!/usr/bin/env bash
set -euo pipefail

# The shared home-operations preset attaches helm-docs as a post-upgrade task
# and Renovate's container does not carry it, while `mise run generate-check`
# fails on a chart README that was not regenerated. The version is pinned and
# Renovate updates it here like any other dependency.

# renovate: datasource=github-releases depName=norwoodj/helm-docs
HELM_DOCS_VERSION=1.14.2
curl -fsSL \
    "https://github.com/norwoodj/helm-docs/releases/download/v${HELM_DOCS_VERSION#v}/helm-docs_${HELM_DOCS_VERSION#v}_Linux_x86_64.tar.gz" \
    | tar -xz -C /usr/local/bin helm-docs
helm-docs --version

runuser -u ubuntu renovate
