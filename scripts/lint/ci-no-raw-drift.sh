#!/bin/bash
# ci-no-raw-drift.sh — verify the CI workflow contains no raw
# `golangci-lint run` or `go run ./cmd/lurpiclint` lines.
# All analysis gates must route through CMake targets.

set -euo pipefail

_ci_yml="${1:-.github/workflows/ci.yml}"

# Read into memory so multiple grep invocations work with stdin.
CI_YML_CONTENT=$(cat "$_ci_yml")
exit_code=0

# Pattern 1: raw golangci-lint invocation (not the install-only action step)
if echo "$CI_YML_CONTENT" | grep -qE 'golangci-lint\s+run\b'; then
  echo "NFR-1 DRIFT: FAIL — raw 'golangci-lint run' found in $_ci_yml"
  exit_code=1
fi

# Pattern 2: raw go run of lurpiclint
if echo "$CI_YML_CONTENT" | grep -qE 'go\s+run\s+.*lurpiclint'; then
  echo "NFR-1 DRIFT: FAIL — raw 'go run ./cmd/lurpiclint' found in $_ci_yml"
  exit_code=1
fi

# Pattern 3: raw go build (must route through CMake)
if echo "$CI_YML_CONTENT" | grep -qE 'go\s+build\s+'; then
  echo "NFR-1 DRIFT: FAIL — raw 'go build' found in $_ci_yml"
  exit_code=1
fi

# Pattern 4: raw go vet (must route through CMake)
if echo "$CI_YML_CONTENT" | grep -qE 'go\s+vet\b'; then
  echo "NFR-1 DRIFT: FAIL — raw 'go vet' found in $_ci_yml"
  exit_code=1
fi

if [ "$exit_code" -eq 0 ]; then
  echo "NFR-1 DRIFT: OK — all analysis gates route through CMake targets"
fi

exit $exit_code
