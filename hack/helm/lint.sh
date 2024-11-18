#!/usr/bin/env sh

# Lint all Helm charts.

set -e

. lib.sh

# Input variables.
CHART_TESTING_VERSION="${CHART_TESTING_VERSION:-}"

# Run chart-testing (wraps `helm lint`).
# See: https://github.com/helm/chart-testing
docker run --rm -v "$(pwd):/charts" "quay.io/helmpack/chart-testing:$(vtag "$CHART_TESTING_VERSION")" \
  ct lint --all --validate-maintainers=false
