#!/bin/bash
# ci-no-suppression.sh — verify CI workflow files contain no failure
# suppression and no floating action tags:
#   - '|| true'            swallows command failures
#   - 'continue-on-error'  marks steps non-gating
#   - uses: <repo>@vN      floating tag (mutable); must be a 40-hex commit SHA
#
# Usage: ci-no-suppression.sh <file-or-dir>...   (default: .github/workflows/)
#   File args are checked directly; a directory arg expands to the
#   git-TRACKED files under it (same convention as ci-no-raw-drift.sh).
#
# The patterns live in THIS script, not in the workflow files, so the gates
# never match their own source.

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
    echo "SUPPRESSION DRIFT: FAIL — no such file or directory: $arg" >&2
    exit 1
  fi
done

if [ ${#files[@]} -eq 0 ]; then
  echo "SUPPRESSION DRIFT: OK — no tracked workflow files to check (nothing under $1 is git-tracked)"
  exit 0
fi

exit_code=0
for f in "${files[@]}"; do
  if grep -n '|| true' "$f"; then
    echo "SUPPRESSION DRIFT: FAIL — '|| true' found in $f"
    exit_code=1
  fi
  if grep -n 'continue-on-error' "$f"; then
    echo "SUPPRESSION DRIFT: FAIL — 'continue-on-error' found in $f"
    exit_code=1
  fi
  if grep -nE 'uses: [^ ]+@v[0-9]' "$f"; then
    echo "SUPPRESSION DRIFT: FAIL — floating action tag in $f (pin to a 40-hex commit SHA)"
    exit_code=1
  fi
done

if [ "$exit_code" -eq 0 ]; then
  echo "SUPPRESSION DRIFT: OK — no suppressed failures, all actions SHA-pinned (${#files[@]} file(s))"
fi

exit "$exit_code"
