#!/bin/bash
# perf-diff.sh — Compare benchmark results against a committed baseline.
#
# Without flags: runs benchmarks, compares to baseline, exits non-zero on
# regression (>20% ns/op or >25% allocs/op) or metadata mismatch.
#
# Flags:
#   --rebaseline  Overwrite the baseline file with the current run's results.
#   --baseline    Path to baseline JSON (default: devdocs/plans/baselines/…)
#
# Environment:
#   LURPIC_PERF_TOLERANCE_NS     ns/op regression % (default 20)
#   LURPIC_PERF_TOLERANCE_ALLOC  allocs/op regression % (default 25)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BENCH_PKGS=(./marks/structure/... ./store/...)
BASELINE="${BASELINE:-devdocs/plans/baselines/marks-catalog-perf-baseline.json}"
REBASELINE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --rebaseline) REBASELINE=true; shift ;;
        --baseline) BASELINE="$2"; shift 2 ;;
        *) echo "unknown flag: $1"; exit 1 ;;
    esac
done

BASELINE_PATH="$REPO_ROOT/$BASELINE"

if [ ! -f "$BASELINE_PATH" ] && [ "$REBASELINE" = false ]; then
    echo "ERROR: baseline file not found: $BASELINE_PATH"
    echo "  Run with --rebaseline to create it."
    exit 1
fi

# ---- Metadata helpers ----
goos=$(go env GOOS)
goarch=$(go env GOARCH)
go_version=$(go version | awk '{print $3}')
gomaxprocs=$(nproc 2>/dev/null || echo unknown)
cpu=$( (cat /proc/cpuinfo 2>/dev/null | grep 'model name' | head -1 | sed 's/.*: //') || echo unknown)
captured=$(date +%Y-%m-%d)

# ---- Run benchmarks ----
BENCH_OUT=$(cd "$REPO_ROOT" && go test -run='^$' -bench='.' -benchmem -count=1 -timeout=300s "${BENCH_PKGS[@]}" 2>&1)

# Parse benchmark results
declare -A NEW_NS NEW_ALLOCS
while IFS= read -r line; do
    if [[ ! "$line" =~ ^Benchmark ]]; then
        continue
    fi
    # go test -bench output:  Name-16  N  ns/op-val  ns/op  B/op-val  B/op  allocs-val  allocs/op
    name=$(echo "$line" | awk '{print $1}' | sed 's/-[0-9]*$//')
    ns_op=$(echo "$line" | awk '{print $3}' | tr -d '[:alpha:]')
    allocs_op=$(echo "$line" | awk '{print $7}' | tr -d '[:alpha:]')
    if [ -n "$name" ] && [ -n "$ns_op" ] && [ -n "$allocs_op" ]; then
        NEW_NS["$name"]=$ns_op
        NEW_ALLOCS["$name"]=$allocs_op
    fi
done <<< "$BENCH_OUT"

# ---- Rebaseline ----
if [ "$REBASELINE" = true ]; then
    cat > "$BASELINE_PATH" <<JSONHEAD
{
  "meta": {
    "goos": "${goos}",
    "goarch": "${goarch}",
    "go_version": "${go_version}",
    "gomaxprocs": "${gomaxprocs}",
    "cpu": "${cpu}",
    "captured": "${captured}"
  },
  "benchmarks": {
JSONHEAD
    first=true
    for name in "${!NEW_NS[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            echo "," >> "$BASELINE_PATH"
        fi
        printf '    "%s": {\n      "ns_op": %s,\n      "allocs_op": %s\n    }' \
            "$name" "${NEW_NS[$name]}" "${NEW_ALLOCS[$name]}" >> "$BASELINE_PATH"
    done
    printf "\n  }\n}\n" >> "$BASELINE_PATH"
    echo "Rebaselined to $BASELINE_PATH"
    exit 0
fi

# ---- Read baseline metadata ----
# Use jq if available; otherwise fall back to awk-based parser.
if command -v jq &>/dev/null; then
    base_goos=$(jq -r '.meta.goos' "$BASELINE_PATH")
    base_goarch=$(jq -r '.meta.goarch' "$BASELINE_PATH")
    base_go_version=$(jq -r '.meta.go_version' "$BASELINE_PATH")
    base_gomaxprocs=$(jq -r '.meta.gomaxprocs' "$BASELINE_PATH")
    base_cpu=$(jq -r '.meta.cpu' "$BASELINE_PATH")

    # Read benchmark values
    declare -A BASELINE_NS BASELINE_ALLOCS
    while IFS= read -r name; do
        [ -z "$name" ] && continue
        BASELINE_NS["$name"]=$(jq -r ".benchmarks[\"$name\"].ns_op" "$BASELINE_PATH")
        BASELINE_ALLOCS["$name"]=$(jq -r ".benchmarks[\"$name\"].allocs_op" "$BASELINE_PATH")
    done < <(jq -r '.benchmarks | keys[]' "$BASELINE_PATH")
else
    # Awk-based fallback — also extracts the meta block
    eval "$(awk '
        function trim(s) { gsub(/^[ \t"]+|[ \t",]+$/, "", s); return s }

        /"goos"/      { _meta["goos"]      = trim(substr($0, index($0, ":")+1)) }
        /"goarch"/    { _meta["goarch"]    = trim(substr($0, index($0, ":")+1)) }
        /"go_version"/{ _meta["go_version"]= trim(substr($0, index($0, ":")+1)) }
        /"gomaxprocs"/{ _meta["gomaxprocs"]= trim(substr($0, index($0, ":")+1)) }
        /"cpu"/       { _meta["cpu"]       = trim(substr($0, index($0, ":")+1)) }

        /"Benchmark/  { gsub(/[",]/, "", $1); name=$1 }
        /"ns_op"/     { split($0, a, ":"); gsub(/[ ,]/, "", a[2]); ns[name]=a[2] }
        /"allocs_op"/ { split($0, a, ":"); gsub(/[ ,]/, "", a[2]); allocs[name]=a[2] }

        END {
            for (k in _meta) printf "base_%s=%s\n", k, _meta[k]
            for (n in ns)     printf "BASELINE_NS[\"%s\"]=%s\n", n, ns[n]
            for (n in allocs) printf "BASELINE_ALLOCS[\"%s\"]=%s\n", n, allocs[n]
        }
    ' "$BASELINE_PATH")"
fi

# ---- Metadata guard ----
METADATA_FAIL=false
meta_check() {
    local label="$1" actual="$2" expected="$3"
    if [ "$actual" != "$expected" ]; then
        echo "FAIL  metadata ${label}: baseline=${expected}  runner=${actual}"
        METADATA_FAIL=true
    fi
}

meta_check "goos"       "$goos"       "${base_goos:-}"
meta_check "goarch"     "$goarch"     "${base_goarch:-}"
meta_check "go_version" "$go_version" "${base_go_version:-}"
meta_check "gomaxprocs" "$gomaxprocs" "${base_gomaxprocs:-}"

if [ "$METADATA_FAIL" = true ]; then
    echo ""
    echo "METADATA MISMATCH — baseline captured on a different machine/environment."
    echo "Re-baseline with --rebaseline if this is the intended comparison host."
    exit 1
fi

# ---- Benchmark comparison ----
TOLERANCE_NS="${LURPIC_PERF_TOLERANCE_NS:-20}"
TOLERANCE_ALLOC="${LURPIC_PERF_TOLERANCE_ALLOC:-25}"

HAD_ERROR=false

for name in "${!NEW_NS[@]}"; do
    base_ns="${BASELINE_NS[$name]:-}"
    base_alloc="${BASELINE_ALLOCS[$name]:-}"

    if [ -z "$base_ns" ]; then
        echo "WARN: no baseline for $name — skipping"
        continue
    fi

    new_ns="${NEW_NS[$name]}"
    new_alloc="${NEW_ALLOCS[$name]}"

    ns_pct=$(echo "scale=2; ($new_ns - $base_ns) / $base_ns * 100" | bc -l 2>/dev/null || echo 0)
    alloc_pct=$(echo "scale=2; ($new_alloc - $base_alloc) / $base_alloc * 100" | bc -l 2>/dev/null || echo 0)

    NS_FAIL=false; ALLOC_FAIL=false
    if [ "$(echo "$ns_pct > $TOLERANCE_NS" | bc -l 2>/dev/null)" = 1 ]; then NS_FAIL=true; fi
    if [ "$(echo "$alloc_pct > $TOLERANCE_ALLOC" | bc -l 2>/dev/null)" = 1 ]; then ALLOC_FAIL=true; fi

    if [ "$NS_FAIL" = true ] || [ "$ALLOC_FAIL" = true ]; then
        printf "FAIL  %s\n" "$name"
        echo "  ns/op:    baseline=$base_ns  now=$new_ns  change=${ns_pct}%  (tolerance=${TOLERANCE_NS}%)"
        echo "  allocs/op: baseline=$base_alloc  now=$new_alloc  change=${alloc_pct}%  (tolerance=${TOLERANCE_ALLOC}%)"
        HAD_ERROR=true
    else
        printf "OK    %s  ns/op=%s (%+.1f%%)  allocs/op=%s (%+.1f%%)\n" \
            "$name" "$new_ns" "$ns_pct" "$new_alloc" "$alloc_pct"
    fi
done

if [ "$HAD_ERROR" = true ]; then
    echo ""
    echo "PERFORMANCE REGRESSION DETECTED — investigate or run with --rebaseline if intentional."
    exit 1
fi

echo ""
echo "All benchmarks within tolerance."
exit 0
