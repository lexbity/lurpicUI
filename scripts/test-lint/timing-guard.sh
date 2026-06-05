#!/bin/bash
# Timing-threshold guard: flags sub-100ms wall-clock comparisons and
# time.Sleep in tests that should use channel-gated synchronization.
# Usage: ./scripts/test-lint/timing-guard.sh [files...]

set -euo pipefail

exit_code=0

# Pattern 1: time.Since( compared against < 100ms
while IFS=: read -r file line col rest; do
    if [ -z "$file" ]; then continue; fi
    # Extract the compared duration value  
    if echo "$rest" | grep -Pq '<\s*\d+\s*\*\s*time\.(Millisecond|Microsecond|Nanosecond)\b'; then
        echo "$file:$line:$col: timing assertion compares against <100ms threshold — use channel-gating instead"
        exit_code=1
    fi
done < <(rg --no-heading -n --type go 'time\.Since\(.*\)\s*[<>]\s*\d+\s*\*\s*time\.(Millisecond|Microsecond|Nanosecond)' 2>/dev/null || true)

# Pattern 2: time.Sleep in test files (often a sign of flaky timing)
while IFS=: read -r file line col rest; do
    if [ -z "$file" ]; then continue; fi
    echo "$file:$line:$col: time.Sleep used — prefer channel-gated synchronization"
    exit_code=1
done < <(rg --no-heading -n --type go 'time\.Sleep\(' 2>/dev/null || true)

exit $exit_code
