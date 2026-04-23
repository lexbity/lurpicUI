package engine

import (
	"fmt"
	"sync"
	"time"

	"codeburg.org/lexbit/ui_replay/model"
)

// CommitGate validates and gates result commits to prevent stale data from advancing checkpoints.
type CommitGate struct {
	expectedVersion int64
	actualVersion   int64
	committed       bool
	validatedAt     time.Time
	mu              sync.RWMutex
}

// NewCommitGate creates a new commit gate with an expected version.
func NewCommitGate(expectedVersion int64) *CommitGate {
	return &CommitGate{
		expectedVersion: expectedVersion,
	}
}

// Validate checks if the actual version matches the expected version.
func (cg *CommitGate) Validate(actualVersion int64) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	cg.actualVersion = actualVersion
	cg.validatedAt = time.Now()

	if actualVersion != cg.expectedVersion {
		return fmt.Errorf("version mismatch: expected %d, got %d", cg.expectedVersion, actualVersion)
	}

	return nil
}

// Commit marks the result as committed if validation passed.
func (cg *CommitGate) Commit() error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	if cg.actualVersion == 0 {
		return fmt.Errorf("commit attempted without validation")
	}

	if cg.actualVersion != cg.expectedVersion {
		return fmt.Errorf("commit blocked: version mismatch (%d != %d)", cg.actualVersion, cg.expectedVersion)
	}

	cg.committed = true
	return nil
}

// IsCommitted returns true if the result was successfully committed.
func (cg *CommitGate) IsCommitted() bool {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.committed
}

// GetVersionInfo returns the version information.
func (cg *CommitGate) GetVersionInfo() (expected, actual int64, validated bool) {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.expectedVersion, cg.actualVersion, cg.actualVersion != 0
}

// UnstableWaitHandler handles waits that may become unstable.
type UnstableWaitHandler struct {
	timeout          time.Duration
	maxRetries       int
	backoffStrategy  BackoffStrategy
	unstableDetected bool
	attempts         int
	mu               sync.RWMutex
}

// BackoffStrategy defines how to backoff between retries.
type BackoffStrategy func(attempt int) time.Duration

// DefaultBackoff provides exponential backoff.
func DefaultBackoff(attempt int) time.Duration {
	return time.Duration(attempt) * 100 * time.Millisecond
}

// LinearBackoff provides linear backoff.
func LinearBackoff(attempt int) time.Duration {
	return time.Duration(attempt) * 50 * time.Millisecond
}

// NewUnstableWaitHandler creates a new unstable wait handler.
func NewUnstableWaitHandler(timeout time.Duration, maxRetries int) *UnstableWaitHandler {
	return &UnstableWaitHandler{
		timeout:         timeout,
		maxRetries:      maxRetries,
		backoffStrategy: DefaultBackoff,
	}
}

// SetBackoffStrategy sets a custom backoff strategy.
func (uwh *UnstableWaitHandler) SetBackoffStrategy(strategy BackoffStrategy) {
	uwh.backoffStrategy = strategy
}

// Execute runs the wait with instability detection.
func (uwh *UnstableWaitHandler) Execute(waitFn func() error) error {
	uwh.mu.Lock()
	uwh.unstableDetected = false
	uwh.attempts = 0
	uwh.mu.Unlock()

	lastErr := fmt.Errorf("max retries exceeded")

	for attempt := 0; attempt < uwh.maxRetries; attempt++ {
		uwh.mu.Lock()
		uwh.attempts = attempt + 1
		// Any retry indicates potential instability
		if attempt > 0 {
			uwh.unstableDetected = true
		}
		uwh.mu.Unlock()

		// Try the wait
		err := waitFn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Backoff before retry
		if attempt < uwh.maxRetries-1 {
			time.Sleep(uwh.backoffStrategy(attempt))
		}
	}

	return fmt.Errorf("unstable wait after %d attempts: %w", uwh.maxRetries, lastErr)
}

// IsUnstable returns true if instability was detected.
func (uwh *UnstableWaitHandler) IsUnstable() bool {
	uwh.mu.RLock()
	defer uwh.mu.RUnlock()
	return uwh.unstableDetected
}

// GetAttempts returns the number of attempts made.
func (uwh *UnstableWaitHandler) GetAttempts() int {
	uwh.mu.RLock()
	defer uwh.mu.RUnlock()
	return uwh.attempts
}

// EnvironmentNormalizer removes incidental noise from environment data.
type EnvironmentNormalizer struct {
	fieldsToNormalize []string
}

// NewEnvironmentNormalizer creates a new environment normalizer.
func NewEnvironmentNormalizer() *EnvironmentNormalizer {
	return &EnvironmentNormalizer{
		fieldsToNormalize: []string{
			"hostname",
			"process_id",
			"thread_id",
			"memory_address",
		},
	}
}

// Normalize removes incidental noise from environment data.
func (en *EnvironmentNormalizer) Normalize(data map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{})

	for key, value := range data {
		// Skip noise fields
		if en.isNoiseField(key) {
			continue
		}

		// Normalize timestamps to relative values
		if en.isTimestampField(key) {
			if ts, ok := value.(time.Time); ok {
				normalized[key] = en.normalizeTimestamp(ts)
				continue
			}
		}

		normalized[key] = value
	}

	return normalized
}

// NormalizeArtifact removes incidental noise from artifact data.
func (en *EnvironmentNormalizer) NormalizeArtifact(artifactType string, data []byte) ([]byte, error) {
	// For now, just return data as-is for binary artifacts
	// Text artifacts could be processed to remove timestamps, etc.
	return data, nil
}

// isNoiseField checks if a field is considered noise.
func (en *EnvironmentNormalizer) isNoiseField(field string) bool {
	for _, noise := range en.fieldsToNormalize {
		if field == noise {
			return true
		}
	}
	return false
}

// isTimestampField checks if a field is a timestamp.
func (en *EnvironmentNormalizer) isTimestampField(field string) bool {
	return field == "timestamp" || field == "created_at" || field == "modified_at"
}

// normalizeTimestamp converts absolute timestamp to relative offset.
func (en *EnvironmentNormalizer) normalizeTimestamp(ts time.Time) string {
	return "[TIMESTAMP]"
}

// RerunComparisonHooks provides hooks for rerun comparison.
type RerunComparisonHooks struct {
	OnBeforeRerun   func(runIndex int)
	OnAfterRerun    func(runIndex int, result *model.RunResult, err error)
	OnDriftDetected func(report *DriftReport)
}

// DefaultRerunHooks provides default no-op hooks.
func DefaultRerunHooks() *RerunComparisonHooks {
	return &RerunComparisonHooks{
		OnBeforeRerun:   func(runIndex int) {},
		OnAfterRerun:    func(runIndex int, result *model.RunResult, err error) {},
		OnDriftDetected: func(report *DriftReport) {},
	}
}
