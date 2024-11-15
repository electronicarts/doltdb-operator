#!/usr/bin/env sh

# Generate README.md documentation for all Helm charts.

set -e

. helm/lib.sh

# Input variables.
HELM_DOCS_VERSION="${HELM_DOCS_VERSION:-latest}"
DOCKER_HUB_PROXY="${DOCKER_HUB_PROXY:-docker.io}"
HELM_DOCS_REPO="${HELM_DOCS_REPO:-jnorwood/helm-docs}"

# Run helm-docs.
# See: https://github.com/norwoodj/helm-docs
docker run --rm -v "$(pwd):/helm-docs" "$DOCKER_HUB_PROXY/${HELM_DOCS_REPO}:$HELM_DOCS_VERSION" --document-dependency-values
