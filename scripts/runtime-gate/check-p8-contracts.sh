#!/bin/bash
# P8 source contract checks per the runtime performance specification.

set -e

echo "Checking P8 source contracts..."

# Check that job and asset code don't use NumCPU()-1 pattern
echo "  Checking for NumCPU()-1 pattern in runtime/config.go and job/job.go..."
if grep -E "NumCPU\(\)[[:space:]]*-[[:space:]]*1" runtime/config.go job/job.go 2>/dev/null; then
    echo "FAIL: Found NumCPU()-1 pattern"
    exit 1
fi
echo "    PASS: No NumCPU()-1 pattern found"

# Check that job/reserve_test.go exists and has required tests
echo "  Checking for job/resource_test.go..."
if [ ! -f "job/resource_test.go" ]; then
    echo "FAIL: job/resource_test.go not found"
    exit 1
fi
echo "    PASS: job/resource_test.go exists"

# Check for required test functions in job/resource_test.go
echo "  Checking for required P8 job tests..."
for test in "TestJobReservationCoversAllLiveStages" "TestLateCancelledResultReleasesWithoutCommit" "TestFullResultQueueDoesNotSpawnWorkers" "TestWorkerCountDoesNotChangeVirtualTrace"; do
    if ! grep -q "func $test" job/resource_test.go; then
        echo "FAIL: $test not found in job/resource_test.go"
        exit 1
    fi
done
echo "    PASS: All required job tests present"

# Check that assets/resource_test.go exists
echo "  Checking for assets/resource_test.go..."
if [ ! -f "assets/resource_test.go" ]; then
    echo "FAIL: assets/resource_test.go not found"
    exit 1
fi
echo "    PASS: assets/resource_test.go exists"

# Check for required test functions in assets/resource_test.go
echo "  Checking for required P8 assets tests..."
for test in "TestConfiguredBudgetReachesActualManager" "TestWalletReservationLifecycle" "TestWalletRefusesOverCapacity" "TestWalletBatchReservation"; do
    if ! grep -q "func $test" assets/resource_test.go; then
        echo "FAIL: $test not found in assets/resource_test.go"
        exit 1
    fi
done
echo "    PASS: All required assets tests present"

# Check that NewManagerWithBudget exists
echo "  Checking for NewManagerWithBudget..."
if ! grep -q "func NewManagerWithBudget" assets/job.go; then
    echo "FAIL: NewManagerWithBudget not found in assets/job.go"
    exit 1
fi
echo "    PASS: NewManagerWithBudget exists"

# Check that PoolScheduler exists
echo "  Checking for PoolScheduler..."
if ! grep -q "type PoolScheduler" assets/job.go; then
    echo "FAIL: PoolScheduler not found in assets/job.go"
    exit 1
fi
echo "    PASS: PoolScheduler exists"

echo "All P8 source contracts passed!"
