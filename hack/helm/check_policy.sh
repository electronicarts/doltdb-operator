#!/usr/bin/env sh

# Validate all Helm charts by applying custom OPA policies to the templated Kubernetes YAML.
# Validation only runs if a chart contains a policy subdirectory.
# The charts will be templated using the default values.yaml, as well as any values-*.yaml files inside the chart.

set -e

. hack/lib.sh

# Input variables.
HELM_VERSION="${HELM_VERSION:-latest}"
CONFTEST_VERSION="${CONFTEST_VERSION:-}"
DOCKER_HUB_PROXY="${DOCKER_HUB_PROXY:-docker.io}"
CONFTEST_ARGS="${CONFTEST_ARGS:-}"

# A volume is useful for passing files between Docker containers.
shared_volume="$(docker volume create)"
trap 'docker volume rm -f "$shared_volume" > /dev/null' EXIT

check_policy_with_values() {
  dir=$1
  values_path=$2

  values_file="$(basename "$values_path")"

  echo "Templating $dir using $values_file"

  # See if we have a top level required yaml and if so, then inject it to the 'helm template' command below.
  if [ -f "$dir"/placeholder-required-values.yaml ]; then
    placeholderRequiredValuesFile="--values /project/$dir/placeholder-required-values.yaml"
  fi

  # Run `helm template`.
  docker run --rm -v "$shared_volume:/shared" -v "$(pwd):/project:ro" --entrypoint=/bin/sh \
    "$DOCKER_HUB_PROXY/alpine/helm:$HELM_VERSION" \
    -c "mkdir -p /shared/$(dirname "$values_path") && \
      helm template --values /project/$values_path $(basename "$dir") $placeholderRequiredValuesFile /project/$dir > /shared/$values_path" || {
    echo "❗❗ Failure while templating $dir using $values_file ❗❗"
    return 1
  }

  echo "Validating output yaml from $dir using $values_file"

  # Run `conftest test` on the templated raw yaml.
  # shellcheck disable=SC2086
  docker run --rm -v "$shared_volume:/shared" -v "$(pwd):/project:ro" -w "/project/$dir" \
    "$DOCKER_HUB_PROXY/openpolicyagent/conftest:$(vtag "$CONFTEST_VERSION")" \
    test $CONFTEST_ARGS "/shared/$values_path" || {
    echo "❗❗ Failure while checking $dir output yaml using $values_file against policies. Ensure that the chart complies with the policies in its /policies subdirectory. ❗❗"
    return 1
  }
}

check_policy() {
  dir=$1

  if [ ! -d "$dir"/policy ]; then
    echo "Skipping $dir"
    return 0
  fi

  # Always run the policy check using the default values.yaml.
  check_policy_with_values "$dir" "$dir/values.yaml"

  # If other *-values.yaml files exist in the chart's ci directory,
  # (this directory and format are also used by https://github.com/helm/chart-testing),
  # then run the policy check using them as well.
  for file in "$dir"/ci/*-values.yaml; do
    if [ -f "$file" ]; then
      check_policy_with_values "$dir" "$file"
    fi
  done
}

# Find all Chart.yaml files recursively, then do policy check on each chart folder.
find . -name "Chart.yaml" -maxdepth 10 -mindepth 1 -type f | while read -r file; do
  dir=$(dirname "$file")
  check_policy "$dir"
done
