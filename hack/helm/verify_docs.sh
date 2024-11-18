#!/usr/bin/env sh

# Verify that README.md documentation was generated for all Helm charts.

set -e

. hack/helm/lib.sh

# Input variables.
HELM_DOCS_VERSION="${HELM_DOCS_VERSION:-latest}"
DOCKER_HUB_PROXY="${DOCKER_HUB_PROXY:-docker.io}"
HELM_DOCS_REPO="${HELM_DOCS_REPO:-jnorwood/helm-docs}"

# Temporary volume for holding generated docs, since we will not be overwriting the original README.md files.
temp_volume="$(docker volume create)"
trap 'docker volume rm -f "$temp_volume" > /dev/null' EXIT

# Copy the entire project to the volume.
docker run --rm -v "$(pwd):/project:ro" -v "$temp_volume:/generated" "$DOCKER_HUB_PROXY/alpine" \
  sh -c 'cp -R /project/* /generated'

echo "Generating docs"

# Generate docs on the copied project.
docker run --rm -v "$temp_volume:/helm-docs" "$DOCKER_HUB_PROXY/${HELM_DOCS_REPO}:$HELM_DOCS_VERSION" --document-dependency-values

echo "Comparing generated docs to actual docs"

# Check that the generated docs and the actual docs match.
docker run --rm -v "$(pwd):/project:ro" -v "$temp_volume:/generated" "$DOCKER_HUB_PROXY/alpine" \
  /project/helm/compare_docs.sh /generated /project || {
  echo "❗❗ Docs need to be regenerated. Run 'make generate-docs'. ❗❗"
  exit 1
}
