package engine

import (
	"errors"
	"testing"
	"time"
)

func TestCommitGate_ValidateAndCommit(t *testing.T) {
	t.Run("valid version allows commit", func(t *testing.T) {
		gate := NewCommitGate(100)

		err := gate.Validate(100)
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}

		err = gate.Commit()
		if err != nil {
			t.Errorf("Commit() error = %v", err)
		}

		if !gate.IsCommitted() {
			t.Error("IsCommitted() should be true after successful commit")
		}
	})

	t.Run("invalid version blocks commit", func(t *testing.T) {
		gate := NewCommitGate(100)

		err := gate.Validate(200)
		if err == nil {
			t.Error("Validate() should error with mismatched version")
		}

		err = gate.Commit()
		if err == nil {
			t.Error("Commit() should error with mismatched version")
		}

		if gate.IsCommitted() {
			t.Error("IsCommitted() should be false after failed commit")
		}
	})

	t.Run("commit without validation fails", func(t *testing.T) {
		gate := NewCommitGate(100)

		err := gate.Commit()
		if err == nil {
			t.Error("Commit() should error without validation")
		}
	})

	t.Run("version info", func(t *testing.T) {
		gate := NewCommitGate(100)
		gate.Validate(100)

		expected, actual, validated := gate.GetVersionInfo()
		if expected != 100 {
			t.Errorf("Expected = %d, want 100", expected)
		}
		if actual != 100 {
			t.Errorf("Actual = %d, want 100", actual)
		}
		if !validated {
			t.Error("Validated should be true")
		}
	})
}

func TestUnstableWaitHandler_Execute(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		handler := NewUnstableWaitHandler(1*time.Second, 3)

		attempts := 0
		waitFn := func() error {
			attempts++
			return nil
		}

		err := handler.Execute(waitFn)
		if err != nil {
			t.Errorf("Execute() error = %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
		if handler.IsUnstable() {
			t.Error("IsUnstable() should be false on success")
		}
	})

	t.Run("retries on failure then succeeds", func(t *testing.T) {
		handler := NewUnstableWaitHandler(1*time.Second, 3)

		attempts := 0
		waitFn := func() error {
			attempts++
			if attempts < 2 {
				return errors.New("temporary error")
			}
			return nil
		}

		err := handler.Execute(waitFn)
		if err != nil {
			t.Errorf("Execute() error = %v", err)
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
		if !handler.IsUnstable() {
			t.Error("IsUnstable() should be true after retry")
		}
	})

	t.Run("fails after max retries", func(t *testing.T) {
		handler := NewUnstableWaitHandler(1*time.Second, 2)

		attempts := 0
		waitFn := func() error {
			attempts++
			return errors.New("persistent error")
		}

		err := handler.Execute(waitFn)
		if err == nil {
			t.Error("Execute() should error after max retries")
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("custom backoff strategy", func(t *testing.T) {
		handler := NewUnstableWaitHandler(1*time.Second, 2)
		handler.SetBackoffStrategy(LinearBackoff)

		attempts := 0
		waitFn := func() error {
			attempts++
			if attempts == 1 {
				return errors.New("error")
			}
			return nil
		}

		err := handler.Execute(waitFn)
		if err != nil {
			t.Errorf("Execute() error = %v", err)
		}
	})

	t.Run("attempts tracking", func(t *testing.T) {
		handler := NewUnstableWaitHandler(1*time.Second, 2)

		waitFn := func() error {
			return errors.New("error")
		}

		handler.Execute(waitFn)
		if handler.GetAttempts() != 2 {
			t.Errorf("GetAttempts() = %d, want 2", handler.GetAttempts())
		}
	})
}

func TestEnvironmentNormalizer_Normalize(t *testing.T) {
	normalizer := NewEnvironmentNormalizer()

	t.Run("removes noise fields", func(t *testing.T) {
		data := map[string]interface{}{
			"hostname":    "test-server",
			"process_id":  12345,
			"scene_id":    "basic",
			"step_count":  5,
		}

		normalized := normalizer.Normalize(data)

		if _, exists := normalized["hostname"]; exists {
			t.Error("hostname should be removed")
		}
		if _, exists := normalized["process_id"]; exists {
			t.Error("process_id should be removed")
		}
		if normalized["scene_id"] != "basic" {
			t.Error("scene_id should be preserved")
		}
	})

	t.Run("normalizes timestamps", func(t *testing.T) {
		data := map[string]interface{}{
			"timestamp": time.Now(),
			"created_at": time.Now(),
			"value":     42,
		}

		normalized := normalizer.Normalize(data)

		if normalized["timestamp"] != "[TIMESTAMP]" {
			t.Errorf("timestamp = %v, want [TIMESTAMP]", normalized["timestamp"])
		}
		if normalized["created_at"] != "[TIMESTAMP]" {
			t.Errorf("created_at = %v, want [TIMESTAMP]", normalized["created_at"])
		}
		if normalized["value"] != 42 {
			t.Error("value should be preserved")
		}
	})
}

func TestEnvironmentNormalizer_NormalizeArtifact(t *testing.T) {
	normalizer := NewEnvironmentNormalizer()

	t.Run("preserves binary data", func(t *testing.T) {
		data := []byte{0x00, 0x01, 0x02, 0x03}
		result, err := normalizer.NormalizeArtifact("screenshot", data)
		if err != nil {
			t.Errorf("NormalizeArtifact() error = %v", err)
		}
		if string(result) != string(data) {
			t.Error("Binary data should be preserved")
		}
	})
}

func TestDefaultRerunHooks(t *testing.T) {
	hooks := DefaultRerunHooks()

	t.Run("hooks are callable", func(t *testing.T) {
		hooks.OnBeforeRerun(1)
		hooks.OnAfterRerun(1, nil, nil)
		hooks.OnDriftDetected(&DriftReport{Detected: false})
	})
}
