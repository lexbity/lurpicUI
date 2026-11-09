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
  # Strip comment lines (YAML `# ...` and shell `# ...` inside run blocks) so
  # prose like "cgo build" or "go test -race requires cgo" in comments does not
  # trip the patterns; only executable lines are checked.
  content=$(grep -v '^[[:space:]]*#' "$f")
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
  # Pattern 6: ${{ github.event.* }} interpolated INSIDE a run: block — that is
  # string interpolation into a shell command (script-injection risk).
  # Expression-only fields (if:, cancel-in-progress:, group:, key:) are
  # evaluated by GitHub's expression engine, not the shell, and are allowed.
  if awk '
    /^[ ]*run:[ ]*$/          { inrun=1; ind=match($0,/[^ ]/)-1; next }
    /^[ ]*run:[ ]*(\||>)/     { inrun=1; ind=match($0,/[^ ]/)-1; next }
    /^[ ]*run:[ ]*[^ ]/ {
      rest=$0; sub(/^[ ]*run:[ ]*/, "", rest)
      if (rest ~ /\$\{\{[^}]*github\.event/) { print; bad=1 }
      next
    }
    inrun {
      if ($0 ~ /^[ ]*$/) next
      c=match($0,/[^ ]/); ci = c>0 ? c-1 : 0
      if (ci <= ind) { inrun=0 }
      else if ($0 ~ /\$\{\{[^}]*github\.event/) { print; bad=1 }
    }
    END { exit bad ? 1 : 0 }
  ' "$f"; then :; else
    echo "NFR-1 DRIFT: FAIL — \${{ github.event.* }} interpolated inside a run: block in $f"
    exit_code=1
  fi
done

if [ "$exit_code" -eq 0 ]; then
  echo "NFR-1 DRIFT: OK — all analysis gates route through CMake targets (${#files[@]} file(s) checked)"
fi

exit "$exit_code"
