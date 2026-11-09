#!/bin/bash
# ci-no-raw-drift.sh — verify CI workflow files contain no raw analysis
# invocations (go run/build/test/vet, golangci-lint run) and no
# `${{ github.event.* }}` interpolation.  All analysis gates must route
# through CMake targets.
#
# Usage: ci-no-raw-drift.sh <file-or-dir>...   (default: .github/workflows/)
#   File args are checked directly.  A directory arg expands to the
#   git-TRACKED files under it, so deferred/untracked workflows (e.g.
#   android.yml, gitignored until the Android slice lands) are not policed
#   before they are tracked.

set -euo pipefail

if [ $# -eq 0 ]; then
  set -- .github/workflows/
fi

files=()
for arg in "$@"; do
  if [ -d "$arg" ]; then
    while IFS= read -r f; do
      files+=("$f")
    done < <(git ls-files -- "$arg")
  elif [ -f "$arg" ]; then
    files+=("$arg")
  else
    echo "NFR-1 DRIFT: FAIL — no such file or directory: $arg" >&2
    exit 1
  fi
done

if [ ${#files[@]} -eq 0 ]; then
  echo "NFR-1 DRIFT: OK — no tracked workflow files to check (nothing under $1 is git-tracked)"
  exit 0
fi

exit_code=0
for f in "${files[@]}"; do
  content=$(cat "$f")
  check() { # check <extended-regex> <description>
    if echo "$content" | grep -qE "$1"; then
      echo "NFR-1 DRIFT: FAIL — $2 found in $f"
      exit_code=1
    fi
  }
  check 'golangci-lint\s+run\b' "raw 'golangci-lint run'"
  check 'go\s+run\s'            "raw 'go run' (must route through CMake)"
  check 'go\s+build\s+'         "raw 'go build' (must route through CMake)"
  check 'go\s+vet\b'            "raw 'go vet' (must route through CMake)"
  check 'go\s+test\b'           "raw 'go test' (must route through CMake)"
  check '\$\{\{[^}]*github\.event' 'raw ${{ github.event.* }} interpolation (script-injection risk; bare github.event_name in if: is fine)'
done

if [ "$exit_code" -eq 0 ]; then
  echo "NFR-1 DRIFT: OK — all analysis gates route through CMake targets (${#files[@]} file(s) checked)"
fi

exit "$exit_code"
