#!/usr/bin/env sh

# Verify that the README.md file contents match for all charts, between two directories.
# Used to compare generated README's against the actual README's.

set -e

# Args.
expected="${1?expected is required}"
actual="${2?actual is required}"

# Temporary file for saving md5 checksums of the expected docs.
sum=$(mktemp)
trap 'rm -f $sum' EXIT

# Generate expected checksums.
(cd "$expected" && find . -type f -name README.md -exec md5sum {} + >"$sum")

# Check actual checksums against expected.
(cd "$actual" && md5sum -c "$sum")
