#!/usr/bin/env sh

# Update all Chart.lock files and archives for local chart dependencies.

set -e

. helm/lib.sh

# Input variables.
HELM_VERSION="${HELM_VERSION:-latest}"
DOCKER_HUB_PROXY="${DOCKER_HUB_PROXY:-docker.io}"

update_dependencies() {
  dir=$1

  if [ ! -f "$dir"/Chart.yaml ]; then
    echo "Skipping $dir"
    return 0
  fi

  echo "Processing $dir"

  # Run `helm dependency update`.
  docker run --rm -v "$(pwd):/project" "$DOCKER_HUB_PROXY/alpine/helm:$HELM_VERSION" \
    dependency update --skip-refresh "/project/$dir" || {
    echo "❗❗ Failure while updating $dir dependencies ❗❗"
    return 1
  }

}

# Find all Chart.yaml files recursively, then do helm dependency update on each chart folder.
find . -name "Chart.yaml" -maxdepth 10 -mindepth 1 -type f | while read -r file; do
  dir=$(dirname "$file")
  update_dependencies "$dir"
done

