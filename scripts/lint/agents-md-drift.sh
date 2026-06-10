#!/bin/bash
# agents-md-drift.sh — verify every CMake target referenced in AGENTS.md
# actually exists in CMakeLists.txt.

set -euo pipefail

AGENTS_MD="${1:-AGENTS.md}"
CMAKE_LISTS="${2:-CMakeLists.txt}"

exit_code=0

# Extract every `--target <name>` from AGENTS.md code spans.
# We look for the pattern inside backtick-delimited inline code or fenced blocks.
targets=$(grep -o '\--target [a-zA-Z0-9_-]*' "$AGENTS_MD" | sed 's/--target //' | sort -u)

if [ -z "$targets" ]; then
  echo "AGENTS-MD-DRIFT: no --target references found in $AGENTS_MD"
  exit 1
fi

for target in $targets; do
  # Each target must appear as an add_custom_target(<name> in CMakeLists.txt
  # OR as a target in add_dependencies.
  if grep -qE "add_custom_target\($target" "$CMAKE_LISTS"; then
    echo "AGENTS-MD-DRIFT: OK  $target → $CMAKE_LISTS"
  else
    echo "AGENTS-MD-DRIFT: MISSING  $target referenced in $AGENTS_MD but not defined in $CMAKE_LISTS"
    exit_code=1
  fi
done

# Also check that no `make <target>` references remain (they all must use CMake).
if grep -qnE 'make (test-unit|lint)' "$AGENTS_MD"; then
  echo "AGENTS-MD-DRIFT: stale make reference in $AGENTS_MD"
  exit_code=1
fi

exit $exit_code
